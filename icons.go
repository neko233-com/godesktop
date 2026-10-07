package godesktop

import "github.com/neko233-com/godesktop/internal/platform"

// Icon creates a vector icon: files, search, source-control, debug, extensions,
// settings, account, close, chevron-right, chevron-down, split, minus, maximize,
// terminal. Unknown names draw nothing.
func Icon(name string) *Element { e := element(iconKind); e.icon = name; return e }

var iconPaths = map[string][][]float32{
	"files":          {{8, 7, 19, 7, 19, 21, 8, 21, 8, 7}, {5, 17, 5, 3, 15, 3, 15, 5}},
	"search":         {{15, 15, 21, 21}, {6, 3, 12, 3, 16, 7, 16, 12, 12, 16, 6, 16, 2, 12, 2, 7, 6, 3}},
	"source-control": {{6, 5, 6, 17}, {6, 12, 15, 12, 17, 10, 17, 6}, {4, 3, 8, 3, 8, 7, 4, 7, 4, 3}, {4, 17, 8, 17, 8, 21, 4, 21, 4, 17}, {15, 2, 19, 2, 19, 6, 15, 6, 15, 2}},
	"debug":          {{4, 3, 4, 19, 17, 11, 4, 3}, {13, 16, 21, 16, 21, 22, 13, 22, 13, 16}},
	"extensions":     {{3, 3, 10, 3, 10, 10, 3, 10, 3, 3}, {3, 13, 10, 13, 10, 20, 3, 20, 3, 13}, {13, 13, 20, 13, 20, 20, 13, 20, 13, 13}, {15, 2, 22, 4, 20, 11, 13, 9, 15, 2}},
	"settings":       {{8, 3, 16, 3, 16, 6, 20, 8, 22, 13, 19, 16, 16, 17, 16, 21, 8, 21, 8, 17, 4, 15, 2, 10, 5, 7, 8, 6, 8, 3}, {9, 9, 15, 9, 15, 15, 9, 15, 9, 9}},
	"account":        {{8, 3, 16, 3, 18, 7, 16, 11, 8, 11, 6, 7, 8, 3}, {3, 21, 3, 17, 8, 14, 16, 14, 21, 17, 21, 21, 3, 21}},
	"close":          {{6, 6, 18, 18}, {18, 6, 6, 18}},
	"chevron-right":  {{9, 6, 15, 12, 9, 18}},
	"chevron-down":   {{6, 9, 12, 15, 18, 9}},
	"split":          {{3, 4, 21, 4, 21, 20, 3, 20, 3, 4}, {12, 4, 12, 20}},
	"minus":          {{6, 12, 18, 12}},
	"maximize":       {{6, 6, 18, 6, 18, 18, 6, 18, 6, 6}},
	"terminal":       {{3, 5, 21, 5, 21, 20, 3, 20, 3, 5}, {7, 9, 10, 12, 7, 15}, {13, 15, 17, 15}},
}

func (f *frame) paintIcon(e *Element, bounds, clip rect) {
	size := min(float32(24), min(bounds.w, bounds.h))
	scale := size / 24
	x, y := bounds.x+(bounds.w-size)/2, bounds.y+(bounds.h-size)/2
	for _, path := range iconPaths[e.icon] {
		for i := 0; i+3 < len(path); i += 2 {
			f.appendCommand(platform.Command{Kind: platform.Line, Bounds: platform.Rect{X: x + path[i]*scale, Y: y + path[i+1]*scale, W: (path[i+2] - path[i]) * scale, H: (path[i+3] - path[i+1]) * scale}, Clip: nativeRect(clip), Color: nativeColor(e.foreground), Radius: 1.4 * scale})
		}
	}
}
