# Suggested Asana Updates for Cherri P10R9 (Read-Only Handoff Record)

**Asana Project:** `davidpovarsky/cherri` (`1219603572114432`)  
**Policy Note:** Per explicit P10R9 task instructions, Asana was treated as read-only during this run. The following evidence-backed updates are prepared for posting under `NORMAL` mode (`[MAIN_SESSION]`).

---

## 1. Architecture & Shared Engine Contract (`[איתן | ARCH] [MAIN_SESSION]`)

```text
[איתן | ARCH -> נועם | DEV] VERIFIED
Execution Mode: MAIN_SESSION
Branch: agent/cherri-p10r9-unified-engine-hybrid-import-20261111
Subagent Invocation: NONE (MAIN_SESSION)
Deliverables:
- docs/architecture/unified-import-and-compiler-contract.md
- docs/architecture/action-emitter-ownership.json (updated with P10R9 shared components)
- docs/architecture/shared-engine-audit.md & docs/architecture/architecture-before-after.md
Summary: Consolidated forward and reverse Shortcut compilation/import onto one shared native document (DecodeNativeShortcutDocument), one deterministic reverse action registry (DefaultReverseActionRegistry, 496 standard actions / 381 Apple IDs), one shared action emission context (EvaluateActionEmission), and one shared hybrid import planner (PlanHybridImport). Preserved Pure Original (bcc0095754e2cc0baaeb4d93dae172b8153b4716), Pure+Pack (141b9cc96cf1df3f9d1593548dc49ae8a46a256a, 20 modules), and the 99-intent Sindre Sorhus Actions App boundary untouched.
```

---

## 2. Unified Engine & Hybrid Readable Import Implementation (`[נועם | DEV] [MAIN_SESSION]`)

```text
[נועם | DEV -> רוני | QA] READY_FOR_QA
Execution Mode: MAIN_SESSION
Branch: agent/cherri-p10r9-unified-engine-hybrid-import-20261111
Subagent Invocation: NONE (MAIN_SESSION)
Implementation Files:
- import_fidelity.go, import.go, reverse_action_registry.go, hybrid_import.go
- action.go, decompile.go, args.go, main.go, language_cli_adapter.go, mobile_compiler.go
- language_backend_session.go, language_backend_adapter.go
Summary: Implemented --fidelity=hybrid across Legacy and Cherri v2, fixed binary plist trailer trimming on bplist00 inputs (012), populated includeCategory in EnsureCanonicalStandardActions(), added binary plist fallback for XML-unsafe control characters (\x0b) in CanonicalBackendSession.Finalize() (HYB16), and enforced strict V2 syntax validation in semantic V2 import (HYB28).
```

---

## 3. QA, Adversarial Battery (`HYB01`–`HYB30`) & 38-Shortcut Corpus Verification (`[רוני | QA] [MAIN_SESSION]`)

```text
[רוני | QA -> תומר | INT] VERIFIED
Execution Mode: MAIN_SESSION
Branch: agent/cherri-p10r9-unified-engine-hybrid-import-20261111
Subagent Invocation: NONE (MAIN_SESSION)
Verification Evidence:
- 38/38 real-world corpus shortcuts (27,652 actions) pass lossless and hybrid import + unsigned compile in both legacy and v2 modes (1,463 NATURAL_VERIFIED actions across 28 callables; 26,188 EXACT_NATIVE fallbacks; 1 SOURCE_INVALID at 035:723).
- 30/30 adversarial tests (TestHYB01..TestHYB30) PASS.
- Dual SSoT propagation test (TestP10R9_SSoTPropagation_DeclarativeAndGoHandler) PASS.
- 1,000 sequential mixed compilations test (TestP10R9_SequentialMixedCompilesIsolation) PASS.
- Physical Apple hardware execution remains 0/38 (BLOCKED_DEVICE).
```
