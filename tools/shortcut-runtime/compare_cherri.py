#!/usr/bin/env python3
"""tools/shortcut-runtime/compare_cherri.py

Compares extracted Apple runtime surfaces (built-ins, dyld identifiers, App Intents,
ToolKit registry, parameter surface) against the current Cherri catalog.

Fixes previous schema bug by properly parsing app-provided-actions.json and reading
shortcutActionIdentifier, separating discoverable vs total App Intents.
"""

import argparse
import json
import os
import pathlib
import sys


NON_USER_PARAMETER_KEYS = {
    "ShowWhenRun",
    "IntentAppDefinition",
    "ShowHeaders",
    "WFTakeScreenshotIgnoreContextualAssistanceLayers",
    "WFCompoundType",
    "WFContentItemFilter",
    "WFContentItemLimitEnabled",
    "WFContentItemInputParameter",
    "WFGetUpcomingItemCalendar",
}


def load_json(path, default=None):
    if path is None:
        return default
    if isinstance(path, (dict, list)):
        return path
    actual_path = path
    if not os.path.exists(actual_path):
        if os.path.exists(str(actual_path) + ".gz"):
            actual_path = str(actual_path) + ".gz"
        else:
            return default
    try:
        if str(actual_path).endswith(".gz"):
            import gzip
            with gzip.open(actual_path, "rt", encoding="utf-8") as f:
                return json.load(f)
        return json.loads(pathlib.Path(actual_path).read_text(encoding="utf-8-sig"))
    except Exception as e:
        print(f"Warning: Failed to load {path}: {e}", file=sys.stderr)
        return default


def load_lines(path):
    if path is None:
        return []
    if isinstance(path, (list, set)):
        return list(path)
    actual_path = path
    if not os.path.exists(actual_path):
        if os.path.exists(str(actual_path) + ".gz"):
            actual_path = str(actual_path) + ".gz"
        else:
            return []
    try:
        if str(actual_path).endswith(".gz"):
            import gzip
            with gzip.open(actual_path, "rt", encoding="utf-8") as f:
                return [line.strip() for line in f.read().splitlines() if line.strip()]
        return [line.strip() for line in pathlib.Path(actual_path).read_text(encoding="utf-8").splitlines() if line.strip()]
    except Exception as e:
        print(f"Warning: Failed to load lines from {path}: {e}", file=sys.stderr)
        return []


