# Cherri P10R7 — P10R6 Baseline Reconciliation & Metric Integrity

## 1. Objective

Establish a single authoritative reconciliation across all 38 real-world specimens (`001`–`038`) connecting:
- Canonical P10R6 sanitized baseline (`docs/apple-runtime/p10r6/p10r6-38-sanitized-matrix.json` & `docs/apple-runtime/p10r7/p10r7-baseline-sanitized.json`)
- Verified P10R7 post-fix audit (`docs/apple-runtime/p10r7/p10r7-38-sanitized-matrix.json` & `p10r7-sanitized-diff-matrix.json`)

Every number below is computed directly by summing the verified `001`–`038` JSON rows (no manual rounding, placeholder rows, or stale copy-pasted totals).

---

## 2. Historical Metric Discrepancy Audit & Correction

During P10R7 baseline verification, three concrete accounting errors were identified in P10R6 / early P10R7 artifacts and corrected permanently:

1. **Sum Mismatch in P10R6 Prose (`28,103` / `35,016` vs `27,652` / `34,355`):**
   - Summing the authoritative `001`–`038` JSON matrix rows yields **`27,652` source actions** and **`34,355` compiled actions** (`+6,703` net action bloat in P10R6).
   - `docs/apple-runtime/p10r6/p10r6-38-corpus-summary.md` additionally had a copy-paste cell typo on specimens `018` and `021` (`3,383` instead of `3,381`, accounting for `+4`), while narrative summaries in `p10r6-38-corpus-summary.md`, `p10r6-final-audit.md`, and `p10r6-corpus-privacy-review.md` quoted `28,103` and `35,016`.
2. **Tautological P10R6 Oracle Hardcoding (`38/38` vs `3/38` & `0/38`):**
   - `tools/p10r6/corpus_audit.py` originally hardcoded `ACTION_SEQUENCE_PASS = "PASS"` and `FULL_WORKFLOW_SEMANTIC_PASS = "PASS"` without invoking a structural plist comparator.
   - Under the independent strict oracle (`tools/p10r7/independent_fidelity_oracle.py`), only **`3/38`** specimens preserved strict action sequence in the P10R6 baseline (rising to **`21/38`** in P10R7), net corpus action bloat fell from **`+6,703`** to **`+6`** (`27,652 -> 27,658`), and **`0/38`** reach `FULL_WORKFLOW_SEMANTIC_PASS`.
   - `tools/p10r6/corpus_audit.py` has been updated to delegate sequence and semantic gates directly to `independent_fidelity_oracle.evaluate_shortcut_fidelity`, and `p10r6-38-sanitized-matrix.json` & P10R6 prose reports have been reconciled.
3. **Initial P10R7 Reconciliation Table Row Scrambling:**
   - An earlier draft of this file repeated specimen counts (`001=338/390`, `002=382/448`, `036=338/390`) that conflicted with `p10r7-baseline-sanitized.json` (`001=162/192`, `002=205/256`). All 38 rows below are machine-generated directly from the authoritative JSON matrices and enforced by `tools/p10r7/reconcile_private_and_sanitized_reports.py --verify`.

---

## 3. Reconciled Totals (`001`–`038` Sum of Rows)

| Metric | P10R6 Baseline (Authoritative Matrix Sum) | P10R7 Final (Strict Oracle) | Change |
| --- | ---: | ---: | ---: |
| **Total Source Actions** | 27,652 | 27,652 | 0 |
| **Total Compiled Actions (Legacy)** | 34,355 | 27,658 | -6,697 |
| **Net Action Bloat / Delta** | +6,703 (24.24%) | +6 (0.022%) | -6,697 |
| **Strict Action Sequence Pass (`STRICT_ACTION_SEQUENCE_PASS`)** | 3 / 38 | 21 / 38 | +18 |
| **Full Semantic Parity Pass (`FULL_WORKFLOW_SEMANTIC_PASS`)** | 0 / 38 (hardcoded 38/38 retracted) | 0 / 38 | Honest oracle |
| **Physical Apple Device Execution (`APPLE_RUNTIME_EXECUTED`)** | 0 / 38 (`BLOCKED_DEVICE`) | 0 / 38 (`BLOCKED_DEVICE`) | Hardware unavailable |

