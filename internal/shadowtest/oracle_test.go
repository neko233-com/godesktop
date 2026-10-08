package shadowtest

import (
	"errors"
	"math"
	"testing"
)

func sampled(t *testing.T, spec Spec, x, y float64) float64 {
	t.Helper()
	value, err := SampleCoverage(spec, x, y)
	if err != nil {
		t.Fatal("independent mask convolution failed", err)
	}
	if !finite(value) || value < 0 || value > spec.Opacity {
		t.Fatal("reference alpha escaped normalized bounds", value)
	}
	return value
}

func rendered(t *testing.T, spec Spec) *Mask {
	t.Helper()
	mask, err := Render(spec)
	if err != nil {
		t.Fatal("bounded independent render failed", err)
	}
	return mask
}

// The mathematical rectangle/circle integrals below are closed-form test
// references. Production reference code uses binary masks and sampled kernels.
func normalInterval(point, lower, upper, sigma float64) float64 {
	return (math.Erf((upper-point)/(sigma*math.Sqrt2)) - math.Erf((lower-point)/(sigma*math.Sqrt2))) / 2
}

func TestSampleGaussianRectangleMatchesIndependentErfIntegral(t *testing.T) {
	spec := Spec{Rect: Rect{3, 5, 14, 9}, Sigma: 2.5, Opacity: .65, Samples: 16}
	maximum := 0.0
	for _, point := range [][2]float64{{-4, -2}, {2, 4}, {3, 5}, {10, 9}, {17, 14}, {17.5, 14.5}, {20, 10}, {10, 18}, {3.125, 8.875}} {
		expected := normalInterval(point[0], 3, 17, spec.Sigma) * normalInterval(point[1], 5, 14, spec.Sigma) * spec.Opacity
		err := math.Abs(sampled(t, spec, point[0], point[1]) - expected)
		maximum = max(maximum, err)
		if err > .0001 {
			t.Fatalf("sample %v differs from independent rectangle integral by %.8f", point, err)
		}
	}
	t.Logf("maximum rectangle integral error %.8f", maximum)
	for _, sigma := range []float64{MinPositiveSigma, .05, .25, 64} {
		spec := Spec{Rect: Rect{0, 0, 20, 20}, Sigma: sigma, Opacity: 1, Samples: 16}
		expected := normalInterval(0, 0, 20, sigma) * normalInterval(10, 0, 20, sigma)
		if err := math.Abs(sampled(t, spec, 0, 10) - expected); err > .00003 {
			t.Fatalf("sigma %.5f edge integral error %.8f", sigma, err)
		}
	}
}

func TestSampleRoundedCircleIntegralAndSmallSigmaPrecision(t *testing.T) {
	for _, test := range []struct{ radius, sigma float64 }{{1, .25}, {4, 2}, {4, 24}, {150, .05}} {
		spec := Spec{Rect: Rect{-test.radius, -test.radius, 2 * test.radius, 2 * test.radius}, Radius: test.radius, Sigma: test.sigma, Opacity: .9, Samples: 16, Scale: 4}
		expected := (1 - math.Exp(-test.radius*test.radius/(2*test.sigma*test.sigma))) * spec.Opacity
		if err := math.Abs(sampled(t, spec, 0, 0) - expected); err > .00015 {
			t.Fatalf("circle r=%g sigma=%g center integral error %.8f", test.radius, test.sigma, err)
		}
	}
	large := Spec{Rect: Rect{-150, -150, 300, 300}, Radius: 150, Sigma: .05, Opacity: 1, Samples: 16}
	coordinate := 150 / math.Sqrt2
	if edge := sampled(t, large, coordinate, coordinate); edge < .48 || edge > .52 {
		t.Fatal("tiny sigma around a large circle aliased its curved edge", edge)
	}
}

