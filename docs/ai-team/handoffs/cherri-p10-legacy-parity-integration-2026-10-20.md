# EXECUTION PROMPT — Cherri P10 + Legacy Parity integration and verified Remote Builder cutover

**Date:** 2026-10-20  
**Repository:** https://github.com/davidpovarsky/cherri  
**Asana implementation task:** https://app.asana.com/0/1219603572114432/1219665038834438  
**Already verified parity work:** https://app.asana.com/0/1219603572114432/1219611976168814  
**Separate Apple runtime gate (keep open):** https://app.asana.com/0/1219603572114432/1219603551026304

## 0. Mission, explicit approval, and boundaries

**EXECUTE this integration; do not merely plan or return a recommendation.** The project owner has authorized you to integrate the proven Legacy-to-v2 parity changes into the existing active P10 development branch, preserve ALL P10 capabilities, and safely promote the verified, coherent compiler/docs/Skill tuple to the existing Cherri Remote Builder **after** verification. Do not ask again for approval to perform these operations within the P10 development branch. Report a real blocker rather than claiming success if access/CI/hardware is missing.

**Authorized destination:** `agent/cherri-p10-apple-actions-coverage` and its currently selected P10 Remote Builder integration, using a temporary integration branch/worktree first.

**Explicitly NOT authorized:** merging or rebasing `main`/default branch, merging other unrelated PRs, shipping a release, starting P11, altering signing credentials, using an external signing service, deleting or rewriting history, force-pushing existing shared branches, modifying unrelated apps/repos, or representing simulator/Go/plist results as physical Apple Shortcuts runtime proof. Do not trigger any iOS signing or release. Do not mark the Apple physical-device task complete.

All completed statuses require real test evidence. This task is independent of finishing the remaining 118 built-in / 173 App Intent P10 backlog; do not quietly expand scope to implement those.

## 1. Current ground truth (REFETCH — never assume frozen history)

As independently checked on 2026-10-20:

| Item | Current observed value |
|---|---|
| P10 active target branch | `agent/cherri-p10-apple-actions-coverage` |
| P10 branch HEAD | `15c22783e85dd26f9a1ddb45c542d17e67fb7cc0` |
| Legacy-parity source branch | `agent/cherri-v2-legacy-parity` |
| Parity branch HEAD | `e39feafe57e078c9b9ab16b573a13479d3da49aa` |
| Last P10/parity merge base | `3b426079dc2baecc260f27880d3fc48b68b096f3` |
| Relative divergence | P10: **1** unique commit (team setup); parity: **12** unique commits (compiler parity + docs/policy) |
| Older product-completion branch | `agent/cherri-v2-product-completion` at `27c5c0c4d933d302252b1180a7f3f92dc36144e3`, already **an ancestor** of the two active branches; do **not** blindly merge it separately |
| Previous P10 active compiler/doc pins | `dfc400d5380d9bedc3f856610708f91cf686e33f` / `a701b19d6b7ba0b8cfc9423e0a03dc2663ad104b` |
| Parity implementation compiler code pin | `96dc1036dc8b222f0954933a22d8d698dba95931` |
| Parity docs pin (reachable) | `8daeebc33b55b691493dfe75ff823c24dce44c21` in `davidpovarsky/cherrilang.org` |
| Parity schema fingerprint | `15eed22550806d167aa399fb7b95b761f540a8e9bf2fbb560cb2dd9112b0ddaa` |
| Parity codec fingerprint | `3e7e3555eefe1e030d254df0a22c62fdc3a139f92e39da34f6d841f8bfa1689e` |
| Proven parity gate in source branch | 47/47 canonical fixtures (9 direct, 38 normalized), 17/17 representative new Apple actions, 0 comparator differences |
| Source branch CI | Run `39294616584`, Linux/macOS/Nix passed on attempt 2 |
| Cherri Remote Builder location | `/home/chatgpt-agent/workspaces/cherri-remote` on paired `torah-social` (Linux ARM64) |
| Remote runner | `$HOME/workspaces/cherri-remote/bin/cherri-remote-runner.sh` |

**CRITICAL merge conflict:** both branches add `.agents/`, `.codex/`, `AGENTS.md`, `GEMINI.md`, and `docs/ai-team/` from the common base. The parity branch also introduced newer team policy with two execution modes. Do not resolve these collisions by blindly choosing one entire tree; preserve both the P10 team setup and the current two-tier agent-execution semantics. No false claims that subagents were invoked. Normal mode is the default unless the user explicitly requests team/subagents.

