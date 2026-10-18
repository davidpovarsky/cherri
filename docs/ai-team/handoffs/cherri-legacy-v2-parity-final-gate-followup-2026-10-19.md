# Cherri Legacy ↔ v2 — final parity-gate remediation and independent handoff

**Date:** 2026-10-19  
**Repository:** https://github.com/davidpovarsky/cherri  
**Work branch:** `agent/cherri-v2-legacy-parity` — continue here, do not merge.  
**Latest code fix under review:** `c164b7143474609f3b0e760573cf294a9cf91554`  
**Subsequent team-policy commits:** `f53604ab95bbf85785b09686c95994d55ae2c9ce`, `c2e1c5eba682b93dc988b5a16f9ef8ce617a65f8`. Always re-fetch and record actual starting HEAD.  
**Asana parent task:** https://app.asana.com/0/1219603572114432/1219611976168814  
**Status:** BLOCKED_PARITY_GATE_FAILED; NOT owner approved; no merge, release, P11, or completion.

> THIS IS AN EXECUTION PROMPT, NOT A COMPLETION REPORT. A successful unit-test suite does not mean the differential parity gate passed. Treat all findings below as reproducible starting evidence and refresh against the exact current SHA.

## 0. Independently established baseline (2026-10-18/19)

1. GitHub Actions run https://github.com/davidpovarsky/cherri/actions/runs/39160747317 **succeeded** on `c164b714` (Linux, macOS, Nix jobs). This does not demonstrate full parity: `.github/workflows/build-test.yml` runs `go test -v ./tools/legacy-parity/...`, NOT the standalone `go run ./tools/legacy-parity` gate.
2. Checked-in `docs/language-v2/legacy-parity-results.json` on `c164b714`: **48** fixture records, **9** `VERIFIED_PARITY`, **19** `PROVEN_EQUIVALENT`, **20** `PARITY_DIFFERENCE`, **0** upstream/migrate/v2 compile failures; **17/17** selected new Apple action samples compare equal. The 20 are *not* proven equivalent.
3. Independent clean Linux ARM64 run of `go run ./tools/legacy-parity` on a clone at `c164b714`: **exit 1**, discovered **47** fixtures: 9 direct parity, 19 normalized equivalence, **19 unresolved differences**, 0 compilation failures, 17/17 action samples equal. Resolve why the saved report says 48/20 while the tracked clean checkout produces 47/19. Do not invent a cause; verify tracked, ignored, generated, and untracked fixture sets and working-directory assumptions.
4. The standalone gate correctly exits nonzero for unresolved differences, but CI does not invoke it. The previous Asana summary's “zero unproven differences / ready for owner review” claim was therefore inaccurate.
5. The FaceTime regression (`tests/enum.cherri`, `WFFaceTimeContact`, `WFFaceTimeType`) is demonstrably fixed in the current unit fixture and direct parity results; preserve these fixes. The 17 action samples and unit tests are real progress. They do NOT prove every action in the full P10 inventory is covered.
6. `skills/cherri-shortcuts/compatibility-manifest.json` still pins compiler `dfc400d5380d9bedc3f856610708f91cf686e33f` and docs `a701b19d6b7ba0b8cfc9423e0a03dc2663ad104b`, with schema fingerprint `15eed22550806d167aa399fb7b95b761f540a8e9bf2fbb560cb2dd9112b0ddaa` in the inspected branch. The verified remote runner currently selects the older P10 compiler/docs tuple. Updating only manifest text is forbidden; publish/adopt a coherent compiled/tested compiler+docs+schema tuple when actually ready.
7. Asana contains **no provisioned AI Teammate accounts** for the named roles. `eitan`, `noam`, `roni` and `tomer` are **logical Antigravity/Codex subagent identities**, not assignable Asana users. The orchestrator must ACTUALLY invoke them with `invoke_subagent` (or report the environment cannot), and record invocation evidence; task labels alone are NOT execution.

## 1. Required delegation and ownership

**Orchestrator:** read repository `AGENTS.md`, `GEMINI.md`, `docs/ai-team/TEAM.md`, `docs/ai-team/ASANA_POLICY.md` and this document first. Coordinate; do not impersonate a role, perform substantial implementation in place of a designated subagent, or fabricate an invocation. Use only the necessary subagents:

- **איתן / eitan — ARCH, first:** audit the baseline and all unresolved differences; define narrow mechanically checkable equivalence contracts and test strategy. Reviewer: Roni. Deliver a fixture-by-fixture matrix with precise native plist/flow differences, the 47-versus-48 root cause, severity, and fix owner. Do not proclaim structural differences equivalent merely because v2 has fewer actions.
- **נועם / noam — DEV:** implement the agreed parser/migration/lowerer/backend/compare fixes and rigorous regression tests, minimizing surgical diffs and preserving Legacy behavior. Reviewer: Roni. Deliver small compilable changes and exact SHA/test evidence.
- **רוני / roni — independent QA (mandatory):** independently reproduce the full gate, review normalized semantic invariants, write/verify adversarial mutations, cross-platform evidence, and regression results; no self-review by developer. Give PASS/FAIL/BLOCKED per criterion and actual artifacts/CI links.
- **תומר / tomer — INT:** own full CLI gate integration into CI, deterministic fixtures, artifact publishing, docs/Skill/manifest coordination and branch readiness. Review with Eitan and Roni. No automatic merge, release or unapproved cross-repo publication.

Consult יעל/yael for historical upstream schema questions only if evidence calls for it; consult מיה/maya only if iOS UI or Shortcuts display changes are being validated. Do not fan out to seven agents without cause.

**Execution honesty:** Role names in an Asana task mean *planned owner*, not “invoked”; invocation timestamps/IDs and each agent's actual output must exist before attributing completed work. No later autonomous Asana modification unless the user separately requests an update/sync; this request authorizes the initial coordination handoff only.

## 2. Phase A — baseline, deterministic test corpus, actual differences [Eitan + Tomer; Roni verifies]

A1. Verify Git status, remotes, selected feature branch, current SHA, compiler/docs/schema provenance and unchanged base behavior. Work only in isolated feature branch/worktree. No main merge, destructive resets or modifications of unrelated applications.

A2. Make fixture discovery deterministic on a **clean checkout**, on macOS, Linux and CI. Compare `git ls-files tests/*.cherri`, actual `tests/*.cherri` glob, hidden/generated/ignored files, the checked-in JSON list and the prior 48-versus-47 discrepancy. Investigate `corpus-batch001.cherri` and any other discrepant filename without presupposing it is the cause. Pin the corpus manifest and stable sorted fixture list. No implicit dependence on local untracked files, stale output, timestamps, cwd, or silently omitted files. Missing fixture => explicit gate failure.

A3. Build and capture the baseline with the exact same source SHA and current Go toolchain:
    go build ./...
    go test -count=1 ./...
    go test -count=1 ./tools/legacy-parity/...
    go run ./tools/legacy-parity

Record exit codes separately. A zero exit for the unit-test command must never be presented as a passing full CLI gate.

A4. Enumerate **every** `PARITY_DIFFERENCE` without truncation. The checked-in unresolved list is: `arrays.cherri`, `conditionals.cherri`, `control-flow-output.cherri`, `date-comparisons.cherri`, `decomp-expected.cherri`, `decomp-me.cherri`, `dictionary.cherri`, `empty-types.cherri`, `functions.cherri`, `getas-typecast.cherri`, `globals.cherri`, `math.cherri`, `menus.cherri`, `nil.cherri`, `numbers.cherri`, `repeats.cherri`, `standard.cherri`, `types.cherri`, `variables.cherri`, `web.cherri`. Keep this list synchronized from programmatic results, not hardcoded pass counts.

A5. For each fixture include: original .cherri, migrated .cherri, Legacy and v2 unsigned plists, action count and order, native identifiers, control-flow grouping, parameters, optional/default values, serialization types, typed output references, UUID graph, variable scope, triggers, import questions, workflow metadata and error handling. Classify as (a) exact parity; (b) mechanically proven semantic equivalence with narrow reversible witness; (c) actual behavioral defect needing code fix; (d) blocked on Apple device runtime and NOT owner-approved. Every equivalence must carry a reproducible test and evidence, not narrative assertion.

