#!/usr/bin/env python3
"""tools/shortcut-runtime/generate_p10_ledger.py

Exhaustive Apple Shortcuts coverage audit, 26-to-27 delta analyzer, and
machine-readable ledger generator for Cherri v2 P10.

Sourced from Cherri compiler action metadata and versioned Apple runtime evidence:
- Builtin action definitions (WorkflowKit / ActionKit)
- Apple App Intents (installed system apps)
- ToolKit SQLite registry (BackgroundShortcutRunner)
- Parameter encodings & states (sharded live runtime probes)
"""

import argparse
import datetime
import gzip
import json
import os
import pathlib
import re
import subprocess
import sys

try:
    from action_dispositions import classify_parameter_disposition
except ImportError:
    from .action_dispositions import classify_parameter_disposition


TAXONOMY_STATUSES = [
    "COVERED_VERIFIED",
    "COVERED_PARTIAL",
    "NEW_IN_27",
    "NEW_NATIVE_ACTION",
    "MISSING_PARAM",
    "MISSING_TYPE",
    "MISSING_VARIANT",
    "MISSING_APP_INTENT_DESCRIPTOR",
    "MISSING_SERIALIZER",
    "NEEDS_TARGET_PROBE",
    "DEFERRED_NO_EVIDENCE",
    "EXCLUDED_HIDDEN_INTERNAL",
    "EXCLUDED_CONFIG_ONLY",
    "EXCLUDED_NONRUNNABLE",
    "EXCLUDED_THIRD_PARTY_DEPENDENCY",
    "EXCLUDED_UNSUPPORTED_TARGET",
    "BLOCKED_EXTERNAL",
]

# Non-user implicit/UI metadata keys that are handled by the emitter or runtime
NON_USER_PARAMETER_KEYS = {
    "ShowWhenRun": "Runtime UI preference (toggle to show interface during execution); emitter handles or defaults to false",
    "IntentAppDefinition": "AppIntent transport descriptor identifying target application bundle; emitted via AppIntentDescriptor",
    "ShowHeaders": "HTTP request visual inspector state in Shortcuts UI; actual headers emitted via WFHTTPHeaders",
    "WFTakeScreenshotIgnoreContextualAssistanceLayers": "System screenshot layer exclusion flag; defaults to true",
}

# Third-party or discontinued external dependencies in historical built-ins
THIRD_PARTY_PREFIXES = [
    "is.workflow.actions.dropbox.",
    "is.workflow.actions.evernote.",
    "is.workflow.actions.trello.",
    "is.workflow.actions.wordpress.",
    "is.workflow.actions.pocket.",
    "is.workflow.actions.instapaper.",
    "is.workflow.actions.giphy.",
    "is.workflow.actions.tumblr.",
    "is.workflow.actions.cloudapp.",
    "is.workflow.actions.deskconnect.",
    "is.workflow.actions.facebook.",
]


def load_json_or_gz(path, default=None):
    if not path:
        return default
    p = pathlib.Path(path)
    if not p.exists():
        if p.with_suffix(p.suffix + ".gz").exists():
            p = p.with_suffix(p.suffix + ".gz")
        elif pathlib.Path(str(p) + ".gz").exists():
            p = pathlib.Path(str(p) + ".gz")
        else:
            return default
    try:
        if p.name.endswith(".gz"):
            with gzip.open(p, "rt", encoding="utf-8") as f:
                return json.load(f)
        raw_bytes = p.read_bytes()
        for enc in ["utf-8-sig", "utf-8", "utf-16", "latin1"]:
            try:
                return json.loads(raw_bytes.decode(enc))
            except (UnicodeDecodeError, json.JSONDecodeError):
                continue
        return default
    except Exception as e:
        print(f"Warning: Failed to load {p}: {e}", file=sys.stderr)
        return default


def extract_cherri_catalog():
    cmd = ["go", "run", ".", "--actions-json", "--no-ansi"]
    try:
        res = subprocess.run(cmd, capture_output=True, text=True, check=True)
        data = json.loads(res.stdout)
        if not data or not data.get("actions"):
            print("Error: cherri --actions-json returned empty or invalid actions catalog", file=sys.stderr)
            sys.exit(1)
        return data
    except Exception as e:
        print(f"Error: Failed to run cherri --actions-json: {e}", file=sys.stderr)
        sys.exit(1)


