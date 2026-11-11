#!/usr/bin/env python3
"""
tools/p10r7/run_38_audit.py
Executes the full 38-shortcut corpus audit pipeline for Cherri P10R7.

Steps per specimen (001-038):
1. Verifies input files (.plist and .json) and computes SHA256.
2. Evaluates RAW_JSON_DECODE_PASS and BPLIST_OR_XML_DECODE_PASS.
3. Decompiles with --legacy and -o <work_dir>/<id>_legacy.cherri.
4. Decompiles with V2 (no --legacy) and -o <work_dir>/<id>_v2.cherri.
5. Compiles decompiled legacy cherri with --legacy --skip-sign -o <work_dir>/<id>_legacy.shortcut.
6. Compiles decompiled v2 cherri with compile --skip-sign -o <work_dir>/<id>_v2.shortcut.
7. Evaluates strict fidelity with independent_fidelity_oracle.py.
8. Writes private detailed results to <audit-root>/p10r7-private/p10r7-audit-results.json (outside git).
9. Writes sanitized matrix to docs/apple-runtime/p10r7/p10r7-38-sanitized-matrix.json (zero PII, zero private strings/URLs).
"""

import argparse
import hashlib
import json
import os
import pathlib
import plistlib
import subprocess
import sys
import time
from typing import Any, Dict, List, Optional

# Import the independent oracle
script_dir = pathlib.Path(__file__).resolve().parent
sys.path.insert(0, str(script_dir))
import independent_fidelity_oracle as oracle

HARNESS_VERSION = "p10r7-audit-harness-20261110.1"

def sha256_file(p: pathlib.Path) -> str:
    h = hashlib.sha256()
    with open(p, "rb") as f:
        while chunk := f.read(65536):
            h.update(chunk)
    return h.hexdigest()

def run_cmd(cmd: List[str], timeout: int = 60) -> tuple[int, str, str, float]:
    t0 = time.time()
    try:
        res = subprocess.run(
            cmd,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            timeout=timeout,
            text=True,
            encoding="utf-8",
            errors="replace"
        )
        dt = time.time() - t0
        return res.returncode, res.stdout, res.stderr, dt
    except subprocess.TimeoutExpired:
        return -1, "", "TIMEOUT", time.time() - t0
    except Exception as e:
        return -2, "", str(e), time.time() - t0

def sanitize_mismatch(mismatch: Optional[Dict[str, Any]]) -> Optional[Dict[str, Any]]:
    if not mismatch:
        return None
    # Strictly strip any detail that might contain private values
    return {
        "gate": mismatch.get("gate", ""),
        "category": mismatch.get("category", ""),
        "path": mismatch.get("path", "")
    }

