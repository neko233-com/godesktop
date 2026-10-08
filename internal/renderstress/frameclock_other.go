//go:build !windows

package main

import "errors"

func windowsWindowPresentation() (string, uint64, error) {
	return "", 0, errors.New("Windows native presentation requires an owned Windows HWND")
}
