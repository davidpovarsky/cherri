# Project Board

### 2026-11-23 — Pagination macro result and V2 main completion

The K10 pagination observer rejected the compiler's required terminal Nothing.
A narrow extractor now verifies that exact boundary and the original results
List separately, preserving the full graph for lossless roundtrips and leaving
Legacy behavior and the macro allowlist unchanged. One clean a8402e05 replay
passed 32 CLI cases, 32 complete roundtrips, 144 model cases, 44 invalid-source
cases and 7 causal faults; native executions remain 0. The original failed CI
and causal compile are preserved. See [the bounded proof](docs/language-v2/evidence/p15-paginate-observer-20261123/README.md).


This is a living working-memory document for this repository.
It is intentionally lightweight. Humans and coding agents should update it when useful discoveries, ideas, plans, optimizations, problems, or follow-up work arise during normal development.
Do not turn this into a duplicate issue tracker or a dump of temporary thoughts.

## Inbox

Quick captures that still need classification.

## Ideas & Opportunities

Potential improvements, features, optimizations, or architectural ideas.

## Discoveries & Tips

### 2026-11-23 — Acceptance detail fidelity and verified continuation

K476's original full179 remains 111 PASSED / 4 FAILED / 28 NOT_RUN / 36 PENDING_EXTERNAL. The report discarded detailed pending diagnostics while the audit preserved them; unchanged END01 correctly rejects those original pairs. The minimal repair preserves the authoritative detail. Genuine causal RED and focused six-parent/ten-event GREEN remain retained separately.

Root committed the five reviewed files as 08dab62c1dd838a4a45f4d6bfcea2ad5e0817402, tree 8cbfbeb56ab540b96f545ca6a6f2029b3048b797, parent K476. Its separate clean qualification passed three parents and 239 subcases (242 named events). The nested nine-parent/89-child RP52 producer is locally PASSED; editor E05 and related results still require ios-ui. Nested counts are not added to outer totals.

Earlier dirty-candidate RP52 32-event, E05 26-control and detail GREEN results retain their actual source attribution and initial missing-temp failure. The retained E05 probe was verified against identical committed blobs without rerunning it. No old report, native outcome, full179 result or actual END delivery is replaced. Publication was pending at the checkpoint cutoff. [Bounded Chapter32 evidence](docs/language-v2/evidence/p15-oracle-checkpoint32-20261123/README.md).

### V2 main output and bounded export graphs (2026-11-23)

V2 main fallthrough emits a parameter-free Nothing after the macro's named File
producer. Export validators must check that terminal boundary explicitly while
preserving every macro action and the complete native roundtrip. The boundary
does not justify allowing Nothing or other effects inside the export graph, and
a local graph model remains separate from Apple File/runtime acceptance.

### Cherri investigation notes (2026-10-06)

- **Named Magic Variable optimization:** Real Shortcut evidence shows that an action output can use `CustomOutputName`, and subsequent references can point directly to the original `ActionOutput`, allowing some usages of `Set Variable` to potentially be eliminated. Investigate compiler/decompiler support and when this transformation is semantically safe. This may affect the compiler, decompiler, corpus analyzer, structural comparator, generated action metadata/docs, Skill behavior, and iOS/app consumers if shared metadata needs extension. This is an investigation opportunity, not an implemented optimization.
- **Inline type coercion / Magic Variable coercion:** Real Shortcut evidence shows that a downstream input can reference an existing action output with `WFCoercionVariableAggrandizement`, for example `CoercionItemClass: WFContactContentItem`. This allowed a vCard/file output to feed Choose from List as contacts without a separate `is.workflow.actions.detect.contacts` action. Investigate whether this generalizes to other Shortcuts content types and whether Cherri can use it to avoid redundant conversion/detection actions. This is an investigation opportunity, not an implemented feature.


Useful technical discoveries, undocumented behavior, shortcuts, implementation tricks, platform behavior, or reusable knowledge discovered while working.

## Experiments / Investigations

Things worth testing or researching before deciding whether to implement them.

## Open Questions

Important unresolved questions or uncertainties.

## Planned / Todo

Concrete work that is worth doing but is not part of the current task.

