package godesktop

import (
	"math"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/neko233-com/godesktop/internal/platform"
)

func TestPointerCaptureAndKeyboardFocus(t *testing.T) {
	cx := &Context{wake: func() {}}
	clicks := 0
	a := application{context: cx, frame: testFrame()}
	a.frame.targets = []target{{"a", rect{0, 0, 40, 40}, func(*Context) { clicks++ }}, {"b", rect{50, 0, 40, 40}, func(*Context) { clicks += 10 }}}
	a.handle(platform.Event{Kind: platform.PointerDown, X: 5, Y: 5})
	a.handle(platform.Event{Kind: platform.PointerUp, X: 55, Y: 5})
	if clicks != 0 {
		t.Fatal("release over another button activated it")
	}
	a.handle(platform.Event{Kind: platform.PointerDown, X: 5, Y: 5})
	a.handle(platform.Event{Kind: platform.Cancel})
	a.handle(platform.Event{Kind: platform.PointerUp, X: 5, Y: 5})
	if clicks != 0 {
		t.Fatal("cancelled capture activated a button")
	}
	a.handle(platform.Event{Kind: platform.PointerDown, X: 5, Y: 5})
	a.handle(platform.Event{Kind: platform.PointerUp, X: 5, Y: 5})
	a.handle(platform.Event{Kind: platform.KeyDown, Key: platform.Tab})
	a.handle(platform.Event{Kind: platform.KeyDown, Key: platform.Enter})
	a.handle(platform.Event{Kind: platform.KeyDown, Key: platform.Tab, Modifiers: platform.Shift})
	a.handle(platform.Event{Kind: platform.KeyDown, Key: platform.Space})
	if clicks != 12 {
		t.Fatalf("clicks: %d", clicks)
	}
}

func TestCloseRequestAndExplicitQuitRemainSeparate(t *testing.T) {
	actions, quits := 0, 0
	cx := &Context{windowAction: func(action int) {
		if action != 3 {
			t.Fatal(action)
		}
		actions++
	}, quit: func() { quits++ }}
	cx.RequestClose()
	if actions != 1 || quits != 0 {
		t.Fatal("request bypassed application close guard")
	}
	cx.Quit()
	if actions != 1 || quits != 1 {
		t.Fatal("explicit shutdown did not use force path")
	}
	cx.closed = true
	cx.RequestClose()
	cx.Quit()
	if actions != 1 || quits != 1 {
		t.Fatal("closed context affected another run")
	}
}

func TestKeyRepeatAndElementBounds(t *testing.T) {
	cx := &Context{wake: func() {}}
	var input InputEvent
	a := application{context: cx, frame: testFrame(), input: func(_ *Context, e InputEvent) bool { input = e; return false }}
	a.view = func(*Context) *Element { return Button("action", func(*Context) {}).Key("button") }
	a.handle(platform.Event{Kind: platform.Draw, X: 120, Y: 40})
	if b, ok := cx.ElementBounds("button"); !ok || b != (Bounds{0, 0, 120, 40}) {
		t.Fatalf("interactive bounds %+v %v", b, ok)
	}
	a.handle(platform.Event{Kind: platform.KeyDown, Key: 37, Modifiers: platform.Shift | 16})
	if !input.Repeat || input.Modifiers != ModifierShift {
		t.Fatalf("repeat encoding leaked into modifiers: %+v", input)
	}
	clicks := 0
	a.frame.targets = []target{{"button", rect{0, 0, 120, 40}, func(*Context) { clicks++ }}}
	a.focused = "button"
	a.handle(platform.Event{Kind: platform.KeyDown, Key: platform.Enter, Modifiers: 16})
	if clicks != 0 {
		t.Fatal("held Enter repeatedly activated generic button")
	}
	if _, ok := cx.ElementBounds("missing"); ok {
		t.Fatal("missing element reported bounds")
	}
}

func TestDispatchIsConcurrentAndReentrant(t *testing.T) {
	var wakes atomic.Int32
	cx := &Context{wake: func() { wakes.Add(1) }}
	count := 0
	var group sync.WaitGroup
	for i := 0; i < 100; i++ {
		group.Add(1)
		go func() { defer group.Done(); cx.Dispatch(func() { count++ }) }()
	}
	group.Wait()
	cx.Dispatch(func() { cx.Dispatch(func() { count += 10 }) })
	cx.drain()
	cx.drain()
	cx.drain()
	if count != 110 || wakes.Load() != 3 {
		t.Fatalf("count=%d wakes=%d", count, wakes.Load())
	}
	cx.mu.Lock()
	cx.closed = true
	cx.mu.Unlock()
	if cx.Dispatch(func() { t.Fatal("executed after shutdown") }) {
		t.Fatal("accepted callback after shutdown")
	}
	cx.Invalidate()
	if wakes.Load() != 3 {
		t.Fatal("woke a closed context")
	}
}

func TestInvalidWindowOptions(t *testing.T) {
	view := func(*Context) *Element { return nil }
	for _, width := range []float32{-1, float32(math.NaN()), float32(math.Inf(1)), 16385} {
		if err := Run(WindowOptions{Width: width}, view); err == nil {
			t.Fatalf("accepted width %v", width)
		}
	}
	if err := Run(WindowOptions{}, nil); err == nil {
		t.Fatal("accepted nil view")
	}
	if err := Run(WindowOptions{Title: "a\x00b"}, view); err == nil {
		t.Fatal("accepted NUL title")
	}
}
