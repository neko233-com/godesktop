package godesktop

import (
	"fmt"
	"math"
	"slices"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/neko233-com/godesktop/internal/platform"
)

func TestFrameRebuildDrainsStateAndRemovesOldTargets(t *testing.T) {
	state, builds := 0, 0
	cx := &Context{wake: func() {}}
	a := application{context: cx, frame: testFrame(), view: func(*Context) *Element {
		builds++
		if state == 0 {
			return Button("old", func(*Context) {}).Key("old")
		}
		return Text("new")
	}}
	a.handle(platform.Event{Kind: platform.Draw, X: 100, Y: 80})
	if len(a.frame.targets) != 1 {
		t.Fatal("initial button missing")
	}
	cx.Dispatch(func() { state = 1 })
	a.handle(platform.Event{Kind: platform.Draw, X: 40, Y: 20})
	if state != 1 || builds != 2 || len(a.frame.targets) != 0 {
		t.Fatalf("state=%d builds=%d targets=%v", state, builds, a.frame.targets)
	}
	if len(a.frame.commands) != 1 || a.frame.commands[0].Text != "new" {
		t.Fatalf("stale display list: %+v", a.frame.commands)
	}
	if clip := a.frame.commands[0].Clip; clip.W > 40 || clip.H > 20 {
		t.Fatalf("stale viewport: %+v", clip)
	}
}

func TestKeyboardNavigationWrapsAndIgnoresRemovedFocus(t *testing.T) {
	clicks := []string{}
	a := application{context: &Context{wake: func() {}}, frame: testFrame()}
	a.frame.targets = []target{{key: "a", click: func(*Context) { clicks = append(clicks, "a") }}, {key: "b", click: func(*Context) { clicks = append(clicks, "b") }}}
	for _, key := range []int{platform.Tab, platform.Space, platform.Tab, platform.Enter, platform.Tab, platform.Enter} {
		a.handle(platform.Event{Kind: platform.KeyDown, Key: key})
	}
	if !slices.Equal(clicks, []string{"a", "b", "a"}) {
		t.Fatalf("activation order: %v", clicks)
	}
	a.focused = "removed"
	a.handle(platform.Event{Kind: platform.KeyDown, Key: platform.Enter})
	if len(clicks) != 3 {
		t.Fatal("removed focus was activated")
	}
	a.handle(platform.Event{Kind: platform.KeyDown, Key: platform.Tab, Modifiers: platform.Shift})
	if a.focused != "b" {
		t.Fatalf("reverse navigation: %q", a.focused)
	}
	a.frame.targets = nil
	a.handle(platform.Event{Kind: platform.KeyDown, Key: platform.Tab})
	a.handle(platform.Event{Kind: platform.KeyDown, Key: platform.Space})
}

func TestPointerRejectsOutsideReleaseAndRemovedButton(t *testing.T) {
	clicks := 0
	a := application{context: &Context{wake: func() {}}, frame: testFrame()}
	a.frame.targets = []target{{"a", rect{10, 10, 30, 30}, func(*Context) { clicks++ }}}
	a.handle(platform.Event{Kind: platform.PointerUp, X: 20, Y: 20})
	a.handle(platform.Event{Kind: platform.PointerDown, X: 20, Y: 20})
	a.handle(platform.Event{Kind: platform.PointerUp, X: -5, Y: -5})
	a.handle(platform.Event{Kind: platform.PointerDown, X: 20, Y: 20})
	a.frame.targets = []target{{"replacement", rect{10, 10, 30, 30}, func(*Context) { clicks++ }}}
	a.handle(platform.Event{Kind: platform.PointerUp, X: 20, Y: 20})
	if clicks != 0 || a.pressed != "" {
		t.Fatalf("clicks=%d capture=%q", clicks, a.pressed)
	}
}

func TestDisabledButtonAndFocusIndicator(t *testing.T) {
	f := testFrame()
	f.focus = "active"
	viewport := rect{0, 0, 300, 100}
	f.layout(Row(Button("enabled", func(*Context) {}).Key("active"), Button("disabled", nil)).Gap(10), viewport, viewport, "root")
	if len(f.targets) != 1 || f.targets[0].key != "active" {
		t.Fatalf("targets: %+v", f.targets)
	}
	focusLines, disabledLabel := 0, false
	for _, cmd := range f.commands {
		if cmd.Kind == platform.Rectangle && cmd.Color == nativeColor(RGB(0x93c5fd)) {
			focusLines++
		}
		if cmd.Text == "disabled" {
			disabledLabel = true
			if cmd.Color.A != 0.45 {
				t.Fatalf("disabled alpha: %g", cmd.Color.A)
			}
		}
	}
	if focusLines != 2 || !disabledLabel {
		t.Fatalf("focus lines=%d disabled label=%v", focusLines, disabledLabel)
	}
}

func TestColumnGrowCrossAxisAndEmptyViewport(t *testing.T) {
	f := testFrame()
	viewport := rect{0, 0, 100, 220}
	f.layout(Column(Column().Height(40).Width(30).Background(RGB(1)), Column().Grow(1).Background(RGB(2)), nil, Column().Grow(3).Background(RGB(3))).Padding(10).Gap(10), viewport, viewport, "root")
	want := []platform.Rect{{X: 10, Y: 10, W: 30, H: 40}, {X: 10, Y: 60, W: 80, H: 35}, {X: 10, Y: 105, W: 80, H: 105}}
	if len(f.commands) != len(want) {
		t.Fatalf("commands: %+v", f.commands)
	}
	for i, cmd := range f.commands {
		if cmd.Bounds != want[i] {
			t.Errorf("child %d: %+v want %+v", i, cmd.Bounds, want[i])
		}
	}
	f.commands = nil
	f.layout(Text("hidden"), rect{}, rect{}, "root")
	f.layout(nil, viewport, viewport, "root")
	if len(f.commands) != 0 {
		t.Fatal("empty tree/viewport generated commands")
	}
}

