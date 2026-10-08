package godesktop

import (
	"math"

	"github.com/neko233-com/godesktop/internal/platform"
)

// ShadowStyle describes one rounded rectangular shadow behind an element.
// Geometry uses DIP. Blur is twice the Gaussian standard deviation, bounded
// to [0, 128]; zero makes a sharp, antialiased shadow. Spread is bounded to
// [-128, 128], and offsets to [-4096, 4096]. Non-finite inputs become zero.
// Color uses straight alpha, independently of the element's background.
// Spread expands/insets both the caster rectangle and its corner radius.
type ShadowStyle struct {
	Color            Color
	OffsetX, OffsetY float32
	Blur, Spread     float32
}

// Shadow paints behind the element without changing measurement or input.
// Its rounded silhouette follows Radius/ClipRounded. It may extend beyond the
// element's bounds, but remains inside ancestor rectangular/rounded clips.
// A positioned Stack child can keep its body coordinates while using the
// Stack's full inherited clip. Tight flow parents must reserve halo space.
// A zero-alpha color disables the shadow. Each visible shadow uses one GPU
// instance, no texture, and no additional render pass or background work.
func (e *Element) Shadow(style ShadowStyle) *Element {
	style.Color = Color{unit(style.Color.R), unit(style.Color.G), unit(style.Color.B), unit(style.Color.A)}
	style.OffsetX = boundedFinite(style.OffsetX, -4096, 4096)
	style.OffsetY = boundedFinite(style.OffsetY, -4096, 4096)
	style.Blur = boundedFinite(style.Blur, 0, 128)
	style.Spread = boundedFinite(style.Spread, -128, 128)
	e.shadow = style
	return e
}

func boundedFinite(value, lower, upper float32) float32 {
	if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
		return 0
	}
	return min(upper, max(lower, value))
}

func (f *frame) paintShadow(e *Element, bounds, clip rect) {
	s := e.shadow
	if s.Color.A <= 0 || clip.w <= 0 || clip.h <= 0 {
		return
	}
	caster := rect{bounds.x + s.OffsetX - s.Spread, bounds.y + s.OffsetY - s.Spread, bounds.w + 2*s.Spread, bounds.h + 2*s.Spread}
	if caster.w <= 0 || caster.h <= 0 {
		return
	}
	sigma := s.Blur / 2
	extent := 4*sigma + 1
	paint := rect{caster.x - extent, caster.y - extent, caster.w + 2*extent, caster.h + 2*extent}
	if visible := paint.intersect(clip); visible.w <= 0 || visible.h <= 0 {
		return
	}
	f.appendCommand(platform.Command{Kind: platform.Shadow, Bounds: nativeRect(caster), Clip: nativeRect(clip), Color: nativeColor(s.Color), Radius: min(max(0, e.radius+s.Spread), min(caster.w, caster.h)/2), ShadowSigma: sigma})
}
