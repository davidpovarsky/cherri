# S2 Build & Test: failure preserved, precise remaining oracle identified

[Run 44452839062](https://github.com/davidpovarsky/cherri/actions/runs/44452839062) ran exact source 7b0100a2a9ce5991ca8ffe82c805278e5aa7754e. The overall result is **FAILURE**. Linux and macOS both failed the full Go step; Nix passed. Later success-dependent matrix/discovery/release steps were skipped, so this archive does not establish a complete CI pass.

The only failed leaf in each complete host log is **TestLANG002_EnumAndTypeAliases/ValidEnum** (its parent is also reported failed). At metadata_matrix_test.go:273 the old test expects a raw PRODUCTION string in Quick Look WFInput but the shared scalar codec now supplies a Text ActionOutput attachment. InvalidEnum passes on both hosts. A clean local replay of exact S2 independently reproduces that same failure; its JSONL and stderr are retained.

The canonical quicklook declaration uses variable input WFInput, and the generated schema specifies codec token with attachment(output)/attachment(variable) accepted forms. A valid consumer repair must check the exact PRODUCTION Text producer, unique preceding UUID edge, producer cardinality and ordinary literal parity. It must not accept an arbitrary nonempty reference or remove enum negatives. This archive preserves the original failing test/log; repair and verification belong to a separate source commit.

## Actual macOS K01 improvement

The preceding b34 run had failed its unpacked actual self-test. On S2 the following now pass in the actual macOS job:

- TestP15K01UnpackedPackageContract: 31.02 seconds, including all twelve named nested controls.
- unpacked_actual_self_test: 10.94 seconds, with the original test's final behavior preserved.
- TestSkillPackageProductionEvidence: 24.81 seconds.

This is fresh macOS evidence for the TMPDIR portability repair at exact S2. It does not turn the overall failed build green or transfer the result to another source.

The mandatory real WorkflowKit REQUIRED encoding step also passed on this run. Its original phase files, process triplet, exact source hashes and independent strict-intake proof are archived separately in ../required-encoding-7b0100a-44452839062.

All raw logs and metadata remain byte-preserved; the manifest binds each Git blob. No test timeout/panic was observed in either complete job log.
