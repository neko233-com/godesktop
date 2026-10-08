package godesktop

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func shadowCompiler(t *testing.T, cpp bool) string {
	t.Helper()
	names := []string{"gcc", "clang", "cc"}
	if cpp {
		names = []string{"g++", "clang++", "c++"}
	}
	for _, name := range names {
		if compiler, err := exec.LookPath(name); err == nil {
			return compiler
		}
	}
	t.Skip("headless native ABI/shader math validation requires a C/C++ compiler")
	return ""
}

func compileShadowProbe(t *testing.T, source string, cpp bool) string {
	t.Helper()
	dir := t.TempDir()
	extension := ".c"
	if cpp {
		extension = ".cpp"
	}
	path := filepath.Join(dir, "probe"+extension)
	exe := filepath.Join(dir, "probe.exe")
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	include, err := filepath.Abs("internal/platform")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	args := []string{"-O2", "-I", include, path, "-o", exe}
	if cpp {
		args = append(args, "-std=c++11")
	} else {
		args = append(args, "-std=c11")
	}
	command := exec.CommandContext(ctx, shadowCompiler(t, cpp), args...)
	command.WaitDelay = 3 * time.Second
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compile headless shadow probe: %v\n%s", err, output)
	}
	return exe
}

func runShadowProbe(t *testing.T, exe string, input []byte) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, exe)
	command.Stdin = bytes.NewReader(input)
	command.WaitDelay = 3 * time.Second
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("headless shadow probe: %v\n%s", err, output)
	}
	return output
}

func TestShadowActualCAndCPPABIMapping(t *testing.T) {
	const source = `
#include "gpu_scene.h"
int main(void) {
  GDCommand c={0}; c.kind=7; c.bounds=(GDRect){20,30,80,40};
  c.clip=(GDRect){0,0,200,100}; c.color=(GDColor){.1f,.2f,.3f,.4f};
  c.radius=8; c.font_size=6; c.rounded_bounds[0]=(GDRect){1,2,3,4}; c.rounded_radii[0]=2;
  GDGPUInstance v=gd_gpu_instance(&c);
  if(sizeof(c)!=168 || sizeof(v)!=160 || offsetof(GDGPUInstance,rounded_bounds)!=80) return 1;
  if(v.kind!=7 || v.bounds.x!=-5 || v.bounds.y!=5 || v.bounds.w!=130 || v.bounds.h!=90) return 2;
  if(v.uv.x!=20 || v.uv.y!=30 || v.uv.w!=80 || v.uv.h!=40 || v.padding[0]!=0 || v.padding[1]!=6) return 3;
  if(v.clip.w!=200 || v.color.a!=.4f || v.radius!=8 || v.rounded_bounds[0].x!=1 || v.rounded_radii[0]!=2) return 4;
  c.font_size=0; v=gd_gpu_instance(&c);
  if(v.bounds.x!=19 || v.bounds.y!=29 || v.bounds.w!=82 || v.bounds.h!=42 || v.padding[1]!=0) return 5;
  c.kind=1; c.font_size=14; v=gd_gpu_instance(&c);
  if(v.bounds.x!=20 || v.bounds.w!=80 || v.uv.x!=0 || v.uv.w!=1 || v.padding[1]!=0) return 6;
  return 0;
}`
	for _, cpp := range []bool{false, true} {
		t.Run(fmt.Sprintf("cpp=%t", cpp), func(t *testing.T) { runShadowProbe(t, compileShadowProbe(t, source, cpp), nil) })
	}
}

