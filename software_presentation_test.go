package godesktop

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestSoftwarePresentationActualFootprintBounds(t *testing.T) {
	// Execute the scalar helper that Surface::targets uses after the actual
	// D3D12 footprint/allocation queries. This is allocation admission testing;
	// it creates no device/resources and proves no native queue quiescence.
	surface, err := os.ReadFile("internal/platform/dx12_surface.h")
	if err != nil {
		t.Fatal(err)
	}
	text := string(surface)
	start := strings.Index(text, "inline bool software_front_layout(")
	if start < 0 {
		t.Fatal("actual software footprint admission helper missing")
	}
	opening := strings.Index(text[start:], "{") + start
	depth, end := 1, opening+1
	for end < len(text) && depth > 0 {
		switch text[end] {
		case '{':
			depth++
		case '}':
			depth--
		}
		end++
	}
	if depth != 0 {
		t.Fatal("actual software footprint helper incomplete")
	}
	capture, err := os.ReadFile("internal/platform/gpu_capture_windows.h")
	if err != nil {
		t.Fatal(err)
	}
	captureText := string(capture)
	capacity := strings.Index(captureText, "static constexpr size_t capacity=")
	if capacity < 0 {
		t.Fatal("actual GPU capture capacity missing")
	}
	capacityEnd := capacity + strings.Index(captureText[capacity:], ";") + 1
	source := `
#include <cstdint>
#include <cstddef>
#include <cstdio>
using UINT=uint32_t;
using UINT64=uint64_t;
using std::size_t;
namespace gd_dx12 { struct Capture { ` + captureText[capacity:capacityEnd] + " };\n" + text[start:end] + ` }
int main() {
  unsigned width,height,pitch; unsigned long long offset,bytes,allocation,wantedFront;
  int missing,wanted; unsigned row=0;
  while(std::scanf("%u %u %llu %u %llu %llu %d %d %llu",&width,&height,&offset,&pitch,&bytes,&allocation,&missing,&wanted,&wantedFront)==9) {
    UINT64 front=UINT64(-1);
    bool result=gd_dx12::software_front_layout(width,height,offset,pitch,bytes,allocation,missing?nullptr:&front);
    if(result!=(wanted!=0) || (!missing && front!=wantedFront)) {
      std::printf("actual footprint row %u: accepted=%d front=%llu wanted=%d/%llu\n",row,result,(unsigned long long)front,wanted,wantedFront);
      return 1;
    }
    ++row;
  }
  std::printf("actual software footprint cases=%u\n",row);
}
`
	const cap = uint64(64 * 1024 * 1024)
	tests := []struct {
		name                      string
		width, height, pitch      uint32
		offset, bytes, allocation uint64
		nilOutput, valid          bool
		front                     uint64
	}{
		{"minimum", 1, 1, 256, 0, 4, 65536, false, true, 4},
		{"unaligned-width-two-rows", 65, 2, 512, 0, 772, 65536, false, true, 520},
		{"offset-last-pixel-at-boundary", 65, 2, 512, 512, 1284, 65536, false, true, 520},
		{"last-pixel-one-byte-outside", 65, 2, 512, 512, 1283, 65536, false, false, 0},
		{"max-dimension-single-row", 16384, 1, 65536, 0, 65536, 65536, false, true, 65536},
		{"timestamp-cap-exact", 1, 1, 256, cap - 20, cap - 16, cap, false, true, 4},
		{"timestamp-cap-one-byte-over", 1, 1, 256, 0, cap - 15, cap, false, false, 0},
		{"target-cap-exact", 1, 1, 256, 0, 4, cap, false, true, 4},
		{"target-cap-one-byte-over", 1, 1, 256, 0, 4, cap + 1, false, false, 0},
		{"large-bounded-front", 4096, 4095, 16384, 0, cap - 16384, cap, false, true, cap - 16384},
		{"packed-at-cap-no-timestamp-room", 4096, 4096, 16384, 0, cap, cap, false, false, 0},
		{"packed-over-cap", 8192, 4096, 32768, 0, cap - 16, cap, false, false, 0},
		{"zero-width", 0, 1, 256, 0, 4, 65536, false, false, 0},
		{"zero-height", 1, 0, 256, 0, 4, 65536, false, false, 0},
		{"width-over-limit", 16385, 1, 65792, 0, 65792, 131072, false, false, 0},
		{"height-over-limit", 1, 16385, 256, 0, 4194308, 65536, false, false, 0},
		{"pitch-not-256-aligned", 65, 2, 260, 0, 520, 65536, false, false, 0},
		{"pitch-shorter-than-row", 65, 2, 256, 0, 516, 65536, false, false, 0},
		{"offset-beyond-footprint", 1, 1, 256, 5, 4, 65536, false, false, 0},
		{"zero-allocation", 1, 1, 256, 0, 4, 0, false, false, 0},
		{"huge-offset-no-overflow", 1, 1, 256, ^uint64(0), 4, 65536, false, false, 0},
		{"huge-footprint-no-overflow", 1, 1, 256, 0, ^uint64(0), 65536, false, false, 0},
		{"huge-allocation-no-overflow", 1, 1, 256, 0, 4, ^uint64(0), false, false, 0},
		{"huge-row-pitch-no-overflow", 1, 16384, ^uint32(255), 0, cap - 16, cap, false, false, 0},
		{"null-output", 1, 1, 256, 0, 4, 65536, true, false, 0},
	}
	var input bytes.Buffer
	for _, test := range tests {
		missing, valid := 0, 0
		if test.nilOutput {
			missing = 1
		}
		if test.valid {
			valid = 1
		}
		fmt.Fprintf(&input, "%d %d %d %d %d %d %d %d %d\n", test.width, test.height, test.offset, test.pitch, test.bytes, test.allocation, missing, valid, test.front)
	}
	output := runShadowProbe(t, compileShadowProbe(t, source, true), input.Bytes())
	if strings.TrimSpace(string(output)) != fmt.Sprintf("actual software footprint cases=%d", len(tests)) {
		t.Fatalf("actual footprint corpus incomplete: %s", output)
	}
}
