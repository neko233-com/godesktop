package editor

import (
	"strings"
	"testing"
)

func TestPositionFromRunesDoesNotAllocateWholeLongLine(t *testing.T) {
	b, err := New(strings.Repeat("x", 8<<20))
	if err != nil {
		t.Fatal(err)
	}
	if allocations := testing.AllocsPerRun(5, func() {
		if p := b.PositionFromRunes(0, 2); p != (Position{0, 2}) {
			panic("incorrect position")
		}
	}); allocations != 0 {
		t.Fatalf("short cursor query copies a complete 8 MiB line: %v allocations", allocations)
	}
	unicodeBuffer, _ := New("😀界\r\nend")
	for _, test := range []struct {
		line, column int
		want         Position
	}{{-1, -1, Position{}}, {0, 1, Position{0, 2}}, {0, 2, Position{0, 3}}, {0, 100, Position{0, 3}}, {100, 100, Position{1, 3}}} {
		if got := unicodeBuffer.PositionFromRunes(test.line, test.column); got != test.want {
			t.Fatal(test, got)
		}
	}
}
