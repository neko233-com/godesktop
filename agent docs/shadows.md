# Native rounded shadows and positioned Stack children

Published v0.17.0 / 996b5ff16189d95ea72298ee48bd03366b27757e passes all five
exact-source jobs in CI 37713329201 and independent fresh-cache public-module
validation. Intel's original density1.5 curve now reads139 against138, with
all original tolerance/lifecycle guards preserved. Window/main popup integration
in gocode remains separate and unpublished. The following candidate history,
failed original source and preparation receipts are retained without rebinding.

2026-10-08 local candidate; public core v0.16.0, gocode v0.23.0 and its
installed application remain unchanged. Exact-source cross-platform CI,
publication, independent application consumption and popup integration are
still required. The candidate does not establish complete VS Code UI parity.

Exact-source d330f56111038a86b9b1186594006bee7dde8147 / CI 37710675617
attempt 1 passes Ubuntu and ARM Metal, including all native ARM density/recovery
checks. Intel passes density 1 but rejects tiny-bottom-curve at density 1.5:
actual RGB 108, independent RGB 138, alpha 0.4572441002361213 at float32 DIP
query (43.933990478515625,256.0660400390625); tolerance 4 stays unchanged.
Its real completed 720x480 GPU frame and failed.json are retained in the Intel
artifact. Both Windows jobs pass all twelve recovery matrix cases, but reach
their aggregate 15m limit during final native integration after the actual
held 5000ms WARP drain passes. Cancelled jobs are not complete CI proof.
Only the overall Windows job budget changes to 20m; original per-scenario
bounds and repeated/pixel/coverage gates stay unchanged. Native Intel precision
correction and fresh exact-source CI are still pending; no v0.17.0 tag exists.

The new local candidate changes only positive-sigma Metal coordinate
reconstruction: a nested block selects `math_mode(safe)`, `contract(off)` and
`precise::divide` before subtracting the flat origin. Unknown Metal pragmas
are compile errors. Library options remain nil; sharp/ordinary/glyph arithmetic,
scalar shadow functions, HLSL/DXIL, oracles and native tolerance 4 stay unchanged.
Apple's [MSL specification](https://developer.apple.com/metal/Metal-Shading-Language-Specification.pdf)
section 1.6.3 documents these scoped controls for Xcode16/macOS15, while
sections 6.6 and 8.4 distinguish function selection and arithmetic precision.
Small float32 query perturbations can reproduce RGB108 in the independent
oracle, but do not identify Intel's actual emitted instruction. Three focused
strict-cgo/race/shuffle CPU repeats pass in 18.331s and strict vet in 8.507s;
these headless checks do not execute the new Metal fragment division or prove
runtime pragma recognition. New actual Intel/ARM compilation and all-density
GPU pixels remain mandatory. Shader SHA256 is
eca14410d57ba7c00d859db8ae32769d0d4277f1be85a2c583701e0b0f7ce025.

## API and geometry

`Element.Shadow(ShadowStyle)` copies one straight-alpha color, DIP offsets,
`Blur` and `Spread`. Blur is twice the Gaussian standard deviation and clamps
to 0..128 DIP; spread clamps to -128..128 and each offset to -4096..4096.
Non-finite style values become zero; color channels clamp to 0..1. Zero alpha
disables the command. Negative spread that collapses a caster emits nothing.

The caster translates the element rectangle by the offsets and expands it by
spread on each side. Its radius follows Radius/ClipRounded plus spread,
clamped to half its shortest side. A zero blur integrates the mask over the
actual device pixel box; positive blur evaluates the Gaussian-convolved mask
at the device pixel center, with fixed quadrature and four-sigma support.
Those two raster contracts are distinct. Existing ordinary primitive AA is
unchanged. This is an original GPU implementation and introduces no copied
VS Code assets, browser, OS popup surface or bitmap blur texture.

The shadow paints before the element's own background and rounded descendant
mask. It respects all real ancestor rectangular and rounded masks, but can
extend outside its own body. A fully offscreen body can still cast an onscreen
halo. Shadows do not change measurement, layout, hit geometry, visible keys,
focus order or rounded-mask depth. Tight flow parents still clip the halo.

`Element.Position(x,y)` only changes direct Stack-child layout. Coordinates
are relative to the Stack's padded inner origin; child size is its explicit
or intrinsic size. Positions are finite and bounded to +/-1,000,000 DIP,
including negative offsets. Positioned children do not contribute to Stack
intrinsic measurement. Their shadows receive the actual Stack inner ancestor
clip, so a floating body keeps its exact position/hit geometry without tight
row/column spacers cutting the halo. Other parent kinds and ordinary Stack
children retain their prior behavior. The API does not relax ancestor clips.

## GPU encoding and cost

Command kind 7 discriminates shadows; kind 6 remains intrinsic color glyphs.
GDCommand remains 168 bytes and GDGPUInstance remains 160 bytes, with existing
offset assertions intact. For kind 7 only, GDCommand.font_size stores sigma;
the GPU instance reuses UV for the caster rectangle and padding[1] for sigma.
Glyph padding[0] is unchanged. Scene bounds cover the four-sigma halo plus AA.
Each shadow costs one instance in the existing ordered batch, with no extra
render pass, descriptor/texture asset, worker or invalidation timer.

