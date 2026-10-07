# Native color glyphs

The v0.17.0 gocode Mac screenshots exposed a real rendering defect: CoreText
selected Apple Color Emoji, but the Metal atlas retained only alpha and tinted
it with the label foreground. Opaque face details became a solid silhouette.

The candidate fix recognizes the actual shaped run font's
[`kCTFontTraitColorGlyphs`](https://developer.apple.com/documentation/coretext/ctfontsymbolictraits/traitcolorglyphs).
It retains premultiplied RGBA for those glyphs and applies label opacity without
overwriting intrinsic RGB. Ordinary fonts retain R8 coverage and foreground tint.
CoreText still owns shaping, fallback, ZWJ, modifiers and variation selection.

Mixed glyph pages use a fixed 16-texture Metal table. Each existing 80-byte
instance carries its page slot in padding; adjacent geometry and mixed text
stay in painter order in one batch. User bitmaps use a separate texture slot,
retaining the existing 128 distinct bitmap/frame contract. Texture arrays are
an [MSL 1.2 feature](https://developer.apple.com/library/archive/documentation/Miscellaneous/Conceptual/MetalProgrammingGuide/WhatsNewiniOS10tvOS10andOSX1012/WhatsNewiniOS10tvOS10andOSX1012.html).
The Windows shader/instance ABI and public command API are unchanged.

An active glyph atlas retains at most 16 pages, 16,384 glyphs and 16 MiB of actual
pixel storage. R8 pages are 1024 square; ordinary RGBA pages are 512 square;
larger supported color glyphs use 1024-square RGBA pages counted at 4 MiB.
Accounting uses actual page bytes, including older epochs still held by GPU
submissions. Upload and peak counters use the same actual sizes. Completion
ownership of textures, staging buffers and retired epochs remains explicit.

`go run ./internal/metaltest` adds actual completed drawable PNG/JSON checks for
faces, flags, skin tones, ZWJ and variation selectors. Red and blue label colors
must produce the same intrinsic emoji RGB; half opacity must halve RGB over
black. Independent whole-line CoreText reference colors, ordinary red glyphs,
shader clipping and one mixed draw call are checked. Large font cycling forces
at least two RGBA cache evictions, with final residency at most 16 MiB and peak
in-flight residency at most 64 MiB. The same color pixels are verified after
three diagnostic resource recoveries and a subsequent Run lifecycle.

These checks run in the existing Intel and ARM Mac jobs at normal, 1.5 and 2
drawable density. Windows cannot validate AppKit/Metal locally. Candidate
f723217's ARM job passes all three densities; actual PNGs were inspected.
Reference opaque RGB matched 100%, foreground tint difference was zero and
half-opacity error at most one channel unit. Normal mixed residency is 2 MiB;
2x pressure forces two evictions with 16 MiB peak and 5 MiB final residency.
Both Windows jobs and Ubuntu also pass CI 37573206185. Intel failed the existing
10-second close-guard readiness gate twice, before the new color pixel gate.
Its timeout now logs atomic frame/renderer state; workflow_dispatch diagnosis
compares ordinary/readback close lifecycles and actual Metal pixel checks.
No timeout or assertion has been relaxed and no cause is claimed yet.
Diagnostic source e0c65ef/run 37574223528 fails both ordinary and readback
10-second close readiness. Ordinary reached three completed frames; readback
had one in-flight submission. Its pixel run could not drain Metal recovery
within five seconds. A candidate shader now uses explicit static texture-slot
dispatch and level-zero sampling, retaining one mixed painter-order batch.
Its exact Intel/ARM CI and diagnosis are pending; driver causality is unproven.
Public core promotion, independent gocode source CI and installed release
verification remain pending.

Final source 6706e59bd83b07705eed1e2d378dc954dd2a2970 passes all five jobs in
[CI 37574802922](https://github.com/neko233-com/godesktop/actions/runs/37574802922),
including both Mac architectures' full normal/1.5/2 pixel, eviction and recovery
gates. Its [Intel diagnosis 37574803095](https://github.com/neko233-com/godesktop/actions/runs/37574803095)
passes ordinary/readback close and all pixel checks without changed deadlines.
The actual Intel normal PNG was inspected: faces/details, flag, skin tone/ZWJ,
heart, half opacity, red ordinary text and clipping are correct. Opaque reference
color matching is 100%, tint difference zero and opacity error one channel unit.
The combined static-slot/explicit-LOD change resolves the observed regression;
the individual driver/compiler cause has not been isolated. Immutable public
v0.11.0 tags this exact source. App v0.18.0 public-module/native/release/install
promotion remains pending; v0.17.0 remains installed/public.

Glyph byte counters measure one atlas representation's actual page sizes;
CPU mirrors, GPU textures and per-submission staging also have their own storage
and lifetimes. They are not total process resident memory measurements.

Independent public/installed gocode v0.18.0 source 10bebad now passes all five
jobs in CI 37575737909; publication 37576841022 reuses tested packages. Both Mac
normal/1.5/2 verify actual editor and PTY color glyphs; ARM 200% terminal and
Intel 150% split PNGs were inspected. Signed automatic/direct/mirror bodies,
actual prior v0.4.0 rollback and user's installed v0.18.0 GUI/icons/Settings,
VSIX, actual GiB split/source hash and ConPTY pass with unchanged settings/PATH.
Child color-glyphs.md/status.md record exact scope, image counts and asset hashes.
Color glyph support does not establish general editor grapheme/bidi/IME,
full VS Code UI parity or official Copilot VSIX compatibility.
Published core v0.11.0 retains DirectWrite monochrome coverage on Windows.
The following candidate adds Windows color rendering; promotion is separate.

## Windows candidate

DirectWrite retains shaping, fallback and the already selected glyph indices.
Base DirectWrite color-table presence selects color handling, including
SVG/sbix/CBDT without COLR. This bounded once-per-retained-face probe avoids
SDK-dependent overload declarations. Color cache misses use TranslateColorGlyphRun and a lazy native
Direct2D/D3D11 WARP rasterizer. COLR palette layers, SVG and bitmap formats have
explicit native drawing paths. Newer systems additionally query Factory8 and
Context7 at runtime for COLRv1 paint trees. Private compatibility declarations
record the exact Microsoft SDK source revision; no guessed vtable indices or
full SDK headers are vendored. Systems without the newer interfaces use Factory4.

Native command-list bounds provide bounded per-glyph raster rectangles. RGBA
copies are premultiplied and cached; D3D12 applies label alpha without replacing
intrinsic RGB. Palette foreground entries/SVG currentColor and paint text-color
attributes use foreground-specific keys; intrinsic COLR/paint colors reuse cache
entries across label colors. Ordinary glyphs retain R8 and normal foreground tint.
The lazy rasterizer is rebuilt on actual device recovery and released at shutdown.

D3D12 now has a fixed 16-slot glyph table plus a separate bitmap binding, with
explicit nonuniform descriptor indexing/LOD. The 80-byte instance/public command ABI stays
unchanged. Mixed R8/RGBA/geometry preserves painter order in one draw; 128 distinct
user bitmap/frame residency remains separate. Active glyph pixels retain the
16 MiB/16-page/16,384-entry cap; actual live/in-flight page sizes and upload bytes
are counted. R8 uses 1024-square pages, RGBA uses 512 or bounded 1024 square.

The owned-HWND colortest checks 😀, skin-tone/ZWJ and heart/variation selection
against independent whole-layout Direct2D ENABLE_COLOR_FONT rendering, which
bypasses the atlas/translation/shader. RGB comparison allows one DIP of raster
phase, requires 95% matches within 24 channel units and separate 85% mask IoU.
Intrinsic red/blue label differences and half-opacity error are at most two units.
Ordinary red glyphs, strict clipping, one draw, real window resize, two capacity
evictions, one actual D3D12 RemoveDevice recovery and a second Run are required.
The existing Unicode fixture additionally requires actual yellow emoji pixels
and precisely two pages/2 MiB, replacing its old grayscale-only 1 MiB contract.

Local 100/150/200% candidate pixels pass; the native 150% PNG and independent
reference were inspected. Tint difference is zero, half-opacity error is one;
reference matches are 95.36/98.89/98.45%. Modern local Segoe UI Emoji uses the
COLRv1 path; its gradients/details remain visible. These are renderer-density
diagnostic HWND fixtures, not proof of physical monitor hot-plug or every font.
Windows 2022/2025 CI will retain all three densities' PNG/JSON evidence.

Exact-source full regression, five-platform CI, immutable core publication,
independent gocode/public-module/editor/ConPTY/release and installed upgrades are
pending. Dedicated SVG/bitmap/currentColor font fixtures and arbitrary third-party
font coverage remain open; enum/API paths alone do not prove those formats.

Initial source 03ed1c7/run 37581933475 passes both Mac architectures and Ubuntu,
but both Windows jobs fail existing ordinary readiness/input gates before a first
GPU submission. A local WARP+GPU-validation close gate passes. The subsequent
candidate replaces FontFace4's overloaded format probe with the stable base
TryGetFontTable API and retains each face/table classification within atlas bounds.
Opt-in bounded first-frame stage traces and a manual Windows 2022/2025 diagnostic
compare ordinary/readback and validation-off/on lifecycles with the same ten-second
readiness guard. SDK overload declarations are a suspected difference; source CI
and diagnostic evidence are pending, and no driver/ABI cause is yet established.

Diagnostic 37584001053/9d4b134 completes scene/font/upload stages: Windows 2022
passes both validation-off ordinary/readback paths, but validation-on blocks
inside the first ExecuteCommandLists until the unchanged ten-second guard fails.
Windows 2025's four ordinary diagnostic lifecycles pass; full source CI still
fails synchronous first-frame paint probes. The font-table change is not claimed
to resolve this; the trace rules out a stalled font probe in that diagnostic.

The next shader uses one explicitly NonUniformResourceIndex-marked array sample
instead of a sixteen-case sampling switch, keeping mixed painter-order batching
and GPU validation enabled. A wave may contain several instance/page indices.
Generated DXIL is renamed to gpu_shader_dx12.h: go list confirms cgo tracks it
in HFiles, whereas the previous .inc was absent and local shader-only iterations
could reuse old native objects. Local WARP/validation Unicode previously failed
its three-second synchronous paint probe in all three repeats; the newly tracked
shader passes the same three-repeat gate. Exact Windows 2022/2025 source/diagnostic
results and all color/bitmap/native/public-module promotion checks remain pending.

Source fc1a57c/CI 37585540976 now passes both Windows full repeated race/native/
coverage/PE and bitmap/recovery gates; readiness no longer stalls. Windows 2022
then fails the unchanged 95% color-reference gate at 100% (92.62%), while 2025
passes 100/150 but its 200% client is clamped by a small monitor to width 1028.
Both failed owned PNGs are retained; actual Windows 2022 COLRv0 face/details and
the independent reference were visually inspected. No source is published yet.

The subsequent candidate permits large readback-only owned fixture track sizes,
without changing physical monitor/system DPI. Color cache keys retain a bounded
1/64-pixel baseline phase, native layers draw directly at that phase after a
command-list bounds prepass, and GPU glyph quads use integer physical placement.
This avoids an extra filtering pass through a phase-zero colored bitmap.
The independent whole-layout reference explicitly sets NO_SNAP, matching the
renderer's existing IsPixelSnappingDisabled=true contract. The 95% RGB/85% mask/
two-channel/two-eviction/recovery deadlines and assertions remain unchanged.
Actual local normal/150/200% color/reference/opacity/clip/resize/eviction/restart
passes: reference color matches are 100%, tint/half-opacity differences are zero.
Exact Windows source CI, published module and gocode promotion remain pending.

## Published Windows color milestone

Public v0.12.0 tags immutable c42b4b43f0a27452937850871681f26746e39d7f after
all five jobs pass in [CI 37587610645](https://github.com/neko233-com/godesktop/actions/runs/37587610645).
Both Windows 2022/2025 normal/150/200% RGB and mask matches are 100%; tint
difference is zero and half-opacity error at most one channel unit. Their actual
2022 normal and 2025 200% PNGs were inspected: legacy COLRv0 and modern COLRv1
faces/details/skin-tone/ZWJ/heart, opacity, red ordinary text and strict clipping
are correct. Fixed fixture dimensions retain the intended 640/960/1280-pixel
client widths on small CI monitors. Normal mixed residency is 2 MiB/one draw.
Both Windows full race/strict-cgo/native/bitmap/PE/WARP/recovery and both Mac/
Ubuntu gates pass. Mac ARM's first 150% viewport timeout passed same-source retry;
there is no weakened gate or established hardware cause. Local final full
three-repeat validation is 3d68d6baff4d4a75b41980086a5c54f0 (root 96.7%).
Independent gocode v0.19.0 public-module/source/package/release/install validation
now passes at immutable 4616656 in all-five CI 37590651597/publication 37592393163.
Actual Windows 2022/2025 terminal and shared split PNGs show intrinsic colors;
native real decoded ANSI emoji gates retain resize/interrupt/exit/source checks.
The first application's `??` output regression is fixed by owned PowerShell UTF-8
startup, with real OEM-437/ASCII PowerShell 7/5.1 tests. Signed direct/mirror bodies,
old-source GUI rollback and installed actual GiB/native/VSIX/terminal/settings/
PATH checks pass. Mac three-density gates remain required. Child color-glyphs.md
and status.md retain exact PNG counts, hashes and the unchanged Linux retry scope.
Dedicated SVG/bitmap/currentColor font fixtures and the previously recorded full
editor/VS Code/GPUI/official Copilot VSIX limits remain open.
