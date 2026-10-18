# Cherri — Team Assignments & Work Routing

**Asana Project:** [Cherri (1219603572114432)](https://app.asana.com/0/1219603572114432/list)  
**Base Branch:** `agent/cherri-p10-apple-actions-coverage`  
**Setup PR:** https://github.com/davidpovarsky/cherri/pull/6  
**Setup Task:** `1219611682706483`

---

## 1. Project Scope & Technical Boundaries
- P10 Apple Actions coverage: 173 discoverable App Intents, 118 missing built-in actions.
- Compiler state reset integrity (`compiler_state.go`) and roundtrip decompilation parity.
- Signing service integration (Shortcuty Sign / HubSign) and Linux ARM64 remote builder.

---

## 2. Active Open Tasks & Role Assignments

| Task / Area | Logical Lead | Reviewer / QA | Scope & Constraints |
|---|---|---|---|
| **[P10] 173 Missing App Intents** | יעל (yael) | נועם (noam) | Classify and prioritize 173 discoverable App Intents against verified Apple schemas. |
| **[P10] 118 Missing Built-in Actions** | יעל (yael) | נועם (noam) | Map and implement eligible built-in actions in canonical catalog and compiler. |
| **[P10] Raw JSON File Request Parity** | נועם (noam) | יעל (yael) + רוני (roni) | Resolve F5 403 on iPad; verify `WFHTTPBodyType=File` and `WFRequestVariable` direct text. |
| **[P10] HTML file via Set Name -> Show Webpage** | נועם (noam) | מיה (maya) + רוני (roni) | Native Text HTML -> Set Name -> Show Webpage chain without Base64/data URLs. |
| **Remote Builder Linux Signed Compile** | תומר (tomer) | דנה (dana) + נועם (noam) | Fix false-success exit code on unsigned output; enforce AEA1 signature validation. |
| **[P10R2] Acceptance Test Hardening** | רוני (roni) | תומר (tomer) | Replace weak/toy tests (cat-file, Unicode math) with real producer-path adversarial tests. |
| **[P10R2] Timeout Reporting & Ledger** | רוני (roni) | תומר (tomer) | Reconcile coverage ledgers between report and json; correct timeout parameter evidence. |
| **Compiler Architecture & State Reset** | איתן (eitan) | נועם (noam) | Ensure `compiler_state.go` provides authoritative reset across sequential `go test` runs. |

---

## 3. Mandatory Subagent Delegation & Approval Gates
- **חובת האצלה:** הסוכן הראשי פועל כמתאם בלבד ומחויב להפעיל סוכני משנה (`invoke_subagent`). כל קוד מועבר לרוני לאימות.
- **Release & P11:** Do not move to P11 or declare P10 complete based on reporting-only gates.
- **Physical Device:** Tasks with physical iPad runtime verification remain open until verified on hardware.
- **Asana Policy:** Autonomous updates and verified completion are authorized per `docs/ai-team/ASANA_POLICY.md`.
