//go:build windows && cgo

package main

import (
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	ui "github.com/neko233-com/godesktop"
	"github.com/neko233-com/godesktop/testing/winprobe"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if os.Getenv("GODESKTOP_TEST_INPUT_ISOLATION") != "1" {
		return errors.New("native focus fixture requires isolated owned message replay")
	}
	var events, cancellations atomic.Int32
	var focused atomic.Bool
	var uiThread atomic.Uint32
	var wrongThread atomic.Bool
	threadID := syscall.NewLazyDLL("kernel32.dll").NewProc("GetCurrentThreadId")
	currentThread := func() uint32 { id, _, _ := threadID.Call(); return uint32(id) }
	title := fmt.Sprintf("godesktop owned focus events %d", os.Getpid())
	started := false
	var failure error
	watchdog := time.AfterFunc(15*time.Second, func() {
		fmt.Fprintln(os.Stderr, "native window focus events timed out")
		os.Exit(2)
	})
	defer watchdog.Stop()
	err := ui.Run(ui.WindowOptions{Title: title, Width: 400, Height: 260, Input: func(_ *ui.Context, e ui.InputEvent) bool {
		if uiThread.Load() != 0 && uiThread.Load() != currentThread() {
			wrongThread.Store(true)
		}
		switch e.Kind {
		case ui.WindowFocusChanged:
			focused.Store(e.Focused)
			events.Add(1)
		case ui.InputCancelled:
			cancellations.Add(1)
		}
		return false
	}}, func(cx *ui.Context) *ui.Element {
		uiThread.Store(currentThread())
		if !started && cx.RenderedFrames() > 2 {
			started = true
			go func() {
				w, err := winprobe.Find(title, uint32(os.Getpid()))
				check := func(message uint32, value uintptr, count int32, wantFocus bool) {
					if err != nil {
						return
					}
					err = w.Send(message, value, 0)
					if err == nil && (events.Load() != count || focused.Load() != wantFocus) {
						err = fmt.Errorf("owned message 0x%x/%d focus events=%d focused=%t, want %d/%t", message, value, events.Load(), focused.Load(), count, wantFocus)
					}
				}
				// Physical activation during fixture creation was isolated. The
				// first inactive state is not a change from the initial inactive.
				check(0x6, 0, 0, false)  // WM_ACTIVATE / WA_INACTIVE
				check(0x6, 1, 1, true)   // WA_ACTIVE
				check(0x6, 1, 1, true)   // repeated activation
				check(0x6, 2, 1, true)   // WA_CLICKACTIVE is still the same state
				check(0x215, 0, 1, true) // WM_CAPTURECHANGED retains only Cancel
				check(0x8, 0, 1, true)   // WM_KILLFOCUS retains only Cancel
				if err == nil && cancellations.Load() != 2 {
					err = fmt.Errorf("ordinary focus/capture cancellation count=%d, want 2", cancellations.Load())
				}
				check(0x6, 0, 2, false)
				check(0x6, 0, 2, false)
				check(0x6, 1, 3, true)
				// Replay must not bypass the physical-input exclusion. Send an
				// unwrapped activation only to the PID-verified fixture HWND.
				if err == nil {
					var result uintptr
					ok, _, callErr := syscall.NewLazyDLL("user32.dll").NewProc("SendMessageTimeoutW").Call(uintptr(w), 0x6, 0, 0, 0x2|0x20, 3000, uintptr(unsafe.Pointer(&result)))
					if ok == 0 {
						err = fmt.Errorf("owned physical activation probe: %w", callErr)
					} else if events.Load() != 3 || !focused.Load() {
						err = errors.New("unwrapped activation escaped isolated native fixture")
					}
				}
				// The high word is minimized state, not an activation enum.
				check(0x6, 1<<16, 4, false)
				check(0x6, 2, 5, true)
				if err == nil && wrongThread.Load() {
					err = errors.New("native focus callback left the UI thread")
				}
				cx.Dispatch(func() { failure = err; cx.Quit() })
			}()
		}
		cx.Invalidate()
		return ui.Column(ui.Text("Owned native activation and capture cancellation"))
	})
	if err != nil {
		return err
	}
	if failure != nil {
		return failure
	}
	fmt.Println("native window activation event contract passed: active/inactive dedup, independent cancel, isolation, UI thread")
	return nil
}
