# Validation gates

1. Unit/property/protocol tests prove coordinate, transaction, version, bounds,
   cancellation and cleanup behavior. Race checks cover asynchronous boundaries.
2. Windows amd64 uses GOAMD64=v1, strict cgo, owned HWND messages/GPU pixels,
   PE/system-DLL checks and native clipboard only on disposable CI runners.
3. macOS Intel/ARM builds native code and verifies real Metal/AppKit submissions,
   pixels, resource lifetime and same-window recovery.
   Color glyph acceptance compares intrinsic RGB with whole-line CoreText,
   preserves ordinary tint, checks opacity/clipping/one mixed batch, forces RGBA
   eviction and verifies color after recovery/restart at normal/1.5/2 density.
4. gocode verifies the published module with GOWORK=off and its own native window.
   Delayed opener/scan acceptance holds worker latency, then uses real file bytes,
   actual Node VSIX and owned Windows typing/resize/Cancel/tab messages/GPU rows.
   A passed Windows gate does not override a real Mac synchronization failure.
5. Large-file benchmarks report actual size, index/cache memory, first view,
   random/tail seeks and responsiveness. Do not substitute tiny fixture claims.
6. Installer/update tests operate on an owned disposable install root, then verify
   the user's requested local install. Verify hashes, rollback and settings safety.
7. Visual checks read only the owned application's GPU output. Pin upstream UI
   source revision and record dimensions, DPI, theme, font and meaningful states.

Native group acceptance adds shared UTF-16/CRLF edits, independent carets/scroll,
actual group focus and horizontal sash drag, nested right/down geometry, stale
held toolbar click and scoped Cancel/Save. Large split acceptance streams actual
first/tail pages, closes the original view, reads its survivor and hashes unchanged
bytes; Windows 2025 includes a real GiB. Both Mac architectures run normal/1.5/2.
Release bytes, installed GUI and exact CI/source evidence remain separate gates;
child groups.md records the current nine-group bound and incomplete layout scope.
Native VSIX acceptance adds hidden opens, actual visible/shared identities,
independent UTF-16 selection/reveal, causal focus receipts, disposed/reopened
references, native mouse focus/close and disk-acknowledged CRLF save.

Workflow changes: scripts/validate-github-actions.ps1 (actionlint/ShellCheck),
appropriate Go checks and git diff --check. Paid Copilot requests use isolated
synthetic workspaces; ordinary CI performs real process handshake without prompts.

Native bitmap acceptance captures only the owned HWND/Metal drawable. Verify
premultiplied alpha, real parent clipping, odd-row RGBA uploads, 128 simultaneous
distinct texture pixels and pinned residency, count/byte eviction, actual upload
counts, device recovery and two successive Run lifecycles. CPU residency is
bounded; fence-held in-flight GPU resources are accounted for separately.

Native viewport acceptance requires actual red/green/blue GPU clipping, removal
of offscreen targets, native positioned horizontal wheel/modifiers, translated
click and Control release. Windows uses signed negative screen coordinates on an
owned partially offscreen fixture. Mac probes construct owned native NSEvents;
normal/1.5/2 density PNG/JSON artifacts prove native handler and drawable behavior,
not every physical device. Unit/fuzz gates cover both axes, padding, clamping,
passive geometry/focus separation and stale geometry removal.

Native workspace search acceptance uses owned actual query typing, toggle/result
clicks and completed GPU highlighting. Verify unsaved overlays, Unicode UTF-16
selection, disk-stale rejection and real file-backed tail navigation without
source writes. Actual Windows 2025 GiB scan/native results are distinct from the
16 MiB console/GUI and Mac normal/1.5/2 fixtures. Use standard regexp/Git oracles
and fuzz/held-receipt cancellation/identity tests; record I/O, allocation, race
mode and hardware/run scope with timings. Public released bytes and the user's
installed GUI must pass before claiming promotion. Exact v0.13.0 evidence is in
the child search.md/status.md; PCRE2/full parity remain open.

Replacement adds real query/capture/field input, before/after GPU pixels, actual
external disk-stale rejection, pointer-down/new-preview/pointer-up refusal, real
per-file saves and blank-editor/native Undo. Worker/model gates hold receipts
and use Node JavaScript capture oracles. Public v0.8.0 prepared transactions retain
old history and reject identity/version/caret changes. Both Mac normal/1.5/2 and
Windows console/GUI pass; released bytes/rollback/user install are separate gates.
Exact v0.14.0 source/hash/evidence and global-undo/diff/regex/size gaps are in child
replace.md/status.md. No narrow native gate establishes full VS Code parity.
