//go:build (!windows && !darwin) || !cgo

package godesktop

import (
	"errors"
	"testing"

	"github.com/neko233-com/godesktop/internal/platform"
)

func TestUnsupportedBackendDoesNotInvokeViewAndReleasesRunGuard(t *testing.T) {
	for i := 0; i < 2; i++ {
		err := Run(WindowOptions{}, func(*Context) *Element { t.Fatal("unavailable backend invoked view"); return nil })
		if !errors.Is(err, platform.ErrUnavailable) {
			t.Fatalf("run %d: %v", i, err)
		}
	}
}
