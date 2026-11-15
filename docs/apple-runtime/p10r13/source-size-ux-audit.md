# Cherri P10R13 — Source Size, Memory Footprint & Editor UX Scalability Audit

This document presents the P10R13 Phase 6 quantitative audit of generated `.cherri` source size, byte attribution, compilation latency, and memory footprint across all 38 normalized real-world Shortcut specimens (`27,652` total actions).

> [!IMPORTANT]
> **Machine-Readable Companion:** See [source-size-ux-audit.json](file:///C:/Users/DAVID/Code/cherri-p10r13-simulator-debug-fidelity-20261115/docs/apple-runtime/p10r13/source-size-ux-audit.json) for per-specimen (`001`..`038`) measurements. All data is strictly anonymized; zero private user strings, URLs, or payloads appear in this report.

---

## 1. Aggregate Corpus Size & Composition Summary

| Metric | Value | Share of Generated Source | Notes |
|---|---:|---:|---|
| **Total Input Plist Bytes (`001.plist`..`038.plist`)** | `62,980,131 B` (`60.06 MiB`) | — | Includes embedded base64 media, icons, and subworkflows |
| **Total Generated Legacy `.cherri` Bytes** | `79,816,460 B` (`76.12 MiB`) | `100.0%` | `1.267x` ratio vs raw input plist bytes |
| **Total Generated V2 `.cherri` Bytes** | `79,942,515 B` (`76.24 MiB`) | `100.16%` | `1.269x` ratio vs raw input plist bytes |
| **1. Natural Cherri Statements (`23,353` actions, `84.45%`)** | `15,051,221 B` (`14.35 MiB`) | **`18.86%`** | Average **`644 B` / action** (includes large base64/JSON text literals inside natural `text(...)` / `base64Encode(...)` calls) |
| **2. Exact Fallback Calls (`4,299` actions, `15.55%`)** | `52,644,376 B` (`50.21 MiB`) | **`65.96%`** | Driven by embedded subworkflows (`runworkflow` carrying full child `WFWorkflow` plists) and nested dictionary/menu structures serialized as escaped XML plists inside `rawPlist(...)` / `native.plist(...)` |
| **3. Header Directives & Workflow Metadata** | `12,133,622 B` (`11.57 MiB`) | **`15.20%`** | Driven by lossless root workflow metadata (`WFWorkflowImportQuestions`, icon data, input content item classes) |
| **Total Planning + Verification Duration (`38` shortcuts)** | `180,040 ms` (`180.04 s`) | — | Average `4.74 s` per shortcut across 27,652 actions |
| **Total Legacy Recompilation Duration (`38` shortcuts)** | `47,251 ms` (`47.25 s`) | — | Average `1.24 s` per shortcut |
| **Total V2 Recompilation Duration (`38` shortcuts)** | `34,189 ms` (`34.19 s`) | — | Average `0.90 s` per shortcut (**`27.6%` faster than Legacy**) |
| **Peak Go Heap Alloc (`runtime.MemStats.Alloc`)** | `331,559,456 B` (`316.20 MiB`) | — | Bounded across all 38 shortcuts in a single process |
| **Peak Go Heap Sys (`runtime.MemStats.HeapSys`)** | `590,938,112 B` (`563.56 MiB`) | — | Well within desktop and iPadOS 27 memory budgets |

---

## 2. Root-Cause Analysis of Multi-Megabyte `.cherri` Source Files

Inspection of the per-specimen distribution in `source-size-ux-audit.json` reveals a strong Pareto concentration:

1. **Embedded Subworkflows (`is.workflow.actions.runworkflow` — `316` actions across the corpus):**
   - Several flagship shortcuts embed entire sub-shortcuts inside `WFWorkflow` dictionary parameters of `is.workflow.actions.runworkflow` or store large base64 asset bundles inside `gettext` / `dictionary` actions.
   - When an action containing an embedded `WFWorkflow` or large binary/base64 payload is emitted via `rawAction("...", rawPlist("..."))` / `native.action("...", parameters: native.plist("..."))`, XML plist framing + string literal escaping (`\"`, `\n`) expands the payload by ~`1.35x`.
2. **Natural Statements Are Compact (`18.86%` of total bytes for `84.45%` of all actions):**
   - For standard control flow, variables, math, text manipulation, URLs, and images, natural Cherri statements are **`5x`–`10x` smaller** than the equivalent XML plist representation.
   - Promoting `+863` additional actions to `NATURAL_VERIFIED` in P10R13 eliminated `863` inline XML plist blocks (`rawPlist` / `native.plist`), directly improving human readability and reducing editor token count.

---

## 3. Editor & Mobile UX Implications (`CodeEditorView` / iOS & iPadOS App)

When opening imported `.cherri` files in the iOS/iPadOS app (`CodeEditorView`) or CLI/desktop editors:

| File Size Tier | Specimen Count | Input Plist Range | Generated `.cherri` Range | UX & Editor Behavior |
|---|---:|---|---|---|
| **Tier 1 — Compact (`< 100 KB`)** | `21 / 38` (`55.3%`) | `2 KB – 85 KB` | `1.5 KB – 95 KB` | Instant syntax highlighting, zero-lag typing, immediate preview rendering. |
| **Tier 2 — Medium (`100 KB – 1 MB`)** | `10 / 38` (`26.3%`) | `90 KB – 800 KB` | `110 KB – 980 KB` | Sub-second load; full syntax highlighting and folding remain responsive. |
| **Tier 3 — Large (`1 MB – 20+ MB`)** | `7 / 38` (`18.4%`) | `1.1 MB – 18 MB` | `1.4 MB – 23 MB` | Dominated by inline `rawPlist(...)` / `native.plist(...)` strings and base64 assets. Requires lazy/virtualized tokenization or sidecar payload folding for smooth mobile editing. |

### Actionable UX & Architecture Recommendations for P10R14+

1. **Foldable / Sidecar-Backed `native.plistRef("...")` Option for Large Fallback Payloads:**
   - The `.cherri.import-manifest.json` sidecar already stores exact lossless parameter trees. Adding an opt-in `--compact-sidecar-refs` import mode that emits `native.action("...", parameters: native.sidecarRef("action-142"))` for fallback payloads exceeding `4 KB` would reduce Tier 3 `.cherri` source files by **`65%+`** while preserving 100% lossless compilation via the existing sidecar overlay.
2. **CodeEditorView Large-Literal Collapsing on iPadOS:**
   - Fold single-line `rawPlist("...")`, `native.plist("...")`, and base64 `text("...")` literals longer than `512` characters by default in the mobile editor view so users see clean 1-line action summaries rather than multi-page escaped XML strings.
3. **Prefer V2 Dialect for Large Imports:**
   - V2 compilation across the 38-shortcut corpus is **`27.6%` faster** than Legacy (`34.19 s` vs `47.25 s`) and avoids preprocessor regex scanning overhead on large string literals.
