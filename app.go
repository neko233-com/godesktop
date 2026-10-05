package godesktop

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/neko233-com/godesktop/internal/platform"
)

// WindowOptions describes the initial client area, in device-independent pixels.
type WindowOptions struct {
	Title         string
	Width, Height float32
	Background    Color
}

// Context gives a view access to the UI event loop. It is valid until Run returns.
// Dispatch, Invalidate, and Quit are safe to call from background goroutines.
type Context struct {
	mu      sync.Mutex
	pending []func()
	closed  bool
	wake    func()
	quit    func()
}

// Dispatch schedules a state mutation on the UI thread and invalidates the view.
// It returns false after shutdown. Keep callbacks short; do I/O in a goroutine.
func (c *Context) Dispatch(fn func()) bool {
	if fn == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return false
	}
	c.pending = append(c.pending, fn)
	c.wake()
	return true
}

// Invalidate requests a frame. Native event loops coalesce repeated requests.
func (c *Context) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closed {
		c.wake()
	}
}

// Quit requests application shutdown.
func (c *Context) Quit() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closed {
		c.quit()
	}
}

func (c *Context) drain() {
	c.mu.Lock()
	pending := c.pending
	c.pending = nil
	c.mu.Unlock()
	for _, fn := range pending {
		fn()
	}
}

type application struct {
	context          *Context
	view             func(*Context) *Element
	frame            frame
	pressed, focused string
}

func (a *application) handle(event platform.Event) {
	switch event.Kind {
	case platform.Draw:
		a.context.drain()
		a.frame.commands = a.frame.commands[:0]
		a.frame.targets = a.frame.targets[:0]
		clear(a.frame.measured)
		clear(a.frame.keys)
		a.frame.focus = a.focused
		root := a.view(a.context)
		viewport := rect{0, 0, event.X, event.Y}
		a.frame.layout(root, viewport, viewport, "root")
		platform.Present(a.frame.commands)
	case platform.PointerDown:
		a.pressed = ""
		for i := len(a.frame.targets) - 1; i >= 0; i-- {
			t := a.frame.targets[i]
			if t.bounds.contains(event.X, event.Y) {
				a.pressed = t.key
				a.focused = t.key
				a.context.Invalidate()
				break
			}
		}
	case platform.PointerUp:
		pressed := a.pressed
		a.pressed = ""
		for i := len(a.frame.targets) - 1; i >= 0; i-- {
			t := a.frame.targets[i]
			if t.bounds.contains(event.X, event.Y) {
				if t.key == pressed {
					t.click(a.context)
					a.context.Invalidate()
				}
				break
			}
		}
	case platform.Cancel:
		a.pressed = ""
	case platform.KeyDown:
		if event.Key == platform.Tab && len(a.frame.targets) > 0 {
			next := 0
			if event.Modifiers&platform.Shift != 0 {
				next = len(a.frame.targets) - 1
			}
			for i, t := range a.frame.targets {
				if t.key == a.focused {
					next = (i + 1) % len(a.frame.targets)
					if event.Modifiers&platform.Shift != 0 {
						next = (i + len(a.frame.targets) - 1) % len(a.frame.targets)
					}
					break
				}
			}
			a.focused = a.frame.targets[next].key
			a.context.Invalidate()
		} else if event.Key == platform.Enter || event.Key == platform.Space {
			for _, t := range a.frame.targets {
				if t.key == a.focused {
					t.click(a.context)
					a.context.Invalidate()
					break
				}
			}
		}
	}
}

var running atomic.Bool

// Run opens a native window and blocks until it closes. Call it from main's
// goroutine; macOS requires the process's main OS thread. One window is supported.
// View and event callbacks execute on the UI thread. Panics become returned errors.
func Run(options WindowOptions, view func(*Context) *Element) error {
	if view == nil {
		return errors.New("godesktop: view must not be nil")
	}
	if strings.ContainsRune(options.Title, 0) {
		return errors.New("godesktop: title must not contain NUL")
	}
	if options.Width == 0 {
		options.Width = 960
	}
	if options.Height == 0 {
		options.Height = 640
	}
	for _, v := range []float32{options.Width, options.Height} {
		if v < 1 || v > 16384 || math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return errors.New("godesktop: window dimensions must be finite and between 1 and 16384")
		}
	}
	if options.Title == "" {
		options.Title = "godesktop"
	}
	if options.Background == (Color{}) {
		options.Background = RGB(0x111827)
	}
	if !running.CompareAndSwap(false, true) {
		return errors.New("godesktop: another window is already running")
	}
	defer running.Store(false)
	cx := &Context{wake: platform.Wake, quit: platform.Quit}
	defer func() { cx.mu.Lock(); cx.closed = true; cx.pending = nil; cx.mu.Unlock() }()
	a := &application{context: cx, view: view, frame: frame{measure: platform.MeasureText, textCache: make(map[textKey]dimensions), measured: make(map[*Element]dimensions), keys: make(map[string]bool)}}
	var callbackErr error
	err := platform.Run(platform.Options{Title: options.Title, Width: options.Width, Height: options.Height, Background: nativeColor(options.Background)}, func(e platform.Event) {
		if callbackErr != nil {
			return
		}
		defer func() {
			if p := recover(); p != nil {
				callbackErr = fmt.Errorf("godesktop: UI callback panicked: %v", p)
				platform.Quit()
			}
		}()
		a.handle(e)
	})
	return errors.Join(err, callbackErr)
}
