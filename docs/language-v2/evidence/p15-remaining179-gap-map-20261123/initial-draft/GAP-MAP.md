# Frozen K11 remaining179 gap map

Source: 9b4135e705b08f765b7304e01212d3b9658806fb; tree d3688f219f7454e80778cbcb3013967b6d339aad.

The original K8 report is unchanged: 114 PASSED, 0 FAILED, 26 NOT_RUN, 38 PENDING_EXTERNAL, and one AI_EVAL_NOT_RUN. This audit maps the 64 NOT_RUN/PENDING_EXTERNAL rows; AI01 is the separate quality-evaluation blocker paired with RP50. It is not a new acceptance run.

## Main findings

- T05/F04 and related RP20 retain concrete transport omissions. L01 has genuine Dictionary shape proof but no verified standalone nested native List transport or proof of intrinsic target impossibility. N03 remains unresolved native semantics plus migration proof; the retained native diagnostic failed without result fields.
- Of 31 runtime-pending rows, ten have exact existing reviewed RuntimePoC exporters; 21 deliberately reject ios-runtime until their complete joined producer/intake exists.
- E01/E02/E03/E05/E06 have real local functionality but no complete XCTest intake; the existing generic UI collector cannot pass their local-only guard. E07 has an existing scoped UI path.
- Separate combined editor/function/signing gates lack actual joined producers. Existing component implementations should be reused, not described as absent.
- B01/BRG01 contain a retained historical payload-evidence gap. A fresh clean snapshot cannot repair history.
- D03 and CI01/RP47 have real production/evidence paths awaiting qualified final-source execution; K10 component successes remain formally ineligible where the containing run failed.

## Read-only partition

Existing formal runtime mapping: V01, V03, L02, L04, N01, FN01, FN02, FN03, S01, F01.

Missing complete Apple producer/intake: T01, FN04, M02, F02, RP12, RP14, RP15, RP17, RP18, RP19, RP21, RP22, RP24, RP25, BRG08, BRG10, BRG11, BRG12, BRG13, BRG14, BRG16.

Compiler source comparison at the scoped audited paths (internal/language plus shared adapters, signing.go and mobile_compiler.go) is byte-identical to R7 ca316a3dee252456c4bec751594cf249c709b331. This is source equivalence, not a new execution or a full-repository equivalence claim.

Native-roundtrip already satisfies native-structure under main.go:524-547, so literal set-difference fields in older intake summaries must not create a second missing local tier for L02/L04.

## All 64 requirements

### Original NOT_RUN

#### B01 — Verify isolated base/remote HEAD and keep unrelated agent work unchanged

Classification: historical_evidence_gap.

The initial preservation inventory is not recoverable from today's clean checkout. The retained scout confirms current ancestry and snapshots but explicitly lacks the original untracked payload and contemporaneous digest manifest.

Smallest next action: Reuse the existing scout; search only a newly identified original backup source. If none exists, retain this historical gap for owner disposition. A new snapshot cannot retroactively satisfy it.

References: `docs/language-v2/p15-repository-preservation-scout-20261120.md`, `docs/language-v2/backend-recovery/takeover.md`, `tools/language-acceptance/assertion_scope.go:20`.

#### T05 — Optional presence differs from optional value and JSON null

Classification: compiler_transport_limitation.

Presence-aware function/default routes exist, including false, zero and empty values; ordinary containers still reject possible absence, and authored JSON null is rejected before serialization. Literal JSON text and the bounded File-body proof do not implement general JSON-null/container semantics. The final evaluator explicitly keeps T05 NOT_RUN.

Smallest next action: Design and prove lossless JSON-value and optional-container transport while retaining raw plist-null rejection. Add a complete scenario producer and causal controls before retiring the incomplete-scope guard.

References: `internal/language/analysis/analyzer.go:1194`, `internal/language/analysis/optional_transport.go:9`, `p15_raw_null_boundary_test.go:13`, `p15_optional_transport_boundary_test.go:56`, `tools/language-acceptance/main.go:1235`, `tools/language-acceptance/function_transport_execution.go:17`.

#### L01 — Nested lists/maps preserve shape, cardinality and element order

Classification: compiler_transport_limitation_with_unproved_target_boundary.

Nested arrays/maps inside Dictionary fields have real shape/order/empty/scalar controls. Direct and inferred native List<List> rows fail closed; dynamic AnyContent hierarchy remains unverified. The contract permits an exact unsupported shape only if truly unencodable; the compiler's current rejection is not proof of that target fact.

Smallest next action: Use the retained public nested-list diagnostic to establish actual target hierarchy, then implement a verified carrier or prove a narrowly bounded genuine target limitation. Add a full L01 producer; do not promote Dictionary-only observations.

References: `internal/language/analysis/analyzer.go:1420`, `internal/language/lower/literal_list.go:43`, `tools/language-acceptance/literal_collection_execution.go:17`, `tools/language-acceptance/literal_collection_execution.go:38`, `p15_nested_list_analysis_boundary_test.go`, `tools/shortcuts-simulator-qa/nested_list_probe.py`, `docs/language-v2/p15-list-absence-oracle-20261120.md`.

#### N03 — Exact case-sensitive text equality and legacy comparison migration

Classification: semantic_behavior_and_migration_unresolved.

Literal Text equality folds exactly, while dynamic equality uses native condition codes 4/5. Migration replaces explicit 'if legacy' with ordinary 'if'. The independent diagnostic graph exists, but the retained source-bound Apple attempt ended ERROR without result fields, so neither case sensitivity nor a specific compiler defect is established; legacy equivalence is still unproved.

Smallest next action: Diagnose/replay the existing public diagnostic only after a bounded Apple handoff, preserving folded/dynamic/If/Match controls and original failed attempt. If behavior contradicts exact equality, repair the shared lowering with explicit legacy migration coverage; otherwise complete the missing migration proof and full N03 producer.

References: `internal/language/lower/lower.go:4442`, `internal/language/lower/lower.go:4456`, `internal/language/migrate/migrate.go:463`, `p15_n03_text_equality_diagnostic_test.go:37`, `docs/language-v2/p15-n03-native-diagnostic-20261119.md`, `docs/apple-runtime/p10r15/r2-n03-error-20261120-audit.json`, `tools/language-acceptance/main.go:1748`.

#### F04 — Value loop collects one semantic element per iteration

Classification: compiler_transport_limitation.

The analyzer explicitly rejects List-valued and Optional value-loop results because Repeat Results lack verified nested cardinality and a per-element presence flag. Scalar/for-item full-graph controls are retained observations, not full F04 credit.

Smallest next action: Implement a one-semantic-element-per-iteration carrier including zero iterations, nested Lists and Optional absence; retain existing scalar paths and add independent full-graph/cardinality/presence controls before a full producer.

References: `internal/language/analysis/loop_result_shape.go:8`, `tools/language-acceptance/combined_value_block_execution.go:20`, `tools/language-acceptance/combined_value_block_execution.go:152`.

#### D03 — Active fork docs separated from upstream reference

Classification: qualified_docs_and_app_evidence.

The docs/public-site/installed-Skill producer and strict consumer intake exist. K10's D03 docs component was real, but its app run failed the unchanged Done-button assertion. K11 has a scoped documentation-toolbar visibility repair awaiting actual full app verification.

Smallest next action: Use K11's existing D03 consumer workflow plus the exact same-tuple successful full app unit/negative/UI records and strict intake. Preserve K10's component success and failed overall app qualification separately.

References: `tools/language-acceptance/documentation_consumers_intake.go:18`, `tools/language-acceptance/documentation_intake.go`, `ios/scripts/documentation_evidence.py`, `ios/CherriUITests/CherriEditorUITests.swift:469`, `ios/CherriApp/Documentation/DocumentationSearchView.swift`, `docs/language-v2/evidence/p15-k10-ios-full-20261123/README.md`.

#### CI01 — Encoding EXPECT intentionally broken in test

Classification: qualified_native_encoding_evidence.

The real REQUIRED WorkflowKit positive/corrupted-EXPECT/restored producer and strict intake already exist. K10's component passed, but its failed whole Build & Test run makes those artifacts formally ineligible for final-source closure.

Smallest next action: Intake the existing final-K11 macOS encoding artifact only if the full exact-SHA workflow is eligible. Keep all original three phases, exact corruption and artifact/source identities; no new compiler feature is indicated.

References: `tools/language-acceptance/native_encoding_execution.go`, `tools/language-acceptance/native_encoding_intake.go`, `tools/language-acceptance/main.go:828`, `.github/workflows/build-test.yml`, `docs/language-v2/evidence/p15-k10-macos-terminal-20261123/README.md`.

#### CI02 — Selected real iOS language fixtures

Classification: missing_complete_runtime_producer.

The frozen runner's local fallback is nonempty-IR smoke and earns no runtime credit. The reviewed RuntimePoC formal exporter deliberately publishes only ten scoped requirement IDs, excluding this full-suite row; generic smoke or four QA fixture outcomes cannot supply the complete contract.

Smallest next action: Add a source-bound producer/intake for all required safe new-language computed assertions with reset channels and per-case identities. Reuse qualified exact native originals only for their established clauses; do not relabel generic PoC output.

References: `tools/language-acceptance/assertion_scope.go:27`, `tools/language-acceptance/evidence_map.go:80`, `tools/shortcuts-simulator-qa/runtime_poc_formal_evidence.py:43`, `tools/language-acceptance/main.go:1911`.

#### CI03 — Final required workflows use exact final SHA

Classification: final_repository_and_ci_evidence_gap.

The four-successful-workflow discovery supplies only a CI record. The old forty-character-SHA repository smoke is intentionally uncredited. Exact final HEAD, checkout, event/attempt and artifact identity must still be independently established.

Smallest next action: Combine actual final K11 required-workflow results with a concrete repository provenance producer, preserving failed/pending/stale-run rejection. Do not interpret a successful discovery or today's remote SHA as complete closure.

References: `tools/language-acceptance/main.go:1887`, `tools/language-acceptance/main.go:2040`, `tools/language-acceptance/assertion_scope.go:29`, `tools/language-acceptance/main.go:828`.

#### CI04 — No private raw corpus or unrequested signing

Classification: security_and_repository_audit_gap.

Analysis/no-signing boundaries and sanitized public/private handling exist, but RP43 and related tests explicitly do not grant CI04. Absence of one private_key.pem file does not audit raw private corpus, credentials or requested signing across delivered artifacts.

Smallest next action: Produce a bounded audit of tracked/published artifacts, private-corpus source and explicit signing authorization/provenance without exposing private payloads or credential values. Bind actual inventory and observed results to final source.

References: `tools/language-acceptance/main.go:1891`, `tools/language-acceptance/assertion_scope.go:30`, `tools/language-acceptance/output_signing_execution.go:21`, `docs/language-v2/p15-private-corpus-r7-five-parents-20261123.md`.

#### END01 — Final pushed branches/artifacts/provenance and clean worktree

Classification: final_delivery_provenance_gap.

The existing local smoke only checks that git status succeeded. It does not establish both clean repositories, remote branch identity, actual artifacts, delivered tuple or limitations ledger.

Smallest next action: At the real final handoff, retain exact code/docs refs, remote equality, clean-status receipts, artifact digests and delivery ledger in a concrete source-bound producer. Do not manufacture a historical preservation proof.

References: `tools/language-acceptance/main.go:1902`, `tools/language-acceptance/assertion_scope.go:31`.

#### RP20 — Container and optional shape

Classification: compiler_transport_limitation_and_missing_combined_producer.

Many container components are implemented: typed nested Dictionary values, canonical empty values, present optionals and strict native round trips. This joined scenario still lacks a producer and includes JSON-null/optional-container/native-nested-List boundaries that remain unsupported or unverified.

Smallest next action: Resolve T05/L01 transport boundaries, then add one actual joined RP20 scenario with explicit independent order/cardinality/presence expectations. Reuse component implementation and original controls without inheriting their acceptance IDs.

References: `tools/language-acceptance/function_transport_execution.go:17`, `tools/language-acceptance/literal_collection_execution.go:38`, `internal/language/analysis/optional_transport.go:9`, `p15_brg09_container_test.go`, `tools/language-acceptance/main.go:1911`.

#### RP26 — Caller input and Void

Classification: missing_combined_function_producer.

Caller-input preservation, complete dispatch guards and Void/main-output behavior have substantial separate tests. No actual joined producer executes the contract's input-through-chain + partial-envelope + bare-return/fallthrough scenario; the formal RuntimePoC exporter deliberately excludes it.

Smallest next action: Add a distinct combined native-graph producer from the existing real compiler paths and a matching Apple computed-result producer/intake. Keep malformed envelope and no-output cases explicit; do not inherit FN04/M02 or function-smoke credit.

References: `tools/language-acceptance/function_default_execution.go:32`, `tools/language-acceptance/main_output_execution.go:25`, `p15_function_envelope_completeness_test.go:124`, `p15_function_envelope_completeness_test.go:153`, `tools/language-acceptance/main.go:1911`, `tools/shortcuts-simulator-qa/runtime_poc_formal_evidence.py:64`.

#### RP39 — Format, rename and stale edits

Classification: missing_combined_editor_producer.

Formatting, scoped rename and stale-version rejection are implemented and independently tested, but the joined Unicode/shadowing/format-twice/out-of-order/caret scenario has no local producer. The iOS collector labels the formatter service result RP39 but exports no RP39 UI record.

Smallest next action: Add the exact joined production scenario and strict unit/service/UI intake with causal edits/range/version controls. Reuse real formatter/rename/publication paths; do not retag separate component results as the joined scenario.

References: `tools/language-acceptance/main.go:1911`, `ios/CherriTests/CherriCoreIntegrationTests.swift:340`, `ios/CherriTests/CherriCoreIntegrationTests.swift:362`, `ios/CherriTests/CherriCoreIntegrationTests.swift:382`, `ios/scripts/collect_ios_test_evidence.py:18`, `ios/scripts/collect_ios_test_evidence.py:57`.

#### RP40 — Actual Swift integration

Classification: missing_complete_app_integration_producer.

The real Swift editor/bridge, completion UI and XCTest route exist. Current RP40 service test only checks valid analyze plus nonempty completion; the collector exports service and UI, whereas the contract requires actual app-integration (including diagnostics/rename/compile). Service does not satisfy integration.

Smallest next action: Add a complete actual Swift call-path scenario and exact app-integration/UI intake, checking the required registry/protocol behavior rather than only export existence or completion cardinality.

References: `ios/CherriTests/CherriCoreIntegrationTests.swift:324`, `ios/CherriUITests/CherriEditorUITests.swift:551`, `ios/scripts/collect_ios_test_evidence.py:19`, `ios/scripts/collect_ios_test_evidence.py:65`, `tools/language-acceptance/main.go:545`, `tools/language-acceptance/main.go:1911`.

