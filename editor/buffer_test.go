package editor

import (
	"reflect"
	"testing"
)

func TestUTF16PositionsAndCRLFRoundTrip(t *testing.T) {
	b, err := New("你😀x\r\nÁ\r\n")
	if err != nil {
		t.Fatal(err)
	}
	if b.Text() != "你😀x\r\nÁ\r\n" || b.EOL() != "\r\n" {
		t.Fatal("EOL was lost")
	}
	if p := b.PositionFromRunes(0, 2); p != (Position{0, 3}) {
		t.Fatalf("emoji UTF-16 position: %+v", p)
	}
	if n, err := b.OffsetAt(Position{1, 1}); err != nil || n != 7 {
		t.Fatalf("CRLF offset %d: %v", n, err)
	}
	if p := b.PositionAt(7); p != (Position{1, 1}) {
		t.Fatalf("position %+v", p)
	}
	before := b.Snapshot()
	if _, err = b.Apply([]Edit{{Range{Position{0, 2}, Position{0, 3}}, "bad"}}, nil); err == nil {
		t.Fatal("split surrogate accepted")
	}
	if b.Version() != 1 || b.Text() != before.Text() {
		t.Fatal("invalid edit mutated document")
	}
}
func TestTransactionsUndoSavedRevisionAndSnapshot(t *testing.T) {
	b, _ := New("one\ntwo\nthree")
	b.SetSelection(Selection{Position{1, 0}, Position{1, 3}})
	saved := b.Snapshot()
	event, err := b.ReplaceSelection("你😀\nnew")
	if err != nil || event.Version != 2 || b.Text() != "one\n你😀\nnew\nthree" {
		t.Fatalf("edit: %v %+v %q", err, event, b.Text())
	}
	if b.Selection().Active != (Position{2, 3}) || !b.Dirty() {
		t.Fatal("caret/dirty lost")
	}
	b.MarkSaved()
	if _, ok := b.Undo(); !ok || b.Text() != saved.Text() || !b.Dirty() || b.Version() != 3 {
		t.Fatal("undo save state")
	}
	if saved.Text() != "one\ntwo\nthree" {
		t.Fatal("worker snapshot mutated")
	}
	if _, ok := b.Redo(); !ok || b.Dirty() || b.Version() != 4 {
		t.Fatal("redo save state")
	}
	b.Undo()
	b.ReplaceSelection("branch")
	if _, ok := b.Redo(); ok {
		t.Fatal("redo survived a new edit")
	}
}
func TestAtomicBatchAndIncrementalChangeOrdering(t *testing.T) {
	b, _ := New("abc\ndef\nghi")
	before := b.Snapshot()
	_, err := b.Apply([]Edit{{Range{Position{0, 0}, Position{0, 2}}, "x"}, {Range{Position{0, 1}, Position{0, 3}}, "y"}}, nil)
	if err == nil || b.Text() != before.Text() || b.Version() != 1 {
		t.Fatal("overlapping edit partially applied")
	}
	event, err := b.Apply([]Edit{{Range{Position{0, 1}, Position{0, 2}}, "😀"}, {Range{Position{2, 0}, Position{2, 3}}, "last"}}, nil)
	if err != nil || b.Text() != "a😀c\ndef\nlast" {
		t.Fatalf("batch %q: %v", b.Text(), err)
	}
	if !reflect.DeepEqual(event.Changes[0].Range, Range{Position{2, 0}, Position{2, 3}}) {
		t.Fatal("LSP changes not descending")
	}
	if _, ok := b.Undo(); !ok || b.Text() != before.Text() {
		t.Fatalf("batch undo %q", b.Text())
	}
	if _, ok := b.Redo(); !ok || b.Text() != "a😀c\ndef\nlast" {
		t.Fatalf("batch redo %q", b.Text())
	}
}
