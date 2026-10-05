// Package godesktop provides a declarative, native desktop UI for Go.
//
// Call Run from main. Views are rebuilt only when invalidated; application state
// and event handlers stay in Go, while a small native bridge owns the window,
// text shaping, and GPU rendering. All geometry uses device-independent pixels.
// The API is experimental and currently supports one window per process.
package godesktop
