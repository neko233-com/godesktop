//go:build windows && cgo

package godesktop

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestNativeRoundedDescendantClipping(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "rounded-native.exe")
	if data, err := exec.Command("go", "build", "-race", "-o", exe, "./internal/roundedtest").CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, data)
	}
	if data, err := exec.Command(exe).CombinedOutput(); err != nil {
		t.Fatalf("actual rounded clipping: %v %s", err, data)
	}
}