Use Markdown checkboxes where useful:

- [ ] Example item

## Done

- [x] Repair Remote Builder staging/promotion for the current paired runtime (2026-11-22). Clean published source 607691de now stages and binds both compiler and migrator, enforces exact 23/29-file packages and 39 entrypoint obligations, and rejects changed migration bytes/sidecars. Eighteen focused controls passed; an actual clean Linux amd64 Cherri pair passed packaging, extraction, doctor and eight wrapper operations plus ten controlled install scenarios. Historical failures remain; final tuple and live ARM64 activation are still separate. [Scoped proof](docs/language-v2/p15-remote-paired-runtime-20261122.md).

- [x] Execute and independently intake OpenMinis candidate258/fd/D6 (2026-11-20). Single run44731965480 passed signing34, source/runtime packages, actual ARM64 Alpine and scoped package acceptance. All8 original ZIPs,22 members,25 immutable source blobs and four published binary identities were checked. Original486 failure remains; newM02 tuple, Apple and final179 remain open. [Exact scope](docs/language-v2/p15-openminis-source-selection-44731965480-20261120.md).


- [x] Align native nested-List diagnostics across analysis, check, service and compile (2026-11-20). Original13-case mismatch reproduced; clean3852 has151 root+109 core named passes and official47/17 parity, zero failures/skips. Dictionary inline arrays/raw native data and explicitly unverified reference rows retain scope. L01/T05/F04/FN05 native semantics remain open. [Evidence](docs/language-v2/p15-native-list-analysis-boundary-20261120.md).


- [x] Complete independent RP42/RP44/RP45 Linux component producers (2026-11-20). Run44697366234 attempt1 at clean9654056e passed210/233/235 outer and14/39/41 actual root events, zero failures/skips; all64 immutable source blobs, raw/process bytes and six distinct tier records were independently checked. BusyBox10 original-byte controls passed separately. Original failed source/run receipts remain unchanged. Published tuple, final integrated179, Apple and deployment remain open. [Exact component proof](docs/language-v2/p15-extracted-install-linux-20261120.md).

- [x] Complete the RP42 local extracted-install producer and preserve first Linux failures (2026-11-20). Clean95611881 passed210 outer and14 actual scenario events with both CLI/skill-package records,64 immutable source hashes, zero failures/skips and strict incomplete/final-rejection controls. Original Linux44694030530 remains failed; its94 successful inner observations do not replace formal success. Actual BusyBox10 byte controls passed separately. RP44/RP45 full strict replay, coherent published consumers and final179 remain pending. [Proof and original receipts](docs/language-v2/p15-extracted-install-linux-followup-20261120.md).

- [x] Repair and verify extracted-install component boundaries (2026-11-20). Actual migration flag parsing, raw capability JSON, both tool roles/source/digests, exact docs, handled update rollback, selected companion and owned lock cleanup have causal local failures and successful bounded replays. Final clean9c06 runtime/ownership10 and strict intake150 events pass; all23 original local attempts retain their original source/bytes. Complete RP42/RP44/RP45 Linux producers and the coherent published consumer tuple remain pending. [Scope and retained evidence](docs/language-v2/p15-extracted-install-local-20261120.md).

- [x] Verify BRG19 safe migration/import/edit (2026-11-20). Independent actual public CLI scenario preserves originals on unsupported conversion, emits V2, roundtrips XML/binary exactly and permits only one digest-bound scalar edit. All15 collateral and8 policy faults per format reject. Clean component53 and final strict258 events pass with68 immutable source blobs; initial source-count NOT_RUN is retained. Related rows, Apple and release closure remain separate. [Proof and original receipts](docs/language-v2/p15-safe-import-edit-brg19-20261120.md).

- [x] Verify RP33 structural migration with actual native differential evidence (2026-11-20). Lexical regions, native reference/callee identity and entry-source include resolution preserve the supported original programs; ambiguous Legacy includes inside multiline literals fail before writing. Import-question comparison follows a unique original target through normalization and still rejects wrong targets. Clean434 full47/47 programs+17/17 actions; clean d233 actual producer251 outer/59 required inner passes with64 source blobs and two RP33-only tiers. Related IDs, BRG19, Apple and release closure remain independent. [Proof and original receipts](docs/language-v2/p15-structural-migration-rp33-20261120.md).

