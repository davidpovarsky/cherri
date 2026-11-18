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
aliases are rejected.

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
wrapper.

A successful remote service response and container validation are
**signing-service evidence**. They do not establish Apple signature validity,
importability, installed-consumer behavior or Shortcut runtime results.

## Actual execution producers

- `TestP15K05BuildPreservesExistingFiles`: actual wrapper source/output alias,
  failed compilation with a previous delivery, and healthy unsigned control.
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
- `TestP15K05LiveSigning`: opt-in real configured service execution, exact clean
  compiler VCS identity, source/original/gated-input/request-input/signed hashes,
  validation tier and retained synthetic artifacts.

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
