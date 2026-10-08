//go:build (windows || darwin) && cgo

// shadownative reads only its own completed native GPU frames and replays input
// only to its own window. Shader mathematics/CPU images are not GPU evidence.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	ui "github.com/neko233-com/godesktop"
	"github.com/neko233-com/godesktop/internal/platform"
	"github.com/neko233-com/godesktop/internal/shadowtest"
)

type pixelReport struct {
	Name      string  `json:"name"`
	DeviceX   int     `json:"device_x"`
	DeviceY   int     `json:"device_y"`
	Actual    [3]int  `json:"actual_rgb"`
	Expected  [3]int  `json:"expected_rgb"`
	Unclipped float64 `json:"unclipped_shadow_alpha"`
}
type runReport struct {
	Scene         string               `json:"scene"`
	AdapterPolicy string               `json:"adapter_policy"`
	Presentation  string               `json:"native_presentation"`
	Density       float64              `json:"diagnostic_drawable_density"`
	MonitorDPI    uint32               `json:"os_monitor_dpi"`
	CaptureWidth  int                  `json:"capture_width"`
	CaptureHeight int                  `json:"capture_height"`
	BeforeIdle    platform.RenderStats `json:"before_idle"`
	AfterIdle     platform.RenderStats `json:"after_idle"`
	Closed        platform.RenderStats `json:"closed"`
	ViewsBefore   uint64               `json:"views_before_idle"`
	ViewsAfter    uint64               `json:"views_after_idle"`
	BackgroundHit uint32               `json:"background_hits"`
	BodyHit       uint32               `json:"body_hits"`
	ParentHit     uint32               `json:"parent_hits"`
	OldRejected   bool                 `json:"closed_context_rejected"`
	Pixels        []pixelReport        `json:"pixels"`
}

func main() {
	density := flag.Float64("density", 1, "owned diagnostic drawable density: 1, 1.5, or 2; does not change display settings")
	adapter := flag.String("adapter", "hardware", "hardware or warp on Windows; Metal uses its native device")
	output := flag.String("output", ".cache/shadow-native/current", "fixed owned evidence directory")
	recovery := flag.Bool("recovery", false, "force one real device recovery in the first Run")
	flag.Parse()
	if err := run(*density, *adapter, *output, *recovery); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(density float64, adapter, output string, recovery bool) error {
	if density != 1 && density != 1.5 && density != 2 {
		return errors.New("diagnostic density must be 1, 1.5, or 2")
	}
	cleanup, err := prepareOwnedTemp(output)
	if err != nil {
		return err
	}
	defer cleanup()
	// Keep every report tied to this invocation. Remove only this fixture's
	// fixed evidence files, never its output directory or sibling executable.
	if err := resetEvidence(output); err != nil {
		return err
	}
	var reports []runReport
	for lifecycle := range 3 {
		if err := configureNative(density, adapter, recovery && lifecycle == 0); err != nil {
			return err
		}
		report, err := runWindow(density, recovery && lifecycle == 0, filepath.Join(output, fmt.Sprintf("run-%d.png", lifecycle)), lifecycle == 2)
		if err != nil {
			data, _ := json.MarshalIndent(report, "", "  ")
			_ = os.WriteFile(filepath.Join(output, "failed.json"), data, 0600)
			return fmt.Errorf("owned shadow Run %d: %w; report=%s", lifecycle, err, data)
		}
		reports = append(reports, report)
	}
	if err := cleanup(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(reports, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(output, "current.json"), append(data, '\n'), 0600); err != nil {
		return err
	}
	fmt.Printf("native shadow GPU contract passed: policy=%s backend=%s diagnostic-density=%g runs=3 recovery=%t\n", nativeAdapterPolicy(adapter), reports[0].Closed.Backend, density, recovery)
	return nil
}

func resetEvidence(output string) error {
	for _, name := range []string{"current.json", "failed.json", "run-0.png", "run-1.png", "run-2.png"} {
		if err := os.Remove(filepath.Join(output, name)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func prepareOwnedTemp(output string) (func() error, error) {
	cache, err := filepath.Abs(filepath.Join(".cache", "shadow-native"))
	if err != nil {
		return nil, err
	}
	root, err := filepath.Abs(output)
	if err != nil {
		return nil, err
	}
	artifact, err := filepath.Abs(filepath.Join("bin", "shadow-native"))
	if err != nil {
		return nil, err
	}
	allowed := ""
	for _, parent := range []string{cache, artifact} {
		relative, err := filepath.Rel(parent, root)
		if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			allowed = parent
			break
		}
	}
	if allowed == "" {
		return nil, errors.New("shadow output must stay inside .cache/shadow-native or bin/shadow-native")
	}
	for current := root; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("shadow scratch ancestors must not be reparsed")
		}
		if strings.EqualFold(current, filepath.Dir(allowed)) {
			break
		}
	}
	for current := cache; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("shadow private temp ancestors must not be reparsed")
		}
		if strings.EqualFold(current, filepath.Dir(cache)) {
			break
		}
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cache, 0700); err != nil {
		return nil, err
	}
	temp := filepath.Join(cache, "owned-temp-"+filepath.Base(root))
	if err := os.Mkdir(temp, 0700); err != nil && !os.IsExist(err) {
		return nil, err
	}
	info, err := os.Lstat(temp)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("invalid owned shadow temp directory")
	}
	for _, key := range []string{"TMP", "TEMP", "TMPDIR"} {
		if err := os.Setenv(key, temp); err != nil {
			return nil, err
		}
	}
	return func() error {
		entries, err := os.ReadDir(temp)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if len(entries) != 0 {
			return fmt.Errorf("owned shadow temporary files remain: %v", entries)
		}
		return os.Remove(temp)
	}, nil
}

