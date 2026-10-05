#!/usr/bin/env python3
"""tools/shortcut-runtime/merge_parameters.py

Deterministic merger for parameter extraction shard artifacts.
Produces:
  - parameter-encodings.json (compatible with ShortcutKit extract-encoding-table.js / verify-encodings.js)
  - parameter-encodings-summary.json
  - parameter-timeouts.json
  - parameter-errors.json
"""

import argparse
import glob
import json
import os
import pathlib
import sys


def load_shard(file_path):
    path = pathlib.Path(file_path)
    if not path.exists():
        raise FileNotFoundError(f"Shard file not found: {file_path}")
    try:
        content = path.read_text(encoding="utf-8")
        data = json.loads(content)
        if not isinstance(data, dict):
            raise ValueError(f"Shard data must be a JSON object, got {type(data).__name__}")
        return data
    except json.JSONDecodeError as e:
        raise ValueError(f"Malformed JSON in shard file {file_path}: {e}")


def merge_shards(shard_files, expected_shard_count=None):
    # Sort files deterministically
    sorted_files = sorted(shard_files)

    shards = []
    found_indices = set()

    for sf in sorted_files:
        data = load_shard(sf)
        shards.append(data)
        idx = data.get("shardIndex")
        if idx is not None:
            found_indices.add(idx)

    missing_shards = []
    if expected_shard_count is not None:
        for i in range(expected_shard_count):
            if i not in found_indices:
                missing_shards.append(i)

    # Aggregates
    merged_action_parameters = {}
    merged_output_names = {}
    merged_defaults = {}
    merged_unavailable = set()
    merged_statuses = {}
    merged_timeouts = []
    merged_errors = []
    merged_param_classes = {}
    merged_state_selectors = {}

    total_actions_attempted = 0
    total_successes = 0
    total_timeouts = 0
    total_errors = 0
    total_params_observed = 0
    shard_summaries = []

    for shard in shards:
        idx = shard.get("shardIndex", -1)
        summary = shard.get("summary", {})
        shard_summaries.append(summary)

        total_actions_attempted += shard.get("actionsAttempted", len(shard.get("actionStatuses", {})))
        total_successes += summary.get("successes", 0)
        total_timeouts += summary.get("timeouts", 0)
        total_errors += summary.get("errors", 0)

        # Unavailable actions
        for u in shard.get("unavailableActions", []):
            merged_unavailable.add(u)

        # Output names
        for k, v in shard.get("actionOutputNames", {}).items():
            merged_output_names[k] = v

        # Defaults
        for k, v in shard.get("actionDefaults", {}).items():
            if k not in merged_defaults:
                merged_defaults[k] = {}
            merged_defaults[k].update(v)

        # Action parameters
        for k, v in shard.get("actionParameters", {}).items():
            merged_action_parameters[k] = v
            total_params_observed += len(v)

        # Action statuses
        for k, v in shard.get("actionStatuses", {}).items():
            merged_statuses[k] = v

        # Timeouts and errors
        for t in shard.get("timeouts", []):
            merged_timeouts.append(t)
        for e in shard.get("errors", []):
            merged_errors.append(e)

        # State class selectors if present
        for sc, sels in shard.get("stateClassSelectors", {}).items():
            if sc not in merged_state_selectors:
                merged_state_selectors[sc] = sorted(set(sels))
            else:
                merged_state_selectors[sc] = sorted(set(merged_state_selectors[sc]) | set(sels))

        # Parameter classes
        for pc, info in shard.get("parameterClasses", {}).items():
            entry = merged_param_classes.setdefault(pc, {
                "stateClass": info.get("stateClass"),
                "count": 0,
                "defaultExamples": [],
                "usedBy": []
            })
            entry["count"] += info.get("count", 0)
            if info.get("stateClass") and not entry.get("stateClass"):
                entry["stateClass"] = info["stateClass"]

            for ub in info.get("usedBy", []):
                if ub not in entry["usedBy"] and len(entry["usedBy"]) < 5:
                    entry["usedBy"].append(ub)

            for ex in info.get("defaultExamples", []):
                dumped = json.dumps(ex, sort_keys=True)
                if len(entry["defaultExamples"]) < 3 and not any(json.dumps(x, sort_keys=True) == dumped for x in entry["defaultExamples"]):
                    entry["defaultExamples"].append(ex)

    # Sort dictionary keys deterministically
    sorted_action_parameters = {k: merged_action_parameters[k] for k in sorted(merged_action_parameters.keys())}
    sorted_output_names = {k: merged_output_names[k] for k in sorted(merged_output_names.keys())}
    sorted_defaults = {k: {pk: merged_defaults[k][pk] for pk in sorted(merged_defaults[k].keys())} for k in sorted(merged_defaults.keys())}
    sorted_unavailable = sorted(merged_unavailable)
    sorted_statuses = {k: merged_statuses[k] for k in sorted(merged_statuses.keys())}
    sorted_param_classes = {k: merged_param_classes[k] for k in sorted(merged_param_classes.keys())}
    for pc_info in sorted_param_classes.values():
        pc_info["usedBy"] = sorted(pc_info["usedBy"])

    sorted_timeouts = sorted(merged_timeouts, key=lambda x: x.get("identifier", ""))
    sorted_errors = sorted(merged_errors, key=lambda x: x.get("identifier", ""))

    encodings_output = {
        "parameterClasses": sorted_param_classes,
        "stateClassSelectors": merged_state_selectors,
        "actionParameters": sorted_action_parameters,
        "actionOutputNames": sorted_output_names,
        "actionDefaults": sorted_defaults,
        "unavailableActions": sorted_unavailable
    }

    summary_output = {
        "schemaVersion": "2.0.0",
        "expectedShards": expected_shard_count,
        "foundShards": len(shards),
        "missingShards": missing_shards,
        "totalActionsDiscovered": len(sorted_statuses),
        "actionsAttempted": total_actions_attempted,
        "actionsCompleted": len(sorted_statuses),
        "actionsSuccessful": total_successes,
        "actionsTimedOut": total_timeouts,
        "actionsError": total_errors,
        "actionsUnavailable": len(sorted_unavailable),
        "totalParametersObserved": total_params_observed,
        "parameterClassesObserved": len(sorted_param_classes),
        "shardSummaries": sorted(shard_summaries, key=lambda x: x.get("shardIndex", 0))
    }

    return encodings_output, summary_output, sorted_timeouts, sorted_errors


