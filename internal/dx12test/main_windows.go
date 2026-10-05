//go:build windows && cgo

// dx12test validates real DXIL execution through fenced GPU readback, independent
// of desktop composition, display drivers, HWND visibility or GDI capture.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/neko233-com/godesktop/internal/platform"
)

type nativeReport struct {
	Backend            string   `json:"backend"`
	Adapter            string   `json:"adapter"`
	Software           bool     `json:"software"`
	VendorID           uint32   `json:"vendor_id"`
	DeviceID           uint32   `json:"device_id"`
	AdapterFlags       uint32   `json:"adapter_flags"`
	DebugLayer         bool     `json:"debug_layer"`
	GPUValidation      bool     `json:"gpu_validation"`
	ShaderModel        string   `json:"shader_model"`
	FrameSlots         uint32   `json:"frame_slots"`
	UsedSlots          uint32   `json:"used_slots_mask"`
	MaxInFlight        uint32   `json:"max_in_flight"`
	Submitted          uint64   `json:"submitted"`
	Completed          uint64   `json:"completed"`
	OwnershipDeferrals uint64   `json:"ownership_deferrals"`
	ReadbackWaits      uint64   `json:"diagnostic_readback_waits"`
	Instances          uint32   `json:"instances_per_frame"`
	DrawCalls          uint32   `json:"draw_calls_per_frame"`
	InstanceBytes      uint32   `json:"instance_bytes_per_frame"`
	CPU                []uint64 `json:"cpu_samples_nanos"`
	GPU                []uint64 `json:"gpu_samples_nanos"`
}

func quantile(values []uint64, percent int) uint64 {
	copyOfValues := append([]uint64(nil), values...)
	sort.Slice(copyOfValues, func(i, j int) bool { return copyOfValues[i] < copyOfValues[j] })
	return copyOfValues[(len(copyOfValues)-1)*percent/100]
}

func pixel(probe platform.GPUProbe, frame, x, y int) color.NRGBA {
	offset := (frame*probe.Height+y)*probe.Stride + x*4
	p := probe.Pixels[offset : offset+4]
	return color.NRGBA{R: p[2], G: p[1], B: p[0], A: p[3]}
}

func validate(probe platform.GPUProbe, report nativeReport) (int, error) {
	if probe.Width != 192 || probe.Height != 128 || probe.Stride != 768 || probe.Frames < 6 || len(probe.Pixels) != probe.Frames*probe.Height*probe.Stride {
		return 0, errors.New("invalid native GPU readback dimensions")
	}
	if report.Backend != "direct3d12" || report.Adapter == "" || report.ShaderModel != "6.0" || report.Submitted != uint64(probe.Frames) || report.Completed != report.Submitted {
		return 0, fmt.Errorf("invalid native submission report: %+v", report)
	}
	if report.VendorID == 0x1414 && report.DeviceID == 0x8c && !report.Software {
		return 0, errors.New("Microsoft Basic Render Driver was incorrectly reported as hardware")
	}
	if report.FrameSlots != 3 || report.UsedSlots != 7 || report.MaxInFlight != 3 || report.OwnershipDeferrals != 1 || report.Instances != 8 || report.DrawCalls != 1 || report.InstanceBytes != 640 {
		return 0, fmt.Errorf("GPU instance or in-flight ownership validation failed: %+v", report)
	}
	if len(report.CPU) != probe.Frames || len(report.GPU) != probe.Frames || quantile(report.CPU, 95) == 0 || quantile(report.GPU, 95) == 0 {
		return 0, errors.New("native CPU/GPU timestamps missing")
	}
	checks := 0
	for frame := range probe.Frames {
		base, blend := color.NRGBA{26, 77, 204, 255}, color.NRGBA{140, 38, 102, 255}
		if frame%2 != 0 {
			base, blend = color.NRGBA{204, 51, 26, 255}, color.NRGBA{230, 26, 13, 255}
		}
		// Expected samples express painter order and raster coverage directly;
		// they do not call the instance builder or share the HLSL distance code.
		samples := []struct {
			name string
			x, y int
			want color.NRGBA
		}{
			{"clear", 0, 0, color.NRGBA{13, 26, 51, 255}},
			{"instance stride and changing color", 12, 12, base},
			{"premultiplied alpha", 40, 28, blend},
			{"clip left", 31, 24, base},
			{"clip top", 34, 18, base},
			{"clip right", 50, 24, base},
			{"rounded corner", 80, 8, color.NRGBA{13, 26, 51, 255}},
			{"rounded interior", 100, 28, color.NRGBA{0, 255, 0, 255}},
			{"horizontal line", 32, 80, color.NRGBA{255, 255, 255, 255}},
			{"outside line", 32, 84, color.NRGBA{13, 26, 51, 255}},
			{"R8 half coverage", 84, 80, color.NRGBA{134, 141, 153, 255}},
			{"R8 full coverage", 108, 80, color.NRGBA{255, 255, 255, 255}},
			{"empty clip", 165, 55, color.NRGBA{13, 26, 51, 255}},
			{"diagonal line", 148, 28, color.NRGBA{255, 255, 0, 255}},
			{"last instance", 156, 98, color.NRGBA{255, 128, 0, 255}},
		}
		for _, sample := range samples {
			got := pixel(probe, frame, sample.x, sample.y)
			channels := [][2]uint8{{got.R, sample.want.R}, {got.G, sample.want.G}, {got.B, sample.want.B}, {got.A, sample.want.A}}
			for _, pair := range channels {
				if difference := int(pair[0]) - int(pair[1]); difference < -3 || difference > 3 {
					return checks, fmt.Errorf("GPU frame %d %s at (%d,%d): got %v want %v", frame, sample.name, sample.x, sample.y, got, sample.want)
				}
			}
			checks++
		}
		antialias := pixel(probe, frame, 83, 11)
		if antialias.G < 60 || antialias.G > 205 || antialias.R > 16 || antialias.B > 54 || antialias.A != 255 {
			return checks, fmt.Errorf("GPU frame %d rounded-edge antialias missing: %v", frame, antialias)
		}
		checks++
	}
	return checks, nil
}

