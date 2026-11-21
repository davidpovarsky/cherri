# RP12 and BRG10: distinct capture and written-order execution

**Evidence status:** the original b810 execution below is retained as history. Its before-binding completeness claim is superseded by the clean 2435236d correction at the end of this report.

Test source: `b81068956a48f3e3a272ae27604eb4a389c29277`. The worktree was clean before and after the actual execution on 21 November 2026. This report preserves the existing 179 requirements and their required tiers.

## Verified local result

The focused registrar command was:

```text
go test ./tools/language-acceptance -run ^Test(CaptureOrder|EditorContract(Rejects|Inventory|Strict)) -json -count=1 -timeout=6m
```

It exited 0 in 40.156 seconds with 126 passing test events, zero failures and zero skips. The count includes the existing 104 editor inventory and rejection controls, exercised after extraction of the common exact-source process helpers. The two fresh nested production processes produced 58 passing RP12 events and 120 passing BRG10 events; neither process had a failed or skipped child. Their measured durations were 14.268 and 10.157 seconds. Each receipt binds the exact source, 84 source files, command, fixed required and observed event inventories, raw stdout and stderr, and measured process interval. Budgets remain the existing three-minute production limit plus one-minute process allowance.

RP12 now executes its written-order unit and native graph scenario independently: reversed, mixed, positional, alias and native calls; once-only producer ownership and argument encoding; duplicate and unknown labels; flow order; default guards and scalar/aggregate argument snapshots. BRG10 independently executes those named effects together with mutable and clipboard captures. Its joined compiled fixture captures before later writes, consumes the saved producers twice, and emits B before A despite declaration order. Eight targeted changes to the joined graph must be rejected, including late reads, wrong binding, duplicate evaluation, scope rebinding and missing saved consumers. Existing capture and terminal-consumer controls also execute under the BRG10 test prefix. Existing passing requirement records are never inputs to either producer.

Twenty additional intake mutations are rejected after the actual baseline is accepted, including foreign packages, missing or skipped required children, wrong source hashes, dirty source, wrong tiers, wrong requirements and detached commands. Relevant mutations reconcile the artifact and process hashes, so merely changing an outer digest cannot hide an omitted production test. An unchanged relocated copy is accepted.

## Acceptance boundary

The run produced four actual local records: unit and native-structure for each requirement. Both runner results are `PENDING_EXTERNAL`; the local manifest report represents the missing iOS runtime tier as two `NOT_RUN` rows. There are zero completely passed requirements and `closure_complete=false`. Final-mode validation rejects this local-only proof. The 179-row audit cannot grant a related ID evidence by association. Historical capture-only runtime traces are not accepted as observations of the complete combined scenario.

These checks inspect actual production analysis and emitted native structures. They do not execute Apple Shortcuts, establish observed Apple effect order, or establish saved values on an iOS device. Those required observations remain open. RP14 typed-capture acceptance is a separate immutable requirement and is not closed by this work.

## Original evidence

[Machine-readable summary](evidence/p15-capture-order-20261121/summary.json) contains the exact contract and artifact digests. [Original archive](evidence/p15-capture-order-20261121/original-evidence.zip) retains the clean driver's raw output, both nested processes, process receipts and resulting evidence records. The earlier dirty development execution is retained under `development-only`; it is not a clean-source acceptance claim. The archive contains no private shortcut corpus.

## Peer-review correction: producer before consumer

A read-only peer review found that the original joined oracle checked B-before-A and the correct A/B UUID bindings without independently requiring both producers to precede the actual call. The inherited C01 structural checks had the same missing producer-before-binding condition. The original 126 outer events and 58/120 nested events remain valid execution history, but they must not be described as proving that missing causal condition. The original archive and summary are unchanged.

Correction source `2435236df4076915613c4f5ae77a36c5a146c3c8` adds explicit marker positions before the sole joined invocation. Separate controls move A after the call, and move both B and A after it while retaining their relative order and UUID ownership; rejection must identify the new causal gate. The shared C01 oracle also requires every actual consumer of a marker UUID to follow its producer, including the function argument Dictionary encoding. Six function/custom forms and the native resize form each move all producers after their consumers and require rejection specifically for that order violation. Duplicate/missing producer identities and missing consumers remain invalid. A second peer review confirmed the narrow code correction before execution completed.

The same six-minute focused registrar command ran on the exact clean correction source, with unchanged process budgets. It exited 0 in 180.563 seconds: 126 PASS, zero FAIL/SKIP. RP12 now requires and observed 65 passing production events (132.286 seconds); BRG10 requires and observed 129 (12.258 seconds). Each receipt still binds 84 exact source files. This fresh run is the current local completeness evidence; it does not retroactively alter the first run.

The outcome remains four local records, zero full requirement PASS, two `PENDING_EXTERNAL` runner results, and `closure_complete=false`. Actual Apple evaluation is still unproved. [Correction summary](evidence/p15-capture-order-20261121/causal-order-correction.json) and [unaltered correction raw evidence](evidence/p15-capture-order-20261121/causal-order-correction.zip) retain the exact source, raw output, fixed inventories, hashes and timings.
