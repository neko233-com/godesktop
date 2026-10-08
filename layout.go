package godesktop

import (
	"fmt"

	"github.com/neko233-com/godesktop/internal/platform"
)

type textKey struct {
	text string
	size float32
	font string
}
type dimensions struct{ w, h float32 }
type target struct {
	key    string
	bounds rect
	click  func(*Context)
}
type frame struct {
	roundedClips [platform.MaxRoundedClips]platform.RoundedClip
	roundedDepth int
	targetClips  map[string][platform.MaxRoundedClips]platform.RoundedClip
	commands     []platform.Command
	targets      []target
	measure      func(string, float32) (float32, float32)
	fontMeasure  func(string, float32, string) (float32, float32)
	textCache    map[textKey]dimensions
	measured     map[*Element]dimensions
	focus        string
	hover        string
	keys         map[string]bool
	boundKeys    map[string]rect
}

func (f *frame) textSize(e *Element) dimensions {
	k := textKey{e.text, e.fontSize, e.fontFamily}
	if d, ok := f.textCache[k]; ok {
		return d
	}
	var w, h float32
	if e.fontFamily != "" && f.fontMeasure != nil {
		w, h = f.fontMeasure(e.text, e.fontSize, e.fontFamily)
	} else {
		w, h = f.measure(e.text, e.fontSize)
	}
	d := dimensions{nonnegative(w), nonnegative(h)}
	// Bound the cache even when labels contain ever-changing state.
	if len(f.textCache) >= 1024 {
		clear(f.textCache)
	}
	f.textCache[k] = d
	return d
}

func (f *frame) size(e *Element) dimensions {
	if e == nil {
		return dimensions{}
	}
	if d, ok := f.measured[e]; ok {
		return d
	}
	var d dimensions
	if e.kind == iconKind {
		d = dimensions{24, 24}
	} else if e.kind == textKind || e.kind == buttonKind {
		d = f.textSize(e)
	} else {
		count := 0
		for _, c := range e.children {
			if c == nil {
				continue
			}
			if e.kind == stackKind && c.positioned {
				continue
			}
			child := f.size(c)
			if e.kind == stackKind {
				d.w, d.h = max(d.w, child.w), max(d.h, child.h)
			} else if e.kind == rowKind {
				d.w += child.w
				d.h = max(d.h, child.h)
			} else {
				d.h += child.h
				d.w = max(d.w, child.w)
			}
			count++
		}
		if count > 1 && e.kind != stackKind {
			if e.kind == rowKind {
				d.w += float32(count-1) * e.gap
			} else {
				d.h += float32(count-1) * e.gap
			}
		}
	}
	if e.kind == imageKind {
		w, h := e.bitmap.Size()
		d = dimensions{float32(w), float32(h)}
	}
	px, py := e.insets()
	d.w += px * 2
	d.h += py * 2
	if e.width > 0 {
		d.w = e.width
	}
	if e.height > 0 {
		d.h = e.height
	}
	f.measured[e] = d
	return d
}

func nativeRect(r rect) platform.Rect { return platform.Rect{X: r.x, Y: r.y, W: r.w, H: r.h} }
func nativeColor(c Color) platform.Color {
	return platform.Color{R: unit(c.R), G: unit(c.G), B: unit(c.B), A: unit(c.A)}
}

func (f *frame) rectangle(bounds, clip rect, color Color, radius float32) {
	if color.A <= 0 || clip.w <= 0 || clip.h <= 0 {
		return
	}
	f.appendCommand(platform.Command{Kind: platform.Rectangle, Bounds: nativeRect(bounds), Clip: nativeRect(clip), Color: nativeColor(color), Radius: min(radius, min(bounds.w, bounds.h)/2)})
}

func (f *frame) appendCommand(command platform.Command) {
	command.RoundedClips = f.roundedClips
	f.commands = append(f.commands, command)
}