#### RP41 — Preview bounded source edit

Classification: missing_complete_preview_integration_producer.

Bounded source editing exists and the E07 XCTest covers a target edit plus an invalid index. The collector supplies RP41 UI and a service-typed result; no distinct RP41 integration producer proves mapped versus generated/unmapped behavior across the real preview path.

Smallest next action: Add the exact mapped/unmapped joined integration scenario and strict intake, retaining all unrelated source/comments and explanatory read-only behavior. Reuse the existing E07 production edit path and qualified UI observations only within their scope.

References: `ios/CherriTests/CherriCoreIntegrationTests.swift:422`, `ios/CherriUITests/CherriEditorUITests.swift:818`, `ios/scripts/collect_ios_test_evidence.py:44`, `ios/scripts/collect_ios_test_evidence.py:64`, `tools/language-acceptance/main.go:1832`, `tools/language-acceptance/main.go:1911`.

#### RP47 — Native encoding negative gate

Classification: qualified_native_encoding_evidence.

The real REQUIRED WorkflowKit positive/corrupted-EXPECT/restored producer and strict intake already exist. K10's component passed, but its failed whole Build & Test run makes those artifacts formally ineligible for final-source closure.

Smallest next action: Intake the existing final-K11 macOS encoding artifact only if the full exact-SHA workflow is eligible. Keep all original three phases, exact corruption and artifact/source identities; no new compiler feature is indicated.

References: `tools/language-acceptance/native_encoding_execution.go`, `tools/language-acceptance/native_encoding_intake.go`, `tools/language-acceptance/main.go:828`, `.github/workflows/build-test.yml`, `docs/language-v2/evidence/p15-k10-macos-terminal-20261123/README.md`.

#### RP48 — Real runtime computed assertions

Classification: missing_complete_runtime_producer.

The frozen runner's local fallback is nonempty-IR smoke and earns no runtime credit. The reviewed RuntimePoC formal exporter deliberately publishes only ten scoped requirement IDs, excluding this full-suite row; generic smoke or four QA fixture outcomes cannot supply the complete contract.

Smallest next action: Add a source-bound producer/intake for all required safe new-language computed assertions with reset channels and per-case identities. Reuse qualified exact native originals only for their established clauses; do not relabel generic PoC output.

References: `tools/language-acceptance/assertion_scope.go:27`, `tools/language-acceptance/evidence_map.go:80`, `tools/shortcuts-simulator-qa/runtime_poc_formal_evidence.py:43`, `tools/language-acceptance/main.go:1911`.

#### RP49 — Final SHA and artifact binding

Classification: final_artifact_binding_producer_gap.

Existing CI discovery can emit the CI tier only after all four exact-SHA workflows succeed. No distinct executed RP49 producer combines repository, packaging, event/attempt inventory and earlier-SHA/missing-artifact negative controls.

Smallest next action: Add a concrete joined final-provenance/packaging producer with causal stale/missing-artifact rejection; intake actual immutable final outputs without re-stamping prior versions.

References: `tools/language-acceptance/main.go:2040`, `tools/language-acceptance/evidence_map.go:631`, `tools/language-acceptance/main.go:1911`, `tools/language-acceptance/main.go:828`.

#### BRG01 — Takeover preserves interrupted work

Classification: historical_evidence_gap.

The retained takeover scout found a historical backup payload gap. Committed ancestry and present clean inventories do not establish that every unpublished byte from the original interrupted work was preserved.

Smallest next action: Use the original takeover/scout and any genuinely newly located contemporaneous archive. Otherwise retain the unresolved history; no current test or new backup can reconstruct absent original evidence.

References: `docs/language-v2/p15-repository-preservation-scout-20261120.md`, `docs/language-v2/backend-recovery/takeover.md`, `tools/language-acceptance/main.go:1911`.

#### BRG15 — Function transport and recursion

Classification: missing_combined_function_producer_with_transport_boundaries.

Functions, terminating recursion, presence defaults, structured FN05 transport, caller input and dispatch guards are implemented in separate bounded scenarios. BRG15 has no actual joined producer. The complete interaction including injected lowering failure is untested as one scenario, and nested native Lists/absence inside ordinary containers retain explicit limits.

Smallest next action: Add one exact complete BRG15 local scenario using existing canonical code, including error propagation and lookalike input; resolve any demonstrated shape defect without assuming all functions are absent. Then add matching actual Apple observations and strict intake.

References: `tools/language-acceptance/function_transport_execution.go:17`, `tools/language-acceptance/function_return_execution.go:16`, `tools/language-acceptance/function_combined_default_execution.go:16`, `tools/language-acceptance/function_default_execution.go:32`, `p15_function_envelope_completeness_test.go`, `tools/language-acceptance/main.go:1911`.

#### BRG25 — One checked public CLI/mobile route

Classification: missing_joined_mobile_entrypoint_producer.

Checked CLI/mobile compilation and shared analysis are implemented. The existing Swift BRG25 test only checks valid analysis, language version and fingerprint; it does not execute all routing/error/no-signature scenarios. Collector service tier alone cannot satisfy integration/build/UI.

Smallest next action: Add a joined actual CLI+CABI+app scenario for directive-looking literals, type/malformed-function errors, old-parser boundary and no output/signature; bind actual build/UI artifacts without borrowing unrelated IDs.

References: `ios/CherriTests/CherriCoreIntegrationTests.swift:451`, `mobile_compiler.go`, `p15_checked_compile_contract_test.go`, `tools/language-acceptance/supplemental_execution.go:155`, `ios/scripts/collect_ios_test_evidence.py:66`, `tools/language-acceptance/main.go:1911`.

#### BRG26 — Exact explicit signing payload

Classification: missing_joined_signing_and_native_producer.

Exact payload signing/error handling has real local RP43/P10 controls, but RP43 explicitly excludes BRG26. The formal RuntimePoC exporter excludes generic signing smoke. No joined A-then-B/stale-state/first-failure/real imported-artifact producer exists for this gate.

Smallest next action: Compose one actual local causal signing sequence with injected provider, then a separately authorized public synthetic real signing/import/runtime observation bound to the same intended artifact. Never treat AEA1 header classification as an Apple signature proof.

References: `tools/language-acceptance/output_signing_execution.go:21`, `p15_signing_atomic_contract_test.go`, `signing_test.go:32`, `tools/shortcuts-simulator-qa/runtime_poc_formal_evidence.py:43`, `tools/language-acceptance/main.go:1911`.

#### BRG27 — Real editor ranges, completion and stale edits

Classification: missing_joined_editor_service_integration_producer.

The actual Swift editor/LSP and BRG27 UI test exist. Existing collector supplies BRG27 UI only, while complete service+integration producers for incomplete Unicode code, ranges, contextual edits, cancellation, hover/rename/format are absent for this joined gate.

Smallest next action: Add the exact joined actual bridge/LSP scenario with independently expected source edits, ranges and response versions plus strict tier-specific intake. Reuse component implementations and retain original UI methods.

References: `ios/CherriUITests/CherriEditorUITests.swift:890`, `ios/scripts/collect_ios_test_evidence.py:49`, `tools/language-acceptance/editor_contract_inventory.json`, `tools/language-acceptance/supplemental_execution.go:139`, `tools/language-acceptance/main.go:1911`.

