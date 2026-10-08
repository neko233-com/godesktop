# Exact native timeout receipt consumer

2026-10-08 the consumer's initial implementation/validation was CPU-only; Root's
later actual r4 CLI proof is recorded separately below. Public core remains
v0.16.0 and this working candidate is unpublished. This reviewer executed no GPU,
native window or device probe. Root owns source CI and native acceptance.

The private internal/dx12timeout command's schema1 producer reports26 required
scalar fields. The consumer now validates the complete object before typed
decoding: every exact-case field must occur once, null is rejected, and missing,
duplicate, unknown or trailing input fails. Typed decoding retains uint64 fence
values exactly and rejects scalar type/overflow/fraction corruption. Existing
16KiB stdout,128KiB stderr, owned child-PID,40s process/5s pipe limits and actual
timeout/cancellation gates are unchanged. The validation schema derives its
names from the typed report, so adding a field requires actual producer output.

The tracked fixture testdata/timeout-native-20261008.json is a byte-exact copy of
.cache/software-presentation/software-candidate-timeout-native.log, SHA256
9250bc32d5d9e4540c8786a8c606afa1d22b768d564fe76b1a4ce461e238b3f2.
These bytes came from the earlier actual child227032, submitted fence3 pending
at2, original drain5000ms, MAX18446744073709551615, zero counted completion,
terminal rejection0ms and destruction15ms. The test checks the immutable digest
and replays those values through the new consumer. That replay proves parsing;
it cannot substitute for executing a newly built native CLI. The original binary
and raw native result remain separate historical evidence.

Tests remove, null and duplicate each field of those actual producer bytes. A
small corruption table rejects negative/overflow/fraction/string fences, wrong
boolean/string types, field case aliases and malformed/successful/overflow
HRESULT reasons. Existing unknown/trailing/PID-replay/output-limit negatives
remain. Bounded output still drains child pipes after reaching its storage cap.

The focused CGO=0 count3/shuffle test passed in0.031s. The complete parent command
used GOWORK=off, CGO_ENABLED=0, Windows AMD64v1, count3, shuffle=on and p=1;
go test -v -timeout=5m ./... exited0 in19.1209128s. The root package took8.157s,
dx12timeout0.033s, and full go vet ./... exited0 in0.9860844s. Native windows&&cgo
wrappers were excluded; this suite started no GPU/UI work. Actual-source scalar
footprint admission and all shadow CPU models passed each repetition. The
shadow corpus reports Gaussian1088/sharp816, no fallback samples, zero failures
and HLSL/Metal numeric disagreement0; those CPU oracles are not native Metal proof.
The exclusive owned TMP was empty before and after all commands.

The fixed full log is .cache/software-candidate-final-nocgo.log, SHA256
136caf665d8e2bca0bb43fb8554f6faf08c16a5eaf367b1db07d70fb4fe70380.
An earlier fixed log, if present, is preserved as
.cache/software-candidate-final-nocgo.pre-exact-schema.log.snapshot. Detailed
validation/vet and source manifests are in .cache/dx12timeout-consumer-current/.
source-pre.json records156 source inputs before edits; source-post.json records
157 afterward, including the new immutable receipt. Their comparison shows only
the two consumer Go files and that receipt changed. All other parent sources,
including actual Device/Surface/bridge/shaders/producer, stayed byte-identical.
Historical driver controls, compile failures and native successes are retained
in windows-device-removal.md and their original cache records.

Final main.go SHA256 is
efe48e71c470e261be830a40ef0c2b57748039932df12c01b8ff76493e7abaf4;
main_test.go SHA256 is
bdd077a5e7f1bda5f3ca726e56bc255328cdb7b30bb9d885c30e3240caa6e28e.
The frozen native producer header remains
4335d5564c5c4e73518f0750072ef6b8f4795ebd01a890721a59d1e33cc10fa5;
Device remains6fa93ada75f5fbecdf3e2f0563b7dc102a07e8e4f264be710dc70ac73dead837.
No native guard, version, published tag, workflow or public ABI changed here.

## Final strict-cgo consumer validation, no GPU execution

