# P10R11 Source Size & Performance Benchmark (Measured Physical Files)

**Generated:** 2026-11-12  
**Compiler Binary SHA-256:** `875c297660e1eeb8ce7a46c8755957f78a4be5434be9440ad8e151a88220136d`  
**Measurement Method:** Direct physical file inspection (`os.path.getsize`) across all 38 normalized originals (`bplist00` binary plists) and freshly emitted Cherri source files (`legacy` and `v2`).

---

## 1. Disjoint Corpus Reconciliation (Denominator = 27,652)

Across all 38 corpus shortcuts in the private test corpus, the exact action accounting is:

| Category | Baseline (P10R10) | Expanded (P10R11) | Description |
| :--- | :---: | :---: | :--- |
| **`NATURAL_VERIFIED`** | **10,871** (39.31%) | **11,480** (41.52%) | Cleanly represented as natural Cherri callable actions (including promoted `fileRequest`/`downloadURL`/`getValue` in P10R11) |
| **`EXACT_NATIVE_FALLBACK`** | **16,780** (60.68%) | **16,171** (58.48%) | Preserved as exact native blocks (`CONTROL_REGION`: 10,160, `UNREPRESENTABLE_PARAM`: 5,815, `THIRD_PARTY`: 137, `AMBIGUOUS_VARIANT`: 46, `UNKNOWN_NATIVE_ID`: 13) |
| **`SOURCE_INVALID`** | **1** (0.00%) | **1** (0.00%) | Specimen `035` action index `723` (orphan EndIf marker without open condition) |
| **TOTAL ACTIONS** | **27,652** (100.0%) | **27,652** (100.0%) | **10,871 + 16,780 + 1 = 27,652** (Baseline) / **11,480 + 16,171 + 1 = 27,652** (P10R11) |

---

## 2. Key Specimens: Measured Physical Byte Sizes

| Specimen | Actions | Native File (`bplist00`) | Legacy Source (`.cherri`) | V2 Source (`.cherri`) | Largest Leaf | Significant Payloads (>=1KB) | Notes |
| :---: | :---: | :---: | :---: | :---: | :---: | :---: | :--- |
| **007** | 10 | **531,945 B** | **538,802 B** | **538,830 B** | 526,430 B | 528,117 B | Dominated by 526 KB embedded base64 string |
| **012** | 2,474 | **405,698 B** | **2,232,938 B** | **2,264,326 B** | 12,047 B | 27,950 B | Highly discrete (2,474 actions); Cherri wrapper text expands compact binary plist |
| **017** | 8 | **14,007,968 B** | **9,604,708 B** | **9,604,791 B** | 7,352,210 B | 7,352,210 B | Single giant embedded text leaf of 7.35 MB; binary plist contains redundant offset table |
| **018** | 2,683 | **1,057,130 B** | **3,155,766 B** | **3,192,051 B** | 515,839 B | 586,846 B | Large utility shortcut with 515 KB embedded payload leaf |
| **026** | 180 | **6,433,993 B** | **9,603,732 B** | **9,606,109 B** | 14,664 B | 8,720,000 B | Deep nested dictionary payload items |
| **027** | 2,398 | **328,270 B** | **2,156,466 B** | **2,186,542 B** | 20,284 B | 31,500 B | Dense discrete action sequences |
| **034** | 521 | **9,115,042 B** | **9,468,115 B** | **9,474,379 B** | 3,679,548 B | 9,017,028 B | Multiple large dictionary payload leaves (3.68 MB + 3.22 MB + 1.06 MB) |
| **035** | 2,155 | **5,892,051 B** | **7,680,101 B** | **7,705,462 B** | 5,488,968 B | 5,507,770 B | Single giant embedded text payload leaf of 5.49 MB |

---

## 3. Explanation of Corrected Prior Estimates

1. **Specimen 017:** Prior report cited `7,352,100 B`. Measured file analysis demonstrates that **7,352,210 B** is the byte length of the *single largest UTF-8 string leaf* inside action 0 (`is.workflow.actions.gettext`), while the physical binary plist file on disk is **14,007,968 B**. The Cherri emitted source file is **9,604,708 B** (Cherri represents the text string cleanly without binary plist dictionary index overhead).
2. **Specimen 035:** Prior report cited `5,490,000 B`. Measured file analysis shows that **5,488,968 B** is the exact byte length of the *single largest embedded text leaf* in action 82, while the physical binary plist file is **5,892,051 B** and the generated Cherri source is **7,680,101 B**.
3. **Specimen 012 & 027:** These shortcuts contain thousands of discrete small actions (2,474 and 2,398 actions respectively). In Apple's binary plist format (`bplist00`), small repeated dictionary keys are deduplicated into single byte tokens (405 KB and 328 KB). In Cherri hybrid source, each action requires human-readable text and metadata wrappers, resulting in source files of ~2.2 MB. This overhead is legitimate and expected until complex parameter DSL coverage is promoted in later milestones.