#### BRG28 — Preview bounded edit and measured performance

Classification: missing_joined_preview_performance_producer.

Preview editing and measured small/large/embedded-asset component benchmarks exist. No complete BRG28 joined integration/benchmark producer owns the actual edit, cancellation/cache observations and performance fixtures. Collector provides bounded E07 UI plus service only.

Smallest next action: Add a distinct measured joined scenario and integrate actual preview/UI observations with preserved input and timing inventories; do not copy E08/RP51 benchmark records under BRG28.

References: `tools/language-acceptance/editor_performance_execution.go:20`, `tools/language-acceptance/editor_benchmark_intake.go:10`, `ios/CherriTests/CherriCoreIntegrationTests.swift:422`, `ios/scripts/collect_ios_test_evidence.py:44`, `tools/language-acceptance/main.go:1911`.

#### BRG31 — Actual encoding negative gate and exact-SHA CI

Classification: joined_final_ci_runtime_security_repository_gap.

A real encoding component and four-workflow discovery exist, but they do not supply this full gate's runtime/security/repository tiers. The formal RuntimePoC exporter deliberately omits the generic smoke label; failed K10 workflow artifacts remain ineligible.

Smallest next action: Create a joined source-bound final gate from actual required EXPECT corruption/restoration, successful final workflows, full computed runtime assertions, repository and privacy/signing audit. Validate every original artifact and reject stale/failed/cancelled inputs; no inherited closure.

References: `tools/language-acceptance/native_encoding_intake.go`, `tools/language-acceptance/main.go:2040`, `tools/language-acceptance/main.go:828`, `tools/shortcuts-simulator-qa/runtime_poc_formal_evidence.py:43`, `tools/language-acceptance/main.go:1911`.

### Original PENDING_EXTERNAL

#### V01 — let captures current var value before later assignment

Classification: existing_runtime_producer_awaiting_qualified_observation.

Current-value capture before a later assignment. K8 already verified the required local tiers. The frozen formal exporter contains this exact runtime mapping; no new compiler omission is demonstrated by the open status.

Smallest next action: Use the existing exact-source RuntimePoC producer and original strict artifact/reset/result intake after actual successful execution. Require ios27-runtime-poc:variables; separate QA or generic SUCCESS cannot replace it. Re-run no local component solely for this read-only audit.

References: `tools/language-acceptance/binding_capture_execution.go:9`, `tools/shortcuts-simulator-qa/runtime_poc_formal_evidence.py:43`, `tools/language-acceptance/evidence_map.go:79`, `tools/language-acceptance/main.go:828`.

#### V03 — System clipboard read captured once

Classification: existing_runtime_producer_awaiting_qualified_observation.

Clipboard snapshot before later clipboard writes. K8 already verified the required local tiers. The frozen formal exporter contains this exact runtime mapping; no new compiler omission is demonstrated by the open status.

Smallest next action: Use the existing exact-source RuntimePoC producer and original strict artifact/reset/result intake after actual successful execution. Require ios27-runtime-poc:variables; separate QA or generic SUCCESS cannot replace it. Re-run no local component solely for this read-only audit.

References: `tools/language-acceptance/binding_capture_execution.go:17`, `tools/shortcuts-simulator-qa/runtime_poc_formal_evidence.py:43`, `tools/language-acceptance/evidence_map.go:79`, `tools/language-acceptance/main.go:828`.

#### T01 — Typed defaults preserve false/zero/empty text/list/map vs omission

Classification: missing_complete_apple_producer_and_intake.

The full local scenario for typed omitted/supplied false, zero, empty Text/List/Map and Optional defaults has genuine K8 unit/native evidence. Its dedicated current validator explicitly refuses an unimplemented ios-runtime producer; historical short traces cannot substitute. This is a missing evidence-producing/validation path, not proof that the local language feature is absent.

Smallest next action: Add a bounded actual Apple execution/observer for this exact already-defined joined source and independent expected results, with fresh identity/reset and causal forged/missing/stale-observation controls. Add only the matching strict intake branch after review; preserve original contract, local producer and historical trace. A generic mapping expansion is insufficient.

References: `tools/language-acceptance/function_default_execution.go:23`, `tools/language-acceptance/evidence_map.go:36`, `tools/shortcuts-simulator-qa/runtime_poc_formal_evidence.py:43`, `tools/language-acceptance/main.go:950`.

#### S01 — Multiple variable tokens after Hebrew/emoji/combining marks

Classification: existing_runtime_producer_awaiting_qualified_observation.

Exact computed Unicode output with independent byte/reset witnesses. K8 already verified the required local tiers. The frozen formal exporter contains this exact runtime mapping; no new compiler omission is demonstrated by the open status.

Smallest next action: Use the existing exact-source RuntimePoC producer and original strict artifact/reset/result intake after actual successful execution. Require ios27-runtime-poc:coverage-s01; separate QA or generic SUCCESS cannot replace it. Re-run no local component solely for this read-only audit.

References: `tools/language-acceptance/runtime_poc_coverage_mapping.go:8`, `tools/shortcuts-simulator-qa/runtime_poc_formal_evidence.py:43`, `tools/language-acceptance/evidence_map.go:79`, `tools/language-acceptance/main.go:828`.

#### L02 — Zero-based first/last/empty/out-of-range/negative indexing

Classification: existing_runtime_producer_awaiting_qualified_observation.

Zero-based first/last/empty/negative/out-of-range computed indexing. K8 already verified the required local tiers. The frozen formal exporter contains this exact runtime mapping; no new compiler omission is demonstrated by the open status.

Smallest next action: Use the existing exact-source RuntimePoC producer and original strict artifact/reset/result intake after actual successful execution. Require ios27-runtime-poc:indexing; separate QA or generic SUCCESS cannot replace it. Re-run no local component solely for this read-only audit.

References: `tools/language-acceptance/literal_collection_execution.go:48`, `tools/shortcuts-simulator-qa/runtime_poc_formal_evidence.py:43`, `tools/language-acceptance/evidence_map.go:79`, `tools/language-acceptance/main.go:828`.

#### L04 — Dynamic bounds checking does not execute unsafe native lookup

Classification: existing_runtime_producer_awaiting_qualified_observation.

Guarded dynamic lookup with empty results for invalid input. K8 already verified the required local tiers. The frozen formal exporter contains this exact runtime mapping; no new compiler omission is demonstrated by the open status.

Smallest next action: Use the existing exact-source RuntimePoC producer and original strict artifact/reset/result intake after actual successful execution. Require ios27-runtime-poc:indexing; separate QA or generic SUCCESS cannot replace it. Re-run no local component solely for this read-only audit.

References: `tools/language-acceptance/literal_collection_execution.go:48`, `tools/shortcuts-simulator-qa/runtime_poc_formal_evidence.py:43`, `tools/language-acceptance/evidence_map.go:79`, `tools/language-acceptance/main.go:828`.

#### N01 — Fractional arithmetic including 5/2 and constraints

Classification: existing_runtime_producer_awaiting_qualified_observation.

Computed fractional division and numerical output. K8 already verified the required local tiers. The frozen formal exporter contains this exact runtime mapping; no new compiler omission is demonstrated by the open status.

Smallest next action: Use the existing exact-source RuntimePoC producer and original strict artifact/reset/result intake after actual successful execution. Require ios27-runtime-poc:numbers; separate QA or generic SUCCESS cannot replace it. Re-run no local component solely for this read-only audit.

