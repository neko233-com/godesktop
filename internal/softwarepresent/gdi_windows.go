//go:build windows && cgo

package main

import (
	"errors"
	"fmt"
	"image"
	"math"
	"os"
	"syscall"
	"unsafe"

	"github.com/neko233-com/godesktop/testing/winprobe"
)

var clientUser = syscall.NewLazyDLL("user32.dll")
var clientGDI = syscall.NewLazyDLL("gdi32.dll")
var clientKernel = syscall.NewLazyDLL("kernel32.dll")

func owned(window winprobe.Window, pid uint32) error {
	if window == 0 || pid == 0 {
		return errors.New("client presentation requires a nonzero owned HWND/PID")
	}
	var actual uint32
	clientUser.NewProc("GetWindowThreadProcessId").Call(uintptr(window), uintptr(unsafe.Pointer(&actual)))
	if actual != pid || pid != uint32(os.Getpid()) {
		return fmt.Errorf("client presentation HWND owner %d differs from this fixture PID %d", actual, pid)
	}
	return nil
}

func ownTopmost(window winprobe.Window, pid uint32) error {
	if err := owned(window, pid); err != nil {
		return err
	}
	ok, _, err := clientUser.NewProc("SetWindowPos").Call(uintptr(window), ^uintptr(0), 0, 0, 0, 0, 0x13) // TOPMOST, NOSIZE|NOMOVE|NOACTIVATE.
	if ok == 0 {
		return fmt.Errorf("place owned client above unrelated windows: %v", err)
	}
	return nil
}

func isIconic(window winprobe.Window) bool {
	result, _, _ := clientUser.NewProc("IsIconic").Call(uintptr(window))
	return result != 0
}

func expose(window winprobe.Window, pid uint32) error {
	if err := owned(window, pid); err != nil {
		return err
	}
	ok, _, err := clientUser.NewProc("InvalidateRect").Call(uintptr(window), 0, 0)
	if ok == 0 {
		return fmt.Errorf("invalidate own client for actual WM_PAINT: %v", err)
	}
	clientUser.NewProc("UpdateWindow").Call(uintptr(window))
	return nil
}

func noDiagnosticMapping(window winprobe.Window, pid uint32) error {
	if err := owned(window, pid); err != nil {
		return err
	}
	name, err := syscall.UTF16PtrFromString(fmt.Sprintf("Local\\godesktop.gpu.%d.%d", pid, uintptr(window)))
	if err != nil {
		return err
	}
	handle, _, callErr := clientKernel.NewProc("OpenFileMappingW").Call(4, 0, uintptr(unsafe.Pointer(name)))
	if handle != 0 {
		clientKernel.NewProc("CloseHandle").Call(handle)
		return errors.New("non-diagnostic visible-client test unexpectedly has a GPU capture mapping")
	}
	if callErr != syscall.Errno(2) {
		return fmt.Errorf("GPU mapping absence not established: %v", callErr)
	}
	return nil
}

func colorRefRGB(pixel uint32) uint32 {
	return (pixel&0xff)<<16 | pixel&0xff00 | (pixel>>16)&0xff
}

func markerPoints(dpi uint32) []struct {
	x, y int
	rgb  uint32
} {
	scale := float64(dpi) / 96
	return []struct {
		x, y int
		rgb  uint32
	}{{int(math.Round(30 * scale)), int(math.Round(20 * scale)), 0xff0000}, {int(math.Round(30 * scale)), int(math.Round(140 * scale)), 0x0000ff}}
}

func clientColor(window winprobe.Window, pid uint32, expected uint32) (bool, error) {
	if err := owned(window, pid); err != nil {
		return false, err
	}
	if isIconic(window) {
		return false, nil
	}
	visible, _, _ := clientUser.NewProc("IsWindowVisible").Call(uintptr(window))
	if visible == 0 {
		return false, nil
	}
	width, height, err := window.ClientSize()
	if err != nil {
		return false, err
	}
	if width < 50 || height < 50 {
		return false, errors.New("owned client unexpectedly too small for visible pixels")
	}
	dc, _, err := clientUser.NewProc("GetDC").Call(uintptr(window))
	if dc == 0 {
		return false, fmt.Errorf("acquire actual own-client DC: %v", err)
	}
	defer clientUser.NewProc("ReleaseDC").Call(uintptr(window), dc)
	for _, point := range [][2]int{{width / 2, 20}, {width / 2, height / 2}, {width - 21, height - 21}} {
		pixel, _, _ := clientGDI.NewProc("GetPixel").Call(dc, uintptr(point[0]), uintptr(point[1]))
		if uint32(pixel) == ^uint32(0) {
			return false, errors.New("actual own-client pixel unavailable")
		}
		rgb := colorRefRGB(uint32(pixel))
		if rgb != expected {
			return false, nil
		}
	}
	for _, marker := range markerPoints(window.DPI()) {
		pixel, _, _ := clientGDI.NewProc("GetPixel").Call(dc, uintptr(marker.x), uintptr(marker.y))
		if uint32(pixel) == ^uint32(0) {
			return false, errors.New("actual own-client marker pixel unavailable")
		}
		if colorRefRGB(uint32(pixel)) != marker.rgb {
			return false, nil
		}
	}
	return true, owned(window, pid)
}