func TestSampleSymmetryTranslationSpreadAndOpacity(t *testing.T) {
	spec := Spec{Rect: Rect{10, 8, 20, 16}, Radius: 5, Sigma: 3, Opacity: .8, Samples: 16}
	for _, point := range [][2]float64{{8.25, 6.5}, {10, 8}, {13.5, 11.25}, {20, 16}, {30.5, 18.25}} {
		original := sampled(t, spec, point[0], point[1])
		if x, y := sampled(t, spec, 40-point[0], point[1]), sampled(t, spec, point[0], 32-point[1]); math.Abs(original-x) > 1e-10 || math.Abs(original-y) > 1e-10 {
			t.Fatal("sampled convolution lost reflected symmetry", original, x, y)
		}
		shifted := spec
		shifted.OffsetX, shifted.OffsetY = 3.5, -2.25
		if value := sampled(t, shifted, point[0]+3.5, point[1]-2.25); math.Abs(value-original) > 1e-10 {
			t.Fatal("DIP translation changed mask/kernel coverage", original, value)
		}
		faded := spec
		faded.Opacity /= 2
		if value := sampled(t, faded, point[0], point[1]); math.Abs(value-original/2) > 1e-12 {
			t.Fatal("opacity was not linear", original, value)
		}
	}
	spread, explicit := spec, spec
	spread.Spread = 2
	explicit.Rect, explicit.Radius = Rect{8, 6, 24, 20}, 7
	if a, b := sampled(t, spread, 9, 7), sampled(t, explicit, 9, 7); a != b {
		t.Fatal("spread did not expand both geometry and corner radius", a, b)
	}
	spread.Spread = -10
	if value := sampled(t, spread, 20, 16); value != 0 {
		t.Fatal("collapsed negative spread retained alpha", value)
	}
}

func TestSampleStraightEdgeGradientAndFarHalo(t *testing.T) {
	spec := Spec{Rect: Rect{0, 0, 40, 24}, Radius: 4, Sigma: 2, Opacity: 1, Samples: 16}
	previous := -1.0
	for _, x := range []float64{-10, -6, -4, -2, 0, 2, 4, 8, 20} {
		value := sampled(t, spec, x, 12)
		if value <= previous {
			t.Fatal("straight Gaussian edge gradient was not increasing", x, previous, value)
		}
		previous = value
	}
	if edge := sampled(t, spec, 0, 12); math.Abs(edge-.5) > .00001 {
		t.Fatal("straight edge Gaussian midpoint was not half coverage", edge)
	}
	if value := sampled(t, spec, -11, 12); value != 0 {
		t.Fatal("far halo escaped the finite five-sigma reference support", value)
	}
}

func TestRenderZeroBlurSupersampledAreaAndDIPDeviceCoordinates(t *testing.T) {
	spec := Spec{Rect: Rect{3.25, 2.5, 4.5, 3}, Opacity: .4, Samples: 8, Width: 12, Height: 10}
	mask := rendered(t, spec)
	if math.Abs(mask.Pixel(3, 2)-.15) > 1e-12 || math.Abs(mask.Pixel(7, 2)-.15) > 1e-12 || mask.Pixel(4, 3) != .4 || mask.Pixel(2, 2) != 0 {
		t.Fatal("sharp supersampled mask did not preserve fractional pixel area", mask.Pixel(3, 2), mask.Pixel(7, 2), mask.Pixel(4, 3))
	}
	area := 0.0
	for y := range spec.Height {
		for x := range spec.Width {
			value := mask.Pixel(x, y)
			area += value
			if math.Abs(mask.AtDIP(float64(x)+.5, float64(y)+.5)-value) > 1e-12 || math.Abs(sampled(t, spec, float64(x)+.5, float64(y)+.5)-value) > 1e-12 {
				t.Fatal("DIP pixel center differed from device coverage", x, y)
			}
		}
	}
	if math.Abs(area-4.5*3*.4) > 1e-12 {
		t.Fatal("supersampled rectangle lost exact covered area", area)
	}
	if mask.Pixel(-1, 0) != 0 || mask.Pixel(12, 0) != 0 || mask.AtDIP(-1, 0) != 0 || mask.AtDIP(math.NaN(), 0) != 0 {
		t.Fatal("out-of-image sample was not transparent")
	}
	image := mask.Image()
	if image.AlphaAt(3, 2).A != 38 {
		t.Fatal("diagnostic Alpha8 image quantized an unexpected value", image.AlphaAt(3, 2))
	}
	image.Pix[2*image.Stride+3] = 255
	if math.Abs(mask.Pixel(3, 2)-.15) > 1e-12 {
		t.Fatal("diagnostic image aliased the unquantized reference")
	}
	dense := Spec{Rect: Rect{2, 2, 4, 4}, Opacity: 1, Samples: 8, Scale: 1.5, Width: 15, Height: 15}
	density := rendered(t, dense)
	if density.Pixel(3, 3) != 1 || density.Pixel(2, 3) != 0 || density.AtDIP(3.5/1.5, 3.5/1.5) != 1 {
		t.Fatal("fractional device density changed DIP geometry")
	}
}