References: `tools/language-acceptance/number_function_execution.go`, `tools/shortcuts-simulator-qa/runtime_poc_formal_evidence.py:43`, `tools/language-acceptance/evidence_map.go:79`, `tools/language-acceptance/main.go:828`.

#### F01 — Mixed AND/OR short-circuits observable branch effects

Classification: existing_runtime_producer_awaiting_qualified_observation.

Exact short-circuit Boolean results and ordered observable effects. K8 already verified the required local tiers. The frozen formal exporter contains this exact runtime mapping; no new compiler omission is demonstrated by the open status.

Smallest next action: Use the existing exact-source RuntimePoC producer and original strict artifact/reset/result intake after actual successful execution. Require ios27-runtime-poc:coverage-f01; separate QA or generic SUCCESS cannot replace it. Re-run no local component solely for this read-only audit.

References: `tools/language-acceptance/runtime_poc_coverage_mapping.go:8`, `tools/shortcuts-simulator-qa/runtime_poc_formal_evidence.py:43`, `tools/language-acceptance/evidence_map.go:79`, `tools/language-acceptance/main.go:828`.

#### F02 — Nested for/repeat scope and zero-based indices

Classification: missing_complete_apple_producer_and_intake.

The full local scenario for complete mixed/shadow loop scope and restored zero-based indices has genuine K8 unit/native evidence. Its dedicated current validator explicitly refuses an unimplemented ios-runtime producer; historical short traces cannot substitute. This is a missing evidence-producing/validation path, not proof that the local language feature is absent.

Smallest next action: Add a bounded actual Apple execution/observer for this exact already-defined joined source and independent expected results, with fresh identity/reset and causal forged/missing/stale-observation controls. Add only the matching strict intake branch after review; preserve original contract, local producer and historical trace. A generic mapping expansion is insufficient.

References: `tools/language-acceptance/loop_execution.go:21`, `tools/language-acceptance/evidence_map.go:36`, `tools/shortcuts-simulator-qa/runtime_poc_formal_evidence.py:43`, `tools/language-acceptance/main.go:950`.

#### FN01 — AST-defined function call returns then caller continues

Classification: existing_runtime_producer_awaiting_qualified_observation.

Function result and caller continuation. K8 already verified the required local tiers. The frozen formal exporter contains this exact runtime mapping; no new compiler omission is demonstrated by the open status.

Smallest next action: Use the existing exact-source RuntimePoC producer and original strict artifact/reset/result intake after actual successful execution. Require ios27-runtime-poc:functions; separate QA or generic SUCCESS cannot replace it. Re-run no local component solely for this read-only audit.

References: `tools/language-acceptance/number_function_execution.go:28`, `tools/shortcuts-simulator-qa/runtime_poc_formal_evidence.py:43`, `tools/language-acceptance/evidence_map.go:79`, `tools/language-acceptance/main.go:828`.

#### FN02 — Presence-tagged argument defaults include false/0/empty

Classification: existing_runtime_producer_awaiting_qualified_observation.

Observed supplied/omitted false, zero and empty-Text defaults. K8 already verified the required local tiers. The frozen formal exporter contains this exact runtime mapping; no new compiler omission is demonstrated by the open status.

Smallest next action: Use the existing exact-source RuntimePoC producer and original strict artifact/reset/result intake after actual successful execution. Require ios27-runtime-poc:functions; separate QA or generic SUCCESS cannot replace it. Re-run no local component solely for this read-only audit.

References: `tools/language-acceptance/number_function_execution.go:40`, `tools/shortcuts-simulator-qa/runtime_poc_formal_evidence.py:43`, `tools/language-acceptance/evidence_map.go:79`, `tools/language-acceptance/main.go:828`.

#### FN03 — No implicit runtime capture; terminating recursive call

Classification: existing_runtime_producer_awaiting_qualified_observation.

Terminating recursion; local illegal-capture controls remain separately mandatory. K8 already verified the required local tiers. The frozen formal exporter contains this exact runtime mapping; no new compiler omission is demonstrated by the open status.

Smallest next action: Use the existing exact-source RuntimePoC producer and original strict artifact/reset/result intake after actual successful execution. Require ios27-runtime-poc:functions; separate QA or generic SUCCESS cannot replace it. Re-run no local component solely for this read-only audit.

References: `tools/language-acceptance/number_function_execution.go:54`, `tools/shortcuts-simulator-qa/runtime_poc_formal_evidence.py:43`, `tools/language-acceptance/evidence_map.go:79`, `tools/language-acceptance/main.go:828`.

#### FN04 — Caller input vs internal dispatcher envelope

Classification: missing_complete_apple_producer_and_intake.

The full local scenario for complete caller input through guarded direct/nested dispatch has genuine K8 unit/native evidence. Its dedicated current validator explicitly refuses an unimplemented ios-runtime producer; historical short traces cannot substitute. This is a missing evidence-producing/validation path, not proof that the local language feature is absent.

Smallest next action: Add a bounded actual Apple execution/observer for this exact already-defined joined source and independent expected results, with fresh identity/reset and causal forged/missing/stale-observation controls. Add only the matching strict intake branch after review; preserve original contract, local producer and historical trace. A generic mapping expansion is insufficient.

References: `tools/language-acceptance/function_default_execution.go:32`, `tools/language-acceptance/evidence_map.go:36`, `tools/shortcuts-simulator-qa/runtime_poc_formal_evidence.py:43`, `tools/language-acceptance/main.go:950`.

#### M02 — Main body explicit return and Void fallthrough

Classification: missing_complete_apple_producer_and_intake.

The full local scenario for main explicit returns, Void/bare/fallthrough and child continuation has genuine K8 unit/native evidence. Its dedicated current validator explicitly refuses an unimplemented ios-runtime producer; historical short traces cannot substitute. This is a missing evidence-producing/validation path, not proof that the local language feature is absent.

Smallest next action: Add a bounded actual Apple execution/observer for this exact already-defined joined source and independent expected results, with fresh identity/reset and causal forged/missing/stale-observation controls. Add only the matching strict intake branch after review; preserve original contract, local producer and historical trace. A generic mapping expansion is insufficient.

References: `tools/language-acceptance/main_output_execution.go:25`, `tools/language-acceptance/evidence_map.go:36`, `tools/shortcuts-simulator-qa/runtime_poc_formal_evidence.py:43`, `tools/language-acceptance/main.go:950`.

#### E01 — Contextual labels/enums/compatible variables completion

Classification: missing_complete_xctest_intake.

Real local service/protocol functionality and bounded original K8 proof exist. The frozen editor inventory specifies only the local tier; validateEditorContractRecord explicitly rejects all other tiers and states that complete XCTest intake is not implemented. The existing UI collector's generic records cannot pass that guard.

Smallest next action: Review actual qualified full-app original XCTest/checkpoint/source observations against the complete requirement, add any missing actual UI scenario additively, and implement strict exact-source XCTest intake with negative controls. Preserve the current local-only guard until complete evidence is supported; do not relabel service evidence as UI.

References: `tools/language-acceptance/editor_contract_inventory.json`, `tools/language-acceptance/editor_contract_execution.go:363`, `tools/language-acceptance/main.go:959`, `ios/scripts/collect_ios_test_evidence.py:18`, `ios/CherriUITests/CherriEditorUITests.swift`, `ios/CherriTests/CherriCoreIntegrationTests.swift`.