The HLSL and Metal implementations integrate the central rectangular strip
analytically and each rounded cap with twelve fixed Gauss-Legendre nodes.
Zero-blur pixel-box coverage uses eight cap nodes. Positive-sigma queries reflect
the symmetric mask toward its top/left edge; cap integration uses normalized
query-relative standard deviations. A compensated split-product circle-equation residual
computes horizontal near-edge distance without rounding an absolute inset.
HLSL marks the compensation precise; Metal uses a local floating-point pragma
around only that compensation/cap calculation. Actual Metal compilation and GPU
execution of this latest precision correction remain CI requirements.
The sharp branch retains `radius - sqrt(edge*(2*radius-edge))` geometry and
its original pixel derivatives.
Premultiplication happens once before ordinary ancestor mask coverage.

Sources: shadow.go, element.go, layout.go, internal/platform/platform.go,
native.go, bridge.h, gpu_scene.h, shaders/ui.hlsl and gpu_shader_metal.h.
The generated DXIL uses pinned DXC 1.9.2609.5 and source/ABI SHA256
9174474954c912e2e21e763b3c2abce7eef884827c2754b1a3768d1022ccc356
(vertex 5180 bytes, fragment 19344 bytes). The complete generated header's
SHA256 is 39499adf35c21d3230d495a0b5d9e4a95a5233d52f8a8745f9e180bcd502e739.

Positive-blur fragments now reconstruct DIP queries from actual SV_Position /
Metal fragment position divided by the exact float32 drawable density, then
subtract a flat caster origin. Zero blur keeps its original interpolated local
coordinate and pixel derivatives. HLSL precise and the new local Metal safe/
precise division branch preserve the intended coordinate calculation. The private
frame uniform remains four
float32 values / 16 bytes: DIP width, DIP height, density at byte offset 8, pad.
Windows root parameter 1 exposes its four DWORD constants to both shader
stages; Metal passes the same float4 and a flat density. This does not change
the public 168/160-byte ABI or add another GPU instance.

## Independent portable evidence

shadow_test.go and position_test.go cover paint/order, genuine ancestor masks,
unchanged hit/measurement, value normalization, disabled/collapsed/offscreen
casters, radius/spread and positioned intrinsic/negative geometry. The actual
C/C++ mapping probe checks the unchanged ABI and reads produced GPU instances.

internal/shadowtest independently rasterizes a supersampled binary rounded mask
and convolves separable Gaussian weights. Its image renderer uses two explicit
passes. It does not use the shader CDF or quadrature. Its tests cover analytic
Erf rectangle integration, circle center, tiny/max sigma, symmetry/translation,
spread/opacity, sharp pixel areas at fractional density, radius clamps,
offscreen halo, separate image/point paths and allocation/work bounds.

The actual-source HLSL/Metal math corpus contains 272 Gaussian and 816 sharp
samples at physical densities 1/1.5/2. Inputs are quantized to the public
float32 geometry before the independent oracle. Ninety-six tiny-sigma cases
refine independent raster phase, and sharp references tile 32x32 pixel samples.
There are no oracle-budget fallback samples. Three strict-cgo/race/shuffle
repeats pass; maximum Gaussian error is 0.003984950 against the unchanged .004
gate, maximum sharp pixel-box error is 0.015231371 against its separate .02
gate, and paired shader-source disagreement is zero. This headless math/ABI
evidence does not prove native DXIL or Metal execution.

Initial corpus failures are retained: subtracting a 200-DIP half-size lost a
.0001-DIP displacement, returning .130987 instead of normal CDF(-1) .158655;
curved samples also failed. Twelve-node local-coordinate evaluation fixes the
real precision/quadrature defects without raising the error guards or imposing
a blur minimum. A single-phase tiny-circle oracle alias was separately refined.

A later independent mirror extension keeps every Gaussian point reflected at
the right, bottom and right-bottom edge. Its 1088 Gaussian / 816 sharp samples
find sixteen actual paired-shader failures, all at sigma .0001; the .05 controls
pass. The unchanged .004 Gaussian gate rejects maximum error .070973937.
For 400x300/radius150, bottom center returns .557241142 against oracle .5,
and a right-bottom curved sample returns .634428620 against .705402557.
This proved the successful earlier top/left corpus did not cover precision
loss while cap quadrature runs near a large bottom coordinate. The new tests
retain that failure evidence. Reflection alone reduced the failure count to
two curved points, still error .006416510. Normalized cap integration and the
compensated residual then pass all original and mirrored tests without a blur
floor or relaxed error guard: 1088 Gaussian / 816 sharp, 384 phase refinements,
zero fallback samples, maximum Gaussian .002883251 and sharp .015231371,
paired disagreement zero. Three strict-cgo/race/shuffle Shadow/Position/ABI/math
repeats pass 16.916s (.cache/shadow-mirror-compensated-race.log); the fresh verbose
corpus passes 3.886s (.cache/shadow-mirror-compensated-metrics.log). The latest
pinned regeneration and -check pass 1.963s. Native latest-shader proof remains
separate and pending.

