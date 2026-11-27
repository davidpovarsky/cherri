# Cherri — Team Assignments & Work Routing

**Asana Project:** [Cherri (1219603572114432)](https://app.asana.com/0/1219603572114432/list)

**Repository:** [davidpovarsky/cherri](https://github.com/davidpovarsky/cherri)

**Routing snapshot:** 2026-11-27 UTC

**Setup PR:** https://github.com/davidpovarsky/cherri/pull/6

**Setup Task:** [1219611682706483](https://app.asana.com/0/1219603572114432/1219611682706483)

---

## 1. Current Source and Session Ownership

This dated routing snapshot records the coordinated P10R15 handoff. Before changing a shared area, reconcile newer task comments and the active session's ownership record. Logical role names describe responsibilities; they do not establish that a named role agent is running.

| Workstream | Recorded owner | Pinned source / branch | Boundary |
|---|---|---|---|
| Primary P10R15 integration and final acceptance | `[תומר \| INT] [MAIN_SESSION]`; [master task](https://app.asana.com/0/1219603572114432/1220635996500993) | Baseline `a7631a04b36e3a4ed0dffbb919d6508252024365` | Owns the original full Go gate, raw505/C0/C1/C2/56, final179, Apple FN04, RP39/full94, and release decisions. Adopt reviewed supplemental commits through a recorded integration handoff. |
| Scoped-block early-exit | Primary MAIN_SESSION | `agent/cherri-p10r15-scoped-block-early-exit-20261127` | Primary owns further scoped-block changes. Coordinate shared `scoped_blocks_legacy.go` edits against the frozen cache implementation. |
| Supplemental paginate, cache, and debug export | Supplemental macro session; see the local ownership record below | Frozen integration `6bd41650de84639c8e21cc692a3e1605948e83b9` | The three component implementations are frozen for review and integration. This source checkpoint does not complete native, provider, or final acceptance obligations. |
| TEAM routing documentation | Supplemental session, delegated documentation work | `agent/cherri-team-routing-20261127`, based on `6bd41650de84639c8e21cc692a3e1605948e83b9` | Owns this routing document only. It does not activate named role processes, certify owner review, or change Asana completion. |

The supplemental workspace is `/mnt/development/projects/agent-workspaces/cherri-macro-completion-20261127`; its `ownership.json` records local component ownership. The frozen integration contains these source commits:

| Component | Component branch | Source commit |
|---|---|---|
| Typed bounded pagination selection | `agent/cherri-p10r15-complete-paginate-20261127` | `9f72b74c5af2274601331448c80d6b9101eb08bd` |
| Guarded Date cache persistence | `agent/cherri-p10r15-complete-cache-20261127` | `e4aeeaa4c6ed12d265c0b51b0cd23cff8d79a13f` |
| Typed local debug export diagnostics | `agent/cherri-p10r15-complete-export-20261127` | `efbbe7d7c09a6d4c029749c2a69eb7d8b925598e` |

## 2. Current Task Routing and Acceptance Boundaries

| Original Asana task | Coordinated responsibility | Remaining boundary |
|---|---|---|
| [P10R15 master — 1220635996500993](https://app.asana.com/0/1219603572114432/1220635996500993) | Primary MAIN_SESSION: final integration and release | Preserve exact source, binary, and evidence provenance through the final handoff. |
| [Formal 179 — 1220432417414104](https://app.asana.com/0/1219603572114432/1220432417414104) and [acceptance hardening — 1219603546838978](https://app.asana.com/0/1219603572114432/1219603546838978) | Primary MAIN_SESSION: original full acceptance gates | Reuse applicable evidence with its original scope. Focused tests, reporting-only checks, and project task counts do not discharge the original final gates. |
| [Apple/runtime — 1220546358756808](https://app.asana.com/0/1219603572114432/1220546358756808) and [native executor — 1220546287612106](https://app.asana.com/0/1219603572114432/1220546287612106) | Primary MAIN_SESSION: native FN04 and Apple acceptance | Require the original native runtime evidence; signing or callback success alone does not prove the expected output. Physical iPad requirements remain open until verified on hardware. |
| [Inspector — 1220655985907047](https://app.asana.com/0/1219603572114432/1220655985907047) | Primary MAIN_SESSION: RP39 and full94 | Preserve formal UI and causal evidence requirements. A focused RP39 receipt covers its recorded case and source. |
| [Macro master — 1220544599390563](https://app.asana.com/0/1219603572114432/1220544599390563) and [macro blocks — 1220544718901241](https://app.asana.com/0/1219603572114432/1220544718901241) | Primary MAIN_SESSION: scoped-block early-exit and final macro integration | Reconcile frozen supplemental source before editing shared macro code; retain original macro acceptance requirements. |
| [Paginate — 1220544665232021](https://app.asana.com/0/1219603572114432/1220544665232021) | Supplemental implementation frozen at the source above; primary coordinates final adoption | Typed bounded selection is a source capability. Native/provider behavior remains subject to the original task's evidence requirements. |
| [Cache — 1220544665232085](https://app.asana.com/0/1219603572114432/1220544665232085) | Supplemental implementation frozen at the source above; primary coordinates final adoption | Guarded Date persistence is a source capability. Preserve native roundtrip and final-source requirements. |
| [Debug export — 1220544665232213](https://app.asana.com/0/1219603572114432/1220544665232213) | Supplemental implementation frozen at the source above; primary coordinates final adoption | Validate native File identity and bytes, source Bool/null/List/nested shape, Apple/device behavior, and final consumer qualification against the original local-file contract. |
| [TEAM setup — 1219611682706483](https://app.asana.com/0/1219603572114432/1219611682706483) | Supplemental documentation update; logical integration responsibility remains תומר | Current owner authorization permits execution and delegation. Routing maintenance does not establish the original owner's review of PR6 or complete the separate static/runtime validation. Retain the original setup DoD. |

The Asana project snapshot observed on **2026-11-27** contained **112 project tasks: 57 completed and 55 open**. This is a dated task-management snapshot. Formal **179-case acceptance** is tracked separately in task `1220432417414104`; neither these counts nor this document changes its outcomes.

The current owner authorization recorded in [AGENTS.md](../../AGENTS.md) governs routine execution and delegation. The original release, physical-device, and evidence requirements remain in force. Follow [ASANA_POLICY.md](ASANA_POLICY.md) for truthful updates and verified completion. Do not move to P11 or declare P10 complete from reporting-only gates.

## 3. Historical Setup Routing

The following setup-era scope and logical role map are retained as history. Their counts and assignments describe the earlier setup baseline; use the current session ownership above for active work. Original base branch: `agent/cherri-p10-apple-actions-coverage`. Original setup PR: https://github.com/davidpovarsky/cherri/pull/6.

### Historical Project Scope & Technical Boundaries

- P10 Apple Actions coverage: 173 discoverable App Intents, 118 missing built-in actions.
- Compiler state reset integrity (`compiler_state.go`) and roundtrip decompilation parity.
- Signing service integration (Shortcuty Sign / HubSign) and Linux ARM64 remote builder.

---

### Historical Tasks & Logical Role Assignments

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

### Original Two-Tier Execution Policy & Approval Gates

- **מדיניות ביצוע (NORMAL מול TEAM):** ברירת המחדל היא ביצוע ישיר בסשן הראשי (`[MAIN_SESSION]`) תוך טעינת הוראות התפקיד מ-`.agents/roles/<role>.md` ו-`PROJECT_ASSIGNMENTS.md`. הפעלת סוכני משנה (`invoke_subagent` / `[SUBAGENT]`) מתבצעת אך ורק לפי בקשה מפורשת של המשתמש ("עבוד במצב צוות" / "הפעל סוכני משנה").
- **Release & P11:** Do not move to P11 or declare P10 complete based on reporting-only gates.
- **Physical Device:** Tasks with physical iPad runtime verification remain open until verified on hardware.
- **Asana Policy:** Autonomous updates and verified completion are authorized per `docs/ai-team/ASANA_POLICY.md`.
