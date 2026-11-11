#!/usr/bin/env python3
"""
tools/p10r7/run_38_audit.py
Executes the full 38-shortcut corpus audit pipeline (P10R8 hardened harness).

Per specimen (001-038), against FRESH outputs only:
 1. Fail-closed inventory: all 38 ids must have a non-empty .plist and .json.
 2. Decompile with --legacy and with the V2 path (import without --legacy).
 3. Compile the decompiled Cherri sources (Legacy and V2) unsigned.
 4. Evaluate with independent_fidelity_oracle (strict native wire fidelity).
 5. Write PRIVATE detailed results outside git, and a SANITIZED matrix for docs.

No step ever trusts a pre-existing output file: each owned output path is removed before its
step runs (audit_integrity.run_compile_step). Missing inputs are FAIL, never SKIPPED.

Exit status is non-zero when --require-gates names gates that are not 38/38.
"""

import argparse
import collections
import datetime
import hashlib
import json
import pathlib
import platform
import plistlib
import subprocess
import sys
from typing import Any, Dict, List, Optional

script_dir = pathlib.Path(__file__).resolve().parent
sys.path.insert(0, str(script_dir))
import independent_fidelity_oracle as oracle  # noqa: E402
from audit_integrity import EXPECTED_SPECIMEN_IDS, check_corpus_inventory, check_runtime_gate, run_compile_step  # noqa: E402

HARNESS_VERSION = "p10r8-audit-harness-20261111.1"


def sha256_file(p: pathlib.Path) -> str:
    h = hashlib.sha256()
    with open(p, "rb") as f:
        while chunk := f.read(65536):
            h.update(chunk)
    return h.hexdigest()


def sanitize_mismatch(mismatch: Optional[Dict[str, Any]]) -> Optional[Dict[str, Any]]:
    if not mismatch:
        return None
    out = {
        "gate": mismatch.get("gate", ""),
        "category": mismatch.get("category", ""),
        "path": oracle.redact_path(mismatch.get("path", "")),
    }
    if mismatch.get("subcategory"):
        out["subcategory"] = mismatch["subcategory"]
    return out


def git_info(repo_root: pathlib.Path) -> Dict[str, Any]:
    def run(*a):
        try:
            return subprocess.check_output(["git", "-C", str(repo_root), *a], text=True, stderr=subprocess.DEVNULL).strip()
        except Exception:
            return ""
    return {"commit": run("rev-parse", "HEAD") or "unknown", "dirty": bool(run("status", "--porcelain"))}


def load_json_strict(p: pathlib.Path):
    """Returns (doc, ok). A present-but-unparseable file is not-ok (FAIL), never skipped."""
    try:
        with open(p, "r", encoding="utf-8-sig") as f:
            return json.load(f), True
    except Exception:
        return None, False


def evaluate_compiled(src_doc, json_doc, json_ok, shortcut_path: pathlib.Path, step: Dict[str, Any]):
    if step["gate"] == "PASS":
        try:
            with open(shortcut_path, "rb") as f:
                reb_doc = plistlib.load(f)
            res = oracle.evaluate_shortcut_fidelity(src_doc, reb_doc, json_doc if json_ok else {})
            if not json_ok:
                res["gates"]["RAW_JSON_DECODE_PASS"] = "FAIL"
                res["gates"]["FULL_WORKFLOW_SEMANTIC_PASS"] = "FAIL"
                res["extendedGates"]["SOURCE_CONTAINER_EQUIVALENCE_PASS"] = "FAIL"
            res["inventory"] = oracle.aligned_action_inventory(src_doc, reb_doc)
            return res, sha256_file(shortcut_path)
        except Exception as e:  # oracle exception is ERROR -> FAIL, never pass
            err = f"ORACLE_ERROR:{type(e).__name__}"
    else:
        err = "COMPILE_FAILED"
    src_n = len(src_doc.get("WFWorkflowActions", []))
    res = {
        "oracleVersion": oracle.ORACLE_VERSION,
        "gates": {g: "FAIL" for g in oracle.GATES},
        "extendedGates": {g: "FAIL" for g in oracle.EXTENDED_GATES},
        "sourceActions": src_n, "compiledActions": 0, "actionDelta": -src_n,
        "sourceDescriptors": 0, "compiledDescriptors": 0,
        "firstMismatch": {"gate": "LEGACY_COMPILE_PASS", "category": err, "path": "compiler", "detail": err},
        "gateMismatches": {}, "gateMismatchCounts": {}, "mismatchCategoryCounts": {},
        "sourceValidity": {"controlFlowValid": None, "controlFlowViolation": None, "danglingReferenceCount": 0},
        "inventory": [],
    }
    res["gates"]["BPLIST_OR_XML_DECODE_PASS"] = "PASS"
    res["gates"]["APPLE_RUNTIME_EXECUTED"] = "BLOCKED_DEVICE"
    return res, ""


