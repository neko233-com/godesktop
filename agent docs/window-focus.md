# Native window activation

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
