//go:build windows

// Package winprobe drives only the native integration fixture's owned window.
package winprobe

import (
	"fmt"
	"math"
	"os"
	"syscall"
	"unsafe"
)

var user = syscall.NewLazyDLL("user32.dll")

type Rect struct{ Left, Top, Right, Bottom int32 }
type Window uintptr

func WorkArea() (Rect, error) {
	var r Rect
	ok, _, err := user.NewProc("SystemParametersInfoW").Call(0x30, 0, uintptr(unsafe.Pointer(&r)), 0)
	if ok == 0 {
		return r, err
	}
	return r, nil
}
func (w Window) ScreenBounds() (Rect, error) {
	var r Rect
	ok, _, err := user.NewProc("GetWindowRect").Call(uintptr(w), uintptr(unsafe.Pointer(&r)))
	if ok == 0 {
		return r, err
	}
	return r, nil
}

// ClientBounds returns the client rectangle in physical screen coordinates.
func (w Window) ClientBounds() (Rect, error) {
	width, height, err := w.ClientSize()
	if err != nil {
		return Rect{}, err
	}
	point := struct{ X, Y int32 }{}
	ok, _, err := user.NewProc("ClientToScreen").Call(uintptr(w), uintptr(unsafe.Pointer(&point)))
	if ok == 0 {
		return Rect{}, err
	}
	return Rect{point.X, point.Y, point.X + int32(width), point.Y + int32(height)}, nil
}

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
	if os.Getenv("GODESKTOP_TEST_INPUT_ISOLATION") == "1" {
		switch message {
		case 0x201, 0x202, 0x200, 0x20a, 0x20e, 0x100, 0x101, 0x104, 0x105, 0x102, 0x1f, 0x8, 0x215, 0x6:
			payload := struct {
				Message, Modifiers uint32
				WParam, LParam     uintptr
			}{Message: message, WParam: key, LParam: parameter}
			if message == 0x104 || message == 0x105 {
				payload.Modifiers |= 4
			}
			var pid uint32
			user.NewProc("GetWindowThreadProcessId").Call(uintptr(w), uintptr(unsafe.Pointer(&pid)))
			if pid == uint32(os.Getpid()) && (message == 0x100 || message == 0x101 || message == 0x104 || message == 0x105) {
				for _, pair := range [][2]uint32{{16, 1}, {17, 2}, {18, 4}} {
					state, _, _ := user.NewProc("GetKeyState").Call(uintptr(pair[0]))
					if state&0x8000 != 0 {
						payload.Modifiers |= pair[1]
					}
				}
			}
			if message == 0x201 || message == 0x202 || message == 0x200 {
				if key&4 != 0 {
					payload.Modifiers |= 1
				}
				if key&8 != 0 {
					payload.Modifiers |= 2
				}
			}
			copy := struct {
				ID   uintptr
				Size uint32
				Data unsafe.Pointer
			}{ID: 0x47445052, Size: uint32(unsafe.Sizeof(payload)), Data: unsafe.Pointer(&payload)}
			ok, _, err := user.NewProc("SendMessageTimeoutW").Call(uintptr(w), 0x4a, 0, uintptr(unsafe.Pointer(&copy)), 0x2|0x20, 3000, uintptr(unsafe.Pointer(&result)))
			if ok == 0 || result != 1 {
				return fmt.Errorf("isolated owned message 0x%x rejected: %w", message, err)
			}
			return nil
		}
	}
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

// Wheel sends a real HWND wheel message at a logical client position. The
// Win32 payload requires signed physical screen coordinates, unlike Pointer.
// Horizontal positive delta scrolls right; vertical positive delta scrolls up.
// modifiers uses Win32 MK_SHIFT/MK_CONTROL bits in the low word.
func (w Window) Wheel(horizontal bool, x, y int, delta int16, modifiers uint16) error {
	scale := float64(w.DPI()) / 96
	point := struct{ X, Y int32 }{int32(math.Round(float64(x) * scale)), int32(math.Round(float64(y) * scale))}
	ok, _, err := user.NewProc("ClientToScreen").Call(uintptr(w), uintptr(unsafe.Pointer(&point)))
	if ok == 0 {
		return fmt.Errorf("ClientToScreen: %w", err)
	}
	message := uint32(0x20a)
	if horizontal {
		message = 0x20e
	}
	position := uintptr(uint32(uint16(int16(point.X))) | uint32(uint16(int16(point.Y)))<<16)
	return w.Send(message, uintptr(uint32(modifiers)|uint32(uint16(delta))<<16), position)
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

// Raise brings the verified test window to the top without moving it.
func (w Window) Raise() {
	user.NewProc("SetWindowPos").Call(uintptr(w), 0, 0, 0, 0, 0, 0x1|0x2|0x10)
	user.NewProc("SetForegroundWindow").Call(uintptr(w))
}

// Pin temporarily keeps the owned test window above other applications.
func (w Window) Pin() func() {
	var pid uint32
	user.NewProc("GetWindowThreadProcessId").Call(uintptr(w), uintptr(unsafe.Pointer(&pid)))
	user.NewProc("SetWindowPos").Call(uintptr(w), ^uintptr(0), 0, 0, 0, 0, 0x1|0x2|0x10)
	return func() {
		var current uint32
		user.NewProc("GetWindowThreadProcessId").Call(uintptr(w), uintptr(unsafe.Pointer(&current)))
		if current == pid {
			user.NewProc("SetWindowPos").Call(uintptr(w), ^uintptr(1), 0, 0, 0, 0, 0x1|0x2|0x10)
		}
	}
}

// Pixel reads a DIP location from the latest completed GPU frame.
func (w Window) Pixel(x, y int) (uint32, error) { return w.gpuPixel(x, y) }
