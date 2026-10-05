//go:build windows && cgo

package godesktop

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/neko233-com/godesktop/internal/testprotocol"
	"github.com/neko233-com/godesktop/internal/winprobe"
)

type lockedOutput struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (b *lockedOutput) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(p)
}
func (b *lockedOutput) String() string { b.mu.Lock(); defer b.mu.Unlock(); return b.buffer.String() }

type nativeProcess struct {
	command *exec.Cmd
	input   io.WriteCloser
	reports chan testprotocol.Report
	done    chan error
	errors  chan error
	stderr  lockedOutput
}

func startNative(t *testing.T, executable, coverDir, scenario string, runs int) *nativeProcess {
	t.Helper()
	p := &nativeProcess{reports: make(chan testprotocol.Report, 128), done: make(chan error, 1), errors: make(chan error, 1)}
	p.command = exec.Command(executable, "-scenario", scenario, "-runs", fmt.Sprint(runs))
	p.command.Env = append(os.Environ(), "GOCOVERDIR="+coverDir, "GODESKTOP_READBACK=1")
	p.command.Stderr = &p.stderr
	var err error
	p.input, err = p.command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := p.command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = p.command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.input.Close(); _ = p.command.Process.Kill() })
	go func() {
		defer func() { close(p.reports); p.done <- p.command.Wait() }()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			var report testprotocol.Report
			if err := json.Unmarshal(scanner.Bytes(), &report); err != nil {
				p.errors <- fmt.Errorf("fixture protocol %q: %w", scanner.Text(), err)
				return
			}
			p.reports <- report
		}
		if err := scanner.Err(); err != nil {
			p.errors <- err
		}
	}()
	return p
}

func (p *nativeProcess) await(t *testing.T, predicate func(testprotocol.Report) bool) testprotocol.Report {
	t.Helper()
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	for {
		select {
		case report, ok := <-p.reports:
			if !ok {
				t.Fatalf("fixture exited before expected report; stderr: %s", p.stderr.String())
			}
			if predicate(report) {
				return report
			}
		case err := <-p.errors:
			t.Fatal(err)
		case <-timer.C:
			t.Fatalf("fixture report timeout; stderr: %s", p.stderr.String())
		}
	}
}

func (p *nativeProcess) snapshot(t *testing.T, run int) testprotocol.Report {
	t.Helper()
	if _, err := io.WriteString(p.input, "probe\n"); err != nil {
		t.Fatal(err)
	}
	return p.await(t, func(r testprotocol.Report) bool { return r.Event == "probe" && r.Run == run })
}

