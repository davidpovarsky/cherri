# P15 K05: verified existing-Shortcut editing

K05 requires the production repair/build/import/edit/sign workflow and an
allowed-delta gate before signing. The implementation uses the existing V2
compiler, hybrid importer, plist codec and public signing command.

## Verified boundary

The default edit path accepts an XML or binary workflow plist, prepares a new or
empty workspace, imports through `cherri import --mode=v2 --fidelity=hybrid`,
compiles the generated source and requires exact decoded-document preservation.
The native V2 provenance spelling is
`native.workflow(native.plist(...), exact: true)`; explicit Legacy import
remains available through `decompile.sh --mode=legacy` and the public Legacy CLI.

`prepare-edit.sh` preserves the caller input and refuses to reuse an occupied
workspace. `build.sh` compiles to a temporary directory beside the requested
output, retains any previous output while validation/signing runs, and promotes
the completed result only after success. Source/output and original/output
aliases are rejected. An explicit unsigned output writes only the named file;
an unrelated existing `SOURCE_unsigned.shortcut` is preserved.

The new thin `cherri verify-edit` command reuses
`internal/shortcutcompare` and the existing plist decoder. It compares every
decoded field and type, including metadata, identities, reference payloads,
Data, Boolean false, integer precision and real-number bit patterns. Existing
parity comparison and alpha-renaming behavior are unchanged. A parity match
cannot grant an edit authorization.

An allowed-delta policy binds the original file's SHA-256 and explicitly names
existing scalar fields, their original values and the requested replacement
values. No wildcard, path prefix, scalar type conversion, duplicate field/path,
missing original value, no-op replacement or broad subtree replacement grants
authorization. XML/binary serialization may differ; the decoded document after
the listed changes must match exactly. JSON companions are not workflow input to
this gate.

This first policy version supports existing scalar replacements. Action
insertion/deletion, array resizing and aggregate replacement remain unsupported
and fail closed. The K05 proof covers the one-field edit required by RP37; it
does not claim every possible edit operation or close other repair/recovery IDs.

## Signing order

The wrapper invokes the existing public `cherri sign` on the same unsigned file
that passed the gate. It does not compile again between comparison and signing.
The existing configured signing-service selection, synthetic-data/privacy
checks and AEA1 validation remain in force.

Prepared workspaces identify their immutable original automatically. Signing
their edited source requires an explicit policy. An imported source copied
elsewhere must also name the original explicitly. A missing comparison
entrypoint, invalid policy or any unlisted change stops finalization before a
signing request. New authored workflows continue to use the ordinary build
wrapper. For a signed build without an original, the wrapper passes
`--require-authored-workflow` into the same V2 compilation. Its canonical
session rejects actual `native.workflow` provenance after lowering and before
finalization, including provenance loaded by a called module function. Comments,
newlines and grouped names cannot change this check. Both Legacy CLI entry
forms reject this V2-only guard explicitly. This is a workflow-origin guard
over retained provenance; it cannot reconstruct an original that was removed
from the source.

A successful remote service response and container validation are
**signing-service evidence**. They do not establish Apple signature validity,
importability, installed-consumer behavior or Shortcut runtime results.

## Actual execution producers

- `TestP15K05BuildPreservesExistingFiles`: actual wrapper source/output alias,
  failed compilation with a previous delivery, healthy unsigned control,
  unchanged unrelated sibling output and both rejected Legacy guard routes.
- `TestP15K05PreparePreservesOriginal`: real preparation and an attempted second
  original in the same workspace.
- `TestP15K05RepairBuildImportEdit`: binary plist import, V2 diagnostic and
  repair, one authorized Text-field change, full native document comparison and
  unchanged original bytes. The fixture includes Data, false and an integer
  greater than 2^53.
- `TestP15K05SigningGateAndExactInput`: absent policy, removed comparator,
  unauthorized extra action, metadata removal, reference damage and an actual
  loopback HTTP signing failure. Captured request bytes must equal the gated
  file. These controlled service failures never count as successful signing.
- `TestP15K05PrepareRequiresComparator`: a removed comparison entrypoint blocks
  preparation and retains the input/workspace.
- `TestP15K05CopiedImportRequiresPolicy`: actual compile controls for copied
  provenance under a leading block comment, split tokens, grouped native target
  and a called local module. Every imported form must be blocked before any
  signing request; authored text, comment and local-module controls still reach
  the controlled rejected signer.
- `TestP15K05LiveSigning`: opt-in real configured service execution, exact clean
  compiler VCS identity, source/original/gated-input/request-input/signed hashes,
  validation tier and retained synthetic artifacts. A temporary clean normal
  clone at the immutable source commit supplies a VCS-stamped binary; the test
  keeps the exact revision/clean-tree guard on linked-worktree hosts.

The shared comparator tests independently reject historical parity relaxations
and ambiguous policies. `TestSkillEditSkippedLiveTestIsNotEvidence` executes the
real live test with its opt-in flag absent and proves that the producer rejects
the skip as missing evidence.

## Source-bound execution

Default Go tests perform no implicit external signing. A dedicated live run
requires `CHERRI_K05_LIVE_SIGNING=1`. The formal K05 producer remains
`PENDING_EXTERNAL` with local observations when that flag is absent. Only the
dedicated live producer may emit K05 integration PASS, after all named local
controls and the actual service run pass.

Use `CHERRI_K05_EVIDENCE_DIR` to retain synthetic originals, source, policy,
unsigned/sign-input/signed artifacts, comparison receipt and signing manifest.
The test refuses to overwrite an existing evidence file. Capture the raw Go
JSON stream and concatenate Output fragments for the selected test before
parsing its receipt.

A clean implementation commit and its actual execution receipt are recorded in
a separate evidence follow-up. Consumer compiler/docs/Skill pins must include
the new CLI entrypoint before final packaged-consumer verification; old
manifest-pinned compilers must not be presented as supporting this wrapper.

## Causal baseline

On unchanged production source fb93cfb1dae4ca3130a744e16bebcc67a8ac9b20:

1. Passing the source path as build output deleted the source.
2. An invalid source deleted the previous requested delivery.
3. Preparing another input into an occupied workspace overwrote its original
   copy.
4. The old semantic import wrapper emitted source without native provenance;
   recompilation regenerated producer/reference UUIDs. Its optional parity
   comparison accepted those changes and missing comparison support.

The healthy baseline build and first preparation succeeded. The recorded RED
observations and later GREEN executions are kept separate from immutable
source-bound receipts. No private corpus inputs are used.

## Follow-up causal findings

The first clean `9e6d661` formal attempt passed its local workflow tests but
stopped before signing: the linked-worktree compiler lacked Go VCS fields.
Its actual failure is retained; no signing-service outcome is inferred from
that attempt. The normal-clone build recipe follows the existing K01 producer.

Independent review then identified a shell-regex provenance bypass. Actual
production tests confirmed that a same-line comment, split native member tokens
or a grouped native target compiled and reached a loopback signer without an
original/policy. The canonical session guard fixes this route. A called imported
module function is covered by the same guard, while authored controls pass.

A separate actual test confirmed that the existing public unsigned compiler
overwrote `SOURCE_unsigned.shortcut` even when given an explicit staged output.
The explicit-output path now writes only its requested destination. Review also
found the ordinary filename Legacy route ignored the guard; its actual causal
RED and the healthy explicit Legacy rejection control precede the shared
Legacy-entry fix. None of these loopback or pre-request failures is recorded as
successful remote signing.
