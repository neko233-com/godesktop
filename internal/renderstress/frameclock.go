package main

import (
	"fmt"

	"github.com/neko233-com/godesktop/internal/platform"
	"github.com/neko233-com/godesktop/testing/winprobe"
)

func validateRequiredFrameClock(stats platform.RenderStats, required, presentation string) error {
	if required == "windows-native" {
		return winprobe.ValidateWindowsPresentation(stats, presentation)
	}
	if stats.FrameClock != required {
		return fmt.Errorf("required %s frame clock, got %s", required, stats.FrameClock)
	}
	return nil
}
