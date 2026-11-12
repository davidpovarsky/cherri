# P10R10 Suggested Asana Updates (`[MAIN_SESSION]`)

> Status note: Live Asana tasks receive exact implementation commit SHA, provenance commit SHA, and GitHub Actions run URL/conclusions once commit, provenance metadata update, and CI completion are observed. Hardware runtime tasks remain open with explicit `APPLE_RUNTIME_EXECUTED=BLOCKED_DEVICE` reporting.

## Target Task Mapping

1. **`1220432417377540` — `[P10R9][P0][DEV] Hybrid Import — המרה לקוד Cherri קריא עם fallback Lossless לכל פעולה`**
   - **Execution Tag:** `[נועם | DEV] [MAIN_SESSION]` + `[רוני | QA] [MAIN_SESSION]`
   - **Action:** Post P10R10 completion evidence story (38/38 lossless & hybrid Legacy/V2 bit-for-bit parity, `10,871 / 27,652` = `39.31%` natural readable actions vs `1,463 / 27,652` = `5.29%` in P10R9, `40/40 TestSEM`, `30/30 TestHYB`, provenance stripping, fail-closed verifier, CI URL/jobs). Keep task **OPEN** pending iPadOS app editor/preview sync & owner `main` merge gate.

2. **`1220373052713862` — `[P0][DECOMPILE] Lossless round-trip / Decompiler parity`**
   - **Execution Tag:** `[נועם | DEV] [MAIN_SESSION]`
   - **Action:** Post P10R10 unification story explaining reuse of `decompile.go` semantic passes inside `planUnifiedSemanticHybridImport` (`semantic_import_planner.go`), `#workflowheader` provenance (`import_provenance.go`), and 38/38 lossless+hybrid parity (`035:723` `SOURCE_INVALID`). Keep task **OPEN** (`APPLE_RUNTIME_EXECUTED=BLOCKED_DEVICE`).

3. **`1220432560425651` — `[P10R9][P1][QA] קיצור 035 — בדיקה ושימור source control-flow`**
   - **Execution Tag:** `[רוני | QA] [MAIN_SESSION]`
   - **Action:** Post P10R10 re-verification confirming `035:723` remains classified `SOURCE_INVALID` with 38/38 lossless & hybrid wire identity preserved alongside `931` natural readable actions in `035`. Keep task **OPEN** (`BLOCKED_DEVICE`).

4. **`1220432417414104` — `[P10R9][P1][ARCH/QA] V2 Language Acceptance`**
   - **Execution Tag:** `[איתן | ARCH] [MAIN_SESSION]` + `[רוני | QA] [MAIN_SESSION]`
   - **Action:** Post P10R10 Cherri v2 shared planner & `0/38 -> 38/38` hybrid semantic+native parity vs original semantic V2 (`1/38` recompile, `0/38` exact parity), plus `38/38` Legacy/V2 plist equality. Keep task **OPEN** (`BLOCKED_DEVICE`).

5. **`1220432417485023` — `[P10R9][P1][INT] Promotion: Open Minis Skill + Downstream Pinning`**
   - **Execution Tag:** `[תומר | INT] [MAIN_SESSION]`
   - **Action:** Post updated `skills/cherri-shortcuts/` documentation (`SKILL.md`, `references/workflow.md`) and `compatibility-manifest.json` pinning status on the P10R10 branch. Keep task **OPEN** pending owner-approved promotion QA.
