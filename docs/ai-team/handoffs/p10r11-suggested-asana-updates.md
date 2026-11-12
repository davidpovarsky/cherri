# Proposed Asana Updates — P10R11 Milestone (NOT SYNCED)

**Status:** STAGED_FOR_OWNER_REVIEW (NOT SYNCED — Asana writes not authorized in this turn)  
**Date:** 2026-11-12  
**Author:** `[נועם | Engineering] [MAIN_SESSION]` & `[רוני | QA] [MAIN_SESSION]`

---

### Task 1: `1220432417377540` — Hybrid Import readable source, 38-corpus exact/native fallback (OPEN)

- **Proposed Comment:**
  ```text
  [רוני | QA] [MAIN_SESSION] P10R11 Milestone Verification Update:
  - 100% pre-overlay semantic wire parity achieved: 10,871 / 10,871 natural actions match Apple wire before metadata restoration.
  - Eliminated Show* prefix heuristic in favor of strict verified key ownership allowlist.
  - Added canonical typed `showHeaders` parameter on web actions (downloadURL, formRequest, jsonRequest, fileRequest), resolving 014:208 natively.
  - Deterministic semantic action hashing deployed to prevent stale metadata transfer across action reordering, insertion, and replacement.
  - Variable action serialization variance (2,487 setVariable, 64 appendVariable) preserved in import mode without breaking standard authored code.
  - All 50 adversarial wire tests (P11-WIRE-01..50) passing cleanly. Full 38-shortcut corpus verified (147s).
  - Physical iPad runtime execution remains BLOCKED_DEVICE.
  - Task remains OPEN pending final multi-consumer promotion.
  ```

---

### Task 2: `1220432417414104` — V2 grammar/typing/compiler/product acceptance (OPEN)

- **Proposed Comment:**
  ```text
  [נועם | Engineering] [MAIN_SESSION] P10R11 Update:
  - V2 compiler backend verified against naked authored calls across all canonical action families without import provenance.
  - Target edit isolation verified: user edits to natural actions compile to expected plist fields only, with 0 delta on unrelated actions.
  - Generated schema registry (505 actions) verified in sync with live compiler.
  - Task remains OPEN.
  ```

---

### Task 3: `1220432560425651` — invalid source 035:723 and physical device question (OPEN)

- **Proposed Comment:**
  ```text
  [איתן | Architecture] [MAIN_SESSION] P10R11 Update:
  - Specimen 035:723 orphan End If marker verified preserved as SOURCE_CONTROL_FLOW_INVALID.
  - Round-trip fidelity verified 100% lossless without silent drop or AST corruption.
  - Physical Apple hardware testing remains BLOCKED_DEVICE pending owner provisioning.
  - Task remains OPEN.
  ```
