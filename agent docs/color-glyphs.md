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
Color glyph support does not establish general editor grapheme/bidi/IME,
full VS Code UI parity or official Copilot VSIX compatibility.
Windows currently retains DirectWrite monochrome glyph coverage; native Windows
color-font rendering is a separate unimplemented rendering contract.
