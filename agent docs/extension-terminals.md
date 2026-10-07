# Native extension terminal contract

The optional `clientCapabilities.terminals` bridge implements process-backed
`window.createTerminal`, `window.terminals`, `window.activeTerminal`, lifecycle/
active/state events and stable Terminal objects. A legacy native client without
this capability rejects creation explicitly. Node owns API objects and an ordered
queue; gocode owns native views, ConPTY/PTY processes and immutable cell frames.

Creation overloads accept name/path/arguments or TerminalOptions. Supported fields
are name, shellPath, shellArgs, cwd (string/file Uri), env (including null deletion),
strictEnv, hideFromUser, message, isTransient and Panel location. Arrays are normal
argument vectors; raw string arguments are Windows-only. The creation-options copy
is retained independently of caller mutation. processId is one stable Promise
resolved from a real native PID, or undefined after failed/cancelled startup.
sendText defaults to execution; show defaults to focus and supports preserveFocus.
hide retains the process. dispose is idempotent; closed references reject new input.

Native methods `window/createTerminal` and `window/terminalAction` return full
monotonic snapshots. `syncTerminals` uses the acknowledged bounded document/editor
FIFO. Entire snapshots are validated before object/event changes; stale receipts
cannot restore old focus. Open/close/state/active events retain object identity.
Actual natural process exit, extension disposal and native user close carry reason
Process=2, Extension=4 and User=3. Failed startup carries Unknown=0 and no fake PID.

Bounds: eight native/pending terminals, 32 closure records, 128 queued JS operations
and 1 MiB queued input; each input <=65,534 UTF-8 bytes, options <=64 KiB, arguments
<=128, environment overrides <=256. Names are <=256 bytes; paths/cwd/message/array
arguments/env values <=8,192 bytes; raw Windows arguments <=32,768 bytes. Process
input/output/history and UI shutdown retain the child's terminal.md contracts.

Actual Node protocol tests cover ordered actions, copied options, stable objects,
stale snapshots, atomic rejection, queue recovery/input limits, failed-start PID
resolution, clear missing-capability failure and owned runtime cleanup. gocode's
`-terminal-vsix-smoke` additionally installs a real VSIX and checks actual native
process/cwd/strict environment/Unicode input, ANSI and intrinsic emoji GPU pixels,
show/hide/focus, real exit 7, actual native close and process cleanup with unchanged
editor/disk source. This distinction separates protocol tests from process proof.

Local Windows full core strict-cgo/three-repeat race/vet/fuzz/native/PE/GUI checks
pass (root coverage 96.7%, 7d21aef810cc4137a58c7b9be4f61f1a), alongside three-repeat
protocol/process tests and the new native gate. Cross-platform exact-source
application promotion passes at gocode v0.20.0/05ae59a with GOWORK=off/no replace.
All five app source jobs in CI 37602674156 pass first; publication 37604774438
reuses those packages. Windows console/GUI and both Mac normal/1.5/2 actual VSIX
terminal gates pass. Both Windows, ARM 200%, Intel 150% and installed 150% native
PNG frames were inspected; selected source artifacts use ZIP range/CRC checks.
Signed automatic/direct GitHub/manual ghfast full release bytes, all released
native modes and real old GUI rollback pass. User v0.19→v0.20 updates through the
original stable launcher. Installed GUI/icons/Settings/real VSIX terminals/actual
GiB/source/config/PATH/shortcuts pass; Copilot SDK/LSP/auth=true, no AI prompt.
Core v0.13.0 is
published at immutable 965678a4ed3f1828155038b0c340a7ceda0832e6 after all five
jobs in CI 37599839423 pass. ARM's first existing bitmap timeout after the grid
passes unchanged same-source retry; logs/PNGs remain and no cause is claimed.
Public and installed gocode v0.20.0 is the verified application baseline.

Pseudoterminal, editor/split locations, terminal icons/color, shell integration,
profiles/link providers, environment-variable collections, session persistence and
shutdown-event delivery are not implemented. isTransient is recorded; terminal
sessions currently do not persist. State.shell and shellIntegration are undefined.
This milestone does not establish full terminal or official Copilot VSIX parity.

Reference: official vscode.d.ts at VS Code main
3f07e1aba32acacb8b08ae91bfdc954b580ad1fd (2026-10-07), stable 1.140.0.
