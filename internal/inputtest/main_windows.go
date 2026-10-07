//go:build windows && cgo

package main

import (
	"errors"
	"fmt"
	ui "github.com/neko233-com/godesktop"
	"github.com/neko233-com/godesktop/testing/winprobe"
	"os"
	"sync/atomic"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	var clicked, cancelled, hover, alt atomic.Int32
	var failure error
	started := false
	watchdog := time.AfterFunc(15*time.Second, func() { fmt.Fprintln(os.Stderr, "native menu events timed out"); os.Exit(2) })
	defer watchdog.Stop()
	err := ui.Run(ui.WindowOptions{Title: "godesktop owned menu events", Width: 400, Height: 260, Input: func(_ *ui.Context, e ui.InputEvent) bool {
		switch e.Kind {
		case ui.InputCancelled:
			cancelled.Add(1)
		case ui.PointerMoved:
			hover.Add(1)
		case ui.KeyPressed:
			if e.Key == 'F' {
				alt.Add(1)
			}
		}
		return false
	}}, func(cx *ui.Context) *ui.Element {
		if !started && cx.RenderedFrames() > 3 {
			started = true
			go func() {
				w, err := winprobe.Find("godesktop owned menu events", uint32(os.Getpid()))
				if err == nil {
					err = w.Pointer(0x201, 100, 40)
				}
				if err == nil {
					err = w.Pointer(0x202, 100, 40)
				}
				if err == nil && clicked.Load() != 1 {
					err = errors.New("native press/release did not activate menu")
				}
				if err == nil && cancelled.Load() != 0 {
					err = errors.New("normal capture release cancelled opened menu")
				}
				before := hover.Load()
				if err == nil {
					err = w.Pointer(0x200, 160, 80)
				}
				if err == nil && hover.Load() != before+1 {
					err = errors.New("unpressed pointer hover was not delivered")
				}
				if err == nil {
					err = w.Send(0x104, 'F', 1<<29)
				}
				if err == nil && alt.Load() != 1 {
					err = errors.New("Alt mnemonic system key was not delivered")
				}
				if err == nil {
					err = w.Send(0x215, 0, 0)
				}
				if err == nil && cancelled.Load() != 1 {
					err = errors.New("stolen capture cancellation was lost")
				}
				cx.Dispatch(func() { failure = err; cx.Quit() })
			}()
		}
		cx.Invalidate()
		return ui.Column(ui.Button("File", func(*ui.Context) { clicked.Add(1) }).Key("file").Height(80), ui.Text("Owned native hover and Alt menu event contract"))
	})
	if err != nil {
		return err
	}
	if failure != nil {
		return failure
	}
	fmt.Println("native normal-release / hover / Alt / stolen-capture event contract passed")
	return nil
}
