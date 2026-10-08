package godesktop

import (
	"math"
	"testing"

	"github.com/neko233-com/godesktop/internal/platform"
)

func TestShadowPaintOrderAncestorClipsAndHitGeometry(t *testing.T) {
	f := testFrame()
	style := ShadowStyle{Color: RGBA(0x123456, .5), OffsetX: 6, OffsetY: -3, Blur: 12, Spread: 2}
	surface := Column(Column().Flex(1).Background(RGB(0x0000ff))).Width(100).Height(100).Background(RGB(0xff0000)).ClipRounded(20).Shadow(style).Key("surface").OnClick(func(*Context) {})
	sibling := Column().Width(100).Height(100).Background(RGB(0x00ff00)).Key("sibling").OnClick(func(*Context) {})
	root := Row(surface, sibling).ClipRounded(8)
	viewport := rect{0, 0, 200, 100}
	f.layout(root, viewport, viewport, "root")
	if len(f.commands) != 4 || f.commands[0].Kind != platform.Shadow || f.commands[1].Kind != platform.Rectangle {
		t.Fatalf("shadow was not one instance before the surface: %+v", f.commands)
	}
	shadow := f.commands[0]
	if shadow.Bounds != (platform.Rect{X: 4, Y: -5, W: 104, H: 104}) || shadow.Clip != nativeRect(viewport) || shadow.Radius != 22 || shadow.ShadowSigma != 6 || shadow.Color != nativeColor(style.Color) {
		t.Fatalf("shadow body/blur/spread/incoming clip: %+v", shadow)
	}
	if shadow.RoundedClips[0].Radius != 8 || shadow.RoundedClips[1].Radius != 0 || f.commands[2].RoundedClips[1].Radius != 20 || f.commands[3].RoundedClips[1].Radius != 0 {
		t.Fatalf("shadow included its own clip or leaked into a sibling: %+v", f.commands)
	}
	if f.boundKeys["surface"] != (rect{0, 0, 100, 100}) || len(f.targets) != 2 || f.targetContains(f.targets[0], 110, 50) || f.targetContains(f.targets[0], 1, 1) || !f.targetContains(f.targets[0], 25, 25) {
		t.Fatalf("shadow expanded bounds or rounded hit region: %+v %+v", f.boundKeys, f.targets)
	}
	if f.roundedDepth != 0 || f.roundedClips != ([platform.MaxRoundedClips]platform.RoundedClip{}) {
		t.Fatal("shadow changed the rounded stack lifetime")
	}
}

func TestShadowDoesNotChangeMeasurementAndFollowsClipRounded(t *testing.T) {
	f := testFrame()
	plain := Column(Text("title")).Width(80).Height(40).Padding(4)
	shadowed := Column(Text("title")).Width(80).Height(40).Padding(4).ClipRounded(8).Shadow(ShadowStyle{Color: RGBA(0, .4), Blur: 20, Spread: -3})
	if f.size(plain) != f.size(shadowed) {
		t.Fatal("shadow changed intrinsic layout")
	}
	f.layout(shadowed, rect{20, 20, 80, 40}, rect{0, 0, 200, 100}, "root")
	if f.commands[0].Radius != 5 || f.commands[0].Bounds != (platform.Rect{X: 23, Y: 23, W: 74, H: 34}) {
		t.Fatalf("ClipRounded/spread silhouette: %+v", f.commands[0])
	}
}

func TestShadowVisibleWhenBodyIsOutsideViewport(t *testing.T) {
	f := testFrame()
	e := Column().Shadow(ShadowStyle{Color: RGB(0), Blur: 12}).Key("outside").OnClick(func(*Context) {})
	f.layout(e, rect{105, 30, 20, 20}, rect{0, 0, 100, 100}, "root")
	if len(f.commands) != 1 || f.commands[0].Kind != platform.Shadow || len(f.targets) != 0 || len(f.boundKeys) != 0 {
		t.Fatalf("visible halo was culled with its offscreen body: commands=%+v targets=%+v", f.commands, f.targets)
	}
}

func TestShadowDisabledCollapsedAndOffscreen(t *testing.T) {
	for _, tc := range []struct {
		name  string
		style ShadowStyle
	}{
		{"transparent", ShadowStyle{}},
		{"collapsed", ShadowStyle{Color: RGB(0), Spread: -10}},
		{"outside", ShadowStyle{Color: RGB(0), OffsetX: 200, Blur: 10}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := testFrame()
			f.layout(Column().Shadow(tc.style), rect{20, 20, 20, 20}, rect{0, 0, 100, 100}, "root")
			if len(f.commands) != 0 {
				t.Fatalf("non-visible shadow emitted paint: %+v", f.commands)
			}
		})
	}
}

func TestShadowFiniteAndBoundedValueStyle(t *testing.T) {
	style := ShadowStyle{Color: Color{R: float32(math.NaN()), G: 3, B: -1, A: .5}, OffsetX: -5000, OffsetY: 5000, Blur: 1000, Spread: -1000}
	e := Column().Shadow(style)
	style.Color.A, style.Blur = 0, 0
	want := ShadowStyle{Color: Color{G: 1, A: .5}, OffsetX: -4096, OffsetY: 4096, Blur: 128, Spread: -128}
	if e.shadow != want {
		t.Fatalf("style was not a normalized bounded value: %+v", e.shadow)
	}
	for _, value := range []float32{float32(math.NaN()), float32(math.Inf(1)), float32(math.Inf(-1))} {
		e.Shadow(ShadowStyle{Color: RGB(0), OffsetX: value, OffsetY: value, Blur: value, Spread: value})
		if e.shadow.OffsetX != 0 || e.shadow.OffsetY != 0 || e.shadow.Blur != 0 || e.shadow.Spread != 0 {
			t.Fatalf("non-finite style escaped: %+v", e.shadow)
		}
	}
}

func TestShadowSharpAndRadiusClamp(t *testing.T) {
	f := testFrame()
	f.layout(Column().Radius(100).Shadow(ShadowStyle{Color: RGB(0), Blur: -1, Spread: 2}), rect{10, 10, 20, 10}, rect{0, 0, 100, 100}, "root")
	if len(f.commands) != 1 || f.commands[0].ShadowSigma != 0 || f.commands[0].Radius != 7 {
		t.Fatalf("sharp shadow radius/blur clamp: %+v", f.commands)
	}
}

func TestShadowDoesNotConsumeRoundedClipDepth(t *testing.T) {
	f := testFrame()
	e := Column().Width(100).Height(80).Background(RGB(0xff0000))
	for range platform.MaxRoundedClips {
		e = Column(e).ClipRounded(8).Shadow(ShadowStyle{Color: RGBA(0, .2), Blur: 4})
	}
	viewport := rect{0, 0, 100, 80}
	f.layout(e, viewport, viewport, "root")
	if len(f.commands) != platform.MaxRoundedClips+1 {
		t.Fatalf("shadow instance bound: %+v", f.commands)
	}
	for index := range platform.MaxRoundedClips {
		if f.commands[index].Kind != platform.Shadow || f.commands[index].RoundedClips[index].Radius != 0 {
			t.Fatalf("shadow consumed its own clip slot: %+v", f.commands[index])
		}
	}
}
