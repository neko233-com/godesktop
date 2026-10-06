package editor

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func utf16ByteOffset(s string, offset int) int {
	n := 0
	for i, r := range s {
		if n == offset {
			return i
		}
		if r > 0xffff {
			n += 2
		} else {
			n++
		}
	}
	return len(s)
}
func FuzzBufferTransactions(f *testing.F) {
	f.Add("你😀\r\nsecond\r\n", "界\n", []byte{1, 3, 5, 0, 255, 4})
	f.Add("hello\nworld", "😀", []byte{0, 10, 2, 6})
	f.Fuzz(func(t *testing.T, initial, insert string, steps []byte) {
		if len(initial) > 4096 || len(insert) > 256 || !utf8.ValidString(initial) || !utf8.ValidString(insert) || strings.ContainsRune(initial, 0) || strings.ContainsRune(insert, 0) {
			return
		}
		b, err := New(initial)
		if err != nil {
			t.Fatal(err)
		}
		insert = strings.ReplaceAll(normalize(insert), "\n", b.EOL())
		for i := 0; i+1 < len(steps) && i < 32; i += 2 {
			before := b.Text()
			saved := b.Snapshot()
			b.MarkSaved()
			length := units(before) + 1
			a, z := b.PositionAt(int(steps[i])%length), b.PositionAt(int(steps[i+1])%length)
			if less(z, a) {
				a, z = z, a
			}
			aOffset, _ := b.OffsetAt(a)
			zOffset, _ := b.OffsetAt(z)
			want := before[:utf16ByteOffset(before, aOffset)] + insert + before[utf16ByteOffset(before, zOffset):]
			if _, err := b.Apply([]Edit{{Range: Range{a, z}, Text: insert}}, nil); err != nil {
				t.Fatal(err)
			}
			if b.Text() != want || saved.Text() != before {
				t.Fatalf("transaction %q != %q (snapshot %q)", b.Text(), want, saved.Text())
			}
			if _, ok := b.Undo(); !ok || b.Text() != before || b.Dirty() {
				t.Fatalf("undo %q != %q", b.Text(), before)
			}
			if _, ok := b.Redo(); !ok || b.Text() != want {
				t.Fatal("redo did not restore transaction")
			}
		}
	})
}
