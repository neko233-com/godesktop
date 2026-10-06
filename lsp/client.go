// Package lsp implements bounded, cancellable JSON-RPC over the standard LSP
// Content-Length transport. It can run language servers without a web UI.
package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const MaxMessageBytes = 16 << 20
const maxHeaderBytes = 8192

type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *RPCError) Error() string { return fmt.Sprintf("LSP RPC %d: %s", e.Code, e.Message) }

type Handler func(context.Context, json.RawMessage) (any, error)
type Notification struct {
	Method string
	Params json.RawMessage
}
type Command struct {
	Executable  string
	Arguments   []string
	Directory   string
	Environment []string // nil inherits the parent environment
}
type packet struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}
type outcome struct {
	result json.RawMessage
	err    error
}
type queued struct {
	ctx     context.Context
	data    []byte
	done    chan error
	started chan struct{}
}
type Client struct {
	reader        io.ReadCloser
	writer        io.WriteCloser
	stop          func()
	done          chan struct{}
	closeOnce     sync.Once
	mu            sync.Mutex
	next          uint64
	pending       map[string]chan outcome
	handlers      map[string]Handler
	err           error
	writes        chan queued
	requestSlots  chan struct{}
	notifications chan Notification
	dropped       atomic.Uint64
	readFinished  chan struct{}
	processDone   chan struct{}
}

func Start(ctx context.Context, command Command) (*Client, error) {
	if command.Executable == "" {
		return nil, errors.New("language server executable is required")
	}
	cmd := exec.CommandContext(ctx, command.Executable, command.Arguments...)
	cmd.WaitDelay = 2 * time.Second
	cmd.Dir = command.Directory
	if command.Environment != nil {
		cmd.Env = command.Environment
	}
	hideServerWindow(cmd)
	input, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		input.Close()
		return nil, err
	}
	// Servers must put protocol frames on stdout. Do not capture credentials
	// or arbitrary server diagnostics in application logs.
	cmd.Stderr = io.Discard
	if err = cmd.Start(); err != nil {
		input.Close()
		output.Close()
		return nil, err
	}
	c := Connect(output, input, func() { _ = cmd.Process.Kill() })
	c.processDone = make(chan struct{})
	go func() {
		defer close(c.processDone)
		<-c.readFinished
		err := cmd.Wait()
		if err != nil {
			c.finish(fmt.Errorf("language server exited: %w", err))
		}
	}()
	return c, nil
}

