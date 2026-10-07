//go:build darwin && cgo

package main

import (
	"encoding/json"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"time"

	"github.com/neko233-com/godesktop/internal/platform"
)

const colorText = "😀 🇨🇳 👩🏽‍💻 ❤️"

type colorReport struct {
	Scene             sceneReport `json:"scene"`
	IntrinsicPixels   int         `json:"intrinsic_color_pixels"`
	OpaqueReference   int         `json:"opaque_reference_pixels"`
	ReferenceMatches  int         `json:"reference_color_matches"`
	ReferenceFraction float64     `json:"reference_color_match_fraction"`
	TintDifference    int         `json:"maximum_foreground_tint_difference"`
	OpacityDifference int         `json:"maximum_half_opacity_difference"`
}

func colorCommands() []platform.Command {
	clip := platform.Rect{W: 640, H: 450}
	var commands []platform.Command
	for i, tint := range []platform.Color{{R: 1, A: 1}, {B: 1, A: 1}, {G: 1, A: .5}} {
		commands = append(commands, platform.Command{Kind: platform.Label, Bounds: platform.Rect{X: 16, Y: float32(16 + i*64), W: 600, H: 64}, Clip: clip, Color: tint, FontSize: 32, Text: colorText})
	}
	commands = append(commands,
		platform.Command{Kind: platform.Label, Bounds: platform.Rect{X: 16, Y: 208, W: 600, H: 64}, Clip: clip, Color: platform.Color{R: 1, A: 1}, FontSize: 32, Text: "Go " + colorText + " 界"},
		platform.Command{Kind: platform.Label, Bounds: platform.Rect{X: 16, Y: 336, W: 600, H: 64}, Clip: platform.Rect{X: 16, Y: 336, W: 20, H: 64}, Color: platform.Color{B: 1, A: 1}, FontSize: 32, Text: colorText},
	)
	return commands
}

func validateColors(output string) ([]platform.Command, error) {
	commands := colorCommands()
	pixels, scene, err := render("intrinsic-color-glyphs", commands)
	if err != nil {
		return nil, err
	}
	if err = savePNG(filepath.Join(output, "color-glyphs-gpu.png"), pixels); err != nil {
		return nil, err
	}
	report, reference, checkErr := compareColors(pixels, scene)
	if reference != nil {
		if err = savePNG(filepath.Join(output, "color-glyphs-coretext-reference.png"), reference); err != nil {
			return nil, err
		}
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, err
	}
	if err = os.WriteFile(filepath.Join(output, "color-glyphs.json"), append(data, '\n'), 0644); err != nil {
		return nil, err
	}
	if checkErr != nil {
		return nil, checkErr
	}
	stats := scene.Renderer
	if stats.DrawCalls != 1 || stats.GlyphAtlasPages < 2 || stats.GlyphAtlasPages > 16 || stats.GlyphAtlasBytes > 16*1024*1024 || stats.GlyphAtlasPeakBytes > 64*1024*1024 || stats.GlyphUploadedBytes != stats.GlyphAtlasBytes {
		return nil, fmt.Errorf("mixed R8/RGBA glyph batching or residency differs: %+v", stats)
	}
	if err = validateColorEviction(output, commands); err != nil {
		return nil, err
	}
	return commands, nil
}

func validateColorEviction(output string, finalCommands []platform.Command) error {
	watchdog := time.AfterFunc(30*time.Second, func() { platform.Quit() })
	defer watchdog.Stop()
	var stableAt uint64
	err := platform.Run(platform.Options{Title: "godesktop color glyph eviction", Width: 640, Height: 450, Background: platform.Color{A: 1}}, func(event platform.Event) {
		if event.Kind != platform.Draw {
			return
		}
		stats := platform.RendererStats()
		if stats.GlyphAtlasEpochs >= 2 {
			if stableAt == 0 {
				stableAt = stats.Submitted
			}
			platform.Present(finalCommands)
			if stats.Completed >= stableAt+7 {
				platform.Quit()
				return
			}
		} else {
			// Different large actual color fonts fill RGBA pages across frames.
			// One frame stays below budget; old submitted pages must remain alive
			// until completion while a new bounded atlas epoch is populated.
			platform.Present([]platform.Command{{Kind: platform.Label, Bounds: platform.Rect{X: 16, Y: 16, W: 600, H: 400}, Clip: platform.Rect{W: 640, H: 450}, Color: platform.Color{R: 1, A: 1}, FontSize: float32(180 + stats.Submitted%50), Text: colorText}})
		}
		platform.Wake()
	})
	if err != nil {
		return err
	}
	stats := platform.RendererStats()
	if stableAt == 0 || stats.Completed < stableAt+7 || stats.Completed != stats.Submitted || stats.InFlight != 0 || stats.DroppedFrames != 0 || stats.GlyphAtlasEpochs < 2 || stats.GlyphAtlasPages > 16 || stats.GlyphAtlasBytes > 16*1024*1024 || stats.GlyphAtlasPeakBytes > 64*1024*1024 || stats.DrawCalls != 1 {
		return fmt.Errorf("color glyph eviction did not drain bounded mixed pages: %+v", stats)
	}
	pixels, frame, err := platform.MetalSnapshot()
	if err != nil {
		return err
	}
	if err = savePNG(filepath.Join(output, "color-glyphs-after-eviction-gpu.png"), pixels); err != nil {
		return err
	}
	report, _, checkErr := compareColors(pixels, sceneReport{Frame: frame, Scale: float32(pixels.Bounds().Dx()) / 640, Renderer: stats})
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(output, "color-glyphs-eviction.json"), append(data, '\n'), 0644); err != nil {
		return err
	}
	return checkErr
}

