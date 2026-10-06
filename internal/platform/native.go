//go:build (windows || darwin) && cgo

package platform

/*
#include <stdlib.h>
#include "bridge.h"
*/
import "C"

import (
	"errors"
	"runtime"
	"unsafe"
)

var eventHandler func(Event)

//export gd_go_event
func gd_go_event(kind C.int, x, y C.float, key, modifiers C.int) {
	if eventHandler != nil {
		eventHandler(Event{int(kind), float32(x), float32(y), int(key), int(modifiers)})
	}
}

func color(c Color) C.GDColor {
	return C.GDColor{r: C.float(c.R), g: C.float(c.G), b: C.float(c.B), a: C.float(c.A)}
}
func rectangle(r Rect) C.GDRect {
	return C.GDRect{x: C.float(r.X), y: C.float(r.Y), w: C.float(r.W), h: C.float(r.H)}
}

func Run(options Options, handler func(Event)) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	eventHandler = handler
	defer func() { eventHandler = nil }()
	title := C.CString(options.Title)
	defer C.free(unsafe.Pointer(title))
	var custom C.int
	if options.CustomTitlebar {
		custom = 1
	}
	if message := C.gd_run(title, C.float(options.Width), C.float(options.Height), color(options.Background), custom); message != nil {
		return errors.New("godesktop: " + C.GoString(message))
	}
	return nil
}

// Present transfers one pointer-free command array and one UTF-8 blob per frame.
// The native backend copies both buffers synchronously and retains no Go memory.
func Present(commands []Command) {
	if len(commands) == 0 {
		C.gd_present(nil, 0, nil, 0)
		return
	}
	buffer := C.malloc(C.size_t(len(commands)) * C.size_t(C.sizeof_GDCommand))
	if buffer == nil {
		panic("native command buffer allocation failed")
	}
	defer C.free(buffer)
	native := unsafe.Slice((*C.GDCommand)(buffer), len(commands))
	var blob []byte
	for i, cmd := range commands {
		if uint64(len(blob))+uint64(len(cmd.Text))+uint64(len(cmd.FontFamily)) > uint64(^uint32(0)) {
			panic("native text buffer exceeds 4 GiB")
		}
		native[i] = C.GDCommand{kind: C.int(cmd.Kind), bounds: rectangle(cmd.Bounds), clip: rectangle(cmd.Clip), color: color(cmd.Color), radius: C.float(cmd.Radius), font_size: C.float(cmd.FontSize), text_offset: C.uint32_t(len(blob)), text_length: C.uint32_t(len(cmd.Text))}
		blob = append(blob, cmd.Text...)
		native[i].font_offset, native[i].font_length = C.uint32_t(len(blob)), C.uint32_t(len(cmd.FontFamily))
		blob = append(blob, cmd.FontFamily...)
	}
	var text unsafe.Pointer
	if len(blob) > 0 {
		text = C.CBytes(blob)
		defer C.free(text)
	}
	C.gd_present((*C.GDCommand)(buffer), C.size_t(len(commands)), (*C.char)(text), C.size_t(len(blob)))
}

func MeasureText(text string, size float32) (float32, float32) {
	return MeasureTextWithFont(text, size, "")
}

func MeasureTextWithFont(text string, size float32, font string) (float32, float32) {
	bytes := C.CString(text)
	defer C.free(unsafe.Pointer(bytes))
	fontBytes := C.CString(font)
	defer C.free(unsafe.Pointer(fontBytes))
	var width, height C.float
	C.gd_measure(bytes, C.size_t(len(text)), C.float(size), fontBytes, C.size_t(len(font)), &width, &height)
	return float32(width), float32(height)
}

func Wake() { C.gd_wake() }
func ReadClipboard() (string, error) {
	var text *C.char
	if message := C.gd_read_clipboard(&text); message != nil {
		return "", errors.New(C.GoString(message))
	}
	defer C.free(unsafe.Pointer(text))
	return C.GoString(text), nil
}
func WriteClipboard(text string) error {
	bytes := C.CString(text)
	defer C.free(unsafe.Pointer(bytes))
	if message := C.gd_write_clipboard(bytes); message != nil {
		return errors.New(C.GoString(message))
	}
	return nil
}
func Quit()                   { C.gd_quit() }
func WindowAction(action int) { C.gd_window_action(C.int(action)) }

// RenderedFrames lets the native smoke example require actual submissions.
func RenderedFrames() uint64 { return uint64(C.gd_rendered_frames()) }

func RendererStats() RenderStats {
	s := C.gd_render_stats()
	name := "unavailable"
	switch s.backend {
	case 1:
		name = "direct2d"
	case 2:
		name = "metal"
	case 3:
		name = "direct3d12"
	}
	clock := "unavailable"
	switch s.frame_clock {
	case 1:
		clock = "mtkview"
	case 2:
		clock = "cametaldisplaylink"
	case 3:
		clock = "dxgi"
	}
	return RenderStats{
		Backend: name, FrameClock: clock, FrameSlots: uint32(s.frame_slots), UsedSlotsMask: uint32(s.used_slots_mask),
		InFlight: uint32(s.in_flight), MaxInFlight: uint32(s.max_in_flight),
		Submitted: uint64(s.submitted), Completed: uint64(s.completed),
		DrawCalls: uint64(s.draw_calls), Instances: uint64(s.instances), UploadedBytes: uint64(s.uploaded_bytes),
		BufferWaits: uint64(s.buffer_waits), CPUTimeNanos: uint64(s.cpu_nanos), GPUTimeNanos: uint64(s.gpu_nanos),
		SceneTimeNanos: uint64(s.scene_nanos), AcquireTimeNanos: uint64(s.acquire_nanos), EncodeTimeNanos: uint64(s.encode_nanos),
		FrameRequests: uint64(s.frame_requests), FrameTicks: uint64(s.frame_ticks),
		CoalescedRequests: uint64(s.coalesced_requests), IdlePauses: uint64(s.idle_pauses),
		GlyphRasterizations: uint64(s.glyph_rasterizations), GlyphCacheHits: uint64(s.glyph_cache_hits), GlyphCacheEntries: uint64(s.glyph_cache_entries),
		GlyphAtlasPages: uint64(s.glyph_atlas_pages), GlyphAtlasBytes: uint64(s.glyph_atlas_bytes), GlyphAtlasPeakBytes: uint64(s.glyph_atlas_peak_bytes),
		GlyphAtlasEpochs: uint64(s.glyph_atlas_epochs), GlyphUploadedBytes: uint64(s.glyph_uploaded_bytes),
		DeviceRecoveries: uint64(s.device_recoveries), DroppedFrames: uint64(s.dropped_frames),
	}
}
