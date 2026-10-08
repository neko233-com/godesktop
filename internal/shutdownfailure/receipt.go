// Package main provides an isolated acceptance program for a real native
// five-second final-window-drain failure. Receipt tests do not execute a GPU.
package main

import (
	"errors"
	"strings"

	"github.com/neko233-com/godesktop/testing/winprobe"
)

const timeoutText = "GPU fence did not complete within five seconds"

type report struct {
	Schema          int                  `json:"schema"`
	PID             uint32               `json:"pid"`
	HWND            uint64               `json:"hwnd"`
	Presentation    string               `json:"native_presentation"`
	DPI             uint32               `json:"actual_os_window_dpi"`
	ShutdownPending bool                 `json:"actual_shutdown_pending_property"`
	Pending         winprobe.RenderStats `json:"pending_before_quit"`
	Closed          winprobe.RenderStats `json:"closed_after_run"`
	RunError        string               `json:"run_error"`
	QuitToRunMS     int64                `json:"quit_to_run_return_ms"`
	WorkerJoined    bool                 `json:"worker_joined"`
	WindowDestroyed bool                 `json:"owned_window_destroyed"`
	OldRejected     bool                 `json:"closed_context_rejected"`
	Pass            bool                 `json:"pass"`
}

func validateReport(r report, pid uint32) error {
	if r.Schema != 1 || pid == 0 || r.PID != pid || r.HWND == 0 || r.DPI == 0 || r.Presentation != "committed-dib" || !r.ShutdownPending {
		return errors.New("shutdown failure receipt lost its actual HWND/process/presentation identity")
	}
	if err := winprobe.ValidateWindowsPresentation(r.Pending, r.Presentation); err != nil {
		return err
	}
	if err := winprobe.ValidateWindowsPresentation(r.Closed, r.Presentation); err != nil {
		return err
	}
	p, c := r.Pending, r.Closed
	if p.Submitted < 5 || p.Completed != 4 || p.Completed >= p.Submitted || p.InFlight == 0 || p.InFlight > 2 ||
		uint64(p.InFlight) != p.Submitted-p.Completed ||
		p.DroppedFrames != 0 || p.DeviceRecoveries != 0 || p.FrameSlots != 3 || p.UsedSlotsMask != 7 || p.MaxInFlight > 2 ||
		p.DrawCalls != 1 || p.Instances != 1 || p.UploadedBytes != 160 || p.BufferWaits != 0 {
		return errors.New("shutdown failure did not observe a genuinely pending fifth real single-draw GPU frame")
	}
	if !strings.Contains(r.RunError, timeoutText) || r.QuitToRunMS < 4500 || r.QuitToRunMS > 7000 {
		return errors.New("actual Run did not propagate its original five-second final-drain failure within the bound")
	}
	if c.Submitted < p.Submitted || c.Completed != p.Completed || c.Completed >= c.Submitted ||
		c.DroppedFrames != c.Submitted-c.Completed || c.InFlight != 0 || c.DeviceRecoveries != 0 ||
		c.FrameSlots != 3 || c.UsedSlotsMask != 7 || c.MaxInFlight > 2 || c.BufferWaits != 0 ||
		c.DrawCalls != 1 || c.Instances != 1 || c.UploadedBytes != 160 ||
		!r.WorkerJoined || !r.WindowDestroyed || !r.OldRejected {
		return errors.New("final drain failure lost immutable completion/drop accounting or owned shutdown")
	}
	return nil
}
