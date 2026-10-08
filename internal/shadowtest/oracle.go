// Package shadowtest supplies an independent CPU reference for rounded mask
// shadows. It rasterizes geometry and convolves a sampled Gaussian; it does not
// use the native shaders' CDF approximation or rounded-cap quadrature.
package shadowtest

import (
	"errors"
	"fmt"
	"image"
	"math"
)

const (
	MaxImageSide       = 2048
	MaxImagePixels     = 1 << 20
	MaxSamples         = 16
	MaxFineCells       = 4 << 20
	MaxConvolutionWork = 200_000_000
	MaxPointKernel     = 65536
	MaxSigma           = 64
	MinPositiveSigma   = 0.0001
)

var (
	ErrBounds = errors.New("shadow reference input is outside its finite bounds")
	ErrBudget = errors.New("shadow reference exceeds its memory or work budget")
)

type Rect struct{ X, Y, Width, Height float64 }

// Spec describes an unexpanded element rectangle in DIP. Offset and Spread form
// the caster; its radius is clamped from Radius+Spread to half its shortest side.
// Sigma is the Gaussian standard deviation (public ShadowStyle.Blur / 2).
// Opacity is explicit: zero is transparent. Scale defaults to 1; Samples defaults
// to 8 subpixels per device-pixel axis, bounded to 16. Width/Height are device
// pixels and are required only for Render. Input errors are rejected rather than
// silently applying the production style's normalization.
type Spec struct {
	Rect             Rect
	Radius           float64
	OffsetX, OffsetY float64
	Spread, Sigma    float64
	Opacity          float64
	Scale            float64
	Samples          int
	Width, Height    int
}

type geometry struct {
	rect                 Rect
	radius, sigma, alpha float64
	scale                float64
	samples              int
}

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

func prepare(spec Spec) (geometry, error) {
	if spec.Scale == 0 {
		spec.Scale = 1
	}
	if spec.Samples == 0 {
		spec.Samples = 8
	}
	for _, value := range []float64{spec.Rect.X, spec.Rect.Y, spec.Rect.Width, spec.Rect.Height, spec.Radius, spec.OffsetX, spec.OffsetY, spec.Spread, spec.Sigma, spec.Opacity, spec.Scale} {
		if !finite(value) {
			return geometry{}, fmt.Errorf("nonfinite geometry: %w", ErrBounds)
		}
	}
	if math.Abs(spec.Rect.X) > 1<<20 || math.Abs(spec.Rect.Y) > 1<<20 || spec.Rect.Width < 0 || spec.Rect.Width > 8192 || spec.Rect.Height < 0 || spec.Rect.Height > 8192 || spec.Radius < 0 || spec.Radius > 8192 || math.Abs(spec.OffsetX) > 4096 || math.Abs(spec.OffsetY) > 4096 || math.Abs(spec.Spread) > 128 || spec.Sigma < 0 || spec.Sigma > MaxSigma || (spec.Sigma > 0 && spec.Sigma < MinPositiveSigma) || spec.Opacity < 0 || spec.Opacity > 1 || spec.Scale < .25 || spec.Scale > 4 || spec.Samples < 1 || spec.Samples > MaxSamples {
		return geometry{}, ErrBounds
	}
	caster := Rect{spec.Rect.X + spec.OffsetX - spec.Spread, spec.Rect.Y + spec.OffsetY - spec.Spread, spec.Rect.Width + 2*spec.Spread, spec.Rect.Height + 2*spec.Spread}
	radius := 0.0
	if caster.Width > 0 && caster.Height > 0 {
		radius = min(max(0, spec.Radius+spec.Spread), min(caster.Width, caster.Height)/2)
	}
	return geometry{caster, radius, spec.Sigma, spec.Opacity, spec.Scale, spec.Samples}, nil
}

func (g geometry) empty() bool { return g.alpha == 0 || g.rect.Width <= 0 || g.rect.Height <= 0 }

// inside is binary geometry membership: two rectangular strips and four circle
// quadrants. It uses neither signed-distance antialiasing nor a blur formula.
func (g geometry) inside(x, y float64) bool {
	r := g.rect
	if x < r.X || y < r.Y || x >= r.X+r.Width || y >= r.Y+r.Height {
		return false
	}
	if g.radius == 0 || (x >= r.X+g.radius && x <= r.X+r.Width-g.radius) || (y >= r.Y+g.radius && y <= r.Y+r.Height-g.radius) {
		return true
	}
	centerX := r.X + g.radius
	if x > r.X+r.Width/2 {
		centerX = r.X + r.Width - g.radius
	}
	centerY := r.Y + g.radius
	if y > r.Y+r.Height/2 {
		centerY = r.Y + r.Height - g.radius
	}
	dx, dy := x-centerX, y-centerY
	return dx*dx+dy*dy <= g.radius*g.radius
}

