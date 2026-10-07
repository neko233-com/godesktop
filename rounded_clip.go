package godesktop

import "github.com/neko233-com/godesktop/internal/platform"

func (f *frame) targetContains(t target, x, y float32) bool {
	if !t.bounds.contains(x, y) {
		return false
	}
	for _, clip := range f.targetClips[t.key] {
		if clip.Radius <= 0 {
			continue
		}
		b, r := clip.Bounds, clip.Radius
		if x < b.X || y < b.Y || x >= b.X+b.W || y >= b.Y+b.H {
			return false
		}
		nearestX, nearestY := max(b.X+r, min(x, b.X+b.W-r)), max(b.Y+r, min(y, b.Y+b.H-r))
		dx, dy := x-nearestX, y-nearestY
		if dx*dx+dy*dy > r*r {
			return false
		}
	}
	return true
}

// Compile-time check keeps the documented clip bound tied to the scene ABI.
var _ [4]platform.RoundedClip = [platform.MaxRoundedClips]platform.RoundedClip{}
