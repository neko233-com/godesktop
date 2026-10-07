package extensions

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"

	"github.com/neko233-com/godesktop/lsp"
)

//go:embed host.cjs
var hostSource string

//go:embed terminals.cjs
var terminalSource string

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
	Events      chan Event
	rpc         *lsp.Client
	done        chan struct{}
	dropped     atomic.Uint64
	runtimeRoot string
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
	config, err := json.Marshal(struct {
		Workspace  string      `json:"workspace"`
		Extensions []Extension `json:"extensions"`
	}{workspace, installed})
	if err != nil {
		return nil, err
	}
	// Files avoid Windows' 32 KiB command-line limit for source and manifests.
	// Each process owns an isolated temporary runtime; no credentials are stored.
	if len(config) > lsp.MaxMessageBytes {
		return nil, errors.New("extension configuration exceeds 16 MiB")
	}
	runtimeRoot, err := os.MkdirTemp("", "godesktop-extension-host-")
	if err != nil {
		return nil, err
	}
	sourcePath, configPath := filepath.Join(runtimeRoot, "host.cjs"), filepath.Join(runtimeRoot, "config.json")
	if err := errors.Join(os.WriteFile(sourcePath, []byte(hostSource), 0600), os.WriteFile(filepath.Join(runtimeRoot, "terminals.cjs"), []byte(terminalSource), 0600), os.WriteFile(configPath, config, 0600)); err != nil {
		os.RemoveAll(runtimeRoot)
		return nil, err
	}
	client, err := lsp.Start(ctx, lsp.Command{Executable: node, Arguments: []string{sourcePath, configPath}, Directory: workspace})
	if err != nil {
		os.RemoveAll(runtimeRoot)
		return nil, err
	}
	h := &Host{Events: make(chan Event, 256), rpc: client, done: make(chan struct{}), runtimeRoot: runtimeRoot}
	go func() {
		defer close(h.done)
		defer os.RemoveAll(runtimeRoot)
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
