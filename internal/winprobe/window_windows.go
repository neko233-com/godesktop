//go:build windows

// Package winprobe drives only the native integration fixture's owned window.
package winprobe

import (
	"fmt"
	"math"
	"syscall"
	"unsafe"
)

var user = syscall.NewLazyDLL("user32.dll")
var gdi = syscall.NewLazyDLL("gdi32.dll")

type Rect struct{ Left, Top, Right, Bottom int32 }
type Window uintptr

func Awareness() func() {
	fn := user.NewProc("SetThreadDpiAwarenessContext")
	old, _, _ := fn.Call(^uintptr(3)) // DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 (-4).
	return func() {
		if old != 0 {
			fn.Call(old)
		}
	}
}

func Find(title string, pid uint32) (Window, error) {
	name, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return 0, err
	}
	handle, _, _ := user.NewProc("FindWindowW").Call(0, uintptr(unsafe.Pointer(name)))
	if handle == 0 {
		return 0, fmt.Errorf("window %q was not found", title)
	}
	var owner uint32
	user.NewProc("GetWindowThreadProcessId").Call(handle, uintptr(unsafe.Pointer(&owner)))
	if owner != pid {
		return 0, fmt.Errorf("window belongs to PID %d, expected %d", owner, pid)
	}
	return Window(handle), nil
}

func (w Window) Send(message uint32, key, parameter uintptr) error {
	var result uintptr
	ok, _, err := user.NewProc("SendMessageTimeoutW").Call(uintptr(w), uintptr(message), key, parameter, 0x2|0x20, 3000, uintptr(unsafe.Pointer(&result)))
	if ok == 0 {
		return fmt.Errorf("SendMessageTimeout(0x%x): %w", message, err)
	}
	return nil
}

func (w Window) Close() error {
	ok, _, err := user.NewProc("PostMessageW").Call(uintptr(w), 0x10, 0, 0)
	if ok == 0 {
		return fmt.Errorf("PostMessage(WM_CLOSE): %w", err)
	}
	return nil
}

func (w Window) DPI() uint32 {
	dpi, _, _ := user.NewProc("GetDpiForWindow").Call(uintptr(w))
	return uint32(dpi)
}

func (w Window) Pointer(message uint32, x, y int) error {
	scale := float64(w.DPI()) / 96
	px, py := int16(math.Round(float64(x)*scale)), int16(math.Round(float64(y)*scale))
	parameter := uintptr(uint32(uint16(px)) | uint32(uint16(py))<<16)
	return w.Send(message, 0, parameter)
}

func (w Window) ClientSize() (int, int, error) {
	var r Rect
	ok, _, err := user.NewProc("GetClientRect").Call(uintptr(w), uintptr(unsafe.Pointer(&r)))
	if ok == 0 {
		return 0, 0, fmt.Errorf("GetClientRect: %w", err)
	}
	return int(r.Right), int(r.Bottom), nil
}

func (w Window) Resize(width, height int) error {
	dpi := w.DPI()
	r := Rect{Right: int32(math.Round(float64(width) * float64(dpi) / 96)), Bottom: int32(math.Round(float64(height) * float64(dpi) / 96))}
	ok, _, err := user.NewProc("AdjustWindowRectExForDpi").Call(uintptr(unsafe.Pointer(&r)), 0x00cf0000, 0, 0, uintptr(dpi))
	if ok == 0 {
		return fmt.Errorf("AdjustWindowRectExForDpi: %w", err)
	}
	ok, _, err = user.NewProc("SetWindowPos").Call(uintptr(w), 0, 0, 0, uintptr(r.Right-r.Left), uintptr(r.Bottom-r.Top), 0x2|0x4|0x10)
	if ok == 0 {
		return fmt.Errorf("SetWindowPos: %w", err)
	}
	return nil
}

func (w Window) Show(mode int) { user.NewProc("ShowWindow").Call(uintptr(w), uintptr(mode)) }

// Pixel reads a DIP location from the window's client DC after a paint barrier.
func (w Window) Pixel(x, y int) (uint32, error) {
	dc, _, err := user.NewProc("GetDC").Call(uintptr(w))
	if dc == 0 {
		return 0, fmt.Errorf("GetDC: %w", err)
	}
	defer user.NewProc("ReleaseDC").Call(uintptr(w), dc)
	scale := float64(w.DPI()) / 96
	value, _, err := gdi.NewProc("GetPixel").Call(dc, uintptr(math.Round(float64(x)*scale)), uintptr(math.Round(float64(y)*scale)))
	if uint32(value) == 0xffffffff {
		return 0, fmt.Errorf("GetPixel: %w", err)
	}
	// COLORREF is 0x00bbggrr; return ordinary 0xrrggbb.
	return uint32(value)&0xff<<16 | uint32(value)&0xff00 | uint32(value)>>16&0xff, nil
}
