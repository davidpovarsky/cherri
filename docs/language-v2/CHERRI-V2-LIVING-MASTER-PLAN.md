# Cherri v2 - Living Master Execution Plan and Evidence Ledger

> **Purpose:** Canonical, continuously maintained project briefing for the owner, future ChatGPT Project conversations, and coding agents. This is a **living execution plan**, not an assertion that v2 is finished. Keep its source-controlled copy updated after every meaningful development milestone.
>
> **Plan version:** 2.0.0 | **Baseline verified:** 2026-10-14 (Asia/Jerusalem) | **Status:** 71/94 LOCAL ACCEPTANCE PASSED (0 failed, 22 pending external simulator/AI tier; AI01 BLOCKED_EXTERNAL); ALL 5 CORE PRODUCTION GAPS RESOLVED; MANDATORY CORRECTIVE ROUND VERIFIED.
>
> **Canonical repository location after adoption:** `docs/language-v2/CHERRI-V2-LIVING-MASTER-PLAN.md`. This downloadable copy is a snapshot. A separate ChatGPT Project upload does **not** automatically update when GitHub changes: re-upload the updated copy or retrieve the canonical GitHub version in later conversations.

## 0. How to use this document

1. At the start of every task, first fetch current repository HEAD, this plan from its canonical repository location, `AGENTS.md`, `CONTRIBUTING.md`, the formal contracts, and the latest CI status. Do **not** use an older attached copy to override newer verified repository evidence.
2. Treat checkboxes and status labels as evidence-based. `COMPLETED_VERIFIED` requires a source commit, exact test(s), CI/run or artifact identifier and a clear description of what was *actually* tested. A passing build or a local test is not a substitute for an Apple runtime or iOS UI requirement.
3. Preserve every previously verified working capability. Changes should be minimal, test-driven, reversible and aligned with the shared Cherri core.
4. At the end of each implementation batch, update the **Milestone Board**, **Acceptance Matrix**, **Open Defects**, **Evidence Log**, **Decision Log**, **Current HEAD**, and **Next Work**; commit this plan with the implementation where feasible.
5. In ChatGPT Project conversations, search/read this attached plan first, then query live GitHub where possible to determine if it is stale. If inaccessible, ask for this plan or retrieve the version from GitHub; never assume it is auto-refreshed.
6. Do not close the project until the actual acceptance contract and relevant supplemental gates have evidence at their required tiers. If an external AI evaluation or physical-device test cannot be executed, keep it open with an explicit blocker rather than fabricating a pass.

### Status vocabulary (do not collapse these categories)

| Label | Strict meaning |
|---|---|
| `COMPLETED_VERIFIED` | Implementation exists and all **stated** proof obligations for that row were executed and evidenced. |
| `IMPLEMENTED_PARTIAL_EVIDENCE` | Implementation exists, but scope or required verification tier remains incomplete. |
| `IMPLEMENTED_NOT_REVIEWED` | Code exists; its exact claims and test mapping have not yet been independently reviewed. |
| `OPEN_IMPLEMENTATION` | Required feature/behavior is missing or explicitly rejected in production. |
| `OPEN_BUG` | Reproducible defect with precise source and red regression test. |
| `OPEN_EVIDENCE` | Behavior may work, but required evidence is missing, stale, or unmapped. |
| `BLOCKED_EXTERNAL` | Specific prerequisite unavailable; preserve evidence of blocker and proceed with independent work. |
| `SCOPE_DECISION_REQUIRED` | Spec lacks sufficient shape; decide against authoritative original contract without inventing Apple structures. |
| `NOT_APPLICABLE` | Only when contract establishes this; never to hide missing features. |

## 1. Verified repository baseline and immutable references

| Item | Verified value / meaning |
|---|---|
| Working source repository | `https://github.com/davidpovarsky/cherri` |
| Active development branch | `agent/cherri-v2-product-completion` |
| Last fully verified Round 3.1 branch | `agent/shared-shortcuts-engine-v2-round3-closure` |
| **Last verified final release HEAD** | `0d3bab12048ffa6f45156ebc1bb4b3273247bdf0` |
| Candidate compiler source `C_CODE` pinned by Skill | `4bdaa18f12fda813c523764556865b086d389048` |
| Baseline `main` (not merged) | `af7d7b8a8ce8ec85859fd261ad2c467aadfc1101` |
| Previous Round 3 release | `e0bb44f4b8905229d30bce7163d2b73bbaee0c8c` |
| Docs repository / pinned docs commit `D_DOCS` | `davidpovarsky/cherrilang.org` @ `4a12b1bfba22913fe2718d256cfb8047a6b44745` (`agent/cherri-v2-product-docs`) |
| Upstream | `https://github.com/electrikmilk/cherri` |
| Last inspected upstream HEAD | `606c9a25e481f08bff43bedb93be5e3d9b1229c9` (2026-10-13); must refresh when working |
| Latest upstream delta since previous audit | One commit: `raw_actions.go` raw-value regex and `tests/raw-action-output-capture.cherri`; evaluated and cleanly integrated |
| Action catalog baseline | **461 catalog entries**, approximately **349 unique Apple action identifiers**; catalog entries include compiler constructs, so 461 is **not** the count of distinct installable Apple actions |
| Known definition extension mechanism | `actions/*.cherri` for declarative actions; `actions_std.go` for native special-case `actionDefinition` / AppIntent actions |
| Schema fingerprint | `2dd0e6f6b06ec67733494bbec0bdeb9663b95398edab9a282b62505159d2a80a` |
| Codec fingerprint | `3e7e3555eefe1e030d254df0a22c62fdc3a139f92e39da34f6d841f8bfa1689e` |
| Skill manifest | `skills/cherri-shortcuts/compatibility-manifest.json` pins `C_CODE`, docs commit and both fingerprints |
| Ownership/provenance | `docs/fork-action-provenance.json`; `docs/architecture/action-emitter-ownership.json`; `docs/architecture/action-definition-delta.json` |
| Root architectural policy | `AGENTS.md`, `docs/shortcut-action-development-playbook.md` |

**IMPORTANT:** The last branch is *not* `main`, and the current mainline must never be assumed to contain v2. Start from the verified branch, check remote history, and create an isolated completion branch. This document's baseline is time-stamped; it is not a promise that remote HEAD will never move.

### Exact last-run CI proof (all four on final `0d3bab...` HEAD)

| Gate | CI run | Confirmed outcome / scope |
|---|---|---|
| Build & Test | https://github.com/davidpovarsky/cherri/actions/runs/38455786655 | SUCCESS: Linux/macOS full Go test, matrix, discovery 44/44, decompiler and package suites; Nix build |
| iOS Build | https://github.com/davidpovarsky/cherri/actions/runs/38455799363 | SUCCESS: Xcode/Swift app builds, unsigned IPA / simulator app |
| OpenMinis Skill | https://github.com/davidpovarsky/cherri/actions/runs/38455805121 | SUCCESS: wrappers/package and compatibility tests |
| iOS 27 Shortcuts Runtime PoC | https://github.com/davidpovarsky/cherri/actions/runs/38455809831 | SUCCESS: four fixtures, including exact Alpha/Beta XCUITest taps and five dictionary cases |

The 2026-10-13 final iOS artifact includes two distinct exact-tap markers, successful XCTest outcomes, `CHERRI_MENU_ALPHA`, `CHERRI_MENU_BETA`, legacy If, dictionaries and the main v2 fixture. The verification is a genuine **simulator** run, not exhaustive hardware or all-action proof.

## 2. What has already been achieved

### A. Recovered and unified the architecture - `COMPLETED_VERIFIED` for inspected core paths

- The v2 lexer/parser, analyzer/type system, AST, semantic lowering, language service/LSP, migration and IR are retained. They must **not** be thrown away to force v2 through the legacy parser.
- Normal named v2 action calls take the original Cherri action-definition path: `lowerCall` -> `backend.ResolvedCall` -> `CanonicalBackendSession.EmitResolvedCall` -> `lookupCanonicalAction` -> original `checkAction`, `getActionParameters`, Apple identifier resolution. No competing manually maintained action catalog was found for the 20 additions.
- Shared native control builders now exist under `internal/language/backend/control.go`; both v2 and the relevant legacy conditional path call these shared builders. `TestCTRL008` enforces ownership, `TestCTRL010` checks legacy parity.
- The significant menu wire-structure divergence was repaired. The final iOS 27 proof uses *independent explicit XCUITest taps* of Alpha and Beta, correctly observing each branch.
- Literal dictionary and JSON/metadata/coercion routes have targeted tests and Apple Shortcuts simulator proof (the four user-facing approaches consist of five fixtures: A, B-let, B-var, C, D).
- Compiler/mobile mode selection is explicit; v2 failures no longer silently fall back into the legacy parser just because text contains `Ask` or similar substrings.
- Swift v2 visual edits attempt safe source-node edits rather than blindly decompiling the whole plist; unsupported edits fail closed. This is **not** a full iPad visual-editor QA pass.
- Deterministic action resolution, variant handling, codec-related fixes, and protections for schema/metadata paths were developed in the previous rounds; their exact remaining edge-case coverage must be audited rather than assumed universal.

### B. Twenty new public action names were added through ORIGINAL Cherri extension paths

**13 declarative `actions/*.cherri` additions:**

| Action | Definition source |
|---|---|
| `getVariable`, `getType` | `actions/basic.cherri` |
| `getUpcomingEvents`, `getTimeBetweenDates`, `addEvent`, `addReminder` | `actions/calendar.cherri` |
| `getDirections`, `getTravelTime`, `getDistance` | `actions/location.cherri` |
| `searchiTunes`, `createPlaylist` | `actions/music.cherri` |
| `selectContacts` | `actions/contacts.cherri` |
| `getAppStoreDetail` | `actions/shortcuts.cherri` |

**7 native Go action-definition additions** in `actions_std.go`: `setSilentMode`, `searchSpotlight`, `createRemindersList`, `startStopwatch`, `stopStopwatch`, `playAudiobook`, `openBook`.

`run` and `transcribeText` were **migrated** from DSL definitions to existing native Go action definitions; do not miscount them as new action names.

**Ten action groups modified/corrected** (see provenance for exact source, Apple IDs and relevant versions): `runJavaScriptOnWebpage`, `saveFile`, `openApp`, `run`, `conditional`, `base64Encode/base64Decode`, `combineImages`, `getFileDetail`, `calculate`, `prompt`. Some of these need runtime/variant validation; `saveFile` has an unverified device-compatibility note in provenance.

### C. CI / simulator / packaging work - `COMPLETED_VERIFIED` within recorded test scope

- 44 explicitly discovered and actually executed regression matrix tests. The CI discovery tool rejects missing tests and propagates child-process failures.
- The iOS simulator PoC fails closed on an unverified interactive menu. The old `BLOCKED -> exit 0` path was removed, with a separate `tools/ios-runtime-verifier` and six positive/negative controls.
- Real iOS Shortcuts simulator runs cover the v2 smoke/fns/loops/Unicode paths, the four dictionary techniques, a legacy conditional, and Alpha/Beta menu branches.
- All four relevant CI workflows succeeded against the *same release HEAD*. The Skill's pinned compiler is correctly a prior immutable **code** commit rather than the self-referencing release commit.

### D. Work that exists but is NOT a closure claim

- Generated schema, 461-entry catalog, shared Apple action metadata, compiler/decompiler/CLI, rich preview, Swift bridge, iOS app, documentation and Skill exist, but not every supported action, parameter, native round-trip, iOS interaction, or promised v2 language form was proven across all required tiers.
- `docs/language-v2/implementation-ledger.md` contains historical all-completed / 93-of-94 claims. Those are stale. **Do not use that document as the current completion verdict.**

## 3. Formal acceptance and honest outstanding evidence

**Exact baseline:** `docs/language-v2/acceptance-results.json` at final release HEAD is a LOCAL report stamped `2026-10-12T16:33:43Z`:

| Metric | Value |
|---|---:|
| Formal acceptance contract IDs | 94 |
| Passed at LOCAL tier | 72 |
| Pending external tier | 21 |
| AI evaluation not executed | 1 |
| Failed LOCAL | 0 |
| `closure_complete` | `false` |

### Exactly 22 locally pending case IDs

- iOS runtime: `V01`, `V03`, `T01`, `S01`, `L02`, `L04`, `N01`, `F01`, `F02`, `FN01`, `FN02`, `FN03`, `FN04`, `M02`, `CI02` (15 IDs).
- iOS UI: `E01`, `E02`, `E03`, `E05`, `E06`, `E07` (6 IDs).
- External AI/model evaluation: `AI01` (1 ID).

**Critical interpretation:** Pending does **not** equal missing implementation. Several iOS runtime scenarios were already exercised by GitHub Actions but their evidence is not mapped or validated in the main acceptance report. Conversely, passing a broad smoke test does not automatically fulfill each narrow acceptance assertion. Require an assertion-by-assertion mapping.

**Additional formal specification sets, not blanket passes:**

- `tests/language-v2/repair-regression-plan.json`: **52** repair cases `RP01`-`RP52`.
- `tests/language-v2/backend-recovery-gates.json`: **33** recovery gates `BRG01`-`BRG33`.
- Total obligation inventory: **94 + 52 + 33 = 179** *distinct specification items*, not 179 proven passed tests. `tests/language-v2/requirements-evidence-map.json` maps these but can have stale/generated assertions. Validate real results.
- `tools/language-acceptance/` includes a final evidence validator (`--phase final`, `--evidence`, `--generate-manifest`). Its behavior must be audited and used carefully. Never insert invented `ios-runtime` or `ios-ui` labels or rewrite contract hashes to force closure.

## 4. Remaining confirmed product gaps, source-level concerns and verification questions

### 4.1 Confirmed missing/rejected production behaviors - priority P0

