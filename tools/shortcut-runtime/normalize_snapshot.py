#!/usr/bin/env python3
"""tools/shortcut-runtime/normalize_snapshot.py

Generates a normalized, provenance-aware Apple Runtime Snapshot and Manifest:
  - apple-runtime-snapshot.json
  - apple-runtime-manifest.json

Unifies evidence across builtins, dyld, ToolKit, App Intents, Gallery, and parameter encodings.
Sanitizes runner-specific absolute paths.
"""

import argparse
import datetime
import json
import os
import pathlib
import re
import sys


def sanitize_paths(data):
    """Recursively sanitize absolute runner paths from string values."""
    if isinstance(data, str):
        # Replace /Users/<username> or /Users/runner with ~
        return re.sub(r"/Users/[^/\s'\"]+", "~", data)
    elif isinstance(data, dict):
        return {k: sanitize_paths(v) for k, v in data.items()}
    elif isinstance(data, list):
        return [sanitize_paths(v) for v in data]
    return data


def load_json(path, default=None):
    if not path or not os.path.exists(path):
        return default
    try:
        return json.loads(pathlib.Path(path).read_text(encoding="utf-8"))
    except Exception as e:
        print(f"Warning: Failed to load {path}: {e}", file=sys.stderr)
        return default


def load_lines(path):
    if not path or not os.path.exists(path):
        return []
    try:
        return [line.strip() for line in pathlib.Path(path).read_text(encoding="utf-8").splitlines() if line.strip()]
    except Exception as e:
        print(f"Warning: Failed to load lines from {path}: {e}", file=sys.stderr)
        return []


def build_snapshot(
    builtin_defs_path=None,
    dyld_ids_path=None,
    app_intents_path=None,
    toolkit_summary_path=None,
    gallery_path=None,
    parameter_encodings_path=None,
    roundtrips_path=None,
    env_path=None,
    cherri_sha="unknown",
    shortcutkit_commit="ee52c32937053268e4a035eba4b372fe77ec6411",
    shard_count=10
):
    env = load_json(env_path, {})
    builtin_defs = load_json(builtin_defs_path, {})
    dyld_ids = set(load_lines(dyld_ids_path))
    app_actions_raw = load_json(app_intents_path, [])
    param_encodings = load_json(parameter_encodings_path, {})
    roundtrips = load_json(roundtrips_path, {})
    gallery_raw = load_json(gallery_path, {})

    actions_snapshot = {}

    # Helper to get or init action record
    def get_action_record(ident):
        if ident not in actions_snapshot:
            actions_snapshot[ident] = {
                "identifier": ident,
                "sources": [],
                "metadata": {},
                "parameters": [],
                "encodingEvidence": {},
                "verification": {}
            }
        return actions_snapshot[ident]

    # 1. Built-in definitions
    if isinstance(builtin_defs, dict):
        for ident, defn in builtin_defs.items():
            rec = get_action_record(ident)
            if "builtinDefinition" not in rec["sources"]:
                rec["sources"].append("builtinDefinition")
            rec["metadata"]["actionClass"] = defn.get("ActionClass")
            rec["metadata"]["fillingProvider"] = defn.get("FillingProvider")
            if defn.get("ActionName"):
                rec["metadata"]["name"] = defn["ActionName"]
            if defn.get("Category"):
                rec["metadata"]["category"] = defn["Category"]

    # 2. dyld shared-cache identifiers
    for ident in dyld_ids:
        rec = get_action_record(ident)
        if "dyld" not in rec["sources"]:
            rec["sources"].append("dyld")

    # 3. Installed Apple App Intents
    if isinstance(app_actions_raw, list):
        for item in app_actions_raw:
            ident = item.get("shortcutActionIdentifier")
            if not ident:
                continue
            rec = get_action_record(ident)
            if "appIntent" not in rec["sources"]:
                rec["sources"].append("appIntent")
            rec["metadata"]["app"] = item.get("app")
            rec["metadata"]["bundleIdentifier"] = item.get("bundleIdentifier")
            rec["metadata"]["title"] = item.get("title")
            rec["metadata"]["discoverable"] = item.get("discoverable", False)
            if item.get("parameters"):
                rec["parameters"] = item["parameters"]

    # 4. Gallery actions
    if isinstance(gallery_raw, dict):
        for wf_id, wf in gallery_raw.items():
            if isinstance(wf, dict):
                for act in wf.get("WFWorkflowActions", []):
                    ident = act.get("WFWorkflowActionIdentifier")
                    if ident:
                        rec = get_action_record(ident)
                        if "gallery" not in rec["sources"]:
                            rec["sources"].append("gallery")

    # 5. Parameter encodings
    action_params = param_encodings.get("actionParameters", {})
    action_defaults = param_encodings.get("actionDefaults", {})
    action_outputs = param_encodings.get("actionOutputNames", {})
    unavailable = set(param_encodings.get("unavailableActions", []))

    for ident, plist in action_params.items():
        rec = get_action_record(ident)
        if "parameterProbe" not in rec["sources"]:
            rec["sources"].append("parameterProbe")
        rec["parameters"] = plist
        if ident in action_defaults:
            rec["encodingEvidence"]["defaults"] = action_defaults[ident]
        if ident in action_outputs:
            rec["encodingEvidence"]["outputName"] = action_outputs[ident]
        if ident in unavailable:
            rec["encodingEvidence"]["unavailable"] = True

    # 6. Verification outcomes from roundtrips
    state_results = roundtrips.get("results", {})
    if state_results:
        # Match parameter classes to state verification
        param_classes = param_encodings.get("parameterClasses", {})
        for ident, rec in actions_snapshot.items():
            for p in rec.get("parameters", []):
                sc = p.get("singleStateClass")
                if sc and sc in state_results:
                    rec["verification"][sc] = state_results[sc]

    # Sort sources for determinism
    for rec in actions_snapshot.values():
        rec["sources"] = sorted(rec["sources"])

    # Sort snapshot by action identifier
    sorted_snapshot = {k: actions_snapshot[k] for k in sorted(actions_snapshot.keys())}
    sanitized_snapshot = sanitize_paths(sorted_snapshot)

    # Manifest
    manifest = {
        "schemaVersion": "2.0.0",
        "cherriGitSha": cherri_sha,
        "generatedAt": datetime.datetime.now(datetime.timezone.utc).isoformat(),
        "runner": env.get("runner", "unknown"),
        "macOS": env.get("macOS", "unknown"),
        "build": env.get("build", "unknown"),
        "shortcutsVersion": env.get("shortcutsVersion", "unknown"),
        "shortcutsBuild": env.get("shortcutsBuild", "unknown"),
        "workflowKitVersion": env.get("workflowKitVersion", "unknown"),
        "actionKitVersion": env.get("actionKitVersion", "unknown"),
        "shortcutKitCommit": shortcutkit_commit,
        "shardCount": shard_count,
        "counts": {
            "totalUniqueActions": len(sanitized_snapshot),
            "builtinDefinitions": len(builtin_defs) if isinstance(builtin_defs, dict) else 0,
            "dyldIdentifiers": len(dyld_ids),
            "appIntentsTotal": len(app_actions_raw) if isinstance(app_actions_raw, list) else 0,
            "parameterProbedActions": len(action_params)
        }
    }
    sanitized_manifest = sanitize_paths(manifest)

    return {
        "schemaVersion": "2.0.0",
        "actionCount": len(sanitized_snapshot),
        "actions": sanitized_snapshot
    }, sanitized_manifest