#### E02 — Multiple diagnostics and full ranges on incomplete code

Classification: missing_complete_xctest_intake.

Real local service/protocol functionality and bounded original K8 proof exist. The frozen editor inventory specifies only the local tier; validateEditorContractRecord explicitly rejects all other tiers and states that complete XCTest intake is not implemented. The existing UI collector's generic records cannot pass that guard.

Smallest next action: Review actual qualified full-app original XCTest/checkpoint/source observations against the complete requirement, add any missing actual UI scenario additively, and implement strict exact-source XCTest intake with negative controls. Preserve the current local-only guard until complete evidence is supported; do not relabel service evidence as UI.

References: `tools/language-acceptance/editor_contract_inventory.json`, `tools/language-acceptance/editor_contract_execution.go:363`, `tools/language-acceptance/main.go:959`, `ios/scripts/collect_ios_test_evidence.py:18`, `ios/CherriUITests/CherriEditorUITests.swift`, `ios/CherriTests/CherriCoreIntegrationTests.swift`.

#### E03 — Hover/signature/definition/semantic rename

Classification: missing_complete_xctest_intake.

Real local service/protocol functionality and bounded original K8 proof exist. The frozen editor inventory specifies only the local tier; validateEditorContractRecord explicitly rejects all other tiers and states that complete XCTest intake is not implemented. The existing UI collector's generic records cannot pass that guard.

Smallest next action: Review actual qualified full-app original XCTest/checkpoint/source observations against the complete requirement, add any missing actual UI scenario additively, and implement strict exact-source XCTest intake with negative controls. Preserve the current local-only guard until complete evidence is supported; do not relabel service evidence as UI.

References: `tools/language-acceptance/editor_contract_inventory.json`, `tools/language-acceptance/editor_contract_execution.go:363`, `tools/language-acceptance/main.go:959`, `ios/scripts/collect_ios_test_evidence.py:18`, `ios/CherriUITests/CherriEditorUITests.swift`, `ios/CherriTests/CherriCoreIntegrationTests.swift`.

#### E05 — Rapid edits, cancellation and stale responses

Classification: missing_complete_xctest_intake.

Real local service/protocol functionality and bounded original K8 proof exist. The frozen editor inventory specifies only the local tier; validateEditorContractRecord explicitly rejects all other tiers and states that complete XCTest intake is not implemented. The existing UI collector's generic records cannot pass that guard.

Smallest next action: Review actual qualified full-app original XCTest/checkpoint/source observations against the complete requirement, add any missing actual UI scenario additively, and implement strict exact-source XCTest intake with negative controls. Preserve the current local-only guard until complete evidence is supported; do not relabel service evidence as UI.

References: `tools/language-acceptance/editor_contract_inventory.json`, `tools/language-acceptance/editor_contract_execution.go:363`, `tools/language-acceptance/main.go:959`, `ios/scripts/collect_ios_test_evidence.py:18`, `ios/CherriUITests/CherriEditorUITests.swift`, `ios/CherriTests/CherriCoreIntegrationTests.swift`.

#### E06 — LSP UTF-16/default negotiation and Cocoa offsets

Classification: missing_complete_xctest_intake.

Real local service/protocol functionality and bounded original K8 proof exist. The frozen editor inventory specifies only the local tier; validateEditorContractRecord explicitly rejects all other tiers and states that complete XCTest intake is not implemented. The existing UI collector's generic records cannot pass that guard.

Smallest next action: Review actual qualified full-app original XCTest/checkpoint/source observations against the complete requirement, add any missing actual UI scenario additively, and implement strict exact-source XCTest intake with negative controls. Preserve the current local-only guard until complete evidence is supported; do not relabel service evidence as UI.

References: `tools/language-acceptance/editor_contract_inventory.json`, `tools/language-acceptance/editor_contract_execution.go:363`, `tools/language-acceptance/main.go:959`, `ios/scripts/collect_ios_test_evidence.py:18`, `ios/CherriUITests/CherriEditorUITests.swift`, `ios/CherriTests/CherriCoreIntegrationTests.swift`.

#### E07 — Visual source edit changes intended node only

Classification: existing_ui_producer_awaiting_qualified_observation.

The service's real bounded source edit/rejection already passed K8's integration tier, and the full app contains an exact preview edit UI test and collector mapping. E07 is outside the five-ID local-only editor intake inventory. No missing implementation is demonstrated by this pending row.

Smallest next action: Intake the qualified exact-K11 full-app E07 test, scoped checkpoint and original artifact hashes, then verify actual bounded edit output. Do not borrow that completion for the broader RP41/BRG28 scenarios.

References: `tools/language-acceptance/main.go:1832`, `ios/CherriUITests/CherriEditorUITests.swift:818`, `ios/scripts/collect_ios_test_evidence.py:44`, `tools/language-acceptance/evidence_map.go:138`.

#### RP12 — Named argument effect order

Classification: missing_complete_apple_producer_and_intake.

The full local scenario for written-order named argument effects, snapshots and once-only evaluation has genuine K8 unit/native evidence. Its dedicated current validator explicitly refuses an unimplemented ios-runtime producer; historical short traces cannot substitute. This is a missing evidence-producing/validation path, not proof that the local language feature is absent.

Smallest next action: Add a bounded actual Apple execution/observer for this exact already-defined joined source and independent expected results, with fresh identity/reset and causal forged/missing/stale-observation controls. Add only the matching strict intake branch after review; preserve original contract, local producer and historical trace. A generic mapping expansion is insufficient.

References: `tools/language-acceptance/capture_order_execution.go:21`, `tools/language-acceptance/evidence_map.go:36`, `tools/shortcuts-simulator-qa/runtime_poc_formal_evidence.py:43`, `tools/language-acceptance/main.go:950`.

#### RP14 — Binding capture and native types

Classification: missing_complete_apple_producer_and_intake.

The full local scenario for Number/Bool/Text/List/Map/Clipboard typed capture before writes has genuine K8 unit/native evidence. Its dedicated current validator explicitly refuses an unimplemented ios-runtime producer; historical short traces cannot substitute. This is a missing evidence-producing/validation path, not proof that the local language feature is absent.

Smallest next action: Add a bounded actual Apple execution/observer for this exact already-defined joined source and independent expected results, with fresh identity/reset and causal forged/missing/stale-observation controls. Add only the matching strict intake branch after review; preserve original contract, local producer and historical trace. A generic mapping expansion is insufficient.

References: `tools/language-acceptance/capture_order_execution.go:19`, `tools/language-acceptance/evidence_map.go:36`, `tools/shortcuts-simulator-qa/runtime_poc_formal_evidence.py:43`, `tools/language-acceptance/main.go:950`.

#### RP15 — Compound assignment

Classification: missing_complete_apple_producer_and_intake.

The full local scenario for four compound updates with observed once-only ordered RHS effects has genuine K8 unit/native evidence. Its dedicated current validator explicitly refuses an unimplemented ios-runtime producer; historical short traces cannot substitute. This is a missing evidence-producing/validation path, not proof that the local language feature is absent.

Smallest next action: Add a bounded actual Apple execution/observer for this exact already-defined joined source and independent expected results, with fresh identity/reset and causal forged/missing/stale-observation controls. Add only the matching strict intake branch after review; preserve original contract, local producer and historical trace. A generic mapping expansion is insufficient.