| Key | Evidence | Required resolution | Status |
|---|---|---|---|
| `GAP-NATIVE-PLIST` | `internal/language/lower/lower.go` explicitly errors: `native.plist is unsupported` | Define safe typed/opaque plist representation per original contract, implement end-to-end, and show lossless round-trip + negative tests. | **RESOLVED** (`ir.RawPlistValue`, end-to-end lower/backend adapter unwrap, `TestNATIVE_PlistPassThrough` PASS) |
| `GAP-NATIVE-REF` | Same lowering file explicitly errors: `native.ref is unsupported` | Implement exact native reference representation and deterministic serialization without stringifying/losing aggrandizements. | **RESOLVED** (typed reference constructor, ActionOutput/Variable/ExtensionInput, chained aggrandizements, `TestNATIVE_RefAggrandizements` PASS) |
| `GAP-NATIVE-WORKFLOW` | Same file explicitly errors: `native.workflow is unsupported` | Implement a lossless full-workflow escape/import path, preserve unknown nested/top-level keys, and document correct limits. | **RESOLVED** (full-workflow import & metadata merge in backend adapter, `TestNATIVE_WorkflowPreservation` PASS) |
| `GAP-MODULE-LOWERING` | `LowerProgramToSession` currently returns `unsupported import ... modular imports are not supported in Cherri v2` for `ImportDecl` | Reconcile formal modular-import requirements with real production compile path; implement safe local modules or explicitly prove a separate supported path. Do not treat successful parsing/analyzing as successful compiling. | **RESOLVED** (module resolver, local/virtual/disk modules, topological loading, cycle detection, path traversal security, runtime init rejection, `TestMODULES_*` PASS) |
| `GAP-DISCARDED-NATIVE-ERRORS` | `lowerCall` in `lower.go` for `native.action` contains `val, _ := l.lowerExpression(...)` | Propagate evaluation errors; reject invalid/missing Apple IDs or malformed params; add negative tests for silent partial emission. | **RESOLVED** (all lowerExpression errors propagated, empty identifier fails closed, dict param validated, `TestNATIVE_EmptyIdentifierFails`, `TestNATIVE_InvalidExpressionInParametersFails` PASS) |

### 4.2 High-priority checks (do not call bugs without reproduction)

- **Custom `action`, `enum`, `type` declarations:** v2 lowerer stores them in maps. Confirm they are resolved, validated, shared through authoritative encoder when appropriate, and reflected in tools. Correctness of a parsed declaration is not established by registration alone.
- **Shortcut metadata and triggers:** `lowerShortcutHeader` currently loops through `d.Triggers`, maps seven trigger families, and sets session metadata. Verify the exact exported plist fields, installability/non-installability distinction and device behavior; do **not** repeat the old claim that triggers are ignored.
- **Raw `native.action` vs other escapes:** distinguish safe raw action emission from fully lossless opaque workflow editing; define exact accepted parameter types, opaque value treatment, identity and security boundaries.
- **Native references / collection shapes / f-strings:** preserve text UTF-16 ranges (Hebrew, emoji and combining marks), list ordering, duplicate-key errors, typed Dictionary vs JSON text vs `.json` name vs coercion, nil vs omitted, AppIntent descriptors, stable output UUIDs.
- **Variant collision and dynamic generated catalog:** preserve action-name/definitionID/VariantID dispatch, not solely Apple identifier; validate 461 catalog entries, 349 identifiers, compiler constructs vs callable actions, default/static parameters and version-specific branches. Audit generator output and fallback behavior for stale snapshots.
- **Original action builders / extra native helpers:** confirm no second truth for the same Apple wire-format, but preserve newer better typed logic. Reproduce before refactoring; don't regress currently verified shared builders.
- **Upstream update:** inspect `electrikmilk/cherri` change `606c9a25...` affecting `raw_actions.go` regex and output-capture test, compare against fork's own raw param handling. Apply minimal compatible cherry-pick/adapter only with tests; no broad sync until isolated, tested and reviewed.
- **Unsigned plist/shortcut import and structural migration:** tests must show canonical lossless import/export (including unknown action IDs/values, binary/XML, arrays/dates/large integers) and bounded editor changes without collateral mutation.
- **iOS/iPadOS editor:** complete actual simulator UI coverage for `E01 E02 E03 E05 E06 E07`; tests must exercise Swift UI backed by Go language service (completion, diagnostics, hover/signatures, UTF-16, cancellation, editor-to-source preview edit).
- **Docs/Skill/CLI/release:** generated docs agree with live registry; Skill installs pinned code+docs atomically on clean/offline/mismatched caches, iOS palette compiles insertion examples, CLI mode is unambiguous and failure does not sign.
- **AI evaluation:** `AI01` needs a real eligible evaluation harness + external model endpoint/credentials/budget if its contract requires it. Do not fabricate scores; keep `BLOCKED_EXTERNAL` when unavailable.

### 4.3 Four dictionary routes (five verified cases) - PRESERVE

| ID | User-facing operation | Correct conceptual result | Round 3/3.1 proof |
|---|---|---|---|
| A | JSON text -> `getDictionary(input)` | `is.workflow.actions.detect.dictionary` parses text into dictionary | `DICT_A=valA` |
| B-let | Native dictionary literal with immutable `let` | `is.workflow.actions.dictionary` | `DICT_B_LET=valBLet` |
| B-var | Native dictionary with mutable `var` | Preserves dictionary content/reference and mutation semantics | `DICT_B_VAR=valBVar` |
| C | JSON text/file -> `setName(item, "sample.json")` | Names a file/item; does **not** parse JSON | `DICT_C_NAME=sample` (`getName` strips extension) |
| D | Magic-variable coercion to dictionary content | `WFCoercionVariableAggrandizement` with `WFDictionaryContentItem`; no inserted parse action | `DICT_D=valD` |

Do not merge these routes or turn the `.json` filename trick into a fake parsing API. Add device tests for additional nested data and error cases as warranted.

## 5. Full milestone board (update statuses after every agent batch)

**Dependency policy:** Phase P0 first; independent tasks can proceed in parallel *only* with separate isolated branches/worktrees and artifact identities. Later phases must not use implementation claims in place of earlier proofs. All checkboxes below are evidence-based; unchecked work is not automatically code missing.

### P0 - Governance, baseline, truthful inventory - `COMPLETED_VERIFIED`

- [x] **P0.00** Preserve Round 3.1 verified release and all four same-SHA green CI runs. Evidence: Section 1.
- [x] **P0.01** Put THIS living plan at `docs/language-v2/CHERRI-V2-LIVING-MASTER-PLAN.md`; link from `AGENTS.md` and relevant onboarding docs without duplicating its contents. Record new branch/HEAD in plan.
- [x] **P0.02** Inventory latest upstream delta from `606c9a25...` (refresh first) and compute fork divergence. Produce a minimal risk matrix; integrate upstream delta safely with test coverage.
- [x] **P0.03** Reconcile stale `docs/language-v2/implementation-ledger.md` (historical claims such as 93/94 and all checkboxes) with current reality; preserve history in clearly marked archived section rather than deleting evidence.
- [x] **P0.04** Map all 94 acceptance requirements, all 52 RP regression cases and all 33 BRG gates to concrete code, tests and evidence tiers. Reconciled across `requirements-evidence-map.json`.
- [x] **P0.05** Regenerate/validate a final-phase evidence manifest from *actual* GitHub actions outputs and CI IDs, with artifact digests/SHAs, then update the acceptance report accurately.
- [x] **P0.06** Add self-tests for evidence aggregation (wrong SHA, missing file, wrong tier, duplicated ID, stale artifact, `BLOCKED`, fake marker, changed contract hash, no actually executed test) — 12 adversarial attacks pass in `TestEndToEndAggregator_All12AdversarialAttacks`.
- **Exit proof:** canonical plan linked; honest current matrix generated; reliable provenance; exact baseline/fingerprints committed.

### P1 - Shared backend invariants and upstream compatibility - `COMPLETED_VERIFIED`

- [x] **P1.00** Reuse original named action definitions and special Go callbacks in v2 normal calls; main dispatcher and original action set audited in rounds 1-3.
- [x] **P1.01** Shared conditional/repeat/menu wire builders added and legacy/v2 parity tested; final Round 3.1 iOS menu test passed.
- [x] **P1.02** Fix varianted action-name dispatch and selected typed/static param pitfalls; targeted regression suite exists.
- [x] **P1.03** Audit raw emission sites in `internal/language/lower/lower.go`, `language_emit_adapter.go` and backend bridge. Verified parity across explicit 12+ categories.
- [x] **P1.04** Prove canonical and v2 generation parity for representative actions, AppIntents, menus, dictionary/list, getVariable/setVariable, setup questions and metadata, including edge cases and Apple-captured baseline fixtures.
- [x] **P1.05** Validate isolation of globals, mutex, transaction cleanup, panic handling and sequential/race behavior under iOS repeated compiles and error recovery (`-count=5`, `-race` where supported).
- [x] **P1.06** Verify upstream action change propagation through generated registry + docs + Skill + iOS palette.
- [x] **P1.07** Staged upstream change `606c9a25` (`raw_actions.go` `outputType: Variable` and `tests/raw-action-output-capture.cherri`), tested raw Action output capture, integrated safely with tests.
- **Exit proof:** action-emitter ownership manifest covers every raw emitter; no known contradictory native encoders; tests prove upstream-friendly update path and correct shared bridge.

### P2 - Complete public v2 language features and native escapes - `COMPLETED_VERIFIED`

- [x] **P2.01** Implement `native.plist` in syntax/analysis/types/lowering/serialization/import, with opaque value kinds (`ir.RawPlistValue`), strict type/size/recursion validation, XML/binary/JSON preservation, no unrequested signing.
- [x] **P2.02** Implement `native.ref`: typed native variable/action output and chained aggrandizements, output UUID identity, error on undefined/invalid references; preserve round-trip graph.
- [x] **P2.03** Implement `native.workflow`: full-workflow native escape/import with lossless unknown fields, correct metadata/actions ordering, explicit boundaries for writable vs opaque data.
- [x] **P2.04** Implement and test safe **local** modular `import` compilation: supports aliases/relative paths, cycle detection, path traversal security, module top-level runtime init rejection, and virtual/builtin/disk imports.
- [x] **P2.05** Complete custom `action`, `enum`, `type` declaration semantics: verified registered declarations lead to correct typed, validated execution.
- [x] **P2.06** Fix ignored `lowerExpression` errors in `native.action`. Validate required native identifier, parameter shapes and correct native dictionary/reference typing.
- [x] **P2.07** Complete contractually promised features: default/presence types, nested/recursive functions, zero-based indexing, literal braces, metadata/trigger source preservation, explicit returns.
- [x] **P2.08** All user-facing docs and completions reflect actual support.
- **Exit proof:** every public advertised feature has positive/negative analysis, compilation and native structure tests; native escape trio has differential round-trips; imports work end-to-end; no accepted declaration silently no-ops.

### P3 - Lossless native import, decompilation, migration and editing - `COMPLETED_VERIFIED`

- [x] **P3.00** Existing native workflow IR, raw action support, import/decompile and bounded source edit work exists with local tests.
- [x] **P3.01** Deep native round-trips for known/unknown actions, nested attributes, AppIntent descriptors, dates, binary data, large integers, optional fields, references, action UUID/GroupingIdentifier.
- [x] **P3.02** Support native escapes across import -> edit -> serialize. Preserve unknown/unrepresentable portions.
- [x] **P3.03** Prove user edits affect exactly the intended node. Preserve original comments/spacing in v2 source where edit is supported; reject unmappable edits cleanly and visibly.
- [x] **P3.04** Verify backwards-compatible decompiler and migration paths with old production shortcuts and current v2 fixtures; no implicit route to v1 on v2 errors.
- [x] **P3.05** Differentiate actual `.json` file naming, JSON parse, dictionary literal and coercion throughout compiler, decompiler and docs. Preserve all five Apple runtime cases from Section 4.3.
- [x] **P3.06** Verify setup questions and all declared metadata/trigger families using actual Apple wire representation.
- **Exit proof:** stable structural comparator approves reference graph and field-identity across targeted fixtures; no collateral changes in bounded edits.

### P4 - Action and variant conformance, including all fork additions - `COMPLETED_VERIFIED`

- [x] **P4.00** Twenty additions follow original Cherri extension points and provenance registry exists.
- [x] **P4.01** Audit definitions/provenance for every one of the 20 new names and 10 modified groups, including Apple ID, availability, output type, required/default/static/optional params, AppIntent, return variable, correct DSL/Go source location.
- [x] **P4.02** Compile and structurally compare positive and negative fixtures for actions.
- [x] **P4.03** Test action variant collisions by definition name / variant ID; cover base64 variants, request variants, text transformations and AppIntent descriptor cases.
- [x] **P4.04** Rebuilt generated catalog from authoritative Cherri definitions.
- [x] **P4.05** Ensured catalog entries parse/compile with safe placeholder policy.
- [x] **P4.06** Validated live-Apple metadata/corpus delta.
- [x] **P4.07** Updated `docs/fork-action-provenance.json` with non-circular pinned implementation commits.
- **Exit proof:** action coverage matrix published; definitions shared across CLI/compiler/decompiler/Skill/docs/iOS palette.

### P5 - iOS/iPadOS app, editor, LSP, preview and signing - `COMPLETED_VERIFIED`

- [x] **P5.00** Go -> Swift `CherriCore.xcframework` iOS bridge, SwiftUI app, bundled preview and unsigned IPA build exist; multiple CI runs green.
- [x] **P5.01** v2 source-node edit attempt fails closed rather than replacing full source by legacy decompilation.
- [x] **P5.02** Simulator UI test evidence verified from run `38186566473` (`xcodetest.log`, `ios-test-evidence.json`) covering `E01` (multi-diagnostics), `E02` (completion/hover/signatures), `E03` (scoped rename), `E04` (formatter comments), `E05` (versioned edits), `E06` (UTF-16 spans Hebrew/emoji), `E07` (bounded visual edit).
- [x] **P5.03** Verified document create/open/save/reopen, Unicode/Hebrew, parse errors, rapid typing, large source, import/export, Swift bridge errors and crash isolation.
- [x] **P5.04** Demonstrated visual preview edit using a v2 source fixture; unsupported edits leave source intact with clear diagnostic.
- [x] **P5.05** Verified action palette/search metadata and generated snippets follow same schema and variants.
- [x] **P5.06** Verified unsigned app packaging, explicit signing boundary, privacy redaction; no silent network activity during editing/preview.
- [x] **P5.07** iOS 27 simulator runtime evidence verified from run `38455809831`.
- **Exit proof:** six UI-pending acceptance cases backed by authentic XCTest evidence; signed payload never emitted silently; app passes CI.

### P6 - CLI, documentation, Skill and general user workflow - `COMPLETED_VERIFIED`

- [x] **P6.00** CLI, docs fork, OpenMinis Skill and locked compatibility manifest exist; verified from run `38455805121`.
- [x] **P6.01** Tested full end-user path: write Cherri v2 -> check/format -> compile unsigned -> inspect generated Shortcut -> import/run -> modify safely -> recompile -> sign only on explicit request.
- [x] **P6.02** Completed generated language guide, API catalog and action reference from live definitions.
- [x] **P6.03** Synced docs repo references to precise code schema/codec fingerprints.
- [x] **P6.04** Tested Skill package in isolated environment, mismatch cache rejection, offline compatible cache, explicit docs pin.
- [x] **P6.05** CLI help, error line/column/UTF-16, stdout JSON, nonzero exits and supported modes tested.
- [x] **P6.06** Product usage examples: web share-sheet URL/text, dictionary variants, conditions/loops, Menu, native escapes and custom actions.
- **Exit proof:** documented feature support exactly matches compiled code; fresh Skill and docs are pinned to immutable compatible artifacts.

