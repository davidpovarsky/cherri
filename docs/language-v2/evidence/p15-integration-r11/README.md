# P10R15 integration evidence, revision 11

## Exact results

- The immutable compiler ae7f877 / docs e408a29 formal run executed all 179 contracts: 44 passed, 1 failed (C04), 134 not fully verified. Execution audit: 131 UNVERIFIED and 3 BLOCKED_EXTERNAL. Closure is false. The original failed run and hash-bound outputs are retained.
- Integration source 9b032eb passed 87 internal tests, 295 production test cases and 10 formal producer controls. It includes C03 variadic migration, local S01 Unicode proof and bounded T05 presence/null handling. These are not manually added to the frozen 179 results. S01 still needs iOS runtime; T05 still lacks complete native JSON-null semantics.
- The S=a04c6fe / D=e408a29 consumer matrix passed locally with RemoteBuilder BLOCKED_EXTERNAL, plus eight drift mutations and 21 macro compatibility controls.
- Docs d13176c preserves compiler S and 37 snippet checks. Hosted run 44404859290 succeeded after the workflow cache environment moved to the steps that can access runner.temp. Previous run 44401999288 failed before jobs were created.
- The private 38-input replay failed P12/P13; inputs stayed byte-identical and the tree stayed clean. Only its aggregate is included. The old tracked P13 reports were not regenerated because the threshold check failed before the report-writing block.
- Numeric control 087f0c3 has clean local proof and native run 44408371761 was accepted against the immutable staging ref. Runtime result is pending. The immediate run-list indexing check failed after successful dispatch; the later direct run read verified the exact source without redispatch.

All changes stay on staging/agent branches. No main promotion or release completion is claimed. Normalized archive files are explicitly labeled; original and archived hashes appear in archive-index.json.