// Connect owns both streams. Close must unblock any active Read or Write.
func Connect(reader io.ReadCloser, writer io.WriteCloser, stop func()) *Client {
	c := &Client{reader: reader, writer: writer, stop: stop, done: make(chan struct{}), pending: make(map[string]chan outcome), handlers: make(map[string]Handler), writes: make(chan queued, 8), requestSlots: make(chan struct{}, 64), notifications: make(chan Notification, 256), readFinished: make(chan struct{})}
	go c.readLoop()
	go c.writeLoop()
	return c
}
func (c *Client) Notifications() <-chan Notification { return c.notifications }
func (c *Client) DroppedNotifications() uint64       { return c.dropped.Load() }
func (c *Client) Register(method string, handler Handler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if handler == nil {
		delete(c.handlers, method)
	} else {
		c.handlers[method] = handler
	}
}
func (c *Client) finish(err error) {
	c.closeOnce.Do(func() {
		if err == nil {
			err = io.EOF
		}
		c.mu.Lock()
		c.err = err
		for id, ch := range c.pending {
			ch <- outcome{err: err}
			delete(c.pending, id)
		}
		c.mu.Unlock()
		close(c.done)
		if c.stop != nil {
			c.stop()
		}
		c.writer.Close()
		c.reader.Close()
	})
}
func (c *Client) Close() error {
	c.finish(errors.New("language server closed"))
	if c.processDone != nil {
		select {
		case <-c.processDone:
		case <-time.After(3 * time.Second):
			return errors.New("language server process did not exit")
		}
	}
	return nil
}
func (c *Client) send(ctx context.Context, value packet) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if len(data) > MaxMessageBytes {
		return errors.New("LSP message exceeds 16 MiB")
	}
	q := queued{ctx, data, make(chan error, 1), make(chan struct{})}
	select {
	case c.writes <- q:
	case <-ctx.Done():
		return ctx.Err()
	case <-c.done:
		return errors.New("language server closed")
	}
	select {
	case err := <-q.done:
		return err
	case <-c.done:
		return errors.New("language server closed")
	case <-ctx.Done():
		// The endpoint may have stopped draining its pipe. Close guarantees
		// bounded cancellation of a blocked transport write.
		select {
		case err := <-q.done:
			return err
		default:
		}
		select {
		case <-q.started:
		default:
			return ctx.Err()
		}
		c.finish(ctx.Err())
		return ctx.Err()
	}
}
func (c *Client) writeLoop() {
	for {
		select {
		case <-c.done:
			return
		case q := <-c.writes:
			if err := q.ctx.Err(); err != nil {
				q.done <- err
				continue
			}
			close(q.started)
			err := writeFrame(c.writer, q.data)
			q.done <- err
			if err != nil {
				c.finish(err)
				return
			}
		}
	}
}
func writeFrame(w io.Writer, data []byte) error {
	frame := append([]byte(fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))), data...)
	for len(frame) > 0 {
		n, err := w.Write(frame)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		frame = frame[n:]
	}
	return nil
}
func readFrame(r *bufio.Reader) ([]byte, error) {
	length := -1
	headerBytes := 0
	for {
		var line string
		for {
			part, err := r.ReadSlice('\n')
			headerBytes += len(part)
			if headerBytes > maxHeaderBytes {
				return nil, errors.New("LSP headers exceed 8 KiB")
			}
			line += string(part)
			if err == bufio.ErrBufferFull {
				continue
			}
			if err != nil {
				return nil, err
			}
			break
		}
		if line == "\r\n" || line == "\n" {
			break
		}
		key, value, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok {
			return nil, errors.New("malformed LSP header")
		}
		if strings.EqualFold(key, "Content-Length") {
			if length != -1 {
				return nil, errors.New("duplicate LSP Content-Length")
			}
			var err error
			length, err = strconv.Atoi(strings.TrimSpace(value))
			if err != nil || length < 0 || length > MaxMessageBytes {
				return nil, errors.New("invalid LSP Content-Length")
			}
		}
	}
	if length < 0 {
		return nil, errors.New("missing LSP Content-Length")
	}
	data := make([]byte, length)
	_, err := io.ReadFull(r, data)
	return data, err
}
func (c *Client) readLoop() {
	defer close(c.readFinished)
	defer close(c.notifications)
	reader := bufio.NewReader(c.reader)
	for {
		data, err := readFrame(reader)
		if err != nil {
			c.finish(err)
			return
		}
		var p packet
		if err = json.Unmarshal(data, &p); err != nil || p.JSONRPC != "2.0" {
			c.finish(errors.New("invalid LSP JSON-RPC message"))
			return
		}
		if p.Method != "" {
			if len(p.ID) > 0 && string(p.ID) != "null" {
				select {
				case c.requestSlots <- struct{}{}:
					go c.handle(p)
				case <-c.done:
					return
				default:
					c.reply(p.ID, nil, &RPCError{-32000, "too many server requests", nil})
				}
			} else {
				select {
				case c.notifications <- Notification{p.Method, p.Params}:
				default:
					c.dropped.Add(1)
				}
			}
			continue
		}
		c.mu.Lock()
		ch := c.pending[string(p.ID)]
		delete(c.pending, string(p.ID))
		c.mu.Unlock()
		if ch != nil {
			var err error
			if p.Error != nil {
				err = p.Error
			}
			if len(p.Result) == 0 {
				p.Result = json.RawMessage("null")
			}
			ch <- outcome{p.Result, err}
		}
	}
}
func (c *Client) handle(p packet) {
	defer func() { <-c.requestSlots }()
	c.mu.Lock()
	handler := c.handlers[p.Method]
	c.mu.Unlock()
	if handler == nil {
		c.reply(p.ID, nil, &RPCError{-32601, "unsupported server method: " + p.Method, nil})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	go func() {
		select {
		case <-c.done:
			cancel()
		case <-ctx.Done():
		}
	}()
	result, err := handler(ctx, p.Params)
	if err != nil {
		c.reply(p.ID, nil, &RPCError{-32603, err.Error(), nil})
	} else {
		c.reply(p.ID, result, nil)
	}
}
func (c *Client) reply(id json.RawMessage, value any, rpcErr *RPCError) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	p := packet{JSONRPC: "2.0", ID: id, Error: rpcErr}
	if rpcErr == nil {
		data, err := json.Marshal(value)
		if err != nil {
			p.Error = &RPCError{-32603, err.Error(), nil}
		} else {
			p.Result = data
		}
	}
	_ = c.send(ctx, p)
}
func (c *Client) Call(ctx context.Context, method string, params any, result any) error {
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	c.mu.Lock()
	if c.err != nil {
		err := c.err
		c.mu.Unlock()
		return err
	}
	c.next++
	id := strconv.FormatUint(c.next, 10)
	ch := make(chan outcome, 1)
	c.pending[id] = ch
	c.mu.Unlock()
	if err = c.send(ctx, packet{JSONRPC: "2.0", ID: json.RawMessage(id), Method: method, Params: raw}); err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return err
	}
	select {
	case response := <-ch:
		if response.err != nil {
			return response.err
		}
		if result != nil {
			return json.Unmarshal(response.result, result)
		}
		return nil
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		cancelCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = c.Notify(cancelCtx, "$/cancelRequest", map[string]any{"id": json.Number(id)})
		return ctx.Err()
	}
}
func (c *Client) Notify(ctx context.Context, method string, params any) error {
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return c.send(ctx, packet{JSONRPC: "2.0", Method: method, Params: raw})
}
func (c *Client) Shutdown(ctx context.Context) error {
	err := c.Call(ctx, "shutdown", nil, nil)
	if err == nil {
		err = c.Notify(ctx, "exit", nil)
	}
	c.Close()
	return err
}
