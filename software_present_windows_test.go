//go:build windows && cgo

package godesktop

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNativeSoftwareNonDiagnosticClientPresentation(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "software-presentation-native.exe")
	buildContext, stopBuild := context.WithTimeout(context.Background(), 2*time.Minute)
	defer stopBuild()
	build := exec.CommandContext(buildContext, "go", "build", "-buildvcs=false", "-race", "-o", exe, "./internal/softwarepresent")
	build.WaitDelay = 3 * time.Second
	if data, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build own-client software presentation fixture: %v %s", err, data)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, exe, "-output", ".cache/software-presentation/integration")
	command.WaitDelay = 3 * time.Second
	data, err := command.CombinedOutput()
	if err != nil || !strings.Contains(string(data), "actual non-diagnostic committed-dib client presentation passed: runs=3") {
		t.Fatalf("real non-diagnostic own-HWND software client/paint/resize/idle/closure: %v %s", err, data)
	}
}
