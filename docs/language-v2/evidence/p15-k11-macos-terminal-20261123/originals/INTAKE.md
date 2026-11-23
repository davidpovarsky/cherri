# K11 macOS original terminal intake

**Outcome: FAIL at the unchanged root-package 15-minute cumulative timeout.** This is not an individual assertion failure and does not establish a blocked child.

Frozen source: `9b4135e705b08f765b7304e01212d3b9658806fb`. Build & Test run `45108200176`, attempt1; macOS job `137474256647`. The sole parent observer had already ended terminally. This lane reused its original run/job/artifact metadata and downloaded this failed-job log and the full-suite receipt archive exactly once.

## Exact failure and remaining work

The original log reports `panic: test timed out after 15m0s` and root package FAIL at **900.153 seconds**. At that instant its only named active parent was **TestP15CanonicalBehaviorSourcePropagation (59s)**, specifically **/codec (2s)**. The stack shows p15CanonicalRebuildRead → p15SchemaCommand → Cmd.CombinedOutput/Wait.

The child retained its own original90-second Go test bound and120-second command context; its2-second observed duration is not evidence that either child bound expired. No deadlock is established. The baseline, static mutation/restoration and callback mutation/restoration emitted their original successful-phase hash receipts. Codec and its restoration did not finish before the package alarm.

All1,180 root parents had started;1,169 passed,10 skipped, and this one remained incomplete. All29 opted-in parents had continued;28 completed, leaving no unstarted parallel parent queue. The exact timeout stack and phase-hash lines are retained separately.

## K04 scheduling effect

Both original K04 bodies actually use the existing scheduler in this run, as demonstrated by their PAUSE/CONT events:

- TestP15K04OfflineOracleRejectsBypasses: **PASS92.55s**, including all three original subcases.
- TestP15K04OfflineCacheProduction: **PASS45.27s**.

This establishes scheduling activation and individual completion, **not successful qualification of the whole900-second package**. The retained K10 original also timed out in canonical /codec, then at52s/2s; the two uncontrolled runs do not support a causal global-speedup claim.

## Complete suite receipt and counts

The suite runner invoked all30 discovered packages exactly once and recorded clean source before and after. Ordinary invocation: exit1, **918.709s**. Acceptance invocation: exit0, **1711.823s**; acceptance package summary **1704.855s**.

BOM-before-timestamp-aware parsing, bounded to the two original GO_SUITE_EXECUTE sections, finds:

| Scope | Named PASS | Named SKIP | Named FAIL |
| --- | ---: | ---: | ---: |
| Ordinary | 10,813 | 11 | 0 |
| Acceptance | 5,852 | 3 | 0 |
| Total | 16,665 | 14 | 0 |

There are1,666 parent PASS,13 parent SKIP,14,999 subcase PASS and1 subcase SKIP events. **Zero named FAIL events does not mean success:** the root package terminated by timeout without its final parent result. All other28 ordinary packages completed, including5 packages with no test files.

The original invocation durations sum2630.532s. The suite report timestamp is04:52:52.376203Z, while GitHub records Full Go Test Suite04:09:00–04:58:17Z (2957s) and late console delivery. These clocks remain separate; no cause for the delivery/wall-time discrepancy is inferred.

## Source-local performance assessment

The rounded durations of completed serial root parents sum approximately **474.85s**. The28 completed parallel parents sum **781.14s**; adding the active canonical parent's reported59s gives about840.14s of already-consumed parallel work. Even ideal two-slot packing gives a lower-bound estimate of894.92s before the remaining codec/restoration tail. This is workload arithmetic, not a guarantee or precise scheduler timeline.

Canonical already has the scheduler hook. Its baseline, three actual mutations and three actual restorations are meaningful source-propagation controls. Their private mutation order and all seven real probes should remain intact; this review found no justified probe to omit or replace with a cached claim.

A separate inactive candidate is under independent review: add only the existing first-line hook to TestP15RP08SchemaIdentityProduction (30.62s observed serial) and TestP15SchemaOptionalDeclarationPropagation (21.07s observed serial). Both own private replicas/worktrees and serial mutation sequences. The existing two-slot limit, selected-producer guard, subcase order, source observations, assertions and deadlines would stay unchanged. Potential overlap is a hypothesis; no source edit or test of this candidate has occurred.

## Artifacts and limits

The original log is10,576,046 bytes, SHA256 `3938ab765044742110bee1e509162401902a5cfbcd8d4a9ab9ab5ed2d9e8c1c3`.

Full-suite artifact14790748782 is1098 bytes, SHA256 `2f6ba0b9333ca9f7c1f2be4f3fd07b2986d511aa7d27dbea8164011b8f0cc48f`; its member is6869 bytes, SHA256 `50fcc1ef0b65efbd62d22b41ff10fc1a19b097648173c6188a7e3d2f1ae75712`. The advertised GitHub digest, exact source/run binding and ZIP CRC were verified.

The already-intaken required-encoding archive remains genuine WorkflowKit component evidence and formally ineligible from this failed overall run. No second download was made. Editor-capture artifact14790952964 is advertised in retained metadata but was not needed for this timeout diagnosis and has not been downloaded here. No editor/native acceptance is inferred.

The subsequent macOS P10 gate step did not execute; its release reports were absent. No test, Go command, compiler, provider call, workflow dispatch or new polling loop was executed by this intake.
