# R23 mandatory CI blocker: inactive policy revision for owner review

**Status: INACTIVE / POLICY NOT AUTHORIZED / NOT APPLIED. One synthetic R23 parent qualified successfully.** The root authorized and reviewed exactly one existing unittest parent on an isolated persistent copy; it passed under the original60-second bound. No real-repository R23 v2 gate, active patch, broader suite, Go, workflow or provider was executed. The production approval record remains DRAFT and fail-closed. The storage commits remain separate and unchanged. Prequalification review bytes are retained in review-history/qualification-01-before/.

Source under review: **9b4135e705b08f765b7304e01212d3b9658806fb**, frozen K11. The owned worktree remains clean at storage proof14a7440e839afdceca56a54c608f294e1d01073a; the reviewed gate/workflow/policy sources are byte-identical to9b.

## 1. Confirmed consequence

The current implementation requires an empty protected-path diff from fixed76d722502e20db977e84a095444d0172442655b8 and an empty protected-path working-tree status. Frozen9b has a real committed difference in CherriEditorUITests.swift. The evaluator therefore returns FAIL/exit1.

The full runner executes every registered gate, increments failing for that result, writes the actual report and optional attestation, then exits1. Both Linux and macOS invoke --run-all as the final unguarded command in their mandatory P10 SSoT, Coverage Ledgers, Pack & Release Gates step. No workflow/job/step continue-on-error setting is present. A preceding failure may prevent this step from being reached; even if all preceding checks pass, unchanged9b cannot finish this mandatory step successfully.

This is confirmation of persistent coupling, not discovery of a first-ever R23 failure. The coordinator reports that original CI44971815217 already reached R23. Retained production R23 FAIL and later actual root56 findings remain historical failures. The separately retained K11 Linux original job137474256318 actually stopped earlier at --verify-commit because four generated dependency fingerprints were stale. It did not execute --run-all or actual R23; the fixed-base R23 mandatory blocker remains a distinct next failure once preceding checks pass. No historical result is relabeled.

Exact source references at9b:
- tools/p10r3/run_release_gates.py:198–206: both protected paths and fixed baseline.
- Same file:622–653 and663–678: actual Git commands, predicate and FAIL/exit1.
- Same file:874–896 and932–970: full execution, counter, original outputs and process exit.
- .github/workflows/build-test.yml:92–105 and239–252: mandatory Linux/macOS commands.
- Same workflow:107–114 and254–261: always-uploaded release artifacts do not convert a failed step into success.

## 2. Authorization: what is already granted and what is missing

AGENTS.md:5–14 explicitly authorizes ordinary implementation, commands/tests, transfers, isolated branches, commits, pushes, integration and publication, and supersedes older local confirmation prompts. Those operations do not need another permission round.

AGENTS.md:16–19 separately preserves acceptance criteria and requires actual gate verification. The P15 execution ledger:45–47 states that broad owner authorization does not remove technical acceptance gates. The current R23 acceptance row is explicit: zero protected-path differences, with any protected modification as its negative fixture (docs/apple-runtime/p10/p10r2-acceptance-contract.json:228–236). The current production fixture independently asserts the exact fixed baseline and path list (tools/p10r3/test_coverage_and_gates.py:210–220).

Changing that condition is therefore a **policy revision**, not a source repair that makes the old zero-difference claim true. No reviewed repository rule or active runner option authorizes an automatic baseline update. The runner's complete CLI at799–821 has no update-baseline/waiver option. A search of AGENTS/CLAUDE, team and hidden agent rules, current plan and relevant source found no R23 baseline-update grant. The historical merge-base wording and fixed implementation differ; silently switching to a floating merge base would not resolve this with an explicit source-bound decision.

The coordinator independently read the current authenticated Asana task and latest technical comments and found no explicit R23 owner disposition. Connector-created comments displaying the owner's identity are technical records, not independent evidence that the owner approved a new R23 condition. This review does not treat them as approval.

The U01 exact correction mechanism is a useful implementation precedent, not a blanket grant for R23. Its source-bound allowlist covers two named one-token declaration fixes, different paths and a different obligation (docs/language-v2/p15-u01-upstream-boundary-20261120.md:20–29,39–43). It does not authorize this584-addition/35-deletion UI delta.

If a genuine prior owner instruction approving this exact policy is later located, record that instruction instead of asking again. None is established by the sources reviewed here.

## 3. Exact state proposed for approval

