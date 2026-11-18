# F01 native failure: diagnostic probes

## Observed boundary

The signed local F01 fixture failed in [run 44239178590](https://github.com/davidpovarsky/cherri/actions/runs/44239178590) at source `256f6a639a6f2a47eee5350ffc062746da9bd752`. Its callback was correlated to the requested compiled Shortcut, but reported `ERROR`, error code `1`, and no verified inline trace. The error text was redacted; its length is not evidence of a cause. The failed result and the original source remain unchanged.

The same source revision's Bool/List fixture passed in [run 44239174499](https://github.com/davidpovarsky/cherri/actions/runs/44239174499). This establishes a useful native control for the harness and some shared actions. It does not prove the computed Boolean, Math update, or nested logical-expression paths in F01.

The exact failed revision was rebuilt locally and its two fixtures recompiled. Their source hashes and action-identifier sequences match the published manifest: 91 actions for local F01 and 32 for Bool/List. These graphs were regenerated and contain fresh UUIDs; they are not the originally signed payload bytes. Inspection found no dangling producer references, unbalanced control groups, or external Run Shortcut action in local F01. The authored numeric conditions carry the existing legacy-comparison flag. Those checks have not identified the native cause.

## Four bounded probes

Each probe retains the existing before/after trace wrapper, three fixed UI anchors, final semantic output, and explicit fixture/run correlation. Each is independently selectable with the workflow's `sample_id`. No probe invokes another Shortcut or a network service.

| sample_id | Isolated feature | Native actions | Math actions | If groups | Expected semantic output |
| --- | --- | ---: | ---: | ---: | --- |
| `qa-f01-diag-counter` | Named Number variable, one Math increment, store, and observed value | 18 | 1 | 0 | `CHERRI_QA_F01_DIAG_COUNTER:calls=1` |
| `qa-f01-diag-predicate` | Computed false/true Boolean results, stored variables, and ordinary conditions | 38 | 0 | 4 | `CHERRI_QA_F01_DIAG_PREDICATE:P15_F01_PREDICATE_OK` |
| `qa-f01-diag-control` | Computed true condition, Math update inside an ordinary If expression, its result and exact count | 42 | 2 | 4 | `CHERRI_QA_F01_DIAG_CONTROL:P15_F01_CONTROL_OK:calls=1` |
| `qa-f01-diag-logic` | Computed false AND and true OR with nested Boolean results, plus ordinary positive control; no Math | 65 | 0 | 10 | `CHERRI_QA_F01_DIAG_LOGIC:P15_F01_LOGIC_OK` |

The control probe contains two static Math actions, one in each alternative branch. A successful execution must perform exactly one increment. The logic probe isolates composition without a side-effect counter; it cannot by itself prove the full short-circuit side-effect contract.

The source files are `tools/shortcuts-simulator-qa/fixtures/<sample_id>-v2.cherri`. The existing `qa-short-circuit-local-v2.cherri` remains byte-for-byte unchanged, with SHA-256 `9e55dc6bb913b8cbb06d2ce8cdd167dcc6a0697ba7347949b260d8e2cc768a82`.

| Probe | Source SHA-256 |
| --- | --- |
| counter | `739103463ebf2de7e310dca933924c9560b14964572409e9a60a062e29bee881` |
| predicate | `5056c5e2ceab80a6642c867e2a9d26d238be5f74a93a21f641a99e2a860f8a93` |
| control | `9b64aa49cb6000e66623f74b418bc7b5305d423dc7f169a60eb76817cf6c8f53` |
| logic | `f82b0131708ba28b5512190129ba9494f4adf7bb0523ccaf5606f613ceb7842e` |

## Local verification and its scope

The actual CLI compiled all four fixture files with `--skip-sign --build-profile=debug`. The production-compiler tests in `p15_f01_native_diagnostics_test.go` validate the emitted graphs, prior producer references, balanced groups, original-fixture hash, exact Math input/store relationships, output dependence on the observed value, and AND/OR branch placement. Causal mutations of the increment, stored producer, observed counter, external action, and AND predicate are rejected.

The focused command is:

```text
go test -count=1 -timeout=90s . -run '^TestP15F01(Diagnostic|LocalCounter)'
```

Harness tests validate the four exact selections, reviewed root source paths, fixed Legacy counterpart, exact fresh semantic marker, fixture identity, callback status, and required trace. Incorrect counts, wrong results, stale run IDs, another fixture ID, callback errors, missing trace, and unreviewed selection names cannot satisfy the associated gates. The Swift sanitizer adds only the four exact reviewed fixture names and rejects an unreviewed name; Swift/native execution is pending the signed runner.

```text
py -3 -m unittest discover -s tools/shortcuts-simulator-qa -p 'test_*contract.py'
```

The diagnostic patch extends the existing harness; it does not modify compiler semantics or the formal acceptance record for F01.

## Dispatch and interpretation

Run counter and predicate first to obtain independent controls for named-variable Math and computed predicates. Run control and logic to test the ordinary conditional-update path and logical-result composition respectively. Retain the source SHA, fixture source hash, unsigned and signed payload hashes, exact callback identity, result, trace, and error code for each run.

A failed probe narrows the affected path only after its callback is correlated to its own compiled artifact. A passed simpler probe does not certify the omitted features. If all four pass, rerun the unchanged original local F01 fixture on the same candidate and investigate its combined path if necessary.

**Native status at introduction: NOT_RUN for all four probes. Original signed F01 status: FAIL_RUNTIME, unresolved.** Neither compilation, graph checks, nor a successful diagnostic replaces the full local F01 side-effect oracle or the required final signed acceptance run.