- [x] Verify RP16 exhaustive checked AST and native rejection (2026-11-20). Reproduced three unused/metadata and eleven annotation silent-success gaps before repair. Actual clean compiler114+lower38 and strict producer/intake280 events pass; two RP16-only local records, all114 required children,45 immutable source hashes. Related requirements, Apple runtime and global release closure remain independent. [Proof and original receipts](docs/language-v2/p15-exhaustive-node-acceptance-20261120.md).

- [x] Close statically inferred List-row guard bypasses (2026-11-19). Fourteen parser-valid reference/function/Optional/Union cases now fail explicitly before an artifact is returned; scalar/Dictionary controls and native import round trips pass. Dynamic AnyContent hierarchy is still unverified, so L01/F04/FN05 remain incomplete. See [boundary and executed controls](docs/language-v2/p15-inferred-list-shape-boundary-20261119.md).

- [x] Preserve prior artifacts when P15 migration or signing fails (2026-11-19). Shared migration validation now rejects unparsable generated text before writing; public signing stages output and commits only after validation. Real failed/restored CLI and OS rename controls pass locally. Final clean-SHA acceptance and Apple gates remain separate; see [causal evidence and scope](docs/language-v2/p15-migration-signing-output-preservation-20261119.md).

Completed items that are still useful to retain because they document an important decision, discovery, or implementation.

Example:

- [x] Example improvement
  - Implemented: YYYY-MM-DD
  - Commit/PR: ...
  - Notes: ...

## Archive

Older completed/superseded items that still have historical value.


## P15 formal PoC scope repair — 2026-11-19

Implemented the separate source-bound narrow exporter and actual Python-to-Go intake regression. Raw17 diagnostic rows remain unchanged; only V01/V03/L02/L04/N01/FN01/FN02/FN03 runtime facets are eligible. Twelve Python controls and six Go test/subtest events passed; no native/runtime or179-closure claim. See docs/language-v2/p15-runtime-evidence-scope-20261119.md. Shell/publication wiring and fresh Apple execution remain separate gates.


## P15 Windows Python producer repair — 2026-11-19

Replaced discovery-only interpreter selection with an actual isolated Python3 probe and explicit-config fail-closed behavior. The six original canonical-documentation failures now pass without a transient override; focused9 top-level/81 test events passed. CLI build diagnostics now report context/elapsed time with the same90second budget. See docs/language-v2/p15-python-interpreter-portability-20261119.md. The canceled broad tools replay remains incomplete.


## P15 real module-resolution producer — 2026-11-19

Registered RP27 as an independently executed scenario with all immutable unit/service/migration tiers. The producer runs fifteen named actual multi-file, alias, cycle, library-init migration, and unresolved dependency tests; six corrupted or incomplete evidence controls reject false completion. Focused inventory and producer replay passed (eight test/subtest events, 40.72 seconds); related IDs and native-device obligations inherit no result. Contracts remain unchanged. The focused log is retained at dist/p15-acceptance-causal/rp27-selected.jsonl; final clean integration evidence remains separate.


## P15 AI harness control producer — 2026-11-19

RP50 now executes the real held-out task-runner control suite through exact discovery of fifteen Python tests. It records only harness-unit; the immutable evaluation tier (canonical external-eval) remains missing, RP50 stays PENDING_EXTERNAL, and AI01 stays AI_EVAL_NOT_RUN. Six evidence corruption controls and altered-tier/linked-ID rejection passed, with nine focused Go test/subtest events in 84.86 seconds. Fake subprocess adapters measure no model quality. The initial tier-normalization failure and repaired logs are retained separately under dist/p15-acceptance-causal/rp50-*. No external model was invoked and no whole-project closure is claimed.


## P15 exact S01/F01 runtime intake — 2026-11-19

