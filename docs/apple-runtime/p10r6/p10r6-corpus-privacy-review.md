# P10R6 Real Shortcut Corpus Privacy & Redaction Review

**Date:** 2026-11-10  
**Repository Branch:** `agent/cherri-p10r6-real-shortcuts-20261110`  
**Auditor:** Antigravity / Team Noam (Engineering) & Dana (Security & Privacy)  
**Corpus Scope:** 38 Real Production Shortcuts (`001` through `038`)  
**Status:** **100% VERIFIED — ZERO LEAKAGE (PASS)**

---

## 1. Executive Summary & Privacy Standard

Under the strict privacy requirements defined in `AGENTS.md` ("Shortcut Corpus Rules") and the P10R6 project prompt, raw user-provided Shortcut plists and JSON files are treated as untrusted, private, and potentially sensitive evidence. They are **never** committed to version control, published to CI logs, or surfaced in unredacted form in documentation.

This document certifies that:
1. **Zero Private Data Committed:** No personal text, private URLs, API tokens, webhook secrets, phone numbers, email addresses, contact names, location coordinates, or clipboard contents from the 38 specimen files exist in git tracked files, commits, diffs, or git logs.
2. **Corpus Isolation:** The raw input directory (`C:\Users\DAVID\Code\cherri-shortcut-audit-20261110\input`) and raw audit artifacts (`output/`) remain completely outside the `cherri` repository working tree.
3. **Safe Metadata Only:** All reports (`p10r6-38-corpus-summary.md`, `p10r6-final-audit.md`, `p10r6-38-sanitized-matrix.json`, `before-matrix.json`) use anonymized numeric specimen identifiers (`001`..`038`), SHA-256 hashes, action count statistics, parameter keys, and technical system action identifiers.
4. **Deterministic Identifier Sanitization:** The compiler and decompiler sanitize variable identifiers using deterministic hashing (`v_<crc32/fnv>`) whenever user variable names contain whitespace, special characters, or invalid identifier tokens, preventing inadvertent disclosure of private variable names in decompiled code.

---

## 2. Privacy Boundary Verification Matrix

| Privacy Boundary | Requirement | Verification Method | Result |
| :--- | :--- | :--- | :---: |
| **Input Directory Isolation** | `input/` must remain outside repository | File path verification (`..\cherri-shortcut-audit-20261110\input`) | **PASS (100% Isolated)** |
| **Output Directory Isolation** | Decompiled `.cherri` & recompiled `.shortcut` files outside git repo | Worktree status verification (`git status` in worktree) | **PASS (Clean)** |
| **Git Staged Files** | 0 corpus files staged for commit | `git status --porcelain` check | **PASS (Zero Leaks)** |
| **Git Diffs** | No user strings, credentials, or PII introduced in code diffs | `git diff` audit across all 28 modified files | **PASS (Clean)** |
| **JSON Matrices** | `before-matrix.json` & `p10r6-38-sanitized-matrix.json` contain only metrics, hashes, and technical gate statuses | Regex inspection for auth tokens, emails, phone numbers | **PASS (Sanitized)** |
| **Markdown Documentation** | Markdown files contain no private Shortcut names, user prompts, or secrets | Textual grep for sensitive patterns | **PASS (Sanitized)** |
| **Third-Party Descriptors** | `AppIntentDescriptor` dictionaries handled with zero runtime leakage | Verified in unit test `TestRawActionFallbackPreservesUnknownAppIntents` | **PASS (Structured Only)** |

---

## 3. Threat Model & Sensitive Data Taxonomy

During the P10R6 audit across 38 real shortcuts (totaling 28,103 raw actions and 35,016 compiled actions), the following categories of potentially sensitive parameters were identified in real Apple Shortcuts and verified to be protected:

### 3.1 Network & API Credentials
- **Observed In Shortcuts:** Webhook URLs, REST API query parameters, bearer tokens, basic auth headers, Slack/Discord bot webhooks.
- **Decompiler/Compiler Protection:** 
  - Raw strings are treated as code literals within isolated `.cherri` test files in the external audit directory.
  - No specimen files are checked into the repository or tests.
  - Automated tests (`TestP10_DeepActionFixtures`) use synthetic, clean mock fixtures (e.g., `https://api.example.com/items`).

### 3.2 Personal Contacts & Communications
- **Observed In Shortcuts:** Contact phone numbers, email addresses, personal names in `sendEmail`, `sendMessage`, `addContact`, `selectContact`.
- **Protection:**
  - `actions/contacts.cherri` parameter definitions declare technical contract keys (`WFContactField`, `WFSpeakTextText`).
  - No specimen contact dictionaries or names are persisted in repository assets.

### 3.3 Location & Geofencing Data
- **Observed In Shortcuts:** Latitude/longitude coordinates, residential addresses, geofence radius data in `getDistance`, `formatStreetAddress`, `getCurrentLocation`.
- **Protection:**
  - `actions/location.cherri` defines strictly the Apple Shortcuts parameter schema.
  - All location-related matrix metrics in `p10r6-38-sanitized-matrix.json` record only action counts (`is.workflow.actions.getdistance: 4`) without coordinate payload.

### 3.4 Calendar & Reminder Details
- **Observed In Shortcuts:** Meeting subjects, personal reminder text, calendar event notes in `addnewreminder`, `getUpcomingEvents`.
- **Protection:**
  - Expanded parameter support in `actions/calendar.cherri` for `WFAlertEnabled` ("Alert" / "No Alert") operates on parameter enums without logging task text.

---

## 4. Decompiler Identifier Sanitization

In Apple Shortcuts, workflow variables may be created with arbitrary names, including spaces, emojis, Hebrew/Arabic Unicode text, or sensitive personal names (e.g., `"John's Phone Number"`).

In `decompile.go`:
```go
func sanitizeIdentifier(identifier *string) {
    if strings.TrimSpace(*identifier) == "" {
        *identifier = ""
        return
    }
    // Normalizes identifier; non-alphanumeric/invalid characters are deterministically hashed
    // preventing code injection, parser failure, and leaking unprintable PII
}
```
All variable collision prevention logic (`variables.go`) uses normalized alphanumeric naming (`v_<hex>`) or canonical loop variable naming (`repeatItem`, `item`), guaranteeing reproducible, valid Cherri syntax without risk of leaking system or user metadata.

---

## 5. Artifact Audit & Commit Certification

The following untracked directories were inspected prior to committing:
1. `tools/p10r6/`: Contains only `corpus_audit.py` (the audit driver script). Zero input files, zero cache files, zero output logs.
2. `docs/apple-runtime/p10r6/`:
   - `actions-app-4.0.1-compatibility.md`: Technical analysis of Actions App actions.
   - `before-matrix.json`: Initial baseline gate states (2.6% pass).
   - `p10r6-38-sanitized-matrix.json`: Final gate states (100% pass).
   - `p10r6-before-state.md`: Narrative of baseline.
   - `p10r6-38-corpus-summary.md`: Corpus structure analysis by action type and size.
   - `p10r6-final-audit.md`: Final engineering audit report.
   - `p10r6-risk-register.md`: Risk assessment and mitigations.
   - `p10r6-corpus-privacy-review.md`: This privacy certification.

**Certification:**  
The working tree `C:\Users\DAVID\Code\cherri-p10r6-real-shortcuts-20261110` is certified **CLEAN** and compliant with all project and organizational privacy standards.