def compute_delta_26_to_27(snap26, snap27):
    """Computes exact semantic differences between macOS 26 and macOS 27 runtime snapshots."""
    actions26 = snap26.get("actions", {}) if snap26 else {}
    actions27 = snap27.get("actions", {}) if snap27 else {}

    ids26 = set(actions26.keys())
    ids27 = set(actions27.keys())

    added_ids = sorted(ids27 - ids26)
    removed_ids = sorted(ids26 - ids27)
    common_ids = sorted(ids26 & ids27)

    modified_actions = {}
    for ident in common_ids:
        a26 = actions26[ident]
        a27 = actions27[ident]

        # Parameter diff
        p26 = {p["key"]: p for p in a26.get("parameters", []) if "key" in p}
        p27 = {p["key"]: p for p in a27.get("parameters", []) if "key" in p}

        new_params = sorted(set(p27.keys()) - set(p26.keys()))
        removed_params = sorted(set(p26.keys()) - set(p27.keys()))
        modified_params = {}
        for k in set(p26.keys()) & set(p27.keys()):
            diffs = {}
            if p26[k].get("class") != p27[k].get("class"):
                diffs["class"] = {"old": p26[k].get("class"), "new": p27[k].get("class")}
            if p26[k].get("singleStateClass") != p27[k].get("singleStateClass"):
                diffs["singleStateClass"] = {"old": p26[k].get("singleStateClass"), "new": p27[k].get("singleStateClass")}
            if diffs:
                modified_params[k] = diffs

        # Sources diff
        s26 = set(a26.get("sources", []))
        s27 = set(a27.get("sources", []))
        source_diff = {"added": sorted(s27 - s26), "removed": sorted(s26 - s27)} if s26 != s27 else None

        if new_params or removed_params or modified_params or source_diff:
            modified_actions[ident] = {
                "newParameters": new_params,
                "removedParameters": removed_params,
                "modifiedParameters": modified_params,
                "sourceChanges": source_diff,
            }

    delta = {
        "summary": {
            "total26": len(ids26),
            "total27": len(ids27),
            "addedActionsCount": len(added_ids),
            "removedActionsCount": len(removed_ids),
            "modifiedActionsCount": len(modified_actions),
        },
        "addedActions": added_ids,
        "removedActions": removed_ids,
        "modifiedActions": modified_actions,
    }
    return delta


