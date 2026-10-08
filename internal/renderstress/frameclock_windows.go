//go:build windows

package main

import (
	"os"

	"github.com/neko233-com/godesktop/testing/winprobe"
)

func windowsWindowPresentation() (string, uint64, error) {
	pid := uint32(os.Getpid())
	window, err := winprobe.Find("godesktop native GPU stress", pid)
	if err != nil {
		return "", 0, err
	}
	presentation, err := winprobe.NativePresentation(window, pid)
	return presentation, uint64(window), err
}
