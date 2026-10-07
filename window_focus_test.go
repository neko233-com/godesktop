package godesktop

import (
	"testing"

	"github.com/neko233-com/godesktop/internal/platform"
)

func TestWindowFocusEventMappingAndCancellationRemainSeparate(t *testing.T) {
	cx := &Context{wake: func() {}}
	var received []InputEvent
	a := application{context: cx, pressed: "pressed", hovered: "hovered", focused: "keyboard", input: func(_ *Context, e InputEvent) bool {
		received = append(received, e)
		return false
	}}
	a.handle(platform.Event{Kind: platform.WindowFocus, Key: 1})
	a.handle(platform.Event{Kind: platform.WindowFocus, Key: 0})
	if len(received) != 2 || received[0].Kind != WindowFocusChanged || !received[0].Focused || received[1].Focused {
		t.Fatalf("native activation mapping: %+v", received)
	}
	if a.pressed != "pressed" || a.hovered != "hovered" || a.focused != "keyboard" {
		t.Fatal("window activation synthesized pointer cancellation or element keyboard focus")
	}
	a.handle(platform.Event{Kind: platform.Cancel, Key: 1})
	if received[2].Kind != InputCancelled || received[2].Focused || a.pressed != "" || a.hovered != "" || a.focused != "keyboard" {
		t.Fatal("ordinary cancellation changed its contract or reported native window focus")
	}
	a.handle(platform.Event{Kind: 9, Key: 1})
	if received[3].Kind != KeyReleased || received[3].Focused {
		t.Fatal("key release was misinterpreted as a window activation")
	}
}