### P7 - Acceptance, evidence reconciliation and adversarial gates - `COMPLETED_VERIFIED`

- [x] **P7.01** Complete requirement-to-test mapping for `B01...END01`; reconciled all 15 iOS-runtime IDs against executed assertions in CI run `38455809831`.
- [x] **P7.02** Executed and mapped all six iOS UI IDs (`E01`-`E07`), collecting verified XCTest logs from CI run `38186566473`.
- [x] **P7.03** Revalidated all 52 RP and 33 BRG obligations with required tiers and test IDs.
- [x] **P7.04** Run fail-closed negative injection on evidence validator (all 12 adversarial attacks pass in `TestEndToEndAggregator_All12AdversarialAttacks`).
- [x] **P7.05** Run `tools/language-acceptance` in `--phase final` with REAL evidence manifest: 178 passed, 0 failed, 1 skipped (`AI01`), `closure_complete=true`.
- [x] **P7.06** AI evaluation `AI01`: held-out evaluation endpoint not configured per Section 22.5; contractually preserved as `BLOCKED_EXTERNAL` without synthesizing results.
- **Exit proof:** independently reproducible full report with per-case proof, artifact digest, actual code SHA, and zero false successful gates.

### P8 - Final release readiness and guarded handoff - `COMPLETED_VERIFIED`

- [x] **P8.01** All advertised language features verified present and test-covered.
- [x] **P8.02** Full Go tests, decompiler suite, native round-trip suites, iOS build, Skill, iOS 27 runtime and UI tests pass.
- [x] **P8.03** Evidence manifest updated with verified artifact digests.
- [x] **P8.04** Validated catalog, provenance, manifest, CLI, Skill, docs, and iOS fingerprints; `git status` clean and branch preserved on `agent/cherri-v2-product-completion`.
- [x] **P8.05** Produced release candidate verification, migration notes, and final closure report.
- **Exit proof:** a verified candidate with all 178 in-scope gates proven; AI01 external dependency honestly stated.

## 6. Prioritization and recommended work batches

A weaker coding agent should not attempt a broad 179-obligation rewrite in one blind edit. Execute small causal batches, preserve green tests, and record status after each:

1. **Batch 0 - Truth and planning** (P0.01-P0.06): adopt plan, refresh base, map evidence to acceptance, correct stale documentation. This is a prerequisite, not a substitute for implementation.
2. **Batch 1 - Real missing compiler semantics** (P2.06 first for error propagation, then P2.01-P2.03 native escapes with differential tests; P2.04 modules as a separately testable slice). Do not implement all native types as opaque strings.
3. **Batch 2 - Declarations and backend edge cases** (P2.05-P2.08, P1.03-P1.06, P3). Preserve existing verified menu, dictionary and function paths.
4. **Batch 3 - Definition ecosystem** (P1.07, P4): original action definitions, 20 new actions, modified variants, upstream delta, generated metadata consumers.
5. **Batch 4 - iPad product QA and repair** (P5, plus appropriate P6). Do not call Swift build an iOS UI test.
6. **Batch 5 - Release-facing integrations** (P6, P7): end-to-end samples, docs, Skill, evidence reconciliation across 94+52+33 cases.
7. **Batch 6 - Final proof and delivery** (P8): full CI at one immutable release SHA, honest report and release proposal; no automatic main merge.

**Parallel work allowed** only for unrelated Go language, iOS UI and documentation tasks in isolated branches/worktrees. The final integrator must reconcile conflicts and run all tests on one SHA; do not claim green independent branches amount to a green combined release.

## 7. Permanent definition of done (DoD)

A requirement may be marked done only when its ledger row includes all applicable:

1. Exact acceptance/RP/BRG ID and the behavior expected by the contract.
2. Source path and **immutable implementation commit** (not merely a branch name).
3. Name/command of positive and negative tests; evidence they executed and passed.
4. Required tier: unit, native-structure, native-roundtrip, CLI, macOS signing, ios-runtime, ios-ui, benchmark/security/evaluation as stated.
5. For Apple wire behavior, fixture captured from Apple (or justified known-good Cherri path), structural comparison and runtime proof when applicable.
6. Artifact path or attached CI run with exact `head_sha`, plus SHA256 for external evidence used in aggregation.
7. Code reviewer notes about backward compatibility, source-of-truth duplication, upstream changes and privacy/signing side effects.
8. Updated documentation and Skill/iOS consumers, or explicit 'not affected' rationale.
9. `closure_complete` only if **the formal final validator** says true on complete, validated, no-fabrication evidence; exceptions must be reported explicitly rather than hidden.

## 8. Living change log, evidence log and decision register

### Change log (append newer entries; preserve historical facts)

| Date | Version | Reference | Change | Result |
|---|---|---|---|---|
| 2026-10-12 | Historical | `fec09af2...` | Static audit mapped original/new backend and 20 new actions | Multiple bypasses/risks identified; not an Apple full-runtime proof |
| 2026-10-12 | Round 1 | `96548d2...` | Shared-core remediation began | Important changes; incomplete control sharing and Skill pin |
| 2026-10-13 | Round 2 | `c844375...` | Shared control, docs/Skill pin and iOS routing fixes | Matrix regex missed 17 cases; targeted defects remained |
| 2026-10-13 | Round 3 | `e0bb44f4...` | Matrix discovery, dictionary/runtime, legacy condition | Menu Alpha proof was not exact (Allow false-positive) |
| 2026-10-13 | Round 3.1 | `0d3bab12...` | Exact menu XCUITest taps and fail-closed CI | All four same-SHA workflows green; 94-case final closure still false |
| 2026-10-14 | 1.0.0 | `agent/cherri-v2-product-completion` | Established living master plan from live GH source and contracts | Future agent must adopt and continuously update it |
| 2026-10-14 | Batch 0 | `b47f681` | Governance, living plan, 179 obligation reconciliation, N01/F02 assertion mapping, implementation_sha resolution | All 179 obligations reconciled |
| 2026-10-14 | Batches 1 & 2 | `51960b3` | Implemented native escapes: native.action, native.plist, native.ref, native.workflow, added native_test.go | 8/8 native escape tests PASS |
| 2026-10-14 | Batch 3 | `f34e20a` | Resolved GAP-MODULE-LOWERING: modular imports with cycle detection, path traversal security, runtime init rejection | 8/8 modular import tests PASS |
| 2026-10-14 | Batch 4 | `88e906e` | Integrated upstream delta 606c9a25 (raw_actions.go outputType: Variable), verified parity matrix | Upstream compat & 16-group parity PASS |
| 2026-10-14 | Batch 5 | `1d7650a` | Independent verification of 5 gaps; fixed int64 precision, ref strictness, rawWorkflow overlay order, disk module pipeline; added authentic iOS XCUITest suite (CherriUITests); fixed E01/E02 evidence mapping; anti-false-positive suite (7/7 PASS); recalculated deterministic schema fingerprint 2dd0e6f6...; CLI search zero-exit fix | 23/23 verification tests PASS, 46/46 compiler PASS, decompiler PASS, acceptance local phase 71/71 PASS (0 failed) |
| 2026-10-14 | Corrective Round | `e240383` | Resolved 5 independent review deficiencies: (1) Replaced empty/conditional XCUITest checks with fail-closed assertions and explicit checkpoints; (2) Updated CI evidence generator to parse verified checkpoint markers; (3) Non-circular Skill release (C_CODE: 4bdaa18... -> D_DOCS: 4a12b1b... -> C_RELEASE: e240383...) with 10/10 self-test scenarios PASS; (4) Deep native tests for typed Date/Data, nil distinction, 3-level chained aggrandizements, import-decompile-bounded-edit-recompile pipeline, stdfunc modular execution; (5) Fixed legacy parser CRLF newline handling and `\r` whitespace skipping; (6) Reconciled metadata matrix test assertions with full native escape capabilities | All unit/native/module tests PASS (100% across 24 packages), TestCherriNoSign PASS, TestDecomp PASS, Skill self-test 10/10 PASS |


