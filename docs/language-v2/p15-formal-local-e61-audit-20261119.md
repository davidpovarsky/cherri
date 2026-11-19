# P10R15 exact local179 audit at e61e0d30

The original Linux diagnostic run [44621856878](https://github.com/davidpovarsky/cherri/actions/runs/44621856878) executed all 179 obligations against `e61e0d306bb363c39ade612ef35ccfc8051ddf3c` and exited **1**. Its actual report contains **84 PASSED, 1 FAILED (T08), 85 NOT_RUN, 8 PENDING_EXTERNAL, and 1 AI_EVAL_NOT_RUN**. `closure_complete` remains **false**.

This audit preserves the original result. It does not substitute later RP29 evidence, pending Apple execution, a passing build, or related-ID results into this frozen run.

## Verified provenance

- Original artifact: `14578356436`, ZIP SHA-256 `af9408fc41f90248ccbec2bff05ff32846f3c41f5a08ef22ea37edbaa4d81cfa`.
- Report SHA-256: `89545406ef6af4b56a00e862738aeaaf5adb0a79e253f7f6bacbc830ebfaeee6`.
- Process receipt SHA-256: `1a5de8c1bb8222db4536825051d6ea8800080e22a9f1c00efdffdfc1c48f34ad`.
- Execution audit SHA-256: `3163d7c35cdcb1030094278c0aad073a8c289f55574acfad5f0ae1de8abc64cd`.
- Docs source: `205217a256b5f220724363b44b5ae516ffe6a539`; consumer compiler pin: `b97ae752205f1e747b6c98ff7511f3484dd53bc2`.
- Compiler/docs trees were clean before execution and unchanged afterward. The process records all three immutable contract digests. All 151 local evidence records use the exact implementation SHA.
- Started `2026-11-19T21:45:38.370780+00:00`; finished `2026-11-19T21:51:30.540386+00:00`. Public fixture signing was explicitly opted in; no private corpus was requested.

The original ZIP, report, process receipt and execution audit are archived under [evidence](evidence/p15-formal-local-e61-20261119/). The derived `gap-audit.json` includes every remaining ID and hashes for every original artifact file. Raw receipt/source claims have not been rebound to the documentation commit.

## What the open counts mean

The 85 NOT_RUN rows comprise 26 original requirements, 33 repair scenarios and 26 recovery gates. Sixty explicitly lack an executed dedicated producer: all 33 repair cases, all 26 gates, and D03. The other 25 are downgraded smoke or bounded checks. These counts describe evidence coverage, not 85 proven implementation failures.

Forty-four of those 85 explicitly require Apple runtime/UI/build or native negative-CI tiers. Forty-one lack those tier names, but still include genuine unsupported semantics, repository/consumer evidence, and race obligations; they are not all inexpensive local registration changes.

Exactly V01, V03, S01, L02, L04 and F01 have accepted local tiers and await current-SHA iOS runtime proof. E07 separately needs iOS UI. RP50 has real harness-unit evidence but lacks actual equal-budget held-out evaluation; AI01 remains AI_EVAL_NOT_RUN.

N01 and FN01/FN02/FN03 have eligible narrow runtime export mappings but no accepted local unit/native-structure producers in this run. A successful native run alone cannot complete them. These four are the next assigned producer audit.

T05, L01, F04, FN05, RP20, RP23 and BRG09 include real optional/null, nested-list/cardinality or structured transport gaps. The recent fail-closed safety guards cannot satisfy requirements to support those values losslessly. Additional behavior and exact oracles are still necessary.

RP29 subsequently passed all its local tiers on separate clean source `5b9c3dd2`, with raw proof `7e9761f4`; it remains NOT_RUN in this e61 report. RP51/BRG06 state and race work is assigned separately.

## Priorities for actual production reuse

Existing substantive test families are candidates for dedicated RP06/07/08 and BRG22/23 schema producers, RP44/45 and BRG30 installed Skill/rollback producers, RP43 real signing/output failures, BRG18/19 native import/edit and migration, and BRG32 complete obligation accounting plus the executable AI harness. Each must execute its own complete scenario, preserve all required tiers and add missing causal coverage. No completed linked requirement is a substitute for that execution.

T08 is a genuine executed regression: contextual `chooseFromList` signature help reports `Unknown` instead of the literal-mode-dependent result. Its source-span repair is owned separately. The failed raw result is retained.

## Every remaining requirement

The following table transcribes original audit scope. Exact source/test paths and suggested next actions remain in the derived JSON. A missing producer is explicitly distinguished from an observed failure or accepted local tier awaiting external evidence.

| ID | Original status | Required tiers | Exact remaining scope |
|---|---|---|---|
| B01 | NOT_RUN | repository | A historical ledger substring does not verify the current base, remote HEAD, or unrelated worktrees. |
| U01 | NOT_RUN | repository | Declaration/document existence does not establish preservation against the upstream baseline. |
| V01 | PENDING_EXTERNAL | unit, native-structure, ios-runtime | Required external tier has no validated current-SHA evidence in this local run |
| V03 | PENDING_EXTERNAL | unit, native-structure, ios-runtime | Required external tier has no validated current-SHA evidence in this local run |
| T01 | NOT_RUN | unit, native-structure, ios-runtime | base64Encode lowering does not test false, zero, empty text/list/map, and omission independently. |
| T05 | NOT_RUN | unit, native-structure | Actual presence/false/zero/empty native-field controls and authored-null rejection pass locally; supported lossless native JSON null and all optional-container semantics remain unproved. |
| T08 | FAILED | unit, service | Executed contextual signature regression; original failed receipt retained. |
| S01 | PENDING_EXTERNAL | unit, native-roundtrip, ios-runtime | Required external tier has no validated current-SHA evidence in this local run |
| L01 | NOT_RUN | unit, native-structure | Nonempty lowering does not assert nested shape, cardinality, or order. |
| L02 | PENDING_EXTERNAL | unit, native-structure, ios-runtime | Required external tier has no validated current-SHA evidence in this local run |
| L04 | PENDING_EXTERNAL | native-structure, ios-runtime | Required external tier has no validated current-SHA evidence in this local run |
| N01 | NOT_RUN | unit, native-structure, ios-runtime | A modulus action and action count do not establish fractional division and numeric constraints. |
| N03 | NOT_RUN | unit, native-structure | Nonempty IR does not prove case-sensitive equality or comparison migration. |
| F01 | PENDING_EXTERNAL | unit, native-structure, ios-runtime | Required external tier has no validated current-SHA evidence in this local run |
| F02 | NOT_RUN | unit, native-structure, ios-runtime | Seven actions do not prove group identities, scopes, or zero-based indices. |
| F04 | NOT_RUN | unit, native-structure | A repeat-end UUID does not prove one semantic collected element per iteration. |
| FN01 | NOT_RUN | unit, native-structure, ios-runtime | Action count does not prove returned value and caller continuation. |
| FN02 | NOT_RUN | unit, native-structure, ios-runtime | Action count does not inspect presence tags and explicit false/zero/empty defaults. |
| FN03 | NOT_RUN | unit, native-structure, ios-runtime | Nonempty recursive lowering and any capture diagnostic do not prove recursive results and capture boundaries. |
| FN04 | NOT_RUN | native-structure, ios-runtime | Action count does not inspect dispatcher envelope validation. |
| FN05 | NOT_RUN | unit, native-structure | Text/number lowering does not exercise structured argument/result transport. |
| M02 | NOT_RUN | native-structure, ios-runtime | IR HasExplicitReturn flags do not establish emitted native output/fallthrough behavior. |
| E01 | NOT_RUN | service, ios-ui | A small completion set does not cover contextual labels, enums, compatible variables, or UI. |
| E02 | NOT_RUN | service, ios-ui | Diagnostic count does not inspect complete ranges and incomplete-code UI behavior. |
| E03 | NOT_RUN | service, ios-ui | Rename edit count does not inspect exact exclusions or hover/signature/definition behavior. |
| E05 | NOT_RUN | service, ios-ui | One out-of-order document update does not test cancellation and stale response publication. |
| E06 | NOT_RUN | protocol, ios-ui | One exact UTF-16 offset does not test protocol negotiation and the complete Cocoa span surface. |
| E07 | PENDING_EXTERNAL | integration, ios-ui | Required external tier has no validated current-SHA evidence in this local run |
| D03 | NOT_RUN | docs, integration | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| CI01 | NOT_RUN | negative-ci | Local process fixtures do not execute WorkflowKit; source-bound real macOS correct/corrupt/restored REQUIRED encoding evidence is required. |
| CI02 | NOT_RUN | ios-runtime | Nonempty IR for a runtime fixture is not an iOS runtime execution. |
| CI03 | NOT_RUN | repository, ci | A 40-character local SHA does not prove final workflow head/checkout provenance. |
| CI04 | NOT_RUN | security, repository | Absence of private_key.pem is not a private-corpus, credentials, or signing audit. |
| AI01 | AI_EVAL_NOT_RUN | external-eval | Actual held-out model evaluation has no validated current-SHA evidence in this run |
| END01 | NOT_RUN | repository | Successful git status does not establish a clean tree, pushed branches, artifacts, or delivery provenance. |
| RP06 | NOT_RUN | generation, integration | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| RP07 | NOT_RUN | generation, negative-integration | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| RP08 | NOT_RUN | generation, unit | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| RP11 | NOT_RUN | unit, native-differential | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| RP12 | NOT_RUN | unit, native-structure, ios-runtime | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| RP14 | NOT_RUN | unit, native-structure, ios-runtime | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| RP15 | NOT_RUN | unit, native-structure, ios-runtime | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| RP16 | NOT_RUN | unit, native-negative | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| RP17 | NOT_RUN | unit, native-structure, ios-runtime | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| RP18 | NOT_RUN | unit, native-structure, ios-runtime | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| RP19 | NOT_RUN | unit, native-roundtrip, ios-runtime | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| RP20 | NOT_RUN | unit, native-structure | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| RP21 | NOT_RUN | unit, native-structure, ios-runtime | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| RP22 | NOT_RUN | unit, native-structure, ios-runtime | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| RP23 | NOT_RUN | unit, native-structure | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| RP24 | NOT_RUN | unit, native-structure, ios-runtime | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| RP25 | NOT_RUN | unit, native-structure, ios-runtime | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| RP26 | NOT_RUN | native-structure, ios-runtime | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| RP29 | NOT_RUN | security, service, benchmark | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| RP33 | NOT_RUN | migration, native-differential | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| RP39 | NOT_RUN | unit, service, ios-ui | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| RP40 | NOT_RUN | app-integration, ios-ui | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| RP41 | NOT_RUN | integration, ios-ui | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| RP42 | NOT_RUN | cli, skill-package | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| RP43 | NOT_RUN | integration, security | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| RP44 | NOT_RUN | skill-package, integration | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| RP45 | NOT_RUN | skill-package, integration | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| RP46 | NOT_RUN | docs, integration | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| RP47 | NOT_RUN | negative-ci | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| RP48 | NOT_RUN | ios-runtime | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| RP49 | NOT_RUN | repository, ci, skill-package | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| RP50 | PENDING_EXTERNAL | harness-unit, external-eval | Required external tier has no validated current-SHA evidence in this local run |
| RP51 | NOT_RUN | benchmark, race | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| RP52 | NOT_RUN | integration, repository | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| BRG01 | NOT_RUN | repository | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| BRG06 | NOT_RUN | unit, integration, security | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| BRG08 | NOT_RUN | unit, native-roundtrip, ios-runtime | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| BRG09 | NOT_RUN | native-structure, native-roundtrip | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| BRG10 | NOT_RUN | unit, native-structure, ios-runtime | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| BRG11 | NOT_RUN | unit, native-structure, ios-runtime | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| BRG12 | NOT_RUN | unit, native-structure, ios-runtime | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| BRG13 | NOT_RUN | unit, native-structure, ios-runtime | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| BRG14 | NOT_RUN | unit, native-structure, ios-runtime | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| BRG15 | NOT_RUN | unit, native-structure, ios-runtime | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| BRG16 | NOT_RUN | unit, native-structure, ios-runtime | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| BRG18 | NOT_RUN | native-roundtrip, integration | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| BRG19 | NOT_RUN | integration, native-roundtrip | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| BRG21 | NOT_RUN | security, integration, service | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| BRG22 | NOT_RUN | generation, integration | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| BRG23 | NOT_RUN | repository, integration, generation | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| BRG24 | NOT_RUN | repository, integration, native-structure | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| BRG25 | NOT_RUN | integration, ios-build, ios-ui | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| BRG26 | NOT_RUN | unit, integration, ios-runtime | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| BRG27 | NOT_RUN | service, integration, ios-ui | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| BRG28 | NOT_RUN | integration, ios-ui, benchmark | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| BRG29 | NOT_RUN | generation, docs, integration | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| BRG30 | NOT_RUN | skill-package, integration | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| BRG31 | NOT_RUN | ci, ios-runtime, repository, security | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| BRG32 | NOT_RUN | unit, integration, repository | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
| BRG33 | NOT_RUN | unit, analysis, service, integration | Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation |
