// Package testprotocol defines the JSON protocol for native integration fixtures.
package testprotocol

type Report struct {
	Event        string
	Run, Frame   int
	Clicks       [2]int
	Async        int
	Error, Guard string
	Closed       bool
	NativeFrames uint64
	Metrics      [][2]float32
}
