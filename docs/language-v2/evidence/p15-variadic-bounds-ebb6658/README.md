# Canonical variadic bounds - clean follow-up proof

Source `ebb6658ef5a250b3868b259b90dd9399e7d4c4b6` is the separate follow-up to the immutable filter source `09ca92e` and receipt `56f35fa`. It changes the remapping bound from the growing evaluated-argument slice to the fixed canonical parameter definition.

## Observed results

- The actual pre-fix canonical session panics for three position sequences: `[0,0,1]`, `[0,0,0,2]` and `[-1,0,1]`. Four healthy variadic controls and three invalid fixed-parameter rejections pass in the same baseline run.
- The clean fixed source passes all 62 named tests across ten top-level producers. This includes seven variadic position sequences, five native-output corruption controls, the three fixed-parameter rejection controls, all five filter tests and two existing canonical action/variant tests. No test is skipped.
- A Go overlay restoring only the old adapter reproduces exactly the same three bounds failures.
- Four actual CLI invocations compare retained pre-fix and fixed compilers for normal V2 and Legacy variadic source. Both frontends preserve the complete native action graph after normalizing only generated UUID identities. All three literal values retain their count and order.

## Identity

- Compiler SHA-256: `7aec6242e36cc2d66301cbf5a381d447c4713cb70351af7ecfe5714adb255d55`.
- Embedded VCS revision: `ebb6658ef5a250b3868b259b90dd9399e7d4c4b6`; `vcs.modified=false`.
- The source checkout is clean before and after proof.
- [receipt.json](receipt.json) records the executed commands, results and file identities.
- [manifest.json](manifest.json) binds each published evidence file and its original raw capture hash.
- [actual-session-red.json](actual-session-red.json) and its log preserve the real pre-fix failures.
- The earlier [filter receipt](../p15-filter-09ca92e/README.md) preserves the unchanged Linux regression tests and original filter repair evidence.

No source checkout is rewritten by the overlay. Normal parameter lookup, variadic fallback, metadata conversion, action definitions and error messages retain their established paths.

## Scope

Compiler and native serialization only. No signing, external request, Apple runtime execution or formal requirement promotion occurred. The native collect-counts Simulator audit is a separate source and execution receipt.
