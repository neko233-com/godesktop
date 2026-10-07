package editor

import (
	"context"
	"errors"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestPreparedHistoryMatchesDirectAndSavedBranch(t *testing.T) {
	b, _ := New("你😀 one\r\nlast\r\n")
	oracle, _ := New(b.Text())
	for _, target := range []*Buffer{b, oracle} {
		target.ReplaceSelection("prefix ")
		target.MarkSaved()
		target.Reload("new😀\nline\n")
		target.ReplaceSelection("unsaved ")
	}
	for _, redo := range []bool{false, false, false, true, true, true, false} {
		s := b.HistorySnapshot()
		var p *PreparedEdit
		var err error
		var want ChangeEvent
		if redo {
			p, err = s.PrepareRedo(context.Background())
			want, _ = oracle.Redo()
		} else {
			p, err = s.PrepareUndo(context.Background())
			want, _ = oracle.Undo()
		}
		if err != nil {
			t.Fatal(err)
		}
		got, err := b.CommitPrepared(p)
		if err != nil || !reflect.DeepEqual(got, want) || b.Text() != oracle.Text() || b.Selection() != oracle.Selection() || b.Dirty() != oracle.Dirty() || b.revision != oracle.revision || b.nextRevision != oracle.nextRevision || b.historyBytes != oracle.historyBytes || !reflect.DeepEqual(b.undo, oracle.undo) || !reflect.DeepEqual(b.redo, oracle.redo) {
			t.Fatalf("prepared history differs from direct history: redo=%v err=%v", redo, err)
		}
		if b.CanCommit(p) {
			t.Fatal("history plan reusable")
		}
	}
	for _, target := range []*Buffer{b, oracle} {
		target.ReplaceSelection("branch ")
	}
	if b.Text() != oracle.Text() || b.nextRevision != oracle.nextRevision || b.HistorySnapshot().RedoRevision() != 0 {
		t.Fatal("new branch retained redo or reused revision")
	}
}

func TestPreparedHistoryRejectsStaleAndEmpty(t *testing.T) {
	b, _ := New("value")
	if _, err := b.HistorySnapshot().PrepareUndo(context.Background()); !errors.Is(err, ErrHistoryEmpty) {
		t.Fatal(err)
	}
	if _, err := (HistorySnapshot{}).PrepareUndo(context.Background()); err == nil {
		t.Fatal("unowned history accepted")
	}
	b.ReplaceSelection("new ")
	s := b.HistorySnapshot()
	p, err := s.PrepareUndo(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	other, _ := New(b.Text())
	if other.CanCommit(p) {
		t.Fatal("another document accepted history")
	}
	b.SetSelection(Selection{Position{0, 1}, Position{0, 1}})
	if _, err := b.CommitPrepared(p); !errors.Is(err, ErrPreparedStale) {
		t.Fatal("new caret overwritten", err)
	}
	b.SetSelection(s.source.Selection)
	b.Undo()
	b.Redo()
	if b.CanCommit(p) {
		t.Fatal("same revision at a newer version accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.PrepareUndo(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestFrozenHistoryWorkerAndBoundedCommit(t *testing.T) {
	b, _ := New(strings.Repeat("x", 8<<20))
	b.ReplaceSelection("😀")
	s := b.HistorySnapshot()
	result := make(chan *PreparedEdit, 1)
	failure := make(chan error, 1)
	go func() { p, err := s.PrepareUndo(context.Background()); result <- p; failure <- err }()
	b.Undo() // Zeros the live slice entry while the worker uses its value copy.
	for range 8 {
		b.ReplaceSelection("y")
	}
	p := <-result
	if err := <-failure; err != nil || b.CanCommit(p) || len(p.Snapshot().Line(0)) != 8<<20 || s.UndoRevision() == 0 {
		t.Fatal("snapshot shared live history", err)
	}
	p, err := b.HistorySnapshot().PrepareUndo(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	_, err = b.CommitPrepared(p)
	runtime.ReadMemStats(&after)
	if err != nil || after.TotalAlloc-before.TotalAlloc > 32<<10 {
		t.Fatalf("8 MiB history commit alloc=%d err=%v", after.TotalAlloc-before.TotalAlloc, err)
	}
	t.Logf("8 MiB prepared history commit new Go allocation=%d", after.TotalAlloc-before.TotalAlloc)
}
