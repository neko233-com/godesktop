package godesktop

import (
	"testing"

	"github.com/neko233-com/godesktop/internal/platform"
)

func testFrame() frame {
	return frame{measure: func(text string, size float32) (float32, float32) { return float32(len([]rune(text))) * size / 2, size }, textCache: make(map[textKey]dimensions), measured: make(map[*Element]dimensions), keys: make(map[string]bool)}
}

func TestGrowAndGap(t *testing.T) {
	f := testFrame()
	tree := Row(Column().Width(100).Background(RGB(1)), Column().Grow(1).Background(RGB(2)), Column().Grow(2).Background(RGB(3))).Gap(10)
	viewport := rect{0, 0, 420, 80}
	f.layout(tree, viewport, viewport, "root")
	if len(f.commands) != 3 {
		t.Fatalf("commands: %d", len(f.commands))
	}
	want := []platform.Rect{{X: 0, Y: 0, W: 100, H: 80}, {X: 110, Y: 0, W: 100, H: 80}, {X: 220, Y: 0, W: 200, H: 80}}
	for i, command := range f.commands {
		if command.Bounds != want[i] {
			t.Errorf("child %d: got %+v, want %+v", i, command.Bounds, want[i])
		}
	}
}

func TestStackPaintsSelectionAndCaretWithoutMovingText(t *testing.T) {
	f := testFrame()
	tree := Stack(Column().Width(20).Height(16).Background(RGB(0x264f78)), Text("text"), Row(Column().Width(12), Column().Width(1).Height(16).Background(RGB(0xffffff)))).Gap(100)
	if d := f.size(tree); d != (dimensions{32, 16}) {
		t.Fatalf("overlay measured as flowing children: %+v", d)
	}
	viewport := rect{5, 6, 40, 16}
	f.layout(tree, viewport, viewport, "stack")
	if len(f.commands) != 3 || f.commands[0].Kind != platform.Rectangle || f.commands[1].Kind != platform.Label || f.commands[2].Bounds.X != 17 {
		t.Fatalf("overlay order/bounds %+v", f.commands)
	}
	if f.commands[1].Bounds.X != 5 || f.commands[1].Bounds.Y != 6 {
		t.Fatal("caret moved the shaped text")
	}
}

func TestOverflowClipAndHitArea(t *testing.T) {
	f := testFrame()
	viewport := rect{0, 0, 100, 40}
	f.layout(Column(Button("wide", func(*Context) {}).Key("button").Width(200).Height(100)).Padding(5), viewport, viewport, "root")
	if len(f.targets) != 1 || f.targets[0].bounds != (rect{5, 5, 90, 30}) {
		t.Fatalf("unexpected hit area: %+v", f.targets)
	}
	if f.targets[0].bounds.contains(99, 39) {
		t.Fatal("clipped overflow was clickable")
	}
	if f.targets[0].bounds.contains(95, 10) {
		t.Fatal("right edge must be excluded")
	}
}

func TestMeasurementCacheAndNilChildren(t *testing.T) {
	f := testFrame()
	calls := 0
	f.measure = func(string, float32) (float32, float32) { calls++; return 10, 20 }
	tree := Column(nil, Text("你好"), nil, Text("你好")).Gap(3).Padding(2)
	if d := f.size(tree); d != (dimensions{14, 47}) {
		t.Fatalf("size: %+v", d)
	}
	if calls != 1 {
		t.Fatalf("native measurement called %d times", calls)
	}
}

func TestDuplicateKeys(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("duplicate keys must fail explicitly")
		}
	}()
	f := testFrame()
	viewport := rect{0, 0, 100, 200}
	f.layout(Column(Button("a", func(*Context) {}).Key("same"), Button("b", func(*Context) {}).Key("same")), viewport, viewport, "root")
}

func TestTextOverflowDoesNotScaleGlyphs(t *testing.T) {
	f := testFrame()
	viewport := rect{0, 0, 30, 8}
	f.layout(Text("abcdefghij"), viewport, viewport, "root")
	if len(f.commands) != 1 {
		t.Fatalf("commands: %d", len(f.commands))
	}
	command := f.commands[0]
	if command.Bounds.W != 80 || command.Bounds.H != 16 {
		t.Fatalf("text was scaled: %+v", command.Bounds)
	}
	if command.Clip != nativeRect(viewport) {
		t.Fatalf("wrong clip: %+v", command.Clip)
	}
}

func BenchmarkLayout1000Elements(b *testing.B) {
	children := make([]*Element, 1000)
	for i := range children {
		children[i] = Row(Text("cached label"), Button("action", func(*Context) {}).Key(string(rune(i+32)))).Height(24).Gap(8)
	}
	tree := Column(children...)
	f := testFrame()
	viewport := rect{0, 0, 1000, 24000}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.commands = f.commands[:0]
		f.targets = f.targets[:0]
		clear(f.measured)
		clear(f.keys)
		f.layout(tree, viewport, viewport, "root")
	}
}
