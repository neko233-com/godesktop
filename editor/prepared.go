package editor

import (
	"context"
	"errors"
	"slices"
)

// MaxPreparedEdits bounds worker transaction sorting and protocol changes.
const MaxPreparedEdits = 8192

var ErrPreparedStale = errors.New("prepared edit no longer matches document identity, version or selection")

// PreparedEdit is immutable. Prepare it on a worker from an immutable Snapshot,
// then check/commit it on the owning buffer's UI thread. No live Buffer is read
// or changed by the worker. Strings and line arrays retain snapshot ownership.
type PreparedEdit struct {
	before, after Snapshot
	change        ChangeEvent
	undo          history
	noop          bool
}

// Prepare validates and computes a whole transaction without changing its source
// buffer. Input edits/selection must not be mutated concurrently with this call.
// Cancellation discards the private candidate. UTF-16, EOL, overlap and history
// rules are identical to Apply. Source/preview memory limits belong to the app.
func (s Snapshot) Prepare(ctx context.Context, edits []Edit, selection *Selection) (*PreparedEdit, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.identity == nil || len(s.lines) == 0 {
		return nil, errors.New("cannot prepare an unowned snapshot")
	}
	if len(edits) > MaxPreparedEdits {
		return nil, errors.New("prepared transaction exceeds edit limit")
	}
	if selection != nil {
		value := *selection
		selection = &value
	}
	p := &PreparedEdit{before: s, noop: len(edits) == 0}
	candidate := &Buffer{identity: s.identity, lines: s.lines, eol: s.EOL, version: s.Version, selection: s.Selection}
	var err error
	p.change, err = candidate.apply(ctx, edits, selection, &p.undo)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	p.after = candidate.Snapshot()
	return p, nil
}

// Snapshot exposes the immutable prospective text and selection for preview.
func (p *PreparedEdit) Snapshot() Snapshot { return p.after }

// CanCommit performs allocation-free metadata validation. A new Buffer with the
// same text/version is a different identity. Moving the caret invalidates a plan
// whose prepared selection was based on the old caret. Call only on the UI thread.
func (b *Buffer) CanCommit(p *PreparedEdit) bool {
	return p != nil && p.before.identity != nil && len(p.after.lines) > 0 && p.before.identity == b.identity && p.before.Version == b.version && p.before.Selection == b.selection
}

// CommitPrepared adopts precomputed lines while keeping this buffer's identity,
// monotonic version, saved revision and undo/redo history. No source text is
// scanned/copied; only returned protocol changes and bounded history are handled.
// Preflight every buffer with CanCommit before committing a UI-owned bulk edit.
func (b *Buffer) CommitPrepared(p *PreparedEdit) (ChangeEvent, error) {
	if !b.CanCommit(p) {
		return ChangeEvent{}, ErrPreparedStale
	}
	if p.noop {
		return ChangeEvent{Version: b.version}, nil
	}
	b.lines, b.eol, b.selection = p.after.lines, p.after.EOL, p.after.Selection
	b.nextRevision++
	undo := p.undo
	undo.beforeRevision, undo.afterRevision = b.revision, b.nextRevision
	b.revision = b.nextRevision
	b.record(undo)
	b.version++
	return ChangeEvent{Version: b.version, Changes: slices.Clone(p.change.Changes)}, nil
}
