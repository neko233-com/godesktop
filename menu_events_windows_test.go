//go:build windows && cgo

package godesktop

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeWin32MenuEvents(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "menu-events.exe")
	if output, err := exec.Command("go", "build", "-race", "-o", exe, "./internal/inputtest").CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, output)
	}
	if output, err := exec.Command(exe).CombinedOutput(); err != nil || !strings.Contains(string(output), "event contract passed") {
		t.Fatalf("native menu: %v %s", err, output)
	}
}
