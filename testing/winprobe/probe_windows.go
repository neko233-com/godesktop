//go:build windows

// Package winprobe provides process-owned Win32 window probes for integration
// tests. Find always verifies the expected PID before returning a handle.
package winprobe

import internal "github.com/neko233-com/godesktop/internal/winprobe"

type Window = internal.Window

var Find = internal.Find
var Awareness = internal.Awareness
var ValidateAMD64PE = internal.ValidateAMD64PE