Added only the two source-bound coverage observer records for S01 and F01, bringing formal output to six records/ten IDs while raw seventeen diagnostics stay unchanged. Exact current-SHA source/oracle/reset/context/receipt bindings are preserved; Go rejects old weaker producer IDs, changed values, missing bindings and unrelated repair membership. Full54 Python runtime-chain tests, two additional actual publication controls, and53 focused Go evidence test events passed. Initial stale S01 tier-authorization failure was retained and corrected. No local control is native evidence; fresh Apple execution remains required. See docs/language-v2/p15-runtime-evidence-scope-20261119.md.


## P15 RP29 actual authorization and benchmark producer - 2026-11-19

Independently executed all unchanged RP29 security/service/benchmark tiers at clean source 5b9c3dd2: eleven production tests, 34 named pass events, six fixtures and 400 raw monotonic samples; no skips or failures. All nine clean-source corruption controls passed. Missing or skipped symlink children and detached benchmark summaries cannot satisfy intake. Related requirement IDs and native/race/release tiers inherit no result. Exact raw public receipts and provenance are retained in docs/language-v2/p15-rp29-actual-producer-20261119.md. Final integrated179 execution remains separate.


### 2026-11-19 — P10R15 continuation R2 integration

- Frozen e61 actual Linux/179 failures retained: 84 PASS, T08 FAILED, 94 pending/not-run. Full original archive and per-ID gap audit are committed in `docs/language-v2/p15-formal-local-e61-audit-20261119.md`.
- Contextual signature callee identity repaired; clean 30fd0c9 actual service/LSP and T08 evidence passed. Initial incorrect new Text-mode expectation and both unsuccessful attempts remain explicit in `docs/language-v2/p15-signature-callee-regression-20261119.md`.
- Integrated independently verified RP29 authorization/400-sample benchmark producer, constrained continuation zero-clear oracle repair, native File/N03 diagnostic fixtures, and bounded editor foreground guards. Full iOS has an equivalent standard Intel runner option; no native success is inferred from configuration.
- Existing immutable contracts remain unchanged. New integrated CI, local179, Apple runtime/import, actual consumers/deployment, real AI endpoint and physical-device gates remain open.

## P15 immutable editor evidence source binding — 2026-11-19

Closed an actual intake gap in S03/E08/RP29: a changed source digest with a reconciled process-artifact hash previously passed. Producer and intake now hash exact immutable Git blobs, preserving clean-before/after and Windows CRLF correctness. Implementation 16255d2e passed 172 focused controls (including 67 actual-receipt source mutations); a separate clean replay passed 86 outer events plus the actual RP29 child's 34 events. Original failures and unchanged archived/current receipts are retained. No full179, native UI, race, or release credit is inferred. See [source binding proof](docs/language-v2/p15-editor-source-binding-20261119.md).

## P15 BRG32 and expanded state races — 2026-11-19

Actual BRG32 execution now covers complete179 mapping,a real fresh clone/CLI build,an installed external harness and15Python controls. Eight mutations reject invented/partial evidence. RP50 still requires external evaluation and AI01 is not run. See `docs/language-v2/p15-brg32-obligation-harness-20261119.md`.

Expanded Linux-race87ca5dfc passed6scenarios and64+32 concurrent cases after two retained failing baselines. Cold catalog,import/prepass and reverse metadata use shared native-state ownership. No blanket thread-safety or formal BRG06/RP51 closure is claimed. See `docs/language-v2/p15-state-import-race-20261119.md`.

Living plan2.6.0 retains actual e61 result84PASS/1FAIL/94pending,Apple timeout/loop-index failure,and remaining native,consumer and release work.

### 2026-11-19 — N01 actual fractional arithmetic producer

Clean implementation `db918ee8493907c90da449e67e1be6df1dc43770` passed 22 named producer/intake controls and independently executed both numeric compiler tests. Only unit/native-structure evidence is produced; the unchanged iOS tier remains pending. Original logs and exact hashes are retained in `docs/language-v2/evidence/p15-n01-producer-20261119/`; see `docs/language-v2/p15-n01-actual-producer-20261119.md`. No compiler semantics or immutable contracts changed.


### 2026-11-19 — R2 complete local179 audit