// The public command/instance ABI stays fixed. The private frame constants must
// also agree across the native encoders and both shader stages: the pixel
// shader reconstructs DIP coordinates from the actual device pixel center.
func TestShadowPhysicalPixelFrameContract(t *testing.T) {
	read := func(path string) string {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	compact := func(source string) string { return strings.Join(strings.Fields(source), "") }
	hlsl := read("internal/platform/shaders/ui.hlsl")
	frame := "cbuffer Frame : register(b0)"
	start := strings.Index(hlsl, frame)
	if start < 0 {
		t.Fatal("actual HLSL frame constant buffer missing")
	}
	start += strings.Index(hlsl[start:], "{") + 1
	end := strings.Index(hlsl[start:], "}")
	if end < 0 {
		t.Fatal("actual HLSL frame constant buffer incomplete")
	}
	// Compile the actual HLSL field declaration with a two-float compatibility
	// type, rather than introducing a second hard-coded frame declaration.
	probe := "#include <stddef.h>\ntypedef struct { float x,y; } float2;\nstruct GDProbeFrame {" + hlsl[start:start+end] + `};
_Static_assert(sizeof(struct GDProbeFrame)==16,"frame constants must stay four DWORDs");
_Static_assert(offsetof(struct GDProbeFrame,framePadding)==8,"density must be DWORD two");
int main(void) { struct GDProbeFrame f={{480,320},{1.5f,0}}; return f.framePadding.x!=1.5f; }
`
	runShadowProbe(t, compileShadowProbe(t, probe, false), nil)
	checks := []struct {
		path, expected string
	}{
		{"internal/platform/shaders/ui.hlsl", "nointerpolation float2 shadowOrigin : TEXCOORD13;"},
		{"internal/platform/shaders/ui.hlsl", "precise float2 local=input.uv.x>0?"},
		{"internal/platform/shaders/ui.hlsl", "input.uv.x>0?input.position.xy/framePadding.x-input.shadowOrigin:input.local"},
		{"internal/platform/dx12_device.h", "parameters[1].Constants={0,0,4};"},
		{"internal/platform/dx12_device.h", "parameters[1].ShaderVisibility=D3D12_SHADER_VISIBILITY_ALL;"},
		{"internal/platform/dx12_device.h", "float viewport[4]={static_cast<float>(width),static_cast<float>(height),1,0};"},
		{"internal/platform/dx12_device.h", "SetGraphicsRoot32BitConstants(1,4,viewport,0)"},
		{"internal/platform/dx12_surface.h", "float viewport[4]={dipWidth,dipHeight,density,0};"},
		{"internal/platform/dx12_surface.h", "SetGraphicsRoot32BitConstants(1,4,viewport,0)"},
		{"internal/platform/bridge_windows.cpp", "surface->submit(scene,client.right/scale,client.bottom/scale,scale,background,started,"},
		{"internal/platform/bridge_darwin.m", "float viewport[4]={self.bounds.size.width,self.bounds.size.height,scale,0};"},
		{"internal/platform/bridge_darwin.m", "[encoder setVertexBytes:viewport length:sizeof(viewport) atIndex:1];"},
		{"internal/platform/gpu_shader_metal.h", "constant float4 &frame [[buffer(1)]]"},
		{"internal/platform/gpu_shader_metal.h", "o.density=frame.z;"},
		{"internal/platform/gpu_shader_metal.h", "float2 shadowOrigin [[flat]]; float density [[flat]];"},
		{"internal/platform/gpu_shader_metal.h", "in.uv.x>0?in.position.xy/in.density-in.shadowOrigin:in.local"},
	}
	for _, check := range checks {
		if !strings.Contains(compact(read(check.path)), compact(check.expected)) {
			t.Errorf("%s is missing frame/pixel contract %q", check.path, check.expected)
		}
	}
}

// These compatibility operators let the compiler execute the actual scalar
// HLSL/Metal shadow functions, not a second Go copy of the quadrature algorithm.
// This checks shader mathematics headlessly; GPU encoding/pixels remain native
// acceptance requirements on both platforms.
const shadowMathCompatibility = `
#include <cmath>
#include <cstdio>
typedef unsigned int uint;
struct float2 { float x,y; float2(float a,float b):x(a),y(b){} explicit float2(float a):x(a),y(a){} };
static float2 operator-(float2 a,float2 b){return float2(a.x-b.x,a.y-b.y);}
static float2 operator-(float2 a,float b){return float2(a.x-b,a.y-b);}
static float2 operator*(float2 a,float b){return float2(a.x*b,a.y*b);}
static float min(float a,float b){return a<b?a:b;}
static float max(float a,float b){return a>b?a:b;}
static float2 max(float2 a,float b){return float2(max(a.x,b),max(a.y,b));}
static float2 max(float2 a,float2 b){return float2(max(a.x,b.x),max(a.y,b.y));}
static float shader_abs(float a){return std::fabs(a);}
static float2 shader_abs(float2 a){return float2(shader_abs(a.x),shader_abs(a.y));}
static float length(float2 a){return std::sqrt(a.x*a.x+a.y*a.y);}
static float clamp(float a,float b,float c){return min(c,max(b,a));}
static float saturate(float a){return clamp(a,0,1);}
static float smoothstep(float a,float b,float v){float t=clamp((v-a)/(b-a),0,1);return t*t*(3-2*t);}
static float shadowPixelSpan=1;
static float fwidth(float){return shadowPixelSpan;}
static float2 fwidth(float2){return float2(shadowPixelSpan,shadowPixelSpan);}
#define abs shader_abs
`

func shadowShaderFunctions(t *testing.T, path string, metal bool) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	if metal {
		var decoded strings.Builder
		for _, line := range strings.Split(source, "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "\"") {
				continue
			}
			end := strings.LastIndex(line, "\"")
			part, err := strconv.Unquote(line[:end+1])
			if err != nil {
				t.Fatal(err)
			}
			decoded.WriteString(part)
		}
		source = decoded.String()
	}
	start := strings.Index(source, "float shadowCDF(")
	end := strings.Index(source, "float4 fragment_main(")
	if metal {
		end = strings.Index(source, "fragment float4 fragment_main(")
	}
	if start < 0 || end <= start {
		t.Fatalf("actual shadow shader functions not found in %s", path)
	}
	source = strings.ReplaceAll(source[start:end], "[unroll] ", "")
	source = strings.ReplaceAll(source, "precise ", "")
	if metal {
		source = strings.NewReplacer("shadowCDF", "metalShadowCDF", "shadowInterval", "metalShadowInterval", "shadowSumError", "metalShadowSumError", "shadowProductError", "metalShadowProductError", "shadowCircleResidual", "metalShadowCircleResidual", "shadowCap", "metalShadowCap", "shadowBoxInterval", "metalShadowBoxInterval", "shadowBoxCap", "metalShadowBoxCap", "shadowCoverage", "metalShadowCoverage").Replace(source)
	}
	return source
}

