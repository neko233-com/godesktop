//go:build darwin && cgo

package main

import (
	"image"

	ui "github.com/neko233-com/godesktop"
	"github.com/neko233-com/godesktop/testing/metalprobe"
)

func capture() (*image.RGBA, float64, error) {
	p, _, err := metalprobe.Snapshot()
	if err != nil {
		return nil, 0, err
	}
	return p, float64(p.Bounds().Dx()) / 350, nil
}
func injectWheel(cx *ui.Context, done func(error)) {
	cx.Dispatch(func() { done(metalprobe.Wheel(-36, 0, 30, 30, ui.ModifierShift, true)) })
}
func injectClick(cx *ui.Context, done func(error)) {
	cx.Dispatch(func() {
		err := metalprobe.Pointer(true, 30, 30, 0)
		if err == nil {
			err = metalprobe.Pointer(false, 30, 30, 0)
		}
		done(err)
	})
}

func injectRelease(cx *ui.Context, done func(error)) {
	cx.Dispatch(func() { done(metalprobe.Key(17, 0, false)) })
}
