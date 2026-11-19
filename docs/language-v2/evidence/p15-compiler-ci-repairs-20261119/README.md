# Compiler CI repairs — 19 November 2026

## Verified source

Implementation commit: `ba61a8df7f9fe040ab17689083101871f425b102`.

The bounded root-package gate passed 280 named test events, zero failures, on a clean tree before and after execution. It ran the complete 505-action parameter-contract matrix, every statistic serialization/semantic/hybrid roundtrip case, V02 analysis and actual service/CLI checks, the null-diagnostic corruption controls, required variadic presence, all C03 regressions, and an unconsumed typed native List import graph. The command and raw SHA-256 are in `compiler-repairs-summary.json`. The raw log SHA-256 is `04a190ba03305c593ef763653b71a6599182bd57fb91318e09cdbe809fd7c9e8`.

The entire migration package passed 18 named tests on the preceding local candidate `ac255f91dd4ce478217790b6ab8d4771cc15950b`. Its production code is identical to the final implementation; subsequent changes only corrected a first-error API expectation and added the unconsumed-list graph test. That intermediate commit was amended before publication. The raw log is retained, and the integrated repository gate must still run on its final source SHA.

## Causal baseline

The unchanged production source `1ca19227f5fc04cbe8289ad81f16a7d9db9bea34` reproduced the same CI defects on Windows: 68 named passes, 7 named failures (including parent tests), exit 1. Runtime was 235.29 seconds. Its raw JSON-lines SHA-256 is `ce06959bcb18d5b78d4148dbc655baedf7fbbcec46ed85431f42ba1b61ac8ee2`. The source and tree were clean. The original baseline files remain under the owner's project evidence directory `cherri-p10r15-evidence-20261119/ci-repairs`; no private Shortcut corpus was read by these tests.

## Repairs and preserved contracts

- Required variadic arguments are no longer exempt from requiredness. `url()` and the four other required variadic constructors fail with `E_MISSING_ARGUMENT`; a supplied named `[]` remains a valid empty pack. Expansion carries empty-pack presence to analysis without inventing an element. Optional packs retain omission behavior, and the existing zero/one/many-item C03 wire assertions still pass.
- Semantic import reconstructs native List contents as a List expression through the existing migration AST walker. Number output references remain Number producers rather than being passed to the text-only authored `list(...)` action. Source strings, lexical shadowing, and ordinary authored migration remain distinct. Standalone imported Lists receive an immutable binding so their native action is retained. The actual import/compile graph test checks ordered Number producer references with no added content coercion. The statistic suite separately preserves exact hybrid structure.
- The JSON-null guard oracle now requires both independent diagnostics: `E_NATIVE_JSON_NULL` at `null`, and `E_ARGUMENT_TYPE` at the unsafe Optional-to-Text read. Shared analysis and the real CLI require both exact ranges, with no extra errors. CompileSource continues to return its first error only. Missing, wrong-code, wrong-span, and extra diagnostic mutations are rejected. Production null handling is unchanged.

The first repair attempt exposed the stale CompileSource expectation and is retained as `first-compiler-repairs.*`. The final successful gate did not suppress the null diagnostic or loosen the List action's argument type.

These are focused engineering receipts. They do not replace the combined full repository suite, private corpus regression, native Simulator evidence, 179-requirement acceptance report, or release decision.