## 2. Mandatory bootstrap and task routing

Read in this order (from current target/source as applicable):

1. `CONTRIBUTING.md`, root `AGENTS.md`, `GEMINI.md`, `docs/ai-team/TEAM.md`, `docs/ai-team/ASANA_POLICY.md`, `docs/ai-team/BRANCH_INTEGRATION.md`, `docs/ai-team/PROJECT_ASSIGNMENTS.md`.
2. `docs/language-v2/CHERRI-V2-LIVING-MASTER-PLAN.md` and, for internal compiler work, `docs/agent-technical-reference.md`.
3. `docs/shortcut-action-development-playbook.md` and `docs/corpus-platform.md` if shared action metadata or corpus tooling changes.
4. Parity sources: `docs/language-v2/legacy-feature-parity.md`, `legacy-parity-report.md`, `tools/legacy-parity`, `internal/shortcutcompare`, `tests/fixtures-manifest.json`.
5. Selected canonical Skill: `skills/cherri-shortcuts/SKILL.md`, `compatibility-manifest.json`, `scripts/{setup,doctor,self-test}.sh`; inspect existing runner **without** bypassing its integrity checks.
6. This entire handoff and the current Asana task; inspect all actual branch SHAs and working tree status before editing.

**Role routing (not invented Asana assignees):** Tomer / `tomer` is integration lead, `eitan` resolves architecture/conflict decisions, `noam` performs focused source fixes when necessary, `roni` owns verification, `dana` reviews changes to runner/signing/security boundaries. In NORMAL execution load the appropriate role instructions yourself, label work `[MAIN_SESSION]`, and do **not** call `invoke_subagent`; use actual `[SUBAGENT]` only if the user explicitly requests TEAM mode and document the invocation ID. If a task requires truly independent QA and you lack independent reviewer access, leave that specific gate `READY_FOR_QA` instead of fabricating approval.

Use the **existing** Asana integration task `1219665038834438`. Do not create a second top-level task. Keep the verified parity task `1219611976168814` as historical proven work, and the iOS task `1219603551026304` open. Log evidence and blockers as you work, without spamming duplicate comments.

## 3. Phase A — audit source, preserve a rollback point

A1. Inspect `git status`, worktrees, local/untracked changes, `git remote -v`, `git fetch origin --prune`, `git fetch upstream`, actual branch tips and shared history. Do not discard user changes. If dirty, isolate without destructive reset and report.

A2. Capture immutable original P10 HEAD, parity HEAD, merge base, manifest SHA, runner-selected tuple, GitHub CI run IDs and 505-ish action catalog count as actually observed; use measured counts rather than asserting the catalog size beforehand. Record P10 action-provenance registry SHA, existing acceptance/coverage ledgers, decompiler and bridge baseline.

A3. Compare all touched paths. Protect P10-specific functionality and policies: the real Apple action definitions and fork action provenance, raw JSON POST with File body, native HTML file/Show Webpage route, action parameters and editor signatures, signing/preflight (without actually signing), P10 gates, machine-readable catalog, iOS Action Palette/Swift bridge, preview, static diagnostics, original Legacy mode. Do not regenerate or overwrite provenance simply because a source branch changes; update only if source behavior changed.

A4. Explicitly distinguish already-shared ancestral code from truly new source deltas. The product-completion branch is not a separate feature import unless new remote commits are independently discovered and approved.

A5. Define rollback plan preserving original target tip and runner selection. No `git reset --hard` over user changes, no `push --force`, no production-side manual file replacement, no bypassed doctor, no edited pinned checksum.

## 4. Phase B — integrate into an isolated branch/worktree (not `main`)

B1. Create an isolated worktree/branch (e.g. `agent/cherri-p10-parity-integration-20261020`) **from the current fetched P10 target HEAD**. Merge the parity branch there with `git merge --no-ff --no-commit origin/agent/cherri-v2-legacy-parity` or an equivalently auditable ancestry-preserving integration. Do NOT switch the target branch until the integrated candidate passes its gates. Do NOT cherry-pick merely a compiler file and miss the tests/migrator/metadata.

