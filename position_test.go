package godesktop

import (
	"math"
	"testing"

	"github.com/neko233-com/godesktop/internal/platform"
)

func TestPositionedStackIntrinsicPaintAndHitBounds(t *testing.T) {
	f := testFrame()
	base := Column().Width(100).Height(80)
	popup := Column(Text("popup")).Padding(2).Position(25, 10).Shadow(ShadowStyle{Color: RGBA(0, .2), Blur: 12}).Key("popup").OnClick(func(*Context) {})
	tree := Stack(base, popup).Padding(4)
	if f.size(tree) != (dimensions{108, 88}) {
		t.Fatalf("positioned child changed canvas measurement: %+v", f.size(tree))
	}
	f.layout(tree, rect{10, 20, 160, 120}, rect{0, 0, 200, 160}, "root")
	want := rect{39, 34, 44, 20}
	if f.boundKeys["popup"] != want || f.commands[0].Kind != platform.Shadow || f.commands[0].Clip != (platform.Rect{X: 14, Y: 24, W: 152, H: 112}) {
		t.Fatalf("positioned popup body/ancestor paint clip: %+v %+v", f.boundKeys, f.commands)
	}
	if f.targetContains(f.targets[0], 38, 35) || !f.targetContains(f.targets[0], 40, 35) {
		t.Fatal("positioned shadow expanded hit geometry")
	}
}

func TestPositionedOffscreenBodyKeepsVisibleHalo(t *testing.T) {
	f := testFrame()
	tree := Stack(Column().Width(20).Height(20).Position(-24, 30).Shadow(ShadowStyle{Color: RGB(0), Blur: 12}).Key("outside").OnClick(func(*Context) {})).ClipRounded(12)
	f.layout(tree, rect{100, 50, 100, 80}, rect{0, 0, 300, 200}, "root")
	if len(f.commands) != 1 || f.commands[0].Bounds.X != 76 || f.commands[0].RoundedClips[0].Radius != 12 || len(f.targets) != 0 {
		t.Fatalf("positioned offscreen halo/ancestor mask: %+v %+v", f.commands, f.targets)
	}
}

func TestPositionOnlyAffectsDirectStackChildren(t *testing.T) {
	f := testFrame()
	tree := Row(Column().Width(10).Height(12).Position(50, 60).Background(RGB(0xff0000)), Column().Width(20).Height(12).Background(RGB(0x00ff00)))
	if f.size(tree) != (dimensions{30, 12}) {
		t.Fatal("position changed flow measurement")
	}
	f.layout(tree, rect{0, 0, 30, 12}, rect{0, 0, 100, 100}, "root")
	if f.commands[0].Bounds.X != 0 || f.commands[1].Bounds.X != 10 {
		t.Fatalf("position changed a non-Stack parent: %+v", f.commands)
	}
}

func TestPositionedChildrenDoNotResizeEmptyStack(t *testing.T) {
	f := testFrame()
	tree := Stack(Column().Width(1000).Height(1000).Position(500, 500)).Padding(3)
	if f.size(tree) != (dimensions{6, 6}) {
		t.Fatalf("positioned child contributed to intrinsic size: %+v", f.size(tree))
	}
	e := Column().Position(float32(math.Inf(1)), float32(math.NaN()))
	if e.positionX != 0 || e.positionY != 0 {
		t.Fatal("nonfinite position escaped")
	}
	e.Position(-2e6, 2e6)
	if e.positionX != -1e6 || e.positionY != 1e6 {
		t.Fatal("position was not bounded")
	}
}
