# P10R9 Hybrid Readable Import Coverage Matrix (`001`–`038`)

- **Total Specimens:** `38/38`
- **Total Actions:** `27652`
- **Verified Natural Actions (`NATURAL_VERIFIED`):** `1463` (`5.29%`)
- **Exact Native Fallback Actions (`EXACT_NATIVE_FALLBACK`):** `26188`
- **Source-Invalid Actions (`SOURCE_INVALID`):** `1` (`035` index `723` `CONTROL_FLOW_ORPHAN_END`)
- **Lossless Legacy / V2 Oracle Pass:** `38/38` / `38/38`
- **Hybrid Legacy / V2 Oracle Pass:** `38/38` / `38/38`
- **Hybrid Legacy vs V2 Full Tree Equality:** `38/38`

## 1. Global Classification & Reason Breakdown

| Reason Code | Action Count |
| :--- | ---: |
| `AMBIGUOUS_VARIANT` | 46 |
| `CONTROL_REGION` | 10570 |
| `KNOWN_DISAMBIGUATED` | 1463 |
| `SOURCE_INVALID` | 1 |
| `THIRD_PARTY_NO_CANONICAL_DEF` | 137 |
| `UNKNOWN_NATIVE_ID` | 13 |
| `UNREPRESENTABLE_PARAM` | 15422 |

## 2. Verified Natural Callables Emitted Across the 38-Shortcut Corpus

| Canonical Callable | Verified Instances |
| :--- | ---: |
| `comment` | 655 |
| `stop` | 444 |
| `text` | 152 |
| `alert` | 56 |
| `speak` | 33 |
| `dismissSiri` | 19 |
| `wait` | 16 |
| `waitToReturn` | 13 |
| `vibrate` | 12 |
| `show` | 9 |
| `showWebpage` | 9 |
| `createFolder` | 6 |
| `nothing` | 6 |
| `setBrightness` | 4 |
| `setVolume` | 4 |
| `returnToHomescreen` | 3 |
| `showNotification` | 3 |
| `toggleAppearance` | 3 |
| `define` | 2 |
| `lightMode` | 2 |
| `output` | 2 |
| `reboot` | 2 |
| `setAirdropReceiving` | 2 |
| `shutdown` | 2 |
| `confirm` | 1 |
| `toggleBluetooth` | 1 |
| `toggleSilenceUnknownCallers` | 1 |
| `toggleWifi` | 1 |

## 3. Per-Specimen Hybrid Coverage & Verification Table (`001`–`038`)

