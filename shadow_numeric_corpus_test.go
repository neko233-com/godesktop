package godesktop

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"testing"

	"github.com/neko233-com/godesktop/internal/shadowtest"
)

type shadowCorpusSample struct {
	w, h, radius, sigma float64
	x, y, scale         float64
	name                string
}

func shadowNumericCorpus() []shadowCorpusSample {
	shapes := []shadowCorpusSample{
		{w: 400, h: 300, radius: 150, sigma: .0001},
		{w: 400, h: 300, radius: 150, sigma: .05},
		{w: 80, h: 48, radius: 8, sigma: 64},
		{w: 16, h: 8, radius: 4, sigma: 24},
		{w: 2, h: 1, radius: .5, sigma: .25},
		{w: .5, h: .25, radius: .125, sigma: .05},
		{w: 256, h: .5, radius: .25, sigma: 1},
		{w: .5, h: 256, radius: .25, sigma: 6},
		{w: 96, h: 64, radius: 32, sigma: .25},
		{w: 32, h: 24, radius: 0, sigma: .0001},
	}
	random := rand.New(rand.NewSource(0x2331708))
	sigmas := []float64{.0001, .001, .05, .25, 1, 3, 6, 24, 64}
	for index := range 24 {
		w, h := float64(1+random.Intn(512))/4, float64(1+random.Intn(384))/4
		if index%6 == 0 {
			h = .25
		} else if index%6 == 1 {
			w = .25
		}
		radius := min(w, h) / 2 * []float64{0, .125, .5, 1}[random.Intn(4)]
		shapes = append(shapes, shadowCorpusSample{w: w, h: h, radius: radius, sigma: sigmas[random.Intn(len(sigmas))]})
	}
	var corpus []shadowCorpusSample
	for index, shape := range shapes {
		curve := shape.radius * (1 - 1/math.Sqrt2)
		points := [][2]float64{{shape.w / 2, shape.h / 2}, {shape.w / 2, -shape.sigma}, {shape.w / 2, 0}, {shape.w + shape.sigma, shape.h / 2}, {curve, curve}, {curve + shape.sigma*.25, curve + shape.sigma*.25}, {shape.radius, 0}, {0, 0}}
		for point, position := range points {
			value := shape
			value.x, value.y, value.scale = position[0], position[1], 1
			value.name = fmt.Sprintf("gaussian/%02d/%d", index, point)
			corpus = append(corpus, value)
			// Mirror actual public coordinates before float32 input conversion.
			// Right/bottom caps must remain accurate at large local coordinates;
			// a top/left-only corpus would miss size-minus-local cancellation.
			for _, mirror := range []struct {
				name string
				x, y float64
			}{{"right", shape.w - position[0], position[1]}, {"bottom", position[0], shape.h - position[1]}, {"right-bottom", shape.w - position[0], shape.h - position[1]}} {
				mirrored := value
				mirrored.x, mirrored.y = mirror.x, mirror.y
				mirrored.name += "/" + mirror.name
				corpus = append(corpus, mirrored)
			}
		}
		for _, scale := range []float64{1, 1.5, 2} {
			span := 1 / scale
			points := [][2]float64{{shape.w / 2, shape.h / 2}, {shape.w / 2, -.25 * span}, {shape.w / 2, 0}, {-.25 * span, -.25 * span}, {curve, curve}, {curve + .125*span, curve + .125*span}, {shape.radius, 0}, {0, 0}}
			for point, position := range points {
				value := shape
				value.x, value.y, value.scale, value.sigma = position[0], position[1], scale, 0
				value.name = fmt.Sprintf("sharp/%02d/%g/%d", index, scale, point)
				corpus = append(corpus, value)
			}
		}
	}
	return corpus
}

func shadowCorpusReference(sample shadowCorpusSample) (float64, int, error) {
	// Match native float input coordinates. The minimum sigma's float32
	// representation is 2.6e-12 below the oracle's explicit minimum; retain the
	// mathematical lower bound rather than rejecting this representable style.
	spec := shadowtest.Spec{
		Rect:    shadowtest.Rect{Width: float64(float32(sample.w)), Height: float64(float32(sample.h))},
		Radius:  float64(float32(sample.radius)),
		Sigma:   float64(float32(sample.sigma)),
		Opacity: 1,
		Scale:   sample.scale,
		Samples: 16,
	}
	if spec.Sigma > 0 {
		spec.Sigma = max(spec.Sigma, shadowtest.MinPositiveSigma)
		// Positive-sigma point integration uses Scale only as numerical sampling
		// density; the physical derivative footprint matters only for sharp AA.
		spec.Scale = 1
	} else {
		// Four independent half-width footprints tile the original physical
		// device pixel. This removes the 16x16 diagonal lattice bias without
		// exceeding the oracle's per-call Samples/Scale limits (effective32x32).
		spec.Scale = sample.scale * 2
		span := 1 / sample.scale
		coverage := 0.0
		for _, y := range []float64{-.25, .25} {
			for _, x := range []float64{-.25, .25} {
				value, err := shadowtest.SampleCoverage(spec, float64(float32(sample.x))+x*span, float64(float32(sample.y))+y*span)
				if err != nil {
					return 0, 32, err
				}
				coverage += value / 4
			}
		}
		return coverage, 32, nil
	}
	if spec.Sigma < 1 {
		// A curved edge can align with the binary lattice and bias one phase.
		// Translating both source and query leaves the mathematical Gaussian
		// convolution unchanged; averaging4x4 subcell phases independently
		// refines that lattice without copying shader math or changing .004.
		frequency := max(float64(spec.Samples)*spec.Scale, 12/spec.Sigma)
		coverage := 0.0
		for row := range 4 {
			for column := range 4 {
				shiftX, shiftY := (float64(column)+.5)/(4*frequency), (float64(row)+.5)/(4*frequency)
				phase := spec
				phase.Rect.X, phase.Rect.Y = shiftX, shiftY
				value, err := shadowtest.SampleCoverage(phase, float64(float32(sample.x))+shiftX, float64(float32(sample.y))+shiftY)
				if err != nil {
					return 0, 64, err
				}
				coverage += value / 16
			}
		}
		return coverage, 64, nil
	}
	for {
		value, err := shadowtest.SampleCoverage(spec, float64(float32(sample.x)), float64(float32(sample.y)))
		if !errors.Is(err, shadowtest.ErrBudget) || spec.Samples == 1 {
			return value, spec.Samples, err
		}
		// Wide Gaussian kernels smooth binary-mask discretization. Decrease only
		// lattice density when its declared work budget rejects a large support;
		// the actual shader comparison remains the same .004 absolute bound.
		spec.Samples /= 2
	}
}

