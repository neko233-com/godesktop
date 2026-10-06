//go:build darwin && cgo

package platform

/*
#include <stdlib.h>
#include "bridge.h"
*/
import "C"

import (
	"errors"
	"image"
	"unsafe"
)

// MetalWindowIdentity is an AppKit-window diagnostic. Call on the UI thread.
func MetalWindowIdentity() uint64 { return uint64(C.gd_metal_window_identity()) }

// MetalTestWheel constructs a native NSEvent for the readback-enabled owned view.
// It never posts a global input event. Call on the UI thread.
func MetalTestWheel(dx, dy int, x, y float32, modifiers int, precise bool) error {
	var unit C.int
	if precise {
		unit = 1
	}
	if err := C.gd_metal_test_wheel(C.int(dx), C.int(dy), C.float(x), C.float(y), C.int(modifiers), unit); err != nil {
		return errors.New(C.GoString(err))
	}
	return nil
}

func MetalTestPointer(pressed bool, x, y float32, modifiers int) error {
	var down C.int
	if pressed {
		down = 1
	}
	if err := C.gd_metal_test_pointer(down, C.float(x), C.float(y), C.int(modifiers)); err != nil {
		return errors.New(C.GoString(err))
	}
	return nil
}

func MetalTestKey(key, modifiers int, pressed bool) error {
	var down C.int
	if pressed {
		down = 1
	}
	if err := C.gd_metal_test_key(C.int(key), C.int(modifiers), down); err != nil {
		return errors.New(C.GoString(err))
	}
	return nil
}

// MetalSnapshot reads the most recently completed actual window drawable.
// It is diagnostic-only and requires GODESKTOP_READBACK=1 before Run.
func MetalSnapshot() (*image.RGBA, uint64, error) {
	var raw C.GDGPUSnapshot
	message := C.gd_metal_snapshot(&raw)
	defer C.free(unsafe.Pointer(raw.pixels))
	if message != nil {
		return nil, 0, errors.New(C.GoString(message))
	}
	pixels, err := copySnapshot(raw, true)
	return pixels, uint64(raw.frame), err
}

// MetalTextReference uses independent whole-line CoreText drawing; the renderer
// instead places cached, individually rasterized glyphs in an R8 GPU atlas.
func MetalTextReference(text, font string, size, scale float32, width, height int) (*image.RGBA, error) {
	bytes, family := C.CString(text), C.CString(font)
	defer C.free(unsafe.Pointer(bytes))
	defer C.free(unsafe.Pointer(family))
	var raw C.GDGPUSnapshot
	message := C.gd_metal_text_reference(bytes, C.size_t(len(text)), family, C.size_t(len(font)), C.float(size), C.float(scale), C.uint32_t(width), C.uint32_t(height), &raw)
	defer C.free(unsafe.Pointer(raw.pixels))
	if message != nil {
		return nil, errors.New(C.GoString(message))
	}
	return copySnapshot(raw, false)
}

func copySnapshot(raw C.GDGPUSnapshot, bgra bool) (*image.RGBA, error) {
	width, height, stride := int(raw.width), int(raw.height), int(raw.stride)
	if width <= 0 || height <= 0 || stride != width*4 || uint64(raw.bytes) != uint64(stride)*uint64(height) || uint64(raw.bytes) > 64*1024*1024 || raw.pixels == nil {
		return nil, errors.New("invalid native GPU snapshot")
	}
	pixels := C.GoBytes(unsafe.Pointer(raw.pixels), C.int(raw.bytes))
	if bgra {
		for i := 0; i < len(pixels); i += 4 {
			pixels[i], pixels[i+2] = pixels[i+2], pixels[i]
		}
	}
	return &image.RGBA{Pix: pixels, Stride: stride, Rect: image.Rect(0, 0, width, height)}, nil
}
