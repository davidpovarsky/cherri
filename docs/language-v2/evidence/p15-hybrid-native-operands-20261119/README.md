# Hybrid native operand reconstruction

Implementation source: `58dde2dded6b145e201b5229f409d5f540200a43`. Private regression baseline: `42ffabc2cedf488bf1cbb415efddc5f4c92ec95d`.

The clean implementation commit passed **67 named tests and subtests, 0 failures,
0 skips**, in **13.94 seconds** with Go 1.24.13 on Linux arm64. The
exact command, discovered test names, source cleanliness checks, and raw public
log hash are in [execution-receipt.json](execution-receipt.json). The full public
focused log is [clean-source-focus.jsonl](clean-source-focus.jsonl).

## Actual defects and repair

Semantic import was reconstructing existing ordered native attachments as bare
source symbols. Their inferred `AnyContent` type then failed the intentionally
strict Number/Date comparison check. On the unchanged 38-original baseline,
this demoted 74 native control markers. The importer now retains the explicit
native reference using its existing `rawRef` / `native.ref` renderer on both
operands. No scalar type is invented and no coercion action is inserted.

One further demotion was an Ask for Input default attachment: a native Number
producer became a bare Number source argument to the canonical Text parameter
`defaultValue`. That imported attachment now retains the same explicit native
reference boundary. The prompt definition and ordinary authored argument checks
remain unchanged. A negative regression still rejects a bare Number default.

The public matrix covers all four ordered operators, Shortcut Input and opaque
action outputs, references on either side, and both reference operands. Every
case verifies original native control cardinality, zero candidate demotions,
pre-overlay semantic equality in both dialects, and exact final native wire
parity. Existing invalid-reference, namespace, dynamic-shape and scalar-evidence
rejection tests also execute in the same focused command.

The initial public causal test had eight failing cases: four V2 compilation
failures on opaque action outputs, plus four Legacy failures on Shortcut Input.
The latter were retained and repaired when both dialect renderings were made
explicit. [public-causal-baseline.jsonl](public-causal-baseline.jsonl) contains
only those public synthetic test records; all private test output is excluded.

## Full private baseline, before repair

The five requested tests ran separately and serially on clean `42ffabc2cedf488bf1cbb415efddc5f4c92ec95d`.
Every requested test was discovered, and none skipped. The actual results were:

| Test | Result | Seconds |
| --- | --- | ---: |
| `TestP10R10CorpusMeasure` | PASS | 187.35 |
| `TestP11_WIRE_01_Fresh38x2NativeActionWire` | PASS | 225.87 |
| `TestP12_BASE_02_LifecycleStagesAndReconcileCorpus` | PASS | 337.69 |
| `TestP12_PAR_12_CorpusComplexParamsAndZeroStage3Gate` | FAIL: V2 compilation demotions | 29.29 |
| `TestP13_CorpusFidelityAndSourceSizeAudit` | FAIL: 23,278 below required 23,340 | 178.89 |

The full measurement contained 27,652 actions. Fresh and sequential lifecycle
counts agreed at 23,278 natural actions. There were 29 demoted units / 75 actions,
all attributed to V2 compilation, and no state-dependent count drift. Both
Legacy and V2 still passed the full native wire gate through preserved fallbacks.
The minimum of 23,340 and zero-Stage-3 requirement were not changed.

The 38 originals remained 62,980,131 bytes with content-set SHA-256
`d1aaa34a0f89ec250dd223320382f2c5e2d34a00267a394b32853f46b0e49e33` before and after.
All 76 files in the input directory, including companion data, were unchanged.
Only aggregate results and log hashes are archived here. Original contents,
workflow identities, temporary diagnostic plans, and private raw logs remain
outside version control on the authorized host.

## Scope of this receipt

This is an immutable source repair and baseline failure receipt. It does not
claim private acceptance completion. The definitive five-test run is reserved
for the integrated source containing this repair and the independent Legacy
rawAction integer/real preservation repair. No Apple execution claim is made.
