package godesktop

import (
	"image"
	"image/color"
	"testing"

	"github.com/neko233-com/godesktop/internal/platform"
)

type boundedBitmapFixture struct{ bounds image.Rectangle }

func (b boundedBitmapFixture) ColorModel() color.Model { return color.RGBAModel }
func (b boundedBitmapFixture) Bounds() image.Rectangle { return b.bounds }
func (b boundedBitmapFixture) At(int, int) color.Color { panic("invalid image was read") }

func TestBitmapCopyOriginStrideAndPremultipliedAlpha(t *testing.T) {
	base := image.NewNRGBA(image.Rect(3, 5, 12, 14))
	for y := 5; y < 14; y++ {
		for x := 3; x < 12; x++ {
			base.SetNRGBA(x, y, color.NRGBA{R: 255, G: 64, B: 32, A: 128})
		}
	}
	source := base.SubImage(image.Rect(4, 7, 7, 9))
	b, err := NewBitmap(source)
	if err != nil {
		t.Fatal(err)
	}
	if w, h := b.Size(); w != 3 || h != 2 {
		t.Fatal("subimage dimensions", w, h)
	}
	for i := 0; i < len(b.native.Pixels); i += 4 {
		if got := b.native.Pixels[i : i+4]; got[0] != 128 || got[1] != 32 || got[2] != 16 || got[3] != 128 {
			t.Fatal("premultiplied/stride copy", got)
		}
	}
	base.SetNRGBA(4, 7, color.NRGBA{})
	if b.native.Pixels[0] != 128 {
		t.Fatal("bitmap retained mutable source pixels")
	}
	other, err := NewBitmap(source)
	if err != nil || other.native.ID == b.native.ID {
		t.Fatal("distinct immutable assets share identity", err)
	}
	for _, src := range []image.Image{nil, boundedBitmapFixture{image.Rect(0, 0, 0, 1)}, boundedBitmapFixture{image.Rect(0, 0, 4097, 1)}, boundedBitmapFixture{image.Rect(0, 0, 1, 4097)}} {
		if _, err := NewBitmap(src); err == nil {
			t.Fatal("invalid/bounded image accepted")
		}
	}
	var empty *Bitmap
	if w, h := empty.Size(); w != 0 || h != 0 {
		t.Fatal("nil size")
	}
}

func TestBitmapAspectPaddingClipAndInputIdentity(t *testing.T) {
	b, err := NewBitmap(image.NewRGBA(image.Rect(0, 0, 20, 10)))
	if err != nil {
		t.Fatal(err)
	}
	f := testFrame()
	e := Image(b).Width(80).Height(80).PaddingXY(10, 20).Key("picture").OnClick(func(*Context) {})
	if d := f.size(e); d.w != 80 || d.h != 80 {
		t.Fatal("explicit bitmap layout", d)
	}
	f.layout(e, rect{w: 80, h: 80}, rect{w: 60, h: 80}, "root")
	if len(f.commands) != 1 || f.commands[0].Kind != platform.BitmapImage {
		t.Fatal("bitmap command missing", f.commands)
	}
	c := f.commands[0]
	if c.Bounds.X != 10 || c.Bounds.Y != 25 || c.Bounds.W != 60 || c.Bounds.H != 30 || c.Clip.X != 10 || c.Clip.W != 50 {
		t.Fatal("aspect/inner clipping incorrect", c)
	}
	if len(f.targets) != 1 || f.targets[0].key != "picture" {
		t.Fatal("bitmap input target missing")
	}
	if c.Color.R != 1 || c.Color.G != 1 || c.Color.B != 1 || c.Color.A != 1 {
		t.Fatal("original image color changed")
	}
	f = testFrame()
	f.layout(Image(nil), rect{w: 30, h: 30}, rect{w: 30, h: 30}, "root")
	if len(f.commands) != 0 {
		t.Fatal("nil image painted")
	}
	f = testFrame()
	if d := f.size(Image(b)); d.w != 20 || d.h != 10 {
		t.Fatal("intrinsic bitmap dimensions", d)
	}
	f.layout(Image(b).Padding(50), rect{w: 30, h: 30}, rect{w: 30, h: 30}, "root")
	if len(f.commands) != 0 {
		t.Fatal("empty padded image painted")
	}
	f = testFrame()
	f.layout(Image(b).Foreground(RGBA(0xffffff, 0)), rect{w: 20, h: 10}, rect{w: 20, h: 10}, "root")
	if len(f.commands) != 0 {
		t.Fatal("invisible bitmap consumed native resources")
	}
	f = testFrame()
	f.layout(Image(b), rect{x: 50, w: 20, h: 10}, rect{w: 20, h: 10}, "root")
	if len(f.commands) != 0 {
		t.Fatal("offscreen bitmap consumed native resources")
	}
}
