//go:build darwin && cgo

// metaltest validates the default AppKit renderer by copying its actual drawable
// after GPU completion. No desktop capture or replacement offscreen renderer is used.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/neko233-com/godesktop/internal/platform"
)

type sceneReport struct {
	Frame    uint64               `json:"snapshot_frame"`
	Scale    float32              `json:"scale"`
	Renderer platform.RenderStats `json:"renderer"`
}

type textReport struct {
	Text       string  `json:"text"`
	InkPixels  int     `json:"ink_pixels"`
	Reference  int     `json:"reference_ink_pixels"`
	MaskIOU    float64 `json:"mask_intersection_over_union"`
	BoundsDiff int     `json:"maximum_bounds_difference_pixels"`
}

func main() {
	output := flag.String("output", "bin/metal-pixels", "directory for actual GPU PNGs and diagnostics")
	drawableScale := flag.Float64("drawable-scale", 0, "0 uses system density; 1..4 diagnostically override the actual drawable without changing display hardware")
	flag.Parse()
	var environmentErr error
	if *drawableScale == 0 {
		environmentErr = os.Unsetenv("GODESKTOP_TEST_DRAWABLE_SCALE")
	} else {
		environmentErr = os.Setenv("GODESKTOP_TEST_DRAWABLE_SCALE", fmt.Sprint(*drawableScale))
	}
	if environmentErr != nil {
		fmt.Fprintln(os.Stderr, environmentErr)
		os.Exit(1)
	}
	if err := validate(*output, float32(*drawableScale)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func validate(output string, requestedScale float32) error {
	if err := os.MkdirAll(output, 0755); err != nil {
		return err
	}
	if err := os.Setenv("GODESKTOP_READBACK", "1"); err != nil {
		return err
	}
	clip := platform.Rect{W: 640, H: 450}
	white := platform.Color{R: 1, G: 1, B: 1, A: 1}
	commands := []platform.Command{
		{Kind: platform.Rectangle, Bounds: platform.Rect{X: 16, Y: 16, W: 80, H: 64}, Clip: clip, Color: platform.Color{R: 1, A: 1}},
		{Kind: platform.Rectangle, Bounds: platform.Rect{X: 16, Y: 16, W: 80, H: 64}, Clip: platform.Rect{X: 48, Y: 40, W: 24, H: 20}, Color: platform.Color{B: 1, A: .5}},
		{Kind: platform.Rectangle, Bounds: platform.Rect{X: 120, Y: 16, W: 64, H: 64}, Clip: clip, Radius: 16, Color: platform.Color{G: 1, A: 1}},
		{Kind: platform.Line, Bounds: platform.Rect{X: 24, Y: 120, W: 96, H: 48}, Clip: clip, Radius: 8, Color: white},
	}
	geometry, geometryReport, err := render("geometry", commands)
	if err != nil {
		return err
	}
	if requestedScale > 0 && math.Abs(float64(geometryReport.Scale-requestedScale)) > .001 {
		return fmt.Errorf("actual geometry drawable density %g, expected %g", geometryReport.Scale, requestedScale)
	}
	if err = savePNG(filepath.Join(output, "geometry-gpu.png"), geometry); err != nil {
		return err
	}
	for _, sample := range []struct {
		x, y int
		want color.RGBA
	}{
		{4, 4, color.RGBA{A: 255}},
		{24, 24, color.RGBA{R: 255, A: 255}},
		{50, 44, color.RGBA{R: 128, B: 128, A: 255}},
		{74, 44, color.RGBA{R: 255, A: 255}},
		{120, 16, color.RGBA{A: 255}},
		{152, 48, color.RGBA{G: 255, A: 255}},
		{72, 144, color.RGBA{R: 255, G: 255, B: 255, A: 255}},
		{72, 130, color.RGBA{A: 255}},
	} {
		x, y := physical(sample.x, geometryReport.Scale), physical(sample.y, geometryReport.Scale)
		got := geometry.RGBAAt(x, y)
		if difference(got.R, sample.want.R) > 3 || difference(got.G, sample.want.G) > 3 || difference(got.B, sample.want.B) > 3 || difference(got.A, sample.want.A) > 3 {
			return fmt.Errorf("geometry GPU pixel (%d,%d): got %v, want %v", x, y, got, sample.want)
		}
	}
	texts := []string{"F", "A\u0301", "Á", "你好", "😀", "سلام", "office"}
	commands = nil
	for i, text := range texts {
		commands = append(commands, platform.Command{Kind: platform.Label, Bounds: platform.Rect{X: 16, Y: float32(16 + i*48), W: 500, H: 48}, Clip: clip, Color: white, FontSize: 28, Text: text})
	}
	commands = append(commands, platform.Command{Kind: platform.Label, Bounds: platform.Rect{X: 16, Y: 352, W: 40, H: 48}, Clip: platform.Rect{X: 16, Y: 352, W: 40, H: 48}, Color: white, FontSize: 28, Text: "MMMMMMMMMMMM"})
	unicode, unicodeReport, err := render("unicode", commands)
	if err != nil {
		return err
	}
	if requestedScale > 0 && math.Abs(float64(unicodeReport.Scale-requestedScale)) > .001 {
		return fmt.Errorf("actual Unicode drawable density %g, expected %g", unicodeReport.Scale, requestedScale)
	}
	if err = savePNG(filepath.Join(output, "unicode-gpu.png"), unicode); err != nil {
		return err
	}
	var checks []textReport
	for i, text := range texts {
		scale := unicodeReport.Scale
		width, height := physical(220, scale), physical(48, scale)
		reference, err := platform.MetalTextReference(text, "", 28, scale, width, height)
		if err != nil {
			return err
		}
		if err = savePNG(filepath.Join(output, fmt.Sprintf("text-%d-coretext-reference.png", i)), reference); err != nil {
			return err
		}
		origin := image.Pt(physical(16, scale), physical(16+i*48, scale))
		check := compareText(text, unicode, origin, reference)
		checks = append(checks, check)
		// Different per-glyph subpixel phases may change edge coverage. The
		// independent whole-line raster must still agree on orientation, shaping,
		// location and coverage, including combining marks and fallback fonts.
		if check.InkPixels < 10 || check.Reference < 10 || check.MaskIOU < .60 || check.BoundsDiff > int(math.Ceil(float64(2*scale))) {
			return fmt.Errorf("GPU glyph placement differs from whole-line CoreText: %+v", check)
		}
	}
	for y := physical(352, unicodeReport.Scale); y < physical(400, unicodeReport.Scale); y++ {
		for x := physical(57, unicodeReport.Scale); x < physical(400, unicodeReport.Scale); x++ {
			c := unicode.RGBAAt(x, y)
			if c.R > 3 || c.G > 3 || c.B > 3 {
				return fmt.Errorf("text escaped shader clip at GPU pixel (%d,%d)", x, y)
			}
		}
	}
	// Canonically equivalent accents must use the same shaped glyph placement.
	for y := 0; y < physical(48, unicodeReport.Scale); y++ {
		for x := 0; x < physical(100, unicodeReport.Scale); x++ {
			a := unicode.RGBAAt(physical(16, unicodeReport.Scale)+x, physical(64, unicodeReport.Scale)+y)
			b := unicode.RGBAAt(physical(16, unicodeReport.Scale)+x, physical(112, unicodeReport.Scale)+y)
			if difference(a.R, b.R) > 3 {
				return fmt.Errorf("canonical accent coverage differs at (%d,%d)", x, y)
			}
		}
	}
	report := struct {
		Geometry sceneReport  `json:"geometry"`
		Unicode  sceneReport  `json:"unicode"`
		Text     []textReport `json:"text_checks"`
	}{geometryReport, unicodeReport, checks}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(output, "gpu-validation.json"), append(data, '\n'), 0644); err != nil {
		return err
	}
	fmt.Println(string(data))
	return recoveryLimit(output, commands)
}

// The diagnostic requests resource recovery after real successful submissions;
// it does not disconnect hardware or manufacture command-buffer errors.
func recoveryLimit(output string, commands []platform.Command) error {
	for key, value := range map[string]string{"GODESKTOP_TEST_METAL_RECOVERY": "9", "GODESKTOP_TEST_METAL_RECOVERIES": "4"} {
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}
	watchdog := time.AfterFunc(30*time.Second, func() { platform.Quit() })
	err := platform.Run(platform.Options{Title: "godesktop Metal recovery limit", Width: 640, Height: 450, Background: platform.Color{A: 1}}, func(event platform.Event) {
		if event.Kind == platform.Draw {
			platform.Present(commands)
			platform.Wake()
		}
	})
	watchdog.Stop()
	stats := platform.RendererStats()
	if err == nil || !strings.Contains(err.Error(), "Metal GPU recovery limit exhausted") || stats.DeviceRecoveries != 3 || stats.Submitted < 36 || stats.Completed != stats.Submitted || stats.DroppedFrames != 0 || stats.InFlight != 0 {
		return fmt.Errorf("Metal repeated recovery did not terminate and drain within its limit: error=%v renderer=%+v", err, stats)
	}
	if err := os.Unsetenv("GODESKTOP_TEST_METAL_RECOVERY"); err != nil {
		return err
	}
	if err := os.Unsetenv("GODESKTOP_TEST_METAL_RECOVERIES"); err != nil {
		return err
	}
	pixels, restart, err := render("restart-after-recovery-limit", commands)
	if err != nil {
		return err
	}
	if restart.Renderer.DeviceRecoveries != 0 || restart.Renderer.DroppedFrames != 0 || restart.Frame >= 36 {
		return fmt.Errorf("old recovery callbacks or snapshot contaminated the next Run: %+v", restart)
	}
	if err = savePNG(filepath.Join(output, "restart-after-recovery-limit-gpu.png"), pixels); err != nil {
		return err
	}
	report := struct {
		Diagnostic bool                 `json:"diagnostic_injection_no_hardware_disconnect"`
		Limited    platform.RenderStats `json:"limited_run"`
		Restart    sceneReport          `json:"restart"`
	}{true, stats, restart}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(output, "recovery-limit.json"), append(data, '\n'), 0644)
}

