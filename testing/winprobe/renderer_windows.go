//go:build windows

package winprobe

import (
	"syscall"
	"unsafe"
)

var rendererUser = syscall.NewLazyDLL("user32.dll")

// NativePresentation reads the renderer's actual property from exactly the
// expected process-owned HWND. Missing/unknown properties, a different backend
// and a closed/reused window are errors. This performs no input or rendering.
func NativePresentation(window Window, pid uint32) (string, error) {
	owner := func() uint32 {
		var actual uint32
		rendererUser.NewProc("GetWindowThreadProcessId").Call(uintptr(window), uintptr(unsafe.Pointer(&actual)))
		return actual
	}
	property := func(name string) (uintptr, error) {
		key, err := syscall.UTF16PtrFromString(name)
		if err != nil {
			return 0, err
		}
		value, _, _ := rendererUser.NewProc("GetPropW").Call(uintptr(window), uintptr(unsafe.Pointer(key)))
		return value, nil
	}
	return readWindowPresentation(uintptr(window), pid, owner, property)
}
