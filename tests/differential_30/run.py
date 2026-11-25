#!/usr/bin/env python3
"""Compile real paired sources and preserve strict typed plist differences.

This is a test adapter around tools/p10r7/independent_fidelity_oracle.py.
No plist is authored, repaired, or reconstructed by this runner.
"""
import argparse
import base64
import concurrent.futures
import copy
import datetime
import hashlib
import importlib.util
import json
import pathlib
import plistlib
import shutil
import subprocess
import sys
import time
from collections import Counter

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parents[1]
spec = importlib.util.spec_from_file_location(
    "cherri_fidelity_oracle", ROOT / "tools/p10r7/independent_fidelity_oracle.py")
oracle = importlib.util.module_from_spec(spec)
spec.loader.exec_module(oracle)
IDENTITY_KEYS = {"UUID", "OutputUUID", "GroupingIdentifier"}
DISPLAY_KEYS = {"OutputName", "CustomOutputName", "WFWorkflowActionTitle"}
MISSING = object()


def sha(path):
    return hashlib.sha256(pathlib.Path(path).read_bytes()).hexdigest()


def typed(value):
    """Use explicit scalar tags; bool/int, int/real and data/text stay distinct."""
    if value is MISSING:
        return {"type": "absent"}
    if isinstance(value, dict):
        return {"type": "dictionary", "value": {str(k): typed(v) for k, v in value.items()}}
    if isinstance(value, list):
        return {"type": "array", "value": [typed(v) for v in value]}
    if isinstance(value, bytes):
        return {"type": "data", "base64": base64.b64encode(value).decode("ascii")}
    if isinstance(value, datetime.datetime):
        return {"type": "date", "value": value.isoformat()}
    name = {str: "text", bool: "bool", int: "integer", float: "real", type(None): "null"}.get(type(value), type(value).__name__)
    return {"type": name, "value": value}


def write_json(path, value):
    pathlib.Path(path).write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n")


def category(path, default_keys):
    if path and path[0] != "WFWorkflowActions":
        return "workflow_metadata"
    key = str(path[-1]) if path else ""
    if key in DISPLAY_KEYS or key.startswith("Show-"):
        return "display_candidate"
    if len(path) == 4 and path[:1] == ["WFWorkflowActions"] and path[2] == "WFWorkflowActionParameters" and (path[1], key) in default_keys:
        return "catalog_static_or_default_candidate"
    return "structure_or_value"


def exact_diffs(left, right, default_keys, scope=None):
    result = []

    def visit(a, b, path):
        key = str(path[-1]) if path else ""
        if type(a) is not type(b):
            kind = "added" if a is MISSING else "removed" if b is MISSING else "type_change"
            result.append({"path": path, "kind": kind, "category": category(path, default_keys),
                           "before": typed(a), "after": typed(b)})
            return
        if isinstance(a, dict):
            for k in sorted(set(a) | set(b)):
                visit(a.get(k, MISSING), b.get(k, MISSING), path + [k])
        elif isinstance(a, list):
            for i in range(max(len(a), len(b))):
                visit(a[i] if i < len(a) else MISSING, b[i] if i < len(b) else MISSING, path + [i])
        else:
            equal = a == b
            if scope is not None and isinstance(a, str) and key in IDENTITY_KEYS:
                if key == "UUID":
                    equal = scope.match_uuid(a, b)
                elif key == "OutputUUID":
                    equal = scope.match_reference_uuid(a, b)
                else:
                    equal = scope.match_group(a, b)
            if not equal:
                result.append({"path": path, "kind": "value_change", "category": category(path, default_keys),
                               "before": typed(a), "after": typed(b)})
    visit(left, right, [])
    return result


def graph_audit(doc):
    actions = doc.get("WFWorkflowActions", [])
    producers = {}
    errors = []
    refs = []
    stack = []
    quantity_signals = []
    for i, action in enumerate(actions):
        params = action.get("WFWorkflowActionParameters", {})
        uid = params.get("UUID")
        if uid is not None:
            if not isinstance(uid, str) or not uid:
                errors.append({"kind": "invalid_producer", "action": i})
            elif uid in producers:
                errors.append({"kind": "duplicate_producer", "action": i, "other": producers[uid], "uuid": uid})
            else:
                producers[uid] = i
        for path, uid in oracle.collect_references(params):
            refs.append({"action": i, "path": path, "uuid": uid})
        if "WFControlFlowMode" in params:
            mode = params["WFControlFlowMode"]
            group = params.get("GroupingIdentifier")
            if not isinstance(group, str) or not group:
                errors.append({"kind": "missing_control_group", "action": i})
                continue
            if mode == 0:
                if group in stack:
                    errors.append({"kind": "duplicate_group_start", "action": i})
                stack.append(group)
            elif not stack or stack[-1] != group:
                errors.append({"kind": "control_group_nesting", "action": i, "mode": mode})
            elif mode == 2:
                stack.pop()
            elif mode != 1:
                errors.append({"kind": "unknown_control_mode", "action": i, "mode": mode})

        def quantities(node, path):
            if isinstance(node, dict):
                for key, value in node.items():
                    if key == "Magnitude" and isinstance(value, str) and any(
                            marker in value for marker in ("ActionOutput", "VariableName", "WFTextTokenAttachment")):
                        quantity_signals.append({"action": i, "path": path + [key], "value": value})
                    quantities(value, path + [key])
            elif isinstance(node, list):
                for index, value in enumerate(node):
                    quantities(value, path + [index])
        quantities(params, [])
    for ref in refs:
        ref["producer_action"] = producers.get(ref["uuid"])
        if ref["producer_action"] is None:
            errors.append({"kind": "dangling_reference", **ref})
    if stack:
        errors.append({"kind": "unclosed_control_groups", "groups": stack})
    return {"valid": not errors, "errors": errors, "producers": producers, "references": refs,
            "serialized_quantity_reference_signals": quantity_signals}