| Protected path | Original76d state | Proposed immutable R7 / current9b state |
| --- | --- | --- |
| ios/CherriUITests/CherriEditorUITests.swift | blob1cb31feeb7e641e27f58d52e7e6822c11258c9a0; mode100644;21887 bytes; SHA256d5ef275d13efd5873cd2cfaca040592d6c8ffdfe26eec291344e0b47c3290f89 | blob33028f1f8b53b3855382a1ab6042a2c59402f687; mode100644;54532 bytes; SHA2569c1d5b1d3cded108be2decd85131716da143a6e5ed3fe404966c6285684ad4c8 |
| ios/CherriApp/Views/CodeEditorView.swift | Absent | Remains absent |

Proposed immutable baseline: **ca316a3dee252456c4bec751594cf249c709b331**. Both protected states were mechanically compared at76d, R7 and9b. R7 and9b are identical. The entire current13-method inventory includes all9 original identities plus4 additions.

The original E01 test requires successful compilation after a completion containing an unfilled hole; current canonical completion and the actual Swift integration contract require that hole to be rejected until explicitly filled. The preserved causal report contains both exact method bodies. Moving only the four new methods does not repair changed original methods/helpers. Restoring an inert historical source file, excluding it, changing test identities or adding a duplicate replacement suite would not preserve the intended obligation. No historical UI replay is claimed.

## 4. Smallest reviewable revision

The actual patch is R23-v2-proposal.patch.inactive, revision4:
**SHA25665b4faad9bfdafd5b0bea4383c38f010f8098845e4eafff1e92593f5c309fbca** (33228 bytes).
The small reviewer delta from preserved revision3 is revision-03-to-04.delta.diff:
**SHA256874266d09d8d3bd7c0ff25880e04a4974cd96acecde6f9760e4d80a91d22eabd** (4317 bytes).

It proposes four source paths only:

1. **tools/p10r3/run_release_gates.py** — pin the production R23 definition to exact R7 and the reviewed policy record. Public execute_gate is the single guarded production entry used by both CLI paths and the existing test parent; the prior body becomes private _execute_gate_observation. Original fixed76d fallback expressions and the two-path actual Git diff/status predicate remain. R23-only context checks reject explicit routing overrides by name, require the actual top-level, disable replacement objects, and reject legacy graft routing/default files. Actual ancestry, exact baseline/HEAD/index/working states and the pinned complete approval must all pass before public PASS.
2. **docs/apple-runtime/p10/r23-protected-state-policy-v2.json** — record old/new baselines, unchanged original contract hash, exact blob/mode/byte states, all13 test identities and the real approval source once one exists. Current proposalOnly=true, DRAFT_NOT_APPROVED and null owner-source/time fields remain fail-closed.
3. **tools/p10r3/test_coverage_and_gates.py** — preserve all22 parent identities and the original11 actual-Git actions/assertions byte-for-byte. Their owned fixture now supplies explicit SYNTHETIC_TEST_ONLY policy; the same existing CI-discovered parent adds one-present/one-absent positive, exact metadata/state/context/ancestry rejection controls and a real --gate child exit check. No test bypass is added to production. Revised controls create their own missing directories and prove the alternate-index/legacy-graft masking preconditions causally.
4. **tools/p10r3/generate_catalog_and_pack_ledgers.py** — add exactly the policy-record path, original P10 acceptance contract and both protected paths to the existing dependency globs. The fingerprint algorithm is unchanged; raw bytes/mode checks remain in each real gate execution.

No protected Swift file, workflow, original acceptance contract, test deadline,56-gate registry identity/count, or179-contract identity/tier is changed. This is not a wildcard exemption and has no bypass flag, environment opt-out, floating baseline, inferred approval or per-path omission.

### Why the approval guard is necessary

Merely adding an approval-looking file or its hash to a fingerprint is insufficient. The proposed guard rejects proposalOnly=true, DRAFT/null/incomplete approval, invalid version/scope, duplicate JSON keys, missing or tampered pinned bytes, wrong approval-source digest and missing UTC time. The original R23 contract bytes and exact protected-path inventory must match.

After the routing/top-level/graft boundary, the existing actual Git diff/status observation is retained with its original raw-output digest. Policy or exact-state failure still overrides the public verdict to FAIL. The guard also proves the immutable baseline is an actual ancestor of HEAD, then reads the baseline tree, current HEAD tree and actual index. Their blob/type/mode states must equal the approved states. Present working files must be regular files with exact bytes and executable mode; protected parent symlinks are rejected. The second path must remain absent in baseline, HEAD, index and filesystem, including ignored/untracked or broken-symlink creation. No successful observation is sufficient until the full pinned approval and exact-state guard also succeeds.

