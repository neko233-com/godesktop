# Windows D3D12 removed-device lifetime investigation

2026-10-08 CPU-only independent review. The original forced non-debug WARP
swapchain failure's cause remains unresolved. Unpublished software-path/final-close
r3 critical checks and the later r4 Windows Shadow matrix now have real local
proof below; full default-three/Mac/exact-source CI are required, and no
OS/driver cause is established here.
Public core v0.16.0/source 5178f551a179351391af6eaab62f3f5cf150350e is unchanged.
The local renderer/shader candidate is unpublished. The initial independent
review changed no C++ or shader. Root subsequently authorized the Surface-only
candidate described below. This reviewer still started no native/GPU process
and changed no package, system DLL or registry. Root owns native execution.

## Exact observed controls and evidence

Root's first minimal C++ control creates one owned WARP device/queue and either
one 2048×2048 committed target or three matching flip-discard HWND backbuffers.
It records 32 native clear commands, closes and executes one command list,
optionally calls Present, signals fence value 1, then calls real RemoveDevice.
There is no Go, shader, atlas, extra draw, prewarm or pre-removal drain.
The watchdog is 40s; the control releases its owned interfaces and HWND.

The first control source SHA256 is
5211d180dd4a97d77af2dc91db40358fdea5d18bce81d8fe674e2e2ed48d7ffd;
EXE SHA256 is
620ec3f9f0cc31a6932b87db8367162d8a92cf2cd3e6ed827bde509de7a1903c.
Those original bytes remain at .cache/warp-minimal-control/main.cpp.first.snapshot
and warp-minimal-control.exe.first.snapshot. Exact execution records are
.cache/warp-minimal-control/native-results-first.json:

| Case | Actual result | Before-removal observation | Release evidence |
| --- | --- | --- | --- |
| offscreen | PID226728, exit0, 0.3141s | fence0 of1, one submission | queue0, target0, device0; native disposal returns |
| present | PID187920, exit -1073741819/0xC0000005, 3.1383s | fence0 of1, one submission | swapchain1, queue6, target2 then1; disposal does not return |

Both logs report debug=0, WARP vendor0x1414/device0x8c, removed reason0x887a0005
and fence18446744073709551615. present.log SHA256 is
e6d1ba34942fa2c5bfa97af8fae39f18987f6746802f530db6a147da55cc1a03;
offscreen.log SHA256 is
67942d26d4eb79d42f575a0429e836574954a10966417a999cd91b111d048851.
The original logs did not observe actual loaded module paths. The exit code is
from the owned process result, rather than inferred from the truncated stderr.
Its process absence was rechecked. The two target lines do not identify which
resource or thread faults next; they are not a complete exception stack.

Fence0 is an actual observation before removal, not an atomic assertion that
work remains pending at the exact RemoveDevice instruction: logging and a
QueryInterface occur between that read and removal. No completion/drain is
performed in that interval. Record this distinction when comparing controls.
The existing present/offscreen pair also changes target ownership/HWND/DXGI
behavior, so it does not alone isolate queued Present as the necessary cause.

Earlier .cache/shadow-warp-retire-gdb.log shows a WARP worker fault in
QueryMuxDListForApplication while the UI thread is inside
NtGdiDdDDIDestroyAllocation2 through WindowFrame::release_target and
Window::recover_surface. This proves concurrent driver work/resource destruction
for that prior binary. It does not identify the bad pointer or bind that older
stack to every newly edited header. See [shadows](shadows.md) for prior framework
and immutable-public16 control evidence.

## Actual system and developer-WARP pair

Root subsequently ran the byte-identical module-path pair. The path-control
source SHA256 is 6049d1eeef4e87fcdf3b560c5dcd1418289e8cb55db717b5c1bbcbc324622551;
both EXEs hash 6f880b204aceca82eace5909afb335b29e2e5e271822d29f097b4abdb9bf2f5a.
Actual records are .cache/warp-minimal-control/path-pair-native-results.json
and each system-path-control/app-local-path-control offscreen.log/present.log:

| Runtime / case | PID / exit / elapsed | Actual result |
| --- | --- | --- |
| system / offscreen | 200580 / 0 / 0.1113661s | disposal returns |
| system / present | 192724 / 0xC0000005 / 2.2966499s | disposal AV |
| NuGet1.0.21 / offscreen | 227236 / 0 / 0.1365822s | disposal returns |
| NuGet1.0.21 / present | 194372 / 0xC0000005 / 2.3528471s | disposal AV |

All four observe fence0/one submission before removal, debug0, no watchdog
timeout and their actual owned PID absent afterward. Both system runs actually
load C:\WINDOWS\SYSTEM32\d3d10warp.dll/10.0.26100.9278, SHA256 cd1a6f11…2b2ed52;
both app-local runs actually load the owned cache DLL/1.0.21.0.20260922.2,
SHA256 e79c1055…66517ca3. D3D12/D3D12Core remain System32/10.0.26100.9278 and
DXGI remains System32/10.0.26100.9444 with identical hashes across all four.
Thus changing WARP to the current official developer-test DLL did not remove
this present/disposal failure. It cannot be assigned solely to the older WARP
version. This does not identify which layer is faulty or prove application
correctness; D3D12/DXGI/system and ownership behavior are still shared variables.
Both actual present failures and first-control history remain preserved.

