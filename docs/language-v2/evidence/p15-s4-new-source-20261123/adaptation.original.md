# Inactive new-source S4 independent intake

This is a single read-only audit proposal for all five actual new-source executions. It is not a producer, watcher, precompile, replay or acceptance-contract change. It remains inactive until root reviews the exact adapter hash and authorizes its explicit --execute-reviewed-intake flag. Default invocation returns S4_INTAKE_INACTIVE before reading private inputs.

Source is fixed to 57b6d919757e1d0262993c6f11c6dc1cd17d6177/tree370d85f75d8fb5bba5f9c8afe438208d97a525c8 and its ordinary clone. Actual run root is s57b6d919-replay-01. The approved replay spec474dcf80… and owner/replay code9487c34d…/2bffb8d5… are bound exactly.

## Retained original controls

The four-parent strict core is copied verbatim from audit-remaining-gate.py lines19–50 and65–66, apart from one function indentation level. The lifecycle strict core is copied verbatim from audit-replay.py lines15–35 and45–46. The retained fragments and strict-core-preservation.json bind their exact bytes. A static removal-of-indentation comparison independently reconstructs both originals. Neither original auditor is imported or modified.

The adapter preserves all exact event accounting: parse failures, once-started/once-terminal cases, no orphan/incomplete/duplicate events, one parent run/pass, no fail/skip, one package PASS, no Go timeout or gated NOT_RUN. It preserves the original four historical summary expectations (38 files,27,652 actions,23,353 natural,4,298 fallback,1 invalid,0 demotions where applicable; thirdParty>0) and exact38 per-input P10 summary names checked privately. Lifecycle still requires one reconciliation,38 files,fresh/sequential equality,empty differences,one demotion summary and all original event controls.

Those historical exact summary values are stricter than some current Go parent minima. They are deliberately not relaxed. If new actual output differs, its numeric summary remains in the separate new result, summaryVerified becomes false, aggregate new-source acceptance is false and the audit exits1. Root reviews the cause; there is no retry or changed expectation for green.

## Precise changes to the old audit context

| Old audit assumption | New binding |
| --- | --- |
| Hard-coded R7 and retired worktree | Exact S57b6/tree/ordinary clone; clean source before and after; five exact current test-file hashes |
| Prior r7-replay intake and old adapter | New approved spec hash, exact submitted new replay bytes, per-mode actual original receipt/started/stdout/stderr |
| Earlier R6/R7 streams and precompile used as acceptance inputs | No new-source credit from those historical observations; original historical artifacts remain untouched |
| Old precompile receipt required and compilePreparationReused=true | Explicit false/no separate compile-only prerequisite; current compilation evidence is the five actual source-bound go test executions |
| Four-mode wrapper context implicit | Actual owner receipt,970s/900s/930s,once-mode identity,stream hashes and child-output/receipt equality |
| Lifecycle has no four-mode owner | Preserve direct930s route and reject a claimed lifecycle970s owner |
| Each old mode rehashes the whole corpus | One complete audit after all five terminal receipts; exactly one in-place hash pass over the same38 manifest-bound files |

The configured Go1.24.13 is bound through the exact reviewed replay source. The auditor invokes no Go command and does not claim a separately observed go version. Previous archive preservation proof remains historical; this audit does not relabel or reissue it.

## Authorized read scope proposed for root handoff

After all five receipts exist, the adapter verifies the source and reviewed spec/code. It creates one exclusive independent-intake-01 claim, then hashes only the already-pinned private manifest and its same38 input files (62,980,131 total bytes). It reads no plist semantics, modifies no input, and publishes no corpus names, contents or diagnostic source snippets. Names are used only in memory for exact inventory and P10 per-input identity comparison.

It parses only the five original Go streams and original producer/owner metadata, never a live process. All original hashes are verified before summary/event interpretation. No polling loop, network, compiler, test, native command, provider, signing, regeneration, branch or source mutation exists in the adapter.

Current source inspection uses only fixed read-only Git commands with a sanitized environment and replacement objects disabled. The five full test files are byte-identical to their historical R7 counterparts, but every new receipt is attributed to actual S57b6; this is preservation of assertions, not reuse of execution credit.

## Output and bounds

Proposed one command, still disabled:

    timeout --signal=TERM --kill-after=5s 60s python3 -B audit_new_source_s4.py --execute-reviewed-intake

The60s bound is only for this new read-only intake, not a test or producer deadline. All original Go900/driver930/four-mode-owner970 bounds are unchanged. Root should retain the adapter's original stdout/stderr, exit, actual duration and code hash in an external supervisor receipt.

The adapter exclusively creates:

- s57b6d919-replay-01/independent-intake-01/claim.original.json
- s57b6d919-replay-01/independent-intake-01/result.original.json
- rejection.original.json only if a concrete check raises

A completed result contains only source/test IDs, fixed booleans, numeric summaries/counts, public-safe paths for tracked test source, timings and digests. It explicitly states r7ExecutionCreditReused=false,compilePreparationReused=false,producerExecutionsByThisAuditor=0 and aggregateFormal179AcceptanceGranted=false. Five-parent acceptance is separate from formal179.

All original old auditors, source, input manifest, inputs, receipts and streams remain unchanged. No adapter invocation, private-input read, producer or control suite has occurred in this preparation. Only AST syntax parsing and exact source/core comparisons were performed.