def main():
    parser = argparse.ArgumentParser(description="Deterministic merger for parameter extraction shards")
    parser.add_argument("--shards-dir", help="Directory containing parameter-encodings-shard-*.json")
    parser.add_argument("--shards", nargs="*", help="Explicit shard json files")
    parser.add_argument("--shard-count", type=int, default=10, help="Expected total shard count")
    parser.add_argument("--out-dir", default="out/data", help="Output directory")
    args = parser.parse_args()

    files = []
    if args.shards:
        files = args.shards
    elif args.shards_dir:
        pattern = os.path.join(args.shards_dir, "parameter-encodings-shard-*.json")
        files = sorted(glob.glob(pattern))

    if not files:
        print("Error: No shard files found to merge.", file=sys.stderr)
        sys.exit(1)

    out_dir = pathlib.Path(args.out_dir)
    out_dir.mkdir(parents=True, exist_ok=True)

    encodings, summary, timeouts, errors = merge_shards(files, args.shard_count)

    (out_dir / "parameter-encodings.json").write_text(json.dumps(encodings, indent=2, sort_keys=True), encoding="utf-8")
    (out_dir / "parameter-encodings-summary.json").write_text(json.dumps(summary, indent=2, sort_keys=True), encoding="utf-8")
    (out_dir / "parameter-timeouts.json").write_text(json.dumps(timeouts, indent=2, sort_keys=True), encoding="utf-8")
    (out_dir / "parameter-errors.json").write_text(json.dumps(errors, indent=2, sort_keys=True), encoding="utf-8")

    print(f"Merged {len(files)} shards:")
    print(f"  Actions: {summary['actionsCompleted']} (success: {summary['actionsSuccessful']}, timeout: {summary['actionsTimedOut']}, error: {summary['actionsError']})")
    print(f"  Parameter Classes: {summary['parameterClassesObserved']}")
    if summary["missingShards"]:
        print(f"  WARNING: Missing shards: {summary['missingShards']}", file=sys.stderr)


if __name__ == "__main__":
    main()
