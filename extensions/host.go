package extensions

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"os/exec"
	"path/filepath"
	"sync/atomic"

	"github.com/neko233-com/godesktop/lsp"
)

//go:embed host.cjs
var hostSource string

type Event struct {
	Type    string          `json:"type"`
	Text    string          `json:"text"`
	Path    string          `json:"path"`
	Channel string          `json:"channel"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// Host runs trusted local extension code with the user's permissions. It is not
// a security sandbox. Drain Events continuously; edit requests use acknowledged
// RPC handlers rather than this optional notification stream.
type Host struct {
	Events  chan Event
	rpc     *lsp.Client
	done    chan struct{}
	dropped atomic.Uint64
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
	client, err := lsp.Start(ctx, lsp.Command{Executable: node, Arguments: []string{"-e", hostSource, string(config)}, Directory: workspace})
	if err != nil {
		return nil, err
	}
	h := &Host{Events: make(chan Event, 256), rpc: client, done: make(chan struct{})}
	go func() {
		defer close(h.done)
		defer close(h.Events)
		for notification := range client.Notifications() {
			if notification.Method != "godesktop/event" {
				continue
			}
			var event Event
			if json.Unmarshal(notification.Params, &event) != nil {
				continue
			}
			select {
			case h.Events <- event:
			default:
				h.dropped.Add(1)
			}
		}
	}()
	return h, nil
}

// Register handles requests from extensions, including versioned native edits.
// Handlers must observe cancellation and dispatch UI mutations to the UI thread.
func (h *Host) Register(method string, handler lsp.Handler) { h.rpc.Register(method, handler) }
func (h *Host) DroppedEvents() uint64                       { return h.dropped.Load() + h.rpc.DroppedNotifications() }

// Call terminates a timed-out extension host, including synchronous JS loops.
func (h *Host) Call(ctx context.Context, method string, params, result any) error {
	err := h.rpc.Call(ctx, method, params, result)
	if ctx.Err() != nil {
		h.rpc.Close()
		return ctx.Err()
	}
	return err
}
func (h *Host) Close() error { h.rpc.Close(); <-h.done; return nil }