func scene(background, body, parent *atomic.Uint32) *ui.Element {
	click := func(*ui.Context) { body.Add(1) }
	a := ui.Column().Width(80).Height(50).Position(40, 40).ClipRounded(10).Background(ui.RGBA(0xcc3300, .5)).Shadow(ui.ShadowStyle{Color: ui.RGBA(0x204080, .55), OffsetX: 6, OffsetY: 8, Blur: 12, Spread: 2}).Key("body-a").OnClick(click)
	b := ui.Column().Width(60).Height(40).Position(220, 40).Background(ui.RGB(0x00bb33)).Shadow(ui.ShadowStyle{Color: ui.RGBA(0, .6), OffsetX: 8.25, OffsetY: 4.25}).Key("body-b").OnClick(click)
	c := ui.Stack(ui.Column().Width(80).Height(48).Position(30, 30).ClipRounded(12).Background(ui.RGB(0x0077cc)).Shadow(ui.ShadowStyle{Color: ui.RGBA(0x500020, .65), OffsetX: -24, OffsetY: -26, Blur: 12, Spread: -2}).Key("body-c").OnClick(click)).Width(140).Height(100).Position(30, 180).ClipRounded(22).Key("parent-c").OnClick(func(*ui.Context) { parent.Add(1) })
	d := ui.Stack(ui.Column().Width(20).Height(20).Position(-24, 30).Background(ui.RGB(0xff00ff)).Shadow(ui.ShadowStyle{Color: ui.RGBA(0, .8), Blur: 12}).Key("offscreen-body").OnClick(click)).Width(100).Height(80).Position(320, 180)
	e := ui.Column().Width(40).Height(40).Position(380, 50).Radius(12).Shadow(ui.ShadowStyle{Color: ui.RGBA(0, .6), OffsetX: 3.25, OffsetY: 4.25})
	return ui.Stack(ui.Column().Background(ui.RGB(0xffffff)).Key("background").OnClick(func(*ui.Context) { background.Add(1) }), a, b, c, d, e)
}

