# Joined public frontend and editor evidence — 25 November 2026

## Implemented source

The production candidate is `0a69b6c7dc2bca3f89d1b5573080deb3391bca06` on `agent/cherri-p10r15-public-frontend-routes-20261125`. It includes the exact editor-edit/versioned-snapshot fix `9743cc3385a6ad03a15bda18286d5a1eb4c0511b`. The new route tests were frozen in `6c458de77908fbd8124bfbf626a541ff5ba059ef`; the production routing/diagnostic fix is `598c69dd63349f0748013d28bac3cc867863f291`.

Auto mode now uses the shared lexer and real comment/directive boundaries. Raw strings, nested comments and directive-like text retain their literal meaning. Only actual Legacy markers or directives choose Legacy. Malformed V2 input stays on V2 and returns the original `E_SYNTAX` code and range. Rejected V2 input does not retry through the Legacy parser or reach the signer.

## Actual verification

| Original run | Outcome | Duration |
| --- | --- | --- |
| Immutable new tests before repair | 42 named passes; 13 failing child cases and 3 failing parents | 31.873 s |
| Repaired public routes plus RP52/U04 regressions | 132 named passes; zero failures or skips | 43.268 s |
| Joined requirement inventory and evidence-intake controls | 52 named passes; zero failures or skips | 9.066 s |

The new test file is byte-identical in RED and GREEN: `ab24592680327a7bdf06c64b67ce68394856e359104ea018cf7cbc79b6291506`. The passing public-route run includes 58 named events for the three BRG25 producers. Five shared fixtures cover three literal cases, a typed-value error and a malformed function. Both actual CLI forms, default/V2 modes, check JSON, actual mobile routes, lexical auto-mode boundaries and healthy Legacy/signer controls are exercised.

The test builds the actual CLI with standard Go coverage instrumentation. The original JSONL preserves 28 complete losslessly compressed counter profiles. Independent re-reading verified every profile length and SHA-256 and the actual `initParse` entry block: 26 V2/check/signer controls have zero Legacy entries; the two explicit Legacy controls have one entry each. Positive signer controls reach the local HTTP observer; invalid input leaves no output and no signer call.

The canonical producer records only BRG25 integration and BRG27 service/integration tiers. Immutable requirement titles, scenarios, expected observations, required tiers and exact child-event inventories are checked. Dedicated Apple evidence remains required. These local results do not close BRG25, BRG27 or all 179 cases.

## Original evidence

Base: `/mnt/development/projects/evidence/cherri-p10r15-final-completion-20261125`. All listed originals remain unchanged; source snapshots match before and after.

| Receipt | SHA-256 |
| --- | --- |
| `brg25-public-routes-red-01/result.original.json` | `fd952315cd2fe859e3982df9dbb413e66e08049e92864a820e9447c167ca83dc` |
| `brg25-public-routes-green-02/result.original.json` | `748f88301eb1527940f5795089a983bd5f08e51f7cd224786631c6e91a922005` |
| `joined-public-intake-controls-01/result.original.json` | `641b3134098e588fe09c0c150c3017665099bdff8749fc69a5d49905fca312e2` |
| `joined-public-independent-review-01/result.original.json` | `8debcf5c06b997210d3a8dbddebac0f703ebbb5bd86764c6f601700944e4676e` |

The earlier green-01 launch stopped before Go at capacity admission; it is not a failed semantic run or a passing test. Its original preflight/exception is retained. Native C ABI, Swift and real UI verification continue on a separate Apple branch. Final candidate acceptance must execute the canonical producers again on that exact integrated source.
