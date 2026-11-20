# RP52 — explicit public frontend and trusted reference boundary

This component completes the immutable RP52 **local integration and repository scenario** at source `08bf92990f4dee4289cb0e254173ce974e526d7e`. The dedicated runner executed all nine selected production tests and all 98 required named events; its strict intake and negative controls passed 282 named events. Exactly two records belong to RP52. The local report has one passed case and `closure_complete: false`; the final global CI gate remains enforced.

## Reproduced defect and bounded repair

Clean tests-only source `29ad738d81497a6ba657db84eec42a242ee19fa8` exposed six real failing leaves: omitted, empty and whitespace mobile modes selected Legacy for anchored file-marker or preprocessor input. Unexpected successes first satisfied the actual Text-to-Show native payload/reference oracle. The run recorded 18 named passes and seven failures including the parent, with zero skips. Original raw SHA256: `8a6413539f0888e5c5ba80a393134c344ea81796c6cc8a93d68590009aab0e48`.

Production repair `8665ef0381d6735a04d9f4dce07e560943199243` makes omitted/empty mobile modes and Swift `CherriCompiler.compile` default to V2. Existing explicit `legacy` and explicitly selected `auto` compatibility behavior remains supported. C default exports already selected V2. Five existing Swift historical-reference calls now explicitly select Legacy, and the native specimen helper explicitly selects its former Auto reference behavior; original source specimens and native comparison assertions are preserved. The first clean affected replay passed 58 named events in 40.313 seconds.

The repository audit binds all six C compile export signatures and exact source/mode/profile forwarding, the Swift public default method, and its complete private bridge body. A peer-reviewed counterexample was reproduced at `e7f7dabccaabcc363df2b57e85f44bb9f5804854`: an inactive canonical enum could conceal a live auto-default struct. That clean run passed 29 controls and failed the exact leaf plus its parent. Raw SHA256: `ed88e2c02db3a475ba6f31eb170a74c94d7121c45a4558be4c197c7aa2cd3bc6`. The repaired audit requires the reviewed unconditional top-level declaration, rejects alternate/nested/conditional ownership, and keeps comment, interpolated-string and raw-string opacity controls.

The audit's initial `4ba4a55c` attempt failed at Go compilation because `go/token` collided with the package's existing `token` type. It executed **zero tests**. The alias-only correction is a separate commit; original build output remains in the proof. No semantic verdict is inferred from that attempt.

## Actual complete scenario

| Actual selection | Obligation exercised |
| --- | --- |
| `TestP15RP52DefaultMobileFrontend` | Omitted, empty, whitespace and explicit V2 modes; valid source, three old-source forms, zero-byte rejection and healthy recovery |
| `TestP15RP52ExplicitLegacyReferenceModes` | Explicit Legacy/Auto compatibility, invalid mode and recovery, actual payload/reference oracle |
| `TestP15RP52EntrypointSourceBoundaries` | Six C signatures/forwarders, Swift default/private bridge and causal mode, pointer, signature, shadow, alternate-declaration faults |
| `TestP15U04PublicFrontendRouteMatrix` | Actual current CLI file/compile routes, approved defaults, explicit modes, invalid/conflicting modes and no artifact on rejection |
| `TestP15CLIDefaultRejectsLegacyWithGuidance` | Actual file/compile/check rejection and migration guidance for old user-source forms |
| `TestP15U01ActualDeclarationBoundary` | Actual immutable upstream declaration bytes, complete inventory and only reviewed corrections |
| `TestP15U01DeclarationBoundaryControls` | All twelve declaration/history/hash/correction corruption controls |
| `TestP15U01GitReplacementIsolation` | Active disposable Git replacement fault cannot alter immutable show, batch, tree or diff observations |
| `TestBackendRecovery19StructuralMigrationIntegration` | Real isolated Legacy metadata/reference/include source, migration, canonical V2 native equivalence, original-file preservation, rejected conversion and recovery |

These reused tests execute freshly for RP52. Their names do not grant U01, U04, CL03, RP33, BRG19 or other requirement credit. The dedicated full179 audit explicitly rejects such inheritance.

Direct complete component replay at `b4d770488c246a4b340d87c2cb0c090134ebc0e1`: 98 named PASS, zero failures/skips, 25.156 seconds, 145312-byte raw SHA256 `ca5c4c4414d23e27648621f45db903e208dc13ead27483eb4b4e493bcd945046`.

## Source-bound intake replay

Clean implementation source: `08bf92990f4dee4289cb0e254173ce974e526d7e`.

Outer command:

```text
go test -count=1 -json -timeout=7m -run ^TestPublicFrontend ./tools/language-acceptance
```

Outer actual result: **282 named PASS, zero failures/skips**, 25.282 seconds. Original 345478-byte JSONL SHA256 `55fd6635b4682e07894b92af7045946e761eb378cd5a9420a47bb061ad536106`.

The runner launched the actual production command:

```text
go test -count=1 -json -timeout 3m0s . -run ^(TestP15RP52DefaultMobileFrontend|TestP15RP52ExplicitLegacyReferenceModes|TestP15RP52EntrypointSourceBoundaries|TestP15U04PublicFrontendRouteMatrix|TestP15CLIDefaultRejectsLegacyWithGuidance|TestP15U01ActualDeclarationBoundary|TestP15U01DeclarationBoundaryControls|TestP15U01GitReplacementIsolation|TestBackendRecovery19StructuralMigrationIntegration)$
```

Inner actual result: **98 named PASS, zero failures/skips**, 11.8864693 seconds. Original 145338-byte JSONL SHA256 `a504ecb61555029bcabf60441f6c15209592f1a5d6c3e17b8be973b6313ffaf0`. Source was clean before and after both runs. Every one of the 64 recorded source digests was independently compared with its exact immutable Git blob before this seal.

Both exact tiers require the complete scenario, child ordering, raw/stderr hashes, original bytes, command, finite process interval, source cleanliness and immutable source manifest. Controls reject every missing required event, skipped/failed/unknown/reordered output, malformed JSON/UTF-8, rewritten artifact/source hashes, detached commands/times, missing tiers, fake iOS records and related-ID rebinding. Synthetic streams exercise rejection only. No external evidence import route was added.

## Scope and retained originals

Swift frontend parsing of the real wrapper and new iOS regression source passed and is retained. This establishes parser observations only: the new iOS test has not been linked or executed here. The controlled `#if false` parser probe emitted only an empty source-file tree; it is retained as an observation and supplies no active-branch or runtime proof. The source audit deliberately rejects unreviewed conditional ownership instead of guessing build flags.

The immutable179 contracts, upstream boundary manifest, codec descriptor and all consumer pins are unchanged by this component. No Apple runtime or final179 completion is claimed. Fresh integration and final CI remain root responsibilities.

The companion evidence directory contains 32 byte-identical original files, the complete component patch, the production receipt extracted from concatenated original Go Output events, and `provenance.json`. Original artifact references are unchanged. The local `.gitattributes` preserves captured original bytes without newline conversion. Earlier semantic failures, the build-only failure, and all successful observations remain separately identifiable.

Implementation ancestry after BRG21 proof `d5da07d5e56b95ffa13f676df103e9f433787ada`:

```text
29ad738d tests-only default fallback baseline
8665ef03 bounded default-V2 production repair
4ba4a55c initial repository source audit (build-only failure retained)
a847328c import alias correction
d8e235ee complete private bridge and C signature controls
e7f7dabc tests-only inactive-declaration audit failure
b4d77048 conditional/nested declaration boundary repair
08bf9299 dedicated strict RP52 producer and intake
```