def run_comparison(cherri_actions_path, builtin_defs_path=None, dyld_ids_path=None,
                   app_intents_path=None, toolkit_registry_path=None, toolkit_summary_path=None,
                   parameter_encodings_path=None):
    cherri_data = load_json(cherri_actions_path, {})
    cherri_actions = cherri_data.get("actions", [])
    cherri_map = {}
    for a in cherri_actions:
        ident = a.get("shortcutIdentifier")
        if ident:
            if ident not in cherri_map:
                cherri_map[ident] = []
            cherri_map[ident].append(a)

    cherri_ids = set(cherri_map.keys())

    # 1. Built-in action definitions
    builtin_defs = load_json(builtin_defs_path, {})
    builtin_ids = set(builtin_defs.keys()) if isinstance(builtin_defs, dict) else set()
    builtin_covered = builtin_ids & cherri_ids
    builtin_missing = builtin_ids - cherri_ids
    builtin_pct = (len(builtin_covered) / len(builtin_ids) * 100.0) if builtin_ids else 0.0

    # 2. dyld shared-cache identifiers
    dyld_ids = set(load_lines(dyld_ids_path))
    dyld_covered = dyld_ids & cherri_ids
    dyld_missing = dyld_ids - cherri_ids

    # 3. Apple App Intents from app-provided-actions.json
    app_actions_raw = load_json(app_intents_path, [])
    if isinstance(app_actions_raw, dict):
        app_actions_list = app_actions_raw.get("actions", [])
        if isinstance(app_actions_list, dict):
            app_actions_list = list(app_actions_list.values())
    elif isinstance(app_actions_raw, list):
        app_actions_list = app_actions_raw
    else:
        app_actions_list = []

    apple_app_actions = []
    for item in app_actions_list:
        if not isinstance(item, dict):
            continue
        bundle_id = item.get("bundleIdentifier") or ""
        # Filter to Apple bundles (com.apple.*) or items marked apple
        if bundle_id.startswith("com.apple.") or item.get("appIntentDescriptor", {}).get("BundleIdentifier", "").startswith("com.apple."):
            apple_app_actions.append(item)

    apple_intents_total_ids = {x.get("shortcutActionIdentifier") for x in apple_app_actions if x.get("shortcutActionIdentifier")}
    apple_intents_covered = apple_intents_total_ids & cherri_ids
    apple_intents_missing = apple_intents_total_ids - cherri_ids

    discoverable_actions = [x for x in apple_app_actions if x.get("discoverable") is True]
    discoverable_ids = {x.get("shortcutActionIdentifier") for x in discoverable_actions if x.get("shortcutActionIdentifier")}
    discoverable_covered = discoverable_ids & cherri_ids
    discoverable_missing = discoverable_ids - cherri_ids

    # 4. ToolKit Evidence
    toolkit_summary = load_json(toolkit_summary_path, {})
    toolkit_registry = load_json(toolkit_registry_path, {})
    toolkit_tools = toolkit_registry.get("tools", [])
    apple_runnable_tools = [t for t in toolkit_tools if t.get("classification") == "apple_link_runnable"]
    apple_runnable_ids = {t["identifier"] for t in apple_runnable_tools if t.get("identifier")}
    apple_runnable_covered = apple_runnable_ids & cherri_ids
    apple_runnable_missing = apple_runnable_ids - cherri_ids

    # 5. Parameter surface gaps
    param_encodings = load_json(parameter_encodings_path, {})
    param_encodings_actions = param_encodings.get("actionParameters", {})
    parameter_gaps = []

    for ident in sorted(builtin_covered):
        apple_params = param_encodings_actions.get(ident, [])
        cherri_variants = cherri_map.get(ident, [])

        cherri_param_keys = set()
        cherri_names = []
        for variant in cherri_variants:
            if variant.get("name"):
                cherri_names.append(variant["name"])
            for p in variant.get("parameters", []):
                if p.get("key"):
                    cherri_param_keys.add(p["key"])
            for ek in variant.get("emittedKeys", []):
                cherri_param_keys.add(ek)
            if isinstance(variant.get("staticParameters"), dict):
                for sk in variant["staticParameters"].keys():
                    cherri_param_keys.add(sk)

        apple_keys = {p.get("key") for p in apple_params if p.get("key")}
        unmodeled_keys = apple_keys - cherri_param_keys - NON_USER_PARAMETER_KEYS

        if unmodeled_keys:
            parameter_gaps.append({
                "identifier": ident,
                "cherriName": ", ".join(cherri_names) if cherri_names else "",
                "unmodeledParameterKeys": sorted(unmodeled_keys),
                "appleKeyCount": len(apple_keys),
                "cherriKeyCount": len(cherri_param_keys)
            })

    summary = {
        "cherriActions": len(cherri_actions),
        "cherriUniqueIdentifiers": len(cherri_ids),
        "builtins": {
            "totalDefinitions": len(builtin_ids),
            "covered": len(builtin_covered),
            "missing": len(builtin_missing),
            "coveragePercentage": round(builtin_pct, 2)
        },
        "dyld": {
            "totalIdentifiers": len(dyld_ids),
            "covered": len(dyld_covered),
            "missing": len(dyld_missing)
        },
        "appleAppIntents": {
            "totalExtracted": len(apple_intents_total_ids),
            "totalCovered": len(apple_intents_covered),
            "totalMissing": len(apple_intents_missing),
            "discoverableTotal": len(discoverable_ids),
            "discoverableCovered": len(discoverable_covered),
            "discoverableMissing": len(discoverable_missing)
        },
        "toolkit": {
            "totalTools": toolkit_summary.get("totalTools", toolkit_summary.get("Tools", 0)),
            "visible": toolkit_summary.get("visible", 0),
            "hidden": toolkit_summary.get("hidden", 0),
            "uniqueIdentifiers": toolkit_summary.get("uniqueIdentifiers", 0),
            "providers": toolkit_summary.get("providers", {}),
            "classifications": toolkit_summary.get("classifications", {}),
            "containers": toolkit_summary.get("containers", 0),
            "appleContainers": toolkit_summary.get("appleContainers", 0),
            "thirdPartyContainers": toolkit_summary.get("thirdPartyContainers", 0),
            "appleLinkRunnable": {
                "total": len(apple_runnable_ids),
                "covered": len(apple_runnable_covered),
                "missing": len(apple_runnable_missing)
            }
        },
        "parameterSurface": {
            "actionsWithUnmodeledKeys": len(parameter_gaps)
        }
    }

    missing_builtins_list = sorted(builtin_missing)
    missing_apple_intents_list = sorted(apple_intents_missing)
    missing_apple_runnable_list = sorted(apple_runnable_missing)

    return summary, missing_builtins_list, missing_apple_intents_list, missing_apple_runnable_list, parameter_gaps