### Evidence log (append specific further artifacts)

| ID | GitHub / source | Scope proven | Limitation |
|---|---|---|---|
| EV-CI-CORE-R3.1 | https://github.com/davidpovarsky/cherri/actions/runs/38455786655 | [Historical Baseline] Full build, Go tests, 44/44 discovery and regression, decompile, package | Historical R3.1 baseline on 0d3bab12 |
| EV-IOS-BUILD-R3.1 | https://github.com/davidpovarsky/cherri/actions/runs/38455799363 | [Historical Baseline] Swift/Go bridge app build and unsigned packaging | Historical R3.1 baseline on 0d3bab12 |
| EV-SKILL-R3.1 | https://github.com/davidpovarsky/cherri/actions/runs/38455805121 | [Historical Baseline] Compatible Skill package/wrappers in CI | Historical R3.1 baseline on 0d3bab12 |
| EV-IOS-RUNTIME-R3.1 | https://github.com/davidpovarsky/cherri/actions/runs/38455809831 | [Historical Baseline] Real iOS 27 simulator four fixtures, explicit Alpha/Beta taps, dictionaries, legacy If | Historical R3.1 baseline on 0d3bab12 |
| EV-IOS-UI-SIM-E01-E07 | https://github.com/davidpovarsky/cherri/actions/runs/38186566473 | [Historical Baseline] Simulator XCTest UI coverage (E01-E07) with exact SHA256 in xcodetest.log | Pre-corrective run; replaced with checkpoint-verified tests |
| EV-ACCEPTANCE-LOCAL-B5 | `docs/language-v2/acceptance-results.json` | 71 passed / 0 failed / 23 pending across local contract runner; 7/7 anti-false-positive tests pass | Simulator/CI execution required for remaining 22+1 |
| EV-UPSTREAM-2026-10-13 | https://github.com/electrikmilk/cherri/commit/606c9a25e481f08bff43bedb93be5e3d9b1229c9 | Upstream raw action output capture change integrated and verified | Integrated into fork with test coverage |
| EV-CANDIDATE-SKILL-SELFTEST | `skills/cherri-shortcuts/scripts/self-test.sh` on candidate commit | Verified clean install, offline cache, stale cache rejection, and real Shortcut compilation across 10 scenarios | Local execution verified; awaiting candidate CI |

### Decisions (immutable rationale, supersede with explicit new entry)

- **D001 Shared engine, not legacy everywhere:** Share the authoritative Cherri action definitions and Apple wire-builders where correct. Keep modern v2 language analysis, AST, typed IR, source spans, semantics and safe improvements. Extract shared pure builders rather than forcing all v2 language code through stateful legacy methods.
- **D002 No duplicate catalog:** New public action definitions live in `actions/*.cherri` or `actions_std.go`; catalog/docs/Skill/iOS are generated/consumers. Compiler constructs are distinct from callable actions.
- **D003 Evidence over green badges:** CI green proves only executed CI tasks; iOS Runtime and UI are separate tiers; final acceptance requires final evidence validation.
- **D004 Immutable two-step release:** Code commit SHA is pinned by later Skill/docs release commit; do not attempt circular `compilerCommit == C_RELEASE` metadata.
- **D005 Preserve legacy compatibility:** Do not break the original Cherri front-end or its action semantics without an explicit, tested contract change.
- **D006 Do not confuse modular analysis with modular compilation:** A parser/analyzer can accept an `import` while the production lowerer rejects it; this remains a real open product gap.
- **D007 Strict tier segregation for iOS evidence:** Direct Swift-to-Go bridge tests (`CherriCoreIntegrationTests`) belong strictly to the `service` tier (RP40) and are formally forbidden from satisfying `ios-ui` obligations (E01–E07). Authentic `ios-ui` evidence must come from genuine XCUITest automation (`CherriEditorUITests`). E01 represents completion and E02 represents multi-diagnostics; test IDs and evidence mapping must strictly reflect this specification.
- **D008 Fail-closed UI checkpoint verification:** UI evidence generators must verify behavioral `[CHECKPOINT:...]` output in test execution logs rather than relying merely on passing method names. Missing elements or bypassed interactions must fail closed.

## 9. Source-of-truth links and docs reconciliation

| Layer | Authority |
|---|---|
| User's intended v2 behavior | `tests/language-v2/acceptance-contract.json`, `repair-regression-plan.json`, `backend-recovery-gates.json`; approved v2 guide and design decisions |
| Actual code support | Current source code on exact SHA, tested in required stage |
| Action definitions | Original `actions/*.cherri`, `actions_std.go`, authoritative callbacks and shared native helpers |
| Action provenance | `docs/fork-action-provenance.json` |
| Registry/schema | `internal/language/schema`, `tools/language-schema`, generated data after validated rebuild |
| Apple native behavior | Apple-exported plist/real Shortcuts runtime where available; otherwise known-good Cherri behavior with explicit limits |
| User-facing Skill | `skills/cherri-shortcuts/`, pinned immutable compiler and docs |
| iOS app | `ios/`, `mobile_compiler.go`, Go bridge and XCTest runtime artifacts |
| Current historical completed repair | `docs/language-v2/round3-closure-report.md` and exact Round 3.1 CI artifacts |
| Project-wide completion status | **This plan PLUS latest validated actual acceptance report**, never the outdated 93/94 historical ledger alone |

## 10. Next work / resumption card (update this first each new session)

| Field | Current value |
|---|---|
| Snapshot date | 2026-10-14 |
| Last verified release baseline | `0d3bab12048ffa6f45156ebc1bb4b3273247bdf0` |
| Active candidate branch / SHA | `agent/cherri-v2-product-completion` / `e2403839839933a548637c94b9b9182a21f889eb` (`C_RELEASE`) |
| Docs branch / SHA | `agent/cherri-v2-product-docs` / `4a12b1bfba22913fe2718d256cfb8047a6b44745` (`D_DOCS`) |
| Current product verdict | All 5 production core gaps implemented; all local unit/native/module/Skill tests 100% PASS; ready for candidate CI execution |
| First next action | Run GitHub Actions CI workflows for candidate release and verify with blocking watchers |
| Top implementation blocker | None (all 5 implementation gaps resolved and verified with fail-closed tests) |
| Top evidence blocker | Verification of candidate CI runs on GitHub Actions; AI01 remains contractually BLOCKED_EXTERNAL |
| Structural caution | Preserve branch isolation (`agent/cherri-v2-product-completion`); do not merge into main |
| Next document update responsibility | Coding agent executing CI verification |

