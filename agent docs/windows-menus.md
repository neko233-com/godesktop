# Windows menu input contract

The user's 2026-10-07 correction prioritizes Windows workbench fidelity and actual
File/extension interactions. Mac development and the unpublished IME prototype
are deferred. The prototype's exact tracked patch and two new source files are
preserved under ignored `.cache/ime-wip-20261007/`; it is not published or active.

The previous Windows backend emitted InputCancelled synchronously from every
ordinary ReleaseCapture after PointerReleased. A click could open a popup and
immediately cancel it. The backend now suppresses only that known synchronous
normal release; stolen capture and actual focus loss retain cancellation.
WM_MOUSEMOVE reports unpressed hover as well as dragging. WM_SYSKEYDOWN/UP reach
the input handler, while Alt+F4 still follows DefWindowProc/WM_CLOSE and the
application's existing dirty-close guard. Custom Alt mnemonic characters do not
produce a system beep. These are Windows changes; no Mac input parity is claimed.

Element.FocusRing(false) lets a custom workbench supply its own focus/selection
style. It retains keyboard focus and pointer capture. Default elements keep the
old indicator. The application uses it for source-based menu/button styling.

TestNativeWin32MenuEvents builds a race-enabled owned native process. Actual HWND
press/release activates a button without cancellation; an unpressed WM_MOUSEMOVE,
WM_SYSKEYDOWN and a separate WM_CAPTURECHANGED each reach their required event.
Three repeats pass locally. Complete GOWORK=off Windows strict-cgo, shuffled
three-repeat race tests pass (root 132.003 seconds), including native D3D12,
input/close, bitmap/text/editor/Node/process checks. The application candidate
also passes real native shell File Open/Save As/cancel and UTF-16/emoji disk saves
against this checkout. That workspace-linked result is not independent public
module proof. Exact source CI and immutable core v0.14.0 remain pending.

Final immutable v0.14.0 is 08c8e355110b8e7241411e36781c2a1919d81eb5. All five
jobs pass in https://github.com/neko233-com/godesktop/actions/runs/37624236198,
including both Windows native events/strict-cgo/race/GPU/PE/recovery and existing
Mac/Ubuntu suites. No assertions/deadlines are relaxed. Public application module
acceptance and promotion are recorded separately in its Windows workbench record.

Initial source d0c82c3/CI 37623080158 passes both Windows full strict-cgo/race/
native/PE checks (root 96.8%), new event harness, Intel and Ubuntu. Both Windows
console counter smokes expose an older callback-count assumption: unpressed
hover may request a layout before the first GPU submission, so view callback 2
quits with only one native frame. The counter now waits for two actual completed
submissions and explicitly invalidates while waiting; the assertion/deadline is
retained. ARM's existing repeated Metal recovery gate reports submitted/completed
4, recoveries 0 and times out; logs/artifacts are retained, cause unestablished.
No Mac feature change or relaxed native assertion is made for the Windows task.
