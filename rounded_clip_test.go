package godesktop

import (
	"github.com/neko233-com/godesktop/internal/platform"
	"testing"
)

func TestRoundedClipPropagatesAndRestores(t *testing.T) {
	f := testFrame()
	root := Row(Column(Column().Flex(1).Background(RGB(0xff0000)).OnClick(func(*Context) {}).Key("clipped")).Width(100).Height(80).ClipRounded(20), Column().Width(100).Height(80).Background(RGB(0x00ff00)).OnClick(func(*Context) {}).Key("plain"))
	f.layout(root, rect{0, 0, 200, 80}, rect{0, 0, 200, 80}, "root")
	if len(f.commands) != 2 || f.commands[0].RoundedClips[0].Radius != 20 || f.commands[1].RoundedClips[0].Radius != 0 {
		t.Fatal("clip leaked into sibling", f.commands)
	}
	if f.targetContains(f.targets[0], 1, 1) || !f.targetContains(f.targets[0], 20, 20) || !f.targetContains(f.targets[1], 101, 1) {
		t.Fatal("rounded hit areas differ from visible surface")
	}
	if f.roundedDepth != 0 || f.roundedClips != ([platform.MaxRoundedClips]platform.RoundedClip{}) {
		t.Fatal("frame clip stack was not restored")
	}
}
func TestRoundedClipDepthBound(t *testing.T) {
	f := testFrame()
	e := Column().Width(100).Height(80).Background(RGB(0xff0000))
	for range platform.MaxRoundedClips + 1 {
		e = Column(e).ClipRounded(8)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("excess rounded nesting was silently truncated")
		}
	}()
	f.layout(e, rect{0, 0, 100, 80}, rect{0, 0, 100, 80}, "root")
}
