//go:build windows && cgo

package godesktop

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNativeWindowActivationAndCaptureCancellation(t *testing.T) {
	// Repeated acceptance overwrites one owned executable instead of retaining
	// a different build/report directory on each run. The fixture has no disk
	// workspace or child process, and CommandContext kills only this process.
	root := filepath.Join(".cache", "window-focus-native")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	exe, err := filepath.Abs(filepath.Join(root, "window-focus.exe"))
	if err != nil {
		t.Fatal(err)
	}
	buildContext, stopBuild := context.WithTimeout(context.Background(), 2*time.Minute)
	defer stopBuild()
	if output, err := exec.CommandContext(buildContext, "go", "build", "-buildvcs=false", "-race", "-o", exe, "./internal/focustest").CombinedOutput(); err != nil {
		t.Fatalf("build native focus fixture: %v\n%s", err, output)
	}
	ctx, stop := context.WithTimeout(context.Background(), 20*time.Second)
	defer stop()
	command := exec.CommandContext(ctx, exe)
	command.Env = append(os.Environ(), "GODESKTOP_TEST_INPUT_ISOLATION=1")
	command.WaitDelay = 3 * time.Second
	if output, err := command.CombinedOutput(); err != nil || !strings.Contains(string(output), "native window activation event contract passed") {
		t.Fatalf("native focus: %v\n%s", err, output)
	}
}
