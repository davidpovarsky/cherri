# S01 literal object character compiler repair — 20 November 2026

## Verified component result

Clean source da0ba0e5e49a1be56f97b725f33ff56e3a2c91af passed194 focused root events and88 lower/analysis/backend events, with zero failures or skips. Root coverage includes exact Unicode bytes and UTF-16 references, all six binding/container routes, independent causal corruptions, XML/binary native round trips, existing F01 short-circuit and full S01/F01 fixture graph controls, and source-order checks for two interpolated calls with implicit native input.

This is a compiler component result. The separate full Apple runtime replay is44671266567, pinned to the same source. Its runtime verdict is not included or inferred here. Ordinary non-FString literals in typed containers and implicit input on an enclosing native action are distinct follow-up review boundaries; this report does not grant broad S01 or final179 closure.

## Observed original failure and native cause

The frozen K runtime run44653155762 produced302 bytes where the unchanged public coverage oracle requires313. Its first difference is byte92: the authored literal U+FFFC was missing and subsequent variable tokens were corrupted. The source fixture and expected-result JSON were not changed.

Actual WorkflowKit observation run44666226513 at source1c531a1f30d071a219a80044bfc2347f3b2e4fa8 exercised16 parameter forms through real osascript on macOS26.7.2. Plain native strings retain literal U+FFFC. WFVariableStringParameterState drops an unmapped U+FFFC inside a token template and can disrupt later ranges; complete native placeholder coverage round-trips. This observation has mock=false and ShortcutRuntime=false. It is native serialization evidence, not a Shortcut execution.

The original observation artifact14599361797 has ZIP SHA256952b2a92d6bec6f940a508dead9b6298d924ce78ee7569fad1a17440083c327a. Its actual output SHA256 is7cc36e821d5928ae93611f283ce3f868c84c9191cff9551d9fe729f00b5d265b. The independent audit verifies the pinned shortcutkit prelude and exact combined JXA source against those originals.

## Implementation and causal sequence

The existing lowerer materializeToAttachment path now emits an ordinary Text producer for affected FString literal chunks. The token template refers to that producer's exact UUID. All token positions are real mapped placeholders; the pure shared attachment codec, native parameter vocabulary, original fixture and313-byte expected value remain unchanged.

Tests-only source da3d437d reproduced the original unbound-placeholder defect in all six routes, while six positive/control events passed. Raw SHA256:6f435eef4da461550a3663766afdc5d7b619d2cd74a93cea9a85fac4e64ee5b5.

The first repair placed literal helpers before authored interpolated calls. Peer review identified the implicit-input risk. The first attempted probe at4c10e9a4 had six syntax errors and is retained with no causal credit. Corrected tests-only source2b878da9 then actually reproduced six predecessor changes: seed0 led to real native calls at2/4 instead of1/2 (the assignment route similarly shifted2 to4/6). Raw SHA256:90674b29a6575751e63f9be6a378382437beca0e4e0a7f9fce27f754dafa2ff0.

The final repair first lowers authored interpolation expressions in their original order, then emits the pure literal carriers. It reuses the same existing materializer and canonical reference paths. The clean194-event root replay includes all six corrected implicit-input cases and strict literal-carrier byte/UUID/order controls.

## Exact evidence and compatibility

| Original evidence | Named passes | SHA256 |
| --- | ---: | --- |
| Root S01/F01/UTF-16/shared selection |194 |bd638627113ad318a87a5cbbe0af009661b35596b504162300869f85f574eacb |
| Lower/analysis/backend packages |88 |45ec258d34920be4860775ded297f473c9c11e7568d43243b8639875a83aee2e |

The public source fixture remains4,075 bytes, SHA2564a13f90af2bdc326e24f9dbf785f76a1ae41043e176b5a3188ab794694a19aa3. Its expected-result JSON remains1,503 bytes, SHA2568baa3279a770ca20e3daa6036f416360a308d6f40eb1dd0874a38afeb7d0b10c. Both are byte-identical to K.

An additive codec contract line records the carrier policy. Integrated R4 preserves the independent tagged Optional v2.1 contract as well, producing a new computed fingerprint rather than reusing either component hash. New compiler/docs compatibility publication is still required before final promotion.

[Original archive](evidence/p15-s01-literal-object-carrier-20261120/original-evidence.zip) and [index/audit](evidence/p15-s01-literal-object-carrier-20261120/index.json) retain original observation artifacts, failed and successful test streams, exact tested source blobs, unchanged fixtures and integration maps. No original receipt or semantic expected value was rewritten.
