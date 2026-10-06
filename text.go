package godesktop

import (
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/neko233-com/godesktop/internal/platform"
)

// MeasureText returns the platform-shaped single-line size in logical pixels.
// Call on the UI thread; font should match the displayed Text element.
func MeasureText(text string, size float32, font string) (float32, float32) {
	return platform.MeasureTextWithFont(text, max(1, nonnegative(size)), font)
}

// TextAdvance returns the native single-line typographic advance in DIP, without
// layout padding or pixel rounding. Use it for monospace grids/caret positions;
// MeasureText still returns the label's full layout size. Call on the UI thread.
func TextAdvance(text string, size float32, font string) float32 {
	return platform.TextAdvance(text, max(1, nonnegative(size)), font)
}

// ReadClipboard reads Unicode text on the UI thread in response to a user paste.
func ReadClipboard() (string, error) { return platform.ReadClipboard() }

// WriteClipboard replaces the native clipboard with Unicode text on the UI thread.
func WriteClipboard(text string) error {
	if !utf8.ValidString(text) || strings.ContainsRune(text, 0) {
		return errors.New("clipboard text must be UTF-8 without NUL")
	}
	if len(text) > 16<<20 {
		return errors.New("clipboard text exceeds 16 MiB")
	}
	return platform.WriteClipboard(text)
}
