//go:build windows && cgo

// softwarepresent checks the actual non-diagnostic owned HWND client surface.
// It deliberately disables the framework GPU capture mapping.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	ui "github.com/neko233-com/godesktop"
	"github.com/neko233-com/godesktop/testing/winprobe"
)

type stageReport struct {
	Name           string               `json:"name"`
	RGB            uint32               `json:"client_rgb"`
	Width          int                  `json:"client_width"`
	Height         int                  `json:"client_height"`
	Stats          winprobe.RenderStats `json:"renderer"`
	ReadyWallNanos uint64               `json:"request_to_visible_pixel_wall_nanos"`
}
type runReport struct {
	Stages        []stageReport        `json:"actual_client_stages"`
	Presentation  string               `json:"native_presentation"`
	PID           uint32               `json:"pid"`
	HWND          uint64               `json:"hwnd"`
	DPI           uint32               `json:"actual_os_window_dpi"`
	ExposeBefore  winprobe.RenderStats `json:"before_exposes"`
	ExposeAfter   winprobe.RenderStats `json:"after_exposes"`
	MinBefore     winprobe.RenderStats `json:"before_minimized_idle"`
	MinAfter      winprobe.RenderStats `json:"after_minimized_idle"`
	ViewsExpose   [2]uint64            `json:"views_around_exposes"`
	ViewsMin      [2]uint64            `json:"views_around_minimized_idle"`
	Closed        winprobe.RenderStats `json:"closed"`
	ClosedContext bool                 `json:"closed_context_rejected"`
	ClosedIconic  bool                 `json:"closed_while_minimized"`
	GPUCaptureOff bool                 `json:"diagnostic_gpu_mapping_absent"`
}