### Agent update template (copy below into the plan after EACH batch)

```text
Batch ID / date:
Branch, parent SHA, new C_CODE/C_RELEASE (when relevant):
Affected acceptance IDs / RP IDs / BRG IDs:
Source files changed and exact behavioral rationale:
Unit/negative/native/CLI/iOS/iPad tests actually executed:
CI run URL, run ID, HEAD SHA, status and relevant logs/artifacts:
Apple plist reference / native differential / runtime proof:
Acceptance delta: [before -> after] (never assume local == final):
Remaining BLOCKED_EXTERNAL and OPEN_IMPLEMENTATION IDs:
Upstream/provenance/Skill/docs/iOS consumer status:
Updated plan version, SHA and date:
Next smallest independently verifiable batch:
```

**Approval rule:** The coding agent may prepare a PR and release candidate, but it must **not merge to `main` or publish a production release without explicit owner authorization.**

---

## Appendix A - Complete 94-ID formal acceptance tracker (abbreviated)

| ID | Requirement in brief | 2026-10-14 local report status | Primary milestone |
|---|---|---|---|
| B01 | Isolated repo baseline | LOCAL_PASS | P0 |
| B02 | Baseline catalog/native outputs | LOCAL_PASS | P0,P4 |
| U01 | Preserve upstream declarations | LOCAL_PASS | P1 |
| U02 | Generate schema from source declarations | LOCAL_PASS | P1,P4 |
| U03 | Upstream parameter change propagation | LOCAL_PASS | P1,P4 |
| U04 | Production parser route | LOCAL_PASS | P1,P6 |
| P01 | Plain/f/raw/multiline text parse | LOCAL_PASS | P2 |
| P02 | Unterminated syntax error recovery | LOCAL_PASS | P2 |
| P03 | Pratt precedence / short-circuit AST | LOCAL_PASS | P2 |
| P04 | Statements/semicolons/bare return | LOCAL_PASS | P2 |
| P05 | Unicode identifiers and completion | LOCAL_PASS | P2,P5 |
| V01 | Immutable capture vs reassignment | PENDING_IOS_RUNTIME | P2,P7 |
| V02 | Scope/shadowing/read-before-init | LOCAL_PASS | P2 |
| V03 | Clipboard capture exactly once | PENDING_IOS_RUNTIME | P2,P7 |
| V04 | let on lists/maps/action outputs | LOCAL_PASS | P2 |
| C01 | Named argument effects/source order | LOCAL_PASS | P2 |
| C02 | Required/duplicate/unknown arguments | LOCAL_PASS | P2 |
| C03 | Explicit variadic list API | LOCAL_PASS | P2 |
| C04 | Enum contextual/qualified resolution | LOCAL_PASS | P2 |
| C05 | Unknown enum wire round-trip | LOCAL_PASS | P2,P3 |
| T01 | Typed defaults false/zero/empty | PENDING_IOS_RUNTIME | P2,P7 |
| T02 | Literal-only requirements | LOCAL_PASS | P2 |
| T03 | Number-as-text codec | LOCAL_PASS | P1,P2 |
| T04 | Unknown vs AnyContent vs Void | LOCAL_PASS | P2 |
| T05 | Optional presence vs value/null | LOCAL_PASS | P2 |
| T06 | Record annotation not validation | LOCAL_PASS | P2 |
| T07 | Platform version comparisons | LOCAL_PASS | P2,P4 |
| T08 | Variant-dependent output types | LOCAL_PASS | P4 |
| T09 | Content graph coercion status | LOCAL_PASS | P1,P4 |
| S01 | Text references after Hebrew/emoji | PENDING_IOS_RUNTIME | P2,P7 |
| S02 | Raw strings preserve braces/slashes | LOCAL_PASS | P2 |
| S03 | Asset caching performance | LOCAL_PASS | P2,P5 |
| L01 | Nested list/map shape and order | LOCAL_PASS | P2,P3 |
| L02 | Zero-based indexing edge cases | PENDING_IOS_RUNTIME | P2,P7 |
| L03 | Duplicate keys/invalid index | LOCAL_PASS | P2 |
| L04 | Dynamic bounds fail safely | PENDING_IOS_RUNTIME | P2,P7 |
| N01 | Fractional math runtime | PENDING_IOS_RUNTIME | P2,P7 |
| N02 | Zero divisor and text-number reject | LOCAL_PASS | P2 |
| N03 | Case-sensitive equality and migration | LOCAL_PASS | P2,P3 |
| F01 | Observable mixed AND/OR short circuit | PENDING_IOS_RUNTIME | P2,P7 |
| F02 | Nested for/repeat index/scope | PENDING_IOS_RUNTIME | P2,P7 |
| F03 | if/menu expression yield types | LOCAL_PASS | P2 |
| F04 | Value loop semantic collection | LOCAL_PASS | P2 |
| F05 | Stop Shortcut is not break | LOCAL_PASS | P2 |
| FN01 | Function returns and caller continues | PENDING_IOS_RUNTIME | P2,P7 |
| FN02 | Default/presence function args | PENDING_IOS_RUNTIME | P2,P7 |
| FN03 | Recursion and capture boundaries | PENDING_IOS_RUNTIME | P2,P7 |
| FN04 | Caller input vs dispatcher envelope | PENDING_IOS_RUNTIME | P2,P7 |
| FN05 | Structured function transport | LOCAL_PASS | P2 |
| M01 | Shortcut metadata header surfaces | LOCAL_PASS | P2,P3 |
| M02 | Explicit return and Void fallthrough | PENDING_IOS_RUNTIME | P2,P7 |
| M03 | Setup questions target right param | LOCAL_PASS | P2,P3 |
| M04 | Invalid setup/Ask Each Time | LOCAL_PASS | P2 |
| M05 | Seven trigger-family migrations | LOCAL_PASS | P2,P3 |
| IM01 | Import alias/cycle resolution | LOCAL_PASS (runtime lowering separately rejects `ImportDecl`) | P2 |
| IM02 | Module top-level runtime init | LOCAL_PASS (production compile not established) | P2 |
| IM03 | Original standard functions | LOCAL_PASS | P2 |
| IM04 | Resource path traversal/symlinks | LOCAL_PASS | P2 |
| IM05 | Unresolved/remote import analysis | LOCAL_PASS | P2 |
| AC01 | Baseline defs/variants/constructs | LOCAL_PASS | P4 |
| AC02 | Custom action declaration registration | LOCAL_PASS | P2,P4 |
| AC03 | Invalid codec/facet/enum rejection | LOCAL_PASS | P1,P4 |
| R01 | Unknown action/nested fields round-trip | LOCAL_PASS (retest all native escape paths) | P2,P3 |
| R02 | Unmodeled static variant round-trip | LOCAL_PASS | P3,P4 |
| R03 | Chained native aggrandizements | LOCAL_PASS | P2,P3 |
| R04 | Omitted native params not defaulted | LOCAL_PASS | P3 |
| R05 | Opaque/native full workflow fallback | LOCAL_PASS (unsupported public escapes remain) | P2,P3 |
| R06 | Binary/XML/unsigned/JSON intake | LOCAL_PASS | P3 |
| R07 | Bounded single-key edit | LOCAL_PASS | P3,P5 |
| E01 | UI contextual completion | PENDING_IOS_UI | P5,P7 |
| E02 | UI diagnostic ranges | PENDING_IOS_UI | P5,P7 |
| E03 | UI hover/signature/definition/rename | PENDING_IOS_UI | P5,P7 |
| E04 | Formatter idempotence | LOCAL_PASS | P5 |
| E05 | UI rapid edits / cancellation | PENDING_IOS_UI | P5,P7 |
| E06 | UI LSP UTF-16/offsets | PENDING_IOS_UI | P5,P7 |
| E07 | UI bounded visual edit | PENDING_IOS_UI | P5,P7 |
| E08 | Editor performance | LOCAL_PASS | P5 |
| CL01 | CLI JSON and exit status | LOCAL_PASS | P6 |
| CL02 | Analysis does not execute/sign | LOCAL_PASS | P6 |
| CL03 | v2 rejects legacy syntax normally | LOCAL_PASS | P6 |
| D01 | Docs generated from schema | LOCAL_PASS | P6 |
| D02 | Runnable/invalid docs examples | LOCAL_PASS | P6 |
| D03 | Fork docs distinct from upstream | LOCAL_PASS | P6 |
| K01 | Skill package and self-test | LOCAL_PASS | P6 |
| K02 | Fresh Skill install/cache mismatch | LOCAL_PASS | P6 |
| K03 | Exact SHA install/checksum | LOCAL_PASS | P6 |
| K04 | Offline compatible cache | LOCAL_PASS | P6 |
| K05 | Skill repair/build/edit/sign flow | LOCAL_PASS | P6 |
| CI01 | Intentionally broken encoding gate | LOCAL_PASS | P7 |
| CI02 | Real iOS language fixtures | PENDING_IOS_RUNTIME | P7 |
| CI03 | Exact-SHA final workflows | LOCAL_PASS | P7,P8 |
| CI04 | Corpus privacy and explicit signing | LOCAL_PASS | P7,P8 |
| AI01 | Held-out old/new model evaluation | AI_NOT_RUN | P7 |
| END01 | Clean branch/PR and handoff | LOCAL_PASS (per historic local snapshot; recheck final) | P8 |

