// Package platform is the internal ABI between the Go view system and native UI.
package platform

import "errors"

var ErrUnavailable = errors.New("godesktop: native backend requires Windows or macOS with CGO_ENABLED=1")

type Color struct{ R, G, B, A float32 }
type Rect struct{ X, Y, W, H float32 }

const MaxBitmapEdge = 4096
const MaxBitmapBytes = 64 << 20
const MaxFrameBitmaps = 128

// Bitmap is private to the public immutable wrapper. Native code copies pixels
// on cache misses and never retains Go pointers.
type Bitmap struct {
	ID            uint64
	Width, Height int
	Pixels        []byte
}

type Command struct {
	Kind             int
	Bounds, Clip     Rect
	Color            Color
	Radius, FontSize float32
	Text             string
	FontFamily       string
	Bitmap           *Bitmap
}

const (
	Rectangle   = 1
	Label       = 2
	Line        = 3
	DragRegion  = 4
	BitmapImage = 5
	Draw        = 1
	PointerDown = 2
	PointerUp   = 3
	KeyDown     = 4
	Cancel      = 5
	Tab         = 9
	Enter       = 13
	Escape      = 27
	Space       = 32
	Shift       = 1
)

type Options struct {
	Title          string
	Width, Height  float32
	Background     Color
	CustomTitlebar bool
	CloseRequested func() bool
}

type Event struct {
	Kind               int
	X, Y               float32
	Key, Modifiers     int
	PointerX, PointerY float32
}
