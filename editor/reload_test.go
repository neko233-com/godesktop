package editor

import (
	"strings"
	"testing"
)

func TestReloadMonotonicSavedHistoryEOLAndSnapshots(t *testing.T) {
	b, _ := New("old\r\n你😀\r\n")
	b.SetSelection(Selection{Position{1, 3}, Position{2, 0}})
	b.ReplaceSelection("local")
	prior := b.Snapshot()
	version := b.Version()
	event, err := b.Reload("disk\n😀\n")
	if err != nil || b.Version() != version+1 || b.Dirty() || b.Text() != "disk\n😀\n" || b.EOL() != "\n" {
		t.Fatalf("reload: %v %+v %q", err, event, b.Text())
	}
	// Replay the returned UTF-16 change against the original wire snapshot. The
	// replacement contains its own new EOL, even when the old protocol used CRLF.
	if len(event.Changes) != 1 || event.Changes[0].RangeLength != units(prior.Text()) || event.Changes[0].Text != b.Text() {
		t.Fatal("wire replacement lost EOL or length")
	}
	if prior.Text() != "old\r\n你😀local" {
		t.Fatalf("prior worker snapshot changed: %q", prior.Text())
	}
	if b.Selection().Anchor != (Position{1, 2}) {
		t.Fatalf("surrogate clamp: %+v", b.Selection())
	}
	saved := b.Snapshot()
	if _, ok := b.Undo(); !ok || b.Text() != prior.Text() || b.EOL() != "\r\n" || !b.Dirty() {
		t.Fatalf("undo reload %q", b.Text())
	}
	if _, ok := b.Undo(); !ok || b.Text() != "old\r\n你😀\r\n" {
		t.Fatal("prior undo history lost")
	}
	b.Redo()
	b.Redo()
	if b.Text() != saved.Text() || b.Dirty() || saved.Text() != "disk\n😀\n" || b.Version() != version+5 {
		t.Fatal("redo/savepoint/snapshot/version")
	}
}

func TestReloadRejectsInvalidAndIdenticalReloadKeepsHistory(t *testing.T) {
	b, _ := New("a")
	b.ReplaceSelection("x")
	before := b.Snapshot()
	for _, text := range []string{"\x00", "\xff"} {
		if _, err := b.Reload(text); err == nil || b.Text() != before.Text() || b.Version() != before.Version || !b.Dirty() {
			t.Fatal("invalid reload mutated buffer")
		}
	}
	event, err := b.Reload(before.Text())
	if err != nil || len(event.Changes) != 0 || b.Version() != before.Version || b.Dirty() {
		t.Fatal("identical disk revision")
	}
	b.Undo()
	if !b.Dirty() {
		t.Fatal("identical reload forgot saved revision")
	}
}

func TestReloadHistoryBudgetAndRepeatedEOLTransitions(t *testing.T) {
	b, _ := New("initial\n")
	for i := 0; i < 40; i++ {
		eol := "\n"
		if i%2 == 0 {
			eol = "\r\n"
		}
		text := strings.Repeat("😀", i+1) + eol
		b.Reload(text)
		snap := b.Snapshot()
		b.ReplaceSelection("insert")
		b.Undo()
		if b.Text() != text || b.Dirty() || snap.Text() != text {
			t.Fatalf("EOL transition %d", i)
		}
	}
	b.Reload(strings.Repeat("x", MaxHistoryBytes+1))
	if _, ok := b.Undo(); ok || b.Dirty() {
		t.Fatal("oversized reload retained unbounded history")
	}
}