def main():
    repo_root = script_dir.parents[1]
    default_audit_root = repo_root.parent / "cherri-shortcut-audit-20261110"
    parser = argparse.ArgumentParser(description="P10R7 Full 38-Shortcut Audit Runner")
    parser.add_argument("--binary", type=pathlib.Path, default=default_audit_root / "cherri-p10r7-audit.exe")
    parser.add_argument("--input-dir", type=pathlib.Path, default=default_audit_root / "input")
    parser.add_argument("--work-dir", type=pathlib.Path, default=default_audit_root / "p10r7-work")
    parser.add_argument("--private-out", type=pathlib.Path, default=default_audit_root / "p10r7-private" / "p10r7-audit-results.json")
    parser.add_argument("--sanitized-out", type=pathlib.Path, default=repo_root / "docs" / "apple-runtime" / "p10r7" / "p10r7-38-sanitized-matrix.json")
    args = parser.parse_args()

    if not args.binary.exists():
        print(f"Error: Binary {args.binary} not found", file=sys.stderr)
        sys.exit(1)

    args.work_dir.mkdir(parents=True, exist_ok=True)
    args.private_out.parent.mkdir(parents=True, exist_ok=True)
    args.sanitized_out.parent.mkdir(parents=True, exist_ok=True)

    binary_sha = sha256_file(args.binary)
    print(f"[{HARNESS_VERSION}] Starting audit of 38 shortcuts...")
    print(f"Compiler binary: {args.binary} (SHA256: {binary_sha})")
    print(f"Input directory: {args.input_dir}")
    print(f"Work directory:  {args.work_dir}")

    private_results = []
    sanitized_matrix = []

    total_source_actions = 0
    total_compiled_actions = 0
    gate_counts = {g: 0 for g in oracle.GATES}
    exact_sequence_count = 0
    exact_semantic_count = 0

    for i in range(1, 39):
        sid = f"{i:03d}"
        splist = args.input_dir / f"{sid}.plist"
        sjson = args.input_dir / f"{sid}.json"

        if not splist.exists():
            print(f"Specimen {sid}: MISSING PLIST -> SKIPPED")
            continue

        src_plist_sha = sha256_file(splist)
        src_json_sha = sha256_file(sjson) if sjson.exists() else None

        # 1. Parse source plist
        with open(splist, "rb") as f:
            src_doc = plistlib.load(f)
        src_actions = src_doc.get("WFWorkflowActions", [])
        src_count = len(src_actions)
        total_source_actions += src_count

        # Parse source JSON if present
        json_doc = None
        if sjson.exists():
            try:
                with open(sjson, "r", encoding="utf-8-sig") as f:
                    json_doc = json.load(f)
            except Exception:
                json_doc = None

        # Paths for generated files
        decomp_legacy_cherri = args.work_dir / f"{sid}_legacy.cherri"
        decomp_v2_cherri = args.work_dir / f"{sid}_v2.cherri"
        compiled_legacy_shortcut = args.work_dir / f"{sid}_legacy.shortcut"
        compiled_v2_shortcut = args.work_dir / f"{sid}_v2.shortcut"

        # 2. Decompile Legacy
        rc_dec_leg, out_dec_leg, err_dec_leg, dt_dec_leg = run_cmd([
            str(args.binary), "import", str(splist), "--legacy", "-o", str(decomp_legacy_cherri)
        ], timeout=60)

        # 3. Decompile V2
        rc_dec_v2, out_dec_v2, err_dec_v2, dt_dec_v2 = run_cmd([
            str(args.binary), "import", str(splist), "-o", str(decomp_v2_cherri)
        ], timeout=60)

        # 4. Compile Legacy
        rc_comp_leg, out_comp_leg, err_comp_leg, dt_comp_leg = -1, "", "NOT_RUN", 0.0
        if rc_dec_leg == 0 and decomp_legacy_cherri.exists() and decomp_legacy_cherri.stat().st_size > 0:
            rc_comp_leg, out_comp_leg, err_comp_leg, dt_comp_leg = run_cmd([
                str(args.binary), str(decomp_legacy_cherri), "--legacy", "--skip-sign", "-o", str(compiled_legacy_shortcut)
            ], timeout=60)

        # 5. Compile V2
        rc_comp_v2, out_comp_v2, err_comp_v2, dt_comp_v2 = -1, "", "NOT_RUN", 0.0
        if rc_dec_v2 == 0 and decomp_v2_cherri.exists() and decomp_v2_cherri.stat().st_size > 0:
            rc_comp_v2, out_comp_v2, err_comp_v2, dt_comp_v2 = run_cmd([
                str(args.binary), "compile", str(decomp_v2_cherri), "--skip-sign", "-o", str(compiled_v2_shortcut)
            ], timeout=60)

        # Evaluate with Oracle on the rebuilt legacy shortcut
        if rc_comp_leg == 0 and compiled_legacy_shortcut.exists() and compiled_legacy_shortcut.stat().st_size > 0:
            with open(compiled_legacy_shortcut, "rb") as f:
                reb_doc = plistlib.load(f)
            eval_res = oracle.evaluate_shortcut_fidelity(src_doc, reb_doc, json_doc)
            compiled_sha = sha256_file(compiled_legacy_shortcut)
            compiled_count = eval_res["compiledActions"]
            total_compiled_actions += compiled_count
        else:
            eval_res = {
                "oracleVersion": oracle.ORACLE_VERSION,
                "gates": {g: "FAIL" for g in oracle.GATES},
                "sourceActions": src_count,
                "compiledActions": 0,
                "actionDelta": -src_count,
                "sourceDescriptors": 0,
                "compiledDescriptors": 0,
                "firstMismatch": {
                    "gate": "LEGACY_COMPILE_PASS",
                    "category": "COMPILATION_ERROR",
                    "path": "compiler",
                    "detail": "Failed to compile legacy cherri"
                }
            }
            eval_res["gates"]["RAW_JSON_DECODE_PASS"] = "PASS" if json_doc is not None else "NOT_APPLICABLE"
            eval_res["gates"]["BPLIST_OR_XML_DECODE_PASS"] = "PASS"
            eval_res["gates"]["APPLE_RUNTIME_EXECUTED"] = "BLOCKED_DEVICE"
            compiled_sha = ""
            compiled_count = 0

        # Update execution gates based on actual runs
        eval_res["gates"]["DECOMPILE_PASS"] = "PASS" if rc_dec_leg == 0 else "FAIL"
        eval_res["gates"]["LEGACY_COMPILE_PASS"] = "PASS" if rc_comp_leg == 0 else "FAIL"
        eval_res["gates"]["V2_COMPILE_PASS"] = "PASS" if rc_comp_v2 == 0 else "FAIL"

        delta = compiled_count - src_count

        for g, val in eval_res["gates"].items():
            if val == "PASS":
                gate_counts[g] += 1

        if eval_res["gates"]["STRICT_ACTION_SEQUENCE_PASS"] == "PASS":
            exact_sequence_count += 1
        if eval_res["gates"]["FULL_WORKFLOW_SEMANTIC_PASS"] == "PASS":
            exact_semantic_count += 1

        timing_record = {
            "decompile_legacy_sec": round(dt_dec_leg, 3),
            "decompile_v2_sec": round(dt_dec_v2, 3),
            "compile_legacy_sec": round(dt_comp_leg, 3),
            "compile_v2_sec": round(dt_comp_v2, 3),
        }

        # Build private record
        priv_rec = {
            "specimenId": sid,
            "sourcePlistSha256": src_plist_sha,
            "sourceJsonSha256": src_json_sha,
            "compiledShortcutSha256": compiled_sha,
            "sourceActions": src_count,
            "compiledActions": compiled_count,
            "actionDelta": delta,
            "sourceDescriptors": eval_res["sourceDescriptors"],
            "compiledDescriptors": eval_res["compiledDescriptors"],
            "gates": eval_res["gates"],
            "firstMismatch": eval_res["firstMismatch"],
            "timing": timing_record,
            "errors": {
                "decompile_legacy": (err_dec_leg or out_dec_leg)[-300:] if rc_dec_leg != 0 else "",
                "compile_legacy": (err_comp_leg or out_comp_leg)[-300:] if rc_comp_leg != 0 else "",
                "decompile_v2": (err_dec_v2 or out_dec_v2)[-300:] if rc_dec_v2 != 0 else "",
                "compile_v2": (err_comp_v2 or out_comp_v2)[-300:] if rc_comp_v2 != 0 else "",
            }
        }
        private_results.append(priv_rec)

        # Build sanitized record (ZERO PII)
        sanitized_rec = {
            "specimenId": sid,
            "sourceSha256": src_plist_sha,
            "compiledSha256": compiled_sha,
            "sourceActions": src_count,
            "compiledActions": compiled_count,
            "actionDelta": delta,
            "sourceDescriptors": eval_res["sourceDescriptors"],
            "compiledDescriptors": eval_res["compiledDescriptors"],
            "gates": eval_res["gates"],
            "firstMismatch": sanitize_mismatch(eval_res["firstMismatch"]),
            "timing": timing_record,
        }
        sanitized_matrix.append(sanitized_rec)

        mismatch_desc = ""
        if eval_res["firstMismatch"]:
            mismatch_desc = f"({eval_res['firstMismatch']['category']} at {eval_res['firstMismatch']['path']})"
        print(f"Specimen {sid}: Actions={src_count}->{compiled_count} (Delta={delta:+d}) | Seq={eval_res['gates']['STRICT_ACTION_SEQUENCE_PASS']} Sem={eval_res['gates']['FULL_WORKFLOW_SEMANTIC_PASS']} {mismatch_desc}")

    # Write private results
    with open(args.private_out, "w", encoding="utf-8") as f:
        json.dump(private_results, f, indent=2)
    print(f"\nPrivate detailed results saved to: {args.private_out}")

    # Write sanitized matrix
    sanitized_doc = {
        "harnessVersion": HARNESS_VERSION,
        "oracleVersion": oracle.ORACLE_VERSION,
        "compilerBinarySha256": binary_sha,
        "totalSpecimens": len(sanitized_matrix),
        "totalSourceActions": total_source_actions,
        "totalCompiledActions": total_compiled_actions,
        "totalActionDelta": total_compiled_actions - total_source_actions,
        "strictActionSequencePassCount": exact_sequence_count,
        "fullWorkflowSemanticPassCount": exact_semantic_count,
        "gateSummary": gate_counts,
        "matrix": sanitized_matrix
    }
    with open(args.sanitized_out, "w", encoding="utf-8") as f:
        json.dump(sanitized_doc, f, indent=2)
    print(f"Sanitized matrix saved to: {args.sanitized_out}")

    print("\n================== SUMMARY ==================")
    print(f"Total Shortcuts:               {len(sanitized_matrix)}")
    print(f"Total Source Actions:          {total_source_actions:,}")
    print(f"Total Compiled Actions:        {total_compiled_actions:,}")
    print(f"Total Delta:                   {total_compiled_actions - total_source_actions:+d} (was +6,703 in P10R6!)")
    print(f"Strict Sequence Match:         {exact_sequence_count} / {len(sanitized_matrix)}")
    print(f"Full Semantic Match:           {exact_semantic_count} / {len(sanitized_matrix)}")
    print("---------------------------------------------")
    for g, count in gate_counts.items():
        print(f"{g:<35}: {count:2d} / {len(sanitized_matrix)}")
    print("=============================================")

if __name__ == "__main__":
    main()