Actual Linux run44634809329 at clean4e56dbcfe895baa8f944b83715ecff0d07bfd24e produced all179:87PASS,0FAIL,82NOT_RUN,9PENDING_EXTERNAL,1AI_EVAL_NOT_RUN; closure remains false. The original ZIP/report/process/audit and all hashes are retained in `docs/language-v2/evidence/p15-formal-local-r2-20261119/`. See `docs/language-v2/p15-formal-local-r2-audit-20261119.md`; no runtime or consumer completion is inferred.


### 2026-11-19 — FN01–FN03 actual local producers

Clean3b3711f968464a3f42ee3bc8ebdfde73bc299a14 passed118 producer/intake events; six new core tests passed60 events with38 causal native-graph corruptions. FN01–FN03 now produce real unit/native-structure evidence and remain PENDING_EXTERNAL for their unchanged iOS tiers. No linked requirement inherits success. Scope and original failures/replays: `docs/language-v2/p15-function-actual-producers-20261119.md`.

## P15 R3 assembly and original R2 OpenMinis evidence — 2026-11-19

R2 remains frozen at 4e56dbcf. R3 assembles the locally verified loop-index repair, N01/FN producer proofs, the complete R2 179 audit and actual Linux race evidence. F02 implementation provenance now points to the integrated 6d9a6f3b commit; original component receipts retain b5a51fe6. R2 OpenMinis run 44634814424 succeeded with all original artifact digests verified; the pinned package compiler remains b97ae752, native macro execution and live promotion remain open. See docs/language-v2/p15-openminis-r2-20261119.md.

## P15 schema producer and full-suite session repair — 2026-11-19

Four actual schema scenarios RP06/RP07/BRG22/BRG23 passed at clean a278d9f3 with32 named events and28 rejected receipt mutations; original179 accounting and immutable contracts remain unchanged. Canonical label fixes and fail-closed generator validation are integrated with correct implementation ancestry. R2 Linux full suite exposed nested V2/Legacy test-session ownership; the unchanged regex oracle passes after explicit V2 close at9ee950b1. R2 full CI is still failed; the focused replay does not rewrite it. See docs/language-v2/p15-schema-generation-20261119.md and p15-r2-session-deadlock-20261119.md.

## 2026-11-20 — BRG24 actual canonical source propagation

[תומר | INT] [MAIN_SESSION] Clean a846e904 executed the complete actual static declaration/callback/shared codec rebuild and native-tree scenario, plus nine receipt controls. One bounded BRG24 PASS; full release remains open. Original failed test setup and clean corrected runs retained in [the report](docs/language-v2/p15-canonical-propagation-20261120.md). No Apple or linked-ID inheritance.

## 2026-11-20 — BRG33 approved frontend production execution

[תומר | INT] [MAIN_SESSION] Clean 04c5e847 actually executed all 23 selected production tests and ten receipt controls. One bounded BRG33 PASS; original minimum tiers and independent 179-row accounting remain unchanged. [Proof](docs/language-v2/p15-approved-frontend-20261120.md). No Apple, linked-ID or release inheritance.

- 2026-11-20: Actual unknown-expression injection exposed a silent nil-success fallback in checked lowering (five contexts). The narrow fallback now rejects explicitly; do not equate an unresolved import test with exhaustive AST coverage. RP16 was subsequently completed locally with an independently checked 42-kind inventory and 23 canonical unknown-node contexts; see docs/language-v2/p15-exhaustive-node-acceptance-20261120.md for exact source-bound scope. See docs/language-v2/p15-exhaustive-node-fail-closed-20261120.md.

## P10R15 R3 publication and evidence review — 2026-11-20

The real full179 result at frozen K 8e66254d is 95 PASS / 2 FAIL / 82 pending or unexecuted; no final closure. Both failures select a stale synthetic policy fixture, which a separate test-only component repairs without weakening validation. Docs main D5 was published through the existing branch-restricted Pages environment after successful CI; the feature branch's blocked deployment remains retained. Actual installed/public D03 docs intake passed on one unchanged retry, but app integration is still missing. OpenMinis package source and embedded compiler pins were independently checked. Original artifacts, failures and scope boundaries are preserved in [the frozen R3 audit](docs/language-v2/p15-r3-ci-consumers-audit-20261120.md). Current docs examples total 39; older 37-count receipts are historical.

## P10R15 RP11 local parameter-wire acceptance — 2026-11-20

