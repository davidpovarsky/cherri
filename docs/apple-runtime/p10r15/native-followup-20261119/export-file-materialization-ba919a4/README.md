# debugExport File materialization: source and model repair

The compiler source repair is `ba919a4a7523da0183a58bcde649a047d0e52234`.
These receipts establish source/graph and bounded model correctness only. They do
not establish Apple execution of the production export macro. The final combined
source must still complete the exact signed production export probe.

## Why this repair is needed

The old production macro passed Text directly to Get File of Type. An earlier
native control returned a runtime error, while the local model had assumed that
operation performed a byte-preserving conversion. That unsupported model branch
has been removed. The repaired production boundary uses completed envelope Text
→ Base64 Encode with no line breaks → Base64 Decode → Set Name. Redaction occurs
before this boundary. The output remains one newly produced File and no save,
share, or network operation is introduced.

The separate native literal-File control at run 44429796917 / source 0c286b6
passed exact name plus independent File Extension, File-witness type equality,
Base64 bytes and Get Text content. See the adjacent
`file-extension-0c286b6-44429796917` archive. It supports the corrected filename
oracle and literal Decode File boundary, not production debugExport acceptance.
The original Get Name-only failure at run 44401954515 remains preserved.

## Causal baseline and clean repair verification

The original source e8217da plus only the new materialization regression test
failed all eight intended cases: both frontends, JSON/text, redaction on/off.
The exact pre-gofmt test bytes, failing log, command and hash receipt are retained.
No production source was changed during that baseline.

On clean ba919a4, the focused Go gate passed 12 top-level tests and 36 subtests,
including all eight new materialization cases. It checks exact producer UUID
links and Encode/Decode ordering, default payload privacy, original-value and
returned-File identity, both frontends, and bounded 0/1/1000 compile expansion.
The two Python suites passed 22 tests, including exact immutable nested export
source paths and causal source-path substitutions rejected by the auditor.

The actual CLI producer built the compiler and passed 16 configurations with
exact native-action roundtrips, 80 independent bounded model cases, 16 invalid
CLI options, seven causal graph faults, and all 12 export probe compiles and
lossless roundtrips. Native executions: zero. The report intentionally retains
`PASS_SOURCE_AND_MODEL_ONLY` and `PARTIAL_SOURCE_IMPLEMENTED_PENDING_NATIVE`.

All twelve production export probes now check File type against an independently
decoded native File witness and reconstruct the complete filename using Get Name
plus File Extension. They retain the original semantic success markers, exact
payload/cardinality checks and independent original-value preservation checks.
No localized native type spelling is assumed.

## Reproducibility and scope

The validation receipt records source SHA, clean worktree before/after, exact
commands, exit codes, durations and original log SHA256 values. The producer
records source/schema/macro fingerprints and the actual compiler binary digest
`8a370bcba7d6b498ba503cee7e2663b41a89d0c0e8636dc551a25ff9cc583df5`.
This diagnostic Windows worktree build uses external clean-source receipts; it
is not asserted to be the embedded-VCS packaged release runtime.

The inventory preserves exact raw log bytes. Evidence uses only reviewed
synthetic fixtures. There is no physical-device, full-editor, exported-file
provider persistence, all-native-export-scenarios, or project completion claim.
