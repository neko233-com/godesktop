# UI dispatch without an available drawable

2026-10-08 source audit finds an actual backend defect: Context.Dispatch only
drains inside the Draw event. Windows wake_message requests a frame but the
event loop refuses draws for hidden/minimized windows; AppKit requestFrame has
the same visibility guard. A worker receipt, including an Auto Save timer or
completed write, can consequently wait until the user restores the window.
The existing synthetic activation acceptance does not prove minimized behavior.

The candidate sends a private UIWake=11 through the existing native callback ABI.
Windows handles it in the owned HWND wake message; macOS handles it on the main
queue under its existing Run-generation check. It drains UI callbacks without
Input, View, layout or Present, then requests a normal coalesced frame. The native
visibility/drawable guards still prevent rendering while minimized or hidden.
Draw retains a bounded drain for initial/fallback progress.

Context accepts at most 1024 pending callbacks plus at most 64 in the executing
batch, rejects nil/shutdown/overflow,
and coalesces Dispatch/Invalidate into one outstanding wake. A UI turn executes
at most 64 callbacks and requeues the remainder; reentrant callbacks execute in
a later turn. Windows processes at most 64 native messages before polling GPU
completions and considering visible rendering. Callbacks must remain short;
file/process/network work stays on cancellable workers. Callers must handle a
false Dispatch without waiting for an acknowledgement that cannot arrive.

Three portable and three strict-cgo/race repeats (1.084s) verify actual queue
rejection/reclamation, coalesced
invalidations, bounded batches, no Input/View/geometry changes and deferred
reentrancy. Three owned minimized/hidden Windows native race repeats pass
(12.399s): each actual state runs 128 parent and 128 reentrant callbacks on the
UI thread; views/submitted/completed stay fixed for 150ms, restoration produces
actual green GPU pixels, shutdown completes from minimized UI callbacks, and a
second Run rejects old Context operations. No global input or workspace is used.
Actual gocode minimized Auto Save now passes six private native runs in three
strict-cgo/race repeats (25.572s): both window-change and delayed disk/clean/save
event receipts complete without new View/GPU submissions, followed by verified
restoration and idempotent settings/temp/report checks. Corrected full independent
Windows script passes three shuffled strict-cgo/race repeats (root 124.098s),
vet/fuzz/native/PE/console+GUI gates and merged root coverage 94.6%. Actual ordinary/
recovery bitmap fixtures pass, preserving all pixels/128 assets/eviction/upload/
two-Run/recovery/watchdog assertions. Reports reuse .cache/dispatch-windows-final.log,
windows-validation/current and ui-dispatch-bitmap/{native,recovery}. Full no-cgo/
vet also passes. Shared Metal regression/source CI and
public application-module validation remain pending. Public core v0.15.0 and
installed application v0.22.0 remain the verified distribution.

First full Windows candidate run fails only the old interactive fixture shutdown
in three repeats (root 130.256s): probe replies now arrive without a drawable,
so closing immediately after a reply sees four native submissions despite five
View callbacks. The existing minimum-five-native-submissions assertion remains.
The owned fixture now defers its interactive CloseRequested until five actual
native submissions, explicitly requesting the needed frames. All input/click/
pixel/three-Run/shutdown guards remain. Final full checks are still required;
the initial failure is in .cache/dispatch-windows-first.log. FIFO acceptance now
drains two bounded turns and checks all 100 callbacks rather than a partial list.