---

## 4. Specimen-by-Specimen Reconciliation (`001`–`038`)

| ID | Source Actions | P10R6 Compiled | P10R6 Delta | P10R6 Seq | P10R7 Compiled | P10R7 Delta | P10R7 Seq |
| :---: | ---: | ---: | ---: | :---: | ---: | ---: | :---: |
| 001 | 162 | 192 | +30 | FAIL | 162 | +0 | PASS |
| 002 | 205 | 256 | +51 | FAIL | 204 | -1 | FAIL |
| 003 | 202 | 223 | +21 | FAIL | 202 | +0 | PASS |
| 004 | 247 | 298 | +51 | FAIL | 247 | +0 | PASS |
| 005 | 230 | 280 | +50 | FAIL | 230 | +0 | PASS |
| 006 | 43 | 48 | +5 | FAIL | 43 | +0 | PASS |
| 007 | 10 | 10 | +0 | PASS | 10 | +0 | PASS |
| 008 | 449 | 594 | +145 | FAIL | 449 | +0 | PASS |
| 009 | 449 | 594 | +145 | FAIL | 449 | +0 | PASS |
| 010 | 59 | 64 | +5 | FAIL | 59 | +0 | PASS |
| 011 | 169 | 203 | +34 | FAIL | 169 | +0 | PASS |
| 012 | 2,474 | 3,093 | +619 | FAIL | 2,478 | +4 | FAIL |
| 013 | 73 | 92 | +19 | FAIL | 73 | +0 | PASS |
| 014 | 519 | 630 | +111 | FAIL | 517 | -2 | FAIL |
| 015 | 49 | 59 | +10 | FAIL | 49 | +0 | PASS |
| 016 | 850 | 1,023 | +173 | FAIL | 835 | -15 | FAIL |
| 017 | 8 | 8 | +0 | PASS | 8 | +0 | PASS |
| 018 | 2,683 | 3,381 | +698 | FAIL | 2,695 | +12 | FAIL |
| 019 | 263 | 312 | +49 | FAIL | 263 | +0 | PASS |
| 020 | 161 | 237 | +76 | FAIL | 161 | +0 | PASS |
| 021 | 2,683 | 3,381 | +698 | FAIL | 2,695 | +12 | FAIL |
| 022 | 1,831 | 2,117 | +286 | FAIL | 1,839 | +8 | FAIL |
| 023 | 507 | 632 | +125 | FAIL | 507 | +0 | PASS |
| 024 | 850 | 1,025 | +175 | FAIL | 836 | -14 | FAIL |
| 025 | 2,728 | 3,441 | +713 | FAIL | 2,740 | +12 | FAIL |
| 026 | 180 | 215 | +35 | FAIL | 180 | +0 | PASS |
| 027 | 2,398 | 3,004 | +606 | FAIL | 2,404 | +6 | FAIL |
| 028 | 522 | 657 | +135 | FAIL | 522 | +0 | PASS |
| 029 | 76 | 95 | +19 | FAIL | 76 | +0 | PASS |
| 030 | 850 | 1,025 | +175 | FAIL | 836 | -14 | FAIL |
| 031 | 957 | 1,189 | +232 | FAIL | 955 | -2 | FAIL |
| 032 | 7 | 7 | +0 | PASS | 7 | +0 | PASS |
| 033 | 522 | 657 | +135 | FAIL | 522 | +0 | PASS |
| 034 | 521 | 696 | +175 | FAIL | 523 | +2 | FAIL |
| 035 | 2,155 | 2,607 | +452 | FAIL | 2,154 | -1 | FAIL |
| 036 | 521 | 632 | +111 | FAIL | 519 | -2 | FAIL |
| 037 | 829 | 1,117 | +288 | FAIL | 831 | +2 | FAIL |
| 038 | 210 | 261 | +51 | FAIL | 209 | -1 | FAIL |
| **TOTAL** | **27,652** | **34,355** | **+6,703** | **3/38** | **27,658** | **+6** | **21/38** |