func (f *frame) layout(e *Element, bounds, clip rect, path string) {
	if e == nil || bounds.w <= 0 || bounds.h <= 0 {
		return
	}
	// An outset shadow belongs behind this surface, outside its own clip. Its
	// ancestors still constrain paint, and it never expands layout or hit bounds.
	f.paintShadow(e, bounds, clip)
	clip = clip.intersect(bounds)
	if clip.w <= 0 || clip.h <= 0 {
		return
	}
	background := e.background
	key := e.key
	if key == "" {
		key = path
	}
	if e.click != nil && key == f.hover && e.hoverBackground.A > 0 {
		background = e.hoverBackground
	}
	f.rectangle(bounds, clip, background, e.radius)
	if e.clipRadius > 0 {
		if f.roundedDepth == platform.MaxRoundedClips {
			panic("godesktop: at most four nested rounded clips are supported")
		}
		index := f.roundedDepth
		f.roundedClips[index] = platform.RoundedClip{Bounds: nativeRect(bounds), Radius: min(e.clipRadius, min(bounds.w, bounds.h)/2)}
		f.roundedDepth++
		defer func() { f.roundedDepth--; f.roundedClips[index] = platform.RoundedClip{} }()
	}
	if e.draggable {
		f.appendCommand(platform.Command{Kind: platform.DragRegion, Bounds: nativeRect(bounds), Clip: nativeRect(clip)})
	}
	px, py := e.insets()
	inner := rect{bounds.x + px, bounds.y + py, max(0, bounds.w-px*2), max(0, bounds.h-py*2)}
	if e.key != "" {
		f.recordBounds(e.key, clip)
	}
	if e.click != nil {
		key := e.key
		if key == "" {
			key = path
			f.recordBounds(key, clip)
		}
		f.targets = append(f.targets, target{key, clip, e.click})
		if f.roundedDepth > 0 {
			if f.targetClips == nil {
				f.targetClips = make(map[string][platform.MaxRoundedClips]platform.RoundedClip)
			}
			f.targetClips[key] = f.roundedClips
		} else {
			delete(f.targetClips, key)
		}
		if key == f.focus && e.focusRing {
			f.rectangle(rect{bounds.x, bounds.y, bounds.w, 2}, clip, RGB(0x93c5fd), 0)
			f.rectangle(rect{bounds.x, bounds.y + bounds.h - 2, bounds.w, 2}, clip, RGB(0x93c5fd), 0)
		}
	}
	if e.kind == iconKind {
		f.paintIcon(e, bounds, clip)
		return
	}
	if e.kind == imageKind {
		f.paintBitmap(e, bounds, clip)
		return
	}
	if e.kind == textKind || e.kind == buttonKind {
		d := f.textSize(e)
		textClip := clip.intersect(inner)
		if e.kind == buttonKind {
			inner.x += max(0, (inner.w-d.w)/2)
		}
		inner.y += max(0, (inner.h-d.h)/2)
		// Preserve the shaped dimensions; overflowing text is clipped, not scaled.
		inner.w, inner.h = d.w, d.h
		color := e.foreground
		if e.kind == buttonKind && e.click == nil {
			color.A *= 0.45
		}
		if inner.w > 0 && inner.h > 0 && textClip.w > 0 && textClip.h > 0 {
			f.appendCommand(platform.Command{Kind: platform.Label, Bounds: nativeRect(inner), Clip: nativeRect(textClip), Color: nativeColor(color), FontSize: e.fontSize, Text: e.text, FontFamily: e.fontFamily})
		}
		return
	}
	if e.kind == viewportKind {
		if len(e.children) == 0 || e.children[0] == nil {
			return
		}
		content := e.children[0]
		d := f.size(content)
		w, h := max(inner.w, d.w), max(inner.h, d.h)
		x, y := min(e.scrollX, max(0, w-inner.w)), min(e.scrollY, max(0, h-inner.h))
		f.layout(content, rect{inner.x - x, inner.y - y, w, h}, clip.intersect(inner), path+"/0")
		return
	}
	if e.kind == stackKind {
		for i, c := range e.children {
			if c == nil {
				continue
			}
			child := inner
			if c.positioned {
				d := f.size(c)
				child = rect{inner.x + c.positionX, inner.y + c.positionY, d.w, d.h}
			} else if c.width > 0 {
				child.w = min(child.w, c.width)
			}
			if !c.positioned && c.height > 0 {
				child.h = min(child.h, c.height)
			}
			f.layout(c, child, clip.intersect(inner), fmt.Sprintf("%s/%d", path, i))
		}
		return
	}
	main, total, weight, count := inner.h, float32(0), float32(0), 0
	if e.kind == rowKind {
		main = inner.w
	}
	for _, c := range e.children {
		if c == nil {
			continue
		}
		d := f.size(c)
		length := d.h
		if e.kind == rowKind {
			length = d.w
		}
		if c.flexBasisZero && c.grow > 0 {
			length = 0
		}
		total += length
		weight += c.grow
		count++
	}
	if count > 1 {
		total += float32(count-1) * e.gap
	}
	remaining, offset := max(0, main-total), float32(0)
	for i, c := range e.children {
		if c == nil {
			continue
		}
		d := f.size(c)
		length := d.h
		if e.kind == rowKind {
			length = d.w
		}
		if c.flexBasisZero && c.grow > 0 {
			length = 0
		}
		if weight > 0 {
			length += remaining * c.grow / weight
		}
		child := rect{inner.x, inner.y + offset, inner.w, length}
		if e.kind == rowKind {
			child = rect{inner.x + offset, inner.y, length, inner.h}
			if c.height > 0 {
				child.h = min(child.h, c.height)
			}
		} else if c.width > 0 {
			child.w = min(child.w, c.width)
		}
		f.layout(c, child, clip.intersect(inner), fmt.Sprintf("%s/%d", path, i))
		offset += length + e.gap
	}
}

func (f *frame) recordBounds(key string, bounds rect) {
	if f.keys[key] {
		panic(fmt.Sprintf("godesktop: duplicate element key %q", key))
	}
	f.keys[key] = true
	if f.boundKeys == nil {
		f.boundKeys = make(map[string]rect)
	}
	f.boundKeys[key] = bounds
}
