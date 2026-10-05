//go:build darwin && cgo

package platform

/*
#cgo CFLAGS: -x objective-c -fobjc-arc -mmacosx-version-min=13.0
#cgo LDFLAGS: -framework AppKit -framework Metal -framework MetalKit -framework CoreText -framework CoreGraphics -framework QuartzCore
*/
import "C"

import "runtime"

// Go initializes packages on the startup thread. Keep that thread for AppKit.
func init() { runtime.LockOSThread() }