Root then completed the sample-backed backbuffer-owner ordering comparison.
One control EXE supports the original order and targets-first order, retaining
the live HWND until final destruction. Source SHA256 is
5538e64bf81e49cb2f41e89728a46975ac01c3701ef4efdbe61c8c471c4098cd;
EXE SHA256 is
5a6bff56f466caac1b3fa2488d9f8e9181bb481a434176096b97e2afbef04d56.
Actual .cache/warp-minimal-control/order-pair-native-results.json records:

| Runtime / release order | PID / elapsed | Actual result |
| --- | --- | --- |
| system / original | 185536 / 2.415306s | 0xC0000005 |
| system / targets before swapchain/queue | 176976 / 2.4574814s | 0xC0000005 |
| NuGet1.0.21 / original | 225760 / 2.5963922s | 0xC0000005 |
| NuGet1.0.21 / targets before swapchain/queue | 226576 / 2.4228514s | 0xC0000005 |

All four preserve debug0, one submission, actual fence0 observation, removed
disposal, no watchdog timeout and owned-PID absence. The module paths/hashes
are actually observed and match the prior system/developer-WARP pair. Moving
backbuffer owners before swapchain/queue owners did not repair this AV under
either WARP. Its four logs and the unsuccessful alternative order remain
preserved. This negative result does not prove another teardown order or API
layer to be the cause.

## Reviewed local renderer lifetime

The review reads these unpublished local bytes; it is not new native validation:

| File | SHA256 |
| --- | --- |
| internal/platform/dx12_device.h | 992b264453b3c367691ed90edeebe4a9086f9587a585ed0d55545c1126310bb9 |
| internal/platform/dx12_surface.h | 071d8cf1d483ec03dd50f4be93c54b966f3b598400fb62dd177e8f016ce54c49 |
| internal/platform/bridge_windows.cpp | e114de7a04f62dc3233a77c5289a31c553c98907bffcd1ba545f6dd018f5b8f5 |

Window destruction explicitly resets Surface while Scene/Atlas/bitmap caches
still exist. Recovery also resets Surface before clearing those caches and
opening a new Surface on the same live HWND. This corrects the earlier reverse
C++ member-order exposure; it does not prove removed-driver quiescence.

The pre-final-close review snapshot's Surface destruction attempts finish, releases its command list and all three
allocators, releases its swapchain owner, drops the engine queue owner, then
releases queries/RTV heap/latency handle. Member destruction next releases frame
uploads/atlas references, targets/readbacks, mapped instances and descriptors;
Device destruction follows. All frame allocators are already null. The device
and removal-event owners outlive the frames. Device's own offscreen list and
allocators are also released before its resources. Ordinary frame retirement
still requires collected fence completion.

The current minimal control instead releases list/allocator, swapchain/queue,
descriptor heap/fence, then targets, adapter/factory/device and HWND. Its
application-held refs remain alive at ExecuteCommandLists, it does not reset
the allocator or resubmit after removal, and main owns creation, Present,
Release and DestroyWindow. Its watchdog thread makes no graphics calls.
No definite app-thread/COM ownership or fullscreen contract violation causing
this forced-removal AV has been demonstrated by these sources and logs.

## Primary contracts and their limits