def family(identifier: Optional[str]) -> str:
    return (identifier or "<none>")


def main():
    repo_root = script_dir.parents[1]
    default_audit_root = repo_root.parent / "cherri-shortcut-audit-20261110"
    parser = argparse.ArgumentParser(description="Full 38-Shortcut Audit Runner (P10R8 hardened)")
    parser.add_argument("--binary", type=pathlib.Path, default=default_audit_root / "cherri-p10r8-audit.exe")
    parser.add_argument("--input-dir", type=pathlib.Path, default=default_audit_root / "input")
    parser.add_argument("--work-dir", type=pathlib.Path, default=default_audit_root / "p10r8-work")
    parser.add_argument("--private-out", type=pathlib.Path, default=default_audit_root / "p10r8-private" / "p10r8-audit-results.json")
    parser.add_argument("--sanitized-out", type=pathlib.Path, default=repo_root / "docs" / "apple-runtime" / "p10r8" / "p10r8-38-sanitized-matrix.json")
    parser.add_argument("--require-gates", default="", help="Comma-separated gate names that must be 38/38 PASS for exit 0")
    parser.add_argument("--timeout", type=int, default=120)
    args = parser.parse_args()

    if not args.binary.exists():
        print(f"Error: Binary {args.binary} not found", file=sys.stderr)
        sys.exit(2)
    inventory_errors = check_corpus_inventory(args.input_dir)
    if inventory_errors:
        print("CORPUS INVENTORY FAILED (fail closed):", ", ".join(inventory_errors), file=sys.stderr)
        sys.exit(2)

    args.work_dir.mkdir(parents=True, exist_ok=True)
    args.private_out.parent.mkdir(parents=True, exist_ok=True)
    args.sanitized_out.parent.mkdir(parents=True, exist_ok=True)

    binary_sha = sha256_file(args.binary)
    ginfo = git_info(repo_root)
    print(f"[{HARNESS_VERSION}] compiler {args.binary.name} sha256={binary_sha}")
    print(f"source commit {ginfo['commit']} dirty={ginfo['dirty']}")

    private_results: List[Dict[str, Any]] = []
    sanitized_matrix: List[Dict[str, Any]] = []
    total_source_actions = total_compiled_actions = 0
    gate_counts = {g: 0 for g in oracle.GATES}
    ext_counts = {g: 0 for g in oracle.EXTENDED_GATES}
    family_added: collections.Counter = collections.Counter()
    family_removed: collections.Counter = collections.Counter()
    v2_gate_counts = {"V2_COMPILE_PASS": 0, "STRICT_ACTION_SEQUENCE_PASS": 0, "FULL_WORKFLOW_SEMANTIC_PASS": 0}
    runtime_errors: List[str] = []

    for sid in EXPECTED_SPECIMEN_IDS:
        splist = args.input_dir / f"{sid}.plist"
        sjson = args.input_dir / f"{sid}.json"
        src_plist_sha, src_json_sha = sha256_file(splist), sha256_file(sjson)
        with open(splist, "rb") as f:
            src_doc = plistlib.load(f)
        json_doc, json_ok = load_json_strict(sjson)
        src_count = len(src_doc.get("WFWorkflowActions", []))
        total_source_actions += src_count

        wd = args.work_dir
        leg_cherri, v2_cherri = wd / f"{sid}_legacy.cherri", wd / f"{sid}_v2.cherri"
        leg_sc, v2_sc = wd / f"{sid}_legacy.shortcut", wd / f"{sid}_v2.shortcut"

        # Import (decompile). Every output path is cleared first.
        imp_leg = run_compile_step([str(args.binary), "import", str(splist), "--legacy", "-o", str(leg_cherri)], leg_cherri, args.timeout)
        imp_v2 = run_compile_step([str(args.binary), "import", str(splist), "-o", str(v2_cherri)], v2_cherri, args.timeout)
        # Compile only when this run's import produced a fresh source.
        if imp_leg["gate"] == "PASS":
            cmp_leg = run_compile_step([str(args.binary), str(leg_cherri), "--legacy", "--skip-sign", "-o", str(leg_sc)], leg_sc, args.timeout)
        else:
            if leg_sc.exists():
                leg_sc.unlink()
            cmp_leg = {"gate": "FAIL", "rc": -1, "outputPresent": False, "stderrTail": "NOT_RUN"}
        if imp_v2["gate"] == "PASS":
            cmp_v2 = run_compile_step([str(args.binary), "compile", str(v2_cherri), "--skip-sign", "-o", str(v2_sc)], v2_sc, args.timeout)
        else:
            if v2_sc.exists():
                v2_sc.unlink()
            cmp_v2 = {"gate": "FAIL", "rc": -1, "outputPresent": False, "stderrTail": "NOT_RUN"}

        eval_res, compiled_sha = evaluate_compiled(src_doc, json_doc, json_ok, leg_sc, cmp_leg)
        v2_res, v2_sha = evaluate_compiled(src_doc, json_doc, json_ok, v2_sc, cmp_v2)

        eval_res["gates"]["DECOMPILE_PASS"] = imp_leg["gate"]
        eval_res["gates"]["LEGACY_COMPILE_PASS"] = cmp_leg["gate"]
        eval_res["gates"]["V2_COMPILE_PASS"] = cmp_v2["gate"]
        runtime_errors += check_runtime_gate(eval_res["gates"])

        compiled_count = eval_res["compiledActions"]
        total_compiled_actions += compiled_count
        delta = compiled_count - src_count
        for g, val in eval_res["gates"].items():
            gate_counts[g] += val == "PASS"
        for g, val in eval_res["extendedGates"].items():
            ext_counts[g] += val == "PASS"
        for row in eval_res["inventory"]:
            if row["op"] == "added":
                family_added[family(row["identifier"])] += 1
            elif row["op"] == "removed":
                family_removed[family(row["identifier"])] += 1
        v2_gate_counts["V2_COMPILE_PASS"] += cmp_v2["gate"] == "PASS"
        v2_gate_counts["STRICT_ACTION_SEQUENCE_PASS"] += v2_res["gates"]["STRICT_ACTION_SEQUENCE_PASS"] == "PASS"
        v2_gate_counts["FULL_WORKFLOW_SEMANTIC_PASS"] += v2_res["gates"]["FULL_WORKFLOW_SEMANTIC_PASS"] == "PASS"

        timing = {"import_legacy_sec": imp_leg.get("seconds"), "import_v2_sec": imp_v2.get("seconds"),
                  "compile_legacy_sec": cmp_leg.get("seconds"), "compile_v2_sec": cmp_v2.get("seconds")}
        private_results.append({
            "specimenId": sid,
            "sourcePlistSha256": src_plist_sha, "sourceJsonSha256": src_json_sha, "compiledShortcutSha256": compiled_sha,
            "v2CompiledShortcutSha256": v2_sha,
            "sourceActions": src_count, "compiledActions": compiled_count, "actionDelta": delta,
            "sourceDescriptors": eval_res["sourceDescriptors"], "compiledDescriptors": eval_res["compiledDescriptors"],
            "gates": eval_res["gates"], "extendedGates": eval_res["extendedGates"],
            "firstMismatch": eval_res["firstMismatch"], "gateMismatches": eval_res["gateMismatches"],
            "gateMismatchCounts": eval_res["gateMismatchCounts"], "mismatchCategoryCounts": eval_res["mismatchCategoryCounts"],
            "sourceValidity": eval_res["sourceValidity"], "v2": {"gates": v2_res["gates"], "firstMismatch": v2_res["firstMismatch"],
                                                                  "actions": v2_res["compiledActions"]},
            "steps": {"importLegacy": imp_leg, "importV2": imp_v2, "compileLegacy": cmp_leg, "compileV2": cmp_v2},
            "timing": timing,
        })
        sanitized_matrix.append({
            "specimenId": sid, "sourceSha256": src_plist_sha, "compiledSha256": compiled_sha,
            "sourceActions": src_count, "compiledActions": compiled_count, "actionDelta": delta,
            "sourceDescriptors": eval_res["sourceDescriptors"], "compiledDescriptors": eval_res["compiledDescriptors"],
            "gates": eval_res["gates"], "extendedGates": eval_res["extendedGates"],
            "firstMismatch": sanitize_mismatch(eval_res["firstMismatch"]),
            "gateFirstMismatch": {g: sanitize_mismatch(m) for g, m in eval_res["gateMismatches"].items()},
            "gateMismatchCounts": eval_res["gateMismatchCounts"],
            "sourceValidity": eval_res["sourceValidity"],
            "v2": {"compile": cmp_v2["gate"], "strictSequence": v2_res["gates"]["STRICT_ACTION_SEQUENCE_PASS"],
                   "full": v2_res["gates"]["FULL_WORKFLOW_SEMANTIC_PASS"],
                   "firstMismatch": sanitize_mismatch(v2_res["firstMismatch"])},
            "timing": timing,
        })
        fm = eval_res["firstMismatch"]
        desc = f"({fm['category']} at {oracle.redact_path(fm['path'])})" if fm else ""
        print(f"Specimen {sid}: {src_count}->{compiled_count} ({delta:+d}) Seq={eval_res['gates']['STRICT_ACTION_SEQUENCE_PASS']} "
              f"Full={eval_res['gates']['FULL_WORKFLOW_SEMANTIC_PASS']} V2={cmp_v2['gate']} {desc}")

    now = datetime.datetime.now(datetime.timezone.utc).isoformat(timespec="seconds")
    n = len(sanitized_matrix)
    summary = {
        "harnessVersion": HARNESS_VERSION, "oracleVersion": oracle.ORACLE_VERSION,
        "verificationMode": "PRIVATE_CORPUS_VERIFIED_LOCALLY", "verifiedAtUtc": now,
        "compilerGitCommit": ginfo["commit"], "compilerWorktreeDirty": ginfo["dirty"],
        "compilerBinarySha256": binary_sha, "os": platform.platform(),
        "expectedSpecimenIds": EXPECTED_SPECIMEN_IDS, "totalSpecimens": n,
        "totalSourceActions": total_source_actions, "totalCompiledActions": total_compiled_actions,
        "totalActionDelta": total_compiled_actions - total_source_actions,
        "strictActionSequencePassCount": gate_counts["STRICT_ACTION_SEQUENCE_PASS"],
        "fullWorkflowSemanticPassCount": gate_counts["FULL_WORKFLOW_SEMANTIC_PASS"],
        "gateSummary": gate_counts, "extendedGateSummary": ext_counts,
        "familyAdded": dict(family_added), "familyRemoved": dict(family_removed),
        "v2Summary": v2_gate_counts,
        "sourceInvalidSpecimens": [r["specimenId"] for r in private_results if r["sourceValidity"]["controlFlowValid"] is False],
        "runtimeIntegrityErrors": runtime_errors,
        "matrix": sanitized_matrix,
    }
    with open(args.private_out, "w", encoding="utf-8") as f:
        json.dump({**summary, "matrix": private_results}, f, indent=2)
    with open(args.sanitized_out, "w", encoding="utf-8") as f:
        json.dump(summary, f, indent=2)

    print("\n================== SUMMARY ==================")
    print(f"Source actions {total_source_actions:,} -> compiled {total_compiled_actions:,} (delta {total_compiled_actions - total_source_actions:+d})")
    for g, c in gate_counts.items():
        print(f"{g:<36}: {c:2d} / {n}")
    for g, c in ext_counts.items():
        print(f"{g:<36}: {c:2d} / {n}")
    print(f"V2: {v2_gate_counts}")
    print("added:", dict(family_added), "removed:", dict(family_removed))

    required = [g for g in args.require_gates.split(",") if g]
    unmet = [g for g in required if gate_counts.get(g, ext_counts.get(g, 0)) != n]
    if n != 38 or runtime_errors:
        sys.exit(3)
    if unmet:
        print("REQUIRED GATES NOT MET:", ", ".join(unmet), file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
