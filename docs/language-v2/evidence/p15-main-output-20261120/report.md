# M02 explicit main result: implementation and structural evidence

Implementation: 510ecfdd9187bb0b4488d479a2292b15c193f52c, parent c8b8babf5803cd2cd43f0d37b3ed9212a56ddaad. The original integration branch is unchanged. Implementation is published at audit/cherri-p10r15-m02-source-20261120; this separate evidence commit follows it.

## Result and remaining scope

The clean implementation fixes incidental main-output leakage while preserving authored return values, explicit Legacy output behavior and exact native.workflow imports. This is component structural evidence. Apple execution, M02's iOS tier, BRG14, full Go/CI acceptance and final 179-row acceptance remain open.

| Actual clean-source run | Named PASS | Named FAIL | SKIP | Exit | Seconds |
| --- | ---: | ---: | ---: | ---: | ---: |
| M02 / RP16 / F03 focus | 337 | 0 | 0 | 0 | 16.609 |
| All internal/language packages and Legacy parity tool tests | 322 | 0 | 0 | 0 | 14.75 |
| Expanded affected set | 2328 | 2 | 0 | 1 | 165.672 |

The sets overlap and must not be summed. The expanded affected process really failed: its two events are the existing JSON optional-absence test and its parent. The untouched source includes an optional-absence value in a Dictionary, which the current target guard rejects. This report does not relabel that failure as a pass.

Strict Legacy replay passed all 47 fixtures (9 verified, 38 proven equivalent) and all 17 generated action probes, with zero baseline, migration, compilation or graph differences. Original corpus and comparator remain unchanged.

## Causal history

Tests-only baseline bc6c178f732eeed556b9479c4d9e6c547885407e produced 7 PASS and 7 FAIL events: six distinct fallthrough failures and their parent; six explicit-return controls passed. Raw SHA256: 7cc58b3ab90909285fb0604dd558b934ea7c6561d3b6deb446f28cef45576b6f.

All retained intermediate failures are included: first integration compilation/parity failures, initial whole-graph assumptions, the repeated-migration duplicate-mode defect, stale executable selection, old count/index assumptions, corruption-helper panics and the bounded affected-run timeout. These are historical draft results, not clean-source successes. Shared emission tracking and AST migration idempotence were repaired before the final replay.

## Implementation and oracle integrity

The shared literal outputMode validator runs before analysis/lowering emission. Natural V2 main fallthrough emits parameter-free native Nothing; raw and canonical resolved action identities prevent stale IR state from suppressing it. Structured guaranteed returns and actual native terminal actions retain their behavior. Exact native.workflow imports retain every original action and field.

Entry Legacy migration adds compatibility metadata once through the parsed AST. Repeated conversion preserves existing valid intent. Module conversion adds no synthetic entry declaration. Compiler-only outputMode does not become an Apple plist field.

The 22 complete main native graphs cover unused actions, empty programs, partial and complete returns, zero-iteration paths, child/caller ownership, raw/resolved action transitions, explicit Text/empty Text/false/zero/bare/None returns, metadata validation and exact native preservation. Corruption controls alter actual graphs and causal UUID edges.

Affected function, Optional, value-flow, macro and frontend oracles include the real main boundary; actual output is not stripped to hide it. Legacy differential tests retain their independently selected Legacy mode. No immutable acceptance row, original native specimen, comparator or Legacy corpus was weakened.

## Provenance

The clean source was checked before and after every command. Fresh compiler SHA256: 158b6fd0a3c1ed93e73f8579ba44c0797823f3db5748a7c34888538c3bc44bb5. Its observed Go metadata has no embedded VCS revision in this linked-worktree build. That absence is retained, not presented as an installed-tool identity proof. The immutable source commit, source-file witnesses, actual command/raw receipts and fresh build bind this component run.

New codec fingerprint: eb550a69bd2e0413369d6b0c0863448c755724b4103c5e4a3617125a72ee3122. Schema: dc6548076b8320b15e062439f0d8e7b032e1fd62d032b19b06ab6c79a54890a5. Macro: acb4dd5035ee20786c68a01d323f565e880d2a0b8eedb151145181a4960d455c. The old consumer S/D/K tuple does not establish compatibility for this source.

The archive contains 144 byte-verified originals, 12600416 uncompressed bytes. Every member has an exact original path, size and SHA256. Executables remain in the owned evidence directory with original hashes. Source/publication receipts are included. No runtime verdict is synthesized.

Fresh native acceptance collection and an independently observed Apple driver scenario are subsequent stages.
