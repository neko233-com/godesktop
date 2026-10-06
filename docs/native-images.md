# Native GPU images

`NewBitmap(image.Image)` copies decoded pixels into an immutable, premultiplied
RGBA asset. It supports nonzero image origins and subimage strides. Decode and
construct on an application worker or before `Run`; retain the resulting bitmap
in application state rather than rebuilding it in each view callback.

```go
decoded, err := png.Decode(bytes.NewReader(embeddedPNG))
if err != nil { return err }
logo, err := ui.NewBitmap(decoded)
if err != nil { return err }
// Inside the view callback, reusing logo:
return ui.Image(logo).Width(35).Height(35).Padding(9.5)
```

`Image` preserves aspect ratio, centers within its padded available space and
inherits rectangular clipping. White foreground preserves source colors;
`Foreground` multiplies color and opacity. It can use the same `Key`/`OnClick`
input identity as other elements. Nil, invisible and fully clipped images do not
submit image resources. Size and padding use device-independent coordinates;
bitmap dimensions are source pixels.

Each source is limited to 4096 pixels per axis / 64 MiB. The native CPU residency
cache is bounded to 128 distinct bitmaps / 64 MiB, with the current frame pinned.
A frame exceeding either limit fails explicitly. The application owns its Go
assets and must bound its own collection separately. Native copies retain no Go
pointers. Static textures upload once while resident; evicted assets and device
recovery upload again. Closing `Run` clears the native cache.

Windows uses private D3D12 RGBA textures and aligned upload footprints; macOS
uses private Metal RGBA textures and aligned staging buffers. Text remains on
the separate R8 glyph atlas. Completed-frame fences/handlers release staging and
textures; up to three in-flight frames can retain resources beyond CPU residency.
The 64 MiB limit describes the native CPU cache, not a hard total GPU-memory cap.

`RenderStats` exposes `BitmapCacheEntries`, `BitmapCacheBytes`, `BitmapUploads`
and `BitmapUploadedBytes`. Upload counters measure actual cumulative GPU uploads,
including eviction and recovery, separately from glyph uploads.

`go run ./internal/bitmaptest` validates owned GPU output for color/alpha/crop,
immutable source copies, odd row strides, static reuse, 128 simultaneous distinct
textures, pinned-frame cache eviction, count/byte limits, reupload and two `Run`
lifecycles. `-recovery` adds real Windows device removal or completion-triggered
Mac view recovery. Reports and PNGs are uploaded by the applicable native CI jobs.

Independent macOS applications can use `testing/metalprobe.Snapshot()` after
enabling `GODESKTOP_READBACK=1`. It copies only this process's latest completed
Metal drawable and submission number; it does not capture the desktop or require
screen-recording permission. Windows acceptance uses `testing/winprobe` with the
application's PID and title.