func await(ctx context.Context, condition func() bool) error {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if condition() {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func stable(ctx context.Context, views *atomic.Uint64, recovery bool) (platform.RenderStats, error) {
	last := platform.RendererStats()
	lastViews, since := views.Load(), time.Now()
	var result platform.RenderStats
	err := await(ctx, func() bool {
		result = platform.RendererStats()
		count := views.Load()
		if result.Submitted != last.Submitted || count != lastViews || result.InFlight != 0 || result.Completed+result.DroppedFrames != result.Submitted {
			since = time.Now()
		}
		last, lastViews = result, count
		return result.Completed >= 4 && (!recovery || result.DeviceRecoveries == 1 && result.Completed >= 14) && time.Since(since) >= 150*time.Millisecond
	})
	return result, err
}

func runWindow(density float64, recovery bool, path string, numeric bool) (report runReport, failure error) {
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	guard := time.AfterFunc(45*time.Second, func() { fmt.Fprintln(os.Stderr, "owned shadow native guard expired"); os.Exit(2) })
	defer guard.Stop()
	var views atomic.Uint64
	var backgrounds, bodies, parents atomic.Uint32
	var savedContext *ui.Context
	workerStarted := false
	workerDone := make(chan struct{})
	geometry := tinyGeometry(density)
	title := "godesktop owned shadow GPU acceptance"
	err := ui.Run(ui.WindowOptions{Title: title, Width: 480, Height: 320, Background: ui.RGB(0xffffff)}, func(cx *ui.Context) *ui.Element {
		views.Add(1)
		savedContext = cx
		stats := platform.RendererStats()
		if stats.Submitted < 4 || recovery && (stats.DeviceRecoveries == 0 || stats.Completed < 14) {
			cx.Invalidate()
		}
		if !workerStarted {
			workerStarted = true
			go func() {
				defer close(workerDone)
				var result runReport
				var checkErr error
				if numeric {
					result, checkErr = checkTinyWindow(ctx, title, density, path, &views, geometry)
				} else {
					result, checkErr = checkWindow(ctx, cx, title, density, recovery, path, &views, &backgrounds, &bodies, &parents)
				}
				report, failure = result, checkErr
				cx.Quit()
			}()
		}
		if numeric {
			return tinyScene(geometry)
		}
		return scene(&backgrounds, &bodies, &parents)
	})
	cancel()
	if workerStarted {
		select {
		case <-workerDone:
		case <-time.After(3 * time.Second):
			return report, errors.New("shadow readback worker did not stop after Run")
		}
	}
	report.Closed = platform.RendererStats()
	report.OldRejected = savedContext != nil && !savedContext.Dispatch(func() {})
	if err != nil {
		return report, err
	}
	if failure != nil {
		return report, failure
	}
	if !report.OldRejected || report.Closed.InFlight != 0 || report.Closed.Completed+report.Closed.DroppedFrames != report.Closed.Submitted {
		return report, errors.New("shadow GPU/context ownership was retained after Run")
	}
	if err := validateNativePresentation(report.Closed, report.Presentation); err != nil {
		return report, err
	}
	return report, nil
}

func checkWindow(ctx context.Context, cx *ui.Context, title string, density float64, recovery bool, path string, views *atomic.Uint64, backgrounds, bodies, parents *atomic.Uint32) (report runReport, failure error) {
	report = runReport{Scene: "ordinary-shadow-input", Density: density, AdapterPolicy: nativeAdapterPolicy(os.Getenv("GODESKTOP_SHADOW_ADAPTER_POLICY"))}
	defer func() {
		report.BackgroundHit, report.BodyHit, report.ParentHit = backgrounds.Load(), bodies.Load(), parents.Load()
	}()
	before, err := stable(ctx, views, recovery)
	if err != nil {
		return report, fmt.Errorf("initial completed GPU frames: %w", err)
	}
	if before.Instances != 9 || before.UploadedBytes != 9*160 || before.DrawCalls != 1 || before.BitmapCacheEntries != 0 || before.BitmapUploads != 0 || before.FrameSlots != 3 || before.UsedSlotsMask != 7 || before.BufferWaits != 0 {
		return report, fmt.Errorf("one-instance shadow, single batch and unchanged resource budgets: %+v", before)
	}
	report.Presentation, err = nativePresentation(title, before)
	if err != nil {
		return report, err
	}
	pixels, actualDensity, monitorDPI, err := snapshotNative(title)
	if err != nil {
		return report, err
	}
	report.Density, report.MonitorDPI = actualDensity, monitorDPI
	report.CaptureWidth, report.CaptureHeight = pixels.Bounds().Dx(), pixels.Bounds().Dy()
	if math.Abs(actualDensity-density) > .001 || report.CaptureHeight != int(math.Round(320*density)) {
		return report, fmt.Errorf("actual drawable density/size %g/%v, expected %g", actualDensity, pixels.Bounds(), density)
	}
	report.Pixels, err = checkPixels(pixels, density)
	if err != nil {
		return report, err
	}
	if _, ok := cx.ElementBounds("offscreen-body"); ok {
		return report, errors.New("fully offscreen body retained a hit/bounds identity")
	}
	if bounds, ok := cx.ElementBounds("body-a"); !ok || bounds.X != 40 || bounds.Y != 40 || bounds.Width != 80 || bounds.Height != 50 {
		return report, fmt.Errorf("shadow moved body/hit geometry: %+v %t", bounds, ok)
	}
	f, err := os.Create(path)
	if err != nil {
		return report, err
	}
	err = errors.Join(png.Encode(f, pixels), f.Close())
	if err != nil {
		return report, err
	}
	report.BeforeIdle = platform.RendererStats()
	report.ViewsBefore = views.Load()
	select {
	case <-ctx.Done():
		return report, ctx.Err()
	case <-time.After(200 * time.Millisecond):
	}
	report.AfterIdle, report.ViewsAfter = platform.RendererStats(), views.Load()
	if report.BeforeIdle.Submitted != report.AfterIdle.Submitted || report.ViewsBefore != report.ViewsAfter || report.AfterIdle.InFlight != 0 {
		return report, errors.New("static shadows created a GPU/view busy loop")
	}
	for index, p := range [][2]float32{{90, 102}, {80, 65}, {325, 220}, {35, 185}, {61, 211}} {
		if err := pointerNative(ctx, cx, title, density, p[0], p[1]); err != nil {
			return report, err
		}
		wantBody, wantBackground := uint32(1), uint32(index)
		if index == 0 {
			wantBody, wantBackground = 0, 1
		}
		if index == 1 {
			wantBackground = 1
		}
		wantParent := uint32(0)
		if index == 4 {
			wantBackground = 3
			wantParent = 1
		}
		inputCtx, stopInput := context.WithTimeout(ctx, 2*time.Second)
		err := await(inputCtx, func() bool {
			return bodies.Load() == wantBody && backgrounds.Load() == wantBackground && parents.Load() == wantParent
		})
		stopInput()
		if err != nil {
			return report, fmt.Errorf("halo/rounded/body input %d: body=%d bg=%d parent=%d, expected %d/%d/%d", index, bodies.Load(), backgrounds.Load(), parents.Load(), wantBody, wantBackground, wantParent)
		}
	}
	return report, nil
}

func checkPixels(pixels *image.RGBA, density float64) ([]pixelReport, error) {
	type sample struct {
		name    string
		x, y    float64
		spec    shadowtest.Spec
		color   ui.Color
		body    ui.Color
		clipped bool
	}
	a := shadowtest.Spec{Rect: shadowtest.Rect{X: 40, Y: 40, Width: 80, Height: 50}, Radius: 10, OffsetX: 6, OffsetY: 8, Spread: 2, Sigma: 6, Opacity: .55}
	b := shadowtest.Spec{Rect: shadowtest.Rect{X: 220, Y: 40, Width: 60, Height: 40}, OffsetX: 8.25, OffsetY: 4.25, Opacity: .6}
	c := shadowtest.Spec{Rect: shadowtest.Rect{X: 60, Y: 210, Width: 80, Height: 48}, Radius: 12, OffsetX: -24, OffsetY: -26, Spread: -2, Sigma: 6, Opacity: .65}
	d := shadowtest.Spec{Rect: shadowtest.Rect{X: 296, Y: 210, Width: 20, Height: 20}, Sigma: 6, Opacity: .8}
	e := shadowtest.Spec{Rect: shadowtest.Rect{X: 380, Y: 50, Width: 40, Height: 40}, Radius: 12, OffsetX: 3.25, OffsetY: 4.25, Opacity: .6}
	samples := []sample{
		{"tinted-halo", 90, 102, a, ui.RGB(0x204080), ui.Color{}, false},
		{"premultiplied-order", 80, 65, a, ui.RGB(0x204080), ui.RGBA(0xcc3300, .5), false},
		{"offset-spread-left", 35, 68, a, ui.RGB(0x204080), ui.Color{}, false},
		{"sharp-offset", 285, 65, b, ui.RGB(0), ui.Color{}, false},
		{"sharp-quarter-pixel", 288.25, 84.25, b, ui.RGB(0), ui.Color{}, false},
		{"opaque-body-order", 250, 60, b, ui.RGB(0), ui.RGB(0x00bb33), false},
		{"sharp-rounded-pixel", 386.75, 57.75, e, ui.RGB(0), ui.Color{}, false},
		{"ancestor-rounded-clip", 35, 186, c, ui.RGB(0x500020), ui.Color{}, true},
		{"ancestor-interior-halo", 48, 198, c, ui.RGB(0x500020), ui.Color{}, false},
		{"negative-spread-body", 100, 240, c, ui.RGB(0x500020), ui.RGB(0x0077cc), false},
		{"offscreen-body-visible-halo", 325, 220, d, ui.RGB(0), ui.Color{}, false},
		{"ancestor-rectangular-clip", 318, 220, d, ui.RGB(0), ui.Color{}, true},
	}
	var reports []pixelReport
	for _, sample := range samples {
		x, y := int(math.Floor(sample.x*density)), int(math.Floor(sample.y*density))
		pointX, pointY := (float64(x)+.5)/density, (float64(y)+.5)/density
		sample.spec.Scale, sample.spec.Samples = density, 8
		if sample.spec.Sigma == 0 {
			sample.spec.Samples = 16
		}
		alpha, err := shadowtest.SampleCoverage(sample.spec, pointX, pointY)
		if err != nil {
			return reports, err
		}
		if sample.name == "sharp-rounded-pixel" && (alpha <= .02 || alpha >= .58) {
			return reports, fmt.Errorf("sharp rounded sample did not prove partial pixel coverage: %g", alpha)
		}
		unclipped := alpha
		if sample.clipped {
			if alpha < .04 {
				return reports, fmt.Errorf("%s would not prove clipping: unclipped alpha %g", sample.name, alpha)
			}
			alpha = 0
		}
		color := pixels.RGBAAt(x, y)
		check := pixelReport{Name: sample.name, DeviceX: x, DeviceY: y, Actual: [3]int{int(color.R), int(color.G), int(color.B)}, Unclipped: unclipped}
		for channel, source := range []float32{sample.color.R, sample.color.G, sample.color.B} {
			value := 1 - alpha + float64(source)*alpha
			body := []float32{sample.body.R, sample.body.G, sample.body.B}[channel]
			value = float64(body)*float64(sample.body.A) + value*(1-float64(sample.body.A))
			check.Expected[channel] = int(math.Round(255 * value))
			if difference := abs(check.Actual[channel] - check.Expected[channel]); difference > 4 {
				return append(reports, check), fmt.Errorf("actual GPU %s at %d,%d: got %v expected %v; alpha=%g", sample.name, x, y, check.Actual, check.Expected, alpha)
			}
		}
		reports = append(reports, check)
	}
	return reports, nil
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
