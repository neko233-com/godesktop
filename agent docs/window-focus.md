# Native window activation

2026-10-08 diagnostics-only source 21feee2de0b05e7424dd6297d6e818638df45bfc
in CI 37662570658 attempt 1 passes Windows 2022/2025, ARM and Ubuntu. ARM
includes the complete bitmap -recovery and 1.5/2-density checks; that successful
run does not establish the earlier intermittent timeout's cause. Intel
job 112933540457 fails in setup-go before any test: DNS ENOTFOUND for
raw.githubusercontent.com, then fallback go.dev. Its code/native checks are
skipped and no acceptance artifacts are produced. The unchanged source's
Intel job alone is retried as attempt 2/job 112938588709 and passes all race,
vet, native, Metal bitmap recovery and fractional/double-density checks. Run
37662570658 then completes successfully with all five platforms green on the
same source. First-attempt failure is retained in .cache/ci-37662570658-failed.log;
final exact results and successful Intel log are in
.cache/ci-37662570658-attempt2-results.json and
.cache/ci-37662570658-intel-attempt2.log. This diagnostic commit is not tagged;
subsequent minimized-dispatch source requires separate exact-source CI and
independent application validation.

The Windows-first application needs VS Code's onWindowChange Auto Save behavior.
InputCancelled could not prove window deactivation: keyboard focus loss and
mouse capture cancellation use the same cancellation event. They remain separate
from the new WindowFocusChanged (10) event with InputEvent.Focused.

Windows reads LOWORD(WM_ACTIVATE): WA_ACTIVE/WA_CLICKACTIVE become focused=true,
WA_INACTIVE becomes false, repeated identical states are suppressed. Minimized
state in the high word is not the activation enum. DefWindowProc keeps its normal
USER32 behavior. WM_KILLFOCUS/WM_CAPTURECHANGED retain existing cancellation.
The C callback/scene/GPU ABI does not change: the existing key scalar carries
0/1 for this event. The public callback runs on the UI thread and may precede
the first rendered frame. Focus is window state, not a document change.

Reference: https://learn.microsoft.com/en-us/windows/win32/inputdev/wm-activate.
The existing AppKit key-window delegate reports aligned become/resign-key state,
deduplicates repeats and preserves Cancel. No Mac product feature expansion is
claimed; exact-source native Mac regression is required before publication.

Isolated native tests allow WM_ACTIVATE through existing fixed WM_COPYDATA replay;
unwrapped physical activation remains excluded in opted-in fixtures. Production
input is unchanged by this test transport. Only owned PID/HWND messages are used,
without global desktop input or screen capture.

2026-10-08: independent GOWORK=off strict-cgo/race mapping/cancellation tests pass
three repeats (1.481s), no-cgo repeats pass (0.251s), vet passes. Actual native
focus acceptance passes three race/strict-cgo repeats (6.880s): five effective
activation changes, repeat/CLICKACTIVE/high-word handling, separate keyboard/
capture cancellation, raw-message isolation and callback thread identity. The
fixture reuses .cache/window-focus-native/window-focus.exe and a 20-second owned
process guard. Full independent Windows script passes three shuffled strict-cgo/
race repeats (root 145.240s), all existing native/PE/console+GUI checks and fuzz;
merged root coverage remains 94.0%. Log: .cache/windows-focus-final.log.
Exact-source CI/public v0.16 promotion remain pending. Published/installed gocode
v0.22.0 continues to use immutable public core v0.15.0.

Source ab6f0c47ad7582e13bca7ba77721c61963465726 / CI 37658957528 passes both
Windows jobs, Intel and Ubuntu. ARM's ordinary bitmap check passes, but the
existing bitmap -recovery fixture reaches the unchanged 40-second timeout.
Artifact 11500540562 has run-0-reuse.png with six correct GPU samples and
Completed >=12 at that capture, but no later grid/recovery report. Exact stall
phase/recoveries/cause cannot be inferred. The fixture has no Input callback;
activation has no handler there and original key-window frame requests remain.
Correlation is not proof that focus caused it. Incomplete source is not tagged.
Bitmap acceptance now records bounded atomic phase/stage/UI-stats snapshots
plus one independent native-stats sample per second. Existing watchdog/phase/
pixel/recovery guards remain; timeout writes stderr and owned timeout.json.
Sampler shutdown is explicit. Three race/strict-cgo diagnostics tests pass
(1.167s); actual Windows bitmap/recovery and new exact-source CI follow.
Logs/artifacts reuse ignored .cache/ci-37658957528-arm-*.
