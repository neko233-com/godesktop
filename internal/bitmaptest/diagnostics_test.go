//go:build (windows || darwin) && cgo

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/neko233-com/godesktop/internal/platform"
)

func TestBitmapDiagnosticsTimeoutPreservesObservedValues(t *testing.T) {
	var calls atomic.Uint64
	d := startBitmapDiagnostics(func() platform.RenderStats {
		sequence := calls.Add(1)
		return platform.RenderStats{Backend: "metal", Submitted: sequence, Completed: sequence - 1, DeviceRecoveries: 1, FrameTicks: sequence + 7, BitmapUploads: 18}
	}, time.Millisecond)
	t.Cleanup(d.close)
	progress := bitmapProgress{Run: 0, Phase: 1, Index: 17, LastCompleted: 23, PhaseFrame: 12, Stage: "view-return", UIStatsAt: time.Now(), UIStats: platform.RenderStats{Submitted: 25, Completed: 23, DeviceRecoveries: 1}}
	d.record(progress)
	// The caller's subsequent state change must not mutate the published value.
	progress.Phase = 5
	progress.UIStats.Completed = 99
	deadline := time.After(time.Second)
	poll := time.NewTicker(time.Millisecond)
	defer poll.Stop()
	for d.native.Load() == nil {
		select {
		case <-deadline:
			t.Fatal("sampler did not publish its independent native counters")
		case <-poll.C:
		}
	}
	d.close()
	directory := t.TempDir()
	var stderr bytes.Buffer
	d.writeTimeout(directory, true, &stderr)
	lines := strings.Split(strings.TrimSpace(stderr.String()), "\n")
	if len(lines) != 2 || lines[0] != "bitmap native acceptance timed out" {
		t.Fatalf("watchdog output lost its error or real JSON: %q", stderr.String())
	}
	var report bitmapTimeoutReport
	if err := json.Unmarshal([]byte(lines[1]), &report); err != nil {
		t.Fatal(err)
	}
	if report.TimeoutSeconds != 40 || !report.Recovery || report.CapturedAt.IsZero() || report.Progress == nil || report.NativeSample == nil {
		t.Fatalf("incomplete timeout evidence: %+v", report)
	}
	got := report.Progress
	if got.Run != 0 || got.Phase != 1 || got.Index != 17 || got.LastCompleted != 23 || got.PhaseFrame != 12 || got.Stage != "view-return" || got.ObservedAt.IsZero() || got.UIStatsAt.IsZero() || got.UIStats.Completed != 23 {
		t.Fatalf("watchdog invented or changed UI progress: %+v", got)
	}
	if latest := d.native.Load(); !report.NativeSample.ObservedAt.Equal(latest.ObservedAt) || !reflect.DeepEqual(report.NativeSample.Stats, latest.Stats) || report.NativeSample.Stats.Submitted != calls.Load() || report.NativeSample.Stats.Completed+1 != calls.Load() {
		t.Fatal("watchdog did not retain the actual independently sampled native counters")
	}
	stored, err := os.ReadFile(filepath.Join(directory, "timeout.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(stored)) != lines[1] {
		t.Fatal("owned artifact and stderr report differ")
	}
}

func TestBitmapDiagnosticsShutdownJoinsInProgressSampler(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var enterOnce, releaseOnce sync.Once
	var calls atomic.Uint64
	d := startBitmapDiagnostics(func() platform.RenderStats {
		sequence := calls.Add(1)
		enterOnce.Do(func() { close(entered) })
		<-release
		return platform.RenderStats{Submitted: sequence, Completed: sequence}
	}, time.Millisecond)
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }); d.close() })
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("sampler never entered the native counter reader")
	}
	closed := make(chan struct{})
	go func() { d.close(); close(closed) }()
	select {
	case <-d.stop:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not signal the sampler")
	}
	select {
	case <-closed:
		t.Fatal("shutdown returned while the counter reader remained active")
	default:
	}
	releaseOnce.Do(func() { close(release) })
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not join the released sampler")
	}
	if calls.Load() != 1 || d.native.Load() == nil || d.native.Load().Stats.Completed != 1 {
		t.Fatal("sampler lost its receipt or started another read after shutdown")
	}
	d.close() // Idempotent cleanup also joins the already finished goroutine.
}

func TestBitmapDiagnosticsConcurrentPublicationAndTimeoutSnapshot(t *testing.T) {
	d := startBitmapDiagnostics(func() platform.RenderStats { return platform.RenderStats{Submitted: 9, Completed: 7} }, time.Millisecond)
	t.Cleanup(d.close)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		for i := range 256 {
			d.record(bitmapProgress{Run: 0, Phase: 1, Index: i, LastCompleted: uint64(i), Stage: "view-return"})
		}
	}()
	for range 256 {
		report := d.timeoutReport(false)
		if report.Progress != nil && (report.Progress.Index < 0 || report.Progress.Index > 255 || report.Progress.LastCompleted != uint64(report.Progress.Index)) {
			t.Fatal("timeout snapshot mixed fields from separate UI publications")
		}
		if _, err := json.Marshal(report); err != nil {
			t.Fatal(err)
		}
	}
	<-finished
	if got := d.timeoutReport(false).Progress; got == nil || got.Index != 255 || got.LastCompleted != 255 {
		t.Fatal("final publication disappeared")
	}
}
