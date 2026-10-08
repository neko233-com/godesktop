//go:build windows && cgo

package main

import (
	"context"
	"fmt"
	"image"
	"math"
	"os"

	ui "github.com/neko233-com/godesktop"
	"github.com/neko233-com/godesktop/internal/platform"
	"github.com/neko233-com/godesktop/testing/winprobe"
)

func configureNative(density float64, adapter string, recovery bool) error {
	if adapter != "hardware" && adapter != "warp" {
		return fmt.Errorf("invalid adapter %q", adapter)
	}
	for key, value := range map[string]string{"GODESKTOP_READBACK": "1", "GODESKTOP_TEST_INPUT_ISOLATION": "1", "GODESKTOP_GPU_ADAPTER": adapter, "GODESKTOP_TEST_DRAWABLE_SCALE": fmt.Sprint(density), "GODESKTOP_TEST_DEVICE_REMOVAL": "0"} {
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}
	if adapter == "hardware" {
		if err := os.Setenv("GODESKTOP_GPU_ADAPTER", ""); err != nil {
			return err
		}
	}
	if err := os.Setenv("GODESKTOP_SHADOW_ADAPTER_POLICY", adapter); err != nil {
		return err
	}
	if recovery {
		return os.Setenv("GODESKTOP_TEST_DEVICE_REMOVAL", "8")
	}
	return nil
}
func nativeAdapterPolicy(adapter string) string {
	if adapter == "warp" {
		return "forced-WARP"
	}
	return "hardware-preferred-with-software-fallback"
}
func validateNativePresentation(stats platform.RenderStats, presentation string) error {
	return winprobe.ValidateWindowsPresentation(stats, presentation)
}
func nativePresentation(title string, stats platform.RenderStats) (string, error) {
	pid := uint32(os.Getpid())
	window, err := winprobe.Find(title, pid)
	if err != nil {
		return "", err
	}
	presentation, err := winprobe.NativePresentation(window, pid)
	if err != nil {
		return "", err
	}
	if os.Getenv("GODESKTOP_SHADOW_ADAPTER_POLICY") == "warp" && presentation != "committed-dib" {
		return "", fmt.Errorf("forced WARP must use actual committed-dib presentation, got %s", presentation)
	}
	return presentation, validateNativePresentation(stats, presentation)
}
func snapshotNative(title string) (*image.RGBA, float64, uint32, error) {
	w, err := winprobe.Find(title, uint32(os.Getpid()))
	if err != nil {
		return nil, 0, 0, err
	}
	pixels, err := w.Capture()
	if err != nil {
		return nil, 0, 0, err
	}
	return pixels, float64(pixels.Bounds().Dx()) / 480, w.DPI(), nil
}
func pointerNative(_ context.Context, _ *ui.Context, title string, density float64, x, y float32) error {
	w, err := winprobe.Find(title, uint32(os.Getpid()))
	if err != nil {
		return err
	}
	// Diagnostic GPU density can differ from OS DPI. Replay client pixels for
	// the renderer's real scale; never move the pointer or send global input.
	px, py := int16(math.Round(float64(x)*density)), int16(math.Round(float64(y)*density))
	parameter := uintptr(uint16(px)) | uintptr(uint16(py))<<16
	if err := w.Send(0x201, 1, parameter); err != nil {
		return err
	}
	return w.Send(0x202, 0, parameter)
}
