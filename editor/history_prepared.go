package editor

import (
	"context"
	"errors"
)

var ErrHistoryEmpty = errors.New("no history entry in the requested direction")

// HistorySnapshot freezes only the top entries, by value, together with the
// immutable source text. It never shares the mutable undo/redo slice. Capture on
// the buffer's owning thread, then prepare on a worker. Ordinary Snapshot does
// not retain history strings.
type HistorySnapshot struct {
	source               Snapshot
	undo, redo           history
	undoDepth, redoDepth int
	revision             uint64
}

func (b *Buffer) HistorySnapshot() HistorySnapshot {
	s := HistorySnapshot{source: b.Snapshot(), undoDepth: len(b.undo), redoDepth: len(b.redo), revision: b.revision}
	if len(b.undo) > 0 {
		s.undo = b.undo[len(b.undo)-1]
	}
	if len(b.redo) > 0 {
		s.redo = b.redo[len(b.redo)-1]
	}
	return s
}

// Revision tokens identify history entries within one buffer identity. Zero
// means an empty stack. Applications must also guard their document identity.
func (s HistorySnapshot) UndoRevision() uint64 { return s.undo.afterRevision }
func (s HistorySnapshot) RedoRevision() uint64 { return s.redo.afterRevision }

func (s HistorySnapshot) matches(b *Buffer) bool {
	return s.revision == b.revision && s.undoDepth == len(b.undo) && s.redoDepth == len(b.redo) && s.UndoRevision() == b.HistorySnapshot().UndoRevision() && s.RedoRevision() == b.HistorySnapshot().RedoRevision()
}

func (s HistorySnapshot) PrepareUndo(ctx context.Context) (*PreparedEdit, error) {
	return s.prepare(ctx, false)
}
func (s HistorySnapshot) PrepareRedo(ctx context.Context) (*PreparedEdit, error) {
	return s.prepare(ctx, true)
}

func (s HistorySnapshot) prepare(ctx context.Context, redo bool) (*PreparedEdit, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.source.identity == nil || len(s.source.lines) == 0 {
		return nil, errors.New("cannot prepare unowned history")
	}
	item, depth, kind := s.undo, s.undoDepth, uint8(1)
	if redo {
		item, depth, kind = s.redo, s.redoDepth, 2
	}
	if depth == 0 {
		return nil, ErrHistoryEmpty
	}
	b := &Buffer{identity: s.source.identity, lines: s.source.lines, eol: s.source.EOL, version: s.source.Version, selection: s.source.Selection, revision: s.revision}
	var change ChangeEvent
	if redo {
		b.redo = []history{item}
		change, _ = b.Redo()
	} else {
		b.undo = []history{item}
		b.historyBytes = len(item.removed) + len(item.inserted)
		change, _ = b.Undo()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &PreparedEdit{before: s.source, after: b.Snapshot(), change: change, undo: item, historyKind: kind, historySource: s}, nil
}
