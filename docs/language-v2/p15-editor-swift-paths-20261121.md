# E05/E06 Swift execution path identity — 21 November 2026

## Verified result and remaining scope

The owned-directory path correction passed an actual Windows Go/Swift replay at
254313031edf3cad9fd78d97c265eb06a21fd392: **9 outer Go tests passed, no failures
or skips**, in 125.531 seconds, with a clean source tree before and after.
The nested production inventories were E05 mobile **20**, E06 LSP **33**, E06
mobile **35**, Swift publication **26 controls**, and Foundation position
**17 controls**. All 107 recorded source files were reconciled with the tested
Git blobs; raw output, generated Swift and executable hashes matched.

E05 remains PENDING_EXTERNAL with only its service tier verified. E06 remains
PENDING_EXTERNAL with only its protocol tier verified. Neither record passes
the required editor UI tier. This is Windows Swift evidence; an actual macOS CI
replay on the final integrated source is still required.

## Original failure and causal observation

Original Build & Test run
[44938761076](https://github.com/davidpovarsky/cherri/actions/runs/44938761076),
macOS job 136951106592, failed E05/E06 intake at
editor_swift_intake.py:106: "detached Swift compiler/executable command".
The retained original log is 5,608,167 bytes, SHA-256
d0678188eff54024022ab0da333f5231ff6e12255ba8837574cc1f112fc9e457.
The available artifact inventory contained only suite summaries and the encoding
gate, so the original individual Swift command tuples cannot be recovered.
The common macOS /var alias is a plausible explanation, not an observed original
tuple.

A causal replay at clean source
b187d93fb74a2e27636a0459abe56a0d0afb24a4 invoked both real Swift probes through
owned Windows directory junctions. Both compiled and executed successfully.
The unchanged intake rejected the aliased expected output directory with the
same exact-command error, accepted the physical directory against the same
untouched receipts and binaries, and rejected an aliased expected repository.
Both junction entries were removed after observation; their physical evidence
remains retained.

The first correction,
45360fa8f2552f06830498a7ceda073a0a2bacf6, used Go filepath.EvalSymlinks.
Its actual replay failed the alias setup assertion before E05/E06 execution
because that operation retained the Windows junction spelling. This RED is
preserved; it is not a Swift compiler failure or a passing correction.

## Correction and strict comparison

The final correction at 254313031edf3cad9fd78d97c265eb06a21fd392 resolves only
the two existing absolute owned directories before invoking any producer. It
uses the producers' Python Path.resolve(strict=True) operation under a
30-second context and bounded output, then independently requires os.SameFile
for each original/resolved directory. The physical paths are used in the actual
invocation and its outer receipt.

The file editor_swift_intake.py is byte-identical to the original source (SHA-256
11365ad99ed4b763b6fc855fc1ac5926505fb7e4887f65333b3379db641dbc9b).
Archived commands are not rewritten or normalized. Source, generated file,
binary, raw output, fixed control inventory and execution identity checks remain
strict. The actual E05 and E06 test each rejected three changes to the expected
tuple: original alias, foreign directory and foreign repository.

The clean replay command was:

    go test ./tools/language-acceptance -run ^TestEditorContractCanonicalSwiftExecutionPaths$ -json -count=1 -timeout=6m

Outer JSONL SHA-256:
d7a927da20caf39570bc66e38a3cf623db693c59d7affbe4a6cd355665c8b805.

## Public CI evidence retention

The macOS full-suite step now retains public editor probe originals and uploads
a bounded ZIP plus inventory even if the suite fails. The collector accepts only
the exact E01/E02/E03/E05/E06 probe paths, rejects foreign or linked paths and
uses fixed limits of 128 files, 8 MiB per file and 32 MiB total. Reads are bounded
and must match the observed file size. It emits no environment dump and grants no
formal acceptance.

The first capture of the successful replay failed closed on an actual Windows
Swift .exp sidecar. Commit d21d3bede6853c3034708c0032aae2a793e12671 adds only
the observed .exp/.lib names for the two known probe stems. Its **8 collector
controls passed**, including wrong stem/extension, foreign path, linked directory,
oversize and growing-file rejection. The collector then retained **34 unchanged
original files, 586,423 bytes** from the successful replay. Every archived byte
was compared to the original. The first post-capture audit wrapper incorrectly
expected 33 files; its subsequent independent inventory comparison is recorded
explicitly in the capture summary and required no repeated execution.

The local capture used run ID 0 solely as test context, not as a GitHub run.
Its archive is 166,572 bytes, SHA-256
bc265feae11a7fc168a9f413d749e3fb3d946e89b8f0a981a35235f6d3b02562.
The workflow test commands and existing execution budgets are unchanged.

## Retained proof

[Manifest](evidence/p15-editor-swift-paths-20261121/manifest.json),
[reconciliation](evidence/p15-editor-swift-paths-20261121/reconciliation.json),
and [original evidence archive](evidence/p15-editor-swift-paths-20261121/original-evidence.zip)
retain the original alias observation, both repair attempts, actual receipts and
binaries, the capture failure and correction, and the original macOS error
excerpt. The archive contains 54 entries, 301,671 bytes, SHA-256
fd950616a3739013c8f0e1c4145019a623663998a1cb96127f47dd639ef99b32.

The three immutable acceptance contracts remain byte-identical. This change
does not close E05/E06, the macOS CI failure, or formal 179 acceptance without
their separately required final-source evidence.
