//go:build windows && cgo

// This executable is driven by native_windows_test.go through real Win32 messages.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	ui "github.com/neko233-com/godesktop"
	"github.com/neko233-com/godesktop/internal/platform"
	"github.com/neko233-com/godesktop/internal/testprotocol"
)

var output sync.Mutex

type probeTarget struct {
	context  *ui.Context
	snapshot func()
}

var active atomic.Pointer[probeTarget]

func emit(report testprotocol.Report) {
	output.Lock()
	defer output.Unlock()
	if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
		log.Fatal(err)
	}
}

func main() {
	scenario := flag.String("scenario", "interactive", "native integration scenario")
	runs := flag.Int("runs", 1, "sequential windows in the same process")
	flag.Parse()
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		log.Fatal("fixture requires windows/amd64")
	}
	go func() { time.Sleep(40 * time.Second); log.Fatal("native integration watchdog expired") }()
	go func() {
		input := bufio.NewScanner(os.Stdin)
		for input.Scan() {
			if input.Text() == "probe" {
				if target := active.Load(); target != nil {
					target.context.Dispatch(target.snapshot)
				}
			}
		}
	}()
	for iteration := 0; iteration < *runs; iteration++ {
		run(iteration, *scenario)
	}
}

func run(iteration int, scenario string) {
	clicks, async, frames := [2]int{}, 0, 0
	var saved *ui.Context
	guard := ""
	options := ui.WindowOptions{Title: fmt.Sprintf("godesktop-test-%d-%d", os.Getpid(), iteration), Width: 480, Height: 260, Background: ui.RGB(0x102030)}
	if scenario == "custom-desktop" {
		options.CustomTitlebar = true
		options.Width = 10000
		options.Height = 10000
	}
	err := ui.Run(options, func(cx *ui.Context) *ui.Element {
		saved = cx
		frames++
		if frames == 1 {
			if scenario == "quit" {
				cx.Quit()
			}
			active.Store(&probeTarget{cx, func() {
				emit(testprotocol.Report{Event: "probe", Run: iteration, Frame: frames, Clicks: clicks, Async: async, Guard: guard, NativeFrames: platform.RenderedFrames()})
			}})
			if scenario == "panic-view" {
				panic("fixture-view-panic")
			}
			if scenario == "panic-dispatch" {
				cx.Dispatch(func() { panic("fixture-dispatch-panic") })
			}
			if scenario == "reentrant" {
				nested := ui.Run(ui.WindowOptions{}, func(*ui.Context) *ui.Element { panic("nested view must not run") })
				if nested != nil {
					guard = nested.Error()
				}
			}
			if scenario == "interactive" {
				for i := 0; i < 8; i++ {
					go func() {
						for j := 0; j < 16; j++ {
							cx.Dispatch(func() { async++ })
						}
					}()
				}
			}
		}
		metrics := make([][2]float32, 0, 4)
		for _, text := range []string{"", "Unicode 你好 Á 😀", "a\x00b", "large"} {
			w, h := platform.MeasureText(text, 20)
			metrics = append(metrics, [2]float32{w, h})
		}
		emit(testprotocol.Report{Event: "frame", Run: iteration, Frame: frames, Clicks: clicks, Async: async, Guard: guard, NativeFrames: platform.RenderedFrames(), Metrics: metrics})
		if scenario == "empty" {
			return nil
		}
		if scenario == "custom-desktop" {
			return ui.Column(ui.Text("Custom titlebar").Height(36).Draggable(), ui.Column().Flex(1), ui.Column().Height(22).Background(ui.RGB(0x0078d4)))
		}
		firstKey, secondKey := "primary", "secondary"
		if scenario == "duplicate" {
			secondKey = firstKey
		}
		return ui.Column(
			ui.Row(
				ui.Button("Primary", func(*ui.Context) {
					if scenario == "panic-click" {
						panic("fixture-click-panic")
					}
					clicks[0]++
				}).Key(firstKey).Width(140).Height(40).Background(ui.RGB(0x229944)),
				ui.Button("Secondary", func(*ui.Context) { clicks[1]++ }).Key(secondKey).Width(140).Height(40),
				ui.Button("Disabled", nil).Key("disabled").Width(110).Height(40),
			).Height(40).Gap(10),
			ui.Row(
				ui.Column().Width(80).Height(60).Background(ui.RGB(0xff0000)),
				ui.Column().Width(80).Height(60).Background(ui.RGBA(0x0000ff, 0.5)),
			).Width(100).Height(60),
			ui.Text(fmt.Sprintf("Unicode 你好 Á 😀 — %d / %d", clicks[0], clicks[1])).FontSize(20),
		).Padding(10).Gap(10)
	})
	active.Store(nil)
	closed := saved != nil && !saved.Dispatch(func() { panic("closed context executed") })
	if saved != nil {
		saved.Invalidate()
		saved.Quit()
	}
	message := ""
	if err != nil {
		message = err.Error()
	}
	emit(testprotocol.Report{Event: "closed", Run: iteration, Frame: frames, Clicks: clicks, Async: async, Error: message, Guard: guard, Closed: closed, NativeFrames: platform.RenderedFrames()})
}