def build_ledger(cherri_catalog, snap26, snap27, gaps26, gaps27, missing_builtins, missing_intents, missing_toolkit):
    """Builds the comprehensive per-record ledger across all inventories."""
    cherri_actions = cherri_catalog.get("actions", [])
    cherri_map = {}
    for a in cherri_actions:
        ident = a.get("shortcutIdentifier")
        if ident:
            if ident not in cherri_map:
                cherri_map[ident] = []
            cherri_map[ident].append(a)

    actions27 = snap27.get("actions", {}) if snap27 else {}
    actions26 = snap26.get("actions", {}) if snap26 else {}

    # Combine all known actions
    all_known_ids = set()
    all_known_ids.update(cherri_map.keys())
    all_known_ids.update(actions27.keys())
    all_known_ids.update(actions26.keys())
    if missing_builtins:
        all_known_ids.update(missing_builtins)
    if missing_intents:
        all_known_ids.update(missing_intents)
    if missing_toolkit:
        all_known_ids.update(missing_toolkit)

    ledger_rows = []

    for ident in sorted(all_known_ids):
        is_new_in_27 = bool(snap26 and ident in actions27 and ident not in actions26)
        is_in_26 = bool(snap26 and ident in actions26)
        introduced_in = "macOS 27" if is_new_in_27 else ("macOS 26" if is_in_26 else "unknown")

        # Resolve Apple metadata
        apple_meta = actions27.get(ident) or actions26.get(ident) or {}
        meta_dict = apple_meta.get("metadata", {})
        builtin_meta = meta_dict.get("builtin", {})
        tk_meta = meta_dict.get("toolkit", {})

        os_avail = "unknown"
        if apple_meta.get("macOnly") or meta_dict.get("macOnly") or ident.startswith("is.workflow.actions.filter.windows"):
            os_avail = "macOS-only"
        elif "platforms" in apple_meta and apple_meta["platforms"]:
            os_avail = ",".join(sorted(apple_meta["platforms"]))
        elif "unsupportedEnvironments" in builtin_meta:
            os_avail = "macOS" if "Memory" in str(builtin_meta["unsupportedEnvironments"]) else "unknown"

        row = {
            "identifier": ident,
            "title": "",
            "category": "unknown",
            "sources": [],
            "cherriCallables": [],
            "status": "OPEN_IMPLEMENTATION",
            "eligibility": "UNKNOWN",
            "introducedIn": introduced_in,
            "isNewIn27": is_new_in_27,
            "osAvailability": os_avail,
            "unmodeledKeys": [],
            "userSuppliedGaps": [],
            "nonUserKeys": [],
            "parameterDispositions": [],
            "outputType": "Unknown",
            "cherriOutputType": "None",
            "outputTypeConcordance": "UNASSESSED",
            "rationale": "",
        }

        row["sources"] = apple_meta.get("sources", [])
        if "builtinDefinition" in row["sources"]:
            row["category"] = "builtin"
        elif "appIntent" in row["sources"] or "WFLinkActionProvider" in str(tk_meta):
            row["category"] = "app_intent"
        elif "toolkit" in row["sources"]:
            row["category"] = "toolkit"
        elif ident.startswith("is.workflow.actions."):
            row["category"] = "builtin"
        else:
            row["category"] = "cherri_custom"

        # Determine title
        row["title"] = (
            builtin_meta.get("Name")
            or tk_meta.get("name")
            or (cherri_map[ident][0].get("title") if ident in cherri_map else "")
            or ident.split(".")[-1]
        )

        # Output type
        row["outputType"] = (
            builtin_meta.get("Output", {}).get("Type")
            or tk_meta.get("outputType")
            or "Unknown"
        )

        # Check Cherri coverage
        if ident in cherri_map:
            variants = cherri_map[ident]
            row["cherriCallables"] = [v.get("name") for v in variants]
            row["cherriOutputType"] = variants[0].get("outputType", "Unknown")

            # Check output type concordance
            if row["outputType"] != "Unknown" and row["cherriOutputType"] not in ("Unknown", "None", ""):
                if row["outputType"].lower() in row["cherriOutputType"].lower() or row["cherriOutputType"].lower() in row["outputType"].lower() or row["cherriOutputType"] == "variable":
                    row["outputTypeConcordance"] = "MATCH"
                else:
                    row["outputTypeConcordance"] = "DIFFERS"

            # Check parameter gaps via auditable dispositions
            gaps_info = next((g for g in (gaps27 or gaps26 or []) if g.get("identifier") == ident), None)
            if gaps_info and gaps_info.get("unmodeledParameterKeys"):
                unmodeled = gaps_info["unmodeledParameterKeys"]
                row["unmodeledKeys"] = unmodeled

                cherri_declared = set()
                cherri_emitted = set()
                cherri_static = set()
                for v in variants:
                    for p in v.get("parameters", []):
                        if p.get("key"): cherri_declared.add(p["key"])
                    for ek in v.get("emittedKeys", []): cherri_emitted.add(ek)
                    if isinstance(v.get("staticParameters"), dict):
                        for sk in v["staticParameters"].keys(): cherri_static.add(sk)

                is_filter = ident.startswith("is.workflow.actions.filter.") or any("filter" in (v.get("name") or "").lower() for v in variants)
                disps = []
                user_gaps = []
                non_user = []
                for k in unmodeled:
                    d = classify_parameter_disposition(ident, k, cherri_declared, cherri_emitted, cherri_static, is_filter)
                    disps.append(d)
                    if d["disposition"] == "OPEN_USER_PARAMETER":
                        user_gaps.append(k)
                    else:
                        non_user.append(k)

                row["parameterDispositions"] = disps
                row["userSuppliedGaps"] = user_gaps
                row["nonUserKeys"] = non_user

                has_needs_evidence = any(d.get("disposition") == "NEEDS_RUNTIME_EVIDENCE" for d in disps)
                if user_gaps:
                    row["status"] = "MISSING_PARAM"
                    row["rationale"] = f"Action has {len(user_gaps)} unmodeled user parameters: {', '.join(user_gaps)}"
                elif has_needs_evidence:
                    row["status"] = "COVERED_PARTIAL"
                    row["rationale"] = f"Action has parameter(s) requiring runtime evidence: {[d['parameterKey'] for d in disps if d.get('disposition') == 'NEEDS_RUNTIME_EVIDENCE']}"
                elif row["outputTypeConcordance"] == "DIFFERS":
                    row["status"] = "COVERED_PARTIAL"
                    row["rationale"] = f"Parameter mapped but output type differs (Apple: {row['outputType']}, Cherri: {row['cherriOutputType']})"
                else:
                    row["status"] = "COVERED_VERIFIED"
                    row["rationale"] = f"All {len(non_user)} unmodeled keys have verified non-user or builder dispositions ({', '.join(non_user)})"
            else:
                if row["outputTypeConcordance"] == "DIFFERS":
                    row["status"] = "COVERED_PARTIAL"
                    row["rationale"] = f"Action mapped but output type differs (Apple: {row['outputType']}, Cherri: {row['cherriOutputType']})"
                else:
                    row["status"] = "COVERED_VERIFIED"
                    row["rationale"] = "Fully covered in Cherri with all declared parameters mapped"

            row["eligibility"] = "ELIGIBLE_USER_FACING"
        else:
            # Not in Cherri: classify missing item
            # 1. Third-party dependency
            if any(ident.startswith(pfx) for pfx in THIRD_PARTY_PREFIXES):
                row["status"] = "EXCLUDED_THIRD_PARTY_DEPENDENCY"
                row["eligibility"] = "EXCLUDED_THIRD_PARTY"
                row["rationale"] = "Requires discontinued or third-party service integration"
            # 2. Hidden or internal
            elif tk_meta.get("visibilityFlags") == 0 or tk_meta.get("classification") == "apple_link_hidden":
                row["status"] = "EXCLUDED_HIDDEN_INTERNAL"
                row["eligibility"] = "EXCLUDED_INTERNAL"
                row["rationale"] = "Marked as internal/hidden in Apple ToolKit registry (visibilityFlags=0)"
            # 3. Configuration only
            elif tk_meta.get("classification") == "apple_configuration_only":
                row["status"] = "EXCLUDED_CONFIG_ONLY"
                row["eligibility"] = "EXCLUDED_CONFIG_ONLY"
                row["rationale"] = "Control Center / Lock Screen widget configuration intent, not a runnable shortcut action"
            # 4. Native built-in missing
            elif row["category"] == "builtin" and ("builtinDefinition" in row["sources"] or (missing_builtins and ident in missing_builtins)):
                row["status"] = "NEW_NATIVE_ACTION"
                row["eligibility"] = "ELIGIBLE_USER_FACING"
                row["rationale"] = "Apple Shortcuts native built-in action missing from Cherri" + (" (introduced in macOS 27)" if is_new_in_27 else "")
            # 5. Discoverable App Intent or ToolKit runnable
            elif row["category"] == "app_intent" or tk_meta.get("classification") == "apple_link_runnable":
                if apple_meta.get("metadata", {}).get("discoverable", False) or tk_meta.get("classification") == "apple_link_runnable":
                    row["status"] = "MISSING_APP_INTENT_DESCRIPTOR"
                    row["eligibility"] = "ELIGIBLE_APP_INTENT"
                    row["rationale"] = "First-party Apple App Intent runnable in Shortcuts" + (" (introduced in macOS 27)" if is_new_in_27 else "")
                else:
                    row["status"] = "EXCLUDED_NONRUNNABLE"
                    row["eligibility"] = "EXCLUDED_NONRUNNABLE"
                    row["rationale"] = "Non-discoverable / internal App Intent"
            else:
                row["status"] = "DEFERRED_NO_EVIDENCE"
                row["eligibility"] = "DEFERRED"
                row["rationale"] = "Insufficient runtime execution evidence to model reliably"

        ledger_rows.append(row)

    return ledger_rows


