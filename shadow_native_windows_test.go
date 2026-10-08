//go:build windows && cgo

package godesktop

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNativeShadowOwnedGPUPixelsAndLifecycle(t *testing.T) {
	root, err := filepath.Abs(filepath.Join(".cache", "shadow-native"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(root, "shadow-native.exe")
	build, stopBuild := context.WithTimeout(context.Background(), 2*time.Minute)
	defer stopBuild()
	if data, err := exec.CommandContext(build, "go", "build", "-buildvcs=false", "-race", "-o", exe, "./internal/shadownative").CombinedOutput(); err != nil {
		t.Fatalf("build native shadow: %v\n%s", err, data)
	}
	for _, adapter := range []string{"hardware", "warp"} {
		for _, density := range []string{"1", "1.5", "2"} {
			t.Run(adapter+"/"+density, func(t *testing.T) {
				ctx, stop := context.WithTimeout(context.Background(), 95*time.Second)
				defer stop()
				output := filepath.Join(root, fmt.Sprintf("%s-density%s", adapter, density))
				command := exec.CommandContext(ctx, exe, "-adapter", adapter, "-density", density, "-recovery", "-output", output)
				command.Env = append(os.Environ(), "GODESKTOP_READBACK=1", "GODESKTOP_TEST_INPUT_ISOLATION=1")
				command.WaitDelay = 3 * time.Second
				data, err := command.CombinedOutput()
				if err != nil || !strings.Contains(string(data), "native shadow GPU contract passed") {
					t.Fatalf("owned actual shadow %s/%s: %v\n%s", adapter, density, err, data)
				}
				t.Log(string(data))
			})
		}
	}
}
