# P10R15 R6 — distinct RP19 / BRG08 Unicode local proof

## Verified result

The clean source `82560cfb93bbf1716f55b723d9bb78140691c5cd` completed the actual combined local replay on 21 November 2026, 23:24:02.771313–23:27:45.700657 UTC. The outer gate passed all 26 fixed tests with no failure, skip or timeout, within the original 9-minute Go / 570-second driver budgets. Elapsed time was 222.937 seconds for the driver and 157.086 seconds for the Go package.

| Distinct requirement | Fixed actual nested tests | Process seconds | Local tiers verified | Remaining tier |
| --- | ---: | ---: | --- | --- |
| RP19 — Literal and Unicode f-strings | 46 PASS, 0 FAIL, 0 SKIP | 106.0835252 | unit; native-roundtrip | ios-runtime |
| BRG08 — Inline text references share canonical codec | 61 PASS, 0 FAIL, 0 SKIP | 26.5336657 | unit; native-roundtrip | ios-runtime |

The Runner emits two **PENDING_EXTERNAL** results and four separate source-bound local evidence records. The generic local manifest report preserves its own representation: 0 passed, 0 failed, 2 skipped / NOT_RUN with “Pending external / CI execution tier.” Neither row receives full acceptance. No S01, S02 or other linked requirement inherits these results.

## What actually executed

RP19 independently checks Number 42 and Bool false semantic values and the exact `42false` formatting, real Text action outputs, adjacent and repeated references, Hebrew marks, a non-BMP ZWJ emoji, a combining accent and newline. Its fixed UTF-16 attachment offsets are 22, 23, 26 and 30. The complete five-action native graph binds every producer, typed parameter, output reference and final returned value.

BRG08 uses its own fixture and independent expected graph: a Dictionary → key(payload) → Text transform is retained on the direct reference and both inline references, alongside ordinary literal braces and raw backslashes. Its fixed UTF-16 offsets are 25, 26, 29, 33 and 34. The complete six-action graph binds all actions, transform order, actual input ownership and final output.

Both scenarios execute causal semantic and native corruptions, including offsets, missing/extra attachments or actions, exact text bytes, missing/late/duplicate/malformed/all-zero/case-colliding UUIDs, returned observations and requirement-specific value or transform mutations. The copy helper retains the original native scalar kinds. Registrar controls reject source, event, package, tier, requirement and artifact substitutions; the original 128-file source cap is checked before launch.

Each scenario actually writes, imports and recompiles all eight combinations of XML/binary input, hybrid/lossless import and Legacy/V2 output. The fixed passing child events and source-bound helper establish those executions. The raw logs retain one compiled native XML specimen and eight recompiled native XML specimens per scenario. Independent decoding verified each specimen’s byte count and SHA-256, and every resulting full plist tree and XML byte sequence equals that scenario’s compiled original. Temporary binary input files are not claimed as retained originals.

The RP19 compiled specimen is 4,520 bytes, SHA-256 `fe3a1b11e7a129845bfcb26ee556504308a749402a3627ad0c7f16a8bce5db0c`. The BRG08 compiled specimen is 7,220 bytes, SHA-256 `ed4e29cf16bf9a7fa09ab36849f0307e3e29ba7287760c6516453a70a83d074f`.

## Original failure and narrow correction

The original clean `24cada33084844eaffa27c63c13a694260ef60ec` replay failed after 190.281 seconds: 3 outer tests passed and 1 failed, without timeout. RP19’s ten unit events passed, then its native oracle rejected the actual inline Text action’s `CustomOutputName:"result"` because the oracle expected `"Text"`. BRG08 independently completed all 61 local events, but the outer failure prevented a complete formal intake packet. That attempt grants no new formal credit.

Inspection of the frozen compiler source established the cause: `lowerBinding` holds `currentBindingName` through the runtime literal; the resolved call, attachment token and backend action all intentionally carry the enclosing `let result` name. Commit `82560cfb` changes only the oracle, its fixed event inventory and its test count. The producer and reference now both require `result`. A new reconciled corruption changes both to `Text` while keeping UUID ownership and offsets intact, and must be rejected by the exact native-field gate. RP19 increases from 45 to 46 fixed events; BRG08 remains 61. No compiler behavior, immutable contract, shared source cap or time budget changes.

The original failure remains byte-for-byte in the archive. The new green execution does not relabel it.

## Evidence and independent reconciliation

The sibling evidence directory `evidence/p15-r6-unicode-combined-20261121/` contains:

- `originals.zip`: all 23 original files from the original failed and corrected successful attempts, with separate source-labelled directories.
- `manifest.json`: archive SHA-256, each original member’s byte count and digest, and explicit source/scope boundaries.
- `independent-reconciliation.json`: all 117 source paths, Git blob identities, SHA-256 values and byte counts; the fixed 46/61 event inventories; all process/raw bindings; 18 native specimens; original failure metadata; and the unchanged pending-runtime results.

Every source witness was read directly from the tested Git commit and independently matched to both actual process receipts. The 117 witnesses total 2,691,639 bytes. Their sorted-path digest remains `bebcedcda5c5e62ab1c824cb171416b18028976edb22381c74cdee8533d4203d`. Exact clean Git identity separately binds the whole checkout. The three original acceptance, repair and recovery contracts remain byte-identical.

| Original | SHA-256 |
| --- | --- |
| Corrected outer raw JSONL | `cef9a51d514f58ed7aad3367f92566588fbe50b71c0bbcc116bde0e484631a82` |
| RP19 raw JSONL | `12417b708c1f5a025499bc837e42faf0624142318352d631a1e7ab2861196438` |
| RP19 process receipt | `a5f17f24d39d412dc0c63ad08147c7841fbf98e04fc9ec6db9a2325fd95e29df` |
| BRG08 raw JSONL | `2987fe0eb15936059aacc1e991f54f5a3d43ea6ff18c69504be3bbd747294c86` |
| BRG08 process receipt | `07b72fa1e9f6a347d3fbd8e0a9cd27c19f0f0bf9dcbf34457c55a4f8427f76f7` |

No Apple rendered text, transformed runtime value, iOS UI, release-wide acceptance or final combined-source CI is asserted by this proof. Those gates remain open.
