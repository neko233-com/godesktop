//go:build windows && cgo

// colortest reads only its owned HWND's completed D3D12 render target.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"github.com/neko233-com/godesktop/internal/platform"
	"github.com/neko233-com/godesktop/testing/winprobe"
)

const title = "godesktop Windows color glyph acceptance"
const text = "😀 👩🏽‍💻 ❤️"

func commands() []platform.Command {
	var result []platform.Command
	clip := platform.Rect{W: 640, H: 450}
	for i, tint := range []platform.Color{{R: 1, A: 1}, {B: 1, A: 1}, {G: 1, A: .5}} {
		result = append(result, platform.Command{Kind: platform.Label, Bounds: platform.Rect{X: 16, Y: float32(16 + i*64), W: 600, H: 64}, Clip: clip, Color: tint, FontSize: 32, Text: text})
	}
	return append(result,
		platform.Command{Kind: platform.Label, Bounds: platform.Rect{X: 16, Y: 208, W: 600, H: 64}, Clip: clip, Color: platform.Color{R: 1, A: 1}, FontSize: 32, Text: "Go " + text + " 界"},
		platform.Command{Kind: platform.Label, Bounds: platform.Rect{X: 16, Y: 336, W: 600, H: 64}, Clip: platform.Rect{X: 16, Y: 336, W: 20, H: 64}, Color: platform.Color{B: 1, A: 1}, FontSize: 32, Text: text},
	)
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	output := flag.String("output", ".cache/color-windows", "owned HWND PNG/JSON directory")
	flag.Parse()
	if err := os.MkdirAll(*output, 0755); err != nil {
		return err
	}
	if err := os.Setenv("GODESKTOP_READBACK", "1"); err != nil {
		return err
	}
	watchdog := time.AfterFunc(60*time.Second, func() { fmt.Fprintln(os.Stderr, "color acceptance timed out"); os.Exit(2) })
	defer watchdog.Stop()
	for run := 0; run < 2; run++ {
		os.Unsetenv("GODESKTOP_TEST_DEVICE_REMOVAL")
		if run == 0 {
			os.Setenv("GODESKTOP_TEST_DEVICE_REMOVAL", "7")
		}
		phase, phaseFrame := 0, uint64(0)
		var pressureEpoch uint64
		var scale float64
		var failed error
		err := platform.Run(platform.Options{Title: title, Width: 640, Height: 450, Background: platform.Color{A: 1}}, func(event platform.Event) {
			if event.Kind != platform.Draw {
				return
			}
			stats := platform.RendererStats()
			if stats.GlyphAtlasBytes > 64<<20 || stats.GlyphAtlasPeakBytes > 64<<20 {
				failed = fmt.Errorf("in-flight atlas exceeded 64 MiB: %+v", stats)
			}
			if failed != nil {
				platform.Quit()
				return
			}
			if phase == 0 && stats.Completed >= 8 || phase == 1 && stats.Completed >= phaseFrame+8 || phase == 3 && stats.Completed >= phaseFrame+8 {
				window, findErr := winprobe.Find(title, uint32(os.Getpid()))
				if findErr != nil {
					failed = findErr
					platform.Quit()
					return
				}
				captured, captureErr := window.Capture()
				if captureErr != nil {
					failed = captureErr
					platform.Quit()
					return
				}
				if phase == 0 {
					scale = float64(captured.Bounds().Dx()) / 640
				}
				failed = verify(*output, fmt.Sprintf("run-%d-phase-%d", run, phase), captured, scale, stats)
				if failed != nil || run == 1 || phase == 3 {
					platform.Quit()
					return
				}
				phaseFrame = stats.Submitted
				if phase == 0 {
					// Resize only this owned HWND, retaining its rendering density.
					failed = window.Resize(int(720*scale*96/float64(window.DPI())), int(480*scale*96/float64(window.DPI())))
					phase = 1
				} else {
					phase = 2
					pressureEpoch = stats.GlyphAtlasEpochs
				}
			}
			if phase == 2 {
				if stats.GlyphAtlasEpochs >= pressureEpoch+2 {
					phase = 3
					phaseFrame = stats.Submitted
				} else {
					platform.Present([]platform.Command{{Kind: platform.Label, Bounds: platform.Rect{X: 16, Y: 16, W: 600, H: 400}, Clip: platform.Rect{W: 640, H: 450}, Color: platform.Color{R: 1, A: 1}, FontSize: float32(180 + stats.Submitted%50), Text: text}})
					platform.Wake()
					return
				}
			}
			platform.Present(commands())
			platform.Wake()
		})
		if err != nil {
			return err
		}
		if failed != nil {
			return failed
		}
		stats := platform.RendererStats()
		if stats.Completed+stats.DroppedFrames != stats.Submitted || stats.InFlight != 0 || stats.DrawCalls != 1 || stats.GlyphAtlasBytes > 16<<20 || stats.GlyphAtlasPages > 16 {
			return fmt.Errorf("mixed glyph cache did not drain: %+v", stats)
		}
		if run == 0 && (phase != 3 || stats.DeviceRecoveries != 1 || stats.GlyphAtlasEpochs < pressureEpoch+2 || stats.DroppedFrames < 1 || stats.DroppedFrames > 3) {
			return fmt.Errorf("actual recovery/eviction missing: %+v", stats)
		}
		if run == 1 && (stats.DeviceRecoveries != 0 || stats.GlyphAtlasEpochs != 0 || stats.GlyphAtlasBytes != 2<<20 || stats.DroppedFrames != 0) {
			return fmt.Errorf("Run did not reset color cache: %+v", stats)
		}
	}
	return nil
}

