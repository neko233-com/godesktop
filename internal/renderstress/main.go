// renderstress exercises actual native submissions rather than Go layout alone.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
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
	requireClock := flag.String("require-frame-clock", "cametaldisplaylink", "required native frame clock")
	glyphAtlas := flag.Bool("glyph-atlas", false, "vary text every frame and require bounded per-glyph reuse")
	glyphEviction := flag.Bool("glyph-eviction", false, "vary large font sizes to exercise atlas eviction and GPU lifetime")
	output := flag.String("output", "", "optional JSON report filename")
	flag.Parse()
	if *frames < 6 || *frames > 10000 {
		return errors.New("frames must be between 6 and 10000")
	}
	if *glyphAtlas && *glyphEviction {
		return errors.New("choose glyph reuse or glyph eviction validation")
	}
	watchdog := time.AfterFunc(30*time.Second, func() { fmt.Fprintln(os.Stderr, "GPU stress watchdog expired"); os.Exit(1) })
	defer watchdog.Stop()
	var samples []uint64
	var sceneSamples, acquireSamples, encodeSamples []uint64
	var previous uint64
	var mismatch error
	var idleStarted, idleFinished bool
	var idleBefore, idleAfter platform.RenderStats
	started := time.Now()
	err := ui.Run(ui.WindowOptions{Title: "godesktop native GPU stress", Width: 1000, Height: 650}, func(cx *ui.Context) *ui.Element {
		stats := platform.RendererStats()
		if stats.Backend != *requireBackend {
			mismatch = fmt.Errorf("required %s renderer, got %s", *requireBackend, stats.Backend)
			cx.Quit()
			return nil
		}
		if stats.FrameClock != *requireClock {
			mismatch = fmt.Errorf("required %s frame clock, got %s", *requireClock, stats.FrameClock)
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
			cx.Quit()
		} else if stats.Completed >= *frames {
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
			for range 3 {
				cx.Invalidate()
			}
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
	if stats.Submitted < *frames || stats.Completed != stats.Submitted || stats.InFlight != 0 {
		return fmt.Errorf("submissions did not drain: %+v", stats)
	}
	if stats.FrameSlots != 3 || stats.UsedSlotsMask != 7 || stats.MaxInFlight == 0 || stats.MaxInFlight > 3 || stats.BufferWaits != 0 {
		return fmt.Errorf("three-slot asynchronous ownership failed: %+v", stats)
	}
	if stats.Instances < 2000 || stats.DrawCalls == 0 || (!*glyphEviction && stats.DrawCalls > 2) || stats.UploadedBytes != stats.Instances*80 {
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
		return fmt.Errorf("frame coalescing or wake after idle failed: %+v", stats)
	}
	if *glyphAtlas && (stats.GlyphRasterizations < 5 || stats.GlyphRasterizations > 14 || stats.GlyphCacheEntries != stats.GlyphRasterizations || stats.GlyphCacheHits < *frames*32*3 || stats.GlyphAtlasPages != 1 || stats.GlyphAtlasBytes != 1024*1024 || stats.GlyphAtlasPeakBytes != 1024*1024 || stats.GlyphAtlasEpochs != 0 || stats.GlyphUploadedBytes == 0 || stats.GlyphUploadedBytes > 10*1024*1024) {
		return fmt.Errorf("changing text did not reuse a bounded glyph atlas: %+v", stats)
	}
	if *glyphEviction && (stats.GlyphAtlasEpochs == 0 || stats.GlyphRasterizations < *frames*10 || stats.GlyphCacheHits < *frames*32*3 || stats.GlyphCacheEntries > 16384 || stats.GlyphAtlasPages > 16 || stats.GlyphAtlasBytes > 16*1024*1024 || stats.GlyphAtlasPeakBytes > 64*1024*1024 || stats.GlyphUploadedBytes < 16*1024*1024) {
		return fmt.Errorf("glyph eviction/resource ownership failed: %+v", stats)
	}
	report := struct {
		Renderer   platform.RenderStats `json:"renderer"`
		Scene      string               `json:"scene"`
		Samples    int                  `json:"cpu_samples"`
		CPU50      uint64               `json:"cpu_p50_nanos"`
		CPU95      uint64               `json:"cpu_p95_nanos"`
		Scene95    uint64               `json:"scene_p95_nanos"`
		Acquire95  uint64               `json:"drawable_acquire_p95_nanos"`
		Encode95   uint64               `json:"encode_p95_nanos"`
		Elapsed    float64              `json:"elapsed_seconds"`
		IdleBefore platform.RenderStats `json:"idle_before"`
		IdleAfter  platform.RenderStats `json:"idle_after"`
	}{Renderer: stats, Scene: fmt.Sprintf("2048 rounded quads + 32 text commands; changing colors; native GPU; glyph reuse=%t, eviction=%t", *glyphAtlas, *glyphEviction), Samples: len(samples), CPU50: percentile(samples, 50), CPU95: percentile(samples, 95), Scene95: percentile(sceneSamples, 95), Acquire95: percentile(acquireSamples, 95), Encode95: percentile(encodeSamples, 95), Elapsed: time.Since(started).Seconds(), IdleBefore: idleBefore, IdleAfter: idleAfter}
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
