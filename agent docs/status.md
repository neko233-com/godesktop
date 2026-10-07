# Verified status

2026-10-07 Windows shader/cache correction candidate: diagnosis 37584001053 at
9d4b134 shows Windows 2022 validation-off ordinary/readback pass; validation-on
stops inside first ExecuteCommandLists. Windows 2025 ordinary diagnostic passes,
but full 37583999968 still fails first-frame probes on both Windows jobs; Mac/
Ubuntu pass. Font probing is not the diagnosed stall. A single explicitly
nonuniform glyph-array sample replaces the sixteen-case shader switch, retaining
one mixed batch/validation. DXIL now uses a .h tracked by cgo HFiles; .inc was
absent and could hide local shader changes. Actual local WARP+validation Unicode
passes three repeats, where the prior cached shader failed all three identical
paint deadlines. Exact source CI/diagnosis and promotion remain pending.

2026-10-07 initial Windows color source 03ed1c7/CI 37581933475 passes Mac Intel/
ARM and Ubuntu; both Windows jobs fail preexisting readiness/input gates before
the first GPU submission. It is not published. Local WARP+GPU-validation close
passes. A candidate uses bounded once-per-face base font-table probing instead
of SDK overloaded FontFace4 declarations; first-frame traces and a manual two-OS
WARP validation/readback matrix preserve existing readiness deadlines.
Exact-source rechecks/CI and diagnostic evidence remain pending; no cause claimed.

2026-10-07 Windows color-font candidate: DirectWrite-shaped native COLR/paint
glyphs retain premultiplied RGBA and label opacity, with ordinary R8, one mixed
D3D12 draw and actual bounded byte accounting. Owned HWND 100/150/200% reference/
color/clip/resize/two eviction/actual recovery/restart pixels pass locally.
DXIL reproducibility and actionlint/ShellCheck pass. The first full three-repeat
race run only fails the prior Unicode fixture's exact 1 MiB grayscale assertion;
it now requires precise 2 MiB/two pages/one draw plus actual yellow emoji pixels.
Final local full strict-cgo/three-repeat race/vet/fuzz/PE/console/GUI pass; merged
root coverage is 96.7% (289661ba13fc4d79a191c9b616cc10c9). Native color at all
three densities, bitmap reuse/128-entry and byte eviction/actual recovery,
hardware/WARP DXIL ownership/pixels and regular glyph stress/recovery pass.
Final actual font-format detection passes three-repeat Unicode integration and
all density color checks. No-cgo fallback also passes. Source CI/public-module/
release/install remain pending.
color-glyphs.md records format scope and remaining SVG/bitmap/font coverage.

