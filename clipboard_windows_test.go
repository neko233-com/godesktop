//go:build windows && cgo

package godesktop

import (
	"os"
	"strings"
	"testing"
)

func TestNativeClipboardRoundTrip(t *testing.T) {
	if os.Getenv("GODESKTOP_CLIPBOARD_TEST") != "1" {
		t.Skip("clipboard mutation is restricted to disposable CI runners")
	}
	const fixture = "godesktop 你好 😀\r\nsecond line"
	verified := false
	err := Run(WindowOptions{Title: "godesktop clipboard acceptance", Width: 320, Height: 240}, func(cx *Context) *Element {
		if !verified {
			if err := WriteClipboard(fixture); err != nil {
				t.Error(err)
				cx.Quit()
				return nil
			}
			value, err := ReadClipboard()
			if err != nil || value != fixture {
				t.Errorf("Unicode clipboard %q: %v", value, err)
			} else {
				verified = true
			}
			if err := WriteClipboard(""); err != nil {
				t.Error(err)
			}
			cx.Quit()
		}
		return Text("Clipboard fixture")
	})
	if err != nil || !verified {
		t.Fatalf("clipboard native run: verified=%v err=%v", verified, err)
	}
}

func TestClipboardRejectsInvalidTextBeforeNativeMutation(t *testing.T) {
	for _, value := range []string{"a\x00b", string([]byte{0xff}), strings.Repeat("x", (16<<20)+1)} {
		if err := WriteClipboard(value); err == nil {
			t.Fatal("invalid clipboard payload accepted")
		}
	}
}
