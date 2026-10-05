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
	output := flag.String("output", "", "optional JSON report filename")
	flag.Parse()
	if *frames < 6 || *frames > 10000 {
		return errors.New("frames must be between 6 and 10000")
	}
	watchdog := time.AfterFunc(30*time.Second, func() { fmt.Fprintln(os.Stderr, "GPU stress watchdog expired"); os.Exit(1) })
	defer watchdog.Stop()
	var samples []uint64
	var previous uint64
	var mismatch error
	started := time.Now()
	err := ui.Run(ui.WindowOptions{Title: "godesktop native GPU stress", Width: 1000, Height: 650}, func(cx *ui.Context) *ui.Element {
		stats := platform.RendererStats()
		if stats.Backend != *requireBackend {
			mismatch = fmt.Errorf("required %s renderer, got %s", *requireBackend, stats.Backend)
			cx.Quit()
			return nil
		}
		if stats.Submitted > previous {
			previous = stats.Submitted
			samples = append(samples, stats.CPUTimeNanos)
		}
		if stats.Completed >= *frames {
			cx.Quit()
		} else {
			cx.Invalidate()
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
			cells = append(cells, ui.Text("GPU").Width(50).Height(16).FontSize(12))
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
	if stats.Instances < 2000 || stats.DrawCalls == 0 || stats.DrawCalls > 2 || stats.UploadedBytes != stats.Instances*80 {
		return fmt.Errorf("instancing/batching failed: %+v", stats)
	}
	if len(samples) < 3 || percentile(samples, 95) == 0 {
		return errors.New("native frame CPU timings were not collected")
	}
	report := struct {
		Renderer platform.RenderStats `json:"renderer"`
		Scene    string               `json:"scene"`
		Samples  int                  `json:"cpu_samples"`
		CPU50    uint64               `json:"cpu_p50_nanos"`
		CPU95    uint64               `json:"cpu_p95_nanos"`
		Elapsed  float64              `json:"elapsed_seconds"`
	}{stats, "2048 rounded quads + 32 shared-text commands; changing colors; native GPU", len(samples), percentile(samples, 50), percentile(samples, 95), time.Since(started).Seconds()}
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