The same frozen consumer/test/receipt bytes subsequently passed GOWORK=off,
GOEXPERIMENT=cgocheck2, CGO_ENABLED=1, Windows AMD64v1 focused race/count3/shuffle
tests. The package reports1.126s (command wall8.7110852s), shuffle seed
1791418674202882100. Only the decoder tests ran; none calls nativeProbe or the
command entry point. Thus compiling/linking the real producer adds no new native
execution proof. Focused vet exited0 in4.6855182s, and race/buildvcs=false build
exited0 in5.0765559s. Raw vet/build logs are empty because neither emitted a
diagnostic; validation.json records their actual exit codes and timings.

The new .cache/dx12timeout-consumer-final/check.exe is build-only, SHA256
2af327c541c8c6b52125ae68b1f345ef1001ad530e016c43a335c5a555d0898f.
Read-only Go build-info confirms race, cgocheck2, CGO=1 and AMD64v1. The focused
log SHA256 is
7af51cb4eefd7c94acdcfa5ac2c63daaa1e15254ea4b6ed4c18eb72e3d64a846.
Exact source-pre/source-post manifests cover157 parent source/metadata/receipt
inputs and compare with zero changes; the private TMP is empty. No CPP, bridge,
UI or consumer source changed during these commands. The first read-only
post-manifest shell capture missed a foreach closing brace, emitted no output
and wrote no manifest; its failure record is retained. Corrected capture and
comparison succeeded without repeating GPU work or changing any source.

This final directory's validation.json explicitly has nativeExecuted=false.
The historical child227032 native receipt and Root's separate r3 window checks
remain their original real executions; replay and the unexecuted new CLI never
inherit those executions. Source CI and publication remain Root-owned.

## Root's subsequent actual r4 strict-consumer CLI proof

Root subsequently ran the new strict-consumer CLI binary
.cache/software-presentation/software-candidate-r4-timeout.exe, SHA256
4c9372134a68153e4d04f9dc945c072c8a9c3955ceea4b59da635906d1f293fb.
Its own child225620 used the actual Device/pipeline/offscreen resources: an
initial successful drain preceded holding the queue, submitted fence3 remained
pending at2, and the original bounded drain failed after5000ms. The actual
removed fence was18446744073709551615 with Completed0/Submitted1. Subsequent
drain/wait/signal/submit all rejected, first error stayed unchanged, and terminal,
retirement and destructor durations were0ms. The producer reported pass=true
and the strict consumer accepted the complete typed receipt in that actual run.
Raw evidence is .cache/software-presentation/software-candidate-r4-timeout-native.log,
SHA2560b5e7563b65782214a72d792723d4fd89d6f60ecc2549ec5fd98f4dd093d180f.
This is real offscreen device-timeout proof; it creates no Window and does not
substitute for the separate r3 held-fifth-frame Window shutdown/error return.

The exact r4 source manifest is
.cache/software-presentation/software-candidate-r4-source.json, SHA256
317456dc7b07bfe8f4359da54bb66bbfbf2a1533bf180b3d51c1fc50d7bc549f.
It binds157 inputs including probe_windows.cpp
(5e5ac8af258c5b4c8bb4a098382f97879012d6dc4ff2d8df1b0d6434e0932f1e),
the frozen native header/Device and the unchanged strict consumer efe48e71.
Earlier r3/pre-r4 preparation stamping omitted that CLI translation unit. The
explicit additional binding repairs the inventory; the original preparation
snapshot is preserved as software-candidate-r4-source.before-probe-binding.snapshot.json
with native_executed=false. The final source stamp also describes preparation,
so its native_executed=false does not erase later actual matrix/CLI executions.

Root's independent r4 Shadow matrix passed24 native invocations/72 Window Runs
in99.0928186s, covering ordinary/recovery, debug settings0/1, hardware/WARP adapter
policies and diagnostic densities1/1.5/2. Its actual native result has
NativeExecuted=true and Published=false; source hashes stayed unchanged and all
owned case PIDs were absent. This reviewer read records only. The new matrix is
separate from this CLI and from the earlier CPU-only validation.json/check.exe.
The complete default-three Windows suite is still pending in Root's exclusive
run; local results cover neither Mac native execution nor exact-source CI.
Public core17 promotion remains gated on those remaining validations.
