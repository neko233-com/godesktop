//go:build !windows || !cgo

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "native shutdown failure acceptance requires Windows with cgo")
	os.Exit(1)
}
