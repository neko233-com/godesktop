//go:build windows && cgo

// inputlatency observes an owned HWND's real, fenced GPU output after a native
// input message. It measures the complete UI pipeline plus consumer polling;
// it does not claim physical keyboard or scanout latency.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"time"

	ui "github.com/neko233-com/godesktop"
	"github.com/neko233-com/godesktop/internal/platform"
	"github.com/neko233-com/godesktop/internal/winprobe"
)

const title = "godesktop owned input-to-GPU probe"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func colorFor(sequence int) uint32 { return 0x203040 + uint32(sequence)*0x010101 }

func run() error {
	count := flag.Int("samples", 40, "native character events and actual fenced pixel observations (3..100)")
	output := flag.String("output", "", "optional JSON report file")
	flag.Parse()
	if *count < 3 || *count > 100 {
		return fmt.Errorf("samples must be between 3 and 100")
	}
	if err := os.Setenv("GODESKTOP_READBACK", "1"); err != nil {
		return err
	}
	ready, stopped := make(chan struct{}), make(chan struct{})
	results := make(chan struct {
		times []uint64
		err   error
	}, 1)
	var cx *ui.Context
	go func() {
		select {
		case <-ready:
		case <-stopped:
			return
		case <-time.After(15 * time.Second):
			platform.Quit()
			results <- struct {
				times []uint64
				err   error
			}{err: fmt.Errorf("native input probe did not start")}
			return
		}
		times, err := observe(*count)
		results <- struct {
			times []uint64
			err   error
		}{times, err}
		cx.Quit()
	}()
	sequence := 0
	err := ui.Run(ui.WindowOptions{Title: title, Width: 480, Height: 320, Input: func(_ *ui.Context, event ui.InputEvent) bool {
		if event.Kind == ui.Character && event.Key == 'a' {
			sequence++
			return true
		}
		return false
	}}, func(context *ui.Context) *ui.Element {
		if cx == nil {
			cx = context
			close(ready)
		}
		return ui.Column().Background(ui.RGB(colorFor(sequence)))
	})
	close(stopped)
	if err != nil {
		return err
	}
	result := <-results
	if result.err != nil {
		return result.err
	}
	stats := platform.RendererStats()
	if sequence != *count || stats.Backend != "direct3d12" || stats.Completed != stats.Submitted || stats.InFlight != 0 || stats.DroppedFrames != 0 || stats.DeviceRecoveries != 0 {
		return fmt.Errorf("input state or actual GPU submissions did not drain: sequence=%d renderer=%+v", sequence, stats)
	}
	sorted := append([]uint64(nil), result.times...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	report := struct {
		Scope          string               `json:"scope"`
		AdapterRequest string               `json:"adapter_request"`
		Samples        []uint64             `json:"message_to_fenced_pixel_samples_nanos"`
		P50            uint64               `json:"message_to_fenced_pixel_p50_nanos"`
		P95            uint64               `json:"message_to_fenced_pixel_p95_nanos"`
		Maximum        uint64               `json:"message_to_fenced_pixel_max_nanos"`
		Poll           uint64               `json:"consumer_poll_interval_nanos"`
		Renderer       platform.RenderStats `json:"renderer"`
	}{"WM_CHAR -> Go input/state -> native GPU submission -> completed HWND GPU readback; includes SendMessage and 1ms polling; no physical keyboard/scanout measurement", os.Getenv("GODESKTOP_GPU_ADAPTER"), result.times, sorted[(len(sorted)-1)/2], sorted[(len(sorted)-1)*95/100], sorted[len(sorted)-1], uint64(time.Millisecond), stats}
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

func observe(count int) ([]uint64, error) {
	window, err := winprobe.Find(title, uint32(os.Getpid()))
	if err != nil {
		return nil, err
	}
	await := func(expected uint32) error {
		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			pixel, err := window.Pixel(24, 24)
			if err == nil && pixel == expected {
				return nil
			}
			time.Sleep(time.Millisecond)
		}
		return fmt.Errorf("owned HWND GPU pixel did not reach %#x within one second", expected)
	}
	if err := await(colorFor(0)); err != nil {
		return nil, err
	}
	times := make([]uint64, 0, count)
	for sequence := 1; sequence <= count; sequence++ {
		started := time.Now()
		if err := window.Send(0x0102, 'a', 1); err != nil {
			return nil, err
		}
		if err := await(colorFor(sequence)); err != nil {
			return nil, err
		}
		times = append(times, uint64(time.Since(started)))
	}
	return times, nil
}