References: `tools/language-acceptance/combined_arithmetic_execution.go:20`, `tools/language-acceptance/evidence_map.go:36`, `tools/shortcuts-simulator-qa/runtime_poc_formal_evidence.py:43`, `tools/language-acceptance/main.go:950`.

#### RP17 — Comparisons and arithmetic

Classification: missing_complete_apple_producer_and_intake.

The full local scenario for joined typed comparisons, fractional arithmetic and scientific Modulus has genuine K8 unit/native evidence. Its dedicated current validator explicitly refuses an unimplemented ios-runtime producer; historical short traces cannot substitute. This is a missing evidence-producing/validation path, not proof that the local language feature is absent.

Smallest next action: Add a bounded actual Apple execution/observer for this exact already-defined joined source and independent expected results, with fresh identity/reset and causal forged/missing/stale-observation controls. Add only the matching strict intake branch after review; preserve original contract, local producer and historical trace. A generic mapping expansion is insufficient.

References: `tools/language-acceptance/combined_arithmetic_execution.go:22`, `tools/language-acceptance/evidence_map.go:36`, `tools/shortcuts-simulator-qa/runtime_poc_formal_evidence.py:43`, `tools/language-acceptance/main.go:950`.

#### RP18 — Observable short circuit

Classification: missing_complete_apple_producer_and_intake.

The full local scenario for complete eight AND/OR cases plus four mixed trees with counted ordered effects has genuine K8 unit/native evidence. Its dedicated current validator explicitly refuses an unimplemented ios-runtime producer; historical short traces cannot substitute. This is a missing evidence-producing/validation path, not proof that the local language feature is absent.

Smallest next action: Add a bounded actual Apple execution/observer for this exact already-defined joined source and independent expected results, with fresh identity/reset and causal forged/missing/stale-observation controls. Add only the matching strict intake branch after review; preserve original contract, local producer and historical trace. A generic mapping expansion is insufficient.

References: `tools/language-acceptance/short_circuit_execution.go:20`, `tools/language-acceptance/evidence_map.go:36`, `tools/shortcuts-simulator-qa/runtime_poc_formal_evidence.py:43`, `tools/language-acceptance/main.go:950`.

#### RP19 — Literal and Unicode f-strings

Classification: missing_complete_apple_producer_and_intake.

The full local scenario for joined Unicode action-value formatting and exact text after real native execution has genuine K8 unit/native evidence. Its dedicated current validator explicitly refuses an unimplemented ios-runtime producer; historical short traces cannot substitute. This is a missing evidence-producing/validation path, not proof that the local language feature is absent.

Smallest next action: Add a bounded actual Apple execution/observer for this exact already-defined joined source and independent expected results, with fresh identity/reset and causal forged/missing/stale-observation controls. Add only the matching strict intake branch after review; preserve original contract, local producer and historical trace. A generic mapping expansion is insufficient.

References: `tools/language-acceptance/combined_unicode_execution.go:20`, `tools/language-acceptance/evidence_map.go:36`, `tools/shortcuts-simulator-qa/runtime_poc_formal_evidence.py:43`, `tools/language-acceptance/main.go:950`.

#### RP21 — Safe zero-based indexing

Classification: missing_complete_apple_producer_and_intake.

The full local scenario for complete guarded safe-index matrix including dynamic/nonintegral/large invalid values has genuine K8 unit/native evidence. Its dedicated current validator explicitly refuses an unimplemented ios-runtime producer; historical short traces cannot substitute. This is a missing evidence-producing/validation path, not proof that the local language feature is absent.

Smallest next action: Add a bounded actual Apple execution/observer for this exact already-defined joined source and independent expected results, with fresh identity/reset and causal forged/missing/stale-observation controls. Add only the matching strict intake branch after review; preserve original contract, local producer and historical trace. A generic mapping expansion is insufficient.

References: `tools/language-acceptance/combined_index_arithmetic_execution.go:20`, `tools/language-acceptance/evidence_map.go:36`, `tools/shortcuts-simulator-qa/runtime_poc_formal_evidence.py:43`, `tools/language-acceptance/main.go:950`.

#### RP22 — Loop item and index bindings

Classification: missing_complete_apple_producer_and_intake.

The full local scenario for named repeat and tuple-for shadowed item/index identities has genuine K8 unit/native evidence. Its dedicated current validator explicitly refuses an unimplemented ios-runtime producer; historical short traces cannot substitute. This is a missing evidence-producing/validation path, not proof that the local language feature is absent.

Smallest next action: Add a bounded actual Apple execution/observer for this exact already-defined joined source and independent expected results, with fresh identity/reset and causal forged/missing/stale-observation controls. Add only the matching strict intake branch after review; preserve original contract, local producer and historical trace. A generic mapping expansion is insufficient.

References: `tools/language-acceptance/loop_execution.go:27`, `tools/language-acceptance/evidence_map.go:36`, `tools/shortcuts-simulator-qa/runtime_poc_formal_evidence.py:43`, `tools/language-acceptance/main.go:950`.

#### RP24 — Function return and recursion

Classification: missing_complete_apple_producer_and_intake.

The full local scenario for add/continuation plus terminating recursion and forbidden captures has genuine K8 unit/native evidence. Its dedicated current validator explicitly refuses an unimplemented ios-runtime producer; historical short traces cannot substitute. This is a missing evidence-producing/validation path, not proof that the local language feature is absent.

Smallest next action: Add a bounded actual Apple execution/observer for this exact already-defined joined source and independent expected results, with fresh identity/reset and causal forged/missing/stale-observation controls. Add only the matching strict intake branch after review; preserve original contract, local producer and historical trace. A generic mapping expansion is insufficient.

References: `tools/language-acceptance/function_return_execution.go:13`, `tools/language-acceptance/evidence_map.go:36`, `tools/shortcuts-simulator-qa/runtime_poc_formal_evidence.py:43`, `tools/language-acceptance/main.go:950`.

#### RP25 — Function default and transport

Classification: missing_complete_apple_producer_and_intake.

The full local scenario for thirteen complete structured/presence-default transport scenarios has genuine K8 unit/native evidence. Its dedicated current validator explicitly refuses an unimplemented ios-runtime producer; historical short traces cannot substitute. This is a missing evidence-producing/validation path, not proof that the local language feature is absent.

Smallest next action: Add a bounded actual Apple execution/observer for this exact already-defined joined source and independent expected results, with fresh identity/reset and causal forged/missing/stale-observation controls. Add only the matching strict intake branch after review; preserve original contract, local producer and historical trace. A generic mapping expansion is insufficient.

References: `tools/language-acceptance/function_combined_default_execution.go:16`, `tools/language-acceptance/evidence_map.go:36`, `tools/shortcuts-simulator-qa/runtime_poc_formal_evidence.py:43`, `tools/language-acceptance/main.go:950`.

#### RP50 — Executable AI evaluation harness

Classification: actual_live_quality_measurement_pending.

Harness controls, strict v2 Sonnet profile, source/seed/anchor safeguards and canonical Go intake exist. They are not model quality results. K10's one sealed cohort was retired with zero generations; no current quality result is granted.

Smallest next action: After the final candidate and planned source additions are settled, perform the reviewed unchanged twelve public calibrations, create one fresh independently anchored cohort, then execute the authorized bounded twelve-outcome/no-retry evaluation and the two canonical Go intakes. Root owns the actual provider/anchor handoff; preserve all failed or zero-generation originals.

References: `tools/language-ai-eval/fresh-instance-protocol-v2.md`, `tools/language-acceptance/ai_evaluation_execution.go`, `tools/language-acceptance/ai_measurement_intake.go`.