B2. Resolve conflicts at the correct abstraction level, particularly `AGENTS.md`, team routing files, role definitions, `docs/ai-team/ASANA_POLICY.md`, generated documentation, the schema, Go compiler internals, and `compatibility-manifest.json`. Reconcile **substance**, do not blindly choose `--ours`/`--theirs`, do not create duplicate sources of truth. Keep Cherri core as owner of action serialization and regenerate dependent metadata.

B3. Preserve both original (`--legacy`) and v2 language modes, actual function semantics and safe inline macro blocks; prevent reintroduction of `Run Shortcut` calls for trivial inlined code. Ensure the `TestFunctionBodyMutation_DetectedByNormalizedComparator` regression still catches changes to function defaults, math operations, recursive thresholds and body logic. Do not falsely label normalized comparison as byte-for-byte parity.

B4. Resolve Go migration/action/schema differences without losing iOS bridge, preview, decompiler, native Apple action parameters or provenance. If conflicts are too ambiguous to resolve with evidence, stop with a precise blocker instead of discarding features.

B5. Use a non-circular **two-commit** publication protocol if needed: first commit the integrated *implementation* (including resolution of conflicts, tests and generator changes) as an immutable code SHA; then make a **separate manifest/docs metadata commit** pinning that exact implementation SHA and a real reachable docs SHA. If the implemented shared schema changed, derive a fresh fingerprint from the integrated compiler, regenerate/reconcile `cherrilang.org` documentation on an approved non-default development branch, verify its commit is reachable, and then pin it; never copy an old fingerprint or guess a SHA. If unchanged, reusing the proven docs commit is permitted after verification. Compiler/docs/codecs and generated catalogs must match. The target head can differ from its pinned compiler commit due to metadata-only commits; that is intentional and must be tested.

B6. Keep the integration branch and its working tree clean; do not edit `main`, publication tags, PR #2 or unrelated branches.

## 5. Phase C — full differential and P10 regression suite on merged code

Minimum tests **on the same integrated source SHA**, not on the original parity branch alone:

```sh
go build ./...
go test -count=1 ./...
go test -count=1 ./internal/shortcutcompare/...
go test -count=1 -v ./internal/shortcutcompare -run '^TestFunctionBodyMutation_DetectedByNormalizedComparator$'
go run ./tools/legacy-parity
go run ./tools/language-acceptance
go test -run TestCherriNoSign -v .
go test -run TestDecomp -v .
go test -run TestForkActionProvenanceRegistry -v .
go test ./tools/... -v
```

Run canonical P10R2/P10R3 native producer and adversarial gates as documented in the current repository, and machine-checked test discovery (e.g. `go run ./tools/ci-test-discovery -run '^Test(CORE|CODEC|CTRL|META|LANG|IOS|GEN)'` where applicable). Do not substitute a self-referential test for a real producer-path failure case. Tests must not leave generated `.cherri` or `.shortcut` files in tracked/fixture directories. The parity runner must not change managed JSON in a normal CI run.

**Hard acceptance:** 47/47 canonical Legacy fixtures and 17/17 previously verified new-action samples still pass; zero unproven parity differences; strong adversarial mutation coverage including function body defaults/math/condition, keys/tokens/control flow, missing fixture. If corpus or provenance count legitimately changes, explain exactly why and update the contract with actual evidence—not by lowering thresholds.

**P10 preservation gates:**
- No regression to source-defined Apple action inventory (`--actions-json`), fork action provenance, catalog/schema/action signatures; verify deterministic catalog and compare before/after counts plus action IDs and parameter shape changes, not counts alone.
- Preserve P10 File-body JSON HTTP request (`WFHTTPBodyType=File`, correct `WFRequestVariable`/raw text), vCard contact chooser, RTL content, native Set Name HTML -> Show Webpage, and native location outputs. Use sanitized fixtures/p10 known examples, NEVER upload private user Shortcut raw data to public repo.
- Preserve `--legacy`, v2 syntax, module migration/`#include`, `copy/paste` inline, function `Run Shortcut` semantics, canonical list/dictionary values, branch/menu/repeat behavior and decompilation round-trip where supported.
- Preserve P10 signing/preflight safe failure: rejecting invalid `AEA1`/unsigned results and malformed native plist before any signing. **No external signing request in this task.**
- Confirm desktop/iOS Action Palette, native preview and LSP interfaces still obtain metadata from shared core, not a parallel static database. Build/check those consumers if changed, otherwise record static contract and remaining runtime gates.
- Preserve all prior P10 accepted tests and the P9 fallback behavior; do not claim completing the 118/173 unfinished coverage backlog.
- Report **actual** stdout/exit codes, exact compiler SHA, manifest/doc fingerprints, test artifacts and `git status`.

