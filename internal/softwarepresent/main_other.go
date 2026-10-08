//go:build !windows || !cgo

package main

import "fmt"

func main() { fmt.Println("software native presentation acceptance requires Windows and cgo") }
