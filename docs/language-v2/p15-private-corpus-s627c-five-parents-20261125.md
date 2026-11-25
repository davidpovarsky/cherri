# Fresh private-corpus S4 verification on S627c

Date: 2026-11-25. This records five actual fresh producer executions and the subsequent independent original intake. It grants the S4 corpus evidence scope only.

## Source and preserved inputs

- Compiler source: `627c3e7eb4a54a0364b5a5563db89291fa953b97`.
- Compiler tree: `7448a8e89ed1b5c8ca914f4d7c0c05197a161495`.
- Original private cohort: 38 files, 62,980,131 bytes.
- Original input manifest SHA-256: `766ac82ef7d92f50a894e50bc84d3fed9cc2d375f81ffe2771dbe8a5a6de5f39`.
- Input names and contents remain private. The five original assertion files, test identities, selectors, stream oracles, and execution limits were preserved.

## Actual executions

| Original parent | Driver seconds | Outcome |
| --- | ---: | --- |
| `TestP12_PAR_12_CorpusComplexParamsAndZeroStage3Gate` | 214.402 | PASS |
| `TestP13_CorpusFidelityAndSourceSizeAudit` | 186.345 | PASS |
| `TestP10R10CorpusMeasure` | 159.572 | PASS |
| `TestP11_WIRE_01_Fresh38x2NativeActionWire` | 212.919 | PASS |
| `TestP12_BASE_02_LifecycleStagesAndReconcileCorpus` | 331.553 | PASS |

Each execution used `go test -count=1 -json -timeout=15m -run ^<original-parent>$ .` with Go 1.24.13. The driver retained its 930-second boundary; the first four parents retained their 970-second owners. Lifecycle used its original direct driver. There were no skipped, failed, incomplete, duplicate, or orphaned case events and no timeouts. Every parent and package terminal was checked independently.

## Original corpus results

The original summary contains 27,652 actions: 23,353 natural, 4,298 fallback, one invalid action, zero demotions, and 137 third-party actions. These are action classifications, not a claim that every input action is natively valid. The invalid classification is preserved.

The fresh 38-by-two wire check passed. The lifecycle reconciliation passed with equal fresh/sequential natural counts and an empty difference set. The full original corpus hashes were checked in place once by the independent intake; all five source guards and retained stream hashes passed.

The independent intake completed on 2026-11-25 at 02:34:17 UTC with exit 0 in 3.072 seconds. It accepted all five new-source parents. Source HEAD/tree and clean state were unchanged before and after.

## Retained evidence

- Original intake: `/mnt/development/projects/diagnostics/cherri-private-s4-20261123/s627c3e7e-replay-20261125-01/independent-intake-01/result.original.json`. SHA-256: `441b47acf90a903b62b19cc96d5fc76b293bad07d4bc347b9a493e29d3e30d8c`.
- Original intake supervisor result: `/mnt/development/projects/diagnostics/cherri-private-s4-20261123/intake-627c3e7e-20261125-supervisor-01/result.original.json`. SHA-256: `a0d7c2fe2eb58b9e556718ee549a002b33b41c439e4d4340f6d9119b87e4644c`.
- Root handoff with all five exact process, stream, specification, assertion-source, and owner bindings: `/mnt/development/projects/evidence/cherri-p10r15-final-completion-20261125/s4-source-627c-accepted-v2.original.json`. SHA-256: `fb1aa15d48546a51e4c17ff686b16b08fb2bc566e31782529555961cb2373821`.
- Asana acceptance milestone: task `1220432417414104`, story `1220886760407730`.

The v2 root handoff corrects the derivative v1 label `invalidSources` to the actual action classification. Both summaries remain retained; no producer or independent-intake original was rewritten.

## Acceptance boundary

These executions establish the five S4 corpus gates at the pinned compiler source. They do not supply Apple execution, AI quality measurements, unsupported general language semantics, complete 179-case acceptance, or final release approval. Fresh exact-source CI, remaining runtime work, consumer validation, and final acceptance remain separate requirements.
