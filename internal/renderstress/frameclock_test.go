package main

import (
	"testing"

	"github.com/neko233-com/godesktop/internal/platform"
)

func TestRequiredFrameClockDoesNotBroadenDXGI(t *testing.T) {
	for _, test := range []struct {
		backend, clock, required, path string
		valid                          bool
	}{
		{"direct3d12", "dxgi", "dxgi", "", true},
		{"direct3d12", "d3d12-fence", "dxgi", "committed-dib", false},
		{"direct3d12", "dxgi", "windows-native", "dxgi", true},
		{"direct3d12", "d3d12-fence", "windows-native", "committed-dib", true},
		{"direct3d12", "dxgi", "windows-native", "committed-dib", false},
		{"direct3d12", "d3d12-fence", "windows-native", "dxgi", false},
		{"direct3d12", "dxgi", "windows-native", "", false},
		{"metal", "dxgi", "windows-native", "dxgi", false},
		{"direct3d12", "unavailable", "windows-native", "committed-dib", false},
		{"metal", "cametaldisplaylink", "cametaldisplaylink", "", true},
		{"metal", "mtkview", "cametaldisplaylink", "", false},
	} {
		if err := validateRequiredFrameClock(platform.RenderStats{Backend: test.backend, FrameClock: test.clock}, test.required, test.path); (err == nil) != test.valid {
			t.Errorf("backend=%s clock=%s required=%s actual=%s: %v", test.backend, test.clock, test.required, test.path, err)
		}
	}
}
