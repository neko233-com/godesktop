//go:build windows && cgo

package main

import (
	"github.com/neko233-com/godesktop/testing/winprobe"
	"image"
	"os"
)

func capture() (*image.RGBA, float64, error) {
	w, err := winprobe.Find(title, uint32(os.Getpid()))
	if err != nil {
		return nil, 0, err
	}
	pixels, err := w.Capture()
	return pixels, float64(w.DPI()) / 96, err
}