2026-10-07 gocode v0.18.0 promotion: immutable source/package
10bebade1495f234d8bf52cba72491ad598421c9 independently imports public core v0.11.0
with GOWORK=off/no replace and passes all five jobs in
[CI 37575737909](https://github.com/neko233-com/gocode/actions/runs/37575737909).
Both Mac normal/1.5/2 gates prove intrinsic editor/PTY emoji plus actual
command/string/ANSI terminal pixels. Actual ARM 200% terminal and Intel 150%
split PNGs were inspected; both 200% terminal reports have 717 color glyph pixels.
Complete local strict-cgo/three-repeat race/vet and console/GUI regressions pass.
Publication 37576841022 reuses tested packages, tags exact source and maintains
seven free distribution files at da39222; two Python policy tests pass.

Real signed automatic/direct GitHub/manual ghfast.top bodies, released native
gates and actual old v0.4.0 GUI rollback pass. User stable 0.5.1 launcher applies
v0.17→v0.18/source 10bebad. Installed GUI/icons/Settings/groups/VSIX, actual GiB
independent split/source hash, highlighted ConPTY and unchanged config/PATH pass.
Settings shows Up to date (0.18.0), auto route/updates and gopls ready. Copilot
authenticated/LSPInitialized/SDKConnected=true; networkPromptSent=false.
Child evidence/gitlink f8c83111a63089f3d0b49895caa110b38bdc4be4 retains immutable
tested source and metadata. Child status.md records exact asset hashes, retained
Windows color-font/Mac resize/editor/API/VS Code/GPUI/official VSIX gaps.
The accepted official SDK/Language Server route and full parity goal stay active.

2026-10-07 public core v0.11.0: immutable source
6706e59bd83b07705eed1e2d378dc954dd2a2970 passes all five jobs in
[CI 37574802922](https://github.com/neko233-com/godesktop/actions/runs/37574802922)
and Intel diagnosis 37574803095. Native color, mixed one-draw batching, opacity,
clip, bounded RGBA eviction and recovery/restart pass on both Mac architectures
at normal/1.5/2. Actual Intel normal and ARM normal/200% PNGs were inspected.
Static texture-slot/explicit-LOD sampling resolves the earlier Intel regression
without relaxed close/readiness/drain gates; individual driver cause is unisolated.
Local strict-cgo/three-repeat race/vet, Windows native close, actionlint/ShellCheck
and diff checks pass. Workflow lint now includes the manual diagnostic workflow.
Independent gocode v0.18.0 public dependency/native/release/install is pending;
public/installed app remains v0.17.0. See color-glyphs.md for scope/evidence.

2026-10-07 candidate color-glyph fix: actual Mac v0.17.0 screenshots exposed
alpha-only emoji tinting. Metal now retains intrinsic premultiplied RGBA for
color fonts, batches mixed glyph pages in one draw and counts actual storage
within the existing 16 MiB/16-page cache. New owned drawable color/opacity/clip,
independent CoreText, RGBA eviction and recovery checks are in color-glyphs.md.
Local strict-cgo/three-repeat Windows race checks pass. Mac source CI and
application/release promotion are pending; v0.10.0/v0.17.0
remain the independently tested public/installed releases below.
Candidate f723217's CI 37573206185 passes both Windows jobs, Ubuntu and ARM Mac
including normal/1.5/2 color, eviction/recovery pixels. Actual ARM PNGs were
inspected: exact intrinsic RGB, correct opacity/clipping and one mixed draw.
Intel's existing close guard timed out twice before the new pixel gate. A
diagnostic workflow compares ordinary/readback drawable lifecycles and records
atomic timeout state without relaxing the 10-second readiness requirement.
Diagnosis 37574223528/e0c65ef also fails readback readiness and pixel recovery
drain. Static texture-slot dispatch/level-zero sampling is the next candidate;
it retains the mixed batching/atlas contracts and has no verified Intel result yet.

2026-10-07 native extension-editor release: core v0.10.0 adds per-view identity,
shared documents, visible/column/range/selection events, optional native open/show/
selection/reveal, disposal/reopened-instance guards and bounded ordered operations.
Process-owned source/config files remove Windows' command-line length limit and
clean up on exit; fixed reads reject GiB input before allocation. PositionFromRunes
short queries no longer copy a whole line. Three-repeat actual Node/editor races
pass, including >32 KiB configuration and zero-object 8 MiB short-position queries.
Released gocode model and actual owned VSIX/UTF-16/CRLF/save/pixels plus opener/editor
regressions pass at 100%/150% density. A causal receipt fixes the newly observed
delayed-hidden-open focus regression. See extension-editors.md for bounds and verified
independent public-module/native/source/package/release/install evidence. Core
v0.10.0 is published at immutable 8da1ad62268bc5ef1de58ec8703fa288fcd86250 after
all five jobs in [CI 37567374583](https://github.com/neko233-com/godesktop/actions/runs/37567374583)
pass. Public/installed app is v0.17.0/source aa9d838; full parity stays active.

Full local Windows framework strict-cgo/three-repeat race/native/fuzz/PE/system-DLL/
GUI checks pass; merged root coverage is 96.7% (92f1a84a47ba4c9981e97a56d6608e17).
Additional three-repeat actual Node races prove two concurrent commands opening
the same shared TextDocument carry distinct invocation-local receipts. Vet/lint/
diff checks pass. Exact five-platform source CI and immutable v0.10.0 promotion
passed; gocode public-module/source/package/install promotion also passed.

gocode v0.17.0 source aa9d838b8b59bd82d0ce0fe52ffb2225213708c9 passes all five jobs
in [CI 37570381921](https://github.com/neko233-com/gocode/actions/runs/37570381921).
Actual Node/native views share documents with independent selection/reveal and
stable/disposed/reopened identities; hidden opens retain causal focus receipts.
Public-module strict-cgo/three-repeat race/vet and console/GUI gates pass, including
the 1024×728/96-DPI fixture (model 31.3%, search 81.4%). Windows 2025/installed GUI
prove actual GiB independent pages, original-view close/survivor and source hash.
Both Mac architectures pass normal/1.5/2. Windows 100%, ARM 200%, Intel 150% and
installed 150% screenshots were inspected; Mac emoji remains a solid fallback.
The caption-ink pixel wait is corrected; the first ARM 200% Undo timeout did not
recur in two later source runs, with unchanged gates/key-history traces retained.

[Publication 37571358158](https://github.com/neko233-com/gocode/actions/runs/37571358158)
tags exact source and reuses tested packages. Seven metadata files at eb0de72 pass
two policy tests. Real signed automatic/direct GitHub/manual ghfast.top bodies,
all released native gates and actual old v0.4.0 GUI rollback pass. User v0.16→v0.17
uses the stable 0.5.1 launcher; installed GUI/icons/Settings/groups/VSIX/actual GiB/
highlighted terminal pass. Settings shows Up to date (0.17.0), auto route/updates
and gopls ready. Configuration/PATH hashes and stable shortcuts remain unchanged.
Official Copilot authenticated/LSP/SDK=true without a prompt. Child status.md
records immutable hashes and the unfinished full API/rendering/layout/VS Code/
GPUI/official Copilot VSIX scope.
Child documentation/gitlink 33c2b6ad988b8c59b4128760f0fd432f8e5695c7 retains the
exact tested source aa9d838 and metadata eb0de72; published tags remain immutable.

2026-10-07 gocode v0.16.0 promotion: immutable application/package source
38f7d15533361ffc0b7a6e0c9b74d0dc21839962 passed all five jobs in
[CI 37561829837](https://github.com/neko233-com/gocode/actions/runs/37561829837),
independently importing public core v0.9.0/cb8b077 with GOWORK=off/no replace.
Native right/down groups share canonical text/history/service identity and keep
independent caret/scroll/tabs/pages. Group close protects shared dirty resources;
held toolbar actions and background opens retain group review/origin identity.
Actual shared-index GiB first/tail pixels, original-view close, survivor read and
whole-file SHA256 pass on Windows 2025 and the installed GUI. Full local three-repeat
race/strict-cgo/vet/console/GUI and all-five native/protocol/installer/package gates
pass; both Mac architectures run normal/1.5/2. Windows 100%, ARM 200%, Intel 150%
and installed 150% captures were inspected. Mac emoji's solid monochrome appearance
remains a framework rendering gap. Separate actual GiB race search measured
16.7425142 s / 811,864 new Go allocation bytes / 2,147,492,044 I/O bytes in that run.

[Publication 37562709833](https://github.com/neko233-com/gocode/actions/runs/37562709833)
reuses tested artifacts and tags exact source 38f7d15. Seven metadata files at
5664fde pass two policy tests. Actual signed automatic gh-proxy.com, separate
GitHub and manual ghfast.top ZIP bodies pass integrity; all released native gates
and actual old v0.4.0/cfcd3513 GUI rollback pass. User v0.15→v0.16 updates through
the existing stable launcher via direct GitHub. Installed GUI/both icons/Settings,
native groups/actual GiB split and highlighted terminal pass. ConPTY is verified;
Settings shows Up to date (0.16.0), automatic route and gopls ready. Update-config
and user-PATH hashes match before/after; both mirrors and desktop/nested Start Menu
stable targets remain, with MSI/launcher baseline 0.5.1. Copilot authenticated/
LSP/SDK=true, networkPromptSent=false. Child documentation-only evidence/gitlink
9aeedac5caf724cd937293c1283bc82caa4954bf retains exact tested source/metadata.
Limits, hashes and incomplete persistence/VSIX groups/docking/multi-window,
production/GPUI/VS Code/official Copilot VSIX scope remain in child groups.md/status.md.

2026-10-07 gocode v0.15.0 promotion: immutable application/package source
9ba89081f8aea463853c58a6c0955a9a91c950af passed all five jobs in
[CI 37557043933](https://github.com/neko233-com/gocode/actions/runs/37557043933),
independently importing public core v0.9.0/cb8b077 with GOWORK=off/no replace.
Native grouped workspace Undo/Redo prepares real history transitions on a bounded
dedicated worker. Complete identity/version/caret/stack/expiry preflight precedes
all buffer commits and callbacks; confirmation supports All/current-file/Cancel.
Newer/closed/reopened members split safely; undo never writes disk. Three-repeat
race, full local public-module strict-cgo/vet/console/GUI and all-five native/
protocol/installer gates pass. The first Windows 100% glyph predicate was corrected
after exact owned reproduction; a missing-label overlay still fails with zero ink.
Final Windows 100%, Mac ARM 200%, Intel 150% and installed 150% PNGs were reviewed.
Actual Windows 2025 GiB race search: 32.7752619 s / 748,960 new Go allocation bytes /
2,147,492,044 I/O bytes; this is a scoped measurement. Native GiB and existing
VSIX/recovered-gopls/watch/opener/terminal/search/tabs/MSI/package checks pass.

[Publication 37558228545](https://github.com/neko233-com/gocode/actions/runs/37558228545)
reuses tested packages and tags exact source 9ba8908. Seven metadata files at
35bc1bd pass two policy tests. Real signed automatic/direct/manual ghfast.top
archive bodies, released native gates and actual v0.4.0 native rollback pass.
User update v0.14→v0.15 selects source 9ba8908; baseline MSI/launcher stays 0.5.1.
Installed GUI/icons/Settings/grouped history/search/tabs/highlighted terminal and
official ConPTY pass; Auto=true/mode=auto, both mirrors, stable shortcuts and
normalized user PATH are retained. Actual Settings shows Up to date (0.15.0) and
gopls ready. Copilot authenticated/LSP/SDK=true, networkPromptSent=false. The first
terminal fixture omitted GPU readback opt-in; corrected invocation passes. Public
child gitlink advances to evidence 3afdc1d0b262d9f563936ffdbe216c9d341a6948.
Exact hashes, limits, source and remaining closed-resource/provider history and
production/GPUI/VS Code/official Copilot VSIX gaps are in child history.md/status.md.

2026-10-07 framework v0.9.0 promotion: immutable source
cb8b0777c87c4abe0d6f75b5bbf014069a42874b passed all five jobs in
[CI 37555224866](https://github.com/neko233-com/godesktop/actions/runs/37555224866)
(Windows 2022/2025 amd64, Mac Intel/ARM and Ubuntu), and the release tag resolves
to that source. HistorySnapshot/PrepareUndo/PrepareRedo preserve real stack/save/
revision semantics and reject stale ownership/version/caret/history. Full local
strict-cgo/race/native/fuzz/PE/GUI passes; root coverage 96.7%. Actual local 8 MiB
history commit allocated 224 new Go bytes. gocode now independently downloads
public v0.9.0 with GOWORK=off/no replace; source/package/native/install promotion
for grouped workspace history remains pending. Installed app is still v0.14.0.

2026-10-07 prepared history candidate: HistorySnapshot freezes top stack entries
by value, independently of ordinary Snapshot. PrepareUndo/PrepareRedo use private
worker buffers; CommitPrepared preserves saved revision, original stack entries,
monotonic version and the next revision sequence. Direct history/CRLF/UTF-16/save/
branch oracles, empty/stale identity/version/caret and frozen-history race tests
pass locally. Actual 8 MiB history commit allocates 224 new Go bytes in this scoped
run (32 KiB guard). Full local Windows strict-cgo/race/vet/native/fuzz/PE/GUI pass;
merged root coverage remains 96.7%. Five-platform public CI is pending. gocode global
workspace history is a candidate using an ignored local modfile; published/user
versions remain core v0.8.0 / app v0.14.0 until promotion is verified.

2026-10-07 gocode v0.14.0 promotion: immutable source/package
de2a98fb0a041cd88c0ec5ac658bb2fd41a62f9b passed all five jobs in
[CI 37552538844](https://github.com/neko233-com/gocode/actions/runs/37552538844),
independently importing public godesktop v0.8.0 with GOWORK=off/no replace.
Native workspace replacement prepares complete snapshots off UI, preflights all
targets, commits the buffer batch and uses existing per-file saves. Earlier
unsaved text/history/EOL survive, stale query/edit/caret/reopen/disk and changed
captured reviews reject, late save conflicts retain dirty undoable source. The
real blank-editor focus bug found by native Undo is fixed. Local full strict-cgo/
race/vet/console/GUI and new Node oracle/held-receipt tests pass. Mac Intel/ARM
normal/1.5/2 actual events/Metal pixels pass. Windows 100% preview, ARM 200%
changed-review and Intel 150% Undo PNGs were visually reviewed. Windows 2025 actual
GiB race search remains green (32.0390007 s / 748,656 new Go allocation bytes in
this scoped run); native GiB and existing tabs/VSIX/editor/gopls/watch/opener/
terminal/MSI/package gates stay green.

[Publication 37553566847](https://github.com/neko233-com/gocode/actions/runs/37553566847)
reuses tested packages and tags v0.14.0 at de2a98f. Metadata 7749adf changes seven
known files and passes policy tests. Real signed automatic/direct/manual ghfast.top
ZIP bodies, released new/existing native gates and actual original v0.4.0 GUI
rollback pass. The user's actual v0.13.0 updater selected 0.14.0/source de2a98f;
stable MSI/launcher remains 0.5.1. Installed GUI/icons/Settings, replacement/
search/tabs/caption/highlighted terminal, embedded ConPTY and update checks pass.
Actual 1920×1230 / 150% preview/Settings PNGs were visually reviewed: Up to date
(0.14.0), automatic route and gopls ready. Auto=true/mode=auto, both mirrors,
desktop/nested Start Menu stable targets and normalized user PATH are preserved.
Copilot authenticated/LSP/SDK=true; no AI prompt was sent. Public child gitlink
advances to evidence c5af3fba393d33d7bb140de0e6973ab28f0a56a3. Per-file disk saves
are not filesystem-wide atomicity; full global undo/individual/diff/JS/PCRE2 and
production/GPUI/VS Code/official Copilot VSIX parity remain active. Exact limits,
package hashes and source evidence are in child replace.md/status.md.

2026-10-07 framework v0.8.0 promotion: immutable source
d5d139dd365733e494589df8d471e2346c7a1729 passed all five jobs in
[CI 37549368977](https://github.com/neko233-com/godesktop/actions/runs/37549368977)
(Windows 2022/2025 amd64, Mac Intel/ARM, Ubuntu) and is tagged/published as v0.8.0.
Snapshot.Prepare / CanCommit / CommitPrepared retain buffer identity, monotonic
version, saved revision and older undo/redo history. Local full strict-cgo/race/
native/fuzz/PE/GUI passed with merged root coverage 96.7%; actual 8 MiB worker
preparation/commit and concurrent newer source tests pass. Local measured commit
allocates 64 Go bytes, a scoped measurement rather than a universal guarantee.
gocode candidate independently downloads/imports public v0.8.0 with GOWORK=off
and no replace; replacement planner/JavaScript capture oracle/three-repeat held
receipt race tests pass. Native replacement/source/package promotion is pending;
the user's installed/released app remains v0.13.0 with its original v0.7.0 core.

2026-10-07 prepared transaction candidate: Snapshot.Prepare computes private
UTF-16/EOL/selection/undo/protocol state on a cancellable worker. UI CanCommit
checks identity/version/selection; CommitPrepared adopts text without scanning
the source and retains earlier history/save points. Empty/unowned/stale plans,
overlap/NUL/surrogate errors and same-version replacement buffers are rejected.
Three-repeat race oracle/history/worker tests pass. Actual 8 MiB source commit
allocates 64 new Go bytes locally; preparation races with newer live edits and
cannot commit afterward. Full local strict-cgo/race/vet/native/fuzz/PE/GUI gates
pass; merged root coverage is 96.7%. Public/cross-platform application evidence
is pending.
This is a foundation for actual native workspace replace, which is still open.

2026-10-07 gocode v0.13.0 search promotion: immutable source/package
`d4a869e24c9acc2d93cd9aefb17f9303064423b2` passed all five jobs in
[CI 37547147426](https://github.com/neko233-com/gocode/actions/runs/37547147426),
independently using public godesktop v0.7.0 with GOWORK=off. Native content search
covers unsaved snapshots, Unicode folding, case/word/Go regex, ignore/include/
exclude, grouped virtual rows and verified UTF-16/file-backed navigation. Actual
Windows 2025 1,073,741,824-byte single-line race scan reported 16.8320796 s /
811,736 new Go allocation bytes, distinct from the local 1.53 s / 360,624-byte run.
Native actual GiB tail navigation preserves source hashes. Mac Intel/ARM normal/
1.5/2 native event/drawable gates pass; character diagnostics keep punctuation
separate from virtual-key codes. Windows 100% results/GiB, ARM 150% Unicode and
Intel 200% large-navigation PNGs were visually inspected. Existing five-platform
tabs/VSIX/editor/opener/watch/recovered-LSP/terminal/MSI/package gates stay green.

[Publication 37548090734](https://github.com/neko233-com/gocode/actions/runs/37548090734)
reuses the exact tested packages; tag v0.13.0 is verified at d4a869e. Metadata
c708e34 changes seven known files and passes Python policy tests. CI ZIP/MSI bodies
match published digests. Actual signed automatic/direct/manual ghfast.top ZIP
integrity, released native search/existing gates and original v0.4.0 native GUI
rollback with shared extensions pass. The user's real v0.12.0 updater selected
0.13.0/source d4a869e; stable MSI/launcher baseline remains 0.5.1. Installed
GUI/icons/Settings, search/tabs/captions, highlighted ConPTY and update checks
pass. Actual 1920×1230 / 150% selected-Unicode/Settings PNGs were visually reviewed;
Settings reports Up to date (0.13.0), auto route and gopls ready. Auto=true/mode=auto,
both mirrors, desktop/nested Start Menu stable launcher targets and normalized
user PATH are preserved. Copilot authenticated/LSP/SDK=true; no AI prompt was sent.
Public child gitlink advances to evidence ae0770bd0f50dcbb8ec8c5ebdf38889edab23156.
Full production/GPUI/VS Code/official Copilot VSIX parity remains active; workspace
replace and other documented gaps remain open. See child search.md/status.md for
precise bounds, package hashes and scope. No new framework version is required.

2026-10-07 framework v0.7.0 milestone: native Viewport/ScrollOffset, passive keyed
geometry, positioned horizontal/vertical wheel and key-release events are added.
Windows actual viewport pixels, clipped translated click, wheel coordinates/
modifiers at a negative physical screen origin and native Control release pass.
The final full strict-cgo/race/native/fuzz/PE suite passed (root coverage 96.7%).
The initial full check correctly rejected changed duplicate-key wording in its
existing panic gate. That assertion now uses the generalized element-key
contract, retaining actual panic/close checks. Mac owned native-event/drawable
normal/1.5/2 gates now pass.
The first two candidate runs exposed Mac diagnostic constant names and the fact
that unposted CGEvents lack an AppKit window attachment. The final 5e7789b bridge
uses actual Quartz-to-owned-window/view conversion and the shared native wheel
delivery method. Mac Intel/ARM now pass their complete native/glyph/bitmap/
viewport/recovery suites in 37540201901; normal/1.5/2 reports contain delta X=3,
pointer (30,30), Shift, translated green click and Control release. ARM 200% and
Intel 150% scrolled GPU PNGs were visually inspected. All five jobs in
[CI 37540201901](https://github.com/neko233-com/godesktop/actions/runs/37540201901)
passed Windows 2022/2025 amd64, Mac Intel/ARM and Ubuntu. Immutable tag v0.7.0
is verified at 5e7789bca48f69ddd144d2e391f3bee225f60286 and published.
gocode v0.12.0/source def207c289953b51fe87ab33d28a69e1b7dab3ab independently
consumes public v0.7.0 with GOWORK=off. All five jobs in CI 37541346744 pass native
40-tab clipping/wheel/drag/identity/ordered/MRU/release gates, existing VSIX,
opener/file-watch/recovered-gopls/terminal checks and Windows 2025 actual GiB/MSI
lifecycle. Mac normal/1.5/2 captures pass; source Windows 100%, ARM 150% stable
release and Intel 200% second-MRU PNGs were visually inspected. Publication
37542564399 reused tested packages. Metadata c9a4244 changes seven known channel
files and passes Python policy gates. Real automatic/direct/manual ghfast.top ZIP
integrity, released native gates and original v0.4.0 GUI rollback pass.

The user's v0.11.0 updater selected 0.12.0/source def207c via direct GitHub. Stable
baseline remains 0.5.1. Actual installed GUI/icons/Settings/source preservation,
40-tab/caption/ConPTY/update checks pass; 1920×1230 / 150% Settings/last-tab PNGs
were visually inspected. Auto=true/mode=auto, both mirrors, shortcuts and user
PATH remain valid. Official Copilot authenticated/LSP/SDK=true with no prompt.
The public child gitlink advances to evidence 3f9347b6e623069c3587a0cfdaa96d29c1f99519.
Full GPUI/VS Code/official Copilot VSIX production parity remains active.

2026-10-07 framework v0.6.0 milestone: immutable Bitmap/Image API and native
D3D12/Metal RGBA texture caching are implemented. Local Windows strict-cgo race,
fuzz, native input/PE/GUI and workflow/DXIL checks passed (merged root coverage
96.8% before the final offscreen branch). New native bitmap checks passed real
alpha/crop pixels, 128 simultaneous distinct textures, frame pinning, bounded
count/byte eviction, reupload and repeated Run cleanup. Normal uploads=152;
actual device-removal recovery uploads=153 with one recovery/dropped frame;
second Run uploads exactly once. Owned reuse/grid PNGs were visually inspected.
Immutable source 3bf3e4dfd1b4a221eb61018f201295b1ce3404f7 passed all five jobs in
[CI 37532765849](https://github.com/neko233-com/godesktop/actions/runs/37532765849)
and was published as v0.6.0. Windows 2022/2025, Mac Intel/ARM and Ubuntu all pass;
Mac reports match normal/recovery 152/153 uploads, one recovery and one second-Run
upload. Downloaded Mac ARM reuse/recovery and Intel 128-image PNGs were visually
inspected. Public testing/metalprobe exposes owned completed-drawable readback.
Initial c27a91d CI 37532594163 was superseded/cancelled for that public probe;
only final 3bf3e4d is the promoted source.

Child v0.11.0/source 05439efb84156feeb14b1d45f76631bd1d0673cc passed all five jobs
in 37534367695, with independent public v0.6.0 and actual Windows GPU-logo/caption/
tab checks plus Mac normal/1.5/2 drawable captures. Publication 37535500899 reused
tested packages. Metadata b6c7af4 changes seven known channel files and passed
local policy/public-byte gates; actual automatic/direct/manual ZIP SHA256 and
released-native checks plus original v0.4.0 GUI rollback passed. The user's
v0.10.0 updater selected 0.11.0/source 05439ef via direct GitHub. Stable baseline
is still 0.5.1. Installed GUI/icons/Settings/visual/tab/opener/VSIX/save/gopls/
watch/terminal/close and ConPTY checks passed; Auto=true/mode=auto is preserved.
Official Copilot authenticated/LSP/SDK=true, networkPromptSent=false. Downloaded
Windows 2022, Mac ARM 150%/closed 200% and Intel 200%, then actual installed README/
Settings pixels were visually inspected. Source references, viewport/DPI, explicit
scope gaps and no-prompt SDK-first route are maintained in the child harness.
Full production/VS Code/official Copilot VSIX parity remains active.

2026-10-07 gocode v0.10.0 promotion: immutable application/package source
`0a86cec6e6a56aa439c1474c4bfc82a047ce5122` passed all five jobs in
[CI 37527105167](https://github.com/neko233-com/gocode/actions/runs/37527105167),
including Windows 2022/2025 amd64, Mac Intel/ARM, actual GiB browsing/MSI lifecycle
and native VSIX/gopls/reload/terminal checks. [Publication 37528232380](https://github.com/neko233-com/gocode/actions/runs/37528232380)
reused these tested packages. godesktop v0.5.3 remains the public dependency;
the core implementation/tag is unchanged. Native opening and batched Explorer
traversal now run on bounded workers, preserving dirty documents, cancelled/closed
identity and newer focus. VSIX/Problems/definition navigation awaits real targets.
Node commands/providers wait for prior document/focus acknowledgement.

Initial source a335ff0 CI 37525452237 passed Windows/Ubuntu and new Mac opener
acceptance, but both real Mac VSIX editor gates failed with no active document.
This exposed the missing Node FIFO receipt barrier. That source was not published;
the corrected source passed both Mac gates. Earlier local fixed-coordinate mouse
tests also required actual final-position source-row pixels after async startup.
Exact-source Windows 2022 pending/read-completed owned-GPU PNGs were inspected.

Metadata `ba8555174b53d9e1119b615d3ebdb1d130ad327d` changes only seven known
channel/hash/install files; two Python policy tests passed. No extra metadata
full-platform CI is claimed. Actual signed automatic/direct/manual-ghfast ZIP
bytes, released native opener/four-close/VSIX/large/terminal/gopls/reload checks
and true v0.4.0/source cfcd3513 GUI rollback passed with shared extensions.
The user's real old -update command separately applied v0.10.0 via direct GitHub.
Selected payload/source is 0.10.0/0a86cec; stable MSI/launcher baseline stays 0.5.1.
Installed native checks, icon/shortcut files, normalized PATH and Settings/source
preservation passed. The inspected installed captures show the awaited README,
original dirty tab, automatic=true/automatic route, Up to date (0.10.0), gopls ready.
Copilot authenticated/LSP/SDK=true, networkPromptSent=false; no new AI prompt.
The parent pins documentation-only child evidence
`2dda7f1a0a71499a2570956c2eadedc0f8be1631`, retaining exact verified code/metadata.
Full production/VS Code/official Copilot VSIX parity remains active; root setup
still precedes ui.Run, Explorer is capped and recursive live-tree behavior is pending.

Framework v0.5.3 is published at immutable source
`d22bf93187911a2fd829c993a6b096338e9a1b64`, with all five jobs green in CI
`37513434533`. Saved disk reloads preserve buffer identity, monotonic UTF-16
protocol versions, prior undo transactions and EOL transitions. Targeted repeated
Windows race tests passed for invalid input, surrogate selection clamping, saved
undo/redo revisions, snapshot immutability and history budgets. The full framework
platform gates and public module promotion passed. Local full Windows amd64
strict-cgo/race/native/PE/fuzz validation passed (Repeat=1, FuzzSeconds=3), reports
`.cache/windows-validation/2e2d4f8115224d3d80a0aea19d3122be`; combined root native
coverage is 96.5%. gocode v0.9.0/source
`abadd6d0d53109f7445a41ee92652ada1491d7ff` independently pins this public module,
passed all five jobs in `37518877841` and publication `37520177277`. Open editable
files now have bounded parent-directory watches/reconciliation, clean versioned
reload, dirty conflict confirmation and explicit hash-checked overwrite. Windows
uses deletion-sharing reads and modern native rename with bounded retry/revalidation.
The parent pins documentation-only child evidence
`ba93e3d884020bc7ded564ce0933d38eb82ab7e6`, retaining exact verified code/metadata.

Framework v0.5.2 is published at immutable source
`ff69721c1a807d8be044786597bf33033f55d966`, all five jobs green in CI
`37488721747`. TextAdvance exposes DirectWrite's native advance and unrounded
CoreText typographic bounds for monospace grids; MeasureText retains label layout
semantics. Real native trailing-space/monospace checks gate this public module.
The child independently pins v0.5.2 with GOWORK=off and has published its real
ConPTY/PTY terminal with PowerShell/zsh input highlighting as v0.7.0 and independent
language-server recovery as v0.8.1. Promotion
and installed evidence follow below; terminal contracts/gaps are in its harness.

Initial metrics source 3abf0a6 failed both Mac CI jobs in run 37487621956:
Menlo label sizing returned 11 DIP for one M and 36 for four because MeasureText
includes rounded layout padding. Windows and portable results do not override
this negative control. The corrected TextAdvance implementation passed both real
Mac jobs plus both Windows jobs and Ubuntu; the failed source was not published.

2026-10-06 framework release v0.4.0: source
`f733ea9d6c40cad3f83cf89051f49185367bbc93`,
[five-platform CI](https://github.com/neko233-com/godesktop/actions/runs/37440861010),
[release](https://github.com/neko233-com/godesktop/releases/tag/v0.4.0).

Verified native renderer, UTF-16 document/snapshots/transactions/undo, native
selection/clipboard APIs, pointer/repeat input, Stack overlays, LSP transport and
live VSIX native edit/provider/state integration. Windows short-path URI identity
has a negative-control regression; macOS Intel/ARM and Ubuntu checks passed.

gocode's Copilot LSP/SDK, native acceptance, diagnostics and related work is
currently being integrated into its next public revision. Latest native account
acceptance passed suggestion rendering/Tab/acknowledgement/chat/cancel/retry with
GOWORK=off. This is not proof of full official Copilot VSIX compatibility.

New objective remains active: GB browsing, generic LSP, harness, free installer
channels, signed/hashed update metadata and local installation. No claim of full
production or all VS Code capabilities yet. Record further milestones here.

gocode now has local real GiB bounded-memory/native browsing and official gopls
formatting/hover/definition/unsaved completion/diagnostic-clear acceptance. See
its agent docs/status.md and docs/large-files.md for measured evidence and gaps.
Public source promotion/CI, installers, updater and requested installation remain
in progress; the framework source itself remains the released v0.4.0 revision.

gocode v0.4.0 promoted at `cfcd3513aedc4ec50ae19625fbd2f04446039abe`:
[five-platform CI](https://github.com/neko233-com/gocode/actions/runs/37451395121)
and [release](https://github.com/neko233-com/gocode/releases/tag/v0.4.0).
Both Windows runners, macOS Intel/ARM and Ubuntu passed, including real gopls,
native window/icons and Windows 2025 actual GiB/native browsing. A real Windows
8.3 alias negative-control regression now preserves the live unsaved document.

gocode distribution source `8bccd29b23cd181e074f1c8cdefc6be1ea2e8f59` passed
[all five CI jobs](https://github.com/neko233-com/gocode/actions/runs/37459941417).
This includes actual Windows MSI installation, corrupted-cabinet rollback,
upgrade/downgrade rejection, native launch, shortcuts/icons and uninstall/PATH
preservation, plus macOS Intel/ARM bundles and updater ZIPs. Real native settings
mouse/keyboard persistence and GPU capture, signed/hashed update and actual compiled
health/rollback tests passed. Managed pinned Copilot sidecar installation passed
official LSP/SDK handshakes. No full VS Code/production parity claim.

The real v0.5.0 Windows ZIP update check exposed PowerShell 5 non-canonical ZIP
paths. The updater rejected the archive and preserved its old version. v0.5.0 is
held as a prerelease; v0.5.1 adds canonical ZIP paths and actual package extraction/
native health gates. New CI/public-asset acceptance and final local installation
remain pending. The child harness records routes, ownership/rollback and free
channel contracts; root framework source still corresponds to released v0.4.0.

Next framework change in development: CloseRequested guard and RequestClose,
with explicit Quit reserved for a decided/forced shutdown. Both native OS closes
and the macOS application quit menu must defer to the UI-thread guard. This is
needed for gocode's unsaved-document confirmation; it is not yet in the published
framework dependency or installed app. Verify native reject/confirm, explicit force
and callback-panic shutdown before publishing a new framework module.

gocode v0.5.1 is published at immutable source
`e7f2d69011040b6c458cb54fc73f4cff695015b6`, with all five jobs green in CI
`37461737830` and publication `37462680852`. Real public-byte checks passed signed
update from the v0.4.0 source binary to v0.5.1, native large-file rendering,
direct GitHub/manual ghfast SHA256 integrity, and actual prior-source rollback.
The requested local per-user MSI/CLI installation is now at
`C:\Users\14170\AppData\Local\Programs\gocode`, reports the exact release source,
and passed GUI icon/settings/GPU/source-preservation checks. Auto updates are on,
mode auto, current GitHub metadata verifies and reports up-to-date. Installed
managed official Copilot SDK/LSP handshake is authenticated (no prompt), and
managed gopls v0.23.0 passed real installed native language acceptance. Root full
Windows race/native/fuzz/PE suite for the close guard passed with combined root
coverage 96.8%; Mac CI is pending.

Framework v0.5.0 at `eff4097c302ed52db98b144352b183ce058120ac` is now
published, with all five CI jobs green in run `37464670639`, including macOS
Intel/ARM close rejection, force and panic paths. The parent gitlink retains the
verified child metadata/evidence source `232dcfc5b14a1acf57518836fdbdd08a7367b434`.

Next bridge revision fixes save event semantics: a rejected native save updates
the current document state without reporting a successful save; accepted saves
use a per-document saveId to deduplicate RPC acknowledgements and notifications.
Real Node-host race tests cover success, duplicate delivery, newer unsaved state
and the next accepted save. gocode's new close/async-save source has not yet been
promoted; its first CI exposed hard-coded discard coordinates on smaller Windows
runner windows. The installed app remains the verified v0.5.1 editor release.

Local full Windows amd64/GOAMD64=v1 strict-cgo validation passed after making
the OS-close harness wait for rendered frames before sending focus keys. HWND
creation alone can precede the first focusable tree. Three repeated actual
OS-close tests passed; core coverage remains 96.8% with native coverage merge,
plus race, fuzz, PE/GUI smoke. Bridge tests also reject older duplicate saveIds.

Framework v0.5.1 is published at immutable source
`fd5ee95f0fd5df73e8076971abb282d616bd7725`, with all five jobs green in CI
`37471310727`. gocode independently pins that public module with GOWORK=off.

gocode v0.6.0 is published at application source
`242a1a917b189ded070ac00c7ac75f8eba9b6296`, all five jobs green in CI
`37474545017`, publication `37475908095`. Generated free-channel metadata
`2ff7885cd40be9b5a4a94e29d51598d760674f18` also passed all five jobs in
manual CI `37476959355`. Child evidence commit
`83f804ce101f8efc7fa2972f8029efcf73c9dcfa` changes documentation only and
retains that verified code/metadata; the parent gitlink advances to it. Its
evidence-only commit skips redundant native CI; release source gates were kept.

Real public signed ZIP/native checks exercised automatic accelerated update,
all close modes, VSIX save and large-file rendering, then rolled the owned root
back to the original v0.4.0 source and actually rendered its GUI. A shared builtin
VSIX upgrade first broke old-release startup; a real old/new/old negative control
prevented promotion. Versioned hidden builtin roots now preserve old shared stores
and actual rollback startup passed. A separate direct body download timed out;
automatic and manual gh-proxy signed SHA256 checks passed without weakening
integrity. Do not describe direct range-probe reachability as full download success.

User installation now selects v0.6.0/source 242a1a9 through the real v0.5.1
stable launcher at `C:\Users\14170\AppData\Local\Programs\gocode`, with auto
updates=true/mode=auto. Shortcuts/PATH remain verified. Installed owned-GPU GUI/
icon/settings, close save, actual VSIX Document.save/CRLF/undo/redo/completion and
real gopls native acceptance passed. Installed official Copilot LSP/SDK reports
authenticated=true/lspInitialized=true/sdkConnected=true/networkPromptSent=false.
The inspected native Settings capture reports Up to date (0.6.0), automatic route
and gopls ready. See the child's agent docs for exact evidence and remaining
editor/services/distribution/full VS Code and official Copilot VSIX gaps. The
overall production/parity objective remains active.

2026-10-07 terminal promotion: public godesktop v0.5.2/source ff69721 passed
all five jobs in 37488721747. Its TextAdvance API resolves a real Mac label-padding
negative control without changing MeasureText layout semantics. The independent
child pins this public module and publishes gocode v0.7.0/source
`8a0fa07670a3018b42ab86eb2dd45e814099c7c7`, all five source jobs green in
`37498949904`, publication `37500332017`. Windows 2022/2025 use the same official
pinned/hash-verified modern ConPTY embedded in PE resources; older system conhost
dropped alternate-screen notifications. No system component was replaced. Real
PowerShell 7/5.1 and Mac zsh input highlighting, PTY/VT/Unicode/alternate screen,
native resize/Ctrl+C/exit and bounded process/history lifecycle checks passed.
Downloaded Windows 2022 owned-GPU terminal captures were visually inspected.

Actual signed public v0.7.0 automatic and separate direct GitHub full downloads
plus manual ghfast.top SHA256 checks passed. Native large-file/VSIX/four-close/
terminal acceptance ran from those released bytes; the owned root then rolled
back to v0.4.0/source cfcd3513 and actually rendered its native GUI with the same
extension store. The user installation separately updated via its actual stable
v0.5.1 -update launcher to v0.7.0/source 8a0fa076, direct GitHub route. Its selected
payload is 0.7.0; MSI/stable-launcher baseline stays 0.5.1. Auto=true/mode=auto,
installed -version/-update-check, embedded runtime, real terminal, native GUI/
icons/Settings/source-preservation, real gopls and Copilot protocol checks passed.
Desktop/Start Menu GUI shortcuts, icon files and normalized user PATH are valid.
Installed owned-GPU Settings/input/output captures were visually inspected:
Up to date (0.7.0), automatic route, gopls ready, distinct pre-Enter syntax colors
and output truecolor after resize. Copilot authenticated/LSP/SDK=true and
networkPromptSent=false. Official Copilot VSIX and full VS Code parity remain
unfinished; the child harness retains exact bounds and capability gaps.

Generated free-channel metadata `cacee0d8b5228f63eb37748269721a266530a279`
passed all five jobs in manual CI `37500982783`, including Windows MSI lifecycle,
GiB native browsing, gopls/Copilot and Mac bundle packages. Child evidence commit
`2573e677e39d0a1f694c446c50c1ab8e3550defd` updates documentation only and
retains these exact verified code/metadata commits; the parent gitlink advances
to it. Evidence-only commits skip redundant native CI, preserving source gates.
The production/full-parity goal remains active.

2026-10-07 language recovery promotion: gocode v0.8.1 immutable application source
`9a498ab4ae866419a1a8e1c08eb49b15a3be9db5` passed all five jobs in
`37509901232`; publication `37511249130` reused its tested Windows MSI/ZIP and Mac
Intel/ARM packages. Generic servers now restart independently with four bounded
backoff retries, replay current unsaved snapshots, cancel old work and guard
session/document identity. Queues/requests/events/diagnostic text have explicit
bounds; palette Restart Language Servers resets the crash budget. Real stdio
crash/manual repair/initialize cancellation and actual native gopls recovery
tests passed. A stopped-after-crash negative control failed the regression gate.

Earlier v0.8.0 source b767b83 passed logical/native CI but its downloaded Win2022
final capture still showed the prior greet()/Output frame. It remains a prerelease,
with no artifact/tag replacement. v0.8.1 waits for actual owned GPU completed-token
and Problems glyph ink before saving that exact passing capture. The corrected
Windows 2022 source-CI capture and final installed gopls/Settings captures were
visually inspected. Same-source CI 37508299422 passed four jobs but Win2025's
official Copilot initialize hit 20 seconds; unchanged source then passed all five
in 37509901232, including the handshake, final pixels, GiB browsing and MSI gates.

Generated metadata `5b3a7f309610d5ed095fb38c4e7d8361b6b93392` changes only the
seven known channel/hash/install files; local Python hash/input-policy checks
passed. Real signed public automatic ghfast.top, separate direct GitHub and manual
ghfast.top full ZIP downloads passed SHA256. Released-byte native large-file,
four close modes, VSIX save, terminal and recovered gopls passed; owned-root
rollback then actually rendered v0.4.0/source cfcd3513 with shared extensions.
The user's stable v0.5.1 launcher separately applied v0.8.1 via direct GitHub.
Current payload/source is 0.8.1/9a498ab; MSI/launcher baseline remains 0.5.1.
Installed real recovered gopls/final pixels, highlighted terminal, GUI/icons/
Settings/source preservation, version/update metadata, shortcuts/icon files/PATH
passed. Auto=true/mode=auto is retained. Copilot authenticated/LSP/SDK=true,
networkPromptSent=false; no new AI prompt was sent. Child documentation-only
evidence `9970c5bd07dfad08fec045d1d9e13cc03743c9ac` preserves verified code and
metadata; the parent gitlink advances to it. Full production/VS Code/official
Copilot VSIX parity and broader language/process supervision remain unfinished.

2026-10-07 editable-file promotion: child v0.9.0/source abadd6d passed all five
jobs in 37518877841, including native VSIX/gopls external reload, language recovery,
Windows 2025 GiB/MSI and Mac packages. Publication 37520177277 reused those exact
tested artifacts. Windows 2022 source-CI conflict/dialog/final owned-GPU captures
and installed final reload/Settings captures were visually inspected.

Original child 6001f54 failed Windows 2022 actual-gopls/atomic rename with Access
denied (37515729600), while the other four jobs passed. It was not published.
Deletion sharing alone still failed a held-descriptor legacy rename test. The
corrected FileRenameInfoEx/POSIX path retains old descriptors, respects other read
sharing conflicts and retries with hash verification/cancellation. Long Unicode
path, external writes during retry, readonly files and ten repeated races passed.
An exact-size native filename buffer exposed missing NUL storage and was corrected.
Source 0ad8fec's standalone BSD root notice then failed actual archive extraction
(also Mac ARM in superseded 37518524702). Appending the notice to existing
LICENSE.txt preserved old-updater layout; final source abadd6d passed all gates.

Generated metadata 626671a changes only seven known channel/checksum/install files
and passed local two Python tests. Signed public v0.9.0 automatic direct GitHub,
separate direct full download and manual ghfast.top full bytes passed SHA256;
released-byte native large browsing/four close/VSIX/terminal/recovered-gopls/
file-watch/gopls passed. The owned root then actually rendered the original
v0.4.0/source cfcd3513 after rollback with shared extensions.

The user's existing stable launcher updated via actual -update/direct GitHub to
selected 0.9.0/source abadd6d. MSI/launcher baseline stays 0.5.1. Installed version/
metadata, ConPTY, file watch, gopls recovery, highlighted terminal, save, GUI/icon/
Settings/source preservation, shortcuts/icon files and normalized PATH passed.
Auto=true/mode=auto remains; Settings shows Up to date (0.9.0). Copilot official
SDK/LSP handshake is authenticated, networkPromptSent=false. The final child
evidence ba93e3d updates documentation only; the gitlink advances to it. Full
production/VS Code/official Copilot VSIX parity remains active, including recursive
workspace watch, async ordinary open/traversal, huge-browser reindex and diff/merge.
