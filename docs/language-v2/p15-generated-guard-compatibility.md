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

## Separate self-contained F01 diagnostic

`tests/language-v2/v2-short-circuit-local-runtime.cherri` returns the exact successful result `P15_F01_LOCAL_OK:calls=1`. It has no Run Shortcut target or network action. It uses action-derived zero/one values to compute runtime Boolean bindings, two skipped RHS blocks (`false && rhs`, `true || rhs`), and an identical directly evaluated RHS as a positive control. Both branches of that RHS increment the observable counter once and yield a Boolean.

The native graph test verifies the original number producers and comparison outputs; precise AND/OR guard branches; all six possible branch-local Math/Set Variable updates; exact producer UUIDs; Boolean yields; the one initial counter store; all six failure observations; the success initialization; and the returned verdict/counter attachments. It rejects eager RHS, wrong increment, cross-wired update, missing positive control, external target, hidden counter, and altered success-marker mutations.

This test is a native graph oracle, not an Apple execution emulator. The signed target will be wrapped separately with trace/callback evidence. The original missing-target fixture remains unchanged.

The standalone source compiled through the actual CLI in Sentinel job `job_64048ccbee9a` and produced 79 actions. The CLI placed its output beside the source; the synthetic artifact was copied to the external evidence directory for inspection. The initial graph inspection hit a console encoding error while printing the final token string; the artifact had already been decoded and saved, and an ASCII-safe follow-up verified the final output attachments. No native execution is inferred from compilation.

The complete lower package passed in Sentinel job `job_95369479263c` (0.791 seconds). Initial local graph and six mutation tests passed in `job_47080e180225` (0.177 seconds); the final bounded rerun includes the additional success-marker mutation and the original F01/guard tests.
