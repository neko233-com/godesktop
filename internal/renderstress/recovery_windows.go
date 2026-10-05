//go:build windows

package main

import (
	"fmt"
	"image/png"
	"os"

	"github.com/neko233-com/godesktop/internal/winprobe"
)

func recoveryPixels(output string) error {
	window, err := winprobe.Find("godesktop native GPU stress", uint32(os.Getpid()))
	if err != nil {
		return err
	}
	pixel, err := window.Pixel(24, 24)
	if err != nil {
		return err
	}
	if pixel != 0x2663a3 && pixel != 0x319473 {
		return fmt.Errorf("recovered HWND GPU pixel is incorrect: %#x", pixel)
	}
	if output == "" {
		return nil
	}
	pixels, err := window.Capture()
	if err != nil {
		return err
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
