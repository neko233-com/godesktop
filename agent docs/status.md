# Verified status

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