A6. Specifically scrutinize semantics of removing/reordering `setvariable`, `nothing`, `calculateexpression` vs `math`, action-token forwarding, `CustomOutputName`, changes in dictionary/list `WFItems` encoding, condition comparisons and early outputs, repeat result accumulation, menu branches, UTF-16 interpolation offsets, global variables, dates, coercion, function isolation/`Run Shortcut`, default workflow input classes and types. Do NOT globally erase meaningful fields to force equality. Establish liveness, control-flow and nested-branch invariants before eliminating operations.

## 3. Phase B — fix source truth, migration, native wire, and tests [Noam; Eitan reviews design; Roni QA]

B1. Fix actual behavior differences in canonical shared sources; use syntax/AST, analyzer, lowerer, native backend, migration and schema layers as appropriate. Preserve original compiler (`--legacy`) and v2 functionality; avoid duplicated action catalogs, fixtures tailored only to success, user-data loss or weakening diagnostics. Reverify FaceTime wire keys `WFFaceTimeContact` / `WFFaceTimeType`, quantity/color, metadata, triggers and `#ref` fail-closed handling.

B2. Keep the Legacy `copy/paste` and modern macro-equivalent **inline** (no surprise `runworkflow`) while preserving side effects and variable scoping. Preserve the intentionally different `Run Shortcut` mechanism of user-defined functions, recursion, argument passing and return values; protect hot-loop performance, but do not silently change function semantics as part of parity repair.

B3. For any optimization proven sound, add an explicit before/after native plist and adversarial test demonstrating the same executed semantics. Never normalize away missing/different required action parameters, missing variables/outputs, wrong boolean branches, malformed UTF-16 offsets, missing events/triggers or truncated action graphs. Unsupported cases must yield a clear FAIL/BLOCKED diagnostic.

B4. Test every new action callable from the **actual provenance/catalog inventory**, not only a fixed 17-sample list. Parameter-less smoke compilation is insufficient: validate all schema-defined required/optional fields, defaults, enum encodings, outputs, value types, token wiring, platform restrictions and both Legacy and v2 where supported. Classify full coverage separately from the 17 representative samples.

B5. Keep independent regression tests for every repaired fixture, including migration round-trips, decompilation where supported, handling of unknown directives (`#trigger`, `#ref`, `#question`, `#include`), bad/unknown action arguments, bad variable references and mixed legacy/v2 syntax. Diagnostics must fail-closed with location; no silent dropping.

## 4. Phase C — make the FULL differential gate a CI blocker [Tomer; Roni owns adversarial verification]

C1. Add an explicit, non-optional `go run ./tools/legacy-parity` step to the tracked `.github/workflows/build-test.yml` alongside the existing `go test -count=1 ./...` and `go test ./tools/legacy-parity/...`. A command that merely tests the gate's internal unit tests is NOT sufficient. Run the command in at least Linux and macOS jobs; verify what Nix actually executes before claiming Nix full-gate coverage. Fail the workflow on **any** baseline failure, migration failure, v2 failure, mismatched/untracked fixture inventory, unproven difference or action mismatch. CI exit status must be nonzero for these.

C2. Make the fixture corpus and result JSON reproducible: do not mask missing fixtures, overwrite baselines during comparison, rely on stale JSON, or write generated report as a side effect to a tracked path in ordinary CI without checking. Capture full diffs, coverage counts, compiler SHA, docs SHA, schema fingerprint and exact runner job URLs. The report must distinguish raw equality, normalized equality and Apple-runtime-verified behavior.

C3. Adversarial/mutation tests must prove the full gate fails if: (1) a required action parameter is removed, (2) a parameter value/type or Apple identifier changes, (3) action order/branch target/loop condition changes, (4) a trigger or metadata input type is dropped, (5) a needed UUID/output token is broken, (6) a fixture disappears, (7) a case is misclassified as proven equivalent, (8) an action is added/removed to inventory without test coverage. Include copy/paste inline regression and the prior FaceTime case. Test negative cases against **the CLI gate's exit code**, not only comparator helpers.

