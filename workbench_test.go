package godesktop

import (
	"github.com/neko233-com/godesktop/internal/platform"
	"testing"
)

func TestWorkspaceFlexAndFonts(t *testing.T) {
	customCalls := 0
	f := frame{measure: func(s string, size float32) (float32, float32) { return float32(len(s)) * 8, size }, fontMeasure: func(s string, size float32, font string) (float32, float32) {
		customCalls++
		return float32(len(s)) * 10, size
	}, textCache: map[textKey]dimensions{}, measured: map[*Element]dimensions{}, keys: map[string]bool{}}
	view := Column(Text("title").Height(36).Draggable(), Column(Text("code").FontFamily("Consolas"), Text("code").FontFamily("Menlo"), Text("code").FontFamily("Consolas")).Height(2000).Flex(1), Icon("close").Height(22).Key("close").OnClick(func(*Context) {}))
	f.layout(view, rect{w: 800, h: 600}, rect{w: 800, h: 600}, "root")
	if customCalls != 2 {
		t.Fatalf("custom font cache calls=%d", customCalls)
	}
	if len(f.targets) != 1 || f.targets[0].bounds.y != 578 {
		t.Fatalf("footer was pushed out of viewport: %v", f.targets)
	}
	lines, drag := 0, 0
	for _, c := range f.commands {
		if c.Kind == platform.Line {
			lines++
			if c.Radius <= 0 {
				t.Fatal("zero stroke")
			}
		}
		if c.Kind == platform.DragRegion {
			drag++
			if c.Bounds.H != 36 {
				t.Fatal("drag region exceeds title")
			}
		}
	}
	if lines != 2 || drag != 1 {
		t.Fatalf("icon=%d drag=%d", lines, drag)
	}
	if f.size(Icon("unknown")).w != 24 {
		t.Fatal("icon intrinsic size")
	}
}
func TestWindowActionsAfterShutdown(t *testing.T) {
	var calls []int
	cx := &Context{windowAction: func(action int) { calls = append(calls, action) }}
	cx.Minimize()
	cx.ToggleMaximize()
	cx.closed = true
	cx.Minimize()
	cx.ToggleMaximize()
	if len(calls) != 2 || calls[0] != 1 || calls[1] != 2 {
		t.Fatalf("actions %v", calls)
	}
	(&Context{}).Minimize()
}

func TestInputConsumptionAndPaddingXY(t *testing.T) {
	wakes, inputs, clicks := 0, 0, 0
	cx := &Context{wake: func() { wakes++ }}
	a := application{context: cx, frame: testFrame(), input: func(_ *Context, e InputEvent) bool { inputs++; return e.Kind == Character }, view: func(*Context) *Element { return Text("title").PaddingXY(10, 0).Height(24) }}
	a.handle(platform.Event{Kind: platform.Draw, X: 100, Y: 80})
	if w, h := cx.WindowSize(); w != 100 || h != 80 {
		t.Fatalf("viewport %g,%g", w, h)
	}
	a.handle(platform.Event{Kind: int(Character), Key: '你'})
	if inputs != 1 || wakes != 1 {
		t.Fatal("input was not consumed")
	}
	a.focused = "button"
	a.frame.targets = []target{{key: "button", click: func(*Context) { clicks++ }}}
	a.handle(platform.Event{Kind: platform.KeyDown, Key: platform.Enter})
	if clicks != 1 || inputs != 2 {
		t.Fatal("unconsumed event lost button behavior")
	}
	f := testFrame()
	e := Text("a").PaddingXY(10, 0)
	if d := f.size(e); d.w != 28 || d.h != 16 {
		t.Fatalf("independent padding %v", d)
	}
	e.Padding(3)
	if x, y := e.insets(); x != 3 || y != 3 {
		t.Fatal("uniform padding did not reset independent padding")
	}
	_ = cx.RenderedFrames()
}
