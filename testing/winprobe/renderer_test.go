package winprobe

import (
	"errors"
	"testing"
)

func TestWindowPresentationRequiresActualOwnedProperty(t *testing.T) {
	for _, test := range []struct {
		name          string
		window        uintptr
		pid, owner    uint32
		backend, path uintptr
		changed       bool
		want          string
	}{
		{"actual-dxgi", 1, 42, 42, 3, 1, false, "dxgi"},
		{"actual-committed-dib", 1, 42, 42, 3, 2, false, "committed-dib"},
		{"zero-hwnd", 0, 42, 42, 3, 1, false, ""},
		{"zero-pid", 1, 0, 0, 3, 1, false, ""},
		{"foreign-pid", 1, 42, 43, 3, 1, false, ""},
		{"closed-hwnd", 1, 42, 0, 3, 1, false, ""},
		{"wrong-backend", 1, 42, 42, 2, 1, false, ""},
		{"missing-property", 1, 42, 42, 3, 0, false, ""},
		{"unknown-property", 1, 42, 42, 3, 3, false, ""},
		{"reused-hwnd", 1, 42, 42, 3, 1, true, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			reads := 0
			owner := func() uint32 {
				reads++
				if test.changed && reads > 1 {
					return 43
				}
				return test.owner
			}
			property := func(name string) (uintptr, error) {
				switch name {
				case "godesktop.backend":
					return test.backend, nil
				case "godesktop.presentation":
					return test.path, nil
				default:
					t.Fatalf("unexpected property read %q", name)
					return 0, nil
				}
			}
			got, err := readWindowPresentation(test.window, test.pid, owner, property)
			if test.want == "" {
				if err == nil || got != "" {
					t.Fatalf("invalid actual HWND identity admitted: %q %v", got, err)
				}
			} else if err != nil || got != test.want {
				t.Fatalf("actual HWND path %q, want %q: %v", got, test.want, err)
			}
		})
	}
	want := errors.New("native query failed")
	_, err := readWindowPresentation(1, 42, func() uint32 { return 42 }, func(string) (uintptr, error) { return 0, want })
	if !errors.Is(err, want) {
		t.Fatal("native property read failure lost", err)
	}
}

func TestWindowsPresentationRequiresExactClockAndBackend(t *testing.T) {
	for _, path := range []string{"dxgi", "committed-dib", "", "unknown"} {
		for _, backend := range []string{"direct3d12", "metal", "direct2d", "unavailable"} {
			for _, clock := range []string{"dxgi", "d3d12-fence", "cametaldisplaylink", "unavailable"} {
				want := backend == "direct3d12" && (path == "dxgi" && clock == "dxgi" || path == "committed-dib" && clock == "d3d12-fence")
				err := ValidateWindowsPresentation(RenderStats{Backend: backend, FrameClock: clock}, path)
				if (err == nil) != want {
					t.Errorf("path=%q backend=%q clock=%q: %v", path, backend, clock, err)
				}
			}
		}
	}
}
