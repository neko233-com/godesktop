//go:build !windows

package main

import "errors"

func recoveryPixels(string) error { return errors.New("D3D12 device removal requires Windows") }