func TestShadowDeterministicNumericCorpus(t *testing.T) {
	source := shadowMathCompatibility + shadowShaderFunctions(t, "internal/platform/shaders/ui.hlsl", false) + shadowShaderFunctions(t, "internal/platform/gpu_shader_metal.h", true) + `
int main() { float w,h,r,s,x,y,scale;
  while(std::scanf("%f %f %f %f %f %f %f",&w,&h,&r,&s,&x,&y,&scale)==7) {
    shadowPixelSpan=1.0f/scale;
    std::printf("%.9g %.9g\n",shadowCoverage(float2(x,y),float2(w,h),r,s),metalShadowCoverage(float2(x,y),float2(w,h),r,s));
  }
  return 0;
}
`
	corpus := shadowNumericCorpus()
	var input bytes.Buffer
	for _, sample := range corpus {
		fmt.Fprintf(&input, "%.17g %.17g %.17g %.17g %.17g %.17g %.17g\n", sample.w, sample.h, sample.radius, sample.sigma, sample.x, sample.y, sample.scale)
	}
	executable := compileShadowProbe(t, source, true)
	lines := strings.Split(strings.TrimSpace(string(runShadowProbe(t, executable, input.Bytes()))), "\n")
	if len(lines) != len(corpus) {
		t.Fatalf("actual shader corpus emitted %d samples, expected %d", len(lines), len(corpus))
	}
	failures, fallbacks, gaussians, sharp := 0, 0, 0, 0
	refinements := 0
	gaussianFailures, sharpFailures := 0, 0
	maxGaussian, maxSharp, maxDisagreement := 0.0, 0.0, 0.0
	for index, sample := range corpus {
		var hlsl, metal float64
		if count, err := fmt.Sscanf(lines[index], "%f %f", &hlsl, &metal); err != nil || count != 2 {
			t.Fatal("invalid actual shader sample", index, lines[index], err)
		}
		expected, samples, err := shadowCorpusReference(sample)
		if err != nil {
			t.Fatal("bounded independent reference failed", sample, err)
		}
		if sample.sigma > 0 && samples < 16 {
			fallbacks++
		} else if sample.sigma > 0 && samples > 16 {
			refinements++
		}
		difference := math.Abs(hlsl - expected)
		disagreement := math.Abs(hlsl - metal)
		maxDisagreement = max(maxDisagreement, disagreement)
		tolerance := .004
		if sample.sigma == 0 {
			sharp++
			maxSharp = max(maxSharp, difference)
			// A tiled32x32 binary pixel footprint quantizes sharp boundary area. This
			// separate AA bound never relaxes the Gaussian integral comparison.
			tolerance = .02
			if sample.radius == 0 {
				tolerance = .00002 // Axis-aligned fractional box controls are exact.
			}
		} else {
			gaussians++
			maxGaussian = max(maxGaussian, difference)
		}
		if math.IsNaN(hlsl) || math.IsNaN(metal) || math.IsInf(hlsl, 0) || math.IsInf(metal, 0) || hlsl < 0 || hlsl > 1 || metal < 0 || metal > 1 || disagreement > .000002 || difference > tolerance {
			printFailure := false
			if sample.sigma > 0 {
				printFailure = gaussianFailures < 16
				gaussianFailures++
			} else {
				printFailure = sharpFailures < 8
				sharpFailures++
			}
			if printFailure {
				t.Errorf("actual shader corpus %s w=%g h=%g r=%g sigma=%g point=(%.9g,%.9g) scale=%g referenceSamples=%d: HLSL=%.9f Metal=%.9f reference=%.9f error=%.9f tolerance=%g", sample.name, sample.w, sample.h, sample.radius, sample.sigma, sample.x, sample.y, sample.scale, samples, hlsl, metal, expected, difference, tolerance)
			}
			failures++
		}
	}
	t.Logf("actual source corpus: Gaussian=%d sharp=%d fallbackSamples=%d phaseRefinements=%d maxGaussianError=%.9f maxSharpError=%.9f HLSLMetalDisagreement=%.9f failures=%d", gaussians, sharp, fallbacks, refinements, maxGaussian, maxSharp, maxDisagreement, failures)
	if gaussianFailures > 16 || sharpFailures > 8 {
		t.Errorf("additional actual shader/reference failures beyond the bounded diagnostics: Gaussian=%d sharp=%d", max(0, gaussianFailures-16), max(0, sharpFailures-8))
	}
}