func compareColors(pixels *image.RGBA, scene sceneReport) (colorReport, *image.RGBA, error) {
	report := colorReport{Scene: scene}
	scale := scene.Scale
	width, height := physical(600, scale), physical(64, scale)
	reference, err := platform.MetalTextReference(colorText, "", 32, scale, width, height)
	if err != nil {
		return report, nil, err
	}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			a := pixels.RGBAAt(physical(16, scale)+x, physical(16, scale)+y)
			b := pixels.RGBAAt(physical(16, scale)+x, physical(80, scale)+y)
			half := pixels.RGBAAt(physical(16, scale)+x, physical(144, scale)+y)
			report.TintDifference = max(report.TintDifference, difference(a.R, b.R), difference(a.G, b.G), difference(a.B, b.B))
			report.OpacityDifference = max(report.OpacityDifference, differenceInt(int(a.R)/2, int(half.R)), differenceInt(int(a.G)/2, int(half.G)), differenceInt(int(a.B)/2, int(half.B)))
			if max(a.R, a.G, a.B)-min(a.R, a.G, a.B) > 40 && a.G > 30 {
				report.IntrinsicPixels++
			}
			want := reference.RGBAAt(x, y)
			if want.A < 240 {
				continue
			}
			report.OpaqueReference++
			// Independent whole-line CoreText and cached per-glyph rasterization
			// can differ in subpixel phase. Compare opaque RGB in a small local
			// neighborhood; the existing Unicode mask check covers placement.
			best := 255
			for dy := -physical(1, scale); dy <= physical(1, scale); dy++ {
				for dx := -physical(1, scale); dx <= physical(1, scale); dx++ {
					got := pixels.RGBAAt(physical(16, scale)+x+dx, physical(16, scale)+y+dy)
					best = min(best, max(difference(got.R, want.R), difference(got.G, want.G), difference(got.B, want.B)))
				}
			}
			if best <= 24 {
				report.ReferenceMatches++
			}
		}
	}
	if report.OpaqueReference > 0 {
		report.ReferenceFraction = float64(report.ReferenceMatches) / float64(report.OpaqueReference)
	}
	if report.IntrinsicPixels < 100 || report.OpaqueReference < 100 || report.ReferenceFraction < .80 || report.TintDifference > 3 || report.OpacityDifference > 3 {
		return report, reference, fmt.Errorf("intrinsic emoji color/opacity differs from CoreText: %+v", report)
	}
	// Ordinary glyphs in the same draw must still follow the label's red tint.
	redInk := 0
	for y := physical(208, scale); y < physical(272, scale); y++ {
		for x := physical(16, scale); x < physical(44, scale); x++ {
			c := pixels.RGBAAt(x, y)
			if c.R > 32 && c.G < 4 && c.B < 4 {
				redInk++
			}
		}
	}
	if redInk < 20 {
		return report, reference, fmt.Errorf("mixed ordinary text lost foreground tint: %d red pixels", redInk)
	}
	for y := physical(336, scale); y < physical(400, scale); y++ {
		for x := physical(37, scale); x < physical(616, scale); x++ {
			c := pixels.RGBAAt(x, y)
			if c.R > 3 || c.G > 3 || c.B > 3 {
				return report, reference, fmt.Errorf("color glyph escaped clip at (%d,%d)", x, y)
			}
		}
	}
	return report, reference, nil
}
