// Package testprotocol defines the JSON protocol for native integration fixtures.
package testprotocol

import "github.com/neko233-com/godesktop/internal/platform"

type Report struct {
	Event        string
	Run, Frame   int
	Clicks       [2]int
	Async        int
	Error, Guard string
	Closed       bool
	NativeFrames uint64
	Metrics      [][2]float32
	Renderer     platform.RenderStats
}