func captureClient(window winprobe.Window, pid uint32) (*image.RGBA, error) {
	if err := owned(window, pid); err != nil {
		return nil, err
	}
	visible, _, _ := clientUser.NewProc("IsWindowVisible").Call(uintptr(window))
	if visible == 0 || isIconic(window) {
		return nil, errors.New("actual client capture requires the owned HWND to be visibly restored")
	}
	width, height, err := window.ClientSize()
	if err != nil {
		return nil, err
	}
	if width <= 0 || height <= 0 || width > 16384 || height > 16384 || uint64(width)*uint64(height)*4 > 64*1024*1024 {
		return nil, errors.New("actual own-client capture dimensions exceed its 64MiB bound")
	}
	dc, _, callErr := clientUser.NewProc("GetDC").Call(uintptr(window))
	if dc == 0 {
		return nil, fmt.Errorf("acquire actual own-client capture DC: %v", callErr)
	}
	defer clientUser.NewProc("ReleaseDC").Call(uintptr(window), dc)
	memory, _, callErr := clientGDI.NewProc("CreateCompatibleDC").Call(dc)
	if memory == 0 {
		return nil, fmt.Errorf("allocate own-client capture DC: %v", callErr)
	}
	defer clientGDI.NewProc("DeleteDC").Call(memory)
	info := struct {
		Size          uint32
		Width, Height int32
		Planes, Bits  uint16
		Compression   uint32
		ImageSize     uint32
		XPels, YPels  int32
		Used, Needed  uint32
		Colors        [1]uint32
	}{Size: 40, Width: int32(width), Height: -int32(height), Planes: 1, Bits: 32}
	var address uintptr
	bitmap, _, callErr := clientGDI.NewProc("CreateDIBSection").Call(dc, uintptr(unsafe.Pointer(&info)), 0, uintptr(unsafe.Pointer(&address)), 0, 0)
	if bitmap == 0 || address == 0 {
		if bitmap != 0 {
			clientGDI.NewProc("DeleteObject").Call(bitmap)
		}
		return nil, fmt.Errorf("allocate bounded native client capture bitmap: %v", callErr)
	}
	defer clientGDI.NewProc("DeleteObject").Call(bitmap)
	old, _, _ := clientGDI.NewProc("SelectObject").Call(memory, bitmap)
	if old == 0 || old == ^uintptr(0) {
		return nil, errors.New("select actual client capture bitmap failed")
	}
	defer clientGDI.NewProc("SelectObject").Call(memory, old)
	ok, _, callErr := clientGDI.NewProc("BitBlt").Call(memory, 0, 0, uintptr(width), uintptr(height), dc, 0, 0, 0x00cc0020) // SRCCOPY from own client, never the desktop.
	if ok == 0 {
		return nil, fmt.Errorf("copy actual own-client visible pixels: %v", callErr)
	}
	// CreateDIBSection memory access must follow a successful flush of this
	// locked worker thread's GDI batch; ReadProcessMemory is not a drawing flush.
	ok, _, callErr = clientGDI.NewProc("GdiFlush").Call()
	if ok == 0 {
		return nil, fmt.Errorf("flush actual own-client capture drawing: %v", callErr)
	}
	pixels := make([]byte, width*height*4)
	var copied uintptr
	ok, _, callErr = clientKernel.NewProc("ReadProcessMemory").Call(^uintptr(0), address, uintptr(unsafe.Pointer(&pixels[0])), uintptr(len(pixels)), uintptr(unsafe.Pointer(&copied)))
	if ok == 0 || copied != uintptr(len(pixels)) {
		return nil, fmt.Errorf("read actual owned capture bitmap: %v", callErr)
	}
	if err := owned(window, pid); err != nil {
		return nil, err
	}
	result := image.NewRGBA(image.Rect(0, 0, width, height))
	for offset := 0; offset < len(pixels); offset += 4 {
		result.Pix[offset], result.Pix[offset+1], result.Pix[offset+2], result.Pix[offset+3] = pixels[offset+2], pixels[offset+1], pixels[offset], 255
	}
	return result, nil
}