def generate_coverage_report(ledger, delta_info, snap27_manifest, out_path):
    """Generates human-readable Markdown coverage audit report."""
    total_records = len(ledger)
    by_status = {}
    by_eligibility = {}
    by_category = {}

    for r in ledger:
        st = r["status"]
        el = r["eligibility"]
        cat = r["category"]
        by_status[st] = by_status.get(st, 0) + 1
        by_eligibility[el] = by_eligibility.get(el, 0) + 1
        by_category[cat] = by_category.get(cat, 0) + 1

    eligible_total = sum(v for k, v in by_eligibility.items() if k.startswith("ELIGIBLE_"))
    covered_verified = by_status.get("COVERED_VERIFIED", 0)
    covered_partial = by_status.get("COVERED_PARTIAL", 0)
    covered_total = covered_verified + covered_partial
    eligible_coverage_pct = (covered_total / eligible_total * 100.0) if eligible_total > 0 else 0.0

    lines = []
    lines.append("# Cherri v2 P10 - Complete Apple Shortcuts Action Coverage Audit")
    lines.append("")
    lines.append(f"> **Generated:** {datetime.datetime.now(datetime.timezone.utc).isoformat()} | **Host OS:** macOS {snap27_manifest.get('macOS', '27.x')} ({snap27_manifest.get('build', 'unknown')})")
    lines.append("")
    lines.append("## 1. Executive Coverage Summary")
    lines.append("")
    lines.append("| Metric | Count | Percentage | Notes |")
    lines.append("| --- | --- | --- | --- |")
    lines.append(f"| **Total Raw Discovered Records** | {total_records} | 100% | Union of builtins, App Intents, and ToolKit |")
    lines.append(f"| **Eligible User-Facing Actions** | {eligible_total} | {eligible_total/total_records*100:.1f}% | Excludes internal, config-only, non-runnable, third-party |")
    lines.append(f"| **Covered & Verified** | {covered_verified} | {covered_verified/eligible_total*100:.1f}% of eligible | Mapped in Cherri with verified parameter contracts |")
    lines.append(f"| **Missing Parameter Repairs** | {by_status.get('MISSING_PARAM', 0)} | {by_status.get('MISSING_PARAM', 0)/eligible_total*100:.1f}% | Covered actions needing parameter additions |")
    new_in_27_count = sum(1 for r in ledger if r.get("isNewIn27"))
    lines.append(f"| **Missing Built-in Actions** | {by_status.get('NEW_NATIVE_ACTION', 0)} | {by_status.get('NEW_NATIVE_ACTION', 0)/eligible_total*100:.1f}% | Native built-ins needing Cherri definitions |")
    lines.append(f"| **New in macOS 27 (Version Attribute)** | {new_in_27_count} | {new_in_27_count/total_records*100:.1f}% of total | Introduced in macOS 27 runtime across all categories |")
    lines.append(f"| **Missing App Intents** | {by_status.get('MISSING_APP_INTENT_DESCRIPTOR', 0)} | {by_status.get('MISSING_APP_INTENT_DESCRIPTOR', 0)/eligible_total*100:.1f}% | Apple first-party App Intent Link actions |")
    lines.append(f"| **Excluded / Internal / Config** | {total_records - eligible_total} | {(total_records - eligible_total)/total_records*100:.1f}% | Correctly excluded from denominator |")
    lines.append("")
    lines.append("## 2. Taxonomy Status Breakdown")
    lines.append("")
    lines.append("| Status | Count | Description |")
    lines.append("| --- | --- | --- |")
    for st in TAXONOMY_STATUSES:
        cnt = by_status.get(st, 0)
        lines.append(f"| `{st}` | {cnt} | |")

    lines.append("")
    lines.append("## 3. macOS 26 vs macOS 27 Delta")
    lines.append("")
    if delta_info:
        s = delta_info.get("summary", {})
        lines.append(f"- **Total Actions (macOS 26):** {s.get('total26', 0)}")
        lines.append(f"- **Total Actions (macOS 27):** {s.get('total27', 0)}")
        lines.append(f"- **Newly Discovered Actions in 27:** {s.get('addedActionsCount', 0)}")
        lines.append(f"- **Removed/Deprecated Actions:** {s.get('removedActionsCount', 0)}")
        lines.append(f"- **Modified Parameter/State Actions:** {s.get('modifiedActionsCount', 0)}")
        if delta_info.get("addedActions"):
            lines.append("\n### Newly Added Action Identifiers in macOS 27:")
            for a in delta_info["addedActions"]:
                lines.append(f"- `{a}`")
    else:
        lines.append("macOS 27 live snapshot pending or identical.")

    lines.append("")
    lines.append("## 4. Parameter Repair Inventory (High-Impact Families)")
    lines.append("")
    lines.append("| Identifier | Cherri Name | Missing Keys | Non-User Evaluated |")
    lines.append("| --- | --- | --- | --- |")
    for r in ledger:
        if r.get("unmodeledKeys"):
            user_keys = ", ".join(f"`{k}`" for k in r["userSuppliedGaps"]) if r["userSuppliedGaps"] else "*None (all metadata)*"
            non_user_keys = ", ".join(f"`{k}`" for k in r["nonUserKeys"]) if r["nonUserKeys"] else "*None*"
            callables = ", ".join(r["cherriCallables"]) or r["title"]
            lines.append(f"| `{r['identifier']}` | `{callables}` | {user_keys} | {non_user_keys} |")

    pathlib.Path(out_path).write_text("\n".join(lines) + "\n", encoding="utf-8")
    print(f"Generated coverage report at {out_path} ({len(ledger)} actions classified).")