#### BRG08 — Inline text references share canonical codec

Classification: missing_complete_apple_producer_and_intake.

The full local scenario for joined canonical codec/transformation and repeated Unicode references has genuine K8 unit/native evidence. Its dedicated current validator explicitly refuses an unimplemented ios-runtime producer; historical short traces cannot substitute. This is a missing evidence-producing/validation path, not proof that the local language feature is absent.

Smallest next action: Add a bounded actual Apple execution/observer for this exact already-defined joined source and independent expected results, with fresh identity/reset and causal forged/missing/stale-observation controls. Add only the matching strict intake branch after review; preserve original contract, local producer and historical trace. A generic mapping expansion is insufficient.

References: `tools/language-acceptance/combined_unicode_execution.go:22`, `tools/language-acceptance/evidence_map.go:36`, `tools/shortcuts-simulator-qa/runtime_poc_formal_evidence.py:43`, `tools/language-acceptance/main.go:950`.

#### BRG10 — Capture and written-order evaluation

Classification: missing_complete_apple_producer_and_intake.

The full local scenario for joined capture, Clipboard and written-order argument effects has genuine K8 unit/native evidence. Its dedicated current validator explicitly refuses an unimplemented ios-runtime producer; historical short traces cannot substitute. This is a missing evidence-producing/validation path, not proof that the local language feature is absent.

Smallest next action: Add a bounded actual Apple execution/observer for this exact already-defined joined source and independent expected results, with fresh identity/reset and causal forged/missing/stale-observation controls. Add only the matching strict intake branch after review; preserve original contract, local producer and historical trace. A generic mapping expansion is insufficient.

References: `tools/language-acceptance/capture_order_execution.go:24`, `tools/language-acceptance/evidence_map.go:36`, `tools/shortcuts-simulator-qa/runtime_poc_formal_evidence.py:43`, `tools/language-acceptance/main.go:950`.

#### BRG11 — Parameter forms and typed defaults

Classification: missing_complete_apple_producer_and_intake.

The full local scenario for joined canonical parameter forms and typed defaults has genuine K8 unit/native evidence. Its dedicated current validator explicitly refuses an unimplemented ios-runtime producer; historical short traces cannot substitute. This is a missing evidence-producing/validation path, not proof that the local language feature is absent.

Smallest next action: Add a bounded actual Apple execution/observer for this exact already-defined joined source and independent expected results, with fresh identity/reset and causal forged/missing/stale-observation controls. Add only the matching strict intake branch after review; preserve original contract, local producer and historical trace. A generic mapping expansion is insufficient.

References: `tools/language-acceptance/parameter_forms_execution.go:13`, `tools/language-acceptance/evidence_map.go:36`, `tools/shortcuts-simulator-qa/runtime_poc_formal_evidence.py:43`, `tools/language-acceptance/main.go:950`.

#### BRG12 — Nested and mixed loop identity

Classification: missing_complete_apple_producer_and_intake.

The full local scenario for mixed nested loops, siblings, shadowing and post-inner outer references has genuine K8 unit/native evidence. Its dedicated current validator explicitly refuses an unimplemented ios-runtime producer; historical short traces cannot substitute. This is a missing evidence-producing/validation path, not proof that the local language feature is absent.

Smallest next action: Add a bounded actual Apple execution/observer for this exact already-defined joined source and independent expected results, with fresh identity/reset and causal forged/missing/stale-observation controls. Add only the matching strict intake branch after review; preserve original contract, local producer and historical trace. A generic mapping expansion is insufficient.

References: `tools/language-acceptance/loop_execution.go:32`, `tools/language-acceptance/evidence_map.go:36`, `tools/shortcuts-simulator-qa/runtime_poc_formal_evidence.py:43`, `tools/language-acceptance/main.go:950`.

#### BRG13 — Typed conditions and dynamic short circuit

Classification: missing_complete_apple_producer_and_intake.

The full local scenario for typed conditions, full short-circuit effects and dynamic same/different-case Text comparisons has genuine K8 unit/native evidence. Its dedicated current validator explicitly refuses an unimplemented ios-runtime producer; historical short traces cannot substitute. This is a missing evidence-producing/validation path, not proof that the local language feature is absent. Dynamic Text case sensitivity remains a substantive unresolved native fact, as N03 documents.

Smallest next action: Add a bounded actual Apple execution/observer for this exact already-defined joined source and independent expected results, with fresh identity/reset and causal forged/missing/stale-observation controls. Add only the matching strict intake branch after review; preserve original contract, local producer and historical trace. A generic mapping expansion is insufficient.

References: `tools/language-acceptance/short_circuit_execution.go:22`, `tools/language-acceptance/evidence_map.go:36`, `tools/shortcuts-simulator-qa/runtime_poc_formal_evidence.py:43`, `tools/language-acceptance/main.go:950`.

#### BRG14 — Value blocks, loop results and return

Classification: missing_complete_apple_producer_and_intake.

The full local scenario for joined supported value blocks, scalar loop results, child/main return and continuation has genuine K8 unit/native evidence. Its dedicated current validator explicitly refuses an unimplemented ios-runtime producer; historical short traces cannot substitute. This is a missing evidence-producing/validation path, not proof that the local language feature is absent. Its supported local scope does not claim general nested-List/Optional F04 fulfillment.

Smallest next action: Add a bounded actual Apple execution/observer for this exact already-defined joined source and independent expected results, with fresh identity/reset and causal forged/missing/stale-observation controls. Add only the matching strict intake branch after review; preserve original contract, local producer and historical trace. A generic mapping expansion is insufficient.

References: `tools/language-acceptance/combined_value_block_execution.go:27`, `tools/language-acceptance/evidence_map.go:36`, `tools/shortcuts-simulator-qa/runtime_poc_formal_evidence.py:43`, `tools/language-acceptance/main.go:950`.

#### BRG16 — Bounds and numeric semantics

Classification: missing_complete_apple_producer_and_intake.

The full local scenario for joined numeric bounds, arithmetic and guarded dynamic lookup has genuine K8 unit/native evidence. Its dedicated current validator explicitly refuses an unimplemented ios-runtime producer; historical short traces cannot substitute. This is a missing evidence-producing/validation path, not proof that the local language feature is absent.

Smallest next action: Add a bounded actual Apple execution/observer for this exact already-defined joined source and independent expected results, with fresh identity/reset and causal forged/missing/stale-observation controls. Add only the matching strict intake branch after review; preserve original contract, local producer and historical trace. A generic mapping expansion is insufficient.

References: `tools/language-acceptance/combined_index_arithmetic_execution.go:22`, `tools/language-acceptance/evidence_map.go:36`, `tools/shortcuts-simulator-qa/runtime_poc_formal_evidence.py:43`, `tools/language-acceptance/main.go:950`.

## Limits and handoff

All next actions are proposals for root’s one-owner queue. Frozen K11 was not edited. No mapping expansion, assertion/deadline reduction, source restamping, new seed or provider call occurred. Future code/intake additions would form a new candidate and must precede that candidate’s fresh AI source binding. The final179 run can truthfully report remaining gaps; a green current workflow alone cannot create absent producers or historical evidence.

The exact original contract rows, original K8 result rows and source hashes are in gap-map.json and source-bindings.json. Source-reference existence warnings, if any, are explicitly retained in the JSON and need bounded follow-up before those references are used.
