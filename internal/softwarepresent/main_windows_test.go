//go:build windows && cgo

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/neko233-com/godesktop/testing/winprobe"
)

func TestClientColorRefPreservesRGBChannels(t *testing.T) {
	for _, test := range [][2]uint32{{0, 0}, {0xff, 0xff0000}, {0xff0000, 0xff}, {0x996633, 0x336699}, {0x55aa33, 0x33aa55}} {
		if got := colorRefRGB(test[0]); got != test[1] {
			t.Errorf("COLORREF%#x -> RGB%#x, want%#x", test[0], got, test[1])
		}
	}
}

func TestClientCaptureRejectsUnownedHWNDWithoutDesktopRead(t *testing.T) {
	if _, err := captureClient(0, 42); err == nil {
		t.Fatal("desktop DC fallback was admitted")
	}
	if _, err := captureClient(winprobe.Window(^uintptr(0)), 0); err == nil {
		t.Fatal("missing owner was admitted")
	}
	if err := expose(0, 42); err == nil {
		t.Fatal("unowned desktop paint was admitted")
	}
}

func TestSoftwareOutputRejectsUnownedPathBeforeCreation(t *testing.T) {
	outside := filepath.Join(t.TempDir(), "unowned-output")
	if _, err := prepareOwnedOutput(outside); err == nil {
		t.Fatal("unowned output directory was admitted")
	}
	if _, err := os.Stat(outside); !os.IsNotExist(err) {
		t.Fatal("rejected unowned output was created", err)
	}
}