func TestTextCacheIsBoundedAndKeyedBySize(t *testing.T) {
	f := testFrame()
	calls := 0
	f.measure = func(_ string, size float32) (float32, float32) { calls++; return size, size }
	f.textSize(Text("same"))
	f.textSize(Text("same"))
	f.textSize(Text("same").FontSize(24))
	if calls != 2 {
		t.Fatalf("cache did not distinguish font sizes: %d", calls)
	}
	for i := 0; i < 3000; i++ {
		f.textSize(Text(fmt.Sprint(i)))
		if len(f.textCache) > 1024 {
			t.Fatalf("unbounded text cache: %d", len(f.textCache))
		}
	}
}

func TestIntrinsicLayoutsAndExplicitSizes(t *testing.T) {
	f := testFrame()
	if got := f.size(Row(Text("ab"), nil, Text("c")).Padding(2).Gap(3)); got != (dimensions{31, 20}) {
		t.Fatalf("row intrinsic size: %+v", got)
	}
	if got := f.size(Column(Text("ab"), Text("c")).Padding(2).Gap(3)); got != (dimensions{20, 39}) {
		t.Fatalf("column intrinsic size: %+v", got)
	}
	if got := f.size(Row(Text("ab")).Width(100).Height(80)); got != (dimensions{100, 80}) {
		t.Fatalf("explicit size: %+v", got)
	}
	if got := f.size(nil); got != (dimensions{}) {
		t.Fatalf("nil size: %+v", got)
	}
	viewport := rect{0, 0, 30, 20}
	f.layout(Text("a").Radius(100).Background(RGB(0xabcdef)).Foreground(RGB(0x123456)), viewport, viewport, "root")
	if f.commands[0].Radius != 10 || f.commands[1].Color != nativeColor(RGB(0x123456)) {
		t.Fatalf("radius/foreground: %+v", f.commands)
	}
}

func TestDispatchFIFOAndShutdownRace(t *testing.T) {
	cx := &Context{wake: func() {}}
	var order []int
	for i := 0; i < 100; i++ {
		cx.Dispatch(func() { order = append(order, i) })
	}
	cx.drain()
	for i, v := range order {
		if i != v {
			t.Fatalf("FIFO: %v", order)
		}
	}
	if cx.Dispatch(nil) {
		t.Fatal("nil dispatch accepted")
	}
	var accepted atomic.Int32
	var group sync.WaitGroup
	for i := 0; i < 16; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			for j := 0; j < 100; j++ {
				if cx.Dispatch(func() {}) {
					accepted.Add(1)
				}
				cx.Invalidate()
			}
		}()
	}
	cx.mu.Lock()
	cx.closed = true
	cx.pending = nil
	cx.mu.Unlock()
	group.Wait()
	if cx.Dispatch(func() {}) {
		t.Fatal("closed context accepted state")
	}
	if len(cx.pending) != 0 {
		t.Fatalf("callbacks survived shutdown: %d", len(cx.pending))
	}
}

func TestWindowValidationCoversBothDimensions(t *testing.T) {
	for _, value := range []float32{-1, float32(math.NaN()), float32(math.Inf(-1)), 16385} {
		if err := Run(WindowOptions{Height: value}, func(*Context) *Element { return nil }); err == nil {
			t.Fatalf("accepted height %v", value)
		}
	}
	running.Store(true)
	defer running.Store(false)
	if err := Run(WindowOptions{}, func(*Context) *Element { return nil }); err == nil {
		t.Fatal("accepted a second window")
	}
}

func FuzzLayoutClipsToViewport(f *testing.F) {
	f.Add([]byte{0, 10, 20, 30, 40, 50})
	f.Add([]byte{1, 255, 0, 64, 17, 9, 10, 11})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			return
		}
		if len(data) > 128 {
			data = data[:128]
		}
		children := make([]*Element, 0, len(data))
		for i, b := range data {
			children = append(children, Button(string([]byte{b}), func(*Context) {}).Key(fmt.Sprint(i)).Width(float32(b%80)).Height(float32(b%60)).Grow(float32(b%4)).Padding(float32(b%12)))
		}
		root := Column(children...)
		if data[0]&1 != 0 {
			root = Row(children...)
		}
		root.Gap(float32(data[0] % 12)).Padding(float32(data[0] % 20))
		if data[0]&2 != 0 {
			root = Viewport(root).ScrollOffset(float32(data[0]), float32(data[len(data)-1]))
		}
		viewport := rect{0, 0, 200, 150}
		frame := testFrame()
		frame.layout(root, viewport, viewport, "root")
		for _, cmd := range frame.commands {
			c := cmd.Clip
			if c.X < 0 || c.Y < 0 || c.W < 0 || c.H < 0 || c.X+c.W > 200.001 || c.Y+c.H > 150.001 {
				t.Fatalf("clip outside viewport: %+v", c)
			}
		}
		for _, target := range frame.targets {
			if target.bounds.w <= 0 || target.bounds.h <= 0 {
				t.Fatalf("empty click target: %+v", target)
			}
		}
	})
}
