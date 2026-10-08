// renderstress exercises actual native submissions rather than Go layout alone.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime"
	"sort"
	"time"

	ui "github.com/neko233-com/godesktop"
	"github.com/neko233-com/godesktop/internal/platform"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func percentile(samples []uint64, percentile int) uint64 {
	if len(samples) == 0 {
		return 0
	}
	copyOfSamples := append([]uint64(nil), samples...)
	sort.Slice(copyOfSamples, func(i, j int) bool { return copyOfSamples[i] < copyOfSamples[j] })
	return copyOfSamples[(len(copyOfSamples)-1)*percentile/100]
}

func run() error {
	frames := flag.Uint64("frames", 90, "minimum completed native GPU frames")
	requireBackend := flag.String("require-backend", "metal", "required renderer; refuses a different rendering path")
	requireClock := flag.String("require-frame-clock", "cametaldisplaylink", "required exact clock; windows-native requires the actual owned HWND's D3D12 presentation/clock pair")
	glyphAtlas := flag.Bool("glyph-atlas", false, "vary text every frame and require bounded per-glyph reuse")
	glyphEviction := flag.Bool("glyph-eviction", false, "vary large font sizes to exercise atlas eviction and GPU lifetime")
	deviceRecovery := flag.Bool("device-recovery", false, "remove this Windows renderer's actual D3D12 device and require recovery")
	metalRecovery := flag.Bool("metal-recovery", false, "inject a recovery request after actual Metal GPU completion; hardware stays connected")
	completionRace := flag.Bool("completion-race", false, "diagnostically finish the D3D12 GPU between poll and the idle fence wait")
	output := flag.String("output", "", "optional JSON report filename")
	flag.Parse()
	if *frames < 6 || *frames > 10000 {
		return errors.New("frames must be between 6 and 10000")
	}
	if *requireClock == "windows-native" && (runtime.GOOS != "windows" || *requireBackend != "direct3d12") {
		return errors.New("windows-native frame clock requires the Windows direct3d12 renderer")
	}
	if *glyphAtlas && *glyphEviction {
		return errors.New("choose glyph reuse or glyph eviction validation")
	}
	if *deviceRecovery {
		if runtime.GOOS != "windows" || *requireBackend != "direct3d12" || *glyphAtlas || *glyphEviction {
			return errors.New("device recovery requires the Windows D3D12 scene without glyph stress flags")
		}
		if err := os.Setenv("GODESKTOP_TEST_DEVICE_REMOVAL", "9"); err != nil {
			return err
		}
		if err := os.Setenv("GODESKTOP_READBACK", "1"); err != nil {
			return err
		}
	}
	if *metalRecovery {
		if runtime.GOOS != "darwin" || *requireBackend != "metal" || *glyphAtlas || *glyphEviction || *deviceRecovery {
			return errors.New("Metal recovery requires the macOS Metal scene without other glyph/recovery flags")
		}
		for key, value := range map[string]string{"GODESKTOP_TEST_METAL_RECOVERY": "9", "GODESKTOP_TEST_METAL_RECOVERIES": "1", "GODESKTOP_READBACK": "1"} {
			if err := os.Setenv(key, value); err != nil {
				return err
			}
		}
	}
	if *completionRace {
		if runtime.GOOS != "windows" || *requireBackend != "direct3d12" {
			return errors.New("completion race probe requires Windows D3D12")
		}
		if err := os.Setenv("GODESKTOP_TEST_COMPLETION_RACE", "1"); err != nil {
			return err
		}
	}
	watchdog := time.AfterFunc(30*time.Second, func() { fmt.Fprintln(os.Stderr, "GPU stress watchdog expired"); os.Exit(1) })
	defer watchdog.Stop()
	var samples []uint64
	var sceneSamples, acquireSamples, encodeSamples []uint64
	var previous uint64
	var mismatch error
	var idleStarted, idleFinished bool
	var recoveryObserved bool
	var completedBeforeRecovery uint64
	var windowIdentity uint64
	var windowPresentation string
	var modelState int
	var modelInitialized, dispatchedAfterRecovery bool
	var idleBefore, idleAfter platform.RenderStats
	started := time.Now()
	err := ui.Run(ui.WindowOptions{Title: "godesktop native GPU stress", Width: 1000, Height: 650}, func(cx *ui.Context) *ui.Element {
		stats := platform.RendererStats()
		if *metalRecovery {
			identity := metalWindowIdentity()
			if identity == 0 || (windowIdentity != 0 && identity != windowIdentity) {
				mismatch = fmt.Errorf("Metal recovery replaced the native window: before=%d after=%d", windowIdentity, identity)
				cx.Quit()
				return nil
			}
			windowIdentity = identity
			if !modelInitialized {
				modelInitialized = true
				cx.Dispatch(func() { modelState = 7 })
			}
		}
		if (*deviceRecovery || *metalRecovery) && stats.DeviceRecoveries > 0 && !recoveryObserved {
			recoveryObserved = true
			completedBeforeRecovery = stats.Completed
			if *metalRecovery {
				if modelState != 7 {
					mismatch = fmt.Errorf("Go state was lost during Metal resource replacement: %d", modelState)
					cx.Quit()
					return nil
				}
				cx.Dispatch(func() { modelState, dispatchedAfterRecovery = 11, true })
			}
		}
		if stats.Backend != *requireBackend {
			mismatch = fmt.Errorf("required %s renderer, got %s", *requireBackend, stats.Backend)
			cx.Quit()
			return nil
		}
		if *requireClock == "windows-native" {
			presentation, identity, err := windowsWindowPresentation()
			if err != nil || identity == 0 || (windowIdentity != 0 && windowIdentity != identity) {
				mismatch = fmt.Errorf("owned Windows native presentation identity: before=%d after=%d: %v", windowIdentity, identity, err)
				cx.Quit()
				return nil
			}
			windowIdentity, windowPresentation = identity, presentation
		}
		if err := validateRequiredFrameClock(stats, *requireClock, windowPresentation); err != nil {
			mismatch = err
			cx.Quit()
			return nil
		}
		if stats.Submitted > previous {
			previous = stats.Submitted
			samples = append(samples, stats.CPUTimeNanos)
			sceneSamples = append(sceneSamples, stats.SceneTimeNanos)
			acquireSamples = append(acquireSamples, stats.AcquireTimeNanos)
			encodeSamples = append(encodeSamples, stats.EncodeTimeNanos)
		}
		if idleFinished {
			if *deviceRecovery {
				mismatch = recoveryPixels(*output)
			}
			if *metalRecovery {
				mismatch = metalRecoveryPixels(*output)
			}
			cx.Quit()
		} else if stats.Completed >= *frames+completedBeforeRecovery {
			if !idleStarted {
				idleStarted = true
				go func() {
					// Let the last frame drain and the display clock pause, then
					// observe an idle interval before waking it through Dispatch.
					time.Sleep(250 * time.Millisecond)
					before := platform.RendererStats()
					time.Sleep(150 * time.Millisecond)
					after := platform.RendererStats()
					cx.Dispatch(func() { idleBefore, idleAfter, idleFinished = before, after, true })
				}()
			}
		} else {
			cx.Invalidate()
			// Context now coalesces logical invalidations before crossing the
			// native ABI. This fixture's existing CoalescedRequests assertion
			// exercises the native frame scheduler, so send its two redundant
			// requests directly to that layer; Go queue coalescing has separate
			// bounded/owned-window acceptance. Keep the native count threshold.
			platform.Wake()
			platform.Wake()
		}
		rows := make([]*ui.Element, 0, 32)
		for y := 0; y < 32; y++ {
			cells := make([]*ui.Element, 0, 65)
			for x := 0; x < 64; x++ {
				color := uint32(0x2663a3)
				if (x+y+int(stats.Submitted))%2 == 0 {
					color = 0x319473
				}
				cells = append(cells, ui.Column().Width(12).Height(16).Radius(2).Background(ui.RGB(color)))
			}
			// Shared shaped text, varied clip regions and geometry interleaved with
			// it must remain in painter order while reusing the same texture batch.
			label := "GPU"
			if *metalRecovery {
				label = fmt.Sprintf("GPU %d", modelState)
			}
			fontSize := float32(12)
			if *glyphAtlas {
				label = fmt.Sprintf("GPU %06d", stats.Submitted%1000000)
			}
			if *glyphEviction {
				label, fontSize = "GPU 0123456789", float32(300+stats.Submitted%90)
			}
			cells = append(cells, ui.Text(label).Width(80).Height(16).FontSize(fontSize))
			rows = append(rows, ui.Row(cells...).Height(16).Gap(1))
		}
		return ui.Column(rows...).Padding(20).Gap(2)
	})
	if err != nil {
		return err
	}
	if mismatch != nil {
		return mismatch
	}
	stats := platform.RendererStats()
	if stats.Completed < *frames || stats.Completed+stats.DroppedFrames != stats.Submitted || stats.InFlight != 0 {
		return fmt.Errorf("submissions did not drain: %+v", stats)
	}
	if *deviceRecovery && (!recoveryObserved || stats.Completed-completedBeforeRecovery < *frames || stats.DeviceRecoveries != 1 || stats.DroppedFrames == 0 || stats.DroppedFrames > 3 || stats.GlyphRasterizations < 6 || stats.GlyphAtlasBytes != 1024*1024) {
		return fmt.Errorf("actual device removal did not rebuild a bounded renderer: %+v", stats)
	}
	if *metalRecovery && (!recoveryObserved || stats.Completed-completedBeforeRecovery < *frames || stats.DeviceRecoveries != 1 || stats.DroppedFrames != 0 || !dispatchedAfterRecovery || modelState != 11 || stats.GlyphRasterizations < 6 || stats.GlyphAtlasBytes != 1024*1024) {
		return fmt.Errorf("Metal resource/state recovery failed: model=%d renderer=%+v", modelState, stats)
	}
	if !*deviceRecovery && !*metalRecovery && (stats.DeviceRecoveries != 0 || stats.DroppedFrames != 0) {
		return fmt.Errorf("ordinary rendering unexpectedly lost a device or frame: %+v", stats)
	}
	if stats.FrameSlots != 3 || stats.UsedSlotsMask != 7 || stats.MaxInFlight == 0 || stats.MaxInFlight > 3 || stats.BufferWaits != 0 {
		return fmt.Errorf("three-slot asynchronous ownership failed: %+v", stats)
	}
	if stats.Instances < 2000 || stats.DrawCalls == 0 || (!*glyphEviction && stats.DrawCalls > 2) || stats.UploadedBytes != stats.Instances*160 {
		return fmt.Errorf("instancing/batching failed: %+v", stats)
	}
	if len(samples) < 3 || percentile(samples, 95) == 0 {
		return errors.New("native frame CPU timings were not collected")
	}
	if stats.SceneTimeNanos+stats.AcquireTimeNanos+stats.EncodeTimeNanos != stats.CPUTimeNanos {
		return fmt.Errorf("native timing phases do not add up: %+v", stats)
	}
	if !idleFinished || idleAfter.InFlight != 0 || idleBefore.Submitted != idleAfter.Submitted || idleBefore.FrameTicks != idleAfter.FrameTicks || idleAfter.IdlePauses == 0 {
		return fmt.Errorf("idle clock did not stop: before=%+v after=%+v", idleBefore, idleAfter)
	}
	if stats.Submitted <= idleAfter.Submitted || stats.FrameRequests <= idleAfter.FrameRequests || stats.CoalescedRequests < *frames {
		return fmt.Errorf("frame coalescing or wake after idle failed: minimum_coalesced=%d idle_after=%+v renderer=%+v", *frames, idleAfter, stats)
	}
	if *glyphAtlas && (stats.GlyphRasterizations < 5 || stats.GlyphRasterizations > 14 || stats.GlyphCacheEntries != stats.GlyphRasterizations || stats.GlyphCacheHits < *frames*32*3 || stats.GlyphAtlasPages != 1 || stats.GlyphAtlasBytes != 1024*1024 || stats.GlyphAtlasPeakBytes != 1024*1024 || stats.GlyphAtlasEpochs != 0 || stats.GlyphUploadedBytes == 0 || stats.GlyphUploadedBytes > 10*1024*1024) {
		return fmt.Errorf("changing text did not reuse a bounded glyph atlas: %+v", stats)
	}
	if *glyphEviction && (stats.GlyphAtlasEpochs == 0 || stats.GlyphRasterizations < *frames*10 || stats.GlyphCacheHits < *frames*32*3 || stats.GlyphCacheEntries > 16384 || stats.GlyphAtlasPages > 16 || stats.GlyphAtlasBytes > 16*1024*1024 || stats.GlyphAtlasPeakBytes > 64*1024*1024 || stats.GlyphUploadedBytes < 16*1024*1024) {
		return fmt.Errorf("glyph eviction/resource ownership failed: %+v", stats)
	}
	var completedSinceRecovery uint64
	if *deviceRecovery || *metalRecovery {
		completedSinceRecovery = stats.Completed - completedBeforeRecovery
	}
	report := struct {
		Renderer       platform.RenderStats `json:"renderer"`
		Scene          string               `json:"scene"`
		Samples        int                  `json:"cpu_samples"`
		CPU50          uint64               `json:"cpu_p50_nanos"`
		CPU95          uint64               `json:"cpu_p95_nanos"`
		Scene95        uint64               `json:"scene_p95_nanos"`
		Acquire95      uint64               `json:"drawable_acquire_p95_nanos"`
		Encode95       uint64               `json:"encode_p95_nanos"`
		Elapsed        float64              `json:"elapsed_seconds"`
		IdleBefore     platform.RenderStats `json:"idle_before"`
		IdleAfter      platform.RenderStats `json:"idle_after"`
		BeforeRecovery uint64               `json:"completed_before_recovery,omitempty"`
		SinceRecovery  uint64               `json:"completed_since_recovery,omitempty"`
		CompletionRace bool                 `json:"diagnostic_completion_race,omitempty"`
		MetalRecovery  bool                 `json:"diagnostic_metal_recovery,omitempty"`
		WindowIdentity uint64               `json:"native_window_identity,omitempty"`
		ModelState     int                  `json:"go_model_state_after_recovery,omitempty"`
		Presentation   string               `json:"windows_presentation,omitempty"`
	}{Renderer: stats, Scene: fmt.Sprintf("2048 rounded quads + 32 text commands; changing colors; native GPU; glyph reuse=%t, eviction=%t", *glyphAtlas, *glyphEviction), Samples: len(samples), CPU50: percentile(samples, 50), CPU95: percentile(samples, 95), Scene95: percentile(sceneSamples, 95), Acquire95: percentile(acquireSamples, 95), Encode95: percentile(encodeSamples, 95), Elapsed: time.Since(started).Seconds(), IdleBefore: idleBefore, IdleAfter: idleAfter, BeforeRecovery: completedBeforeRecovery, SinceRecovery: completedSinceRecovery, CompletionRace: *completionRace, MetalRecovery: *metalRecovery, WindowIdentity: windowIdentity, ModelState: modelState, Presentation: windowPresentation}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if *output != "" {
		if err = os.WriteFile(*output, append(data, '\n'), 0644); err != nil {
			return err
		}
	}
	fmt.Println(string(data))
	return nil
}
