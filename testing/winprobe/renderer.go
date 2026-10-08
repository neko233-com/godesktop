package winprobe

import (
	"fmt"

	"github.com/neko233-com/godesktop/internal/platform"
)

// RenderStats describes the owned native renderer's actual submissions and
// resources. This diagnostic API performs no rendering or window input.
type RenderStats = platform.RenderStats

// RendererStats returns a thread-safe copy of the current/last Run's metrics.
// A minimized window may complete prior submissions without submitting new ones.
func RendererStats() RenderStats { return platform.RendererStats() }

// ValidateWindowsPresentation requires the exact D3D12 clock corresponding to
// the actual owned-window presentation path returned by NativePresentation.
// It does not infer a path from a requested adapter or accept arbitrary clocks.
func ValidateWindowsPresentation(stats RenderStats, presentation string) error {
	if stats.Backend != "direct3d12" {
		return fmt.Errorf("Windows presentation requires direct3d12, got %q", stats.Backend)
	}
	wanted := ""
	switch presentation {
	case "dxgi":
		wanted = "dxgi"
	case "committed-dib":
		wanted = "d3d12-fence"
	default:
		return fmt.Errorf("unknown native Windows presentation %q", presentation)
	}
	if stats.FrameClock != wanted {
		return fmt.Errorf("actual Windows presentation %s requires frame clock %s, got %s", presentation, wanted, stats.FrameClock)
	}
	return nil
}

func readWindowPresentation(window uintptr, pid uint32, owner func() uint32, property func(string) (uintptr, error)) (string, error) {
	if window == 0 || pid == 0 || owner() != pid {
		return "", fmt.Errorf("native presentation window is not owned by expected PID %d", pid)
	}
	backend, err := property("godesktop.backend")
	if err != nil {
		return "", err
	}
	if backend != 3 {
		return "", fmt.Errorf("native presentation HWND requires backend 3, got %d", backend)
	}
	value, err := property("godesktop.presentation")
	if err != nil {
		return "", err
	}
	presentation := ""
	switch value {
	case 1:
		presentation = "dxgi"
	case 2:
		presentation = "committed-dib"
	default:
		return "", fmt.Errorf("unknown native HWND presentation property %d", value)
	}
	if owner() != pid {
		return "", fmt.Errorf("native presentation HWND ownership changed during read")
	}
	return presentation, nil
}
