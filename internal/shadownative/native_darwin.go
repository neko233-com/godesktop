//go:build darwin && cgo

package main

import (
	"context"
	"fmt"
	"image"
	"os"

	ui "github.com/neko233-com/godesktop"
	"github.com/neko233-com/godesktop/internal/platform"
	"github.com/neko233-com/godesktop/testing/metalprobe"
)

func configureNative(density float64, _ string, recovery bool) error {
	for key, value := range map[string]string{"GODESKTOP_READBACK": "1", "GODESKTOP_TEST_DRAWABLE_SCALE": fmt.Sprint(density), "GODESKTOP_TEST_METAL_RECOVERY": "0", "GODESKTOP_TEST_METAL_RECOVERIES": "1"} {
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}
	if recovery {
		return os.Setenv("GODESKTOP_TEST_METAL_RECOVERY", "8")
	}
	return nil
}
func nativeAdapterPolicy(_ string) string { return "native-Metal-device" }
func validateNativePresentation(stats platform.RenderStats, presentation string) error {
	if presentation != "metal" || stats.Backend != "metal" || (stats.FrameClock != "cametaldisplaylink" && stats.FrameClock != "mtkview") {
		return fmt.Errorf("actual Metal presentation/clock mismatch: %s %+v", presentation, stats)
	}
	return nil
}
func nativePresentation(_ string, stats platform.RenderStats) (string, error) {
	return "metal", validateNativePresentation(stats, "metal")
}
func snapshotNative(_ string) (*image.RGBA, float64, uint32, error) {
	pixels, _, err := metalprobe.Snapshot()
	if err != nil {
		return nil, 0, 0, err
	}
	return pixels, float64(pixels.Bounds().Dx()) / 480, 0, nil
}
func pointerNative(ctx context.Context, cx *ui.Context, _ string, _ float64, x, y float32) error {
	receipt := make(chan error, 1)
	if !cx.Dispatch(func() {
		err := metalprobe.Pointer(true, x, y, 0)
		if err == nil {
			err = metalprobe.Pointer(false, x, y, 0)
		}
		receipt <- err
	}) {
		return fmt.Errorf("owned Metal pointer UI dispatch rejected")
	}
	select {
	case err := <-receipt:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
