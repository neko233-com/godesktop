//go:build windows

package winprobe

import (
	"fmt"
	"image"
	"math"
	"syscall"
	"unsafe"
)

var gpuKernel = syscall.NewLazyDLL("kernel32.dll")

const gpuCaptureCapacity = 64 * 1024 * 1024

type gpuHeader struct {
	Magic, Version, PID, Width, Height, Stride uint32
	Reserved                                   [2]uint32
	HWND, Sequence, Frame, Bytes               uint64
}

// withGPUReadback consumes a test-enabled, fenced GPU copy from exactly this
// process-owned HWND. It never substitutes GDI/desktop pixels for GPU output.
func (w Window) withGPUReadback(read func(gpuHeader, []byte) error) error {
	property, err := syscall.UTF16PtrFromString("godesktop.backend")
	if err != nil {
		return err
	}
	backend, _, _ := user.NewProc("GetPropW").Call(uintptr(w), uintptr(unsafe.Pointer(property)))
	if backend != 3 {
		return fmt.Errorf("expected D3D12 HWND, backend=%d", backend)
	}
	var pid uint32
	user.NewProc("GetWindowThreadProcessId").Call(uintptr(w), uintptr(unsafe.Pointer(&pid)))
	if pid == 0 {
		return fmt.Errorf("GPU capture HWND is no longer owned")
	}
	name, err := syscall.UTF16PtrFromString(fmt.Sprintf("Local\\godesktop.gpu.%d.%d", pid, uintptr(w)))
	if err != nil {
		return err
	}
	handle, _, callErr := gpuKernel.NewProc("OpenFileMappingW").Call(4, 0, uintptr(unsafe.Pointer(name)))
	if handle == 0 {
		return fmt.Errorf("GPU diagnostic mapping unavailable: %v", callErr)
	}
	defer gpuKernel.NewProc("CloseHandle").Call(handle)
	view, _, callErr := gpuKernel.NewProc("MapViewOfFile").Call(handle, 4, 0, 0, gpuCaptureCapacity+64)
	if view == 0 {
		return fmt.Errorf("Map GPU readback: %v", callErr)
	}
	defer gpuKernel.NewProc("UnmapViewOfFile").Call(view)
	// Copy mapped native memory with ReadProcessMemory rather than converting
	// a syscall-returned address to a Go pointer. Both header reads bracket the
	// pixel copy so the producer's sequence still rejects a concurrent update.
	copyNative := func(address uintptr, target unsafe.Pointer, size uintptr) error {
		var copied uintptr
		ok, _, callErr := gpuKernel.NewProc("ReadProcessMemory").Call(^uintptr(0), address, uintptr(target), size, uintptr(unsafe.Pointer(&copied)))
		if ok == 0 || copied != size {
			return fmt.Errorf("copy GPU diagnostic memory: %v", callErr)
		}
		return nil
	}
	for range 5 {
		var header gpuHeader
		if err := copyNative(view, unsafe.Pointer(&header), unsafe.Sizeof(header)); err != nil {
			return err
		}
		if header.Sequence&1 != 0 {
			continue
		}
		if header.Magic != 0x32314447 || header.Version != 1 || header.PID != pid || header.HWND != uint64(w) || header.Frame == 0 {
			return fmt.Errorf("GPU readback identity/frame is not ready")
		}
		bytes := uint64(header.Width) * uint64(header.Height) * 4
		if header.Width == 0 || header.Height == 0 || header.Stride != header.Width*4 || bytes > gpuCaptureCapacity || header.Bytes != bytes {
			return fmt.Errorf("invalid GPU readback dimensions")
		}
		width, height, err := w.ClientSize()
		if err != nil {
			return err
		}
		if width != int(header.Width) || height != int(header.Height) {
			return fmt.Errorf("GPU readback has not caught up with window resize")
		}
		pixels := make([]byte, int(bytes))
		if err := copyNative(view+64, unsafe.Pointer(&pixels[0]), uintptr(bytes)); err != nil {
			return err
		}
		readErr := read(header, pixels)
		var after uint64
		if err := copyNative(view+40, unsafe.Pointer(&after), unsafe.Sizeof(after)); err != nil {
			return err
		}
		if header.Sequence == after {
			return readErr
		}
	}
	return fmt.Errorf("GPU readback changed during capture")
}

func (w Window) gpuPixel(x, y int) (uint32, error) {
	scale := float64(w.DPI()) / 96
	px, py := int(math.Round(float64(x)*scale)), int(math.Round(float64(y)*scale))
	var value uint32
	err := w.withGPUReadback(func(header gpuHeader, pixels []byte) error {
		if px < 0 || py < 0 || px >= int(header.Width) || py >= int(header.Height) {
			return fmt.Errorf("GPU pixel outside client bounds")
		}
		offset := py*int(header.Stride) + px*4
		value = uint32(pixels[offset+2])<<16 | uint32(pixels[offset+1])<<8 | uint32(pixels[offset])
		return nil
	})
	return value, err
}

func (w Window) gpuCapture() (*image.RGBA, error) {
	var captured *image.RGBA
	err := w.withGPUReadback(func(header gpuHeader, pixels []byte) error {
		captured = image.NewRGBA(image.Rect(0, 0, int(header.Width), int(header.Height)))
		copy(captured.Pix, pixels)
		return nil
	})
	if err != nil {
		return nil, err
	}
	for i := 0; i < len(captured.Pix); i += 4 {
		captured.Pix[i], captured.Pix[i+2] = captured.Pix[i+2], captured.Pix[i]
	}
	return captured, nil
}