RP11 now independently executes eleven captured-Apple native parameter comparisons, semantic Number checks and58 causal wire/reference/range controls. Clean8c1305b3 produced248 outer and83 inner named passes with45 source hashes and two RP11-only tier records. Original failed test expectations remain retained. No linked T03/T09/R03 or Apple/runtime/final release credit is inherited. See docs/language-v2/p15-rp11-parameter-wire-20261120.md.

RP11 follow-up: independent review found unbound mutable initializer/store inputs in the first consumer-only oracle. Clean8cb6ae7b exposed12 causal failures. Strengthened8042bdf8 independently binds each source producer/value to its store and consumer;260 outer and95 inner named cases now pass, including70 causal native controls. The earlier receipts remain unchanged and are qualified by their narrower historical scope.

## 2026-11-20 — U01 repository declaration boundary component

Actual clean-source production passed 135 outer / 14 child named tests on 6bdddd6d5ea3012b806ae82239cd7a7fe49e2138. All 26 declaration inputs were reconciled with four immutable baselines: 24 unchanged since the continuation and two reviewed one-token label corrections. Fifty producer source blobs and the three original child artifacts were independently verified. U01 receives one local repository record only; final 179-row closure remains open. See [the original evidence audit](docs/language-v2/p15-u01-upstream-boundary-20261120.md).

## 2026-11-20 U01 original-object hardening

Clean 06c5c2fa completed 140 outer and 19 actual root events after a real four-operation isolated replacement-ref counterexample. Fifty source blobs and three original artifact references independently audited. Repository U01 component only; all final/native/other-ID gates remain unchanged. See docs/language-v2/p15-u01-original-object-hardening-20261120.md.

## 2026-11-20 R4 intermediate preflight audit

Three original failed combined invocations retained without recategorization. At0dc, five completed RP11/RP16/U01 records independently verified against92 distinct source blobs. F03 and schema focused clean replays pass; RP03 child completion and full final suite remain open. See docs/language-v2/p15-r4-intermediate-preflights-20261120.md.

## 2026-11-20 S01 scoped compiler repair and native replay

Actual WorkflowKit observation explained literal U+FFFC loss. Clean da0ba0e5 passed194 root and88 package events after original six-route failure and a separately reproduced six-route implicit-input regression. Original source/313-byte semantic oracle unchanged; full native replay44671266567 is separate. Further container/enclosing-input boundaries remain open. See docs/language-v2/p15-s01-literal-object-carrier-20261120.md.


## RP43 output/signing repair checkpoint — 2026-11-20

- Production source b351ba06c0728197376955629dfd4701c7a6a153: real output/sibling preservation and encode-before-write repairs; Legacy signing defaults unchanged.
- Clean actual 127 focused/new+compatibility passes and official 47/17 actions pass. Original eight output, twelve signer-preservation, and actual encoder-truncation failures retained; two fixture expectation errors stay distinct.
- Strict RP43 producer source 468392aae763fe22ef7f6dcde4dcdc6e7f489e91: complete 78 required Linux events and 47 immutable source blobs; clean 97 intake/Windows-host controls pass with zero accepted RP43 records.
- RP43 remains NOT_RUN until actual Linux permissions plus complete strict source-bound process execute. No Apple signature/runtime or global 179 closure.
- Report: docs/language-v2/p15-output-signing-repair-20261120.md; original byte archive and manifests in the adjacent evidence directory.


## RP43 actual Linux component — 2026-11-20

- Frozen source 53daeae947f729e434168a8a09661e735386c661 completed one manual Linux run 44705394599: 253 outer and all 78 required inner named passes; no failure, skip or timeout.
- Both RP43-only integration/security records passed strict local intake. The independent audit verified all 47 immutable source blobs, exact commands, process intervals and original raw/artifact hashes. Real Linux permissions and owned loopback signer faults were executed.
- Original causal failures, corrected fixture expectations and Windows NOT_RUN evidence remain unchanged. The registration-only push is not counted as execution.
- Final integrated179, required CI, Apple/native signing and coherent consumer publication remain open. No related-ID inheritance or manifest/pin change.
- Proof: docs/language-v2/p15-output-signing-linux-20261120.md, with the original Linux archive and source/member manifests alongside it.


