//go:build !windows || !cgo

package main

import "errors"

func nativeProbe() ([]byte, error) {
	return nil, errors.New("D3D12 timeout acceptance requires Windows with cgo")
}