func TestShadowShadersAgainstSupersampledGaussianMask(t *testing.T) {
	source := shadowMathCompatibility + shadowShaderFunctions(t, "internal/platform/shaders/ui.hlsl", false) + shadowShaderFunctions(t, "internal/platform/gpu_shader_metal.h", true) + `
int main(){ float w,h,r,s,x,y; while(std::scanf("%f %f %f %f %f %f",&w,&h,&r,&s,&x,&y)==6) {
  std::printf("%.9g %.9g\n",shadowCoverage(float2(x,y),float2(w,h),r,s),metalShadowCoverage(float2(x,y),float2(w,h),r,s)); }
  return 0; }
`
	exe := compileShadowProbe(t, source, true)
	type sample struct{ w, h, radius, sigma, x, y float64 }
	var samples []sample
	for _, shape := range []sample{{w: 80, h: 48, radius: 8, sigma: 6}, {w: 80, h: 48, radius: 0, sigma: 6}, {w: 20, h: 20, radius: 10, sigma: 1}, {w: 16, h: 8, radius: 4, sigma: 24}, {w: 24, h: 12, radius: 6, sigma: .25}, {w: 80, h: 48, radius: 8, sigma: 64}, {w: 400, h: 300, radius: 150, sigma: 2}, {w: 400, h: 300, radius: 150, sigma: .05}, {w: 2, h: 1, radius: .5, sigma: .25}} {
		curve := shape.radius * (1 - 1/math.Sqrt2)
		for _, point := range [][2]float64{{-3, 0}, {0, 0}, {2, 2}, {curve, curve}, {curve + shape.sigma, curve + shape.sigma}, {shape.radius, 0}, {shape.w / 2, -shape.sigma}, {shape.w / 2, 0}, {shape.w / 2, shape.h / 2}, {shape.w + shape.sigma, shape.h / 2}, {shape.w - curve, shape.h - curve}} {
			shape.x, shape.y = point[0], point[1]
			samples = append(samples, shape)
		}
	}
	var input bytes.Buffer
	for _, p := range samples {
		fmt.Fprintf(&input, "%g %g %g %g %g %g\n", p.w, p.h, p.radius, p.sigma, p.x, p.y)
	}
	lines := strings.Split(strings.TrimSpace(string(runShadowProbe(t, exe, input.Bytes()))), "\n")
	if len(lines) != len(samples) {
		t.Fatalf("actual shader probe output count: %d, want %d", len(lines), len(samples))
	}
	for i, p := range samples {
		var hlsl, metal float64
		if _, err := fmt.Sscanf(lines[i], "%f %f", &hlsl, &metal); err != nil {
			t.Fatal(err)
		}
		want, _, err := shadowCorpusReference(shadowCorpusSample{w: p.w, h: p.h, radius: p.radius, sigma: p.sigma, x: p.x, y: p.y, scale: 1})
		if err != nil {
			t.Fatalf("independent bounded mask oracle: %v", err)
		}
		if math.IsNaN(hlsl) || math.IsNaN(metal) || hlsl < 0 || hlsl > 1 || math.Abs(hlsl-metal) > .000002 || math.Abs(hlsl-want) > .004 {
			t.Errorf("actual paired shader versus Gaussian mask sample %+v: HLSL=%.8f Metal=%.8f oracle=%.8f", p, hlsl, metal, want)
		}
	}
}
