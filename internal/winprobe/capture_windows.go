//go:build windows

package winprobe

import (
	"image"
	"unsafe"
)

// Capture reads a fenced GPU copy of the client area of the verified HWND.
func (w Window) Capture() (*image.RGBA, error) { return w.gpuCapture() }

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
