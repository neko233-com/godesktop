//go:build windows && cgo

package main

/*
#cgo CXXFLAGS: -std=c++17 -O2
#cgo LDFLAGS: -static -ld3d12 -ldxgi -luuid -lstdc++
#include <stdlib.h>
int gd_private_dx12_timeout(char **text, size_t *length);
*/
import "C"

import (
	"errors"
	"unsafe"
)

func nativeProbe() ([]byte, error) {
	var text *C.char
	var length C.size_t
	code := C.gd_private_dx12_timeout(&text, &length)
	if text != nil {
		defer C.free(unsafe.Pointer(text))
	}
	if text == nil || length == 0 || uint64(length) > maxReportBytes {
		return nil, errors.New("native timeout probe returned invalid output bounds")
	}
	data := C.GoBytes(unsafe.Pointer(text), C.int(length))
	if code != 0 {
		return data, errors.New("real native timeout/cancellation gates failed")
	}
	return data, nil
}
