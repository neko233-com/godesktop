//go:build !darwin || !cgo

package main

import "errors"

func metalWindowIdentity() uint64 { return 0 }
func metalRecoveryPixels(string) error {
	return errors.New("Metal recovery pixels require macOS with cgo")
}