## P15 bounded post-test acquisition (2026-11-20)

The QA collector now anchors its three post-test02 metadata queries to the original 900-second phase, retains separate original stdout/stderr with strict hashes, and preserves private build evidence required by existing validators. Source/run/fixture checkpoints and incomplete terminal receipts cannot create callback/trace or import/runtime success; completed summaries win exclusive atomic writes. Publication keeps the existing explicit text allowlist and quarantines all raw originals. Actual Windows replay: 89 discovered controls, 85 passes, four POSIX-only skips, zero failures; source/checkpoint, deadline, corruption, cancellation and late-finalizer controls are retained. Native process-group and Simulator replay remain pending. The original Optional c0 and File e4 job-limit cancellations remain unchanged, and no formal requirement is closed by this local repair.

## 2026-11-20 — Apple QA terminal evidence identity follow-up

- Preserved the peer-review RED: four actual controls exposed 11 stale assertion/summary and publisher-continuation failures at 3c433277442cc3407e189b35d8b9b8e462ec1ac5.
- Bind existing assertion source/run/fixture/profile/unsigned bytes and invocation nonce, including exclusive-write races and summary early returns. Persist the actual emitter nonce once so the existing storage reader transition remains explicit. Stop publication if finalization rejects identity.
- Actual combined Windows replay: 98 discovered, 94 PASS, 4 explicitly POSIX-only skips, no failures (113.359 seconds). Original 900-second phase, 45-minute job, callback/trace/value oracles, and private raw originals are preserved. Peer review found no remaining concrete regression in the bounded delta. Clean-source replay and actual macOS execution are separate gates; no runtime or 179 completion granted.

## P10R15 native nested-List observation preparation (2026-11-20)

- Added separate public control and experimental native List-row diagnostics with explicit main returns; no production encoding or formal requirement promotion.
- Local full graphs cover56/78actions;28/33 actual graph corruptions and15 bounded observation controls. Clean-source replay and native selector/callback/publication integration remain separate gates.
- Preserve actual contradictions and type-name bytes. A shape match or Dictionary carrier does not establish native nested-List type. See docs/apple-runtime/p10r15/nested-list-native-observation.md.

- Clean nested-List preparation proof sealed: source15c5d1af,4 Go named passes,15 actual observer controls,56/78 action graphs,61 actual graph corruptions,2 altered-output artifacts rejected; actual fresh CLI unsigned gates passed. Native execution and type evidence remain open. Proof: docs/language-v2/evidence/p15-nested-list-native-probe-20261120.

## Documentation and macro verification checkpoints

- [x] 2026-11-21 [תומר | INT]: Verified immutable documentation tuple S=c46902bb/D8=56811d6b/K=c61de842. Independent RP46 and BRG29 production/intake passed (all88 examples and20 unpacked wrapper calls each; BRG29 canonical generation/520 signatures/505 snippet parity);30 corrupted original evidence records rejected. D8 CI44937336996 passed39 executable examples,12 cache controls and6 actual built-site controls. Evidence: docs/language-v2/p15-r6-documentation-recovery-20261121.md. Full179, Apple runtime and publication remain independently gated.

- [x] 2026-11-21: Corrected documentation recovery intake to bind/revalidate nested originals, exact S/D/K, source-derived inventories, generation JSON events and the actual outer execution interval. Clean a7d4f8eb replay accepted local RP46/BRG29 and rejected all 48 corruptions; original c61/2af formal intake claim is superseded, originals preserved. See docs/language-v2/p15-r6-documentation-nested-intake-20261121.md. Final179/Apple/deployment remain separate gates.

- [x] 2026-11-21: Corrected the macro ledger's final-consumer observer for the exact V2 implicit-void Nothing terminator; clean0be0a15d passed19 compile cases,14 causal graph controls and19 ledger verifier checks. Original OpenMinis d90 failure and skipped steps remain qualified. D8 documentation published at exact56811d6b with successful main CI/Pages and7 live byte comparisons; installed consumers wait for compiler-S decision. See docs/language-v2/p15-r6-macro-oracle-publication-20261121.md.

