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

// Wheel constructs an NSEvent for the owned readback-enabled view, then invokes
// its native scroll handler. Call on the UI thread. dx/dy are native AppKit
// deltas: positive x moves toward earlier/left content; positive y moves up.
// precise selects pixel units, normalized by the renderer to lines. This probe
// never posts global input or manipulates the user's pointer.
func Wheel(dx, dy int, x, y float32, modifiers int, precise bool) error {
	return platform.MetalTestWheel(dx, dy, x, y, modifiers, precise)
}

// Pointer constructs an owned-window NSEvent and invokes the native view's
// press/release handler. Call on the UI thread with readback enabled. It never
// posts global input or moves the user's pointer.
func Pointer(pressed bool, x, y float32, modifiers int) error {
	return platform.MetalTestPointer(pressed, x, y, modifiers)
}

// Key constructs an owned NSEvent for common public key codes (Tab/PageUp/Down,
// arrows, ASCII and modifiers). It exercises keyUp/flagsChanged as well as press.
// Call on the UI thread with readback enabled; no global input is posted.
func Key(key, modifiers int, pressed bool) error {
	return platform.MetalTestKey(key, modifiers, pressed)
}