The guard pins the record's integrity; it cannot establish the human authenticity of an invented statement. The coordinator must bind an actual owner instruction to the exact policy before setting proposalOnly=false, approval.status=APPROVED, filling source/time/digest fields and updating the pinned record hash in the implementation. The draft is fail-closed even if accidentally applied. No approval is fabricated in this packet.

The extra guard is isolated in one R23-specific function rather than a general exception framework. It adds bounded Git observations (10 seconds per observation) and bounded policy/working-file reads without altering any original suite or child deadline. It also prevents a clean return to the old nine-test file from silently qualifying under v2.

## 5. Registry, fingerprint and report effects

R01–R28 and A01–A28 remain exactly56 registered gates. R23 remains a required production-path gate. The current full-run failure accounting and nonzero exit remain unchanged. Successful v2 results would identify policy version/id, approved baseline, prior baseline and pinned record hash; old R23 results are never overwritten or reclassified.

Changing the runner and adding the four exact policy/contract/protected-path inputs changes inputDependencyFingerprint. Current generated ledgers/baseline/report cannot simply be restamped. Follow the existing source-bound generation and actual gate-report sequence after any authorized activation. --verify-commit must continue rejecting stale or non-ancestor metadata. The original p10r2 acceptance file and the three179 inventories remain untouched; the separately versioned owner decision would document the explicit supersession of the old R23 scope condition.

## 6. Options and recommendation

- **Keep v1 unchanged:** no policy decision or source change. Preserve the actual failure and continue other authorized work, while Build & Test cannot finish these mandatory gates successfully. This is the current active state.
- **Approve the exact v2 state above:** the smallest identified route to a coherent future CI candidate while retaining current UI capabilities and all13 methods. It requires a genuine owner policy disposition, reviewed guard controls, actual current-source gates and actual UI evidence. The inactive patch is prepared for that decision.
- **Restore the original protected file:** no demonstrated behavior-preserving solution exists because of the concrete contradictory E01 behavior and modified private helpers. Do not adopt exclusion, redirection or duplicate-suite workarounds.

Proposed decision wording for the coordinator to present only when needed:

“I approve R23 policy version2 for exactly the protected states recorded in this proposal: baseline ca316a3dee252456c4bec751594cf249c709b331, CherriEditorUITests.swift SHA2569c1d5b1d3cded108be2decd85131716da143a6e5ed3fe404966c6285684ad4c8/mode100644, and CodeEditorView.swift remaining absent. Retain all13 current UI tests and the historical76d R23 failure. This approves only the new isolation policy; it does not waive any test, UI/native evidence, or other release gate.”

## 7. Validation state

Revision4 syntax parsing and git apply --check passed before execution. After root reviewed the final delta, exactly one existing TestP10CoverageAndGates.test_c5_06_r23_protected_ios_file_modified_fails_closed parent ran once in qualification-attempt-01/source, using its own persistent TMPDIR. The original command retained timeout --signal=TERM --kill-after=5s 60s.

Actual result: exit0, supervisor duration1.118164637 seconds; original unittest stderr reports1 test in0.930s and OK. All13 copied files, production DRAFT bytes, original22 parent IDs and the3,473-byte original11-action/assertion block stayed unchanged. The active source remained clean at14a7440e before and after. No rerun occurred.

Only owned synthetic Git fixtures received explicitly labeled SYNTHETIC_TEST_ONLY approval records. This qualification establishes the proposal's tested fixture behavior, not real owner authorization or a production R23 v2 PASS. The active gate/policy remain unchanged. A genuine owner disposition is still required before any production policy activation.

### Qualification addendum — 2026-11-23T05:07:18.701522+00:00

Original execution receipt: qualification-attempt-01/result.original.json, SHA256ae064cd2736389aa8d95f5c658ce83d913e3464a45f41dc29c372c3649b2f29f.
Exact command/copied-file/source inventory: qualification-attempt-01/launch-plan.original.json.
Original stdout/stderr, exact driver and separate intake are retained under qualification-attempt-01.
Prequalification REVIEW, control plan, patch summary and manifest are preserved byte-for-byte in review-history/qualification-01-before/.
No inactive patch bytes or DRAFT approval fields were changed by this evidence-only update.