func main() {
	output := flag.String("output", ".cache/software-presentation/current", "owned fixed evidence directory")
	debug := flag.Bool("debug", false, "enable the D3D12 validation layer")
	flag.Parse()
	if err := run(*output, *debug); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(output string, debug bool) (failure error) {
	cleanup, err := prepareOwnedOutput(output)
	if err != nil {
		return err
	}
	cleaned := false
	defer func() {
		if !cleaned {
			failure = errors.Join(failure, cleanup())
		}
	}()
	for _, name := range []string{"current.json", "failed.json", "run-0.png", "run-1.png", "run-2.png"} {
		if err := os.Remove(filepath.Join(output, name)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	debugValue := "0"
	if debug {
		debugValue = "1"
	}
	for key, value := range map[string]string{
		"GODESKTOP_READBACK": "0", "GODESKTOP_GPU_ADAPTER": "warp", "GODESKTOP_GPU_DEBUG": debugValue,
		"GODESKTOP_GPU_TRACE_STAGES": "0", "GODESKTOP_TEST_INPUT_ISOLATION": "1", "GODESKTOP_TEST_DEVICE_REMOVAL": "0",
	} {
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}
	if err := os.Unsetenv("GODESKTOP_TEST_DRAWABLE_SCALE"); err != nil {
		return err
	}
	watchdog := time.AfterFunc(40*time.Second, func() { fmt.Fprintln(os.Stderr, "software presentation 40s watchdog expired"); os.Exit(2) })
	defer watchdog.Stop()
	var reports []runReport
	for iteration := range 3 {
		report, err := runWindow(iteration, filepath.Join(output, fmt.Sprintf("run-%d.png", iteration)))
		reports = append(reports, report)
		if err != nil {
			body, _ := json.MarshalIndent(struct {
				Error string      `json:"error"`
				Runs  []runReport `json:"runs"`
			}{err.Error(), reports}, "", "  ")
			_ = os.WriteFile(filepath.Join(output, "failed.json"), body, 0600)
			return fmt.Errorf("actual non-diagnostic software Run %d: %w", iteration, err)
		}
	}
	body, err := json.MarshalIndent(reports, "", "  ")
	if err != nil {
		return err
	}
	if err := cleanup(); err != nil {
		return err
	}
	cleaned = true
	if err := os.WriteFile(filepath.Join(output, "current.json"), body, 0600); err != nil {
		return err
	}
	fmt.Println("actual non-diagnostic committed-dib client presentation passed: runs=3, raw own-HWND GDI, GPU mapping absent, pixels/resize/expose/minimized/closure")
	return nil
}

func prepareOwnedOutput(output string) (func() error, error) {
	cache, err := filepath.Abs(filepath.Join(".cache", "software-presentation"))
	if err != nil {
		return nil, err
	}
	root, err := filepath.Abs(output)
	if err != nil {
		return nil, err
	}
	relative, err := filepath.Rel(cache, root)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return nil, errors.New("software presentation evidence must stay within its owned workspace cache")
	}
	for cursor := root; ; cursor = filepath.Dir(cursor) {
		info, err := os.Lstat(cursor)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		if err == nil && (info.Mode()&os.ModeSymlink != 0 || !info.IsDir()) {
			return nil, errors.New("software presentation scratch ancestor is not an ordinary directory")
		}
		if cursor == filepath.Dir(cursor) {
			break
		}
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	temporary := filepath.Join(cache, "owned-temp-"+filepath.Base(root))
	if err := os.Mkdir(temporary, 0700); err != nil && !os.IsExist(err) {
		return nil, err
	}
	info, err := os.Lstat(temporary)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("software presentation scratch is not an ordinary directory")
	}
	entries, err := os.ReadDir(temporary)
	if err != nil || len(entries) != 0 {
		return nil, errors.New("software presentation scratch is not empty")
	}
	for _, key := range []string{"TMP", "TEMP", "TMPDIR"} {
		if err := os.Setenv(key, temporary); err != nil {
			return nil, err
		}
	}
	return func() error {
		entries, err := os.ReadDir(temporary)
		if err != nil {
			return err
		}
		if len(entries) != 0 {
			return errors.New("non-diagnostic software scratch retained files after shutdown")
		}
		return os.Remove(temporary) // Empty owned directory only.
	}, nil
}

func wait(ctx context.Context, predicate func() (bool, error)) error {
	timer := time.NewTicker(5 * time.Millisecond)
	defer timer.Stop()
	for {
		ready, err := predicate()
		if err != nil || ready {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func pause(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(200 * time.Millisecond):
		return nil
	}
}

func runWindow(iteration int, path string) (report runReport, failure error) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	var views atomic.Uint64
	var saved *ui.Context
	started := false
	finished := make(chan struct{})
	color := uint32(0x336699) // UI-thread owned; the worker only dispatches writes.
	title := fmt.Sprintf("owned software presentation %d-%d", os.Getpid(), iteration)
	err := ui.Run(ui.WindowOptions{Title: title, Width: 320, Height: 200}, func(cx *ui.Context) *ui.Element {
		views.Add(1)
		saved = cx
		if !started {
			started = true
			go func() {
				runtime.LockOSThread()
				defer runtime.UnlockOSThread()
				restore := winprobe.Awareness()
				defer restore()
				defer close(finished)
				defer cx.Quit()
				report, failure = checkWindow(ctx, cx, title, path, iteration == 2, &views, func(next uint32) { color = next })
			}()
		}
		return ui.Stack(
			ui.Column().Background(ui.RGB(color)),
			ui.Column().Width(40).Height(20).Position(10, 10).Background(ui.RGB(0xff0000)),
			ui.Column().Width(40).Height(20).Position(10, 130).Background(ui.RGB(0x0000ff)),
		)
	})
	cancel()
	if started {
		select {
		case <-finished:
		case <-time.After(3 * time.Second):
			return report, errors.New("non-diagnostic presentation worker did not stop after Run")
		}
	}
	report.Closed = winprobe.RendererStats()
	report.ClosedContext = saved != nil && !saved.Dispatch(func() {})
	if err != nil {
		return report, err
	}
	if failure != nil {
		return report, failure
	}
	if !report.ClosedContext || report.Closed.InFlight != 0 || report.Closed.Completed != report.Closed.Submitted || report.Closed.DroppedFrames != 0 || report.Closed.FrameSlots != 3 || report.Closed.UsedSlotsMask != 7 || report.Closed.MaxInFlight == 0 || report.Closed.MaxInFlight > 2 || report.Closed.BufferWaits != 0 {
		return report, errors.New("actual non-diagnostic software frame/context ownership did not close within the original three-slot bounds")
	}
	return report, winprobe.ValidateWindowsPresentation(report.Closed, report.Presentation)
}

func checkWindow(ctx context.Context, cx *ui.Context, title, path string, closeMinimized bool, views *atomic.Uint64, setColor func(uint32)) (report runReport, failure error) {
	initialRequest := time.Now()
	pid := uint32(os.Getpid())
	var window winprobe.Window
	if err := wait(ctx, func() (bool, error) {
		w, err := winprobe.Find(title, pid)
		if err != nil {
			return false, nil
		}
		window = w
		stats := winprobe.RendererStats()
		if stats.Submitted < 4 {
			cx.Invalidate()
		}
		return stats.Completed >= 4 && stats.Completed == stats.Submitted && stats.InFlight == 0, nil
	}); err != nil {
		return report, err
	}
	if err := ownTopmost(window, pid); err != nil {
		return report, err
	}
	presentation, err := winprobe.NativePresentation(window, pid)
	if err != nil || presentation != "committed-dib" {
		return report, fmt.Errorf("expected actual committed-dib HWND: %s %v", presentation, err)
	}
	report.Presentation, report.PID, report.HWND = presentation, pid, uint64(window)
	report.DPI = window.DPI()
	if err := noDiagnosticMapping(window, pid); err != nil {
		return report, err
	}
	report.GPUCaptureOff = true
	stage := func(name string, expected uint32, minimum uint64, requestAt time.Time) error {
		if err := wait(ctx, func() (bool, error) {
			stats := winprobe.RendererStats()
			if err := winprobe.ValidateWindowsPresentation(stats, presentation); err != nil {
				return false, err
			}
			if stats.Completed < minimum || stats.InFlight != 0 || stats.Completed != stats.Submitted {
				return false, nil
			}
			if stats.FrameSlots != 3 || stats.UsedSlotsMask != 7 || stats.MaxInFlight > 2 || stats.BufferWaits != 0 || stats.Instances != 3 || stats.UploadedBytes != 3*160 || stats.DrawCalls != 1 {
				return false, errors.New("actual software frame violated three-slot/two-in-flight/single-draw bounds")
			}
			return clientColor(window, pid, expected)
		}); err != nil {
			return err
		}
		width, height, err := window.ClientSize()
		if err != nil {
			return err
		}
		report.Stages = append(report.Stages, stageReport{Name: name, RGB: expected, Width: width, Height: height, Stats: winprobe.RendererStats(), ReadyWallNanos: uint64(time.Since(requestAt))})
		return nil
	}
	if err := stage("initial-visible-client", 0x336699, 4, initialRequest); err != nil {
		return report, err
	}
	if err := pause(ctx); err != nil {
		return report, err
	}
	report.ExposeBefore, report.ViewsExpose[0] = winprobe.RendererStats(), views.Load()
	for range 8 {
		if err := expose(window, pid); err != nil {
			return report, err
		}
	}
	if err := pause(ctx); err != nil {
		return report, err
	}
	report.ExposeAfter, report.ViewsExpose[1] = winprobe.RendererStats(), views.Load()
	if report.ExposeBefore.Submitted != report.ExposeAfter.Submitted || report.ExposeBefore.FrameTicks != report.ExposeAfter.FrameTicks || report.ViewsExpose[0] != report.ViewsExpose[1] || report.ExposeAfter.InFlight != 0 {
		return report, errors.New("repeated actual WM_PAINT exposure fed back into native View/submissions")
	}
	change := func(next uint32) error {
		done := make(chan struct{})
		if !cx.Dispatch(func() { setColor(next); cx.Invalidate(); close(done) }) {
			return errors.New("owned UI color dispatch rejected")
		}
		select {
		case <-done:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	baseline := winprobe.RendererStats().Completed
	resizeRequest := time.Now()
	if err := window.Resize(480, 300); err != nil {
		return report, err
	}
	if err := stage("resize-without-model-invalidation", 0x336699, baseline+1, resizeRequest); err != nil {
		return report, err
	}
	width, height, err := window.ClientSize()
	if err != nil {
		return report, err
	}
	scale := float64(window.DPI()) / 96
	if math.Abs(float64(width)/scale-480) > 1 || math.Abs(float64(height)/scale-300) > 1 {
		return report, fmt.Errorf("actual resized client dimensions %dx%d at DPI%d do not match 480x300 DIP", width, height, window.DPI())
	}
	baseline = winprobe.RendererStats().Completed
	colorRequest := time.Now()
	if err := change(0xcc6633); err != nil {
		return report, err
	}
	if err := stage("resized-visible-client", 0xcc6633, baseline+1, colorRequest); err != nil {
		return report, err
	}
	window.Show(6)
	if err := wait(ctx, func() (bool, error) { return isIconic(window), nil }); err != nil {
		return report, err
	}
	report.MinBefore, report.ViewsMin[0] = winprobe.RendererStats(), views.Load()
	if err := pause(ctx); err != nil {
		return report, err
	}
	report.MinAfter, report.ViewsMin[1] = winprobe.RendererStats(), views.Load()
	if !isIconic(window) || report.MinBefore.Submitted != report.MinAfter.Submitted || report.MinBefore.FrameTicks != report.MinAfter.FrameTicks || report.ViewsMin[0] != report.ViewsMin[1] || report.MinAfter.InFlight != 0 {
		return report, errors.New("actual minimized software HWND created a View/GPU loop")
	}
	restoreRequest := time.Now()
	baseline = winprobe.RendererStats().Completed
	window.Show(9)
	if err := stage("restore-without-model-invalidation", 0xcc6633, baseline+1, restoreRequest); err != nil {
		return report, err
	}
	baseline = winprobe.RendererStats().Completed
	colorRequest = time.Now()
	if err := change(0x33aa55); err != nil {
		return report, err
	}
	if err := stage("restored-visible-client", 0x33aa55, baseline+1, colorRequest); err != nil {
		return report, err
	}
	pixels, err := captureClient(window, pid)
	if err != nil {
		return report, err
	}
	if pixel := pixels.RGBAAt(pixels.Bounds().Dx()/2, pixels.Bounds().Dy()/2); pixel.R != 0x33 || pixel.G != 0xaa || pixel.B != 0x55 {
		return report, fmt.Errorf("actual own-client BitBlt disagrees with restored GetPixel: %v", pixel)
	}
	for _, marker := range markerPoints(window.DPI()) {
		pixel := pixels.RGBAAt(marker.x, marker.y)
		if uint32(pixel.R)<<16|uint32(pixel.G)<<8|uint32(pixel.B) != marker.rgb {
			return report, fmt.Errorf("actual client BitBlt top-down marker at (%d,%d): %v expected%#x", marker.x, marker.y, pixel, marker.rgb)
		}
	}
	file, err := os.Create(path)
	if err != nil {
		return report, err
	}
	if err := errors.Join(png.Encode(file, pixels), file.Close()); err != nil {
		return report, err
	}
	if closeMinimized {
		window.Show(6)
		if err := wait(ctx, func() (bool, error) { return isIconic(window), nil }); err != nil {
			return report, err
		}
		report.ClosedIconic = true
	}
	return report, nil
}