func (g geometry) sharp(x, y float64) float64 {
	step := 1 / (g.scale * float64(g.samples))
	left, top := x-.5/g.scale, y-.5/g.scale
	covered := 0
	for row := range g.samples {
		for column := range g.samples {
			if g.inside(left+(float64(column)+.5)*step, top+(float64(row)+.5)*step) {
				covered++
			}
		}
	}
	return float64(covered) / float64(g.samples*g.samples) * g.alpha
}

type weightedPoint struct{ position, weight float64 }

// pointAxis constructs independent Gaussian convolution weights on a global
// subpixel lattice. Normalize over the complete five-sigma support, then keep
// only positions that can belong to the binary caster mask. Small sigma uses
// at least twelve samples per sigma to avoid undersampling a narrow kernel.
func pointAxis(point, sigma, frequency, lower, upper float64) ([]weightedPoint, error) {
	first := int64(math.Ceil((point-5*sigma)*frequency - .5))
	last := int64(math.Floor((point+5*sigma)*frequency - .5))
	count := last - first + 1
	if count <= 0 || count > MaxPointKernel {
		return nil, ErrBudget
	}
	points := make([]weightedPoint, 0, int(count))
	total := 0.0
	for index := first; index <= last; index++ {
		position := (float64(index) + .5) / frequency
		z := (position - point) / sigma
		weight := math.Exp(-.5 * z * z)
		total += weight
		if position >= lower && position < upper {
			points = append(points, weightedPoint{position, weight})
		}
	}
	if !finite(total) || total <= 0 {
		return nil, fmt.Errorf("unresolved Gaussian kernel: %w", ErrBounds)
	}
	for index := range points {
		points[index].weight /= total
	}
	return points, nil
}

// SampleCoverage returns unquantized alpha at an arbitrary DIP sample center.
// Positive sigma evaluates a two-dimensional convolution of an independently
// sampled binary mask with separable Gaussian weights. Zero sigma returns the
// supersampled coverage of one device-pixel footprint centered on the point.
// Shape coordinates outside an output canvas can still contribute to its halo.
func SampleCoverage(spec Spec, xDIP, yDIP float64) (float64, error) {
	g, err := prepare(spec)
	if err != nil {
		return 0, err
	}
	if !finite(xDIP) || !finite(yDIP) || math.Abs(xDIP) > 1<<20 || math.Abs(yDIP) > 1<<20 {
		return 0, ErrBounds
	}
	if g.empty() {
		return 0, nil
	}
	if g.sigma == 0 {
		return g.sharp(xDIP, yDIP), nil
	}
	r := g.rect
	if xDIP+5*g.sigma <= r.X || xDIP-5*g.sigma >= r.X+r.Width || yDIP+5*g.sigma <= r.Y || yDIP-5*g.sigma >= r.Y+r.Height {
		return 0, nil
	}
	frequency := max(g.scale*float64(g.samples), 12/g.sigma)
	xs, err := pointAxis(xDIP, g.sigma, frequency, r.X, r.X+r.Width)
	if err != nil {
		return 0, err
	}
	ys, err := pointAxis(yDIP, g.sigma, frequency, r.Y, r.Y+r.Height)
	if err != nil {
		return 0, err
	}
	if int64(len(xs))*int64(len(ys)) > MaxFineCells {
		return 0, ErrBudget
	}
	coverage := 0.0
	for _, y := range ys {
		row := 0.0
		for _, x := range xs {
			if g.inside(x.position, y.position) {
				row += x.weight
			}
		}
		coverage += row * y.weight
	}
	return min(1, max(0, coverage)) * g.alpha, nil
}

// Mask retains unquantized output values; Image makes an independent Alpha8
// diagnostic copy. For positive sigma Render samples the convolved fine grid at
// device-pixel centers; zero sigma averages each pixel's binary subpixel mask.
type Mask struct {
	width, height int
	scale         float64
	alpha         []float64
}

func (m *Mask) Bounds() image.Rectangle { return image.Rect(0, 0, m.width, m.height) }

func (m *Mask) Pixel(xDevice, yDevice int) float64 {
	if xDevice < 0 || yDevice < 0 || xDevice >= m.width || yDevice >= m.height {
		return 0
	}
	return m.alpha[yDevice*m.width+xDevice]
}

