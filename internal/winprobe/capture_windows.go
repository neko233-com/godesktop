//go:build windows

package winprobe

import (
	"fmt"
	"image"
	"unsafe"
)

// Capture reads only the client area of a window previously verified with Find.

func (w Window) Capture() (*image.RGBA, error) {
	width, height, err := w.ClientSize()
	if err != nil {
		return nil, err
	}
	if width < 1 || height < 1 || width > 16384 || height > 16384 {
		return nil, fmt.Errorf("invalid capture size %dx%d", width, height)
	}
	dc, _, err := user.NewProc("GetDC").Call(uintptr(w))
	if dc == 0 {
		return nil, err
	}
	defer user.NewProc("ReleaseDC").Call(uintptr(w), dc)
	memory, _, err := gdi.NewProc("CreateCompatibleDC").Call(dc)
	if memory == 0 {
		return nil, err
	}
	defer gdi.NewProc("DeleteDC").Call(memory)
	bitmap, _, err := gdi.NewProc("CreateCompatibleBitmap").Call(dc, uintptr(width), uintptr(height))
	if bitmap == 0 {
		return nil, err
	}
	defer gdi.NewProc("DeleteObject").Call(bitmap)
	old, _, _ := gdi.NewProc("SelectObject").Call(memory, bitmap)
	ok, _, copyErr := gdi.NewProc("BitBlt").Call(memory, 0, 0, uintptr(width), uintptr(height), dc, 0, 0, 0x00cc0020)
	// GetDIBits requires that the bitmap is no longer selected into a DC.
	gdi.NewProc("SelectObject").Call(memory, old)
	if ok == 0 {
		return nil, copyErr
	}
	type bitmapInfo struct {
		Size                   uint32
		Width, Height          int32
		Planes, BitCount       uint16
		Compression, SizeImage uint32
		X, Y                   int32
		Used, Important        uint32
	}
	info := bitmapInfo{Size: 40, Width: int32(width), Height: -int32(height), Planes: 1, BitCount: 32}
	buffer := make([]byte, width*height*4)
	lines, _, err := gdi.NewProc("GetDIBits").Call(dc, bitmap, 0, uintptr(height), uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&info)), 0)
	if lines != uintptr(height) {
		return nil, fmt.Errorf("GetDIBits: %v", err)
	}
	for i := 0; i < len(buffer); i += 4 {
		buffer[i], buffer[i+2] = buffer[i+2], buffer[i]
		buffer[i+3] = 255
	}
	return &image.RGBA{Pix: buffer, Stride: width * 4, Rect: image.Rect(0, 0, width, height)}, nil
}

// HitTest performs WM_NCHITTEST at a client DIP position and returns its result.
func (w Window) HitTest(x, y int) (int, error) {
	point := struct{ X, Y int32 }{int32(x) * int32(w.DPI()) / 96, int32(y) * int32(w.DPI()) / 96}
	user.NewProc("ClientToScreen").Call(uintptr(w), uintptr(unsafe.Pointer(&point)))
	var result uintptr
	ok, _, err := user.NewProc("SendMessageTimeoutW").Call(uintptr(w), 0x84, 0, uintptr(uint32(uint16(point.X))|uint32(uint16(point.Y))<<16), 0x2|0x20, 3000, uintptr(unsafe.Pointer(&result)))
	if ok == 0 {
		return 0, err
	}
	return int(result), nil
}