func TestRenderRoundedMaskAreaCornersAndRadiusClamp(t *testing.T) {
	spec := Spec{Rect: Rect{2, 2, 8, 8}, Radius: 4, Opacity: 1, Samples: 16, Width: 12, Height: 12}
	mask := rendered(t, spec)
	if mask.Pixel(2, 2) != 0 || mask.Pixel(3, 3) <= 0 || mask.Pixel(3, 3) >= 1 || mask.Pixel(5, 5) != 1 {
		t.Fatal("rounded binary-mask corners lost shape/antialiasing", mask.Pixel(2, 2), mask.Pixel(3, 3), mask.Pixel(5, 5))
	}
	area := 0.0
	for y := range spec.Height {
		for x := range spec.Width {
			area += mask.Pixel(x, y)
		}
	}
	if math.Abs(area-math.Pi*16) > .15 {
		t.Fatal("supersampled circle area differs from mathematical area", area)
	}
	clamped := spec
	clamped.Radius = 100
	other := rendered(t, clamped)
	for y := range spec.Height {
		for x := range spec.Width {
			if mask.Pixel(x, y) != other.Pixel(x, y) {
				t.Fatal("radius clamp did not preserve the same silhouette", x, y)
			}
		}
	}
}

func TestRenderSeparableConvolutionAgreesWithDirectMaskKernel(t *testing.T) {
	spec := Spec{Rect: Rect{8, 6, 8, 8}, Radius: 2, Sigma: 1.25, Opacity: .7, Samples: 16, Width: 24, Height: 20}
	mask := rendered(t, spec)
	for _, pixel := range [][2]int{{4, 10}, {7, 8}, {8, 6}, {10, 10}, {15, 13}, {16, 14}} {
		expected := sampled(t, spec, float64(pixel[0])+.5, float64(pixel[1])+.5)
		if difference := math.Abs(mask.Pixel(pixel[0], pixel[1]) - expected); difference > .0015 {
			t.Fatal("two-pass image convolution disagreed with direct independent mask convolution", pixel, difference)
		}
	}
	for y := range spec.Height {
		for x := range spec.Width {
			if difference := math.Abs(mask.Pixel(x, y) - mask.Pixel(spec.Width-1-x, spec.Height-1-y)); difference > 1e-10 {
				t.Fatal("two-pass Gaussian convolution lost symmetry", x, y, difference)
			}
		}
	}
	offscreen := Spec{Rect: Rect{-3, 3, 2, 4}, Sigma: 1.5, Opacity: 1, Samples: 8, Width: 8, Height: 8}
	halo := rendered(t, offscreen)
	if value := halo.Pixel(0, 4); value <= .05 || value >= .2 || math.Abs(value-sampled(t, offscreen, .5, 4.5)) > .0015 {
		t.Fatal("cropping discarded the offscreen caster's visible halo", value)
	}
}

func TestSampleResolutionConvergesToIndependentCircleIntegral(t *testing.T) {
	spec := Spec{Rect: Rect{-4, -4, 8, 8}, Radius: 4, Sigma: 2, Opacity: 1, Samples: 4}
	expected := 1 - math.Exp(-2)
	coarse := math.Abs(sampled(t, spec, 0, 0) - expected)
	spec.Samples, spec.Scale = 16, 4
	fine := math.Abs(sampled(t, spec, 0, 0) - expected)
	if fine > .00003 || fine >= coarse {
		t.Fatal("independent mask supersampling did not converge", coarse, fine)
	}
}