C4. Produce clean-run proofs for Windows (where the agent works), Linux ARM64 (where available), macOS GitHub Actions, and the Nix build path (indicate if the full gate does not run there). Do not substitute a successful Go build for a native iPad execution test. If Apple hardware validation of representative high-risk shortcuts cannot be performed, record `NOT_RUN`/`BLOCKED_DEVICE` openly and keep hardware-specific acceptance incomplete.

## 5. Phase D — coherent compiler / docs / Skill integration [Tomer; Eitan + Roni review]

D1. Once source fixes and the full gate pass, update the authoritative docs `docs/language-v2/legacy-feature-parity.md`, the structured results JSON, and the honest migration / action / performance guidance. Explicitly explain modern inline `copy/paste` vs function `Run Shortcut` overhead, including hot-loop warning and when functions are intentionally isolated.

D2. Verify the canonical `skills/cherri-shortcuts/SKILL.md`, its action catalog, LSP and app consumers against the final compiler. If external docs (cherrilang.org) require changes, prepare coordinated references/branch/PR and report cross-repo dependency; do not claim update without a real commit or verified existing coverage.

D3. Pin **one coherent, immutable release tuple**: final compiler SHA, docs SHA, manifest schema fingerprint and codec fingerprint; validate through the existing `cherri-remote-runner.sh sync --require-p10` / `status` / canonical `doctor.sh` before claiming remote-builder rollout. No guessed SHA or ad-hoc manifest edits; changing only the compatibility manifest without corresponding verified docs and built compiler is forbidden. Preserve existing P9 fallback and Legacy behavior.

D4. Update role handoffs and branch readiness documentation. No changes to default/main, no PR merge, no release/signing credential change, and no assertion that an iPad runtime check happened when it did not.

## 6. Definition of Done — ALL criteria required

- [ ] Exactly defined, reproducible fixture set with an explained and permanently guarded 47/48 discrepancy; no silent skipped fixtures.
- [ ] `go run ./tools/legacy-parity` exits **0** on clean checkout AND CI (Linux/macOS at minimum), with 0 unproven differences. No grandfathering of 20 outstanding `PARITY_DIFFERENCE` entries.
- [ ] All equivalent transformations have executable semantic evidence, not merely curated textual descriptions or loosened ignore lists.
- [ ] FaceTime, metadata, triggers, list/dict/repeat/branch/function/reference regressions protected by positive and adversarial tests.
- [ ] Full source-derived action inventory compared in both compiler paths; exact coverage denominator reported; 17/17 sample status not misrepresented as all actions.
- [ ] CI contains a standalone full CLI gate step with documented failure injection proving workflow goes red.
- [ ] `go test -count=1 ./...`, CLI parity, migration/decompiler, acceptance and relevant platform jobs pass on the same final code SHA, with raw links and exit codes.
- [ ] Source/docs/Skill schema tuple is internally consistent and, if remote rollout claimed, independently verified by runner and doctor; no stale manifest pretending to select the final compiler.
- [ ] Roni provides an **independent** QA signoff with real subagent invocation record, reproducible logs and remaining risk; Eitan and Tomer review their required scopes.
- [ ] Any unmet hardware or cross-repo verification is marked `BLOCKED`; neither parent nor child task marked DONE solely from green unit tests.
- [ ] All work committed only to feature branch; clean worktree; handoff identifies commits, CI jobs, verification artifacts, blockers and explicit owner decisions. **Do not merge or release.**

## 7. Required final handoff format

Provide a single concise report: (1) branch and base/final SHA; (2) real invocation IDs for `eitan/noam/roni/tomer`, or explicit delegation blocker; (3) fixture inventory before/after including missing-file cause; (4) per-fixture categorization and proof, 20 original differences disposition; (5) source-derived action coverage, not just sample count; (6) full standalone CLI gate exact commands/exit codes on each platform; (7) CI workflow links to jobs running the gate; (8) compiler/docs/Skill/manifest immutable tuple; (9) physical iPad test evidence or explicit `NOT_RUN`; (10) remaining blockers and tasks awaiting owner decision.

**Hard stop:** If the full CLI gate still returns 1, DO NOT claim full parity, ready-for-owner-review, release-ready or Asana completion. File an honest blocked report and continue repairing the actual technical differences.
