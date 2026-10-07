package godesktop

import (
	"github.com/neko233-com/godesktop/internal/platform"
)

type InputKind int

const (
	PointerPressed  InputKind = 2
	PointerReleased InputKind = 3
	KeyPressed      InputKind = 4
	InputCancelled  InputKind = 5
	Character       InputKind = 6
	Scroll          InputKind = 7
	PointerMoved    InputKind = 8
	KeyReleased     InputKind = 9
	// WindowFocusChanged reports native top-level activation. It is independent
	// of keyboard focus within the window and pointer capture cancellation.
	WindowFocusChanged InputKind = 10
	ModifierShift                = 1
	ModifierControl              = 2
	ModifierAlt                  = 4
	ModifierCommand              = 8
)

// InputEvent coordinates are in DIP. Character.Key is a Unicode scalar value;
// Scroll.Y is the number of lines (positive scrolls toward earlier content);
// Scroll.X is horizontal lines (positive scrolls toward later/right content).
// Scroll.PointerX/PointerY give its client position, independently of deltas.
type InputEvent struct {
	Kind               InputKind
	X, Y               float32
	Key, Modifiers     int
	Repeat             bool
	PointerX, PointerY float32
	// Focused is meaningful only for WindowFocusChanged. Callbacks run on the
	// native UI thread, including activation before the first rendered frame.
	Focused bool
}

// RenderedFrames returns successful native submissions for smoke diagnostics.
func (c *Context) RenderedFrames() uint64 { return platform.RenderedFrames() }

// WindowSize returns the current client viewport in DIP.
func (c *Context) WindowSize() (float32, float32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.width, c.height
}
