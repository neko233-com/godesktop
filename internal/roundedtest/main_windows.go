//go:build windows && cgo

package main

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

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
	os.Setenv("GODESKTOP_READBACK", "1")
	var clicks atomic.Int32
	var failure error
	started := false
	bitmap := image.NewNRGBA(image.Rect(0, 0, 12, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 12; x++ {
			bitmap.SetNRGBA(x, y, color.NRGBA{G: 255, A: 255})
		}
	}
	texture, err := ui.NewBitmap(bitmap)
	if err != nil {
		return err
	}
	watchdog := time.AfterFunc(15*time.Second, func() { fmt.Fprintln(os.Stderr, "native rounded clipping timed out"); os.Exit(2) })
	defer watchdog.Stop()
	err = ui.Run(ui.WindowOptions{Title: "godesktop owned rounded clipping", Width: 400, Height: 260, Background: ui.RGB(0x102030)}, func(cx *ui.Context) *ui.Element {
		if !started && cx.RenderedFrames() > 3 {
			started = true
			go func() {
				w, e := winprobe.Find("godesktop owned rounded clipping", uint32(os.Getpid()))
				if e == nil {
					for _, p := range [][3]int{{21, 21, 0x102030}, {40, 40, 0xff0000}, {21, 118, 0x102030}, {161, 21, 0x102030}, {180, 40, 0x00ff00}, {278, 118, 0x102030}} {
						actual, pe := w.Pixel(p[0], p[1])
						if pe != nil || actual != uint32(p[2]) {
							e = fmt.Errorf("native rounded pixel %d,%d=%06x expected %06x: %v", p[0], p[1], actual, p[2], pe)
							break
						}
					}
				}
				if e == nil {
					e = w.Pointer(0x201, 21, 21)
				}
				if e == nil {
					e = w.Pointer(0x202, 21, 21)
				}
				if e == nil && clicks.Load() != 0 {
					e = errors.New("rounded transparent corner remained clickable")
				}
				if e == nil {
					e = w.Pointer(0x201, 40, 40)
				}
				if e == nil {
					e = w.Pointer(0x202, 40, 40)
				}
				if e == nil && clicks.Load() != 1 {
					e = errors.New("rounded interior lost pointer activation")
				}
				if e == nil {
					pixels, ce := w.Capture()
					e = ce
					if e == nil {
						scale := float64(w.DPI()) / 96
						ink := 0
						for y := 150; y < 210; y++ {
							for x := 20; x < 140; x++ {
								p := pixels.RGBAAt(int(math.Round(float64(x)*scale)), int(math.Round(float64(y)*scale)))
								if p.R > 180 || p.G > 180 || p.B > 180 {
									ink++
								}
							}
						}
						if ink < 20 {
							e = errors.New("rounded clipped text/emoji has no real GPU ink")
						}
						if path := os.Getenv("GODESKTOP_ROUNDED_SCREENSHOT"); e == nil && path != "" {
							if e = os.MkdirAll(filepath.Dir(path), 0700); e == nil {
								var f *os.File
								f, e = os.Create(path)
								if e == nil {
									e = png.Encode(f, pixels)
									e = errors.Join(e, f.Close())
								}
							}
						}
					}
				}
				cx.Dispatch(func() { failure = e; cx.Quit() })
			}()
		}
		cx.Invalidate()
		return ui.Column(ui.Column().Height(20), ui.Row(ui.Column().Width(20), ui.Column(ui.Column().Flex(1).Background(ui.RGB(0xff0000)).OnClick(func(*ui.Context) { clicks.Add(1) })).Width(120).Height(100).ClipRounded(20), ui.Column().Width(20), ui.Column(ui.Image(texture).Width(120).Height(100)).Width(120).Height(100).ClipRounded(20)), ui.Column().Height(30), ui.Row(ui.Column().Width(20), ui.Column(ui.Text("█ 😀 世界").FontSize(36).Foreground(ui.RGB(0xffffff))).Width(120).Height(60).ClipRounded(20)), ui.Column().Flex(1)).Background(ui.RGB(0x102030))
	})
	if err != nil {
		return err
	}
	if failure != nil {
		return failure
	}
	fmt.Println("Native completed GPU rounded descendant rectangle/bitmap/text/emoji clipping and transparent-corner/interior pointer acceptance passed")
	return nil
}
