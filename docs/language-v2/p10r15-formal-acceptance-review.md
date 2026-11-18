# P10R15 formal acceptance review

Date: 2026-11-18  
Latest completed formal snapshot source: `8c42876a49081c961d6dce8344cc9879ff8f4a0f`
Verified starting point: P10R14 `ca8a62327e429d5574c8f2a5d440de2c5e553ab2`  
Formal Asana acceptance task: `1220432417414104`  
Status: **IMPLEMENTED_PARTIAL_EVIDENCE — project closure remains open**

## Current result

The fresh local runner at `8c42876a49081c961d6dce8344cc9879ff8f4a0f` executed all registered local producers and reports **179 obligations: 20 passed, 0 failed, 159 pending or unexecuted**. The independent audit contains **20 PASS, 156 UNVERIFIED and 3 BLOCKED_EXTERNAL**, with all 179 distinct IDs bound to that exact source SHA. The runner exited 0; `closure_complete` remains false. Generated at `2026-11-18T16:44:43Z`, job `job_3a30fa58f468`.

This is the latest completed formal snapshot, not a final-candidate claim. The full Go suite at the same source failed three separate families (statistic semantic-import UI naming, supplied-registry enum propagation and immutable-vs-prospective Pack drift). Their actual failure log hash and counts are retained in `../apple-runtime/p10r15/evidence/integrated-8c42876/full-go-receipt.json`. Subsequent repairs do not rewrite that failure.

C02, C04 and K04 add their actual production evidence to the prior 17 verified local IDs. K04 now executes a real compiler with canonical docs, healthy offline cache, isolated docs/schema/commit/cache faults, missing docs and observed network denial. It does not close fresh install, package, checksum/update, signing or installed-consumer requirements. K03 separately reproduced a real late-docs partial overwrite; its repair is still under verification.

The 156 unverified items remain engineering or evidence obligations, not automatically hardware blockers. The external-tier records are F01, E07 and AI01. Actual failed signed runtime cases are retained and under active diagnosis; missing current-SHA runtime evidence does not mean the simulator is unavailable. All 52 repair scenarios and 33 backend recovery gates still require their own independent scenario producers.

Only outputs freshly generated into the source-bound directory are authoritative: [capture hashes](evidence/p15-formal-8c42876/capture-summary.json). Earlier mixed copied captures and the nonstandard implementation-phase diagnostic remain invalid historical attempts. The current declaration map in `tests/` must not be confused with proof; the exact runner-generated map is archived beside this snapshot.

Later source changes, signed runtime, iOS UI, installed consumers, private-corpus boundaries and release CI require their own receipts.

## Reviewable artifacts

- [Formal aggregate](acceptance-results.json): all 179 IDs, statuses and required/executed levels.
- [Execution audit](acceptance-execution-audit.json): per-ID production path, actual test names where present, positive/negative scope, source SHA, evidence and blocker.
- [Local proof records](acceptance-local-evidence.json): only actually executed formal producer records; these are not simulator records.
- [Historical smoke observations](acceptance-local-observations.json): measured outcome and the specific coverage gap for each of the 86 demoted probes.
- `tests/language-v2/requirements-evidence-map.json` remains a declaration/traceability map. A mapped identifier is not proof that its scenario ran.

## Producers accepted at their stated local tier

