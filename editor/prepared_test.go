package editor

import (
	"context"
	"errors"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestPreparedTransactionKeepsSavedHistoryAndOracle(t *testing.T) {
	b, _ := New("你😀 one\r\nlast one\r\n")
	b.ReplaceSelection("prefix ")
	b.SetSelection(Selection{Position{1, 4}, Position{1, 8}})
	before := b.Snapshot()
	edits := []Edit{{Range{Position{0, 11}, Position{0, 14}}, "界"}, {Range{Position{1, 5}, Position{1, 8}}, "😀\nnew"}}
	p, err := before.Prepare(context.Background(), edits, nil)
	if err != nil {
		t.Fatal(err)
	}
	if b.Text() != before.Text() || b.Version() != before.Version || !b.CanCommit(p) {
		t.Fatal("preparation changed its live buffer")
	}
	oracle, _ := New(before.Text())
	oracle.SetSelection(before.Selection)
	want, err := oracle.Apply(edits, nil)
	if err != nil {
		t.Fatal(err)
	}
	event, err := b.CommitPrepared(p)
	if err != nil || event.Version != before.Version+1 || b.Text() != oracle.Text() || b.Selection() != oracle.Selection() || !reflect.DeepEqual(event.Changes, want.Changes) {
		t.Fatalf("prepared commit differs from direct UTF-16/CRLF oracle: %v %+v", err, event)
	}
	if before.Text() != "prefix 你😀 one\r\nlast one\r\n" || p.Snapshot().Text() != b.Text() || b.CanCommit(p) {
		t.Fatal("snapshot changed or prepared edit is reusable")
	}
	b.MarkSaved()
	if _, ok := b.Undo(); !ok || b.Text() != before.Text() || !b.Dirty() || b.Selection() != before.Selection {
		t.Fatal("prepared undo lost the old unsaved transaction/selection")
	}
	if _, ok := b.Undo(); !ok || b.Text() != "你😀 one\r\nlast one\r\n" || !b.Dirty() {
		t.Fatal("older undo history was lost")
	}
	b.Redo()
	b.Redo()
	if b.Dirty() || b.Text() != p.Snapshot().Text() {
		t.Fatal("redo did not return to the saved prepared revision")
	}
}

func TestPreparedRejectsVersionIdentitySelectionAndInvalid(t *testing.T) {
	b, _ := New("😀 value")
	edit := []Edit{{Range{Position{0, 3}, Position{0, 8}}, "next"}}
	p, err := b.Snapshot().Prepare(context.Background(), edit, nil)
	if err != nil {
		t.Fatal(err)
	}
	other, _ := New(b.Text())
	for _, target := range []*Buffer{other, new(Buffer)} {
		if _, err := target.CommitPrepared(p); !errors.Is(err, ErrPreparedStale) {
			t.Fatal("prepared edit accepted another same-version document", err)
		}
	}
	if b.CanCommit(nil) || b.CanCommit(new(PreparedEdit)) {
		t.Fatal("empty plan accepted")
	}
	b.SetSelection(Selection{Position{0, 2}, Position{0, 2}})
	if _, err := b.CommitPrepared(p); !errors.Is(err, ErrPreparedStale) || b.Text() != "😀 value" {
		t.Fatal("old selection plan moved the newer caret")
	}
	b.SetSelection(Selection{})
	b.ReplaceSelection("new ")
	if _, err := b.CommitPrepared(p); !errors.Is(err, ErrPreparedStale) || b.Text() != "new 😀 value" {
		t.Fatal("stale version changed newer edits")
	}
	if _, err := (Snapshot{}).Prepare(context.Background(), nil, nil); err == nil {
		t.Fatal("unowned snapshot accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := b.Snapshot().Prepare(ctx, edit, nil); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled prepare accepted", err)
	}
	invalid := [][]Edit{{{Range{Position{0, 5}, Position{0, 6}}, "split"}}, {{Range{Position{}, Position{0, 2}}, "a"}, {Range{Position{0, 1}, Position{0, 3}}, "b"}}, {{Range{}, "\x00"}}, make([]Edit, MaxPreparedEdits+1)}
	for _, edits := range invalid {
		if _, err := b.Snapshot().Prepare(context.Background(), edits, nil); err == nil {
			t.Fatal("invalid prepared transaction accepted")
		}
	}
	noop, err := b.Snapshot().Prepare(context.Background(), nil, nil)
	version := b.Version()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.CommitPrepared(noop); err != nil || b.Version() != version {
		t.Fatal("empty transaction advanced history", err)
	}
}

func TestPreparedSnapshotWorkerRacesAndBoundedCommit(t *testing.T) {
	b, _ := New(strings.Repeat("x", 8<<20))
	snapshot := b.Snapshot()
	result := make(chan *PreparedEdit, 1)
	failure := make(chan error, 1)
	go func() {
		p, err := snapshot.Prepare(context.Background(), []Edit{{Range{Position{0, (8 << 20) - 6}, Position{0, 8 << 20}}, "needle"}}, nil)
		failure <- err
		result <- p
	}()
	for range 20 {
		b.ReplaceSelection("y")
	}
	p := <-result
	if err := <-failure; err != nil || b.CanCommit(p) || snapshot.Line(0) != strings.Repeat("x", 8<<20) || !strings.HasSuffix(p.Snapshot().Line(0), "needle") {
		t.Fatal("worker observed live mutable text or accepted stale commit", err)
	}
	// Prepare the actual current 8 MiB source; only commit is allocation measured.
	p, err := b.Snapshot().Prepare(context.Background(), []Edit{{Range{Position{0, 100}, Position{0, 101}}, "z"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	_, err = b.CommitPrepared(p)
	runtime.ReadMemStats(&after)
	if err != nil || after.TotalAlloc-before.TotalAlloc > 32<<10 {
		t.Fatalf("commit copied/scanned whole 8 MiB text: new Go allocation=%d, error=%v", after.TotalAlloc-before.TotalAlloc, err)
	}
	t.Logf("actual 8 MiB prepared commit new Go allocation=%d; worker racing newer source rejected", after.TotalAlloc-before.TotalAlloc)
}