## 21 November 2026 — imported-variable presence repair

Root integrated clean source 61614095633e50cbc5377d36ca01061a4a36a100 after causal RED and independent static review. Actual local replay: 6 synthetic, 20 demand and 1,686 broader Optional events PASS, 0 FAIL / 0 SKIP. See docs/language-v2/p15-imported-presence-demand-20261121.md. Private 38-input corpus replays remain in progress; full 179 / Apple / Pure closure is not claimed. The previous compiler pin remains until the next reviewed tuple is frozen.


## 2026-11-22 — reviewed AI transport component

Root integrated the source-bound e2027ee2 provider transport and its preserved offline controls after independently reconciling all 57 unittest terminals, 10 original archive members, and 4 Git source bindings. Fixed HTTPS endpoints, zero retry/redirect, original usage/source retention and 15/90/120-second limits were reviewed. This proves only the mocked transport component. Real generation, AI01, Apple and final 179 acceptance remain open. Compiler S3 remains 9c8; no consumer pin or release promotion was changed. The separate full179 negative-test candidate 36d78698 remains untested and unmerged.


## 2026-11-22 verified local continuation: Legacy operands and full179 negative control

- S3 private proof a892b0e6: all five exact-source9c8 gates PASS; public metadata only. No later-source inheritance.
- Legacy b791ce06: same26-event baseline RED/fixed PASS, independently reconciled; ordinary-clone compiler/migrator carry exactb791 VCS stamps. Private/consumer/finalCI gates at this source remain pending.
- full179 negative8930e497: all5 validator-only events PASS; all179 row and exact-cause assertions preserved. Original36d empty-file failure retained and its valid ingestion rejection explicitly asserted. Aggregate acceptance is still the original K1 report, not this meta-test.
- Evidence: docs/ai-team/evidence/20261121-legacy-math-and-full179-controls. No formal requirement, Apple or release marked complete by this stage.


## 2026-11-22 — R6 verified short-circuit and Apple intake (root)
- Integrated local short-circuit source173d4ced/proof2ef45b2e after independent26/96/139 event and105source-witness verification; RP18/BRG13 still pendingApple.
- Integrated Apple proofc34057f4 for sourcec543:129members verified and six original narrow runtime records canonically recomputed on exactclean source. Startup andFileUTF8 failures remain; no test02/native inheritance.
- Root review0c04c0efae8365a014d08529227e734b625f18b58252d17e02636872c11902d7. No task or full179 closure. HistoricalK1 aggregate unchanged until a new combined run.


## 2026-11-22 — R6 verified joined Unicode integration (root)
- Source 82560cfb / proof 450dd404: root independently verified all 23 original members, 117 Git source witnesses, 26 outer events and 46 RP19 / 61 BRG08 named passes.
- Reassembled fragmented Go output and decoded all 18 retained XML specimens: two actual compiled originals plus 16 actual import/recompile results. Per-requirement bytes and typed trees are exact; original binary inputs are not claimed retained.
- Four local unit/native-roundtrip records; RP19 and BRG08 remain pending Apple runtime. Original 24cada failure remains unchanged. No full requirement or CI inheritance.
- Integration preserves documentation recovery, short-circuit and Unicode mappings and both independent intake guards. Root review SHA256: 9395ccf788c7819145c9abd5d5c49cf2b11583111c376b917800412f8a8dbe0e.

## 2026-11-22 — bounded function-usage scan component

- Clean source `0f50e542` replaces the original ASCII usage regex with equivalent parenthesis/prefix discovery. Baseline and fixed sources each passed72 named events across three repetitions, including20,120 differential byte strings per repetition. Existing Legacy CLI/native/state controls passed10 named events from an ordinary clone; the initial linked-worktree VCS build failure remains retained.
- Public64KiB–4MiB payload and256KiB call-dense body measurements show lower scan cost and zero scanner allocations. These measurements do not establish private S4 causality or completion. Original S4 source/file/timeout metadata is unchanged; no private corpus replay, deadline change, formal record or release promotion occurred.
- Source-bound originals and limits: [function-usage scan proof](docs/language-v2/p15-function-usage-scan-20261122.md). Final integrated and private gates remain separate.