**Adversarial proof:** delete one essential FaceTime wire key, change a function body, mutate an action ID/loop condition, remove one canonical fixture, and demonstrate the full comparison/CI gate FAILS. Restore mutated inputs after each isolated test. Passing normal tests alone is insufficient.

## 6. Phase D — cross-platform CI and controlled publication to P10 branch ONLY

D1. Push the tested integration branch only after local gates are green. Run existing GitHub Actions Linux/macOS/Nix **on that exact SHA**; the Linux and macOS jobs must execute the full standalone `go run ./tools/legacy-parity`, not just the comparator unit tests. Run the canonical P10 gate workflows as applicable. Use one blocking watcher (`gh run watch <id> --exit-status`); inspect failed jobs only if failure occurs. Do not repeatedly trigger redundant jobs.

D2. Require a real QA handoff with evidence; QA checks preserved original P10 behavior AND new parity functions, not just the latter. If no independent reviewer is available, explicitly flag that gate as `READY_FOR_QA`; NEVER falsely report an agent or physical device run.

D3. **Owner has authorized merge into the P10 development branch in this request**, not into `main`. After the integration branch passes all applicable gates and P10 hasn't advanced incompatibly, update P10 by a non-force, auditable merge/fast-forward that preserves its original history; if the remote target moved, rebase/reintegrate in a fresh isolated worktree and re-run tests. Push P10 development branch normally, preserving `main`. Verify resulting P10 HEAD and manifest pins.

D4. Do NOT publish production releases, GitHub tags, IPA, TestFlight, nor `main` changes. Leave real-device Apple Shortcuts testing in its existing separate open Asana task.

## 7. Phase E — adopt the verified tuple in the existing Remote Builder

E1. Before touching any live target, record `cherri-remote-runner.sh status` current release, pinned compiler/docs/schema and the official runner/preflight SHA-256 values; back up configuration safely and record rollback path. **Do not replace the runner script or preflight helper unless a real incompatibility is proven.** Preserve integrity checks and P9 fallback.

E2. Use the existing connected `torah-social` device (Linux ARM64); work inside `$HOME/workspaces/cherri-remote`, not unrelated directories. Once the P10 branch is updated and GitHub verification is green, run:

```sh
RUNNER="$HOME/workspaces/cherri-remote/bin/cherri-remote-runner.sh"
"$RUNNER" sync --require-p10
"$RUNNER" status
```

Inspect the *selected* pinned compiler/docs/skill tuple. Run canonical `doctor.sh`, `self-test.sh` and `"$RUNNER" test` if available; check current compiler/catalog actions and fingerprint, schema docs, and that no silent P9 fallback was selected. Verify the provenance against the **newly integrated** implementation code SHA, not simply the parity branch's historical compiler pin.

E3. Recompile a minimal isolated unsigned v2 sample and a Legacy sample, plus a sanitized representative complex example if available; run native plist preflight and inspect identifiers/wire values. Record SHA-256 and output sizes. Do not sign or send user payloads to external services. Validate useful CLI coverage of `check`, `compile`, `actions`, `catalog` and `docs` through the runner.

E4. If source/build/doctor/CI fails, leave the previously working builder selected (or recover using documented non-destructive rollback), report BLOCKED and DO NOT claim cutover. No direct manual editing of pinned Git objects, manifest fingerprint, runner hash or plugin integrity settings to manufacture a pass.

E5. The current ChatGPT plugin has a separate source-pinned runner integrity contract (version 0.1.3, runner SHA256 `7ae689e9d494e7ddb9a7b9abfb5c5d96bb4321b069dc2847aec07271122cb954`, lint helper SHA256 `51ffca98be2275f4015931e15d73251fff7f7041fb323c9b7aa6973eb57a454f`). Do not require a plugin update merely for a manifest-only source pin change. If a runner change becomes necessary, treat plugin/client update as a separately verified and explicitly described dependency; never silently bypass the expected checks.

## 8. Phase F — app/docs/LSP propagation and honest reporting