| ID | Requirement | Executed tier | Actual producer |
|---|---|---|---|
| U02 | Regenerate assembled schema from original declarations and narrow facets | unit, generation | `TestP15U02GeneratedSchemaProduction` |
| U03 | Simulate an upstream parameter change used by a facet | integration | `TestUpstreamPropagationRehearsal` |
| P01 | Plain, f, raw and multiline text parse without treating ordinary braces as variables | unit | `TestP15P01LiteralAndInterpolationValues` |
| P02 | Unterminated string/comment and partial named call recover | unit, service | `TestP15P02BoundedRecoveryAndCompletion` |
| P03 | Pratt precedence, parentheses and short-circuit tree are correct | unit | `TestP15P03PrecedenceAndComparisonChains` |
| P04 | Semicolon/newline handling and bare return are unambiguous | unit | `TestP15P04ReturnTerminators` |
| P05 | Unicode identifiers and tokenizer/completion agree | unit, service | `TestP15P05UnicodeIdentifierTokenizerCompletion` |
| C02 | Duplicate, missing, unknown labels and extra positional arguments | unit | `TestP15C02ArgumentDiagnosticsProduction`; `TestP15C02AcceptedBindingsProduction`; `TestP15C02ArgumentServiceAndCLI` |
| C04 | Enum contextual and qualified members resolve | unit, service | `TestP15C04EnumResolutionProduction`; `TestP15C04EnumServiceAndCLI`; `TestP15C04EnumContextsAndShadowing`; `TestP15C04EnumFunctionService` |
| T02 | Runtime immutable output does not satisfy literal-required slot | unit | `TestP15T02LiteralRequiredProduction` |
| T06 | User record annotation is not runtime JSON validation | unit | `TestP15T06RecordAnnotationsDoNotValidateExternalValues`; `TestP15T06RecordProofTypeRelations`; `TestP15T06ControlResultsRequireAllBranches` |
| T08 | Variant-dependent output type inferred from literal mode | unit, service | `TestP15T08ConditionalCardinalityProduction`; `TestP15T08ConditionalCardinalityService` |
| N02 | Known zero divisor and text-plus-number are rejected | unit | `tools/language-acceptance:runNumbers` |
| M05 | Seven existing trigger families migrate without install claims | unit, native | `tools/language-acceptance:runMetadata` |
| R06 | XML/binary/unsigned Shortcut/JSON workflow intake | integration | `TestP15R06ProductionImportFormats`; `TestHYB18_SignedShortcutContainerRejected`; `TestHYB20_TrailingGarbageAfterJSONRootRejected`; `TestHYB21_MalformedActionEntriesRejectedWithExactIndex` |
| R07 | Intended single-property edit vs collateral structural change | integration | `TestSEM28_UserEditOnNaturalActionIsolated`; `TestSEM29_UserEditInsideControlRegionIsolated`; `TestSEM30_OnlyTargetActionChangesAndNoCollateralDelta`; `TestSEM34_TamperDetectionAndStrictCompareAsymmetry` |
| CL01 | JSON commands emit parseable response and reliable exit status | integration | `TestP15CLIJSONCommandsAndExitStatus` |
| CL02 | Analysis never emits/signs/executes/installs | security, integration | `TestP15CLIAnalysisHasNoExternalEffects` |
| CL03 | Normal production path rejects legacy public syntax | integration | `TestP15CLIDefaultRejectsLegacyWithGuidance` |
| K04 | Offline compatible cache and docs mismatch | integration | `TestP15K04OfflineCacheProduction` |

The `native` tier for M05 means serialized native parameter assertions. It is not Apple execution or automation installation. F01 and E07 retain local proof but their required runtime/UI acceptance is unresolved. AI01 has no configured eligible external evaluation endpoint. Named Go producers are selected with uncached execution and exact JSON discovery checks; zero exit with no selected test is rejected.

## F01 defects and fixes

Two authored-source defects were reproduced before the fix. A statement such as `if flag == true && run(...) == true` lowered the RHS Run Shortcut while constructing a native predicate table, before the logical guard. A plain Bool condition used HasAnyValue, so native false was treated as a present value rather than compared with true.

The fix preflights predicate operands before emitting any actions. Effectful clauses use the existing lazy expression lowerer. Authored Bool predicates compare their value against native true. Native predicate tables remain available for representable, effect-free clauses.

A follow-up production regression began with a genuine imported workflow and appended authored `&&` and `||` conditions. Both failed before removing the workflow-wide provenance bypass. New Bool bindings and computed logical results under an imported header now receive value semantics.

**Imported-symbol ambiguity remains explicit.** Import provenance records symbols, not per-statement authorship. An original imported bare predicate `if importedBool` can mean native presence and must retain its original wire. An edit intending Boolean truth on that same imported symbol should use `if importedBool == true` or `if isTrue(importedBool)`. The compiler does not guess that an unchanged imported predicate acquired a different meaning.