func TestOracleRejectsNonfiniteBoundsAndBudgets(t *testing.T) {
	base := Spec{Rect: Rect{0, 0, 8, 8}, Radius: 2, Sigma: 1, Opacity: 1, Samples: 8, Width: 16, Height: 16}
	for _, change := range []func(*Spec){
		func(s *Spec) { s.Rect.X = math.NaN() }, func(s *Spec) { s.Radius = math.Inf(1) }, func(s *Spec) { s.Sigma = -1 }, func(s *Spec) { s.Sigma = MinPositiveSigma / 2 }, func(s *Spec) { s.Sigma = 65 }, func(s *Spec) { s.Opacity = 1.1 }, func(s *Spec) { s.Opacity = -.1 }, func(s *Spec) { s.Samples = 17 }, func(s *Spec) { s.Samples = -1 }, func(s *Spec) { s.Scale = .1 }, func(s *Spec) { s.Scale = 4.1 }, func(s *Spec) { s.OffsetX = 4097 }, func(s *Spec) { s.Spread = 129 }, func(s *Spec) { s.Rect.Height = -1 }, func(s *Spec) { s.Rect.X = 1<<20 + 1 },
	} {
		invalid := base
		change(&invalid)
		if _, err := SampleCoverage(invalid, 4, 4); !errors.Is(err, ErrBounds) {
			t.Fatal("invalid sample input accepted", invalid, err)
		}
		if _, err := Render(invalid); !errors.Is(err, ErrBounds) {
			t.Fatal("invalid image input accepted", invalid, err)
		}
	}
	if _, err := SampleCoverage(base, math.Inf(-1), 0); !errors.Is(err, ErrBounds) {
		t.Fatal("nonfinite query accepted", err)
	}
	for _, change := range []func(*Spec){
		func(s *Spec) { s.Width = MaxImageSide + 1 }, func(s *Spec) { s.Width, s.Height = MaxImageSide, MaxImageSide }, func(s *Spec) { s.Width, s.Height, s.Sigma, s.Samples = 64, 64, 64, 16 }, func(s *Spec) { s.Width, s.Height, s.Sigma, s.Samples = 12, 12, 8, 16 },
	} {
		large := base
		change(&large)
		if mask, err := Render(large); mask != nil || !errors.Is(err, ErrBudget) {
			t.Fatal("oversized image/grid/convolution work was accepted", large, err)
		}
	}
	large := Spec{Rect: Rect{-400, -400, 800, 800}, Radius: 100, Sigma: 64, Opacity: 1, Scale: 4, Samples: 16}
	if _, err := SampleCoverage(large, 0, 0); !errors.Is(err, ErrBudget) {
		t.Fatal("unbounded point mask integration was accepted", err)
	}
	base.Width = 0
	if _, err := Render(base); !errors.Is(err, ErrBounds) {
		t.Fatal("empty canvas accepted", err)
	}
}

func TestTransparentCollapsedAndDefaultOracleInputs(t *testing.T) {
	base := Spec{Rect: Rect{2, 2, 8, 8}, Radius: 2, Sigma: 64, Width: 16, Height: 16}
	for _, spec := range []Spec{base, {Rect: base.Rect, Radius: 2, Sigma: 64, Spread: -4, Opacity: 1, Width: 16, Height: 16}} {
		mask := rendered(t, spec)
		if mask.Pixel(5, 5) != 0 || sampled(t, spec, 5, 5) != 0 {
			t.Fatal("transparent/collapsed caster produced shadow")
		}
	}
	base.Sigma, base.Opacity = 0, 1
	defaults := rendered(t, base)
	base.Scale, base.Samples = 1, 8
	explicit := rendered(t, base)
	if defaults.Pixel(2, 2) != explicit.Pixel(2, 2) || defaults.Pixel(5, 5) != 1 {
		t.Fatal("default density/samples differed from explicit inputs")
	}
}