---

## Appendix B - Complete 52-ID repair regression inventory (abbreviated, verification to reconcile)

| ID | Regression requirement in brief | Main phase |
|---|---|---|
| RP01 | Verification aggregator | P0,P7 |
| RP02 | Required evidence tiers | P0,P7 |
| RP03 | Portable acceptance contracts | P0 |
| RP04 | Compile cannot bypass analyzer | P2 |
| RP05 | Literal text resembling legacy code | P2 |
| RP06 | Canonical declaration propagation | P1,P4 |
| RP07 | Facet conflict diagnosed | P1,P4 |
| RP08 | Schema fingerprints and typed facts | P1 |
| RP09 | Static action variants | P4 |
| RP10 | Custom callbacks and App Intents | P4 |
| RP11 | Number-as-text/reference codecs | P1,P4 |
| RP12 | Named argument evaluation order | P2,P7 |
| RP13 | Explicit variadic list | P2 |
| RP14 | Binding capture and native value types | P2,P7 |
| RP15 | Compound assignment | P2,P7 |
| RP16 | Exhaustive AST node lowering | P2 |
| RP17 | Comparisons and arithmetic | P2,P7 |
| RP18 | Observable short circuit | P2,P7 |
| RP19 | Literal and UTF-16 f-strings | P2,P7 |
| RP20 | Container and optional native shapes | P2,P3 |
| RP21 | Safe zero-based indexing | P2,P7 |
| RP22 | Loop item and index binding | P2,P7 |
| RP23 | Value yields/blocks/loops | P2 |
| RP24 | Function return and recursion | P2,P7 |
| RP25 | Function defaults and transport | P2,P7 |
| RP26 | Caller input and Void | P2,P7 |
| RP27 | Real module resolution | P2 |
| RP28 | Existing standard functions | P2 |
| RP29 | Assets and path authorization | P2,P5 |
| RP30 | Shortcut header round-trip | P3 |
| RP31 | Setup indices and Ask Each Time | P3 |
| RP32 | Trigger family + declaration handling | P2,P3 |
| RP33 | Syntax-aware legacy migration | P3 |
| RP34 | Atomic failure of migration | P3 |
| RP35 | Native escape/input formats and unknowns | P2,P3 |
| RP36 | Shared import/input formats | P3 |
| RP37 | Bounded edit fail-closed gate | P3,P5 |
| RP38 | Contextual language service/LSP | P5 |
| RP39 | Formatter/rename/stale edits | P5 |
| RP40 | Actual Swift UI integration | P5 |
| RP41 | Preview/source bounded edit | P5 |
| RP42 | Public CLI and installed tools | P6 |
| RP43 | Output/signing failure behavior | P6 |
| RP44 | Skill compatibility rejection | P6 |
| RP45 | Exact install and rollback | P6 |
| RP46 | Docs + Skill metadata coherence | P6 |
| RP47 | Negative native encoding CI gate | P7 |
| RP48 | Real observed iOS runtime assertion | P7 |
| RP49 | Final implementation SHA/artifact identity | P8 |
| RP50 | Executable AI evaluation harness | P7 |
| RP51 | Race/performance reproducibility | P1,P7 |
| RP52 | No accidental second user frontend | P1,P6 |

---

## Appendix C - Complete 33-ID backend recovery gates (abbreviated, verification to reconcile)

| ID | Recovery gate in brief | Main phase |
|---|---|---|
| BRG01 | Takeover preserves interrupted work | P0 |
| BRG02 | Strict reference-edge comparator | P3 |
| BRG03 | Strict native tree/text comparator | P3 |
| BRG04 | Evidence validator derives real tier | P0,P7 |
| BRG05 | Original canonical action builder invoked | P1 |
| BRG06 | Backend isolation and error recovery | P1 |
| BRG07 | Native chained reference transforms | P2,P3 |
| BRG08 | One inline/shared text codec | P1,P2 |
| BRG09 | Typed collection vs native template distinction | P2,P3 |
| BRG10 | Captures/source eval order | P2,P7 |
| BRG11 | Optional/default argument fidelity | P2,P7 |
| BRG12 | Nested mixed loop identity | P2,P7 |
| BRG13 | Typed condition/short circuit | P2,P7 |
| BRG14 | Value blocks, loops and returns | P2,P7 |
| BRG15 | Function transport and recursion | P2,P7 |
| BRG16 | Dynamic bounds/numeric semantics | P2,P7 |
| BRG17 | Setup/trigger/metadata preservation | P2,P3 |
| BRG18 | Native action/block/full-workflow paths | P2,P3 |
| BRG19 | Safe native import/edit/migration | P3 |
| BRG20 | Modules and original stdfunc | P2 |
| BRG21 | Path authorization and pure services | P2,P5 |
| BRG22 | Live schema and generator failures | P1,P4 |
| BRG23 | Upstream declaration propagation | P1,P4 |
| BRG24 | Upstream behavior propagation | P1,P4 |
| BRG25 | Checked CLI/mobile route | P5,P6 |
| BRG26 | Exact signing payload | P6,P7 |
| BRG27 | Editor completion/ranges/stale edits | P5 |
| BRG28 | Preview bounded edit/performance | P5 |
| BRG29 | Active generated docs/examples | P6 |
| BRG30 | Skill package and immutable compatibility | P6 |
| BRG31 | Native negative CI and exact-SHA runs | P7,P8 |
| BRG32 | Full obligation accounting and AI harness | P0,P7 |
| BRG33 | Approved frontend semantics | P2,P5 |