- Confirm the machine-readable action catalog is the source of truth used by Skill, app bridge, LSP, action palette and rich preview. Regenerate docs where the compiler's action definitions changed; if no consumer source change is necessary, show why with catalog/schema contract comparisons rather than adding duplicate metadata.
- For `cherrilang.org`, use an existing approved feature branch/immutable commit; confirm the exact docs SHA is remotely reachable and its schema fingerprint agrees with compiled Cherri. Never change `main` in any repo.
- Maintain the original source Legacy mode and new parity features simultaneously. Provide a compatibility note about short inline `copy/paste` vs user function dispatch through `Run Shortcut` and performance for hot loops, without misrepresenting function implementation.
- Do not assert physical Apple Shorts runtime proof; macOS Go CI and plist lint are distinct. Existing task `1219603551026304` remains OPEN for simulator/physical runtime evidence.
- Do not conflate P10 integration with closing unfinished P10 action-coverage or product-completion tasks.
- Update ONLY the existing Asana task `1219665038834438` and directly relevant blockers/links per project policy: role + execution mode, branch + commit, QA evidence, CI URLs and blockers, true completion only after actual cutover/verification; no duplicate top-level tasks or false subagent claims. If something fails, leave OPEN / BLOCKED with next action. Leave parent parity work's verified scope unchanged.

## 9. Exact Definition of Done (all must be real)

- [ ] Merge ancestry and P10+parity conflict resolution audited; target P10 history, AI-team policies and existing P10 native action functionality preserved.
- [ ] Actual integrated implementation SHA committed and available in GitHub; separate non-circular manifest commit points to that exact code SHA and compatible remotely reachable docs SHA; schema and codec fingerprints objectively verified.
- [ ] Clean build, full Go tests, all P10 regression, `go run ./tools/legacy-parity` exit 0, canonical fixture manifest, 17 new-action samples, negative/mutation tests all pass on **integrated code** (0 unexplained differences).
- [ ] P10 native action contract preservation (including File POST/HTML and signing failure safeguards), shared catalog, decompiler and Skill proof; no regression of Legacy or v2 behaviors.
- [ ] GitHub Linux/macOS/Nix CI green on the integrated SHA, with full parity CLI step confirmed in Linux and macOS and relevant P10 gate runs documented.
- [ ] Actual P10 development branch updated without force push; `main` unchanged; no unrelated PR/release/tag/P11.
- [ ] Actual `torah-social` Remote Builder on-demand sync selects **new** integrated P10 compiler/docs tuple (not old `dfc400d5/a701b19d`), doctor/self-test and unsigned smoke PASS, native preflight enforced; prior fallback plan preserved.
- [ ] QA reviewer signoff is honestly attributed and evidenced, or specific gate remains OPEN/READY_FOR_QA; no false approval.
- [ ] No source/privacy leaks, external signing calls, unwanted app changes or physical-device completion claims.
- [ ] Existing Asana integration task updated with precise evidence. If any required item is not verified, leave task OPEN with explicit BLOCKED/READY_FOR_QA and explain.

## 10. Mandatory final report format

1. Source branch SHAs, original P10 SHA, integration branch SHA, final P10 SHA and exact merge base; path-level conflicts and resolutions.
2. Code/manifest/docs tuple (immutable SHA + schema + codec), action-count/catalog diff, policy/routing preservation.
3. Test table of commands + exit codes + verified artifacts, including 47 fixtures, 17 new-action samples, P10-specific native paths and adversarial failure tests.
4. CI runs/individual jobs on the final implementation + target branch HEAD; failure history if retried.
5. Remote Builder before/after branch/source/compiler/docs fingerprints, doctor, self-test, smoke outputs and fallback readiness. Never claim adoption based on an isolated install only.
6. Consumer-layer checks (LSP, iOS palette/Swift bridge, Preview, Skill/docs) with PASS/NOT_RUN/BLOCKED.
7. Genuine QA execution mode, reviewer identity or main-session attribution, concrete logs and any remaining risk; explicit statement that Apple physical runtime is still untested if that remains true.
8. Asana task link/status and remaining owner decisions (no main merge, no release).

**STOP CONDITION:** Do not report “fully integrated”, “P10 now active on the new compiler”, or mark the integration task complete unless the actual P10 branch, selected Remote Builder and all applicable gates above have been independently verified. Provide a truthful blocker report instead of optimistic summaries.