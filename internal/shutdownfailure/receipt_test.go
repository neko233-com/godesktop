package main

import (
	"math"
	"testing"

	"github.com/neko233-com/godesktop/testing/winprobe"
)

// This synthetic receipt corpus only tests rejection at the evidence boundary.
// It is not an actual native timeout, removal or resource-ordering test.
func goodReceipt() report {
	pending := winprobe.RenderStats{Backend: "direct3d12", FrameClock: "d3d12-fence", FrameSlots: 3, UsedSlotsMask: 7, InFlight: 1, MaxInFlight: 2, Submitted: 5, Completed: 4, DrawCalls: 1, Instances: 1, UploadedBytes: 160}
	closed := pending
	closed.InFlight, closed.DroppedFrames = 0, 1
	return report{Schema: 1, PID: 42, HWND: 1, DPI: 144, Presentation: "committed-dib", ShutdownPending: true, Pending: pending, Closed: closed, RunError: timeoutText, QuitToRunMS: 5000, WorkerJoined: true, WindowDestroyed: true, OldRejected: true}
}

func TestShutdownFailureRequiresActualFirstErrorAndFrozenCompletion(t *testing.T) {
	if err := validateReport(goodReceipt(), 42); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		edit func(*report)
	}{
		{"old-process", func(r *report) { r.PID = 43 }},
		{"no-window", func(r *report) { r.HWND = 0 }},
		{"wrong-presentation", func(r *report) { r.Presentation = "dxgi" }},
		{"wrong-clock", func(r *report) { r.Pending.FrameClock = "dxgi" }},
		{"missing-real-hold-cause", func(r *report) { r.ShutdownPending = false }},
		{"four-submissions-only", func(r *report) { r.Pending.Submitted = 4 }},
		{"less-than-four-real-completions", func(r *report) { r.Pending.Completed = 3 }},
		{"fake-pending-fifth-already-completed", func(r *report) { r.Pending.Completed = 5 }},
		{"pending-accounting-mismatch", func(r *report) { r.Pending.InFlight = 2 }},
		{"empty-error", func(r *report) { r.RunError = "" }},
		{"later-error-overwrote-first", func(r *report) { r.RunError = "device removed" }},
		{"did-not-wait-five-seconds", func(r *report) { r.QuitToRunMS = 1 }},
		{"terminal-double-wait", func(r *report) { r.QuitToRunMS = 10000 }},
		{"MAX-is-not-completion", func(r *report) { r.Closed.Completed = math.MaxUint64 }},
		{"pending-counted-as-completed", func(r *report) { r.Closed.Completed = 5 }},
		{"discarded-not-accounted", func(r *report) { r.Closed.DroppedFrames = 0 }},
		{"error-rebuilt-device", func(r *report) { r.Closed.DeviceRecoveries = 1 }},
		{"late-frames-lost", func(r *report) { r.Closed.Submitted = 4 }},
		{"terminal-resources-not-closed", func(r *report) { r.Closed.InFlight = 1 }},
		{"excess-inflight", func(r *report) { r.Closed.MaxInFlight = 3 }},
		{"slot-not-exercised", func(r *report) { r.Pending.UsedSlotsMask = 3 }},
		{"upload-budget-wrong", func(r *report) { r.Pending.UploadedBytes = 320 }},
		{"closed-draw-budget-wrong", func(r *report) { r.Closed.DrawCalls = 2 }},
		{"closed-upload-budget-wrong", func(r *report) { r.Closed.UploadedBytes = 0 }},
		{"worker-retained", func(r *report) { r.WorkerJoined = false }},
		{"HWND-retained", func(r *report) { r.WindowDestroyed = false }},
		{"old-context-accepted", func(r *report) { r.OldRejected = false }},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := goodReceipt()
			test.edit(&r)
			if err := validateReport(r, 42); err == nil {
				t.Fatal("invalid shutdown receipt admitted")
			}
		})
	}
}
