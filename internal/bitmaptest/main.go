//go:build (windows || darwin) && cgo

// bitmaptest verifies actual owned GPU pixels, texture reuse, count/byte eviction,
// reloading an evicted immutable asset and cache reset across native Runs.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	ui "github.com/neko233-com/godesktop"
	"github.com/neko233-com/godesktop/internal/platform"
)

const title = "godesktop GPU bitmap acceptance"

// Only the most recent immutable progress and native sample are retained. The
// UI publishes plain values; the watchdog never asks the stalled UI for work.
type bitmapProgress struct {
	ObservedAt    time.Time            `json:"observed_at"`
	Run           int                  `json:"run"`
	Phase         int                  `json:"phase"`
	Index         int                  `json:"index"`
	LastCompleted uint64               `json:"last_completed"`
	PhaseFrame    uint64               `json:"phase_frame"`
	Stage         string               `json:"stage"`
	QuitRequested bool                 `json:"quit_requested"`
	Failure       string               `json:"failure,omitempty"`
	RunError      string               `json:"run_error,omitempty"`
	UIStatsAt     time.Time            `json:"ui_stats_observed_at"`
	UIStats       platform.RenderStats `json:"last_ui_stats"`
}
type bitmapNativeSample struct {
	ObservedAt time.Time            `json:"observed_at"`
	Stats      platform.RenderStats `json:"stats"`
}
type bitmapTimeoutReport struct {
	CapturedAt     time.Time           `json:"captured_at"`
	TimeoutSeconds int                 `json:"timeout_seconds"`
	Recovery       bool                `json:"recovery"`
	Progress       *bitmapProgress     `json:"progress"`
	NativeSample   *bitmapNativeSample `json:"native_sample"`
}
type bitmapDiagnostics struct {
	progress atomic.Pointer[bitmapProgress]
	native   atomic.Pointer[bitmapNativeSample]
	stopOnce sync.Once
	stop     chan struct{}
	done     chan struct{}
}

func startBitmapDiagnostics(stats func() platform.RenderStats, interval time.Duration) *bitmapDiagnostics {
	d := &bitmapDiagnostics{stop: make(chan struct{}), done: make(chan struct{})}
	go func() {
		defer close(d.done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-d.stop:
				return
			case <-ticker.C:
				select {
				case <-d.stop:
					return
				default:
				}
				// RendererStats reads native counters independently of the UI
				// callback. No window action, dispatch or screenshot is requested.
				value := stats()
				d.native.Store(&bitmapNativeSample{ObservedAt: time.Now(), Stats: value})
			}
		}
	}()
	return d
}
func (d *bitmapDiagnostics) record(progress bitmapProgress) {
	progress.ObservedAt = time.Now()
	d.progress.Store(&progress)
}
func (d *bitmapDiagnostics) close() {
	d.stopOnce.Do(func() { close(d.stop) })
	<-d.done // Normal and error returns join the single sampler explicitly.
}
func (d *bitmapDiagnostics) timeoutReport(recovery bool) bitmapTimeoutReport {
	return bitmapTimeoutReport{CapturedAt: time.Now(), TimeoutSeconds: 40, Recovery: recovery, Progress: d.progress.Load(), NativeSample: d.native.Load()}
}
func (d *bitmapDiagnostics) writeTimeout(directory string, recovery bool, stderr io.Writer) {
	fmt.Fprintln(stderr, "bitmap native acceptance timed out")
	data, err := json.Marshal(d.timeoutReport(recovery))
	if err != nil {
		fmt.Fprintln(stderr, "bitmap timeout diagnostics:", err)
		return
	}
	fmt.Fprintln(stderr, string(data))
	// The output directory was created before Run. Only the background
	// watchdog writes this bounded failure report, never a view callback.
	if err := os.WriteFile(filepath.Join(directory, "timeout.json"), append(data, '\n'), 0644); err != nil {
		fmt.Fprintln(stderr, "bitmap timeout report:", err)
	}
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func makeBitmap(width, height int, solid bool) (*ui.Bitmap, error) {
	source := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			c := color.NRGBA{R: 255, A: 255}
			if !solid {
				switch {
				case x >= width/2 && y < height/2:
					c = color.NRGBA{G: 255, A: 255}
				case x < width/2 && y >= height/2:
					c = color.NRGBA{B: 255, A: 255}
				case x >= width/2 && y >= height/2:
					c = color.NRGBA{R: 255, G: 255, A: 128}
				}
			}
			source.SetNRGBA(x, y, c)
		}
	}
	bitmap, err := ui.NewBitmap(source)
	// The original pixels are intentionally changed. GPU output must retain the
	// owned original RGBA copy, not this mutable decoded image.
	clear(source.Pix)
	return bitmap, err
}

