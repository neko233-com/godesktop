package godesktop

type elementKind uint8

const (
	columnKind elementKind = iota
	rowKind
	textKind
	buttonKind
)

// Element is a transient description of a view. Build a fresh tree in each view
// callback. Elements and click callbacks are used only on the UI thread.
type Element struct {
	kind                                                elementKind
	children                                            []*Element
	text                                                string
	key                                                 string
	click                                               func(*Context)
	width, height, grow, padding, gap, radius, fontSize float32
	background, foreground                              Color
}

func element(kind elementKind) *Element {
	return &Element{kind: kind, fontSize: 16, foreground: RGB(0xe5e7eb)}
}

// Column lays out children vertically. Nil children are ignored.
func Column(children ...*Element) *Element {
	e := element(columnKind)
	e.children = children
	return e
}

// Row lays out children horizontally. Nil children are ignored.
func Row(children ...*Element) *Element {
	e := element(rowKind)
	e.children = children
	return e
}

// Text creates a single-line label shaped by the platform's text engine.
func Text(value string) *Element {
	e := element(textKind)
	e.text = value
	return e
}

// Button creates a focusable button. Use Key for stable identity across renders.
// A nil callback creates a disabled button.
func Button(label string, onClick func(*Context)) *Element {
	e := element(buttonKind)
	e.text, e.click = label, onClick
	e.padding, e.radius, e.background = 12, 8, RGB(0x2563eb)
	return e
}

// Key assigns a unique stable identity used for focus and pointer capture.
func (e *Element) Key(key string) *Element    { e.key = key; return e }
func (e *Element) Width(px float32) *Element  { e.width = nonnegative(px); return e }
func (e *Element) Height(px float32) *Element { e.height = nonnegative(px); return e }

// Grow shares remaining space along the parent's main axis using this weight.
func (e *Element) Grow(weight float32) *Element    { e.grow = nonnegative(weight); return e }
func (e *Element) Padding(px float32) *Element     { e.padding = nonnegative(px); return e }
func (e *Element) Gap(px float32) *Element         { e.gap = nonnegative(px); return e }
func (e *Element) Radius(px float32) *Element      { e.radius = nonnegative(px); return e }
func (e *Element) FontSize(px float32) *Element    { e.fontSize = max(1, nonnegative(px)); return e }
func (e *Element) Background(color Color) *Element { e.background = color; return e }
func (e *Element) Foreground(color Color) *Element { e.foreground = color; return e }
