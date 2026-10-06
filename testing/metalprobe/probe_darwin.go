//go:build darwin && cgo

// Package metalprobe provides opt-in, process-owned Metal drawable readback for
// native acceptance tests. Set GODESKTOP_READBACK=1 before starting the window.
package metalprobe

import (
	"image"

	"github.com/neko233-com/godesktop/internal/platform"
)

// Snapshot copies the latest completed drawable and its submission number.
// It reads this process's GPU output, including when its window is occluded;
// no desktop/window-server screenshot or screen-recording permission is used.
// Before a readable completed frame exists it returns an error.
func Snapshot() (*image.RGBA, uint64, error) { return platform.MetalSnapshot() }
