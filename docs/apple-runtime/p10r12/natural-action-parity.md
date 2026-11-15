# P10R12 Natural Action Wire Parity & Independent 38×2 Oracle Verification

**Generated:** `2026-11-15T05:11:58Z`  
**Compiler Binary SHA-256:** `7407548dd6a768c86414ff1495a14d3cfb2ee669bd1e6bd72a08e59a01fe8bad`  
**No-Patch Audit Binary SHA-256:** `87b029d8c7db0845a3b343b9ab63a7fcd9175e82a50c634ad22ea7395870e911` (`-tags cherri_oracle_nopatch`)  
**Anti-Tautology Suite:** **9 / 9 PASS** (`AT-01`..`AT-09`)

---

## 1. Aggregate Multi-Layer Verification Summary (38 × 2 = 76 Runs)

| Verification Layer | Per Dialect | Both Dialects (38 × 2) | Exact Matches | Divergences |
|---|---:|---:|---:|---:|
| **Layer 1: Full Recursive Typed Plist Tree Exact** | 38 / 38 runs | 76 / 76 runs | **76** | **0** |
| **Layer 2: Pre-Overlay Action Identifiers** | 22,490 | 44,980 | **44,980** | **0** |
| **Layer 3: Pre-Overlay Semantic Parameters (Before Restore)** | 22,490 | 44,980 | **44,980** | **0** |
| **Layer 3: Pre-Overlay Non-Identity Incl. `Show-*` Exact** | 22,490 | 44,980 | **44,980** | **0** |
| **Layer 3: Post-Overlay Native Wire Exact** | 22,490 | 44,980 | **44,980** | **0** |
| **Third-Party Native Descriptor Preservation** | 137 | 274 | **274** | **0** |
| **Stage-3 Demotions / Unresolved `CANDIDATE`s / Mode Parity Diffs** | 0 | 0 | — | **0** |

*Note on No-Patch Oracle Contrast:* With metadata restoration compiled out (`nopatch` binary), `prepatch_only_identity_mismatch` is `19,622 / 22,490` in Legacy and `22,490 / 22,490` in V2, while `prepatch_fullparam_equal` is `2,549` (Legacy) and `5,409` (V2), confirming `AT-09` (no-patch oracle genuinely strips overlay restoration without altering compiled source).

---

## 2. Per-Specimen Verification Ledger (`001`..`038`)

| ID | Total | Natural | Fallback | Invalid | Pre-Overlay Semantic Match | Full Tree (Legacy & V2) |
|:---:|---:|---:|---:|---:|---:|:---:|
| 001 | 162 | 121 | 41 | 0 | 121 / 121 | EXACT (L/V2) |
| 002 | 205 | 165 | 40 | 0 | 165 / 165 | EXACT (L/V2) |
| 003 | 202 | 138 | 64 | 0 | 138 / 138 | EXACT (L/V2) |
| 004 | 247 | 215 | 32 | 0 | 215 / 215 | EXACT (L/V2) |
| 005 | 230 | 200 | 30 | 0 | 200 / 200 | EXACT (L/V2) |
| 006 | 43 | 34 | 9 | 0 | 34 / 34 | EXACT (L/V2) |
| 007 | 10 | 8 | 2 | 0 | 8 / 8 | EXACT (L/V2) |
| 008 | 449 | 409 | 40 | 0 | 409 / 409 | EXACT (L/V2) |
| 009 | 449 | 409 | 40 | 0 | 409 / 409 | EXACT (L/V2) |
| 010 | 59 | 50 | 9 | 0 | 50 / 50 | EXACT (L/V2) |
| 011 | 169 | 137 | 32 | 0 | 137 / 137 | EXACT (L/V2) |
| 012 | 2474 | 2100 | 374 | 0 | 2100 / 2100 | EXACT (L/V2) |
| 013 | 73 | 51 | 22 | 0 | 51 / 51 | EXACT (L/V2) |
| 014 | 519 | 442 | 77 | 0 | 442 / 442 | EXACT (L/V2) |
| 015 | 49 | 41 | 8 | 0 | 41 / 41 | EXACT (L/V2) |
| 016 | 850 | 749 | 101 | 0 | 749 / 749 | EXACT (L/V2) |
| 017 | 8 | 6 | 2 | 0 | 6 / 6 | EXACT (L/V2) |
| 018 | 2683 | 2221 | 462 | 0 | 2221 / 2221 | EXACT (L/V2) |
| 019 | 263 | 205 | 58 | 0 | 205 / 205 | EXACT (L/V2) |
| 020 | 161 | 116 | 45 | 0 | 116 / 116 | EXACT (L/V2) |
| 021 | 2683 | 2221 | 462 | 0 | 2221 / 2221 | EXACT (L/V2) |
| 022 | 1831 | 1358 | 473 | 0 | 1358 / 1358 | EXACT (L/V2) |
| 023 | 507 | 382 | 125 | 0 | 382 / 382 | EXACT (L/V2) |
| 024 | 850 | 693 | 157 | 0 | 693 / 693 | EXACT (L/V2) |
| 025 | 2728 | 2263 | 465 | 0 | 2263 / 2263 | EXACT (L/V2) |
| 026 | 180 | 138 | 42 | 0 | 138 / 138 | EXACT (L/V2) |
| 027 | 2398 | 1891 | 507 | 0 | 1891 / 1891 | EXACT (L/V2) |
| 028 | 522 | 399 | 123 | 0 | 399 / 399 | EXACT (L/V2) |
| 029 | 76 | 59 | 17 | 0 | 59 / 59 | EXACT (L/V2) |
| 030 | 850 | 693 | 157 | 0 | 693 / 693 | EXACT (L/V2) |
| 031 | 957 | 741 | 216 | 0 | 741 / 741 | EXACT (L/V2) |
| 032 | 7 | 4 | 3 | 0 | 4 / 4 | EXACT (L/V2) |
| 033 | 522 | 399 | 123 | 0 | 399 / 399 | EXACT (L/V2) |
| 034 | 521 | 436 | 85 | 0 | 436 / 436 | EXACT (L/V2) |
| 035 | 2155 | 1733 | 421 | 1 | 1733 / 1733 | EXACT (L/V2) |
| 036 | 521 | 449 | 72 | 0 | 449 / 449 | EXACT (L/V2) |
| 037 | 829 | 646 | 183 | 0 | 646 / 646 | EXACT (L/V2) |
| 038 | 210 | 168 | 42 | 0 | 168 / 168 | EXACT (L/V2) |