def main():
    parser = argparse.ArgumentParser(description="Compares extracted Apple runtime surfaces against Cherri catalog")
    parser.add_argument("--cherri-actions", required=True, help="Path to cherri-actions.json")
    parser.add_argument("--snapshot", help="Path to apple-runtime-snapshot.json or .json.gz")
    parser.add_argument("--builtin-defs", help="Path to builtin-actions.json")
    parser.add_argument("--dyld-ids", help="Path to builtin-action-identifiers.txt")
    parser.add_argument("--app-intents", help="Path to app-provided-actions.json")
    parser.add_argument("--toolkit-registry", help="Path to toolkit-registry.json")
    parser.add_argument("--toolkit-summary", help="Path to toolkit-summary.json")
    parser.add_argument("--parameter-encodings", help="Path to parameter-encodings.json")
    parser.add_argument("--runtime-evidence-dir", default=None, help="Directory containing Apple runtime evidence files")
    parser.add_argument("--out-dir", default="out", help="Output directory")
    args = parser.parse_args()

    out_dir = pathlib.Path(args.out_dir)
    out_dir.mkdir(parents=True, exist_ok=True)

    evidence_candidates = []
    if args.runtime_evidence_dir:
        evidence_candidates.append(pathlib.Path(args.runtime_evidence_dir))
    evidence_candidates.extend([
        pathlib.Path("docs/apple-runtime/macos-27"),
        pathlib.Path("docs/apple-runtime"),
        pathlib.Path("docs/apple-runtime/macos-26"),
    ])

    builtin_defs = args.builtin_defs
    dyld_ids = args.dyld_ids
    app_intents = args.app_intents
    toolkit_registry = args.toolkit_registry

    if args.snapshot:
        snap = load_json(args.snapshot, {})
        actions = snap.get("actions", {})
        if not builtin_defs:
            builtin_defs = {k: v.get("metadata", {}).get("builtin", {}) for k, v in actions.items() if "builtinDefinition" in v.get("sources", [])}
        if not dyld_ids:
            dyld_ids = [k for k, v in actions.items() if "dyld" in v.get("sources", [])]
        if not app_intents:
            app_intents_list = []
            for k, v in actions.items():
                if "appIntent" in v.get("sources", []):
                    meta = v.get("metadata", {})
                    app_intents_list.append({
                        "bundleIdentifier": meta.get("bundleIdentifier") or v.get("bundleIdentifier") or "",
                        "shortcutActionIdentifier": v.get("identifier") or k,
                        "discoverable": meta.get("discoverable", False)
                    })
            app_intents = {"actions": app_intents_list}
        if not toolkit_registry:
            for edir in evidence_candidates:
                tk_reg = edir / "toolkit-registry.json.gz"
                if tk_reg.exists():
                    toolkit_registry = str(tk_reg)
                    break
                tk_reg_raw = edir / "toolkit-registry.json"
                if tk_reg_raw.exists():
                    toolkit_registry = str(tk_reg_raw)
                    break
            if not toolkit_registry:
                toolkit_registry = snap.get("toolkitEvidence", {})

    if not args.toolkit_summary:
        for edir in evidence_candidates:
            tk_sum = edir / "toolkit-summary.json"
            if tk_sum.exists():
                args.toolkit_summary = str(tk_sum)
                break

    if not args.parameter_encodings:
        for edir in evidence_candidates:
            pe_gz = edir / "parameter-encodings.json.gz"
            if pe_gz.exists():
                args.parameter_encodings = str(pe_gz)
                break
            pe_raw = edir / "parameter-encodings.json"
            if pe_raw.exists():
                args.parameter_encodings = str(pe_raw)
                break

    summary, missing_defs, missing_intents, missing_runnable, gaps = run_comparison(
        cherri_actions_path=args.cherri_actions,
        builtin_defs_path=builtin_defs,
        dyld_ids_path=dyld_ids,
        app_intents_path=app_intents,
        toolkit_registry_path=toolkit_registry,
        toolkit_summary_path=args.toolkit_summary,
        parameter_encodings_path=args.parameter_encodings
    )

    (out_dir / "comparison-summary.json").write_text(json.dumps(summary, indent=2, sort_keys=True), encoding="utf-8")
    (out_dir / "missing-builtins.json").write_text(json.dumps(missing_defs, indent=2), encoding="utf-8")
    (out_dir / "missing-apple-app-intents.json").write_text(json.dumps(missing_intents, indent=2), encoding="utf-8")
    (out_dir / "missing-toolkit-apple-actions.json").write_text(json.dumps(missing_runnable, indent=2), encoding="utf-8")
    (out_dir / "parameter-surface-gaps.json").write_text(json.dumps(gaps, indent=2), encoding="utf-8")

    print("=== Cherri vs Apple Shortcuts Runtime Comparison ===")
    print(f"Cherri catalog: {summary['cherriActions']} actions ({summary['cherriUniqueIdentifiers']} unique identifiers)")
    b = summary["builtins"]
    print(f"Built-in definitions: {b['totalDefinitions']} total | {b['covered']} covered ({b['coveragePercentage']}%) | {b['missing']} missing")
    d = summary["dyld"]
    print(f"dyld shared cache:    {d['totalIdentifiers']} total | {d['covered']} covered | {d['missing']} missing")
    ai = summary["appleAppIntents"]
    print(f"Apple App Intents:    {ai['totalExtracted']} total | {ai['totalCovered']} covered | {ai['totalMissing']} missing")
    print(f"  (Discoverable:      {ai['discoverableTotal']} total | {ai['discoverableCovered']} covered | {ai['discoverableMissing']} missing)")
    tk = summary["toolkit"]
    print(f"ToolKit Registry:     {tk.get('totalTools', 0)} total tools | Visible: {tk.get('visible', 0)} | Apple Runnable: {tk.get('appleLinkRunnable', {}).get('total', 0)} ({tk.get('appleLinkRunnable', {}).get('covered', 0)} covered)")
    print(f"Parameter gaps:       {summary['parameterSurface']['actionsWithUnmodeledKeys']} actions with unmodeled keys")


if __name__ == "__main__":
    main()