[RemoveDevice](https://learn.microsoft.com/en-us/windows/win32/api/d3d12/nf-d3d12-id3d12device5-removedevice)
marks the device unusable and signals monitored fences to UINT64_MAX.
[GetCompletedValue](https://learn.microsoft.com/en-us/windows/win32/api/d3d12/nf-d3d12-id3d12fence-getcompletedvalue)
defines that value as removal. It cannot count as completed GPU work, successful
allocator reuse or worker shutdown. The current wait/submit/poll guards reject
it even if GetDeviceRemovedReason temporarily returns S_OK.

[IUnknown::Release](https://learn.microsoft.com/en-us/windows/win32/api/unknwn/nf-unknwn-iunknown-release)
returns a reference count intended for diagnostics/testing. Dropping one queue
owner, especially when its returned count is nonzero, is not a completion or
worker-join contract. Even a final zero does not supply a documented cross-object
barrier for other runtime threads. No cleanup decision uses these logged counts.

The Microsoft
[CPU Efficiency specification](https://github.com/microsoft/DirectX-Specs/blob/fb5aea403529ef94b9ea94c129f55a0ff92ecc27/d3d/CPUEfficiency.md)
does not have command lists retain referenced resources; callers must not execute
commands whose resources have been destroyed. It also allows destruction of the
command-list object before previous executions complete and requires drivers to
tolerate its destruction after referenced resources. Thus allocator/list-first
release is an internal hypothesis, not a universal destruction-order obligation
or published WARP-worker join. These rules do not permit destroying resources
still used by ordinary healthy GPU work.

The pinned Microsoft
[DeviceResources sample](https://github.com/microsoft/DirectX-Graphics-Samples/blob/287324fbfd8a3068f9631670ab130cd2102ee40d/Samples/Desktop/D3D12Raytracing/src/D3D12RaytracingHelloWorld/DeviceResources.cpp#L447)
HandleDeviceLost first notifies app resource owners, releases each allocator
and backbuffer, then depth resource, queue, list, fence, heaps, swapchain,
device/factory/adapter. It recreates resources using the retained window; it
does not destroy the HWND or wait for a removed-device fence. This supplies a
real sample-backed alternative ordering, not proof it repairs this control.
It also does not prove that releasing a swapchain owner before backbuffer owners
is prohibited. Exact cached sample/spec hashes and revisions are in
.cache/windows-device-removal-review/primary-source-provenance.json.

## HWND, fullscreen and queued presentation

[DXGI destruction guidance](https://learn.microsoft.com/en-us/windows/win32/direct3ddxgi/d3d10-graphics-programming-guide-dxgi#destroying-a-swap-chain)
requires a fullscreen swapchain to return to windowed mode before release,
otherwise DXGI may raise a non-continuable exception. Creation here passes a
null fullscreen descriptor, which
[CreateSwapChainForHwnd](https://learn.microsoft.com/en-us/windows/win32/api/dxgi1_2/nf-dxgi1_2-idxgifactory2-createswapchainforhwnd)
defines as windowed. Production disables DXGI Alt+Enter. A future diagnostic
can record GetFullscreenState before submission; no fullscreen transition is
established by the current control. SetFullscreenState(FALSE,nullptr) is not a
documented cancellation/join for a removed windowed device.

The HWND can have only one flip-model swapchain at a time. The linked deferred
destruction remedy is explicitly
[D3D11 ClearState/Flush](https://learn.microsoft.com/en-us/windows/win32/api/d3d11/nf-d3d11-id3d11devicecontext-flush),
not a D3D12 queue method. Production must release all old app owners before
same-HWND recreation; it cannot import the D3D11 workaround or assume a new
factory forcibly detaches the old swapchain. D3D12 devices are also
[singletons per adapter](https://learn.microsoft.com/en-us/windows/win32/api/d3d12/nf-d3d12-d3d12createdevice):
an existing removed device causes creation to fail. Complete old ownership
release is needed; recreating the factory alone is not recovery.

[MakeWindowAssociation](https://learn.microsoft.com/en-us/windows/win32/api/dxgi/nf-dxgi-idxgifactory-makewindowassociation)
controls message monitoring/mode changes; it does not document a queued-Present
or GPU-work shutdown barrier. The
[Present flags](https://learn.microsoft.com/en-us/windows/win32/direct3ddxgi/dxgi-present)
include RESTART for discarding queued presents during presentation. No documented
guarantee makes an additional Present on a removed device a teardown barrier.
DestroyWindow is likewise not established as a required removed-device detach.
An early-DestroyWindow experiment would destroy the production window identity
and cannot replace same-HWND recovery. Existing source uses one main/UI thread,
so DXGI's cross-thread message-pump deadlock warning does not explain the AV.

## Reviewed healthy-timeout defect and unverified candidate handling

The exact renderer hashes above identify the reviewed baseline. These line
references describe its separate defect, not the cause established by the
forced-removal logs, which already show DXGI_ERROR_DEVICE_REMOVED:

- bridge_windows.cpp:218 calls finish on WM_QUIT; line82 repeats finish in Window
  destruction, then line86 resets Surface.
- dx12_surface.h:115-116 attempts finish in destruction but discards its false
  result; line312-317 returns false from finish when drain/poll/validation fails.
- dx12_device.h:211-229 waits on the real fence, with a 5s deadline at line220
  and timeout return at line226. A failed wait can leave the device healthy and
  the requested fence incomplete.
- dx12_device.h:401-405 drain returns false on signal/wait failure. Line239 in
  retireQueue ignores that result, releases the queue owner at line240 and
  marks closed at line241. Device destruction (line196 onward) continues to
  drop allocators/resources. A preceding Surface finish failure may be retried
  by these destructors rather than becoming one explicit terminal outcome.

If GetDeviceRemovedReason remains S_OK and completion has not been proved,
that path cannot safely proceed as healthy resource retirement. Multiple fresh
5s drain attempts also amplify shutdown latency. Returning an error upstream
does not undo the destructor's resource release. This remains an explicit
pending native-validation item. Root's later working cancellation API is
described below; these original reviewed bytes and evidence are not overwritten.

The acceptable direction is a terminal state that prevents new submission and
distinguishes completed, already removed and failed healthy drain. Preserve the
first error/deadline. If a bounded healthy wait fails, cancel the actual owned
device through supported ID3D12Device5::RemoveDevice before releasing command
storage/resources; do not swallow false and continue healthy retirement.
Already observed UINT64_MAX classifies removal even with a transient S_OK
reason. Cancelled work must become dropped, never completed. Avoid repeated
destructor drains after the terminal outcome; preserve event/device ownership
until their registrations and resources are gone. Unsupported cancellation
capability needs an explicit policy, not an unsafe silent fallback.

This direction establishes the missing healthy-path precondition. It enters
the removed-device teardown under investigation and is not yet a demonstrated
crash-free shutdown fix. It must not use unbounded waits, leaked resources,
thread termination, forced debug mode or timing sleeps.

## Further controls and CPU validation

1. The root-owned system/app-local WARP pair is now actually complete as recorded
   above. CPU comparison confirms only bounded module-path observations were
   added before submission; removal/disposal remains unchanged. Any subsequent
   runtime comparison must preserve byte-identical controls and actual loaded
   WARP/D3D12/D3D12Core/DXGI identities, fence observations and owned exit/stack.
2. The one-factor skip-Present comparison has now actually run under system
   Core/WARP and Agility619 with both WARP variants, with all six swapchain
   cases still failing as recorded below. It excludes queued Present as a
   necessary condition in these controls, not every DXGI/ownership interaction.
3. The sample-backed targets-before-swapchain/queue case has now actually run
   under both system and developer WARP, with all four cases still failing as
   recorded above. Preserve this negative evidence; do not present it as a
   pending experiment or a working teardown fix.
4. Only if still useful, compare DestroyWindow immediately after removal and
   before owned-resource release against the current final-DestroyWindow order.
   This is diagnostic-only and cannot satisfy production HWND-preservation.
5. CPU terminal-state tests should force healthy wait failure with an incomplete
   fence and assert actual-device cancellation precedes every storage/resource
   drop, no second healthy drain, preserved first error, dropped accounting and
   idempotent cleanup. MAX/transient-S_OK must never admit reuse. Such tests
   validate control flow, not WARP quiescence. A later owned native timeout case
   can block only its own queue on a separate unsignalled test fence, let the
   original 5s drain actually fail, then verify cancellation/cleanup and process
   return under the existing outer guard; no pre-removal drain is added to the
   separate immediate-removal controls.

The Microsoft.Direct3D.WARP 1.0.21 x64 developer-test DLL is retained only in
owned ignored cache, SHA256
e79c10550449365adf0a9393d97a0df69941e671ab6e952a78d92da066517ca3.
Its [license](https://www.nuget.org/packages/Microsoft.Direct3D.WARP/1.0.21/License)
permits internal Windows testing and prohibits redistribution. Cache provenance
is .cache/warp-nuget-control/provenance.json. It is not a product runtime,
installer asset or system replacement.

## Agility SDK preparation and actual runtime comparison

The official [release table](https://devblogs.microsoft.com/directx/directx12agility/)
and live NuGet versions index identify Microsoft.Direct3D.D3D12 1.619.6 as the
latest stable package on 2026-10-08, released 2026-09-14, SDK version619.
The exact official archive URL is
https://api.nuget.org/v3-flatcontainer/microsoft.direct3d.d3d12/1.619.6/microsoft.direct3d.d3d12.1.619.6.nupkg.
The owned .cache/agility-sdk-control archive is 35,477,552 bytes, SHA256
08f0489281401aa430fc37322d6c3fc98a8025175aacd714c10d562f4963f1e9.
Extraction selected only nine bounded metadata/header/Core entries. The cached
x64 build/native/bin/x64/D3D12Core.dll is 5,035,320 bytes, SHA256
37fa14281a58cc834076971873006feb8a8d25cddc908d1a345bda1b149ffc7d,
file/product version1.619.6.0.20260914.1. Its Microsoft Authenticode signature
is Valid; package repository-signature verification was not performed.
The package d3d12.h defines D3D12_SDK_VERSION619. Package, metadata, license,
header, extraction and signature hashes are retained in
.cache/agility-sdk-control/provenance.json. No DLL was loaded or executed by
this reviewer; no control EXE was rebuilt for Agility.

The [package license](https://www.nuget.org/packages/Microsoft.Direct3D.D3D12/1.619.6/License)
allows Windows use and conditional object-code redistribution of the package's
explicit distributables. This package lists D3D12Core.dll, d3d12SDKLayers.dll and
d3dconfig.exe. Distribution requires significant application functionality,
protective downstream terms and the stated indemnification obligations; its
trademark/source-license restrictions still apply. It does not authorize
redistributing every packaged PDB, compiler or tool. The
[official getting-started guide](https://devblogs.microsoft.com/directx/gettingstarted-dx12agility/)
recommends excluding the debug layer from production installers and putting
Agility components in an EXE-relative subdirectory to avoid Core/layer mismatch.
No redistribution or product adoption is part of this diagnostic task. WARP's
separate no-redistribution license still applies to its developer-test DLL.

[SetSDKVersion](https://learn.microsoft.com/en-us/windows/win32/api/d3d12/nf-d3d12-id3d12sdkconfiguration-setsdkversion)
requires Windows Developer Mode and must run before device creation; calling
it afterward removes the device. Its LPCSTR directory is relative to the
process EXE, and the matching Core must exist there. The documented minimum
for this API is Windows10 build20348, distinct from Agility's general serviced
1909+ support. Developer Mode was only read here and already reports1; no
registry/system change was made. Missing interface, failed configuration or
missing/mismatched Core must be reported, never silently treated as an Agility
run. Even a successful configuration can select a newer OS Core instead.

The pinned [redistributable specification](https://github.com/microsoft/DirectX-Specs/blob/f3dce4cb4506fd4c9512d697f24760cb0f92a551/d3d/D3D12Redistributable.md)
describes D3D12GetInterface with CLSID_D3D12SDKConfiguration before the device,
or main-EXE SDKVersion/SDKPath exports for application opt-in. The existing
control has neither, so copying Core beside its unchanged EXE does not establish
runtime selection. The older spec's LPCWSTR prototype differs from the current
package/API's LPCSTR; any later diagnostic must use the current contract.
The exact API documentation revision is
ac665cf65de41f7b0c7ffc55c01ca15c3a05ee0f in MicrosoftDocs/sdk-api; both pinned
sources and hashes are saved in this cache.

Root subsequently built one immutable control with a pre-device configuration
branch and a mode that skips only Present while retaining the same swapchain,
targets, commands and teardown. Source SHA256 is
8d83b04edf6cab0f26f413bf7747906f4c2b249336e13a14ef18dfe4cf7133d9;
EXE SHA256 is
39e18b86ef214b24c76f8f17b7616b1ad81d62bf8fe4a4221c5e0c11f9f0ec8c.
Actual .cache/warp-minimal-control/sdk-native-results.json records nine runs:

| Actual runtime | Mode | PID / elapsed | Actual result |
| --- | --- | --- | --- |
| system Core / system WARP | offscreen | 228000 / 0.1345457s | exit0; disposal returns |
| system Core / system WARP | swapchain + Present | 179584 / 2.6902787s | 0xC0000005 |
| system Core / system WARP | swapchain, skip only Present | 227340 / 2.3981544s | 0xC0000005 |
| Core619 / system WARP | offscreen | 205916 / 0.1285414s | exit0; disposal returns |
| Core619 / system WARP | swapchain + Present | 200284 / 2.2183856s | 0xC0000005 |
| Core619 / system WARP | swapchain, skip only Present | 222116 / 2.1929918s | 0xC0000005 |
| Core619 / developer WARP1.0.21 | offscreen | 163212 / 0.1381096s | exit0; disposal returns |
| Core619 / developer WARP1.0.21 | swapchain + Present | 178736 / 2.1844005s | 0xC0000005 |
| Core619 / developer WARP1.0.21 | swapchain, skip only Present | 199260 / 2.1850241s | 0xC0000005 |

All nine observe fence0 of1 before removal, debug0, no outer-watchdog timeout
and owned-PID absence. Both configured runtime groups report SetSDKVersion619
HRESULT0 and actually load Core SHA25637fa1428…149ffc7d from their owned D3D12
subdirectories. Their WARP paths/hashes identify the actual system or developer
DLL; D3D12/DXGI remain the previously observed system DLLs. The failed swapchain
logs retain removed reason0x887a0005, fenceUINT64_MAX and target Release2 then1.
The initial OrderedHashtable/Select-Object console display produced null fields;
the complete bound JSON and native logs were intact. Correcting that display
did not rerun a control or repair native execution.

The newer Core did not remove this failure, and queued Present is unnecessary
for the AV in these swapchain controls. This narrows the observed difference to
the swapchain/backbuffer path versus the tested committed offscreen target; it
does not establish a driver/runtime bug or prove any production replacement.
The prepared .cache/agility-sdk-control directory remains a diagnostic cache;
no SDK/WARP binary is added to the framework or distribution.

## Unpublished software presentation candidate

Root authorized implementation of a testable software-adapter path after the
above real controls. This reviewer owns dx12_surface.h; root owns Device
terminal cancellation and Window message wiring, and another agent owns Go
frame-clock/native acceptance wiring. The Surface working patch has passed
CPU build-only validation, but has not run. No software-window recovery pass
is claimed.
The shaderless single-target controls do not prove a three-target renderer with
atlas uploads, readback and native DIB presentation will safely recover.

Selection uses actual Device.software after adapter creation, including the
existing software flag/Microsoft Basic Render identity handling. Hardware keeps
its HWND flip swapchain, latency event and Present behavior. Software creates
three committed BGRA render targets and no swapchain/latency object. Existing
D3D12 shaders, DIP/pixel-center coordinates, batching, glyph/bitmap resources
and the 16MiB scene/instance limit remain. COPY_SOURCE is the software target's
stable state; every software submission renders then copies to its readback,
even when diagnostic capture is disabled. Timestamp queries remain real.

Targets use actual GetCopyableFootprints and GetResourceAllocationInfo results.
The production software_front_layout helper validates dimensions, 256-byte
row pitch, footprint offset/last-row bounds, a footprint plus16 timestamp bytes
within64MiB, committed target allocation within64MiB and a tight BGRA front
payload within64MiB. There are exactly three target/readback owners and one CPU
frontbuffer. Their checked payload/allocation caps total at most448MiB; optional
diagnostic capture adds its existing64MiB mapping. Existing bounded instance,
atlas and image resources are additional. This is an explicit software cost,
not a claim that all driver/process memory fits448MiB. Resize releases the old
front allocation before allocating the next, avoiding doubled CPU-front peak.

Software slot admission requires opened state, fewer than two in-flight frames,
the round-robin slot already collected and its real completed fence; MAX always
rejects reuse. All three slots rotate. poll maps only fence-confirmed work,
collects every completed submission and selects the highest eligible serial
once for the frontbuffer. Copying rows removes readback pitch padding. An old
slot cannot roll the front image backward. Resize drains/polls before dropping
old targets, discards old front validity and allocates the new size. Recovery
destroys the old Surface; cumulative submission serials continue from saved
stats and no old readback/front owner enters the new Surface.

The existing message loop's bounded64-message turns and visible/dirty admission
remain. pendingCompletion still returns a notification for completed work that
the CPU has not collected; software watches the earliest outstanding fence so
its two-frame admission can resume promptly. There is no new periodic timer,
polling loop or synchronous wait in draw. Hidden/minimized windows continue
collecting existing work but submit no new View/GPU work. Restore and real
resize/state changes explicitly request fresh layout/rendering; expose repaint
only displays the saved valid image. Idle means no additional View, submission
or frame tick after collection, until real work arrives.

The software frame-clock value4/name d3d12-fence represents actual fence-driven
scheduling. It is not DXGI pacing or a vsync claim; hardware retains value3/dxgi.
Backend remains Direct3D12. The real owning HWND presentation property is1 for
DXGI and2 for committed-DIB, assigned only after actual adapter/path selection.
Submitted/Completed count GPU work, FrameTicks counts submissions and ordinary
work has no dropped frames. A GDI repaint changes none of these counters. CPU
submission timing does not include the later asynchronous front-copy/blit cost;
that limitation needs explicit stats documentation or separate measured timing.

Only the UI thread handles the completed image and
[DIB drawing](https://learn.microsoft.com/en-us/windows/win32/api/wingdi/nf-wingdi-setdibitstodevice).
repaint(HDC) uses tight32-bit BGRA with negative height/top-down orientation,
checks the paint clip and native drawing result and owns no persistent DC or
HBITMAP. poll invalidates the window with erase=false after a new completed
front; software
[WM_PAINT](https://learn.microsoft.com/en-us/windows/win32/gdi/wm-paint)
uses BeginPaint/EndPaint to show that image without requesting a Go View.
Initial/size-invalid content must be requested by Window state changes. This
separation avoids an Invalidate→WM_PAINT→View→new-front feedback loop.

[Readback guidance](https://learn.microsoft.com/en-us/windows/win32/direct3d12/readback-data-using-heaps)
requires a completed fence before Map; Map alone does not synchronize GPU work.
The candidate keeps that contract and the existing real GPU capture ABI, rather
than substituting desktop/GDI pixels for shader-result assertions. Native tests
also need actual owning-HWND/DIB paint evidence; a frame-clock property or a
GPU screenshot alone cannot prove that the software image reached the window.

Root's Device candidate requires a retained ID3D12Device5 removal
controller before any queue/resource creation; failure rejects startup. The
controller outlives command storage/resources. Bounded wait/drain failure
terminally cancels the actual owned device, preserves its first error, rejects
later submission and avoids repeated five-second destructor waits. MAX is
terminal removal, not completion. Surface close calls cancelAfterFailedDrain
after unsuccessful finish and preserves its first error; the destructor delegates
to that idempotent close. If this cancellation
contract fails, fail-fast precedes every storage/resource drop; it cannot silently
continue healthy retirement, leak resources or terminate a driver thread.

The [interface documentation](https://learn.microsoft.com/en-us/windows/win32/api/d3d12/nn-d3d12-id3d12device5)
places ID3D12Device5 in Windows10 version1809, while the current
[RemoveDevice method requirements](https://learn.microsoft.com/en-us/windows/win32/api/d3d12/nf-d3d12-id3d12device5-removedevice)
specify Windows10 build20348 for client/server. The capability gate and that
method contract cannot be advertised as support for every older Windows10 build.
These source changes still need native verification. Hardware removal needs its
independent evidence even if the new software path passes.

Initial CPU-only go test -c validation used GOWORK=off, race, strict cgocheck2 and
AMD64v1, exit0. Its five production-source hashes remained identical before
and after compilation; the test EXE was not executed. Surface SHA256 is
9be4a13e34c56136c25d2491e8d1f51ef2d7fa5875a7108176db3632bdffe618;
Device SHA256 is
6fa93ada75f5fbecdf3e2f0563b7dc102a07e8e4f264be710dc70ac73dead837.
Complete source/build evidence is
.cache/windows-device-removal-review/platform-candidate-final-build.json;
EXE SHA256 is ad413050b0f5d7420110801292369ad677fd62779c0cf7907fbaafa3bb1deaaf.
The private build TMP is empty. Scoped diff/whitespace checks passed. Software
completion-event registration now stays armed per outstanding watched fence,
so repeated UI wakes cannot accumulate duplicate software watches. Collection
clears that arm; the existing completion-race diagnostic explicitly rearms
after its own wait consumes the event. Hardware watching remains unchanged.

An independent .cache/windows-device-removal-review/healthy-timeout/probe.exe
was also compiled without execution. It uses actual Device pipeline/resources,
requires a successful initial drain before holding its owned queue, submits one
real frame, then requires the original five-second drain to fail and cancel.
It checks MAX as cancellation with zero counted completion, unchanged first
error and immediate terminal drain/wait/signal/submit rejection. Queue retirement
and destruction must each return within1s, under a40s owned watchdog; no resume
precedes the wait/cancellation checks. Source/binary/Device/shader binding is in
its provenance.json. The first local helper compile failed because boolean
collided with a Windows RPC typedef; its original log/source are preserved,
and renaming only that ignored helper function to json_bool yielded build exit0.
That independent ignored EXE remains build-only; the tracked fixture's later
actual run is recorded below.

The repeatable candidate is now tracked in the private
internal/platform/dx12_timeout_probe_windows.h and internal/dx12timeout command.
It reuses production Device/pipeline/resources and the same held submission,
five-second cancellation and terminal/destructor checks. A private cgo function
returns bounded JSON; bridge.h, public UI APIs and scene/stat ABIs are unchanged.
The command runs its GPU work only in an owned child, binds the JSON to that
actual child PID, rejects malformed/oversized output and applies a40s process
deadline plus5s bounded pipe wait. Non-Windows/no-cgo code is an explicit
unsupported-runtime stub. Root has wired one actual invocation into Windows CI;
its original prepared binary has now passed locally, while new exact-source CI
remains required.

CPU receipt/PID/output-boundary tests passed strict-cgo2/race/shuffle count3 in
1.049s and no-cgo count3 in0.024s; both cgo/no-cgo vet commands exited0.
Those synthetic decoder inputs are not native GPU proof. The tracked fixture
build passed with all eight recorded input hashes stable, test binary not run,
and private TMP empty. The sandbox's VCS-stamping read failure is preserved;
buildvcs=false plus explicit source hashes was used for this build-only artifact.
Private header SHA256 is
4335d5564c5c4e73518f0750072ef6b8f4795ebd01a890721a59d1e33cc10fa5;
.cache/windows-device-removal-review/dx12timeout.exe SHA256 is
332ccdf70940f32c55b1d79cafd93648635cba6c9f8420c3656656a0cd6305a7.
All individual inputs and results are in
.cache/windows-device-removal-review/timeout-fixture-preparation.json. The
original ignored probe and its failed first build remain preserved separately.

The prepared tracked binary was actually run in owned outer PID217168 / child
PID227032, exit0 in7.2236349s. Its real submitted fence3 remained pending at2;
the original bounded drain took5000ms, cancelled the device and observed
18446744073709551615 with zero counted completion. Later drain/wait/signal/submit
all rejected in0ms with the first timeout error unchanged. Retirement took0ms
and destruction15ms. Both owned processes were absent afterward and private TMP
was empty. Evidence is
.cache/software-presentation/software-candidate-timeout-native-result.json
(SHA256196ff91afd3f069805c709eb2ed4b7e8275c17d4a2cf02f3447a35238df09e95)
and software-candidate-timeout-native.log. This proves the earlier Device
timeout path; it does not exercise the new Surface close or Window error return.

The original Surface9be4a13 / bridge86218649 / Device6fa93ad client-r2 binary
also passed three real non-diagnostic Run cycles in3.5779089s, debug0/readback0,
with actual OS DPI and owning-HWND committed-DIB identity. Each closed with
Submitted=Completed=8, Dropped=0, three used slots, maximum in-flight2 and a
rejected closed Context. The final Run closed while actually minimized. Expose
and minimized View/submission/frame-tick counts stayed unchanged; actual native
client colors, upper/lower markers, resize without model invalidation and PNG
capture passed. Its source stamp, result and current.json remain under
.cache/software-presentation/software-candidate-client-r2-*; the receipt SHA256
is cc91fcbd1fbe7e4be38dc8ee38d373bce9aa176d94e3ef91efad2bb930589bf9.
These old bytes are separate from the later close candidate.

### Single final-close candidate, historical CPU compilation stage

This subsection records preparation before Root's actual r3 invocations below.
Its then-pending native checks are historical, not a claim that r3 never ran.

Surface close now returns one cached result and preserves its first error. It
performs the actual finish once while the caller still owns HWND/current state;
an unsuccessful finish must cancel successfully or fail-fast before releasing
any command storage. Cancellation accounts cumulative Dropped=Submitted-Completed
and InFlight=0 without converting MAX into completion. The original
list/all-allocator/swapchain/queue order precedes frame resources/frontbuffer;
later close/finish/destructor calls do not repeat a drain or resource release.
Closed open/resize/submit/ready/repaint/watch admissions reject further work.
The removal watch event remains alive until Device/fence destruction, preserving
its original member order; only the DXGI latency handle closes immediately.

The private diagnosticHoldQueue helper calls actual Device.hold, for an original
five-second pending-frame shutdown test. It adds no public bridge/UI ABI.
Surface SHA256 is
080d7d1e5819a844ea8d55fdc4f6204a7c804f4e4d6f4663140e5f509d146b68.
The real-header C++ syntax-only check passed with production SDK macros and
unchanged168/160/256 command/instance/stats assertions; source bytes stayed
stable during compilation. Evidence is
.cache/windows-device-removal-review/surface-close-syntax.json/log. The first
standalone check omitted those macros and failed on D2D declarations; its original
log/JSON are retained as .first-failure.snapshot. This reviewer ran no native/GPU
process. At that stage bridge integration and real normal/exceptional-close checks
were pending; previous r2 proof could not be reused for the new source.

Required CPU checks exercise the actual layout helper at padding/cap/overflow
boundaries and actual scheduling/terminal policy, rather than duplicating the
algorithm or fabricating a native pass. Root-owned native checks must cover
software ordinary/first-submit removal/recovery/completion-race, all three slots
with maximum in-flight≤2, Completed+Dropped=Submitted, zero idle View/submit/tick
growth, visible DIB colors/orientation, narrow/DPI resize, minimized/hidden
dispatch and shutdown with old-context rejection. Existing shader/glyph/bitmap
pixel, resource budgets and watchdog assertions remain; software clock/path
checks become exact new guards, while hardware's DXGI guards stay exact.

## Root's actual r3 critical native group

Root subsequently executed three native invocations, seven actual Window Runs,
in12.4951427s. All bind the exact source manifest
.cache/software-presentation/software-candidate-r3-source.json, SHA256
ca3330a84b46b7e73e8ee163b215b9652a24f7b368cf7d95f301afa3cb53b656:
Device6fa93ada75f5fbecdf3e2f0563b7dc102a07e8e4f264be710dc70ac73dead837,
Surface080d7d1e5819a844ea8d55fdc4f6204a7c804f4e4d6f4663140e5f509d146b68,
and bridge dfd39938d5bd979f82690f32a27d8c967c052a2689558c280f4bb8eb74945f56.
The group kept its source hashes unchanged and all owned PIDs/private temps
were absent afterward. This reviewer only read those actual records.

The actual held-fifth-frame shutdown (PID227640, exit0,6.2222392s) observed
Submitted5/Completed4/InFlight1 before Quit. Run returned the original
godesktop: GPU fence did not complete within five seconds in5022ms, and its
published final counters were Submitted5/Completed4/Dropped1/InFlight0.
The owned HWND was destroyed, its worker joined and the old Context rejected.
MAX was never counted as completion. Its fixed report is
.cache/shutdown-failure/software-candidate-r3-shutdownfailure/current.json,
SHA25635c02a211033a66a3e985263a5162982516e06eb207cc15ac00100d529321263;
the bound binary SHA256 is
e32ac6de0d0a73c015acbcf71dd9a53437bbb72e56d9e18cb906862cc5c98155.

Forced WARP/debug0/density1 recovery (PID222048, exit0,2.6776558s) passed three
Runs with the original removeAfter8, GPU pixels, idle and closed-context guards.
The recovering Run reports Submitted24/Completed22/Dropped2/recovery1; subsequent
Runs report11/11 and5/5 without dropped work. Real presentation is committed-DIB
with d3d12-fence and maximum in-flight2. Non-diagnostic presentation (PID226824,
exit0,3.5952477s) separately passed three Runs at actual OS DPI144: owning-client
GetDC/BitBlt/GdiFlush colors and upper/lower orientation markers, native resize
and restore without model invalidation, constant idle View/submission/frame ticks
and minimized close. Individual fixed result/log records are
.cache/software-presentation/software-candidate-r3-{shutdownfailure,warp-recovery-debug0-density1,client-debug0}-result.json;
the exact group record is
.cache/software-presentation/software-candidate-r3-critical-summary.json,
SHA2564b63feaefc77ba83790468810e0e8e61943ba03f11806db8781063c4458a99db.
It records native_invocations3 and full_matrix_executed=false for that historical
group. The later matrix below supersedes that matrix-pending stage; default-three
native/Metal/exact-source CI and public-module gates still apply. Limited r3
proof does not publish core17 or claim an OS/driver cause. The strict consumer's
CPU/replay/build validation is separate from these Window invocations; Root's
later new CLI execution is recorded independently in dx12-timeout-receipts.md.

## Root's subsequent actual r4 Windows matrix and CLI

The later actual r4 matrix completed24 cases/72 real Window Runs in99.0928186s,
all passed: ordinary/recovery × debug settings0/1 × hardware/WARP adapter policy
× diagnostic drawable density1/1.5/2. Evidence is
.cache/software-presentation/software-candidate-r4-matrix-results.json, SHA256
22985813d5bb876c6667042d7ff9c413453864cf7e54e9483cd55aee7eb4ddce.
Its NativeExecuted=true, Published=false, and SourceUnchangedThroughMatrix=true
apply to actual executions. All24 owned PIDs were absent afterward and no
original55s outer guard was reached. This reviewer read each fixed case log and
confirmed all24 stored digests match; the reviewer started no GPU process.

All72 closed scene receipts preserve Completed+Dropped=Submitted, maximum
in-flight≤2 and rejected old Context. There are36 actual dxgi/dxgi presentation/
clock rows and36 committed-dib/d3d12-fence rows, keeping hardware and software
contracts distinct. These are shader-result Shadow/recovery cases with the
existing native pixel/resource/idle guards; separate non-diagnostic owning-HWND
client display and exceptional Window closure remain the real r3 checks above.

The r4 source stamp
.cache/software-presentation/software-candidate-r4-source.json is SHA256
317456dc7b07bfe8f4359da54bb66bbfbf2a1533bf180b3d51c1fc50d7bc549f,
with157 bound inputs. It explicitly adds the formerly omitted
internal/dx12timeout/probe_windows.cpp and the immutable historical receipt.
Device6fa93ad/Surface080d7d1e/bridge dfd39938 and native timeout producer are
unchanged; the strict Go receipt consumer/test bytes differ from the earlier r3
preparation. The older pre-correction preparation snapshot remains preserved
with native_executed=false. The final source stamp also remains preparation-only;
later result records supply actual execution evidence rather than changing it.

Root separately executed the new strict-consumer CLI4c937213, child225620:
actual pending fence2/submitted3, original5000ms drain, removed MAX, counted
Completed0/Submitted1, unchanged first error and terminal/retirement/destructor
0ms. Raw .cache/software-presentation/software-candidate-r4-timeout-native.log
and the full source/binary hashes are recorded in dx12-timeout-receipts.md.
That CLI uses real offscreen Device resources and creates no Window; it cannot
be claimed as a second Window close test or replace r3's5/4/1→5/4/drop1/0 receipt.

The full default-three Windows suite is currently Root-owned and pending. This
local matrix covers no Mac native run and no new exact-source CI. Earlier WARP
swapchain failures, original controls and CPU-only records remain historical;
the new evidence neither assigns an OS/driver cause nor publishes core17.
