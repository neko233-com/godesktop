package extensions

import (
	"bufio"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"sync"
)

//go:embed host.cjs
var hostSource string

type Event struct {
	Type    string `json:"type"`
	Text    string `json:"text"`
	Path    string `json:"path"`
	Channel string `json:"channel"`
}
type response struct {
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  string          `json:"error"`
	Event  *Event          `json:"event"`
}

// Host runs trusted local extension code with the user's permissions. It is not
// a security sandbox. Events is buffered; the UI should drain it continuously.
type Host struct {
	Events  chan Event
	cmd     *exec.Cmd
	input   io.WriteCloser
	mu      sync.Mutex
	next    int
	pending map[int]chan response
	closed  bool
	done    chan struct{}
	endErr  error
}

func Start(ctx context.Context, workspace string, installed []Extension) (*Host, error) {
	node, err := exec.LookPath("node")
	if err != nil {
		return nil, errors.New("extension host requires Node.js 22 or later on PATH")
	}
	workspace, err = filepath.Abs(workspace)
	if err != nil {
		return nil, err
	}
	config, _ := json.Marshal(struct {
		Workspace  string      `json:"workspace"`
		Extensions []Extension `json:"extensions"`
	}{workspace, installed})
	cmd := exec.CommandContext(ctx, node, "-e", hostSource, string(config))
	input, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		input.Close()
		return nil, err
	}
	// Protocol output is isolated from extension console logging.
	cmd.Stderr = io.Discard
	h := &Host{Events: make(chan Event, 256), cmd: cmd, input: input, pending: make(map[int]chan response), done: make(chan struct{})}
	if err = cmd.Start(); err != nil {
		input.Close()
		return nil, err
	}
	go func() {
		s := bufio.NewScanner(output)
		s.Buffer(make([]byte, 4096), 4<<20)
		for s.Scan() {
			var r response
			if err = json.Unmarshal(s.Bytes(), &r); err != nil {
				break
			}
			if r.Event != nil {
				select {
				case h.Events <- *r.Event:
				default:
				}
				continue
			}
			h.mu.Lock()
			ch := h.pending[r.ID]
			delete(h.pending, r.ID)
			h.mu.Unlock()
			if ch != nil {
				ch <- r
			}
		}
		// Kill before Wait when malformed/oversized output prevents draining.
		if s.Err() != nil || err != nil {
			_ = cmd.Process.Kill()
		}
		waitErr := cmd.Wait()
		h.mu.Lock()
		h.closed = true
		h.endErr = errors.Join(err, s.Err(), waitErr)
		for id, ch := range h.pending {
			ch <- response{Error: "extension host exited"}
			delete(h.pending, id)
		}
		h.mu.Unlock()
		close(h.Events)
		close(h.done)
	}()
	return h, nil
}

// Call uses JSON-RPC-style request IDs. Cancellation terminates an unresponsive
// host, including synchronous loops in extension activation or command handlers.
func (h *Host) Call(ctx context.Context, method string, params any, result any) error {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return errors.New("extension host is closed")
	}
	h.next++
	id := h.next
	ch := make(chan response, 1)
	h.pending[id] = ch
	data, err := json.Marshal(struct {
		ID     int    `json:"id"`
		Method string `json:"method"`
		Params any    `json:"params"`
	}{id, method, params})
	if err == nil {
		_, err = h.input.Write(append(data, '\n'))
	}
	if err != nil {
		delete(h.pending, id)
	}
	h.mu.Unlock()
	if err != nil {
		return err
	}
	select {
	case r := <-ch:
		if r.Error != "" {
			return fmt.Errorf("extension host: %s", r.Error)
		}
		if result != nil {
			return json.Unmarshal(r.Result, result)
		}
		return nil
	case <-ctx.Done():
		_ = h.cmd.Process.Kill()
		return ctx.Err()
	}
}

func (h *Host) Close() error {
	h.mu.Lock()
	if !h.closed {
		_ = h.input.Close()
	}
	h.mu.Unlock()
	_ = h.cmd.Process.Kill()
	<-h.done
	return nil
}