def main():
    parser = argparse.ArgumentParser(description="Normalizes Apple Shortcuts runtime evidence into versioned snapshot")
    parser.add_argument("--builtin-defs", help="Path to builtin-actions.json")
    parser.add_argument("--dyld-ids", help="Path to builtin-action-identifiers.txt")
    parser.add_argument("--app-intents", help="Path to app-provided-actions.json")
    parser.add_argument("--toolkit-summary", help="Path to toolkit-summary.json")
    parser.add_argument("--gallery", help="Path to gallery-workflows.json")
    parser.add_argument("--parameter-encodings", help="Path to parameter-encodings.json")
    parser.add_argument("--roundtrips", help="Path to encoding-roundtrips.json")
    parser.add_argument("--environment", help="Path to environment.json")
    parser.add_argument("--cherri-sha", default="unknown", help="Cherri git commit SHA")
    parser.add_argument("--shortcutkit-commit", default="ee52c32937053268e4a035eba4b372fe77ec6411")
    parser.add_argument("--shard-count", type=int, default=10)
    parser.add_argument("--out-dir", default="out/data")
    args = parser.parse_args()

    out_dir = pathlib.Path(args.out_dir)
    out_dir.mkdir(parents=True, exist_ok=True)

    snapshot, manifest = build_snapshot(
        builtin_defs_path=args.builtin_defs,
        dyld_ids_path=args.dyld_ids,
        app_intents_path=args.app_intents,
        toolkit_summary_path=args.toolkit_summary,
        gallery_path=args.gallery,
        parameter_encodings_path=args.parameter_encodings,
        roundtrips_path=args.roundtrips,
        env_path=args.environment,
        cherri_sha=args.cherri_sha,
        shortcutkit_commit=args.shortcutkit_commit,
        shard_count=args.shard_count
    )

    (out_dir / "apple-runtime-snapshot.json").write_text(json.dumps(snapshot, indent=2, sort_keys=True), encoding="utf-8")
    (out_dir / "apple-runtime-manifest.json").write_text(json.dumps(manifest, indent=2, sort_keys=True), encoding="utf-8")

    print(f"Generated Apple Runtime Snapshot (schema v{snapshot['schemaVersion']}):")
    print(f"  Actions: {snapshot['actionCount']}")
    print(f"  Runner: {manifest['runner']} macOS {manifest['macOS']} ({manifest['build']})")
    print(f"  ShortcutKit pinned: {manifest['shortcutKitCommit']}")


if __name__ == "__main__":
    main()
