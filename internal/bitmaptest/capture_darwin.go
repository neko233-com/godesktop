//go:build darwin && cgo

package main

import (
	"github.com/neko233-com/godesktop/internal/platform"
	"image"
)

func capture() (*image.RGBA, float64, error) {
	pixels, _, err := platform.MetalSnapshot()
	if err != nil {
		return nil, 0, err
	}
	return pixels, float64(pixels.Bounds().Dx()) / 500, err
}