func render(name string, commands []platform.Command) (*image.RGBA, sceneReport, error) {
	done := make(chan struct{})
	go func() {
		select {
		case <-time.After(30 * time.Second):
			platform.Quit()
		case <-done:
		}
	}()
	err := platform.Run(platform.Options{Title: "godesktop Metal GPU acceptance / " + name, Width: 640, Height: 450, Background: platform.Color{A: 1}}, func(event platform.Event) {
		if event.Kind != platform.Draw {
			return
		}
		platform.Present(commands)
		if platform.RendererStats().Completed >= 7 {
			platform.Quit()
		} else {
			platform.Wake()
		}
	})
	close(done)
	if err != nil {
		return nil, sceneReport{}, err
	}
	stats := platform.RendererStats()
	pixels, frame, err := platform.MetalSnapshot()
	if err != nil {
		return nil, sceneReport{}, err
	}
	if stats.Backend != "metal" || stats.Submitted < 7 || stats.Completed != stats.Submitted || frame != stats.Completed || stats.InFlight != 0 || stats.UsedSlotsMask != 7 || stats.BufferWaits != 0 {
		return nil, sceneReport{}, fmt.Errorf("actual drawable did not drain through three frame slots: frame=%d renderer=%+v", frame, stats)
	}
	scale := float32(pixels.Bounds().Dx()) / 640
	if scale <= 0 || differenceInt(pixels.Bounds().Dy(), physical(450, scale)) > 1 {
		return nil, sceneReport{}, fmt.Errorf("unexpected drawable dimensions %v, scale=%g", pixels.Bounds(), scale)
	}
	return pixels, sceneReport{Frame: frame, Scale: scale, Renderer: stats}, nil
}

