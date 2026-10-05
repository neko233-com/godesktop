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
	"unsafe"
)

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
