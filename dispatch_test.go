package godesktop

import (
	"testing"

	"github.com/neko233-com/godesktop/internal/platform"
)

func TestUIWakeDrainsBoundedWorkWithoutViewOrInput(t *testing.T) {
	wakes, callbacks, views, inputs := 0, 0, 0, 0
	cx := &Context{wake: func() { wakes++ }, width: 800, height: 600, bounds: map[string]Bounds{"retained": {1, 2, 3, 4}}}
	a := application{context: cx, view: func(*Context) *Element { views++; return nil }, input: func(*Context, InputEvent) bool { inputs++; return true }}
	for i := 0; i < maxPendingDispatches; i++ {
		if !cx.Dispatch(func() { callbacks++ }) {
			t.Fatalf("rejected callback %d before queue limit", i)
		}
	}
	if cx.Dispatch(func() { t.Fatal("rejected callback executed") }) || cx.Dispatch(nil) {
		t.Fatal("accepted an overflowing or nil callback")
	}
	for i := 0; i < 100; i++ {
		cx.Invalidate()
	}
	if wakes != 1 {
		t.Fatalf("outstanding native wakes=%d, want one", wakes)
	}
	a.handle(platform.Event{Kind: platform.UIWake})
	if callbacks != maxDispatchBatch || len(cx.pending) != maxPendingDispatches-maxDispatchBatch || wakes != 2 {
		t.Fatalf("first UI work turn callbacks=%d pending=%d wakes=%d", callbacks, len(cx.pending), wakes)
	}
	for len(cx.pending) != 0 {
		a.handle(platform.Event{Kind: platform.UIWake})
	}
	if callbacks != maxPendingDispatches || views != 0 || inputs != 0 {
		t.Fatalf("worker callbacks=%d views=%d inputs=%d", callbacks, views, inputs)
	}
	if w, h := cx.WindowSize(); w != 800 || h != 600 {
		t.Fatal("non-render wake changed viewport")
	}
	if b, ok := cx.ElementBounds("retained"); !ok || b != (Bounds{1, 2, 3, 4}) {
		t.Fatal("non-render wake changed retained geometry")
	}
	if !cx.Dispatch(func() {
		callbacks++
		if !cx.Dispatch(func() { callbacks += 10 }) {
			t.Fatal("reentrant callback was rejected")
		}
	}) {
		t.Fatal("drained queue did not reclaim capacity")
	}
	a.handle(platform.Event{Kind: platform.UIWake})
	if callbacks != maxPendingDispatches+1 {
		t.Fatal("reentrant callback ran recursively in the same UI turn")
	}
	a.handle(platform.Event{Kind: platform.UIWake})
	if callbacks != maxPendingDispatches+11 || len(cx.pending) != 0 || views != 0 || inputs != 0 {
		t.Fatal("reentrant callback did not complete on a later non-render wake")
	}
}
