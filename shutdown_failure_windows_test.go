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

func TestNativeWindowShutdownPropagatesActualFiveSecondFailure(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "shutdown-failure-native.exe")
	buildContext, stopBuild := context.WithTimeout(context.Background(), 2*time.Minute)
	defer stopBuild()
	build := exec.CommandContext(buildContext, "go", "build", "-buildvcs=false", "-race", "-o", exe, "./internal/shutdownfailure")
	build.WaitDelay = 3 * time.Second
	if data, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build actual final-window drain failure fixture: %v %s", err, data)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, exe, "-output", ".cache/shutdown-failure/integration")
	command.WaitDelay = 3 * time.Second
	data, err := command.CombinedOutput()
	if err != nil || !strings.Contains(string(data), "native shutdown failure contract passed: pending-real-frame, five-second Run error, completed/drop accounting, owned HWND closed") {
		t.Fatalf("actual pending fifth-frame shutdown/five-second error/MAX accounting/owned closure: %v %s", err, data)
	}
}
