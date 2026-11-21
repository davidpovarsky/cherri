# Explicit output in public macro runtime fixtures

The two original macro-fixture test failures in Build & Test 44938761076 came from a mismatched observer: qa-require-v2, qa-measure-v2 and qa-measure-release-v2 all end with an authored output(finalSummary), while p15MeasureActions requires the implicit-Void Nothing boundary.

Source 3a07885ca62db797250b31c43d30cb7c8cc670eb changes only the three explicit-output call sites and adds their strict observer. The existing measure helper and M02 Void observer remain byte-identical. All native rows are retained. The new observer requires one earlier finalSummary Text producer, valid unique UUIDs, every producer before its consumer, and the exact sole terminal output envelope referencing the saved summary. Caller-selected output behavior is not inferred from whether the compiled graph happens to end in Nothing.

The clean recorded replay passed 237 named events in 70.875 seconds, with zero failures or skips. It covers M02, ordinary measure cases, both original failed parents, and four explicit fixture/profile combinations. Each combination rejects implicit-Void classification and eleven corruptions of the actual compiled output. Independent static review found no blocker.

This is local native-structure evidence. It does not establish any actual Apple macro result, and the original failed Linux/macOS workflow remains failed.

[Summary and hashes](evidence/p15-macro-explicit-output-20261121/summary.json) and [original clean replay](evidence/p15-macro-explicit-output-20261121/original-evidence.zip) retain the exact commands, measured interval, source identity and raw events.
