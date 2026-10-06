package godesktop

import (
	"errors"
	"image"
	"image/draw"
	"sync/atomic"

	"github.com/neko233-com/godesktop/internal/platform"
)

// Bitmap owns an immutable copy of premultiplied RGBA pixels. Construct it once
// after decoding on an application worker; sharing it between views is safe.
// Native caches upload it once while resident, retaining textures until the
// GPU frames that use them finish. Native caches are bounded and clear on exit.
type Bitmap struct{ native *platform.Bitmap }

var nextBitmapID atomic.Uint64

// NewBitmap copies a decoded image, including images with nonzero origins and
// subimage strides. Each bitmap is limited to 4096 pixels per axis and 64 MiB.
// A nil/empty or oversized image returns an error without allocating pixels.
func NewBitmap(source image.Image) (*Bitmap, error) {
	if source == nil {
		return nil, errors.New("godesktop: bitmap image is nil")
	}
	b := source.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 || w > platform.MaxBitmapEdge || h > platform.MaxBitmapEdge || uint64(w)*uint64(h)*4 > platform.MaxBitmapBytes {
		return nil, errors.New("godesktop: bitmap must be nonempty and at most 4096 × 4096 / 64 MiB")
	}
	copy := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(copy, copy.Bounds(), source, b.Min, draw.Src)
	return &Bitmap{native: &platform.Bitmap{ID: nextBitmapID.Add(1), Width: w, Height: h, Pixels: copy.Pix}}, nil
}

// Size returns source pixel dimensions. A nil bitmap has zero dimensions.
func (b *Bitmap) Size() (int, int) {
	if b == nil || b.native == nil {
		return 0, 0
	}
	return b.native.Width, b.native.Height
}

// Image creates a GPU bitmap element. Width/Height define its available space;
// the image keeps its aspect ratio and is centered inside that space. Foreground
// multiplies its color/opacity (white preserves the original image colors).
func Image(bitmap *Bitmap) *Element {
	e := element(imageKind)
	e.bitmap = bitmap
	e.foreground = RGB(0xffffff)
	return e
}

func (f *frame) paintBitmap(e *Element, bounds, clip rect) {
	if e.bitmap == nil || e.bitmap.native == nil || e.foreground.A <= 0 {
		return
	}
	source := e.bitmap.native
	sx, sy := e.insets()
	inner := rect{bounds.x + sx, bounds.y + sy, max(0, bounds.w-2*sx), max(0, bounds.h-2*sy)}
	visible := clip.intersect(inner)
	if visible.w <= 0 || visible.h <= 0 {
		return
	}
	scale := min(inner.w/float32(source.Width), inner.h/float32(source.Height))
	if scale <= 0 {
		return
	}
	w, h := float32(source.Width)*scale, float32(source.Height)*scale
	fit := rect{inner.x + (inner.w-w)/2, inner.y + (inner.h-h)/2, w, h}
	f.commands = append(f.commands, platform.Command{Kind: platform.BitmapImage, Bounds: nativeRect(fit), Clip: nativeRect(visible), Color: nativeColor(e.foreground), Bitmap: source})
}
