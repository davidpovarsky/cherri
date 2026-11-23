# K10 direct F01: original semantic failure

Run **45101081393**, attempt **1**, completed with **FAILURE** on exact source **4feadab9db4d351628f78593304373631693869a**. Its one fresh, non-self, source-bound Shortcuts callback reports SUCCESS but returns:

```text
CHERRI_QA_F01_RESULT:P15_NEGATED_IS_TRUE
```

The original expected result is `CHERRI_QA_F01_RESULT:P15_F01_OK`. The unchanged callback oracle independently returns **FAIL_RUNTIME** for both runtime and trace gates, with the reason `callback result does not exactly match expected output`. This packet preserves the counterexample; it grants no F01 or formal acceptance.

## Original execution and acquisition

| Layer | Observed result |
| --- | --- |
| Workflow | 45101081393, dispatch, attempt 1, exact K10 head, failure |
| Native job | 137451831060, xcode-27, 05:06:43–05:32:36 UTC, 25m53s |
| XCTest acquisition harness | 4 passed of 4; 0 failed, skipped or expected failures |
| Actual target callback | One correlated non-self SUCCESS, 05:29:09.554Z, wrong semantic result |
| Strict original oracle | runGate FAIL_RUNTIME; traceGate FAIL_RUNTIME |
| Post-test inspection | COLLECTED; acquisitionIncomplete false |
| Public artifact intake | 26 members; all 25 published payload hashes verified; strict synthetic/privacy checks passed |
| Encrypted project prerequisite | Public validator passed; private original verification passed in place on Windows |
| Editor inspection | NEEDS_REVIEW; only 10 cards captured for a 156-action compiled projection |
| Physical device / formal acceptance | Not established |

The actual QA Simulator is iPad mini (A17 Pro), iOS 27.0/build 24A434. The prerequisite environment is macOS 27.0.1/26A434 and Xcode 27.1/27A9275. This is separate from the full iOS test workflow's Xcode 16.4 environment and the dedicated PoC's iOS 27.2 runtime.

The sole final observer ended at 05:32:50.808465Z with return code 1 and no timeout. Independently retained GitHub run/job metadata confirms an actual workflow failure. Earlier observer quota exits remain operational history and are not used to infer the native outcome.

## Exact source and signing boundary

The original full fixture is `tests/language-v2/v2-short-circuit-runtime.cherri`, SHA-256 `6424d99f972447749235d5be9b38bc317dd8d41e7f10e7b046a3033bd39598d0`. The existing callback adapter is `tools/shortcuts-simulator-qa/fixtures/qa-short-circuit-v2.cherri`, SHA-256 `ac71d4b09360e8793f167bb5b58b5df161ada9e5da15906c81735efff1de3147`. Mechanical comparison preserves the complete decision body, with exactly two no-op comment-label renames and the existing output/trace adapter. All four guarded missing-Shortcut names remain present.

The source-bound compiler manifest, output assertions and signing provenance agree on debug / REQUIRED_NDJSON, 156 actions, unsigned size 82,519 bytes / SHA-256 `9b3a4b0101a915f6aec540299f966a499c1a4d3411f18d6d9a13ee5eab5ea1bb`, and signed size 29,268 bytes / SHA-256 `cbdd2cd8721b39fc06de068894eda273853037de7ef215a79b27a2bbffadbbea`. The existing authorized RoutineHub HubSign route reports AEA1_HEADER_PASS. The original executable digest and complete unsigned/signed binary bytes are not retained in these public artifacts.

`source-review.json` preserves the full verdict-assignment order and source truth initialization. The final verdict identifies an actual wrong result; later assignments can overwrite earlier ones, so this packet does not claim that preceding branches passed. Source `truth = true` is a source fact, not an independently observed native input value.

The published 156-row plist comparison omits hidden condition numbers, input tokens and number values and does not identify the relevant rows in the 10-card native capture. Its selected key shapes are retained with **UNOBSERVED** condition values and source-statement mappings. No local reconstruction is represented as the original signed payload, and no compiler/platform cause is asserted here.

## Failed trace is preserved

The original callback oracle emits no accepted trace bytes on this mismatch. The original published `runtime/runtime-trace.jsonl` is a 211-byte COLLECTOR_EVENT / collector_end failure placeholder, SHA-256 `3f89acbdff96e08f5d86e42255662bc5ea50c679d80acf3093ccd0624e165c06`. The exact orchestrator writes that placeholder on omitted/failed trace acquisition.

Successful-runtime canonical trace equality is inapplicable here. Both the empty independent replay and the original placeholder are preserved. The original runtimeContractVerified, compiledShortcutRuntimeExecuted and compiledShortcutCallbackVerified flags remain false; a callback-level SUCCESS is not substituted for the semantic oracle.

An initial intake caller incorrectly applied successful-runtime trace equality unconditionally and stopped before writing its receipts. The completed intake follows the unchanged source guard, records that caller preflight error, and retains all original failed-result bytes. No producer, assertion or acceptance rule was changed.

## Private prerequisite verification and privacy

The unchanged reviewed `c74295ff…` verifier checked the encrypted prerequisite in the existing owner's Windows workspace: **23 original members, 3 source commands and 21 exact-source files**, complete capture, source/context before and after, pinned environment and original budgets. Its mandatory existing public validator ran first and verified the exact public schema/inventory/context layer.

Only fixed public hashes, counts and booleans left that workspace. The packet contains the original encrypted archive and its original public summary, not a plaintext archive, key, private locator or arbitrary private stream. The fixed receipt is **prerequisite-only** and cannot grant native or formal acceptance.

## Files and scope

`original-proof-packet.zip` contains selected original authenticated GitHub receipts, both complete original public artifact ZIPs, the original final observer receipt, exact source fixtures, and explicitly named independent intake receipts. `packet-manifest.json` records every packet-member hash and the Oracle-only failed-step log hash. `intake.json` retains the precise failure, source/signing joins, artifact inventory and limitations.

This is an artifact/source review and pure replay of the existing callback/public validators, followed by the approved private in-place check. No native workflow, Go compiler, test fixture or full CI suite was rerun. The separately successful six-facet Runtime PoC does not close this full F01 failure. Any future repair and required native verification must retain their own exact source and original evidence.