def identity_scope(left, right):
    la = left.get("WFWorkflowActions", [])
    ra = right.get("WFWorkflowActions", [])
    if [x.get("WFWorkflowActionIdentifier") for x in la] != [x.get("WFWorkflowActionIdentifier") for x in ra]:
        return None
    scope = oracle.BijectiveUUIDScope()
    for i, (a, b) in enumerate(zip(la, ra)):
        ap = a.get("WFWorkflowActionParameters", {})
        bp = b.get("WFWorkflowActionParameters", {})
        if isinstance(ap.get("UUID"), str) and isinstance(bp.get("UUID"), str):
            if scope.register_action_uuid(ap["UUID"], bp["UUID"], i):
                return None
        if isinstance(ap.get("GroupingIdentifier"), str) and isinstance(bp.get("GroupingIdentifier"), str):
            if scope.register_group_uuid(ap["GroupingIdentifier"], bp["GroupingIdentifier"]):
                return None
    return scope


def compare(left_path, right_path, default_catalog):
    left = plistlib.loads(pathlib.Path(left_path).read_bytes())
    right = plistlib.loads(pathlib.Path(right_path).read_bytes())
    default_keys = set()
    for doc in (left, right):
        for index, action in enumerate(doc.get("WFWorkflowActions", [])):
            default_keys.update((index, key) for key in default_catalog.get(action.get("WFWorkflowActionIdentifier"), set()))
    raw = exact_diffs(left, right, default_keys)
    scope = identity_scope(left, right)
    alpha = exact_diffs(left, right, default_keys, scope)
    graph_left = graph_audit(left)
    graph_right = graph_audit(right)
    independent = oracle.evaluate_shortcut_fidelity(left, right)
    return {
        "strict_typed_equal": not raw,
        "uuid_alpha_allowed": scope is not None,
        "strict_typed_equal_after_uuid_alpha": not alpha and graph_left["valid"] and graph_right["valid"],
        "action_count": [len(left.get("WFWorkflowActions", [])), len(right.get("WFWorkflowActions", []))],
        "action_identifiers": [[x["WFWorkflowActionIdentifier"] for x in left.get("WFWorkflowActions", [])],
                               [x["WFWorkflowActionIdentifier"] for x in right.get("WFWorkflowActions", [])]],
        "action_tree_equal_after_uuid_alpha": not any(x["path"][:1] == ["WFWorkflowActions"] for x in alpha),
        "graphs_valid": graph_left["valid"] and graph_right["valid"],
        "raw_difference_count": len(raw),
        "alpha_difference_count": len(alpha),
        "difference_categories": dict(Counter(x["category"] for x in alpha)),
        "raw_differences": raw,
        "alpha_differences": alpha,
        "graph": {"left": graph_left, "right": graph_right},
        "independent_oracle": independent,
        "runtime_equivalence": "NOT_RUN",
        "interpretation": "Every difference is retained. Categories are review aids, never semantic exemptions."
    }


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--original", required=True, type=pathlib.Path)
    ap.add_argument("--original-sha", default="98d5191ae121a6bcf663ed0e55495f4ee4c7a6e2")
    ap.add_argument("--fork", required=True, type=pathlib.Path)
    ap.add_argument("--fork-sha", required=True)
    ap.add_argument("--frozen-original", type=pathlib.Path)
    ap.add_argument("--out", required=True, type=pathlib.Path)
    ap.add_argument("--manifest", type=pathlib.Path, default=HERE / "manifest.json")
    ap.add_argument("--lanes", default="original,fork_legacy,fork_v2")
    ap.add_argument("--only", help="Comma-separated fixture ids or two-digit numbers")
    ap.add_argument("--reuse-from", type=pathlib.Path)
    ap.add_argument("--workers", type=int, default=4)
    ap.add_argument("--timeout", type=int, default=60)
    ap.add_argument("--derive-uuids", action="store_true", help="Optional explicit compiler flag; default UUID generation is preferred.")
    args = ap.parse_args()
    args.out = args.out.resolve()
    args.out.mkdir(parents=True, exist_ok=True)
    if (args.out / "results.json").exists():
        ap.error("Output already has a receipt; use a new --out directory to preserve evidence.")
    manifest = json.loads(args.manifest.read_text())
    cases = manifest["fixtures"]
    if args.only:
        wanted = set(args.only.split(","))
        cases = [c for c in cases if c["id"] in wanted or f'{c["number"]:02d}' in wanted]
    lanes = args.lanes.split(",")
    if "frozen_original" in lanes and not args.frozen_original:
        ap.error("--frozen-original is required for frozen_original lane")
    binaries = {"original": args.original.resolve(), "fork_legacy": args.fork.resolve(),
                "fork_v2": args.fork.resolve()}
    if args.frozen_original:
        binaries["frozen_original"] = args.frozen_original.resolve()
    for lane in lanes:
        if lane not in binaries or not binaries[lane].is_file():
            ap.error("Missing compiler for lane " + lane)
    binary_hash = {lane: sha(binaries[lane]) for lane in lanes}
    prior = {}
    if args.reuse_from and (args.reuse_from / "results.json").is_file():
        prior = {(r["fixture_id"], r["lane"]): r for r in json.loads(
            (args.reuse_from / "results.json").read_text())["compilations"]}
    receipt = {
        "schema_version": 1, "suite_id": manifest["suite_id"],
        "created_utc": datetime.datetime.now(datetime.timezone.utc).isoformat(),
        "manifest_sha256": sha(args.manifest), "runner_sha256": sha(__file__), "source_root": str(args.manifest.parent.resolve()),
        "original_sha": args.original_sha, "fork_sha": args.fork_sha,
        "binaries": {lane: {"path": str(binaries[lane]), "sha256": binary_hash[lane]} for lane in lanes},
        "compilations": [], "comparisons": [], "runtime_status": "NOT_RUN",
    }
    write_json(args.out / "manifest.json", manifest)

    def compile_one(case, lane):
        source_lane = ("frozen_original" if "frozen_original" in case["sources"] else "original") if lane == "frozen_original" else lane
        source = args.manifest.parent / case["sources"][source_lane]
        staged_dir = args.out / "sources" / lane
        staged_dir.mkdir(parents=True, exist_ok=True)
        staged = staged_dir / source.name
        shutil.copy2(source, staged)
        artifact_dir = args.out / "artifacts" / lane
        artifact_dir.mkdir(parents=True, exist_ok=True)
        artifact = artifact_dir / (case["shortcut_names"][source_lane] + ".shortcut")
        log_dir = args.out / "logs" / lane
        log_dir.mkdir(parents=True, exist_ok=True)
        logfile = log_dir / (case["id"] + ".log")
        row = {"fixture_id": case["id"], "lane": lane, "source": str(source),
               "source_sha256": sha(source), "binary_sha256": binary_hash[lane],
               "artifact": str(artifact), "log": str(logfile), "expected": case["expected"],
               "runtime": case["runtime"], "shortcut_name": case["shortcut_names"][source_lane]}
        compiler_flags = ["--skip-sign", "--debug", "--no-ansi"]
        if args.derive_uuids:
            compiler_flags.append("--derive-uuids")
        if lane == "fork_legacy":
            compiler_flags.append("--legacy")
        elif lane == "fork_v2":
            compiler_flags.append("--mode=v2")
        row["compiler_flags"] = compiler_flags
        previous = prior.get((case["id"], lane))
        if previous and previous.get("compiler_flags") == compiler_flags and previous["source_sha256"] == row["source_sha256"] and previous["binary_sha256"] == row["binary_sha256"] and previous.get("status") == "COMPILED" and pathlib.Path(previous["artifact"]).is_file() and previous.get("artifact_sha256") == sha(previous["artifact"]):
            for key, destination in [("artifact", artifact), ("log", logfile)]:
                old = pathlib.Path(previous[key])
                if old.exists() and old.resolve() != destination.resolve():
                    shutil.copy2(old, destination)
            row.update({"status": "COMPILED", "returncode": 0, "reused_from": str(args.reuse_from)})
        else:
            command = [str(binaries[lane]), str(staged)] + compiler_flags + ["--output=" + str(artifact)]
            row["command"] = command
            start = time.monotonic()
            try:
                run = subprocess.run(command, cwd=staged_dir, capture_output=True, timeout=args.timeout)
                logfile.write_bytes(run.stdout + b"\n--- STDERR ---\n" + run.stderr)
                upstream_unsigned = staged.with_name(staged.stem + "_unsigned.shortcut")
                if run.returncode == 0 and not artifact.exists() and upstream_unsigned.is_file():
                    shutil.copy2(upstream_unsigned, artifact)
                    row["compiler_produced_path"] = str(upstream_unsigned)
                    row["artifact_collection"] = "Copied exact compiler bytes; upstream ignores --output with --skip-sign."
                row.update({"returncode": run.returncode, "duration_seconds": round(time.monotonic() - start, 3),
                            "status": "COMPILED" if run.returncode == 0 and artifact.is_file() else "COMPILE_FAILED"})
                if row["status"] != "COMPILED":
                    lines = (run.stdout + b"\n" + run.stderr).decode("utf-8", errors="replace").splitlines()
                    row["diagnostics"] = [line for line in lines if any(x in line for x in ("Error:", "error", "panic", "E_"))][-12:]
            except subprocess.TimeoutExpired as exc:
                logfile.write_bytes((exc.stdout or b"") + b"\n--- STDERR ---\n" + (exc.stderr or b""))
                row.update({"status": "COMPILE_TIMEOUT", "returncode": None})
        if row["status"] == "COMPILED":
            try:
                doc = plistlib.loads(artifact.read_bytes())
                row["artifact_sha256"] = sha(artifact)
                row["action_count"] = len(doc["WFWorkflowActions"])
                row["graph"] = graph_audit(doc)
                exact = artifact.with_suffix(".typed.json")
                write_json(exact, typed(doc))
                row["typed_output"] = str(exact)
            except Exception as exc:
                row["status"] = "PLIST_DECODE_FAILED"
                row["decode_error"] = type(exc).__name__ + ": " + str(exc)
        return row

    with concurrent.futures.ThreadPoolExecutor(max_workers=max(1, args.workers)) as executor:
        jobs = [executor.submit(compile_one, case, lane) for case in cases for lane in lanes]
        for future in concurrent.futures.as_completed(jobs):
            row = future.result()
            receipt["compilations"].append(row)
            write_json(args.out / "results.json", receipt)
            print(json.dumps({"fixture_id": row["fixture_id"], "lane": row["lane"], "status": row["status"],
                              "diagnostics": row.get("diagnostics", [])}, ensure_ascii=False), flush=True)

    default_catalog = {}
    catalog = subprocess.run([str(args.fork.resolve()), "--actions-json", "--no-ansi"], capture_output=True)
    if catalog.returncode == 0:
        (args.out / "fork-action-catalog.json").write_bytes(catalog.stdout)
        for action in json.loads(catalog.stdout).get("actions", []):
            keys = default_catalog.setdefault(action.get("shortcutIdentifier"), set())
            keys.update(p.get("key", "") for p in action.get("parameters", []) if "default" in p)
            keys.update(action.get("staticParameters", {}).keys())
    indexed = {(r["fixture_id"], r["lane"]): r for r in receipt["compilations"]}
    compare_dir = args.out / "comparisons"
    compare_dir.mkdir(exist_ok=True)
    for case in cases:
        for left_lane, right_lane in [("original", "fork_legacy"), ("original", "fork_v2"),
                                       ("frozen_original", "original")]:
            left = indexed.get((case["id"], left_lane))
            right = indexed.get((case["id"], right_lane))
            if not left or not right:
                continue
            row = {"fixture_id": case["id"], "left_lane": left_lane, "right_lane": right_lane}
            if left["status"] != "COMPILED" or right["status"] != "COMPILED":
                row["status"] = "BLOCKED_COMPILE"
            else:
                report = compare(left["artifact"], right["artifact"], default_catalog)
                path = compare_dir / f'{case["id"]}.{left_lane}-vs-{right_lane}.json'
                write_json(path, report)
                row.update({"status": "WIRE_EQUAL" if report["strict_typed_equal_after_uuid_alpha"] else "WIRE_DIFFERENT",
                            "report": str(path), "difference_categories": report["difference_categories"],
                            "action_count": report["action_count"]})
            receipt["comparisons"].append(row)
    receipt["compilations"].sort(key=lambda r: (r["fixture_id"], r["lane"]))
    receipt["summary"] = {
        "fixtures": len(cases),
        "native_enabled": sum(c["runtime"]["enabled"] for c in cases),
        "compile_status": {lane: dict(Counter(r["status"] for r in receipt["compilations"] if r["lane"] == lane)) for lane in lanes},
        "comparison_status": dict(Counter(r["status"] for r in receipt["comparisons"])),
        "runtime_status": "NOT_RUN",
    }
    write_json(args.out / "results.json", receipt)
    print(json.dumps(receipt["summary"], indent=2), flush=True)
    return 0 if all(r["status"] == "COMPILED" for r in receipt["compilations"]) else 1


if __name__ == "__main__":
    raise SystemExit(main())
