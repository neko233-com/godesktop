# Native extension editor contract

Published core v0.10.0 adds optional native editor-group synchronization. Installed
gocode v0.17.0 independently imports the public module. Reference:
the official VS Code ViewColumn/TextDocumentShowOptions/TextEditor/window APIs.
This is an implemented subset, not full official extension-host/Copilot VSIX parity.

## Wire ownership

Advertise editorGroups/openDocument/showDocument/applyEdit in clientCapabilities.
Initialize documents first, then an editors snapshot. syncEditors accepts
{generation, active, editors, selectionKind?}; each visible editor contains opaque
id, canonical path, viewColumn (1–9), UTF-16 selection and visibleRanges. One shared
TextDocument can have multiple distinct TextEditors. Hidden documents/tabs are
excluded from visibleTextEditors. A hidden/closed editor is disposed; showing it
again creates a new identity. Surviving objects retain identity on column renumber.
Complete snapshot/coordinate validation precedes mutation and event callbacks;
stale/duplicate generations cannot restore expired views.

syncDocument can carry the same editor snapshot, so callbacks observe current
source/view state. Optional decimal document instance identities are monotonic
within a native host session. Reopened paths replace the old document object;
older full/open/close receipts cannot replace or close its new instance. Go must
verify live editor/document identity and version before edits/save/selection/reveal.
Path/version alone does not identify a resource closed/reopened at the same version.

showTextDocument supports numeric/options overloads, Active/Beside/One through Nine,
preserveFocus and selection when Go implements them. Its Promise returns the actual
acknowledged view ID. openTextDocument uses the optional bounded native worker and
keeps the resource hidden. Generic clients without editorGroups fail explicitly
for unavailable group/show options. Selection/edit/reveal operations are ordered,
with 128 pending operations per editor and 128 native requests/10-second timeout.
Edit builders expire after their synchronous callback. Undo-stop merging fails
explicitly; snippets/multicursor and preview-tab lifecycle remain gaps.

Opening receipts retain focus epoch, group identity/generation and layout members.
Node AsyncLocalStorage isolates at most 128 receipts per invocation, consumed once
when that document is shown. Newer focus/expired targets reject delayed show, rather
than adopting newer focus after a hidden read. Go owns the decision; shared
TextDocuments do not store global receipt state.

## Runtime and bounds

Embedded CJS/configuration use one process-owned temporary directory, removed
after startup failure/termination. Only paths enter Node arguments, avoiding the
Windows 32 KiB command-line limit. Configuration is limited to 16 MiB and contains
workspace/extension metadata, not credentials. Fallback UTF-8 reads inspect size
first, then use a fixed 2 MiB+1 buffer and finally close the handle. Oversized,
growing, invalid UTF-8/NUL text is rejected. gocode has the same 2 MiB service limit
and does not send entire GiB files to extensions.

PositionFromRunes iterates only the requested prefix without allocating the whole
line. A real 8 MiB short query allocates zero Go objects; existing UTF-16/CRLF,
clamping/transaction/fuzz oracles retain coordinate semantics.

## Evidence and gaps

Three-repeat actual Node/VSIX races cover shared-document/distinct-editor identity,
hidden tabs, renumber/disposal, generation/coordinate rejection, reopened-instance
receipts, >32 KiB configuration, cleanup and real GiB fallback rejection. Released
gocode uses GOWORK=off/no replace; model races cover real hidden reads, focus/closed-target
receipts, all nine columns, UTF-16 inactive selection/reveal and GiB rejection
before indexing. Owned -groups-vsix-smoke installs a genuine temporary VSIX and
verifies Node↔Go UI, actual focus/close controls/completed pixels, shared UTF-16/CRLF
edits, independent reveal, events/disposal and actual acknowledged disk save with
an untouched hidden file. It passes at local 150% and process 100% density.
The new hidden open initially bypassed the old focus guard; causal receipts fix
it and the unchanged native stale-open/editor gates pass. Core source
8da1ad62268bc5ef1de58ec8703fa288fcd86250 passes all five jobs in CI 37567374583
and is published immutably as v0.10.0. gocode source aa9d838 passes exact all-five
CI 37570381921 and publication 37571358158; public v0.17.0 bytes, installed GUI/
actual GiB/VSIX/terminal, update integrity/routes/real rollback and settings/PATH
preservation pass. Windows 100%, Mac ARM 200%/Intel 150% and installed 150% PNGs
were inspected. Child status.md records exact hashes and remaining scope.

Full local strict-cgo/three-repeat race/native/fuzz/PE/system-DLL/GUI passes at
96.7% merged root coverage; additional concurrent shared-document receipt tests
and vet pass. Five-platform exact-source CI and immutable module promotion pass.

Full tabGroups/preview/pinning/docking, multicursor/IME/accessibility, options/
decorations/snippets/undo merging, SCM/DAP, remote/webview providers and official
Copilot VSIX remain unfinished. Mac emoji appearance remains a rendering gap.
