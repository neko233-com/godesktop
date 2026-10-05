package godesktop

import "math"

// Color uses straight-alpha, sRGB components in the range [0, 1].
type Color struct{ R, G, B, A float32 }

// RGB constructs an opaque color from a hexadecimal RGB value.
func RGB(hex uint32) Color { return RGBA(hex, 1) }

// RGBA constructs a color from a hexadecimal RGB value and an alpha component.
func RGBA(hex uint32, alpha float32) Color {
	return Color{float32(hex>>16&255) / 255, float32(hex>>8&255) / 255, float32(hex&255) / 255, unit(alpha)}
}

func unit(v float32) float32 { return min(1, nonnegative(v)) }

func nonnegative(v float32) float32 {
	if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) || v < 0 {
		return 0
	}
	return v
}

type rect struct{ x, y, w, h float32 }

func (r rect) contains(x, y float32) bool {
	return x >= r.x && y >= r.y && x < r.x+r.w && y < r.y+r.h
}

func (r rect) intersect(s rect) rect {
	x, y := max(r.x, s.x), max(r.y, s.y)
	return rect{x, y, max(0, min(r.x+r.w, s.x+s.w)-x), max(0, min(r.y+r.h, s.y+s.h)-y)}
}
