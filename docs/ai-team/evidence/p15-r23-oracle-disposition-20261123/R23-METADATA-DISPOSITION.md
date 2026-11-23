# R23 and P10 artifact metadata: preserved failure disposition

## Status and exact source

This read-only diagnosis retains a real production **R23 FAIL**. No protected file, gate, fixed baseline, test identity, assertion, deadline, inventory, acceptance contract or historical result was edited. No native replay or full CI run was performed by this owner.

Source: `ca316a3dee252456c4bec751594cf249c709b331`, tree `df61b6ddf47137998c6053cfada292f0a6a17cca`, branch `audit/cherri-p10r15-r23-20261123`. The source was clean before and after the three assigned checks, whose actual UTC timestamps and original stream hashes are in `checks.json`. This report is a later analysis of those preserved executions, not a rerun.

| Actual check | Exit | Outcome |
| --- | --- | --- |
| `python3 -B tools/p10r3/run_release_gates.py --verify-registry` | 0 | All 56 required gates registered. |
| `python3 -B tools/p10r3/run_release_gates.py --verify-commit` | 1 | Four input dependency fingerprints stale. |
| `python3 -B tools/p10r3/run_release_gates.py --gate R23` | 1 | Committed protected UI file differs from fixed baseline. |

## What R23 requires and why it fails

The production gate in `tools/p10r3/run_release_gates.py` binds `76d722502e20db977e84a095444d0172442655b8..HEAD` and also inspects the working tree. Its protected paths are `ios/CherriUITests/CherriEditorUITests.swift` and `ios/CherriApp/Views/CodeEditorView.swift`. Its original acceptance contract describes zero changed lines in those paths; the living master plan describes protection of concurrent UI work. The historical contract text uses a merge-base command, while the current production implementation explicitly freezes the full `76d7225...` commit. The executed production contract was not changed.

Only `CherriEditorUITests.swift` differs: 584 additions and 35 deletions. Baseline bytes: 21,887; SHA-256 `d5ef275d13efd5873cd2cfaca040592d6c8ffdfe26eec291344e0b47c3290f89`. Current bytes: 54,532; SHA-256 `9c1d5b1d3cded108be2decd85131716da143a6e5ed3fe404966c6285684ad4c8`. The second protected path does not exist at either revision. The full diff and commit history are retained beside this report.

The current suite retains all nine baseline test method names and adds four methods: D03 documentation search, exact palette selection/replacement, generated closer completion/overtype, and exact UTF-16 input/reset. Existing methods also gained strict editor input verification, placeholder handling, current contextual completion checks, and bounded startup/readiness observations. Merely moving four additions into a new test file would leave the changed historical methods and helpers unresolved.

## Concrete conflicting behavior

The original `testE01CompletionDisplayAndInsertionUI` selects a show completion, adds a newline when it sees an opening parenthesis, and then unconditionally requires successful compilation without a diagnostic. The current canonical snippet is `show(<#input#>)`. The actual Swift integration contract `testPaletteSnippetSuppliesRequiredArguments` requires that an unfilled hole be rejected, then explicitly replaces it with a value before compilation. The current E01 UI method verifies this same reject-then-fill sequence. The relevant baseline/current methods and Swift integration source excerpt are retained in `source-analysis.json`.

This is a **source-based incompatibility diagnosis**, not a claim that baseline UI tests were replayed on a device or simulator. Restoring the historical implementation without a behavioral regression has not been demonstrated. Current E01, RP40 and BRG27 requirements still demand actual production app/UI call paths. The historical class is final and its helpers/state are private; an additive Swift extension cannot replace these same stored properties or override the existing methods. No safe additive fix was identified that simultaneously preserves the byte-frozen file, executes it unchanged, and retains the newer required behavior.

Excluded approaches: an inert restored file with replacement compilation; test-source exclusion; altered test identities; duplicate whole suites; dynamic method replacement; changed fixed baseline; relaxed assertions or deadlines; or manufactured PASS receipts. These would not resolve the conflicting requirements.

## Proposed disposition for the owner

Retain the real R23 failure and all newer UI behavior. If the owner intends the approved later UI work to supersede the historical parallel-work isolation condition, that needs an explicit, separately versioned policy disposition identifying the exact approved UI delta and replacement isolation condition. Such a decision would be a contract change, not a code repair or a passing result for the existing R23. No such change was made or assumed by this agent.

The previously repaired actual-Git fixture is reused from `docs/ai-team/evidence/p15-r23-actual-git-fixture-20261122/FIXTURE-INTAKE.md`; it validates fixture behavior and grants no production waiver. Referenced `SCOPE-MAP.md` is absent from the source tree. The transferred continuation package metadata was inspected and lists no path containing `scope-map`, `scope_map`, or `r23`. This does not assert that every archive on the server was searched.

## Separate actionable metadata failure

Four artifacts record fingerprint `40ecf0bfbda0e3cf4bd3ff4227494abaf3f3354fdec4aa607fd0c0ea2fcae865`; current inputs produce `08d292f77f286997a19f615821bb76f269e224120ebcce3282556b05cbe268ca`:

- `docs/apple-runtime/p10/p10r3-gate-results.json`
- `docs/apple-runtime/p10/p10-505-catalog-coverage-matrix.json`
- `docs/apple-runtime/p10/p10-apple-cherri-crosswalk.json`
- `docs/apple-runtime/p10/baseline-state.json`

No source ancestry error was emitted. These stored artifacts were not restamped. A truthful refresh requires actual catalog producer execution at a confirmed frozen source, derived generation from the new raw evidence, and an independently owned actual gate report. The queued sequence, exact commands and ancestry constraint are in `505-REFRESH-PLAN.md`. Refreshing metadata can resolve metadata integrity; it cannot make the unchanged production R23 pass.

## Evidence handling

`checks.json` and the six `*.original` streams are unchanged. The gate's output digest is `7445ab5488f730a742db669ab2a15e768cac52be6a5aa14f4e1c7f5a296ac322`; the SHA-256 of its original JSON stdout is separately recorded by the stream manifest. `evidence-manifest.json` binds every retained file without replacing original timestamps. All evidence remains on Oracle in this directory. Publication and Asana updates belong to the coordinator.
