//go:build (windows || darwin) && cgo

package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"time"

	ui "github.com/neko233-com/godesktop"
	"github.com/neko233-com/godesktop/internal/platform"
)

const title = "godesktop native viewport acceptance"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	output := flag.String("output", ".cache/viewport-native", "owned PNG/JSON directory")
	flag.Parse()
	if err := os.Setenv("GODESKTOP_READBACK", "1"); err != nil {
		return err
	}
	if err := os.MkdirAll(*output, 0755); err != nil {
		return err
	}
	watchdog := time.AfterFunc(30*time.Second, func() { fmt.Fprintln(os.Stderr, "native viewport timed out"); os.Exit(2) })
	defer watchdog.Stop()
	phase := 0
	nextFrame := uint64(6)
	offset := float32(0)
	selected := ""
	released := false
	var input ui.InputEvent
	var failure error
	var cx *ui.Context
	err := ui.Run(ui.WindowOptions{Title: title, Width: 350, Height: 220, Background: ui.RGB(0x101020), Input: func(_ *ui.Context, e ui.InputEvent) bool {
		if e.Kind == ui.Scroll {
			input = e
			offset = min(60, max(0, offset+e.X*20))
			phase = 2
			nextFrame = platform.RendererStats().Completed + 4
			return true
		}
		if e.Kind == ui.KeyReleased && e.Key == 17 {
			released = true
			return true
		}
		return false
	}}, func(view *ui.Context) *ui.Element {
		cx = view
		if platform.RendererStats().Completed >= nextFrame {
			if phase == 0 {
				failure = checkPixels(*output, "initial", false)
				phase = 1
				injectWheel(view, func(err error) {
					if err != nil {
						failure = err
						view.Quit()
					}
				})
			} else if phase == 2 {
				if math.Abs(float64(input.X-3)) > .01 || input.Y != 0 || math.Abs(float64(input.PointerX-30)) > .75 || math.Abs(float64(input.PointerY-30)) > .75 || input.Modifiers&ui.ModifierShift == 0 {
					failure = fmt.Errorf("native wheel delta/position/modifiers: %+v", input)
				}
				if _, ok := view.ElementBounds("red"); ok {
					failure = errors.New("offscreen tile retained its target")
				}
				failure = errors.Join(failure, checkPixels(*output, "scrolled", true))
				phase = 3
				injectClick(view, func(err error) {
					if err != nil {
						failure = err
						view.Quit()
					} else {
						phase = 4
						nextFrame = platform.RendererStats().Completed + 4
					}
				})
			} else if phase == 4 {
				if selected != "green" {
					failure = fmt.Errorf("translated hit selected %q instead of green", selected)
				}
				phase = 5
				injectRelease(view, func(err error) {
					if err != nil {
						failure = err
						view.Quit()
					} else {
						phase = 6
						nextFrame = platform.RendererStats().Completed + 3
					}
				})
			} else if phase == 6 {
				if !released {
					failure = errors.New("native Control release was not delivered")
				}
				phase = 7
				view.Quit()
			}
		}
		if failure != nil {
			view.Quit()
		}
		view.Invalidate()
		tiles := []*ui.Element{}
		for _, tile := range []struct {
			name string
			rgb  uint32
		}{{"red", 0xff0000}, {"green", 0x00ff00}, {"blue", 0x0000ff}} {
			tiles = append(tiles, ui.Column().Width(60).Height(60).Background(ui.RGB(tile.rgb)).Key(tile.name).OnClick(func(*ui.Context) { selected = tile.name }))
		}
		return ui.Column(ui.Viewport(ui.Row(tiles...).Height(60)).Width(120).Height(60).Key("viewport").ScrollOffset(offset, 0), ui.Text("Native viewport / wheel / clipped hit")).Padding(20)
	})
	if err != nil {
		return err
	}
	if failure != nil {
		return failure
	}
	if phase != 7 || cx == nil {
		return errors.New("viewport native phases incomplete")
	}
	f, err := os.Create(filepath.Join(*output, "report.json"))
	if err != nil {
		return err
	}
	defer f.Close()
	err = json.NewEncoder(f).Encode(struct {
		Input    ui.InputEvent
		Selected string
		Released bool
		Stats    platform.RenderStats
	}{input, selected, released, platform.RendererStats()})
	if err == nil {
		fmt.Println("Native wheel deltas/client position/modifiers, GPU viewport clipping and translated hit target passed")
	}
	return err
}

func checkPixels(directory, stage string, scrolled bool) error {
	pixels, scale, err := capture()
	if err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(directory, stage+".png"))
	if err != nil {
		return err
	}
	err = png.Encode(f, pixels)
	err = errors.Join(err, f.Close())
	if err != nil {
		return err
	}
	want := []color.RGBA{{255, 0, 0, 255}, {0, 255, 0, 255}, {16, 16, 32, 255}}
	if scrolled {
		want[0], want[1] = color.RGBA{0, 255, 0, 255}, color.RGBA{0, 0, 255, 255}
	}
	for i, x := range []int{30, 100, 150} {
		if got := pixels.RGBAAt(int(float64(x)*scale), int(30*scale)); got != want[i] {
			return fmt.Errorf("%s GPU pixel %d=%v want %v", stage, x, got, want[i])
		}
	}
	return nil
}