func run() error {
	output := flag.String("output", ".cache/bitmap-native", "owned GPU PNG/JSON directory")
	recovery := flag.Bool("recovery", false, "test actual Windows device loss / completion-triggered Mac view recovery")
	flag.Parse()
	if err := os.Setenv("GODESKTOP_READBACK", "1"); err != nil {
		return err
	}
	base, err := makeBitmap(31, 17, false)
	if err != nil {
		return err
	}
	var small, large []*ui.Bitmap
	for i := range 130 {
		source := image.NewRGBA(image.Rect(0, 0, 31, 17))
		for p := 0; p < len(source.Pix); p += 4 {
			copy(source.Pix[p:p+4], []byte{byte(i), byte(255 - i), 17, 255})
		}
		b, err := ui.NewBitmap(source)
		if err != nil {
			return err
		}
		small = append(small, b)
	}
	for range 18 {
		b, err := makeBitmap(1024, 1024, true)
		if err != nil {
			return err
		}
		large = append(large, b)
	}
	if err := os.MkdirAll(*output, 0755); err != nil {
		return err
	}
	diagnostics := startBitmapDiagnostics(platform.RendererStats, time.Second)
	defer diagnostics.close()
	diagnostics.record(bitmapProgress{Run: -1, Phase: -1, Stage: "prepared"})
	watchdog := time.AfterFunc(40*time.Second, func() {
		diagnostics.writeTimeout(*output, *recovery, os.Stderr)
		// Preserve the hard exit without waiting for native calls or UI work.
		os.Exit(2)
	})
	defer watchdog.Stop()
	var reports []platform.RenderStats
	for run := 0; run < 2; run++ {
		for _, key := range []string{"GODESKTOP_TEST_DEVICE_REMOVAL", "GODESKTOP_TEST_METAL_RECOVERY", "GODESKTOP_TEST_METAL_RECOVERIES"} {
			os.Unsetenv(key)
		}
		if *recovery && run == 0 {
			if runtime.GOOS == "windows" {
				os.Setenv("GODESKTOP_TEST_DEVICE_REMOVAL", "7")
			} else {
				os.Setenv("GODESKTOP_TEST_METAL_RECOVERY", "7")
				os.Setenv("GODESKTOP_TEST_METAL_RECOVERIES", "1")
			}
		}
		phase, index := 0, 0
		last := uint64(0)
		phaseFrame := uint64(0)
		var failed error
		current := base
		var lastUIStats platform.RenderStats
		var lastUIStatsAt time.Time
		quitRequested := false
		runError := ""
		recordProgress := func(stage string) {
			failure := ""
			if failed != nil {
				failure = failed.Error()
			}
			diagnostics.record(bitmapProgress{Run: run, Phase: phase, Index: index, LastCompleted: last, PhaseFrame: phaseFrame, Stage: stage, QuitRequested: quitRequested, Failure: failure, RunError: runError, UIStats: lastUIStats, UIStatsAt: lastUIStatsAt})
		}
		recordProgress("before-run")
		err := ui.Run(ui.WindowOptions{Title: title, Width: 500, Height: 300, Background: ui.RGB(0x101020)}, func(cx *ui.Context) *ui.Element {
			recordProgress("read-renderer-stats")
			stats := platform.RendererStats()
			lastUIStats, lastUIStatsAt = stats, time.Now()
			recordProgress("view-enter")
			if stats.Completed > last {
				last = stats.Completed
				if stats.BitmapCacheEntries > 128 || stats.BitmapCacheBytes > 64<<20 {
					failed = errors.New("native bitmap residency exceeded budget")
				}
				minimum := uint64(6)
				if *recovery && run == 0 {
					minimum = 12
				}
				if phase == 0 && stats.Completed >= minimum {
					if *recovery && run == 0 && stats.DeviceRecoveries != 1 {
						failed = errors.New("bitmap device/view recovery not observed")
					}
					copies := 1 + stats.DeviceRecoveries
					if stats.BitmapUploads != copies || stats.BitmapUploadedBytes != 31*17*4*copies {
						failed = fmt.Errorf("static asset repeatedly uploaded: %+v", stats)
					}
					recordProgress("verify-reuse-pixels")
					if err := verifyPixels(*output, fmt.Sprintf("run-%d-reuse", run)); err != nil {
						failed = err
					}
					recordProgress("reuse-pixels-returned")
					if run == 1 {
						phase = 5
						quitRequested = true
						recordProgress("quit-requested")
						cx.Quit()
						recordProgress("quit-returned")
					} else {
						phase = 1
						index = 0
						current = small[0]
					}
				} else if phase == 1 {
					index++
					if index < len(small) {
						current = small[index]
					} else {
						phase = 2
						phaseFrame = stats.Completed
					}
				} else if phase == 2 && stats.Completed >= phaseFrame+4 {
					if stats.BitmapCacheEntries != 128 || stats.BitmapUploads != 133+stats.DeviceRecoveries {
						failed = fmt.Errorf("128 visible images were not pinned/reused: %+v", stats)
					}
					recordProgress("verify-grid-pixels")
					if err := verifyGrid(*output); err != nil {
						failed = err
					}
					recordProgress("grid-pixels-returned")
					phase = 3
					index = 0
					current = large[0]
				} else if phase == 3 {
					index++
					if index < len(large) {
						current = large[index]
					} else {
						phase = 4
						current = base
						phaseFrame = stats.Completed
					}
				} else if phase == 4 && stats.Completed >= phaseFrame+4 {
					if stats.BitmapUploads != 152+stats.DeviceRecoveries {
						failed = fmt.Errorf("count/byte eviction/reupload count=%d, want %d", stats.BitmapUploads, 152+stats.DeviceRecoveries)
					}
					recordProgress("verify-evicted-pixels")
					if err := verifyPixels(*output, "evicted-base-restored"); err != nil {
						failed = err
					}
					recordProgress("evicted-pixels-returned")
					phase = 5
					quitRequested = true
					recordProgress("quit-requested")
					cx.Quit()
					recordProgress("quit-returned")
				}
				recordProgress("completion-observed")
			}
			if failed != nil {
				quitRequested = true
				recordProgress("quit-after-failure")
				cx.Quit()
				recordProgress("quit-returned")
			}
			recordProgress("invalidate")
			cx.Invalidate()
			if phase == 2 {
				rows := make([]*ui.Element, 8)
				for row := range 8 {
					cells := make([]*ui.Element, 16)
					for column := range 16 {
						cells[column] = ui.Image(small[row*16+column]).Width(24).Height(18)
					}
					rows[row] = ui.Row(cells...).Gap(4).Height(22)
				}
				view := ui.Column(rows...).Padding(20)
				recordProgress("view-return")
				return view
			}
			view := ui.Column(ui.Row(ui.Image(current).Width(124).Height(68), ui.Row(ui.Image(current).Width(124).Height(68)).Width(62).Height(68)).Gap(20), ui.Text("Immutable GPU bitmap / alpha / crop / residency")).Padding(20)
			recordProgress("view-return")
			return view
		})
		if err != nil {
			runError = err.Error()
		}
		recordProgress("run-returned")
		if err != nil {
			return err
		}
		if failed != nil {
			return failed
		}
		if phase != 5 {
			return errors.New("bitmap native phases incomplete")
		}
		reports = append(reports, platform.RendererStats())
	}
	finalProgress := *diagnostics.progress.Load()
	finalProgress.Stage = "write-final-report"
	diagnostics.record(finalProgress)
	file, err := os.Create(filepath.Join(*output, "report.json"))
	if err != nil {
		return err
	}
	defer file.Close()
	if err := json.NewEncoder(file).Encode(reports); err != nil {
		return err
	}
	fmt.Println("Native GPU bitmap pixels, premultiplied alpha, crop, upload reuse, count/byte eviction, reupload and Run cleanup passed")
	return nil
}

