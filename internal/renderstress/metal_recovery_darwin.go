//go:build darwin && cgo

package main

import (
	"fmt"
	"image/png"
	"math"
	"os"

	"github.com/neko233-com/godesktop/internal/platform"
)

func metalWindowIdentity() uint64 { return platform.MetalWindowIdentity() }

func metalRecoveryPixels(output string) error {
	pixels, frame, err := platform.MetalSnapshot()
	if err != nil {
		return err
	}
	stats := platform.RendererStats()
	if frame != stats.Submitted || stats.InFlight != 0 {
		return fmt.Errorf("recovered Metal snapshot is not the latest completed actual drawable: frame=%d renderer=%+v", frame, stats)
	}
	scale := float64(pixels.Bounds().Dx()) / 1000
	pixel := pixels.RGBAAt(int(math.Round(24*scale)), int(math.Round(24*scale)))
	color := uint32(pixel.R)<<16 | uint32(pixel.G)<<8 | uint32(pixel.B)
	if color != 0x2663a3 && color != 0x319473 {
		return fmt.Errorf("recovered Metal drawable pixel is incorrect: %#x", color)
	}
	if output == "" {
		return nil
	}
	file, err := os.Create(output + ".png")
	if err != nil {
		return err
	}
	err = png.Encode(file, pixels)
	closeErr := file.Close()
	if err != nil {
		return err
	}
	return closeErr
}