The synthetic signed-runtime source is `tests/language-v2/v2-short-circuit-runtime.cherri`. A successful execution returns `P15_F01_OK`; deliberately missing shortcut targets are guarded and must not execute. Compilation and structural verification do not close its runtime gate.

## Evidence corrections

| Earlier probe | Why it could not close the contract | Current treatment |
|---|---|---|
| P03 statement count | Never inspected precedence or the logical AST | Historical observation retained; superseded by the current production P03 proof |
| T07 runner-local version helper | Did not exercise production platform comparison | Observation retained; UNVERIFIED |
| IM04 cleaned path prefix | Did not invoke a resource loader or test a symlink escape | Observation retained; UNVERIFIED |
| AC03 nonempty enum registry | Did not inject bad codecs, duplicates or stale facets | Observation retained; UNVERIFIED |
| R03/R04/R05 hand-constructed structs | Did not serialize, import or round-trip the claimed behavior | Observation retained; UNVERIFIED |
| K01–K05 file lengths/substrings | Did not install, unpack, execute, or verify consumer behavior | Observation retained; UNVERIFIED |
| CI01 unequal literal strings | Did not execute a negative CI gate | Observation retained; UNVERIFIED |
| CI04 missing `private_key.pem` | Did not establish repository/corpus/signing hygiene | Observation retained; UNVERIFIED |
| END01 successful `git status` command | Did not establish a clean tree, pushed refs or artifacts | Observation retained; UNVERIFIED |
| RP/BRG IDs linked to passed requirements | Did not run the independent scenario stated by each ID | All 85 remain NOT_RUN |
| Failed assertion labelled PASSED | Contradictory status could previously mint passed evidence | Rejected and preserved as failure |

The full per-ID explanation is in the execution audit and smoke-observation file. Local phase keeps explicit unexecuted work pending and returns success only when there is no real assertion failure or contradiction. Final phase rejects unresolved mandatory tiers and scenarios.

## Historical component verification

- Red F01 authored comparisons and Boolean presence probes before implementation.
- Red hybrid-header RHS probes: SentinelX job `job_1e455010cd32`.
- Green F01/lowering/acceptance and exact-wire control regressions: `job_0c92220981fc`.
- Green formal-scope, full-179 rejection, import formats, edit isolation and mutation tests: `job_cc962689bffb` (acceptance package 24.852s; compiler package 0.691s).
- Initial local snapshot: `job_162570051679`, source `e424174567acbf99bea2a76aa1ec79b05a18470c`, generated `2026-11-18T02:30:06Z`.
- `git diff --check` passed before the implementation commits.

The private corpus and the full repository suite are centralized by the root agent and are not attributed to this component run.

## Current parity and next work

| Surface | Current evidence position |
|---|---|
| Compiler/shared action emission | Existing architecture retained; no replacement action catalog or emitter |
| Authored Boolean/lazy control flow | Local F01 regressions fixed; signed runtime required |
| Imported native payloads | R06/R07 synthetic production paths verified; final private-corpus exact comparison required |
| let/var, defaults, functions, recursion, collections | Existing implementation retained; historical action-count checks do not close these formal IDs |
| Typed records, custom action declarations, modules, native codecs | Specific smoke gaps documented; dedicated production scenarios required |
| Triggers/setup/questions | Seven trigger parameter mappings locally asserted; installation and the remaining metadata contracts remain open |
| Editor/LSP/visual editing | Bounded source-edit integration proven; UI and remaining service contracts remain open |
| CLI, docs, installed Skill and binary pins | Local file/protocol checks do not close installed-consumer and exact-SHA delivery gates |
| Physical iPad | No execution evidence in this component; no device success claimed |

Next, attach independent scenario producers to the 86 original scope gaps and 85 supplemental obligations; replace each scope guard only when the actual requirement is asserted. Continue release profiles, typed macros, signed simulator scenarios, installed Skill, consumers and documentation on the shared core. Final evidence must reference one integrated implementation SHA and distinguish simulator, physical-device, native-structure, unit, and repository tiers.