Primary review then identifies a portability limitation in that interim FMA
version: [Microsoft's mad specification](https://learn.microsoft.com/en-us/windows/win32/direct3dhlsl/mad)
allows either fused or unfused hardware. The headless std::fma wrapper was only
proof of its fused interpretation, not all DXIL implementations. The final
shader therefore removes every mad/fma dependency and uses 4097 mantissa-split
product errors with non-refactorable additions; the CPU wrapper also removes
its mad/fma overloads. A fresh strict-cgo/race corpus passes 3.913s, maximum
Gaussian .002883221, sharp .015231371, paired disagreement/failures zero;
.cache/shadow-mirror-no-fma-first.log. Final pinned regenerate/check passes
2.135s. GPU execution and Metal pragma acceptance still require real proof.

After the physical-fragment uniform change, all Shadow/Position/ABI/actual-source
math checks pass three strict-cgo/race/shuffle repeats in 17.944s
(.cache/shadow-physical-frame-final-race.log). The new wiring test compiles
the actual HLSL frame field declaration to check 16 bytes / density offset 8,
and checks the four-DWORD Windows encoders, pixel-stage root visibility and
Metal float4/density contract. Independent oracle tests pass three repeats
3.961s, and non-GUI native-fixture tests pass three repeats 1.126s
(.cache/shadow-physical-fixture-oracle-race.log). These checks are CPU evidence;
these CPU checks do not replace actual native pixel acceptance.
Full no-cgo tests, strict-cgo vet and pinned DXC regenerate/-check also pass
(.cache/shadow-physical-frame-{nocgo,vet,regenerate,dxil-check}.log).
The source-bound race binary is
.cache/shadow-native/physical-position-shadow-native.exe, SHA256
2bf6ccc0eff137b00c8a9288a86611efd7a038c8e5251bb633fd6baf1c709dde.
Its source hashes are unchanged before/after the build and recorded in
.cache/shadow-native/physical-position-source.json. That immutable stamp was
created before native execution; the subsequent native result binds its hash.

Current local commands use GOWORK=off, GOEXPERIMENT=cgocheck2 and private
.cache/go-cache-shadow, .cache/go-tmp-shadow/TMP/TEMP:

- `go test -buildvcs=false -race -shuffle=on -count=3 -run 'Test(Shadow|Position)' .`
  passes 8.632s; .cache/shadow-final-portable-race.log.
- `go test -buildvcs=false -race -shuffle=on -count=3 ./internal/shadowtest`
  passes 3.990s; .cache/shadow-final-oracle-race.log.
- `go test -buildvcs=false -race -shuffle=on -count=3 ./internal/shadownative`
  runs only non-GUI ownership/reset tests, passes 1.059s;
  .cache/shadow-fixture-nongui-race.log.
- CGO_ENABLED=0 `go test -buildvcs=false ./...` passes all packages;
  .cache/shadow-final-nocgo.log. Strict-cgo `go vet -buildvcs=false ./...`
  passes 9.430s; .cache/shadow-final-vet.log.
- `go run -buildvcs=false ./internal/shadergen -dxc .cache/tools/dxc-1.9.2609/bin/x64/dxc.exe -check`
  passes 1.861s; .cache/shadow-final-dxil-check.log. The same direct pinned
  generator regenerated the header successfully. The PowerShell
  scripts/validate-shaders.ps1 wrapper returned AuthorizationManager check
  failed before running; wrapper success is not claimed.

## Owned native GPU fixture and current unresolved failure

internal/shadownative builds a race-enabled standalone tool. It reads completed
pixels only from its own Windows HWND or Metal drawable and replays input only
to that window. CLI: `-adapter hardware|warp -density 1|1.5|2 -recovery -output`
under fixed .cache/shadow-native or bin/shadow-native subdirectories. Windows
hardware means hardware-preferred with software fallback; no dedicated GPU is
claimed from that policy string. Reports record actual backend separately.

Each invocation runs two independent native lifecycles. The first optionally
removes its actual Windows device after eight submissions; Metal injects a
same-window resource recovery request. Completed readback checks tinted halo,
premultiplied body order, positive/negative spread, offset, fractional sharp
coverage, sharp rounded partial-pixel coverage, actual rectangular/rounded
ancestor masking and a visible halo from a fully offscreen body. Mask samples
require significant unclipped alpha as a negative control; the sharp rounded
sample must have partial coverage alpha in (.02,.58), rejecting a trivial
fully white or opaque comparison. Stable scene budgets are nine instances/1440 upload bytes,
one draw, three slots/mask7, zero bitmap assets/uploads and zero buffer waits.
Idle checks require unchanged submissions and View count. Owned pointer replay
proves halos do not add hits, offscreen bodies do not retain key bounds, rounded
ancestor/body hit rejection and unchanged body coordinates. Shutdown verifies
no in-flight frames, completed+dropped=submitted and closed Context rejection.

The local Windows OS monitor DPI stays 144. Diagnostic densities actually change
the GPU target to 480x320, 720x480 and 960x640 without changing global display
settings. These are actual GPU drawable-density tests, not WM_DPICHANGED or
physical monitor-switch evidence. The original three hardware-preferred cases
pass both Runs, forced recovery, all eleven original pixel guards and
input/idle/shutdown checks;
150% run-0.png is inspected. Fixed reports are in
.cache/shadow-native/hardware-density{1,1.5,2}/current.json and run-{0,1}.png.
The extended fixture passes all six hardware-preferred/forced-WARP x three
drawable-density cases with debug=0 and without recovery, two Runs each and
all twelve pixel/idle/input/shutdown guards; maximum native RGB-channel error
is one (guard four). See .cache/shadow-rounded-partial-matrix.log and
.cache/shadow-native/{hardware,warp}-rounded-density{1,1.5,2}/. At 150%, the
sharp rounded partial sample is RGB151 versus oracle RGB150, with independent
alpha .41015625. Those ordinary matrix captures precede the latest mirrored
precision correction. Latest-shader GPU and extended recovery validation
remain pending below.

Forced WARP with debug=0 passes ordinary rendering and both lifecycles, but
forced actual device removal currently crashes 0xC0000005. Debug=1 recovery
passes; that is a distinct diagnostic result and does not replace the failing
non-debug gate. The owned GDB capture .cache/shadow-warp-gdb-trace.log shows a
WARP worker faulting in QueryMuxDListForApplication while the UI thread releases
a WindowFrame render target during Surface destruction. A queue-first teardown
candidate also fails (.cache/shadow-warp-retire-gdb.log); Queue.Release has not
been established as execution quiescence. No pre-injection drain, dropped-frame
guard relaxation or universal removed-fence completion claim is introduced.
Local OS version is 10.0.26300.0 and d3d10warp.dll is 10.0.26100.9278; that
version information alone does not establish an operating-system defect.
The original and candidate binaries/logs remain private ignored evidence. Full
three-repeat native acceptance, Metal CI and publication remain pending this
real lifecycle failure.

A subsequent allocator-first/cache-retained teardown candidate was first built
without native proof. It releases all command lists/allocators before any
frame target/descriptor/page, retains caches until Surface destruction, and
always treats UINT64_MAX as failure. The original injection threshold of eight
shadow submissions and all dropped-frame guards remain unchanged. Its seven
source hashes are recorded in .cache/shadow-native/allocator-first-source.json;
binary SHA256 is abda2b4d1657ac00fe41e3fa6c77b559d8b008583b578213e9e2ac156df61fbe.
This candidate subsequently crashes forced-WARP/debug=0 recovery with
0xC0000005 after .473s (.cache/shadow-warp-allocator-first-native.log). Its
interim 4d62217d FMA shader is distinct from the final no-FMA precision source.
No Queue.Release synchronization guarantee or successful repair is claimed.

To distinguish a prior backend defect from the new shader workload, immutable
public16/5178f551 is archived under .cache/core16-shadow-control. The rebuilt
original renderstress uses its unchanged ninth-submission removal/90-frame
guards. A separate diagnostic-only plain rectangle/rounding/text program uses
first-submission removal, readback, debug=0/forced-WARP, true dropped-frame
accounting, fourteen post-recovery completions, GPU pixels and two lifecycles.
The 150 archived source entries are byte-identical; only the private diagnostic
package is added. Both controls actually crash forced-WARP/debug=0 without any
Shadow code: the original ninth-submission/90-frame program exits 0xC0000005
in 1.3749s and the first-submission control exits the same code in 1.3722s.
This reproduces the failure in the immutable existing public16 backend; it
does not establish an OS/driver defect or a successful framework repair.
.cache/core16-shadow-control/native-control-results.json binds exact commit,
binary hashes, flags, failures and cleanup. The two owned processes are absent
after return; the verified empty private native-temp directory is removed.
Their provenance records bind archive/source/binary hashes;
the first-submission binary SHA256 is
8e013de10b85e7364d46fad04f197398b18d289d28c8ee1c1fcd3c44615757ac.

The final numeric GPU fixture adds a third independent Run after both original
Runs. Their twelve pixel, nine-instance, recovery, input and idle guards remain
intact. The third uses two 4x4 true ancestor clips, transparent 400x300/radius150
casters at sigma .0001 and independent phase-refined DIP-coordinate mask
references at bottom-flat and curved pixels. Its separate budget is three
instances/480 bytes/one draw, with the same idle/closed-context/resource proof.
The first actual hardware-preferred/debug=0 density-1 execution fails:
the bottom-flat pixel is RGB255 against independent RGB128 / alpha .5,
after both original Runs pass all twelve pixels and nine-instance guards.
The third closes cleanly with five submitted/completed frames, three instances,
480 bytes and rejected closed Context. Exact failure evidence is
.cache/shadow-no-fma-hardware-native-first.log and
.cache/shadow-native/no-fma-hardware-density1/failed.json. This real GPU failure
is separate from the passing headless mathematics and the WARP-removal AV.

The latest candidate replaces interpolated shadow queries with actual physical
fragment positions. Vertex snapping/interpolation remains an explanation of
the old failure rather than a separately isolated root-cause proof. References now
keep the original DIP caster/radius/sigma, with query coordinates evaluated as
float32(device pixel center) / float32(density) minus float32(caster origin).
They do not multiply sigma by density. CPU geometry only proves each selected
point has nontrivial fractional coverage at 1/1.5/2. The five fixed evidence
files now include run-2.png. Reset, unrelated-file retention, outside-root
rejection and all six fractional sample preparations pass three strict-cgo/race
repeats 1.118s (.cache/shadow-third-run-fixture-race.log). No CPU preparation is
reported as a native tiny-sigma pass.

The source-bound physical-position binary now passes all six ordinary native
cases, hardware-preferred/forced-WARP x actual drawable density 1/1.5/2,
debug=0/TRACE=0, in 20.353601s total. Each case has all three Runs, both original
twelve-pixel/nine-instance checks and the separate two tiny-Gaussian pixels /
three-instance check; one draw, input/idle/no-busy-loop/closed-context/frame
accounting guards all pass. Across 156 real native RGB samples, maximum channel
error is one against the unchanged guard four. The bottom-flat pixel is now
RGB128 against oracle RGB128 at every density, retaining the old RGB255 failure
in its separate directory. The curved pixel is RGB117 against oracle RGB116
at densities 1/2 and RGB138 against RGB138 at 1.5. All eighteen owned windows
close; six outer processes and all private owned-temp directories are absent.
The actual 150% ordinary PNG is inspected.

Fixed current.json/run-{0,1,2}.png artifacts are under
.cache/shadow-native/physical-position-{hardware,warp}-density{1,1.5,2}/.
.cache/shadow-native/physical-position-native-results.json binds the immutable
build stamp/executable hash, exact flags, owned PIDs, exit codes and log hashes;
physical-position-native-summary.json records all pixel comparisons. The policy
reports hardware preference with fallback, not proof of a dedicated adapter.
OS monitor DPI remains 144; these are drawable-density GPU tests. Metal/full
repeated CI/publication checks remain outstanding, and the independent
non-debug WARP removal failure is still unresolved.

The same unchanged source-bound binary then passes six hardware-preferred
recovery cases: debug=0/1 x densities 1/1.5/2, TRACE=0. Each has three Runs,
the original removal threshold of eight submitted frames and all old guards.
The first Run records one real recovery, one dropped in-flight frame,
completed+dropped=submitted and final in-flight zero. All original/tiny pixels,
idle/input/closed-context guards pass with maximum channel error one. Debug=0
times are 4.0617823/4.2881509/4.5314595s; debug=1 times are
9.3515387/9.3027943/9.783649s, 41.3193747s total. Adapter descriptions are not
exposed by this fixture; this establishes the reported hardware-preference
policy, not a claim that the actual selected device is a dedicated GPU.

One current-shader forced-WARP/debug=0 recovery density-1 negative control
still exits 0xC0000005 / -1073741819 after .2133051s. It has no successful
closure/current.json report. The separate debug=1 density-1 control passes
4.1967243s, all three Runs and all pixel/idle/accounting guards, including one
real recovery and one dropped frame. WARP debug=0 densities 1.5/2 are not
repeated after the known failure and are not reported as passing. The recovery
runner intentionally returns failure because the real non-debug AV persists.

Fixed evidence is in
.cache/shadow-native/physical-position-{hardware,warp}-recovery-debug{0,1}-density*/,
physical-position-recovery-native-results.json and recovery-summary.json;
.cache/shadow-physical-position-recovery-native.log preserves exact results.
All fourteen ordinary/recovery process IDs are absent and private scratch
directories number zero (.cache/shadow-native/physical-position-cleanup.json).
The one crash's fixed, empty, unreparsed scratch is verified within the owned
workspace and removed non-recursively. The native GUI slot is explicitly
returned to the coordinating agent before any further diagnostics.

## Committed-target Windows presentation candidate: source-bound checks

A subsequent unpublished software-path candidate keeps actual D3D12 rendering
but replaces the WARP swapchain target with committed render targets, fenced
readback and an owned native DIB presentation. Its initial preparation had only
CPU evidence; the first limited native results are recorded below. Hardware
keeps DXGI. This candidate remains unpublished.
The statistics ABI remains 256 bytes, command 168 and instance 160. Native clock
4 maps to `d3d12-fence`; clock 3 remains `dxgi`.

testing/winprobe.NativePresentation reads the exact HWND's backend/presentation
properties and checks expected PID before and after the read. Property 1 is
DXGI, property 2 committed-dib; missing/unknown values and other backends fail.
ValidateWindowsPresentation requires the corresponding exact clock/backend.
renderstress's new `-require-frame-clock windows-native` mode additionally binds
that actual path to the constant owned HWND. The existing exact `dxgi` mode
still rejects a fence clock. All prior 90-frame, slot, idle, recovery, pixel,
input and close guards remain. The classic integration fixture receives the
child's actual RendererStats in its existing UI probe; the parent's statistics
cannot stand in for that separate process. Shadow runs record and validate
their actual presentation before pixels and after closure; forced WARP must
report committed-dib. The capture still reads the fenced GPU mapping.

Three no-cgo repeats pass the native clock mapper / ownership / 64 path-clock-
backend combinations and renderstress strictness checks (winprobe .023s,
platform .022s, renderstress .021s;
.cache/software-presentation-contract-nocgo.log). A headless C++ probe executes
the actual Surface software_front_layout helper and actual Capture capacity,
with 25 size/row-pitch/offset/last-pixel/allocation/timestamp/overflow/null-output
boundary cases. Three repeats pass .929s
(.cache/software-presentation-footprint-nocgo.log). No resource allocation,
queue quiescence, shutdown or actual DIB/GPU acceptance is inferred from that
scalar admission check. Full repeated native matrices and exact-source
cross-platform validation of this later candidate remain pending.

Each run clears only five owned fixed evidence files, so a failed invocation
cannot leave a previous current.json/PNG falsely implying success. It reuses
an unreparsed workspace-owned fixed TMP/TEMP/TMPDIR and removes that empty temp
directory after clean shutdown. It never recursively removes output parents,
the sibling binary or user workspaces. Non-GUI tests prove repeat reset retains
unrelated files and an outside-root output is rejected before creation.

The first strict-cgo/cgocheck2/race/shuffle three-repeat portable checks pass:
platform 1.077s, winprobe 1.059s, renderstress 1.077s, softwarepresent 1.077s,
and shadownative 1.136s. Root Shadow/Position/numeric-corpus/actual scalar-helper
checks pass three repeats 19.569s. Full no-cgo packages and both no-cgo/strict-cgo
vet pass. The final source-bound build/test preparation takes 16.0237047s and
the client CPU tests pass three repeats 1.052s. Fixed logs are
.cache/software-candidate-{contract,portable}-race.log and
.cache/software-presentation/software-candidate-client-cpu.log.

The immutable first build stamp is
.cache/software-presentation/software-candidate-source.json,
SHA256 218f9bb31161e879e89f43d50379c415f086eb72d803eca998355965f03d2c31.
It records all source hashes checked unchanged before/after the four builds.
The shadow binary is
6b1796edf3b35f9e5094905795e43c148ccf2bd0daa39c600e109642f8b533a5;
Device/Surface hashes are respectively 6fa93ada... / 9be4a13e.... Earlier
physical-position binaries, reports and failure snapshots are retained.

One actual forced-WARP/debug=0/TRACE=0 shadow recovery at diagnostic density 1
passes 2.6722913s. All three Runs identify actual committed-dib and
d3d12-fence; both original twelve-pixel/nine-instance scenes and the separate
two-pixel/three-instance tiny-sigma scene pass. The unchanged removal threshold
is eight submissions. The first Run closes with submitted24/completed23/drop1,
one recovery, zero in-flight; later Runs have 11/11 and 5/5 with no drops.
All have three slots/mask7/max-in-flight2, stable idle Views and rejected closed
Contexts. Actual OS DPI is 144, separate from diagnostic density 1. The owned
PID210272 exits and private scratch is absent. Fixed current.json/three PNGs
are under .cache/shadow-native/software-candidate-warp-recovery-debug0-density1;
the outer source/binary/PID/exit/log receipt is
.cache/software-presentation/software-candidate-warp-recovery-debug0-density1-result.json.
This is one bounded recovery result, not a complete replacement-path matrix.

The non-diagnostic client fixture additionally checks actual scheduler
FrameTicks equality during expose/minimized idle. Its separate revision-2
client stamp is baf4f3fb20e2b6f78f30a81cff26c45d4fe12a75454e19efaaafdee47c15b545
(.cache/software-presentation/software-candidate-client-r2-source.json),
binary fb0f8b71bbb0ae0c35c5ee35c0d5951988cad8f707f18b67339041e2a87d0324.
The only change from the first stamped source is the two scheduler-idle guards;
the first stamp/binaries remain intact. Three strict-cgo/race CPU repetitions
pass 1.056s; test/build preparation takes 6.8974961s.

The revision-2 client then passes three real forced-WARP/debug=0/TRACE=0 Runs
in 3.5779089s with READBACK=0 and the diagnostic capture mapping absent.
It reads only its own visibly restored HWND through GetDC/GetPixel and actual
client BitBlt into a top-down DIB, flushes the locked worker thread's GDI batch
before reading bitmap bits, and saves that actual client image. Red upper and
blue lower markers pass both GetPixel and BitBlt, proving orientation. At actual
144-DPI without a drawable-density override, five stages have fresh completed
frames 4→5→6→7→8: initial pixels, OS resize without model Invalidate, orange
model change, restore without model Invalidate, then green model change.
Eight real WM_PAINT exposures keep FrameTicks/Views/submissions 4→4; minimized
idle keeps them 6→6. All three Runs use slots3/mask7/max-in-flight2, no buffer
waits and exact one-draw/three-instance/480-byte guards. The third closes while
actually iconic; all old Contexts reject dispatch. PID191072 and its private
scratch are absent. Fixed receipt/three actual client PNGs are in
.cache/software-presentation/software-candidate-client-r2-debug0/. Wall time
to visible pixels is a separate field; CPUTimeNanos still ends at submission
and does not include later readback/front-copy/WM_PAINT latency.

The separately prepared actual Device timeout binary
332ccdf70940f32c55b1d79cafd93648635cba6c9f8420c3656656a0cd6305a7
passes 7.2236349s total. Its production held WARP submission remains genuinely
pending, initial healthy drain/re-submit succeeds, and the next drain times
out at 5000ms. Real removal returns reason 0x887a0005 and fence MAX, which is
not counted as completion: submitted1/completed0. Subsequent terminal
drain/wait/signal/submit all reject in 0ms, retaining the first timeout error;
queue retirement takes 0ms and Device destruction 15ms. No resume/pre-drain
is used. Outer PID217168 and child227032 exit, private scratch is empty/removed.
The preparation SHA256 is
1e638821ac4132b32cd5099b07bcb676cff64b91a89ef2fad96b1ae0bf8c4c96;
.cache/software-presentation/software-candidate-timeout-native-result.json
binds it, the actual binary and raw native report.

After these three invocations the native GUI slot is explicitly returned.
Review then identifies a separate late-final-close error propagation/owned-HWND
ordering gap in the bridge. Its next Surface/bridge revision must receive a new
build stamp and repeated native checks: these immutable first-pass binaries
cannot stand in for later source. The full HW/WARP/debug/density matrix, actual
late-close failure propagation, both Metal architectures, source CI and public
module/application integration remain outstanding. No v0.17.0 is published.

The next private exception CLI is prepared in internal/shutdownfailure with
shutdown_failure_windows_test.go. It enables input isolation and the bridge's
shutdown-timeout diagnostic hook, waits for the exact owned HWND's native
godesktop.shutdown-pending=1 cause property and a genuinely pending fifth
submission after four real completions, then calls Context.Quit. Its receipt
requires the original five-second error from Run, unchanged Completed,
exact dropped/submitted accounting, zero final InFlight, destroyed owned HWND,
rejected old Context and a three-second worker join. The root wrapper keeps
a two-minute build deadline/three-second pipe wait, forty-second process guard;
the CLI additionally has a thirty-five-second watchdog. It clears only its
fixed current/failed receipts, checks ordinary workspace cache ancestors and
removes empty private scratch before publishing success. This does not separately
timestamp every native resource release against WM_NCDESTROY: that precise
ordering remains a native-source review contract.

Twenty-seven synthetic receipt forgeries plus owned-property short-circuit,
outside-root rejection and three-repeat fixed-file reset tests pass strict-cgo/
race/shuffle three repeats 1.065s and no-cgo receipt tests .021s; both vets and
the preliminary race build pass. Logs are .cache/shutdown-failure-cpu-final-
{race,nocgo}.log. No GPU/GUI execution of this exception fixture has occurred;
the provisional e32ac6de... executable must be rebuilt/stamped after the final
bridge hook and Surface close source freeze before actual native acceptance.

The final revision-3 Surface/bridge close candidate is then rebuilt and tested
against its own source stamp, .cache/software-presentation/software-candidate-
r3-source.json, SHA256
ca3330a84b46b7e73e8ee163b215b9652a24f7b368cf7d95f301afa3cb53b656.
All 155 source inputs match before/after build and after native execution;
bridge SHA dfd39938... / Surface 080d7d1e... / Device 6fa93ada... are frozen.
An initial nonexistent VERSION preflight input is retained as an invalid
preparation record and corrected with stop-on-error; no native test uses that
invalid record. The final build does not edit source, tags or publication data.

Its real owned-window shutdown failure passes 6.2222392s. PID227640/HWND88084396
at actual144 DPI identifies committed-dib/d3d12-fence and the actual native
shutdown-pending cause property. Before Quit, submitted5/completed4/in-flight1
is observed. Run returns the non-nil original error
"godesktop: GPU fence did not complete within five seconds" after 5022ms;
final submitted5/completed4/dropped1/in-flight0 preserves completion through
removal, with no device recovery. The worker joins, owned HWND is destroyed and
old Context rejects dispatch. The timeout CLI binary is e32ac6de...; its actual
receipt is .cache/shutdown-failure/software-candidate-r3-shutdownfailure/current.json.
This runtime proves error propagation/accounting/final closure. Exact native
resource-release-before-WM_NCDESTROY ordering remains explicitly source reviewed.

The new r3 shadow binary 14938afd... passes forced-WARP/debug=0/TRACE=0 recovery
at diagnostic density1 in 2.6776558s with the unchanged eight-submission removal
threshold, three Runs and original/tiny pixel/input/idle guards. Its first Run
has submitted24/completed22/dropped2: both actual in-flight frames are abandoned,
not claimed completed. One recovery, max-in-flight2 and the original accounting
guard pass; the later Runs have 11/11 and 5/5 with zero drops. All 12+12+2 RGB
comparisons pass with maximum channel error1 against unchanged tolerance4.
The new client binary 753d5514... separately passes three non-diagnostic actual
144-DPI Runs in 3.5952477s, preserving READBACK=0, raw GetPixel/BitBlt orientation,
five fresh stages, independently triggered OS resize/restore, stable FrameTicks/
Views/submissions during exposures/minimized idle, and minimized final closure.

These three native invocations total 12.4951427s and seven actual window Runs.
PIDs227640/222048/226824 are absent; private native scratch and Go TMP entries
number zero. Fixed source/binary/PID/exit/log receipts and the aggregate proof
are .cache/software-presentation/software-candidate-r3-*-result.json and
software-candidate-r3-critical-summary.json. The earlier Device-only actual
five-second probe is reused only because its immutable Device source/binary
are unchanged; it is not reported as a new r3 window run. The GUI slot is
explicitly returned after the critical group. Full repeated r3 HW/WARP/debug/
density matrices, the three-repeat root native exception test, both Metal
architectures, exact-source CI/public module and app integration are still
required before publication.

The later revision-4 full shadow matrix is actually complete: hardware-preferred
and forced-WARP × debug0/1 × drawable densities1/1.5/2 × ordinary/recovery,
24 cases and 72 real window Runs, pass in 99.0928186s. All original twelve-pixel/
nine-instance scenes, tiny two-pixel/three-instance scene, old removal threshold8,
input/idle/closed Context and frame-accounting guards remain. Actual owned HWND
properties identify 36 DXGI/dxgi-clock Runs and 36 committed-dib/d3d12-fence Runs,
including non-debug WARP removal. Monitor/window DPI stays144; the three drawable
densities are not claims of global DPI changes. Every outer PID exits and private
scratch is removed. Adapter policy is still hardware preference with fallback;
an actual DXGI path is distinguished from the earlier policy-only evidence.

.cache/software-presentation/software-candidate-r4-matrix-results.json binds
exact cases, executable/source hashes, PIDs, actual properties and per-Run
counters. Its SHA256 is
22985813d5bb876c6667042d7ff9c413453864cf7e54e9483cd55aee7eb4ddce;
source stamp SHA256 is
317456dc7b07bfe8f4359da54bb66bbfbf2a1533bf180b3d51c1fc50d7bc549f,
binding 157 inputs unchanged through the matrix. Revision3 evidence remains
intact. Native product C++ is unchanged from r3; r4 additionally binds the actual
Device timeout receipt/consumer tests. The public module remains v0.16.0;
these unpublished local candidate results are not a v0.17.0 release.

The first default-three full Windows package run then hits its aggregate
five-minute alarm at 300.591s, whole invocation320.5386733s. The active shutdown
fixture was building for about one second; this is not a five-second drain,
40-second process or two-minute build deadline failure. Its original log and
receipt are retained as software-candidate-r4-full-windows.first-five-minute-
failure.{log,receipt.json}. The one retained test executable matches e32ac6de...
and its owned temporary directory is safely cleaned with a separate receipt.
Only the aggregate package allowance changes from five to eight minutes for a
fresh default-three rerun. Per-item/native/build/pixel/idle/ownership guards and
the existing fifteen-minute CI job allowance are unchanged. Completion of that
rerun, both Metal architectures, exact-source CI/publication and independent
application/module integration remain pending at this entry.

The first eight-minute rerun completes the root's three repetitions in 295.252s,
then fails a real Windows shared coverage-metadata replacement in
testing/winprobe. Its original log/receipt are preserved separately as
software-candidate-r4-full-windows.coverage-replace-failure.{log,receipt.json}.
Packages writing the same coverage directory now run with p=1; race, shuffle,
count3, the eight-minute aggregate and every native/build/pixel guard remain.

The corrected full Windows default-three script passes in 383.1714388s,
root291.625s, with merged root coverage95.1% against the original90% floor.
Strict-cgo/race, vet, all original fuzz cases, AMD64/system-DLL PE checks,
console native smoke and the separate GUI subsystem smoke pass. Both private
TMP directories contain zero entries afterward. The full receipt is
.cache/software-presentation/software-candidate-r4-full-windows-receipt.json,
SHA2566559fbdc81081968794642d35cf5750ee609a907d1e98e1a8a83093343fd1021;
the source stamp stays317456dc7b07bfe8f4359da54bb66bbfbf2a1533bf180b3d51c1fc50d7bc549f.
This supplements the distinct 24-case/72-Run matrix, without rebinding the
earlier failures. Both Metal architectures, exact-source CI/publication and
independent application/public-module integration remain pending. Published
core v0.16.0 and installed application v0.23.0 remain unchanged.