func savePNG(path string, pixels *image.RGBA) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	err = png.Encode(file, pixels)
	closeErr := file.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func verify(output, name string, captured *image.RGBA, scale float64, stats platform.RenderStats) error {
	if err := savePNG(filepath.Join(output, name+".png"), captured); err != nil {
		return err
	}
	reference, err := platform.Direct3D12TextReference(text, "", 32, float32(scale), int(600*scale), int(64*scale))
	if err != nil {
		return err
	}
	if err = savePNG(filepath.Join(output, "color-glyphs-direct2d-reference.png"), reference); err != nil {
		return err
	}
	report := struct {
		Scale                                                                         float64
		Renderer                                                                      platform.RenderStats
		ColorPixels, TintDifference, OpacityDifference                                int
		ReferencePixels, ReferenceMatches, ClipPixels, ClipOutside, OrdinaryRedPixels int
		ReferenceFraction                                                             float64
		MaskIntersection, MaskUnion                                                   int
		MaskIoU                                                                       float64
	}{Scale: scale, Renderer: stats}
	for y := 0; y < int(64*scale); y++ {
		for x := 0; x < int(600*scale); x++ {
			first := captured.RGBAAt(int(16*scale)+x, int(16*scale)+y)
			second := captured.RGBAAt(int(16*scale)+x, int(80*scale)+y)
			half := captured.RGBAAt(int(16*scale)+x, int(144*scale)+y)
			v1 := [3]byte{first.R, first.G, first.B}
			v2 := [3]byte{second.R, second.G, second.B}
			vh := [3]byte{half.R, half.G, half.B}
			for i, a := range v1 {
				report.TintDifference = max(report.TintDifference, abs(int(a)-int(v2[i])))
				report.OpacityDifference = max(report.OpacityDifference, abs(int(a)/2-int(vh[i])))
			}
			if first.R > 100 && first.G > 40 && int(first.R) > int(first.B)+30 {
				report.ColorPixels++
			}
			r := reference.RGBAAt(x, y)
			wantInk := max(r.R, r.G, r.B) > 32
			gotInk := max(first.R, first.G, first.B) > 32
			if wantInk || gotInk {
				report.MaskUnion++
			}
			if wantInk && gotInk {
				report.MaskIntersection++
			}
			if r.G > 32 || r.B > 32 {
				report.ReferencePixels++
				// Whole-layout and cached glyph drawing can differ in subpixel phase.
				// Compare within one physical DIP; placement/clip have separate gates.
				best := 255
				for dy := -int(scale + 0.5); dy <= int(scale+0.5); dy++ {
					for dx := -int(scale + 0.5); dx <= int(scale+0.5); dx++ {
						got := captured.RGBAAt(int(16*scale)+x+dx, int(16*scale)+y+dy)
						best = min(best, max(abs(int(r.R)-int(got.R)), abs(int(r.G)-int(got.G)), abs(int(r.B)-int(got.B))))
					}
				}
				if best <= 24 {
					report.ReferenceMatches++
				}
			}
		}
	}
	for y := int(336 * scale); y < int(400*scale); y++ {
		for x := 0; x < int(600*scale); x++ {
			p := captured.RGBAAt(x, y)
			if p.R > 8 || p.G > 8 || p.B > 8 {
				if x >= int(16*scale) && x < int(36*scale) {
					report.ClipPixels++
				} else {
					report.ClipOutside++
				}
			}
		}
	}
	for y := int(208 * scale); y < int(272*scale); y++ {
		for x := int(16 * scale); x < int(65*scale); x++ {
			p := captured.RGBAAt(x, y)
			if p.R > 64 && p.G < 4 && p.B < 4 {
				report.OrdinaryRedPixels++
			}
		}
	}
	if report.ReferencePixels > 0 {
		report.ReferenceFraction = float64(report.ReferenceMatches) / float64(report.ReferencePixels)
	}
	if report.MaskUnion > 0 {
		report.MaskIoU = float64(report.MaskIntersection) / float64(report.MaskUnion)
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(output, name+".json"), append(data, '\n'), 0644); err != nil {
		return err
	}
	if report.ColorPixels < 100 || report.TintDifference > 2 || report.OpacityDifference > 2 || report.ReferenceFraction < .95 || report.MaskIoU < .85 || report.ClipPixels < 10 || report.ClipOutside != 0 || report.OrdinaryRedPixels < 20 {
		return fmt.Errorf("native color/reference/opacity/clip differs: %+v", report)
	}
	if stats.DrawCalls != 1 || stats.GlyphAtlasPages < 2 || stats.GlyphAtlasPeakBytes > 64<<20 {
		return fmt.Errorf("mixed glyph batching/residency differs: %+v", stats)
	}
	fmt.Printf("%s: color=%d tint=%d opacity=%d reference=%f pages=%d bytes=%d epochs=%d recovery=%d\n", name, report.ColorPixels, report.TintDifference, report.OpacityDifference, report.ReferenceFraction, stats.GlyphAtlasPages, stats.GlyphAtlasBytes, stats.GlyphAtlasEpochs, stats.DeviceRecoveries)
	return nil
}
func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