func (m *Mask) AtDIP(x, y float64) float64 {
	if !finite(x) || !finite(y) || x < 0 || y < 0 || x >= float64(m.width)/m.scale || y >= float64(m.height)/m.scale {
		return 0
	}
	x, y = x*m.scale-.5, y*m.scale-.5
	left, top := int(math.Floor(x)), int(math.Floor(y))
	fx, fy := x-float64(left), y-float64(top)
	return (m.Pixel(left, top)*(1-fx)+m.Pixel(left+1, top)*fx)*(1-fy) + (m.Pixel(left, top+1)*(1-fx)+m.Pixel(left+1, top+1)*fx)*fy
}

func (m *Mask) Image() *image.Alpha {
	output := image.NewAlpha(m.Bounds())
	for index, value := range m.alpha {
		output.Pix[index] = uint8(math.Round(min(1, max(0, value)) * 255))
	}
	return output
}

func gaussianKernel(sigma, frequency float64) ([]float64, int) {
	radius := int(math.Ceil(5 * sigma * frequency))
	weights := make([]float64, 2*radius+1)
	total := 0.0
	for index := range weights {
		z := float64(index-radius) / (sigma * frequency)
		weights[index] = math.Exp(-.5 * z * z)
		total += weights[index]
	}
	for index := range weights {
		weights[index] /= total
	}
	return weights, radius
}

// Render allocates at most two MaxFineCells float64 grids (64 MiB combined)
// plus a bounded device image. It includes a five-sigma source halo before
// cropping to the canvas, so an offscreen caster can cast into visible pixels.
// Large blur/image/sample combinations return ErrBudget before allocation.
func Render(spec Spec) (*Mask, error) {
	g, err := prepare(spec)
	if err != nil {
		return nil, err
	}
	if spec.Width <= 0 || spec.Height <= 0 {
		return nil, ErrBounds
	}
	if spec.Width > MaxImageSide || spec.Height > MaxImageSide || int64(spec.Width)*int64(spec.Height) > MaxImagePixels {
		return nil, ErrBudget
	}
	if g.empty() {
		return &Mask{spec.Width, spec.Height, g.scale, make([]float64, spec.Width*spec.Height)}, nil
	}
	frequency := g.scale * float64(g.samples)
	padding := 0
	if g.sigma > 0 {
		padding = int(math.Ceil(5 * g.sigma * frequency))
	}
	width, height := spec.Width*g.samples+2*padding, spec.Height*g.samples+2*padding
	cells := int64(width) * int64(height)
	work := cells
	if padding > 0 {
		work *= int64(2 * (2*padding + 1))
	}
	if cells > MaxFineCells || work > MaxConvolutionWork {
		return nil, ErrBudget
	}
	output := &Mask{spec.Width, spec.Height, g.scale, make([]float64, spec.Width*spec.Height)}
	mask := make([]float64, int(cells))
	for y := range height {
		pointY := (float64(y-padding) + .5) / frequency
		for x := range width {
			if g.inside((float64(x-padding)+.5)/frequency, pointY) {
				mask[y*width+x] = 1
			}
		}
	}
	if g.sigma > 0 {
		weights, radius := gaussianKernel(g.sigma, frequency)
		temporary := make([]float64, len(mask))
		for y := range height {
			for x := range width {
				value := 0.0
				for index, weight := range weights {
					sourceX := x + index - radius
					if sourceX >= 0 && sourceX < width {
						value += mask[y*width+sourceX] * weight
					}
				}
				temporary[y*width+x] = value
			}
		}
		for y := range height {
			for x := range width {
				value := 0.0
				for index, weight := range weights {
					sourceY := y + index - radius
					if sourceY >= 0 && sourceY < height {
						value += temporary[sourceY*width+x] * weight
					}
				}
				mask[y*width+x] = value
			}
		}
	}
	for y := range spec.Height {
		for x := range spec.Width {
			value := 0.0
			if g.sigma == 0 {
				for row := range g.samples {
					for column := range g.samples {
						value += mask[(y*g.samples+row)*width+x*g.samples+column]
					}
				}
				value /= float64(g.samples * g.samples)
			} else {
				fineX, fineY := float64(padding+x*g.samples)+float64(g.samples-1)/2, float64(padding+y*g.samples)+float64(g.samples-1)/2
				left, top := int(math.Floor(fineX)), int(math.Floor(fineY))
				fx, fy := fineX-float64(left), fineY-float64(top)
				value = (mask[top*width+left]*(1-fx)+mask[top*width+left+1]*fx)*(1-fy) + (mask[(top+1)*width+left]*(1-fx)+mask[(top+1)*width+left+1]*fx)*fy
			}
			output.alpha[y*spec.Width+x] = min(1, max(0, value)) * g.alpha
		}
	}
	return output, nil
}
