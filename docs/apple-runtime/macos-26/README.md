# Apple Shortcuts Runtime Extraction Evidence (Schema 2.0.0)

This directory contains durable, sanitized evidence extracted directly from the Apple Shortcuts runtime on macOS and iOS Simulator runners.

It forms the canonical foundation for Cherri's action definitions, serialization models, catalog generation, parameter encodings, and runtime conformance testing.

---

## 1. Extraction Environment & Provenance

| Property | Value |
| --- | --- |
| **Schema Version** | `2.0.0` |
| **Extraction Date** | 2026-10-05T23:28:06Z |
| **Host OS** | macOS 15.x / Darwin 26.6.2 (`build 25G83`) |
| **Shortcuts Version** | 7.0 (`build 4711`) |
| **WorkflowKit Version** | 7.0 |
| **ActionKit Version** | 1.0 |
| **ToolKit Database** | `Tools-prod.v67-17A37C68-36FE-47B4-B3D8-935CEE4548D5.sqlite` (Schema v67) |
| **ToolKit Version** | `77106E4A-25AC-4B5A-9A42-58BC95B1A856` |
| **Reference GitHub Actions Run** | [`37388046421`](https://github.com/davidpovarsky/cherri/actions/runs/37388046421) |
| **Verified Commit SHA** | `7868d2c751f72e6b62ae44e9f7410e5e470d16f4` |

---

## 2. Summary of Extracted Populations

### 2.1 Built-in Actions (`WFAction` / `WorkflowKit`)
- **Total Discovered:** 429
  - Visible: 420
  - Hidden: 9
  - Matched in dyld shared cache: 402
- **Parameter Probing:**
  - 417 actions successfully probed across 4 isolated shards
  - 3 bounded timeouts (known system dialog / camera interactive actions)
  - 9 unavailable (macOS framework differences)
  - **0 unexpected errors**
- **Observed Surface:**
  - 1,187 parameters across 97 distinct parameter classes
  - 51 parameter state classes validated via serialized round-trip

### 2.2 ToolKit Registry (`BackgroundShortcutRunner` / App Intents)
- **Total Tools:** 1,793
- **Providers:**
  - `WFLinkActionProvider`: 1,324
  - `WFBundledActionProvider`: 362
  - `WFInterchangeActionProvider`: 56
  - `WFIntentActionProvider`: 51
- **Entities & Tables Extracted:**
  - Parameters: 5,597
  - Parameter types: 5,805
  - Output types: 1,914
  - Enumeration cases: 7,556
  - Containers: 129 (84 Apple system containers, 45 runner/third-party)
- **Semantic Population Breakdown:**
  - `apple_link_runnable`: 701 (Apple App Intent / Link actions runnable as Shortcuts)
  - `apple_synthesized`: 481 (Protocol/synthesized actions, e.g. open entity / search)
  - `apple_link_hidden`: 113 (Internal / non-exposed link actions)
  - `builtin_visible`: 342 (Builtin actions mirrored in ToolKit)
  - `builtin_hidden`: 20
  - `interchange_action`: 56 (Legacy / interchange actions)
  - `apple_intent`: 51 (SiriKit / Intent actions)
  - `apple_configuration_only`: 29 (Configuration-only parameters)

---

## 3. Comparison with Cherri Catalog

At the time of this extraction baseline:
- **Cherri Total Actions:** 461 action definitions across 349 unique Shortcut identifiers.
- **Built-in Action Coverage:**
  - Covered: 280 / 429 (**65.27%**)
  - Missing: 149
- **Apple Link Runnable Coverage:**
  - Covered: 33 / 701 (**4.71%**)
  - Missing: 668
- **Apple App Intents Coverage:**
  - Covered: 14 / 51

Separating `apple_link_runnable` (701) from synthesized/hidden/configuration entities prevents inflating missing action counts by 1,000+ non-runnable records.

---

## 4. File Manifest

### 4.1 Uncompressed Metadata & Reports
These files are small, uncompressed, and directly readable / diffable in git:

| File | Size | Description |
| --- | --- | --- |
| `apple-runtime-manifest.json` | ~700 B | Top-level counts, environment versions, and extraction hashes. |
| `comparison-summary.json` | ~1.3 KB | Cherri vs. Apple runtime coverage summary. |
| `parameter-encodings-summary.json` | ~1.4 KB | Summary of 4-shard parameter probe execution. |
| `parameter-timeouts.json` | ~500 B | The 3 bounded-timeout actions during probing. |
| `parameter-errors.json` | 2 B | Empty array (`[]`) verifying 0 probe crashes. |
| `parameter-surface-gaps.json` | ~24 KB | Parameters observed in runtime not yet mapped in Cherri. |
| `encoding-table.json` | ~29 KB | Mappings between parameter classes, serialization keys, and types. |
| `encoding-roundtrips.json` | ~19 KB | Roundtrip validation results for parameter states. |
| `toolkit-summary.json` | ~1.3 KB | ToolKit DB schema, counts, providers, and classification stats. |
| `missing-builtins.json` | ~7.4 KB | List of 149 missing built-in action identifiers. |
| `missing-apple-app-intents.json` | ~12 KB | Missing App Intent actions. |
| `missing-toolkit-apple-actions.json` | ~33 KB | List of 668 missing runnable Apple Link tools. |

### 4.2 Compressed Complete Registries (`.json.gz`)
To prevent multi-megabyte JSON blobs from bloating the git repository pack while preserving 100% raw metadata, full registries are stored gzipped (yielding a ~95% size reduction):

| File | Compressed Size | Uncompressed Size | Description |
| --- | --- | --- | --- |
| `apple-runtime-snapshot.json.gz` | ~178 KB | 4.26 MB | Complete unified snapshot (Schema 2.0.0) combining builtins, parameter encodings, App Intents, and ToolKit. |
| `toolkit-registry.json.gz` | ~248 KB | 4.93 MB | Complete extracted ToolKit DB registry with all tools, parameters, types, outputs, and enums. |
| `toolkit-apple-actions.json.gz` | ~88 KB | 1.60 MB | Normalized slice of Apple Link runnable actions and parameters. |
| `toolkit-names.json.gz` | ~104 KB | 922 KB | Human-readable English action titles and synonyms from ToolKit. |
| `parameter-encodings.json.gz` | ~27 KB | 480 KB | Complete per-action parameter probe records and serialization states. |

---

## 5. How to Inspect and Use Evidence

### Shell / `jq`
```bash
# Inspect total tools in ToolKit registry
gzip -dc docs/apple-runtime/toolkit-registry.json.gz | jq '.tools | length'

# Inspect top-level snapshot manifest
gzip -dc docs/apple-runtime/apple-runtime-snapshot.json.gz | jq '.manifest'

# Find an action by identifier
gzip -dc docs/apple-runtime/toolkit-apple-actions.json.gz | jq '.[] | select(.identifier == "is.workflow.actions.openurl")'
```

### Python
```python
import gzip
import json

with gzip.open("docs/apple-runtime/toolkit-registry.json.gz", "rt", encoding="utf-8") as f:
    registry = json.load(f)
    print(f"Loaded {len(registry['tools'])} tools from ToolKit registry")
```

---

## 6. Maintenance & Reproduction

To reproduce or update this extraction:
1. Trigger `.github/workflows/macos-shortcuts-deep-extract.yml` via GitHub Actions.
2. The workflow runs the 4-shard probe, App Intent dump, ToolKit extractor (`tools/shortcut-runtime/extract_toolkit.py`), and normalizer (`tools/shortcut-runtime/normalize_snapshot.py`).
3. Download artifacts and replace files in this directory.
