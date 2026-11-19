# F04 List-valued collect: native cardinality failure

**FAIL_RUNTIME_PRESERVED**. This is a real correlated target callback with an incorrect semantic result, not a missing callback or a queue-only observation.

- Source: `ff04ee8887c701824e5161f3fff1431ff204bb65`.
- Run: https://github.com/davidpovarsky/cherri/actions/runs/44431486363
- Fixture: `qa-f04-yield-list-v2`; 109 native actions.
- Target callback: 2026-11-19T01:38:01.086Z.
- Expected: `CHERRI_QA_F04_LIST_RESULT:pairs=2:scalars=2:shape=LEFT,TAIL|RIGHT,TAIL`.
- Observed: `CHERRI_QA_F04_LIST_RESULT:pairs=4:scalars=2:shape=COUNT_MISMATCH`.
- The exact signed artifact SHA256 is `ddbc45f5e434b94132d2c97ba16777edc0f92fdc446f5f8a1c21b9ea04976788`.

The two scalar iterations produced the expected count of two. Two List-valued iterations produced outer count four rather than two nested List values. This disproves the promised nested cardinality for this source and fixture, consistent with Repeat Results flattening. The callback deliberately reports COUNT_MISMATCH without pretending to verify the indexed shape once the outer count is wrong. Native item types and a general alternative nesting mechanism are not established.

The independent auditor verified the immutable source, fixture, signing and callback correlation, replayed the original exact oracle, and verified all 20 published file hashes. It exited 2 and retained FAIL_RUNTIME_PRESERVED. Its causal success-oracle mutation controls were intentionally not run because the baseline is not a semantic PASS. The strict oracle and original failure are unchanged.

The publication contains synthetic text only; all 13 raster/raw entries were quarantined. The full editor oracle remains NEEDS_REVIEW, and no physical device test occurred. Existing artifact fields such as compiledShortcutRuntimeExecuted=false reflect the failed strict runtime contract; the separately recorded identity-verified SUCCESS callback is preserved verbatim and must not be erased.

F04 stays incomplete. A compiler fix must demonstrate faithful nested List values and empty Lists with native evidence. Dictionary wrapping under a List<List> type or accepting count four as count two would not satisfy this requirement. The paired empty-List run 44431490222 was still queued when this receipt was written.
