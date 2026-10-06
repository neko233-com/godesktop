# Validation gates

1. Unit/property/protocol tests prove coordinate, transaction, version, bounds,
   cancellation and cleanup behavior. Race checks cover asynchronous boundaries.
2. Windows amd64 uses GOAMD64=v1, strict cgo, owned HWND messages/GPU pixels,
   PE/system-DLL checks and native clipboard only on disposable CI runners.
3. macOS Intel/ARM builds native code and verifies real Metal/AppKit submissions,
   pixels, resource lifetime and same-window recovery.
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
