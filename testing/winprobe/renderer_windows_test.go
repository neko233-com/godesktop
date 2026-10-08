//go:build windows

package winprobe

import "testing"

func TestNativePresentationRejectsMissingOwnerWithoutWindow(t *testing.T) {
	// Both short-circuit before any syscall: this is CPU admission testing, not
	// a native-window fixture or an assertion about actual GPU presentation.
	for _, test := range []struct {
		window Window
		pid    uint32
	}{{0, 42}, {Window(^uintptr(0)), 0}} {
		if got, err := NativePresentation(test.window, test.pid); err == nil || got != "" {
			t.Fatal("unowned HWND admission", got, err)
		}
	}
}