def main():
    parser = argparse.ArgumentParser(description="P10 Apple Shortcuts coverage audit and ledger generator")
    parser.add_argument("--macos-26-dir", default="docs/apple-runtime/macos-26")
    parser.add_argument("--macos-27-dir", default="docs/apple-runtime/macos-27")
    parser.add_argument("--cherri-actions", help="Path to cherri-actions.json")
    parser.add_argument("--out-dir", default="docs/apple-runtime/p10")
    args = parser.parse_args()

    out_dir = pathlib.Path(args.out_dir)
    out_dir.mkdir(parents=True, exist_ok=True)

    # 1. Load Cherri Catalog
    if args.cherri_actions:
        p_act = pathlib.Path(args.cherri_actions)
        if not p_act.exists():
            print(f"Error: Specified cherri actions catalog does not exist: {p_act}", file=sys.stderr)
            sys.exit(1)
        cherri_catalog = load_json_or_gz(p_act)
        if not cherri_catalog or not cherri_catalog.get("actions"):
            print(f"Error: Invalid or corrupt cherri actions catalog in {p_act}", file=sys.stderr)
            sys.exit(1)
    else:
        print("Extracting live Cherri catalog via 'cherri --actions-json'...")
        cherri_catalog = extract_cherri_catalog()

    # 2. Load macOS 26 evidence
    d26 = pathlib.Path(args.macos_26_dir)
    snap26 = load_json_or_gz(d26 / "apple-runtime-snapshot.json")
    gaps26 = load_json_or_gz(d26 / "parameter-surface-gaps.json", [])
    missing_builtins_26 = load_json_or_gz(d26 / "missing-builtins.json", [])
    missing_intents_26 = load_json_or_gz(d26 / "missing-apple-app-intents.json", [])
    missing_toolkit_26 = load_json_or_gz(d26 / "missing-toolkit-apple-actions.json", [])

    # 3. Load macOS 27 evidence (fail-closed, no silent fallback to 26)
    d27 = pathlib.Path(args.macos_27_dir)
    if not d27.exists():
        raise FileNotFoundError(f"macOS 27 evidence directory does not exist: {d27}")
    snap27_file = d27 / "apple-runtime-snapshot.json"
    if not (snap27_file.exists() or snap27_file.with_suffix(".json.gz").exists()):
        raise FileNotFoundError(f"macOS 27 snapshot file not found in {d27}")
    snap27 = load_json_or_gz(snap27_file)
    if not snap27:
        raise ValueError(f"Failed to load macOS 27 snapshot from {snap27_file}")

    manifest27 = load_json_or_gz(d27 / "apple-runtime-manifest.json", {})
    if manifest27:
        host_ver = str(manifest27.get("host", {}).get("osVersion") or manifest27.get("macOS") or manifest27.get("osVersion") or "")
        if "26" in host_ver and "27" not in host_ver:
            raise ValueError(f"macOS 26 manifest cannot be claimed as macOS 27: {host_ver}")

    gaps27 = load_json_or_gz(d27 / "parameter-surface-gaps.json", [])
    missing_builtins_27 = load_json_or_gz(d27 / "missing-builtins.json", [])
    missing_intents_27 = load_json_or_gz(d27 / "missing-apple-app-intents.json", [])
    missing_toolkit_27 = load_json_or_gz(d27 / "missing-toolkit-apple-actions.json", [])

    # 4. Compute 26 -> 27 delta
    delta = compute_delta_26_to_27(snap26, snap27)
    (out_dir / "macos-26-to-27-delta.json").write_text(json.dumps(delta, indent=2, sort_keys=True), encoding="utf-8")

    # 5. Build full classification ledger
    ledger = build_ledger(
        cherri_catalog=cherri_catalog,
        snap26=snap26,
        snap27=snap27,
        gaps26=gaps26,
        gaps27=gaps27,
        missing_builtins=missing_builtins_27,
        missing_intents=missing_intents_27,
        missing_toolkit=missing_toolkit_27,
    )
    (out_dir / "p10-action-ledger.json").write_text(json.dumps(ledger, indent=2, sort_keys=True), encoding="utf-8")

    # 6. Generate human-readable coverage report
    generate_coverage_report(ledger, delta, manifest27, out_dir / "p10-coverage-report.md")


if __name__ == "__main__":
    main()