func compareText(text string, actual *image.RGBA, origin image.Point, reference *image.RGBA) textReport {
	check := textReport{Text: text}
	intersection, union := 0, 0
	actualBounds, referenceBounds := image.Rectangle{}, image.Rectangle{}
	for y := 0; y < reference.Bounds().Dy(); y++ {
		for x := 0; x < reference.Bounds().Dx(); x++ {
			a := actual.RGBAAt(origin.X+x, origin.Y+y).R > 32
			b := reference.RGBAAt(x, y).A > 32
			pixel := image.Rect(x, y, x+1, y+1)
			if a {
				check.InkPixels++
				actualBounds = actualBounds.Union(pixel)
			}
			if b {
				check.Reference++
				referenceBounds = referenceBounds.Union(pixel)
			}
			if a && b {
				intersection++
			}
			if a || b {
				union++
			}
		}
	}
	if union != 0 {
		check.MaskIOU = float64(intersection) / float64(union)
	}
	check.BoundsDiff = max(differenceInt(actualBounds.Min.X, referenceBounds.Min.X), differenceInt(actualBounds.Min.Y, referenceBounds.Min.Y), differenceInt(actualBounds.Max.X, referenceBounds.Max.X), differenceInt(actualBounds.Max.Y, referenceBounds.Max.Y))
	return check
}

func physical(point int, scale float32) int { return int(math.Round(float64(float32(point) * scale))) }
func difference(a, b uint8) int             { return differenceInt(int(a), int(b)) }
func differenceInt(a, b int) int {
	if a > b {
		return a - b
	}
	return b - a
}
func savePNG(path string, pixels image.Image) error {
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
