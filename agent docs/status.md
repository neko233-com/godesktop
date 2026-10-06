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

Next distribution milestone remains in progress. Free native MSI prototype
installed/launched with shortcuts/icons, and uninstalled from its remembered
custom root with PATH removal. This is disposable installer acceptance, not the
requested final local installation; update/upgrade/release channels remain pending.
