package main

import (
	"fmt"
	"math"
	"os"
	"runtime"
	"time"

	ui "github.com/neko233-com/godesktop"
)

func main() {
	verified := false
	failure := ""
	var cellWidth, cellHeight float32
	font := "Consolas"
	if runtime.GOOS == "darwin" {
		font = "Menlo"
	}
	watchdog := time.AfterFunc(10*time.Second, func() { os.Exit(2) })
	defer watchdog.Stop()
	err := ui.Run(ui.WindowOptions{Title: "godesktop native text metrics", Width: 420, Height: 140}, func(cx *ui.Context) *ui.Element {
		if cellWidth == 0 {
			cellWidth, cellHeight = ui.MeasureText("M", 14, font)
			runWidth, _ := ui.MeasureText("MMMM", 14, font)
			if cellWidth <= 0 || cellHeight <= 0 || math.Abs(float64(runWidth-4*cellWidth)) > .1 {
				failure = fmt.Sprintf("invalid grid metrics %f %f %f", cellWidth, cellHeight, runWidth)
				cx.Quit()
			}
		}
		if cx.RenderedFrames() >= 2 {
			bounds, ok := cx.ElementBounds("metrics-run")
			if !ok || math.Abs(float64(bounds.Width-4*cellWidth)) > .1 {
				failure = fmt.Sprintf("layout differs from measured run: %+v %f", bounds, cellWidth)
			} else {
				verified = true
			}
			cx.Quit()
		} else {
			cx.Invalidate()
		}
		return ui.Column(ui.Text("MMMM").FontFamily(font).FontSize(14).Width(cellWidth * 4).Key("metrics-run").OnClick(func(*ui.Context) {})).Padding(20)
	})
	if err != nil || !verified || failure != "" {
		fmt.Fprintln(os.Stderr, "native text metrics failed", err, failure)
		os.Exit(1)
	}
	fmt.Printf("native text metrics passed: %s cell %.4f x %.4f\n", font, cellWidth, cellHeight)
}
