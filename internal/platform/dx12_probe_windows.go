//go:build windows && cgo

package platform

/*
#include <stdlib.h>
#include "bridge.h"
*/
import "C"

import (
	"encoding/json"
	"errors"
	"image"
	"unsafe"
)

// Direct3D12TextReference draws an entire layout using native Direct2D color-font
// support, independently of the window's glyph atlas and D3D12 shader.
func Direct3D12TextReference(text, font string, size, scale float32, width, height int) (*image.RGBA, error) {
	if width <= 0 || height <= 0 || uint64(width)*uint64(height) > 64*1024*1024/4 {
		return nil, errors.New("invalid text reference size")
	}
	ctext, cfont := C.CString(text), C.CString(font)
	defer C.free(unsafe.Pointer(ctext))
	defer C.free(unsafe.Pointer(cfont))
	var result C.GDGPUSnapshot
	message := C.gd_dx12_text_reference(ctext, C.size_t(len(text)), cfont, C.size_t(len(font)), C.float(size), C.float(scale), C.uint32_t(width), C.uint32_t(height), &result)
	defer C.free(unsafe.Pointer(result.pixels))
	if message != nil {
		return nil, errors.New(C.GoString(message))
	}
	return &image.RGBA{Pix: C.GoBytes(unsafe.Pointer(result.pixels), C.int(result.bytes)), Stride: int(result.stride), Rect: image.Rect(0, 0, int(result.width), int(result.height))}, nil
}

// GPUProbe is internal acceptance data, not a public window renderer. Pixels
// contain consecutive BGRA frames copied from fenced D3D12 render targets.
type GPUProbe struct {
	Report                        json.RawMessage
	Pixels                        []byte
	Width, Height, Frames, Stride int
}

func Direct3D12Probe(flags uint32, frames uint32) (GPUProbe, error) {
	var result C.GDGPUProbe
	message := C.gd_dx12_probe(C.uint32_t(flags), C.uint32_t(frames), &result)
	defer C.free(unsafe.Pointer(result.json))
	defer C.free(unsafe.Pointer(result.pixels))
	if message != nil {
		return GPUProbe{}, errors.New(C.GoString(message))
	}
	return GPUProbe{
		Report: json.RawMessage(C.GoString(result.json)),
		Pixels: C.GoBytes(unsafe.Pointer(result.pixels), C.int(result.frames*result.height*result.stride)),
		Width:  int(result.width), Height: int(result.height), Frames: int(result.frames), Stride: int(result.stride),
	}, nil
}
