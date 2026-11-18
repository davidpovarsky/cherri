# P15 generated conditional compatibility

## Scope and causal result

The production compiler emitted old-format single-predicate conditions for computed Boolean values and safe list indexing without the compatibility key emitted by an equivalent ordinary authored `if`. The comparison code, numeric operand, and named-variable input were otherwise identical.

Seven generated start sites now use the established ordinary `if` contract: unary Boolean negation; decimal literal index bounds; both dynamic index bounds; dynamic `&&`; dynamic `||`; and comparison expressions that yield Boolean values. These starts retain `WFCondition`, `WFInput`, and the typed comparison field and include `WFConditionalLegacyComparisonBehavior: true`.

This is a correction to the native predicate representation. The public V2 language remains V2. Explicit `if modern:` and original imported predicates keep their own representation. No global builder default changed.

## Execution evidence

Base: `3754f555433c93201ea48a68d89408532780b34f`.

- Causal RED: Sentinel job `job_aa014a437fea` ran actual production compilation. Four comparison/negation/logical cases and a dynamic bounds predicate differed from independently compiled authored predicates solely in the absent compatibility field.
- GREEN: Sentinel job `job_f915c9701182`; `go test -count=1 -timeout 90s . -run '^(TestP15GeneratedGuardsMatchAuthoredNativeIf|TestP15GeneratedBoundsGuardsMatchAuthoredNativeIf|TestP15GuardPredicateOracleRejectsMutations|TestP15GeneratedGuardChangePreserves|TestP15F01|TestP12_CF_)'`; root package PASS in 0.907 seconds.
- The final bounds matrix covers decimal literal, integer literal, and dynamic index paths. The decimal literal follows the single upper-bound path; the integer literal currently follows the existing two-guard dynamic path. This change does not claim an indexing optimization.
- The predicate oracle compares against an independently authored production `if`, including exact named-variable input where applicable. Removing compatibility, changing the condition code or comparison value, or removing input fails the oracle.
- Explicit modern single and compound predicates keep the compatibility key absent. Compound modern predicates retain `WFConditions`.
- Synthetic imported workflows with absent, true, and false compatibility values pass complete Legacy and V2 wire comparison, including pre-overlay capture. Absent and true preserve natural classification; explicit false preserves exact native fallback.

## Native runtime status

The signed R1 F01 sample returned an error. Its source includes intentionally missing Run Shortcut targets to detect eager RHS execution. Native static validation of those targets and the predicate compatibility difference are separate hypotheses. Structural parity and successful compilation do not establish which caused that error.

F01 runtime acceptance remains open until an exact candidate is signed, installed, and executed with a correlated callback. A separate self-contained counter diagnostic is being added without replacing or weakening the original missing-target fixture. No private corpus contents are included here.
