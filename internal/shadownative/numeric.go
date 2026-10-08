//go:build (windows || darwin) && cgo

package main

import (
	"context"
	"errors"
	"fmt"
	"image/png"
	"math"
	"os"
	"sync/atomic"
	"time"

	ui "github.com/neko233-com/godesktop"
	"github.com/neko233-com/godesktop/internal/platform"
	"github.com/neko233-com/godesktop/internal/shadowtest"
)

type tinyCaster struct {
	name                         string
	clipX, clipY, localX, localY float32
	pixelX, pixelY               int
	queryX, queryY               float64
	spec                         shadowtest.Spec // Float32 DIP geometry/query.
}

func tinyGeometry(density float64) []tinyCaster {
	var result []tinyCaster
	for _, sample := range []struct {
		name             string
		clipX, sampleX   float32
		wantedX, wantedY float64
	}{{"tiny-bottom-flat", 440, 441, 200, 300}, {"tiny-bottom-curve", 456, 457, 43.9339828, 256.066017}} {
		x, y := int(math.Floor(float64(sample.sampleX)*density)), int(math.Floor(298*density))
		pointX, pointY := (float64(x)+.5)/density, (float64(y)+.5)/density
		// Match layout's two float32 additions. The renderer keeps instances in
		// DIP and maps the actual fragment position to DIP before subtraction.
		localX := float32(pointX-sample.wantedX) - sample.clipX
		localY := float32(pointY-sample.wantedY) - 296
		worldX, worldY := sample.clipX+localX, float32(296)+localY
		result = append(result, tinyCaster{
			name: sample.name, clipX: sample.clipX, clipY: 296, localX: localX, localY: localY, pixelX: x, pixelY: y,
			queryX: float64((float32(x)+.5)/float32(density) - worldX), queryY: float64((float32(y)+.5)/float32(density) - worldY),
			spec: shadowtest.Spec{
				Rect:   shadowtest.Rect{Width: 400, Height: 300},
				Radius: 150, Sigma: max(shadowtest.MinPositiveSigma, float64(float32(.0002)/2)), Opacity: 1, Scale: 1, Samples: 16,
			},
		})
	}
	return result
}

func tinyScene(geometry []tinyCaster) *ui.Element {
	children := []*ui.Element{ui.Column().Background(ui.RGB(0xffffff))}
	for _, caster := range geometry {
		body := ui.Column().Width(400).Height(300).Radius(150).Position(caster.localX, caster.localY).Shadow(ui.ShadowStyle{Color: ui.RGBA(0, 1), Blur: .0002})
		children = append(children, ui.Stack(body).Width(4).Height(4).Position(caster.clipX, caster.clipY))
	}
	return ui.Stack(children...)
}

func tinyReference(spec shadowtest.Spec, x, y float64) (float64, error) {
	// Independent binary-mask lattice phase refinement, not shader algebra.
	frequency := max(spec.Scale*float64(spec.Samples), 12/spec.Sigma)
	coverage := 0.0
	for row := range 4 {
		for column := range 4 {
			shiftX, shiftY := (float64(column)+.5)/(4*frequency), (float64(row)+.5)/(4*frequency)
			phase := spec
			phase.Rect.X += shiftX
			phase.Rect.Y += shiftY
			value, err := shadowtest.SampleCoverage(phase, x+shiftX, y+shiftY)
			if err != nil {
				return 0, err
			}
			coverage += value / 16
		}
	}
	return coverage, nil
}

func checkTinyWindow(ctx context.Context, title string, density float64, path string, views *atomic.Uint64, geometry []tinyCaster) (report runReport, failure error) {
	report = runReport{Scene: "tiny-gaussian-device-pixels", Density: density, AdapterPolicy: nativeAdapterPolicy(os.Getenv("GODESKTOP_SHADOW_ADAPTER_POLICY"))}
	before, err := stable(ctx, views, false)
	if err != nil {
		return report, err
	}
	if before.Instances != 3 || before.UploadedBytes != 3*160 || before.DrawCalls != 1 || before.FrameSlots != 3 || before.UsedSlotsMask != 7 || before.BufferWaits != 0 || before.BitmapUploads != 0 {
		return report, fmt.Errorf("isolated tiny-mask scene budget: %+v", before)
	}
	report.BeforeIdle, report.ViewsBefore = before, views.Load()
	report.Presentation, err = nativePresentation(title, before)
	if err != nil {
		return report, err
	}
	pixels, actualDensity, dpi, err := snapshotNative(title)
	if err != nil {
		return report, err
	}
	report.Density, report.MonitorDPI = actualDensity, dpi
	report.CaptureWidth, report.CaptureHeight = pixels.Bounds().Dx(), pixels.Bounds().Dy()
	if math.Abs(actualDensity-density) > .001 || report.CaptureHeight != int(math.Round(320*density)) {
		return report, errors.New("tiny-mask actual drawable density differs")
	}
	// Preserve the real completed GPU frame even when a pixel guard rejects it.
	f, err := os.Create(path)
	if err != nil {
		return report, err
	}
	if err := errors.Join(png.Encode(f, pixels), f.Close()); err != nil {
		return report, err
	}
	for _, caster := range geometry {
		alpha, err := tinyReference(caster.spec, caster.queryX, caster.queryY)
		if err != nil {
			return report, err
		}
		if alpha < .03 || alpha > .97 {
			return report, fmt.Errorf("tiny-mask sample did not prove fractional coverage: %s alpha=%g", caster.name, alpha)
		}
		color := pixels.RGBAAt(caster.pixelX, caster.pixelY)
		expected := int(math.Round(255 * (1 - alpha)))
		check := pixelReport{Name: caster.name, DeviceX: caster.pixelX, DeviceY: caster.pixelY, Actual: [3]int{int(color.R), int(color.G), int(color.B)}, Expected: [3]int{expected, expected, expected}, Unclipped: alpha}
		report.Pixels = append(report.Pixels, check)
		for _, actual := range check.Actual {
			if abs(actual-expected) > 4 {
				return report, fmt.Errorf("actual GPU tiny mask %s: got%v expected%v alpha=%g DIPquery=(%g,%g)", caster.name, check.Actual, check.Expected, alpha, caster.queryX, caster.queryY)
			}
		}
	}
	report.BeforeIdle, report.ViewsBefore = platform.RendererStats(), views.Load()
	select {
	case <-ctx.Done():
		return report, ctx.Err()
	case <-time.After(200 * time.Millisecond):
	}
	report.AfterIdle, report.ViewsAfter = platform.RendererStats(), views.Load()
	if report.BeforeIdle.Submitted != report.AfterIdle.Submitted || report.ViewsBefore != report.ViewsAfter || report.AfterIdle.InFlight != 0 {
		return report, errors.New("tiny-mask scene created GPU/View busy loop")
	}
	return report, nil
}
