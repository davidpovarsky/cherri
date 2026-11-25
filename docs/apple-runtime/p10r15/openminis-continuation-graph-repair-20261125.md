# OpenMinis continuation graph verification repair — 2026-11-25

## Reproduced failure

OpenMinis run [45545299814](https://github.com/davidpovarsky/cherri/actions/runs/45545299814)
failed on implementation source `1e02755bbfaf470b4dba1802aa410f29ab4d1188`
in “Validate continuation probe graphs and strict two-run policy”.
Its original checkout-bearing job log is retained without modification at
`/mnt/development/projects/evidence/cherri-p10r15-ci-release-20261125/final-k-workflow-originals-01/45545299814/138973368315-job-log.original.txt`
(SHA-256 `c0bbaa767144de63c6ffdfd780e767be025a4444197e28898d42e1e1211a634e`).

The existing guard expected direct native Text conditions. Current typed
Get Type comparisons use the compiler's exact Text comparison graph:
sentinel snapshots, literal regex quoting, anchored case-sensitive matching,
Items Count, a Boolean branch and its consuming condition. The old guard
rejected these additional effects before checking the source predicates.

## Scoped repair

The continuation verifier now uses the existing
`tools/shortcuts-simulator-qa/native_comparison_graph.py` verifier.
Every admitted comparison region is bound to the original Get Type producers,
their source operands, its exact ten-action region, the consuming condition,
and the original storage or HTTP branch. No general match/replace/count
allowlist is added.

Storage readback values keep their original dynamic String coercion.
HTTP result predicates retain the original dynamic Text and direct numeric
comparisons. Original source fixtures, endpoints, two-run policy and native
execution claims are unchanged. The helper is included in the source-file
digest inventory.

## Completed local evidence

The four unchanged public fixtures were compiled using the existing compiler
binary built from compiler source
`627c3e7eb4a54a0364b5a5563db89291fa953b97`.
This is a structural diagnosis using that real source binary, not a claim that
a new implementation binary was built or that Apple runtime execution passed.

| Public fixture | Native actions | Rejected causal faults |
| --- | ---: | ---: |
| Storage cross-run | 78 | 57 |
| HTTP JSON control | 85 | 73 |
| HTTP JSON fetch | 85 | 76 |
| Numeric boundary | 89 | 17 |
| Total | 337 | 223 |

All four repaired positive graph checks passed. All 63 original causal faults
remain in the original order and still fail validation. The additional 160
faults exercise exact operand identity, sentinel preservation, regex escaping,
anchors, case sensitivity, count semantics, Boolean polarity, branch
boundaries, failure-flag targets and numeric-versus-Boolean distinction.

Original source diagnosis:
`/mnt/development/projects/evidence/cherri-p10r15-ci-release-20261125/openminis-continuation-diagnosis-01/diagnosis.original.json`
(SHA-256 `9af8a24976b78105d71a99e5397df17e93571f3c5c5079899dccdd785907fc70`).

Complete focused causal result:
`/mnt/development/projects/evidence/cherri-p10r15-ci-release-20261125/continuation-guard-complete-causal-01/result.original.json`
(SHA-256 `6a1b6e7c438743032551e16885023255f9dc43548014dbfdd4f4d4f5f445e59d`).

The original full build/compile/import/recompile producer remains pending
the coordinated Go slot and disk admission. This checkpoint does not claim
workflow success, native Shortcuts execution, complete CI or release acceptance.
