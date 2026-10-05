package godesktop

import (
	"math"
	"testing"
)

func TestNumericStylesRejectNonfiniteAndNegativeValues(t *testing.T) {
	for _, value := range []float32{-1, float32(math.NaN()), float32(math.Inf(1)), float32(math.Inf(-1))} {
		e := Text("x").Width(value).Height(value).Grow(value).Padding(value).Gap(value).Radius(value).FontSize(value)
		if e.width != 0 || e.height != 0 || e.grow != 0 || e.padding != 0 || e.gap != 0 || e.radius != 0 || e.fontSize != 1 {
			t.Fatalf("unsanitized style for %v: %+v", value, e)
		}
	}
	if c := RGBA(0xff8040, 2); c.R != 1 || c.G != float32(128)/255 || c.B != float32(64)/255 || c.A != 1 {
		t.Fatalf("RGBA: %+v", c)
	}
	if c := RGBA(0, -1); c.A != 0 {
		t.Fatalf("negative alpha: %v", c.A)
	}
}

func FuzzRectIntersection(f *testing.F) {
	f.Add(float32(0), float32(0), float32(100), float32(80), float32(20), float32(10), float32(60), float32(40))
	f.Add(float32(-10), float32(30), float32(5), float32(6), float32(100), float32(100), float32(1), float32(1))
	f.Fuzz(func(t *testing.T, x, y, w, h, sx, sy, sw, sh float32) {
		for _, v := range []float32{x, y, w, h, sx, sy, sw, sh} {
			if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) || math.Abs(float64(v)) > 10000 {
				return
			}
		}
		a, b := rect{x, y, nonnegative(w), nonnegative(h)}, rect{sx, sy, nonnegative(sw), nonnegative(sh)}
		got := a.intersect(b)
		if got != b.intersect(a) || got.w < 0 || got.h < 0 {
			t.Fatalf("invalid intersection: a=%+v b=%+v got=%+v", a, b, got)
		}
		if got.w > 0 && got.h > 0 {
			if got.x < a.x || got.y < a.y || got.x < b.x || got.y < b.y || got.x+got.w > min(a.x+a.w, b.x+b.w)+0.002 || got.y+got.h > min(a.y+a.h, b.y+b.h)+0.002 {
				t.Fatalf("intersection escapes inputs: %+v", got)
			}
		}
	})
}
