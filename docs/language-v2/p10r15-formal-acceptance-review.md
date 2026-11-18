# P10R15 formal acceptance review

Date: 2026-11-18  
Component source: `e424174567acbf99bea2a76aa1ec79b05a18470c`  
Verified starting point: P10R14 `ca8a62327e429d5574c8f2a5d440de2c5e553ab2`  
Formal Asana acceptance task: `1220432417414104`  
Status: **IMPLEMENTED_PARTIAL_EVIDENCE — project closure remains open**

## Current result

The current local runner reports **179 obligations: 5 passed, 0 failed assertions, 174 pending or unexecuted**. The machine-readable audit distinguishes **5 PASS, 171 UNVERIFIED, and 3 BLOCKED_EXTERNAL**. The 94 original requirements account for 5 passes, 86 unverified obligations, two external runtime/UI obligations, and AI01. The 52 repair scenarios and 33 backend recovery gates have no dedicated scenario producer in this local run.

This replaces the historical 71-pass headline as a formal acceptance claim. The change does not mean that 66 product behaviors regressed. Review found that many earlier checks observed a smaller smoke condition than their requirement described. All 86 such observations remain available separately; they no longer create formal evidence tiers.

This is a component snapshot. Root integration, private-corpus verification, signed simulator results, installed consumer checks, and release CI must be tied to the final integrated SHA before they can extend this evidence.

## Reviewable artifacts

- [Formal aggregate](acceptance-results.json): all 179 IDs, statuses and required/executed levels.
- [Execution audit](acceptance-execution-audit.json): per-ID production path, actual test names where present, positive/negative scope, source SHA, evidence and blocker.
- [Local proof records](acceptance-local-evidence.json): only actually executed formal producer records; these are not simulator records.
- [Historical smoke observations](acceptance-local-observations.json): measured outcome and the specific coverage gap for each of the 86 demoted probes.
- `tests/language-v2/requirements-evidence-map.json` remains a declaration/traceability map. A mapped identifier is not proof that its scenario ran.

## Producers accepted at their stated local tier

| ID | Executed producer | Proven scope | Remaining limitation |
|---|---|---|---|
| U03 | `TestUpstreamPropagationRehearsal` through discovered, uncached Go execution | Real source edit, regeneration, propagation and cleanup rehearsal | Does not close unrelated catalog or release requirements |
| N02 | `Runner.runNumbers` using production analyzer | Literal division/modulus by zero and Text-plus-Number are rejected | Unit negative cases only |
| M05 | `Runner.runMetadata` using migrator and lowerer | Exact parameter assertions for seven migrated trigger families | No automation installation or device trigger execution claim |
| R06 | `TestP15R06ProductionImportFormats`, `TestHYB18`, `TestHYB20`, `TestHYB21` | XML plist, binary plist, unsigned .shortcut and JSON input; both frontends; recursive typed output equality; malformed/signed container rejection | Synthetic fixtures, not the private 38-shortcut corpus |
| R07 | `TestSEM28`, `TestSEM29`, `TestSEM30`, `TestSEM34` | Edited natural action/control regions, bounded changes, collateral-delta and tamper rejection | Does not establish every possible editing workflow |
| F01 | Six `TestP15F01*` producers | Serialized native guard placement, once-only RHS, static skipping, Boolean equality, nested logic, imported-header edits and negative guard mutations | **PENDING_EXTERNAL:** exact-candidate signed Apple runtime |
| E07 | `Runner.runEditor` using `Service.ApplyVisualSourceEdit` | Exact bounded source change and invalid action-index rejection | **PENDING_EXTERNAL:** iOS UI |
| AI01 | No evaluation producer executed | Endpoint blocker recorded with zero executed evaluation tiers | **BLOCKED_EXTERNAL:** held-out equal-budget model evaluation |

Each named Go producer is selected by its exact test name. The runner checks JSON discovery events; an exit code of zero with no selected tests fails verification. Production subprocesses have timeouts and use `-count=1`.

## F01 defects and fixes

Two authored-source defects were reproduced before the fix. A statement such as `if flag == true && run(...) == true` lowered the RHS Run Shortcut while constructing a native predicate table, before the logical guard. A plain Bool condition used HasAnyValue, so native false was treated as a present value rather than compared with true.

The fix preflights predicate operands before emitting any actions. Effectful clauses use the existing lazy expression lowerer. Authored Bool predicates compare their value against native true. Native predicate tables remain available for representable, effect-free clauses.

A follow-up production regression began with a genuine imported workflow and appended authored `&&` and `||` conditions. Both failed before removing the workflow-wide provenance bypass. New Bool bindings and computed logical results under an imported header now receive value semantics.

**Imported-symbol ambiguity remains explicit.** Import provenance records symbols, not per-statement authorship. An original imported bare predicate `if importedBool` can mean native presence and must retain its original wire. An edit intending Boolean truth on that same imported symbol should use `if importedBool == true` or `if isTrue(importedBool)`. The compiler does not guess that an unchanged imported predicate acquired a different meaning.

The synthetic signed-runtime source is `tests/language-v2/v2-short-circuit-runtime.cherri`. A successful execution returns `P15_F01_OK`; deliberately missing shortcut targets are guarded and must not execute. Compilation and structural verification do not close its runtime gate.

## Evidence corrections

| Earlier probe | Why it could not close the contract | Current treatment |
|---|---|---|
| P03 statement count | Never inspected precedence or the logical AST | Observation retained; UNVERIFIED |
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

## Focused verification performed

- Red F01 authored comparisons and Boolean presence probes before implementation.
- Red hybrid-header RHS probes: SentinelX job `job_1e455010cd32`.
- Green F01/lowering/acceptance and exact-wire control regressions: `job_0c92220981fc`.
- Green formal-scope, full-179 rejection, import formats, edit isolation and mutation tests: `job_cc962689bffb` (acceptance package 24.852s; compiler package 0.691s).
- Current local snapshot: `job_162570051679`, source `e424174567acbf99bea2a76aa1ec79b05a18470c`, generated `2026-11-18T02:30:06Z`.
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
