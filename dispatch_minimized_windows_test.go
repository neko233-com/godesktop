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

func TestNativeDispatchWhileMinimizedAndHidden(t *testing.T) {
	root := filepath.Join(".cache", "dispatch-minimized-native")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	exe, err := filepath.Abs(filepath.Join(root, "dispatch-minimized.exe"))
	if err != nil {
		t.Fatal(err)
	}
	buildCtx, stopBuild := context.WithTimeout(context.Background(), 2*time.Minute)
	defer stopBuild()
	if data, err := exec.CommandContext(buildCtx, "go", "build", "-buildvcs=false", "-race", "-o", exe, "./internal/dispatchtest").CombinedOutput(); err != nil {
		t.Fatalf("build owned dispatch fixture: %v\n%s", err, data)
	}
	ctx, stop := context.WithTimeout(context.Background(), 35*time.Second)
	defer stop()
	cmd := exec.CommandContext(ctx, exe)
	cmd.Env = append(os.Environ(), "GODESKTOP_READBACK=1", "GODESKTOP_TEST_INPUT_ISOLATION=1")
	cmd.WaitDelay = 3 * time.Second
	if data, err := cmd.CombinedOutput(); err != nil || !strings.Contains(string(data), "native minimized/hidden dispatch contract passed") {
		t.Fatalf("owned minimized dispatch: %v\n%s", err, data)
	}
}
