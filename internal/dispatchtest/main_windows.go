//go:build windows && cgo

package main

import (
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"syscall"
	"time"

	ui "github.com/neko233-com/godesktop"
	"github.com/neko233-com/godesktop/testing/winprobe"
)

var user = syscall.NewLazyDLL("user32.dll")
var currentThread = syscall.NewLazyDLL("kernel32.dll").NewProc("GetCurrentThreadId")

func threadID() uint32 { value, _, _ := currentThread.Call(); return uint32(value) }
func iconic(w winprobe.Window) bool {
	value, _, _ := user.NewProc("IsIconic").Call(uintptr(w))
	return value != 0
}
func visible(w winprobe.Window) bool {
	value, _, _ := user.NewProc("IsWindowVisible").Call(uintptr(w))
	return value != 0
}
func await(name string, condition func() bool) error {
	until := time.Now().Add(5 * time.Second)
	for time.Now().Before(until) {
		if condition() {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("owned dispatch fixture timed out waiting for %s", name)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	if os.Getenv("GODESKTOP_READBACK") != "1" || os.Getenv("GODESKTOP_TEST_INPUT_ISOLATION") != "1" {
		return errors.New("owned dispatch fixture requires readback and input isolation")
	}
	watchdog := time.AfterFunc(30*time.Second, func() {
		fmt.Fprintln(os.Stderr, "native minimized dispatch fixture timed out")
		os.Exit(2)
	})
	defer watchdog.Stop()
	var previous *ui.Context
	for run := 0; run < 2; run++ {
		cx, err := runWindow(run, previous)
		if err != nil {
			return err
		}
		previous = cx
		var leaked atomic.Bool
		if cx.Dispatch(func() { leaked.Store(true) }) || leaked.Load() {
			return errors.New("stopped native Context accepted a callback")
		}
	}
	fmt.Println("native minimized/hidden dispatch contract passed: 512 UI-thread callbacks including reentrant work, frozen views/GPU while minimized and hidden, rendered restoration, minimized Quit, closed Context across two Runs")
	return nil
}

func runWindow(run int, previous *ui.Context) (*ui.Context, error) {
	title := fmt.Sprintf("godesktop owned minimized dispatch %d-%d", os.Getpid(), run)
	var views atomic.Int32
	var callbacks atomic.Int32
	var uiThread atomic.Uint32
	var native atomic.Pointer[ui.Context]
	started := false
	state := 0 // UI-owned; hidden workers mutate it only through Dispatch.
	done := make(chan error, 1)
	err := ui.Run(ui.WindowOptions{Title: title, Width: 420, Height: 220}, func(cx *ui.Context) *ui.Element {
		native.Store(cx)
		uiThread.Store(threadID())
		views.Add(1)
		if previous != nil {
			// These old-context operations must not minimize/quit or dispatch
			// into a later native Run whose HWND is now the global backend.
			if previous.Dispatch(func() { state = -1000 }) {
				cx.Quit()
				return ui.Text("closed Context unexpectedly accepted work")
			}
			previous.Invalidate()
			previous.Minimize()
			previous.Quit()
		}
		if !started && cx.RenderedFrames() >= 2 {
			started = true
			go func() {
				failure := func() error {
					w, err := winprobe.Find(title, uint32(os.Getpid()))
					if err != nil {
						return err
					}
					stable := func(mode string) error {
						if mode == "minimized" {
							w.Show(6) // SW_MINIMIZE, only this verified owned HWND.
						} else {
							w.Show(0) // SW_HIDE.
						}
						return await(mode+" idle fence", func() bool {
							stats := winprobe.RendererStats()
							matches := iconic(w)
							if mode == "hidden" {
								matches = !visible(w)
							}
							return matches && stats.InFlight == 0 && stats.Submitted == stats.Completed
						})
					}
					checkFrozen := func(stage string, before winprobe.RenderStats, oldViews int32) error {
						// A real stable idle interval detects a render/view busy loop;
						// Dispatch is allowed to request a future visible repaint.
						time.Sleep(150 * time.Millisecond)
						after := winprobe.RendererStats()
						if views.Load() != oldViews || after.Submitted != before.Submitted || after.Completed != before.Completed || after.InFlight != 0 {
							return fmt.Errorf("%s unexpectedly rendered: views %d/%d submitted %d/%d completed %d/%d inFlight=%d", stage, oldViews, views.Load(), before.Submitted, after.Submitted, before.Completed, after.Completed, after.InFlight)
						}
						return nil
					}
					if run == 0 {
						for _, mode := range []string{"minimized", "hidden"} {
							if err = stable(mode); err != nil {
								return err
							}
							before, oldViews := winprobe.RendererStats(), views.Load()
							receipts := make(chan error, 256)
							checkCallback := func() error {
								if threadID() != uiThread.Load() {
									return errors.New("Dispatch callback left the native UI thread")
								}
								if mode == "minimized" && !iconic(w) || mode == "hidden" && visible(w) {
									return errors.New("Dispatch required restoring the owned window")
								}
								state++
								callbacks.Add(1)
								return nil
							}
							for range 128 {
								if !cx.Dispatch(func() {
									receipts <- checkCallback()
									if !cx.Dispatch(func() { receipts <- checkCallback() }) {
										receipts <- errors.New("reentrant Dispatch was rejected below queue capacity")
									}
								}) {
									return errors.New("background Dispatch was rejected below queue capacity")
								}
							}
							for range 256 {
								select {
								case err = <-receipts:
									if err != nil {
										return err
									}
								case <-time.After(5 * time.Second):
									return fmt.Errorf("%s Dispatch receipt did not arrive, completed=%d", mode, callbacks.Load())
								}
							}
							if err = checkFrozen(mode, before, oldViews); err != nil {
								return err
							}
							w.Show(9) // SW_RESTORE: no global mouse/keyboard injection.
							if err = await("visible repaint after "+mode, func() bool {
								stats := winprobe.RendererStats()
								return visible(w) && !iconic(w) && views.Load() > oldViews && stats.Completed > before.Completed
							}); err != nil {
								return err
							}
							pixel, e := w.Pixel(20, 60)
							if e != nil || pixel != 0x22c55e {
								return fmt.Errorf("restored native state GPU pixel=%06x, want green 22c55e: %w", pixel, e)
							}
							fmt.Printf("owned %s dispatch completed callbacks=%d views=%d submitted=%d\n", mode, callbacks.Load(), views.Load(), winprobe.RendererStats().Submitted)
						}
						if callbacks.Load() != 512 {
							return fmt.Errorf("actual callback receipts=%d, want 512", callbacks.Load())
						}
					}
					if err = stable("minimized"); err != nil {
						return err
					}
					before, oldViews := winprobe.RendererStats(), views.Load()
					quitReceipt := make(chan error, 1)
					if !cx.Dispatch(func() {
						err := error(nil)
						if !iconic(w) || threadID() != uiThread.Load() || views.Load() != oldViews || winprobe.RendererStats().Submitted != before.Submitted {
							err = errors.New("minimized Quit callback required a render, restore or different UI thread")
						}
						quitReceipt <- err
						cx.Quit()
					}) {
						return errors.New("minimized shutdown Dispatch was rejected")
					}
					select {
					case err = <-quitReceipt:
						return err
					case <-time.After(5 * time.Second):
						return errors.New("Quit Dispatch stalled while the owned window was minimized")
					}
				}()
				if failure != nil {
					cx.Quit()
				}
				done <- failure
			}()
		}
		if !started {
			cx.Invalidate()
		}
		color := uint32(0xef4444)
		if state > 0 {
			color = 0x22c55e
		}
		return ui.Column(ui.Text("Minimized/hidden native dispatch").Height(30), ui.Column().Height(60).Background(ui.RGB(color)), ui.Text(fmt.Sprintf("UI-owned state: %d", state)))
	})
	if err != nil {
		return native.Load(), err
	}
	select {
	case err = <-done:
		return native.Load(), err
	case <-time.After(5 * time.Second):
		return native.Load(), errors.New("native worker did not stop after window exit")
	}
}
