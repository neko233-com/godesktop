// Native close deferral, explicit force and panic shutdown acceptance.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"

	ui "github.com/neko233-com/godesktop"
	"github.com/neko233-com/godesktop/internal/platform"
)

func main() {
	mode := flag.String("mode", "guard", "guard, force, panic or os")
	flag.Parse()
	requests, allow, started, finished := 0, false, false, false
	ready := false
	done := make(chan struct{})
	var viewFrames atomic.Uint64
	watchdog := time.AfterFunc(10*time.Second, func() {
		stats, _ := json.Marshal(platform.RendererStats())
		fmt.Fprintf(os.Stderr, "close guard timed out: view_frames=%d renderer=%s\n", viewFrames.Load(), stats)
		os.Exit(2)
	})
	defer watchdog.Stop()
	err := ui.Run(ui.WindowOptions{Title: "godesktop close guard", Width: 640, Height: 420, CloseRequested: func(*ui.Context) bool {
		requests++
		fmt.Printf("close request %d allow=%t\n", requests, allow)
		if *mode == "panic" {
			panic("close acceptance")
		}
		return allow
	}}, func(cx *ui.Context) *ui.Element {
		viewFrames.Add(1)
		if !started {
			started = true
			go func() {
				ticker := time.NewTicker(20 * time.Millisecond)
				defer ticker.Stop()
				for {
					select {
					case <-done:
						return
					case <-ticker.C:
						cx.Invalidate()
					}
				}
			}()
		}
		if !ready && cx.RenderedFrames() >= 2 {
			ready = true
			fmt.Println("close guard frame ready")
		}
		if *mode != "os" && requests == 0 && cx.RenderedFrames() >= 2 && !finished {
			finished = true
			cx.RequestClose()
		}
		if requests == 1 && cx.RenderedFrames() >= 4 {
			switch *mode {
			case "guard":
				allow = true
				cx.RequestClose()
			case "force":
				cx.Quit()
			}
		}
		return ui.Column(ui.Text("Unsaved document — keep the window open"), ui.Button("Save and close", func(*ui.Context) { allow = true; cx.RequestClose() }).Key("save-close")).Padding(32)
	})
	close(done)
	valid := (*mode == "guard" || *mode == "os") && requests == 2 && allow && err == nil || *mode == "force" && requests == 1 && err == nil || *mode == "panic" && requests == 1 && err != nil && strings.Contains(err.Error(), "close callback panicked")
	if !valid {
		fmt.Fprintf(os.Stderr, "close guard failed mode=%s requests=%d allow=%t error=%v\n", *mode, requests, allow, err)
		os.Exit(1)
	}
	fmt.Println("native close guard acceptance passed", *mode)
}
