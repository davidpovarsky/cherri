# K11 Linux terminal and WorkflowKit component intake

Frozen source: `9b4135e705b08f765b7304e01212d3b9658806fb`. Build & Test run `45108200176`; Linux job `137474256318`.

## Actual Linux outcome

The original complete Go suite passed. All30 discovered packages ran exactly once:29 ordinary packages under the original15m per-package bound and the acceptance package under its original35m bound. The original receipt has clean source before and after both invocations.

BOM-aware, timestamp-stripped, section-bounded parsing of the original console log finds **16,861 PASS,11 SKIP,0 FAIL** named events. This consists of1,667 parent PASS,11 parent SKIP and15,194 subcase PASS. Ordinary:10,836 PASS/8 SKIP; acceptance:6,025 PASS/3 SKIP. There are25 tested package PASS summaries and5 packages with no test files. Exact skip identities and original reasons remain in `intake.json`; these are not acceptance credit.

Original monotonic invocation durations are674.641s ordinary and1346.726s acceptance. Root package reports659.094s; acceptance package reports1342.317s. GitHub records the overall Full Go step03:50:14–04:48:13Z (3479s). The receipt timestamp04:23:56.228983Z and late console delivery timestamps differ. This intake preserves these independent clocks and does not invent a cause.

The sole failed job step is **P10 SSoT, Coverage Ledgers, Pack & Release Gates**. Its22 fixture tests passed in0.848s and all56 registry entries verified. Then **`--verify-commit` failed on four stale dependency fingerprints**, recorded `e29b07449bd5f3758128bbef51466196a14f2179246b48ba1b3df58c75e784f0` versus actual `0c704ae9a25933ba3ece1e4017af0afe922e6b3899fa7886a57ad965b4ca3c97`. The shell stopped before `--run-all`. Thus **this job did not execute actual R23**. The separate known fixed-base R23 blocker is unchanged. The upload warning confirms no release-gate report or attestation was created.

## Actual WorkflowKit component

The source-bound `required-encoding-gate` archive14789316187 contains15 CRC-verified members. All advertised archive/member digests, six input hashes (including the previously retained pinned shortcutkit prelude), three process receipts/argv/streams, the complete per-form matrices, and26 formal-record artifact references were reconciled mechanically with original bytes.

Actual Darwin arm64 WorkflowKit results:23/23 positive required cases; one exact isolated EXPECT corruption correctly exits1 with its precise diagnostic;23/23 restored cases. Positive/restored reports are byte-identical. Canonical expected rejection remains present. This is actual parameter-state engine execution; Shortcut runtime execution is false.

Only CI01 andRP47 negative-ci records are produced. **No formal acceptance is granted**: the same Build & Test workflow contains a terminal failed Linux job and cannot satisfy the existing successful-run requirement (`tools/language-acceptance/main.go:842–846`). No BRG31, general native execution, later-candidate credit, or canonical Go intake is claimed.

## Preservation and method

Exactly one failed Linux job-log download and one download for each relevant original artifact were used. No test, compiler, gate, provider request or workflow was run. No additional remote polling was started. The intake reads local retained originals and exact frozen Git blobs; it does not modify those originals or frozen source. `intake.json`, `source-bindings.json`, and `artifact-download-intake.json` carry the detailed identities and qualifications.
