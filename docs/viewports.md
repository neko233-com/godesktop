# Native viewports and positioned scroll input

`Viewport(content).ScrollOffset(x, y)` displays one content tree inside the
available padded rectangle. Content keeps its intrinsic size, at least filling
the viewport. Positive logical offsets move content left/up; layout clamps to its
actual excess size. Paint and click/focus targets inherit the same viewport clip.
This is a native geometry operation using the existing D3D12/Metal clipping path.

The application owns offsets and scroll policy. For long lists it should build
only visible items plus leading/trailing extent spacers; Viewport does not build
or virtualize an application collection automatically. Scrollbars and reveal-on-
selection are application state, rather than hidden framework state.

```go
ui.Viewport(ui.Row(items...).Width(contentWidth)).
    ScrollOffset(scrollX, 0).Key("tabs").Height(35).Flex(1)
```

Every visible explicit `Key` is now available through `Context.ElementBounds`,
including passive containers and labels. Bounds are the current clipped client
rectangle. Hidden keys disappear after layout; visible keys must be unique.
Passive keys do not enter Tab focus order. Interactive keys retain stable pointer
capture identity and automatically generated paths remain available as before.

`InputEvent` keeps Scroll.X/Y for deltas and adds Scroll.PointerX/PointerY for
the event's client position in DIP. Positive X scrolls right/later; positive Y
scrolls up/earlier. Native modifiers and fractional line deltas are preserved.
Applications can route a wheel to the pane under its pointer, without confusing
delta values with coordinates. Existing vertical delta semantics are unchanged.

`KeyReleased` reports native key-up and modifier release, including Control.
Applications can end a held-key navigation gesture on release or InputCancelled.
Mac PageUp/PageDown are translated to the same public key codes as Windows.

Windows handles WM_MOUSEWHEEL/WM_MOUSEHWHEEL, converts signed physical screen
coordinates through ScreenToClient and DPI, and refreshes pending layout before
delivery. macOS uses the event's local view position and precise/line deltas,
with AppKit's horizontal direction normalized to the public contract.

`internal/viewporttest` checks real GPU red/green/blue clipping, positioned wheel
deltas/modifiers, removal of offscreen hit targets and clicking translated content.
Windows constructs owned HWND messages at a partly negative screen position.
Mac testing/metalprobe constructs owned native NSEvent wheel/pointer events with
readback enabled; it never posts global input or moves the user's pointer. Its
diagnostic functions require the UI thread. Mac normal/1.5/2 drawable tests retain
PNG/JSON evidence in native CI. These probes do not prove every physical trackpad.