func verifyGrid(directory string) error {
	pixels, scale, err := capture()
	if err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(directory, "128-visible-images.png"))
	if err != nil {
		return err
	}
	err = png.Encode(f, pixels)
	err = errors.Join(err, f.Close())
	if err != nil {
		return err
	}
	for i := range 128 {
		x, y := 20+(i%16)*28+12, 20+(i/16)*22+9
		got := pixels.RGBAAt(int(float64(x)*scale), int(float64(y)*scale))
		want := color.RGBA{byte(i), byte(255 - i), 17, 255}
		if got != want {
			return fmt.Errorf("visible image %d pixel=%v want %v", i, got, want)
		}
	}
	return nil
}

func verifyPixels(directory, stage string) error {
	pixels, scale, err := capture()
	if err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(directory, stage+".png"))
	if err != nil {
		return err
	}
	err = png.Encode(f, pixels)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	for _, sample := range []struct {
		x, y int
		want color.RGBA
	}{{35, 32, color.RGBA{255, 0, 0, 255}}, {110, 32, color.RGBA{0, 255, 0, 255}}, {35, 78, color.RGBA{0, 0, 255, 255}}, {110, 78, color.RGBA{136, 136, 16, 255}}, {175, 32, color.RGBA{255, 0, 0, 255}}, {240, 32, color.RGBA{16, 16, 32, 255}}} {
		c := pixels.RGBAAt(int(float64(sample.x)*scale), int(float64(sample.y)*scale))
		close := func(a, b uint8) bool { return int(a)-int(b) <= 2 && int(b)-int(a) <= 2 }
		if !close(c.R, sample.want.R) || !close(c.G, sample.want.G) || !close(c.B, sample.want.B) {
			return fmt.Errorf("%s GPU sample %d,%d = %v want %v", stage, sample.x, sample.y, c, sample.want)
		}
	}
	return nil
}
