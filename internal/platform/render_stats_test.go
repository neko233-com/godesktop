package platform

import "testing"

func TestNativeFrameClockDiscriminatesPresentation(t *testing.T) {
	for _, test := range []struct {
		value uint32
		want  string
	}{{0, "unavailable"}, {1, "mtkview"}, {2, "cametaldisplaylink"}, {3, "dxgi"}, {4, "d3d12-fence"}, {5, "unavailable"}, {^uint32(0), "unavailable"}} {
		if got := nativeFrameClock(test.value); got != test.want {
			t.Errorf("native clock %d=%q, want %q", test.value, got, test.want)
		}
	}
}
