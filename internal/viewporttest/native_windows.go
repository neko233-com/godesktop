//go:build windows && cgo

package main

import (
	"fmt"
	"image"
	"os"
	"syscall"

	ui "github.com/neko233-com/godesktop"
	"github.com/neko233-com/godesktop/testing/winprobe"
)

func capture() (*image.RGBA, float64, error) {
	w, err := winprobe.Find(title, uint32(os.Getpid()))
	if err != nil {
		return nil, 0, err
	}
	p, err := w.Capture()
	return p, float64(w.DPI()) / 96, err
}
func injectWheel(cx *ui.Context, done func(error)) {
	go func() {
		w, err := winprobe.Find(title, uint32(os.Getpid()))
		if err == nil {
			// Keep only this owned fixture partly offscreen so wheel payloads have
			// negative physical screen coordinates. GPU readback remains process-owned.
			ok, _, moveErr := syscall.NewLazyDLL("user32.dll").NewProc("SetWindowPos").Call(uintptr(w), 0, ^uintptr(119), ^uintptr(99), 0, 0, 0x1|0x4|0x10)
			if ok == 0 {
				err = fmt.Errorf("owned window negative-position setup: %w", moveErr)
			}
			if err == nil {
				bounds, e := w.ClientBounds()
				if e != nil {
					err = e
				} else if bounds.Left >= 0 || bounds.Top >= 0 {
					err = fmt.Errorf("owned wheel fixture was not at negative screen coordinates: %+v", bounds)
				}
			}
			if err == nil {
				err = w.Wheel(true, 30, 30, 120, 4)
			}
		}
		cx.Dispatch(func() { done(err) })
	}()
}
func injectClick(cx *ui.Context, done func(error)) {
	go func() {
		w, err := winprobe.Find(title, uint32(os.Getpid()))
		if err == nil {
			err = w.Pointer(0x201, 30, 30)
		}
		if err == nil {
			err = w.Pointer(0x202, 30, 30)
		}
		cx.Dispatch(func() { done(err) })
	}()
}

func injectRelease(cx *ui.Context, done func(error)) {
	go func() {
		w, err := winprobe.Find(title, uint32(os.Getpid()))
		if err == nil {
			err = w.Send(0x101, 17, 0)
		}
		cx.Dispatch(func() { done(err) })
	}()
}
