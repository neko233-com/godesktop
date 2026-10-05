package main

import (
	"flag"
	"fmt"
	"log"

	ui "github.com/neko233-com/godesktop"
	"github.com/neko233-com/godesktop/internal/platform"
)

func main() {
	smoke := flag.Bool("smoke", false, "draw two frames, dispatch a state update, and exit")
	flag.Parse()
	count, frames := 0, 0
	err := ui.Run(ui.WindowOptions{Title: "godesktop · Go 1.27", Width: 900, Height: 600}, func(cx *ui.Context) *ui.Element {
		frames++
		if *smoke {
			if frames == 1 {
				cx.Dispatch(func() { count++ })
			}
			if frames == 2 {
				if count != 1 {
					panic("dispatched state update was lost")
				}
				cx.Quit()
			}
		}
		return ui.Column(
			ui.Text("godesktop").FontSize(42),
			ui.Text("Native desktop UI, written in Go.").FontSize(20).Foreground(ui.RGB(0x9ca3af)),
			ui.Column(
				ui.Text("COUNTER").FontSize(14).Foreground(ui.RGB(0x93c5fd)),
				ui.Text(fmt.Sprintf("%d", count)).FontSize(64),
				ui.Row(
					ui.Button("− Decrease", func(*ui.Context) { count-- }).Key("decrease").Grow(1),
					ui.Button("+ Increase", func(*ui.Context) { count++ }).Key("increase").Grow(1),
					ui.Button("Reset", func(*ui.Context) { count = 0 }).Key("reset").Background(ui.RGB(0x374151)),
				).Gap(12).Height(52),
			).Padding(28).Gap(16).Radius(16).Background(ui.RGB(0x1f2937)).Grow(1),
			ui.Text("Tab / Shift+Tab to focus · Enter / Space to activate · Windows + macOS").FontSize(14).Foreground(ui.RGB(0x9ca3af)),
		).Padding(36).Gap(24)
	})
	if err != nil {
		log.Fatal(err)
	}
	if *smoke {
		if frames < 2 || platform.RenderedFrames() < 2 {
			log.Fatal("native backend exited before rendering two frames")
		}
		fmt.Printf("native smoke passed: %d view frames, %d native submissions, count=%d\n", frames, platform.RenderedFrames(), count)
	}
}