func (p *nativeProcess) exit(t *testing.T) {
	t.Helper()
	select {
	case err := <-p.done:
		if err != nil {
			t.Fatalf("fixture process: %v\n%s", err, p.stderr.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("fixture did not exit: %s", p.stderr.String())
	}
	if text := p.stderr.String(); strings.Contains(text, "DATA RACE") {
		t.Fatalf("fixture race: %s", text)
	}
}

func mustNative(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func fixtureWindow(t *testing.T, p *nativeProcess, run int) winprobe.Window {
	t.Helper()
	window, err := winprobe.Find(fmt.Sprintf("godesktop-test-%d-%d", p.command.Process.Pid, run), uint32(p.command.Process.Pid))
	mustNative(t, err)
	if dpi := window.DPI(); dpi < 96 || dpi > 768 {
		t.Fatalf("invalid DPI %d", dpi)
	}
	return window
}

func assertClicks(t *testing.T, p *nativeProcess, run int, want [2]int) {
	t.Helper()
	if got := p.snapshot(t, run).Clicks; got != want {
		t.Fatalf("clicks: got %v want %v", got, want)
	}
}

func click(t *testing.T, window winprobe.Window, x, y int) {
	t.Helper()
	mustNative(t, window.Pointer(0x201, x, y))
	mustNative(t, window.Pointer(0x202, x, y))
}

func assertPixel(t *testing.T, window winprobe.Window, x, y int, want uint32) {
	t.Helper()
	mustNative(t, window.Send(0x0f, 0, 0)) // WM_PAINT is synchronous, including Direct2D EndDraw.
	mustNative(t, window.Send(0, 0, 0))
	got, err := window.Pixel(x, y)
	mustNative(t, err)
	for _, shift := range []uint{0, 8, 16} {
		if math.Abs(float64(int(got>>shift&255)-int(want>>shift&255))) > 3 {
			t.Fatalf("pixel (%d,%d): #%06x want #%06x (DPI %d)", x, y, got, want, window.DPI())
		}
	}
}

func TestWindowsAMD64NativeIntegration(t *testing.T) {
	if runtime.GOARCH != "amd64" {
		t.Skip("supported Windows target is amd64")
	}
	executable := filepath.Join(t.TempDir(), "native-fixture.exe")
	build := exec.Command("go", "build", "-race", "-cover", "-coverpkg=github.com/neko233-com/godesktop,github.com/neko233-com/godesktop/internal/platform,github.com/neko233-com/godesktop/internal/testapp", "-o", executable, "./internal/testapp")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("fixture build: %v\n%s", err, output)
	}
	coverDir := os.Getenv("GODESKTOP_NATIVE_COVERDIR")
	if coverDir == "" {
		coverDir = t.TempDir()
	}
	if err := os.MkdirAll(coverDir, 0700); err != nil {
		t.Fatal(err)
	}
	t.Run("64bitPEAndSystemDependencies", func(t *testing.T) { mustNative(t, winprobe.ValidateAMD64PE(executable)) })
	t.Run("customWindowFitsDesktopAndKeepsFooterVisible", func(t *testing.T) {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		restore := winprobe.Awareness()
		defer restore()
		p := startNative(t, executable, coverDir, "custom-desktop", 1)
		p.await(t, func(r testprotocol.Report) bool { return r.Event == "frame" })
		w := fixtureWindow(t, p, 0)
		work, err := winprobe.WorkArea()
		mustNative(t, err)
		bounds, err := w.ScreenBounds()
		mustNative(t, err)
		if bounds.Left < work.Left || bounds.Top < work.Top || bounds.Right > work.Right || bounds.Bottom > work.Bottom {
			t.Fatalf("window %+v exceeds work area %+v", bounds, work)
		}
		_, height, err := w.ClientSize()
		mustNative(t, err)
		dipHeight := float64(height) * 96 / float64(w.DPI())
		assertPixel(t, w, 10, int(dipHeight)-10, 0x0078d4)
		mustNative(t, w.Close())
		p.await(t, func(r testprotocol.Report) bool { return r.Event == "closed" })
		p.exit(t)
	})
	t.Run("realWindowInputPixelsAndSequentialRuns", func(t *testing.T) {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		restore := winprobe.Awareness()
		defer restore()
		p := startNative(t, executable, coverDir, "interactive", 3)
		for run := 0; run < 3; run++ {
			ready := p.await(t, func(r testprotocol.Report) bool { return r.Event == "frame" && r.Run == run && r.Async == 128 })
			for i, metric := range ready.Metrics {
				if metric[1] <= 0 || (i > 0 && metric[0] <= 0) {
					t.Fatalf("native Unicode metric %d: %v", i, metric)
				}
			}
			window := fixtureWindow(t, p, run)
			assertPixel(t, window, 40, 80, 0xff0000)
			assertPixel(t, window, 100, 80, 0x081098)
			assertPixel(t, window, 130, 80, 0x102030)
			click(t, window, 50, 30)
			assertClicks(t, p, run, [2]int{1, 0})
			mustNative(t, window.Pointer(0x201, 50, 30))
			mustNative(t, window.Pointer(0x202, 200, 30))
			assertClicks(t, p, run, [2]int{1, 0})
			mustNative(t, window.Pointer(0x201, 50, 30))
			mustNative(t, window.Pointer(0x202, -10, -10))
			assertClicks(t, p, run, [2]int{1, 0})
			mustNative(t, window.Pointer(0x201, 50, 30))
			mustNative(t, window.Send(0x8, 0, 0)) // WM_KILLFOCUS cancels capture.
			mustNative(t, window.Pointer(0x202, 50, 30))
			assertClicks(t, p, run, [2]int{1, 0})
			click(t, window, 360, 30) // Disabled button must not activate.
			assertClicks(t, p, run, [2]int{1, 0})
			mustNative(t, window.Send(0x100, 9, 0)) // Tab: primary -> secondary.
			mustNative(t, window.Send(0x100, 13, 0))
			mustNative(t, window.Send(0x100, 13, 1<<30)) // Suppress repeated keydown.
			assertClicks(t, p, run, [2]int{1, 1})
			mustNative(t, window.Send(0x100, 9, 0)) // Wrap, skipping disabled.
			mustNative(t, window.Send(0x100, 32, 0))
			assertClicks(t, p, run, [2]int{2, 1})
			mustNative(t, window.Resize(650, 360))
			p.snapshot(t, run)
			width, height, err := window.ClientSize()
			mustNative(t, err)
			scale := float64(window.DPI()) / 96
			if math.Abs(float64(width)/scale-650) > 1 || math.Abs(float64(height)/scale-360) > 1 {
				t.Fatalf("resized client: %dx%d @%d DPI", width, height, window.DPI())
			}
			assertPixel(t, window, 130, 80, 0x102030)
			window.Show(6) // Minimize.
			window.Show(9) // Restore.
			click(t, window, 200, 30)
			assertClicks(t, p, run, [2]int{2, 2})
			mustNative(t, window.Close())
			closed := p.await(t, func(r testprotocol.Report) bool { return r.Event == "closed" && r.Run == run })
			if closed.Error != "" || !closed.Closed || closed.NativeFrames < 5 {
				t.Fatalf("shutdown: %+v", closed)
			}
		}
		p.exit(t)
	})
	for _, scenario := range []struct{ name, error string }{{"panic-view", "fixture-view-panic"}, {"panic-dispatch", "fixture-dispatch-panic"}, {"panic-click", "fixture-click-panic"}, {"duplicate", "duplicate button key"}, {"reentrant", ""}, {"empty", ""}, {"quit", ""}} {
		t.Run(scenario.name, func(t *testing.T) {
			p := startNative(t, executable, coverDir, scenario.name, 1)
			if scenario.name == "panic-click" || scenario.name == "reentrant" || scenario.name == "empty" {
				ready := p.await(t, func(r testprotocol.Report) bool { return r.Event == "frame" })
				window := fixtureWindow(t, p, 0)
				if scenario.name == "panic-click" {
					click(t, window, 50, 30)
				} else {
					if scenario.name == "reentrant" && !strings.Contains(ready.Guard, "already running") {
						t.Fatalf("nested run: %q", ready.Guard)
					}
					mustNative(t, window.Close())
				}
			}
			closed := p.await(t, func(r testprotocol.Report) bool { return r.Event == "closed" })
			if !closed.Closed || (scenario.error == "" && closed.Error != "") || (scenario.error != "" && !strings.Contains(closed.Error, scenario.error)) {
				t.Fatalf("scenario %s: %+v", scenario.name, closed)
			}
			p.exit(t)
		})
	}
}