func writePNG(path string, probe platform.GPUProbe, frame int) error {
	img := image.NewNRGBA(image.Rect(0, 0, probe.Width, probe.Height))
	for y := range probe.Height {
		for x := range probe.Width {
			img.SetNRGBA(x, y, pixel(probe, frame, x, y))
		}
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	encodeErr := png.Encode(file, img)
	return errors.Join(encodeErr, file.Close())
}

func run() error {
	frames := flag.Uint("frames", 90, "GPU frames to validate, between 6 and 240")
	hardware := flag.Bool("require-hardware", false, "fail if no hardware D3D12 / SM6 adapter is available")
	warp := flag.Bool("warp", false, "explicitly validate the D3D12 WARP software adapter")
	debug := flag.Bool("debug", true, "enable debug layer and GPU validation when installed")
	requireDebug := flag.Bool("require-debug-layer", false, "fail if the D3D12 debug layer is unavailable")
	output := flag.String("output", "", "optional artifact directory for JSON and actual GPU images")
	flag.Parse()
	if *frames < 6 || *frames > 240 || (*hardware && *warp) {
		return errors.New("invalid GPU probe options")
	}
	var flags uint32
	if *hardware {
		flags |= 1
	}
	if *warp {
		flags |= 2
	}
	if *debug {
		flags |= 4
	}
	if *requireDebug {
		flags |= 8
	}
	started := time.Now()
	probe, err := platform.Direct3D12Probe(flags, uint32(*frames))
	if err != nil {
		return err
	}
	var native nativeReport
	if err = json.Unmarshal(probe.Report, &native); err != nil {
		return err
	}
	if (*hardware && native.Software) || (*warp && !native.Software) || (*requireDebug && !native.DebugLayer) {
		return errors.New("GPU probe used a device or debug layer that the command forbids")
	}
	checks, err := validate(probe, native)
	if err != nil {
		return err
	}
	if *output != "" {
		if err = os.MkdirAll(*output, 0755); err != nil {
			return err
		}
		if err = writePNG(filepath.Join(*output, "first-gpu-frame.png"), probe, 0); err != nil {
			return err
		}
		if err = writePNG(filepath.Join(*output, "last-gpu-frame.png"), probe, probe.Frames-1); err != nil {
			return err
		}
	}
	report := struct {
		Native      nativeReport `json:"native"`
		Scope       string       `json:"scope"`
		PixelChecks int          `json:"pixel_checks"`
		CPU50       uint64       `json:"cpu_submit_p50_nanos"`
		CPU95       uint64       `json:"cpu_submit_p95_nanos"`
		GPU50       uint64       `json:"gpu_render_p50_nanos"`
		GPU95       uint64       `json:"gpu_render_p95_nanos"`
		Elapsed     float64      `json:"elapsed_seconds"`
	}{native, "Offscreen DXIL/R8 coverage and three-slot ownership; HWND and font atlas acceptance use separate native window tests", checks, quantile(native.CPU, 50), quantile(native.CPU, 95), quantile(native.GPU, 50), quantile(native.GPU, 95), time.Since(started).Seconds()}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if *output != "" {
		if err = os.WriteFile(filepath.Join(*output, "gpu-validation.json"), append(data, '\n'), 0644); err != nil {
			return err
		}
	}
	fmt.Println(string(data))
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
