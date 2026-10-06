# Verified status

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
