//go:build windows && cgo

package godesktop

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/neko233-com/godesktop/internal/winprobe"
)

func TestNativeOSCloseDefersAndConfirms(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	restore := winprobe.Awareness()
	defer restore()
	exe := filepath.Join(t.TempDir(), "closeguard.exe")
	build := exec.Command("go", "build", "-race", "-cover", "-coverpkg=github.com/neko233-com/godesktop,github.com/neko233-com/godesktop/internal/platform", "-o", exe, "./internal/closeguard")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build %v %s", err, output)
	}
	var output lockedOutput
	command := exec.Command(exe, "-mode", "os")
	command.Env = append(os.Environ(), "GOCOVERDIR="+os.Getenv("GODESKTOP_NATIVE_COVERDIR"))
	command.Stdout, command.Stderr = &output, &output
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = command.Process.Kill() })
	poll := func(condition func() bool) {
		t.Helper()
		limit := time.Now().Add(10 * time.Second)
		for time.Now().Before(limit) {
			if condition() {
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatal("owned native close condition timed out: " + output.String())
	}
	var window winprobe.Window
	poll(func() bool {
		var err error
		window, err = winprobe.Find("godesktop close guard", uint32(command.Process.Pid))
		return err == nil
	})
	mustNative(t, window.Close())
	poll(func() bool { return strings.Contains(output.String(), "close request 1 allow=false") })
	if _, _, err := window.ClientSize(); err != nil {
		t.Fatalf("rejected WM_CLOSE destroyed the window: %v", err)
	}
	// Native keyboard focus and Enter explicitly confirm the application's choice.
	mustNative(t, window.Send(0x100, 9, 0))
	mustNative(t, window.Send(0x100, 13, 0))
	if err := command.Wait(); err != nil {
		t.Fatalf("close %v %s", err, output.String())
	}
	if !strings.Contains(output.String(), "close request 2 allow=true") || !strings.Contains(output.String(), "native close guard acceptance passed os") || strings.Contains(output.String(), "DATA RACE") {
		t.Fatal(output.String())
	}
	for _, mode := range []string{"force", "panic"} {
		command = exec.Command(exe, "-mode", mode)
		command.Env = append(os.Environ(), "GOCOVERDIR="+os.Getenv("GODESKTOP_NATIVE_COVERDIR"))
		if data, err := command.CombinedOutput(); err != nil || !strings.Contains(string(data), "native close guard acceptance passed "+mode) {
			t.Fatalf("%s %v %s", mode, err, data)
		}
	}
}
