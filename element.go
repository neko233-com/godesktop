package godesktop

type elementKind uint8

const (
	columnKind elementKind = iota
	rowKind
	textKind
	buttonKind
	iconKind
	stackKind
	imageKind
	viewportKind
)

// Element is a transient description of a view. Build a fresh tree in each view
// callback. Elements and click callbacks are used only on the UI thread.
type Element struct {
	kind                                                elementKind
	children                                            []*Element
	text                                                string
	key                                                 string
	fontFamily, icon                                    string
	flexBasisZero, draggable                            bool
	focusRing                                           bool
	separatePadding                                     bool
	paddingX, paddingY                                  float32
	click                                               func(*Context)
	width, height, grow, padding, gap, radius, fontSize float32
	background, foreground                              Color
	bitmap                                              *Bitmap
	scrollX, scrollY                                    float32
}

func element(kind elementKind) *Element {
	return &Element{kind: kind, fontSize: 16, foreground: RGB(0xe5e7eb), focusRing: true}
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

// Stack overlays children in order, painting the last child on top. Children
// fill the available area unless they specify Width or Height.
func Stack(children ...*Element) *Element {
	e := element(stackKind)
	e.children = children
	return e
}

// Viewport clips a single content tree to its padded available space. Content
// keeps its intrinsic size, at least filling the viewport; only visible paint
// and hit targets are emitted. The application owns its scroll state.
func Viewport(content *Element) *Element {
	e := element(viewportKind)
	e.children = []*Element{content}
	return e
}

// ScrollOffset moves Viewport content toward earlier coordinates by the given
// positive DIP offsets. Layout clamps offsets to the actual content extent.
func (e *Element) ScrollOffset(x, y float32) *Element {
	e.scrollX, e.scrollY = nonnegative(x), nonnegative(y)
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

// Key assigns a unique visible identity for geometry, focus and pointer capture.
// A passive element exposes ElementBounds without entering keyboard focus order.
func (e *Element) Key(key string) *Element { e.key = key; return e }

// FontFamily selects a platform font; an empty name uses the system UI font.
func (e *Element) FontFamily(name string) *Element { e.fontFamily = name; return e }

// Flex distributes remaining space from a zero basis, suitable for workspaces.
func (e *Element) Flex(weight float32) *Element {
	e.grow = nonnegative(weight)
	e.flexBasisZero = true
	return e
}

// OnClick makes an element focusable and clickable, using the same button semantics.
func (e *Element) OnClick(fn func(*Context)) *Element { e.click = fn; return e }

// FocusRing controls the default focus indicator. Custom workbenches can draw
// their own keyboard focus/selection styles without a second generic outline.
// Keyboard focus and click semantics remain available when it is false.
func (e *Element) FocusRing(show bool) *Element { e.focusRing = show; return e }

// Draggable marks an otherwise non-interactive custom titlebar region.
func (e *Element) Draggable() *Element        { e.draggable = true; return e }
func (e *Element) Width(px float32) *Element  { e.width = nonnegative(px); return e }
func (e *Element) Height(px float32) *Element { e.height = nonnegative(px); return e }

// Grow shares remaining space along the parent's main axis using this weight.
func (e *Element) Grow(weight float32) *Element { e.grow = nonnegative(weight); return e }
func (e *Element) Padding(px float32) *Element {
	e.padding = nonnegative(px)
	e.separatePadding = false
	return e
}

// PaddingXY sets independent horizontal and vertical insets.
func (e *Element) PaddingXY(x, y float32) *Element {
	e.paddingX, e.paddingY = nonnegative(x), nonnegative(y)
	e.separatePadding = true
	return e
}
func (e *Element) insets() (float32, float32) {
	if e.separatePadding {
		return e.paddingX, e.paddingY
	}
	return e.padding, e.padding
}
func (e *Element) Gap(px float32) *Element         { e.gap = nonnegative(px); return e }
func (e *Element) Radius(px float32) *Element      { e.radius = nonnegative(px); return e }
func (e *Element) FontSize(px float32) *Element    { e.fontSize = max(1, nonnegative(px)); return e }
func (e *Element) Background(color Color) *Element { e.background = color; return e }
func (e *Element) Foreground(color Color) *Element { e.foreground = color; return e }
