# Root CI regressions — 26 November 2026

## Source and cause

Base integration M is `f1eaa704d15d21835facb5e12b8cfe3f057274e5`.
Production fix `a407a732dbea1154bf6a7729965084855aafbeeb` and test
commit `88c6fe0652bbd9ca93602d35d03f569ae839b489` are separate.
The tested tree is `ecd3c4b65113a3161f424d3032e4f0cdfb673ce7`.

The actual Linux/macOS [M Build](https://github.com/davidpovarsky/cherri/actions/runs/45770101343)
failed. Its original T02 source exposed a real analyzer regression: require
inspected direct named literal nodes instead of the already-bound argument
expressions. It now uses the canonical binder and existing pure Text/Bool
evaluators, covering positional options too, without a second evaluation.
The original T02 test file is unchanged.

Other tests were reconciled with implementations already present:

- The exact original two F04 fixtures and test identities remain. A complete
  fifteen-action collection region checks empty/nonempty List cells, two
  independent loop groups, payload/producer edges and both counts. Four
  corruptions per fixture reject. Exporting the private List to public output
  still fails before emission. Historical signed/runtime failures are retained.
- Authored Legacy empty arrays require an explicit typed empty payload. The
  original opaque imported absent-Value and explicit-empty round-trip controls
  remain byte-identical.
- Both paginate metadata tests require its optional page callback and retain the
  typed result, literal options, catalog denominator and Release semantics.
- The Release require consumer uses its supported Text guard and still requires
  the native-output UUID. Bare unproved AnyContent is separately rejected.
- Twenty-two new Debug/Release cases compare the complete constant-option graph
  to direct literals and require exactly one authored input producer.

## Actual qualification

| Execution | Actual result | Scope |
|---|---|---|
| Focused Go on88c6 |60parents/783unique named passes, zero fail/skip|require,T02,S02,F04,paginate,raw Release consumers|
| Three original acceptance producers |93outer named passes, zero fail/skip|Original production/intake paths and negative controls|
| Nested original structured cohort |1,356unique passes: root1,204/lower109/schema43|T05,L01,F04,RP20;8local unit/native-structure records|
| Other original local producers |S02,L03,T02 passed|Their own immutable tiers only|
| Independent source review |PASS on all8exact source files|No duplicate execution|

These separate nested counts are not summed into a global total. Complete
tracked bytes matched before/after both executions.

The focused owner retained FAIL after successful Go because its postflight
tried to create the same observer-copy file twice. Its Go process was reaped,
its private group was absent, and both complete source snapshots already
matched. A separate read-only intake verified those original facts. A later
next-run admission observed another project's Go and remained blocked; no
process was terminated. The subsequent actual producer execution used a fresh
clean admission and completed PASS.

## Originals and limits

The [public archive manifest](evidence/p15-root-ci-regressions-20261126/manifest.json)
binds23explicitly selected source snapshots, process/Go originals, canonical
structured artifacts, source review and derived intakes.
[public-originals.zip](evidence/p15-root-ci-regressions-20261126/public-originals.zip)
SHA256: `a0d188784ab818775ca1cdcb06a23bd743de86befa541aecf5bb84453b34d4b1`.
Global process inventories and every failed attempt remain protected under
`/var/cache/cherri-p10r15-root-ci-repair-20261126`, outside the public allowlist.

Primary receipts:

- Focused intake: `4e2aca36091d71bb375772199c7363a8a0222161cbd85cf6b930ea6e3ca40b76`.
- Original producers: `d37cc5ad15e9542e904955adf05caa7c4f81fa7222ecc5d449e8852f8b993021`.
- Structured JSONL: `089e58a63a4da9ee6dad7b3d9c188659207a23439277b1beea8e1fcf5ecf91b7`.
- Independent review: `7cd02320e657c89d394f62212c47799553f3ac70a6260f0e29f811045b40b539`.

No Apple execution, physical device, AI evaluation, final179 closure, final CI,
main merge or release is granted by this component qualification.