| ID | Total Actions | `NATURAL_VERIFIED` | `EXACT_NATIVE_FALLBACK` | `SOURCE_INVALID` | Control Flow | Lossless (Leg/V2) | Hybrid (Leg/V2) | Oracle & Tree Equal |
| :--- | ---: | ---: | ---: | ---: | :--- | :--- | :--- | :--- |
| `001` | 162 | 4 | 158 | 0 | VALID | PASS / PASS | PASS / PASS | **PASS (100%)** |
| `002` | 205 | 9 | 196 | 0 | VALID | PASS / PASS | PASS / PASS | **PASS (100%)** |
| `003` | 202 | 24 | 178 | 0 | VALID | PASS / PASS | PASS / PASS | **PASS (100%)** |
| `004` | 247 | 13 | 234 | 0 | VALID | PASS / PASS | PASS / PASS | **PASS (100%)** |
| `005` | 230 | 13 | 217 | 0 | VALID | PASS / PASS | PASS / PASS | **PASS (100%)** |
| `006` | 43 | 2 | 41 | 0 | VALID | PASS / PASS | PASS / PASS | **PASS (100%)** |
| `007` | 10 | 0 | 10 | 0 | VALID | PASS / PASS | PASS / PASS | **PASS (100%)** |
| `008` | 449 | 33 | 416 | 0 | VALID | PASS / PASS | PASS / PASS | **PASS (100%)** |
| `009` | 449 | 34 | 415 | 0 | VALID | PASS / PASS | PASS / PASS | **PASS (100%)** |
| `010` | 59 | 5 | 54 | 0 | VALID | PASS / PASS | PASS / PASS | **PASS (100%)** |
| `011` | 169 | 8 | 161 | 0 | VALID | PASS / PASS | PASS / PASS | **PASS (100%)** |
| `012` | 2474 | 104 | 2370 | 0 | VALID | PASS / PASS | PASS / PASS | **PASS (100%)** |
| `013` | 73 | 3 | 70 | 0 | VALID | PASS / PASS | PASS / PASS | **PASS (100%)** |
| `014` | 519 | 32 | 487 | 0 | VALID | PASS / PASS | PASS / PASS | **PASS (100%)** |
| `015` | 49 | 2 | 47 | 0 | VALID | PASS / PASS | PASS / PASS | **PASS (100%)** |
| `016` | 850 | 99 | 751 | 0 | VALID | PASS / PASS | PASS / PASS | **PASS (100%)** |
| `017` | 8 | 0 | 8 | 0 | VALID | PASS / PASS | PASS / PASS | **PASS (100%)** |
| `018` | 2683 | 115 | 2568 | 0 | VALID | PASS / PASS | PASS / PASS | **PASS (100%)** |
| `019` | 263 | 13 | 250 | 0 | VALID | PASS / PASS | PASS / PASS | **PASS (100%)** |
| `020` | 161 | 12 | 149 | 0 | VALID | PASS / PASS | PASS / PASS | **PASS (100%)** |
| `021` | 2683 | 115 | 2568 | 0 | VALID | PASS / PASS | PASS / PASS | **PASS (100%)** |
| `022` | 1831 | 77 | 1754 | 0 | VALID | PASS / PASS | PASS / PASS | **PASS (100%)** |
| `023` | 507 | 40 | 467 | 0 | VALID | PASS / PASS | PASS / PASS | **PASS (100%)** |
| `024` | 850 | 124 | 726 | 0 | VALID | PASS / PASS | PASS / PASS | **PASS (100%)** |
| `025` | 2728 | 69 | 2659 | 0 | VALID | PASS / PASS | PASS / PASS | **PASS (100%)** |
| `026` | 180 | 2 | 178 | 0 | VALID | PASS / PASS | PASS / PASS | **PASS (100%)** |
| `027` | 2398 | 134 | 2264 | 0 | VALID | PASS / PASS | PASS / PASS | **PASS (100%)** |
| `028` | 522 | 6 | 516 | 0 | VALID | PASS / PASS | PASS / PASS | **PASS (100%)** |
| `029` | 76 | 3 | 73 | 0 | VALID | PASS / PASS | PASS / PASS | **PASS (100%)** |
| `030` | 850 | 124 | 726 | 0 | VALID | PASS / PASS | PASS / PASS | **PASS (100%)** |
| `031` | 957 | 98 | 859 | 0 | VALID | PASS / PASS | PASS / PASS | **PASS (100%)** |
| `032` | 7 | 2 | 5 | 0 | VALID | PASS / PASS | PASS / PASS | **PASS (100%)** |
| `033` | 522 | 6 | 516 | 0 | VALID | PASS / PASS | PASS / PASS | **PASS (100%)** |
| `034` | 521 | 14 | 507 | 0 | VALID | PASS / PASS | PASS / PASS | **PASS (100%)** |
| `035` | 2155 | 80 | 2074 | 1 | INVALID (`035:723`) | PASS / PASS | PASS / PASS | **PASS (100%)** |
| `036` | 521 | 9 | 512 | 0 | VALID | PASS / PASS | PASS / PASS | **PASS (100%)** |
| `037` | 829 | 26 | 803 | 0 | VALID | PASS / PASS | PASS / PASS | **PASS (100%)** |
| `038` | 210 | 9 | 201 | 0 | VALID | PASS / PASS | PASS / PASS | **PASS (100%)** |
