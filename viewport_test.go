package godesktop

import (
	"math"
	"testing"

	"github.com/neko233-com/godesktop/internal/platform"
)

func TestViewportClipsPaintHitTargetsAndNoninteractiveGeometry(t *testing.T) {
	f := testFrame()
	content := Row(
		Button("first", func(*Context) {}).Key("first").Width(60),
		Button("second", func(*Context) {}).Key("second").Width(60),
		Button("third", func(*Context) {}).Key("third").Width(60),
	).Height(40)
	f.layout(Viewport(content).Key("viewport").Padding(5).ScrollOffset(75, 0), rect{10, 20, 100, 50}, rect{w: 300, h: 300}, "root")
	if got := f.boundKeys["viewport"]; got != (rect{10, 20, 100, 50}) {
		t.Fatal("noninteractive viewport geometry", got)
	}
	if _, ok := f.boundKeys["first"]; ok || len(f.targets) != 2 {
		t.Fatal("offscreen first tab retained a hit/focus target", f.targets)
	}
	if f.targets[0].bounds != (rect{15, 25, 45, 40}) || f.targets[1].bounds != (rect{60, 25, 45, 40}) {
		t.Fatal("translated/clipped targets", f.targets)
	}
	for _, c := range f.commands {
		if c.Clip.X < 15 || c.Clip.X+c.Clip.W > 105 || c.Clip.Y < 25 || c.Clip.Y+c.Clip.H > 65 {
			t.Fatal("content paint escaped padded viewport", c)
		}
	}
	// A clipped hit target cannot accept a click outside the viewport even when
	// the unscrolled logical control still extends there.
	if f.targets[0].bounds.contains(14, 30) || f.targets[1].bounds.contains(106, 30) {
		t.Fatal("offscreen hit escaped viewport")
	}
}

func TestViewportClampsBothAxesAndMaintainsIntrinsicContent(t *testing.T) {
	for _, position := range [][2]float32{{1000, 1000}, {-1, -1}, {float32(math.NaN()), float32(math.Inf(1))}} {
		f := testFrame()
		e := Viewport(Column().Width(180).Height(90).Background(RGB(0xff0000))).ScrollOffset(position[0], position[1])
		f.layout(e, rect{w: 100, h: 50}, rect{w: 100, h: 50}, "root")
		want := platform.Rect{W: 180, H: 90}
		if position[0] == 1000 {
			want.X, want.Y = -80, -40
		}
		if len(f.commands) != 1 || f.commands[0].Bounds != want || f.commands[0].Clip != (platform.Rect{W: 100, H: 50}) {
			t.Fatal("clamp/content extent", position, f.commands)
		}
	}
	f := testFrame()
	f.layout(Viewport(Column().Width(20).Height(20).Background(RGB(1))).ScrollOffset(50, 50), rect{w: 100, h: 50}, rect{w: 100, h: 50}, "root")
	if f.commands[0].Bounds != (platform.Rect{W: 100, H: 50}) {
		t.Fatal("short content did not fill viewport")
	}
	f = testFrame()
	f.layout(Viewport(nil), rect{w: 100, h: 50}, rect{w: 100, h: 50}, "root")
	if len(f.commands) != 0 {
		t.Fatal("empty viewport painted content")
	}
}

func TestNamedPassiveBoundsDoNotEnterFocusAndClearAcrossFrames(t *testing.T) {
	a := application{context: &Context{wake: func() {}}, frame: testFrame()}
	visible := true
	a.view = func(*Context) *Element {
		if visible {
			return Column(Text("caption").Key("caption"), Button("click", func(*Context) {}).Key("click"))
		}
		return Column()
	}
	a.handle(platform.Event{Kind: platform.Draw, X: 100, Y: 100})
	if _, ok := a.context.ElementBounds("caption"); !ok || len(a.frame.targets) != 1 {
		t.Fatal("passive key was not recorded independently from focus")
	}
	visible = false
	a.handle(platform.Event{Kind: platform.Draw, X: 100, Y: 100})
	if _, ok := a.context.ElementBounds("caption"); ok {
		t.Fatal("stale keyed geometry survived a new frame")
	}
}

func TestDuplicatePassiveKeyRejected(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("duplicate geometry key accepted")
		}
	}()
	f := testFrame()
	f.layout(Column(Text("one").Key("same"), Text("two").Key("same")), rect{w: 200, h: 100}, rect{w: 200, h: 100}, "root")
}
