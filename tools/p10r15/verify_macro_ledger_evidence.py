#!/usr/bin/env python3
"""Mutate copied real macro-ledger evidence and assert actual verifier exit codes."""
import argparse
import copy
import hashlib
import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import tempfile

sys.dont_write_bytecode = True

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo", type=Path, required=True)
    parser.add_argument("--ledger-dir", type=Path, required=True)
    parser.add_argument("--report", type=Path, required=True)
    options = parser.parse_args()
    producer = Path(__file__).with_name("generate_macro_contract_coverage.py").resolve()
    spec = importlib.util.spec_from_file_location("macro_ledger", producer)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    evidence = options.ledger_dir / "macro-contract-coverage.json"
    original = json.loads(evidence.read_text(encoding="utf-8"))
    original_md = (options.ledger_dir / "macro-contract-coverage.md").read_bytes()
    rows = []
    with tempfile.TemporaryDirectory(prefix="cherri-macro-ledger-faults-") as temp:
        directory = Path(temp)
        target = directory / "macro-contract-coverage.json"
        markdown = directory / "macro-contract-coverage.md"

        def store(value, md=original_md):
            target.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
            markdown.write_bytes(md)

        def check(name, expected=None):
            process = subprocess.run([sys.executable, str(producer), "--repo", str(options.repo.resolve()),
                "--out-dir", str(directory), "--verify"], text=True, encoding="utf-8",
                errors="replace", capture_output=True, timeout=60)
            log = process.stdout + process.stderr
            ok = process.returncode == 0 if expected is None else process.returncode != 0 and expected in log
            rows.append({"case": name, "exitCode": process.returncode, "expectedDiagnostic": expected,
                         "verdict": "PASS" if ok else "FAIL", "diagnosticTail": log[-700:]})
            print(json.dumps({k:rows[-1][k] for k in ("case","exitCode","verdict")}), flush=True)

        store(original)
        check("actual_healthy_evidence")
        faults = (
            ("stale_source", "STALE_MACRO_SOURCE_EVIDENCE"),
            ("edited_observation", "EDITED_MACRO_OBSERVATIONS"),
            ("missing_child", "MISSING_OR_DUPLICATE_ASANA_CHILD"),
            ("premature_closure", "STRUCTURAL_EVIDENCE_CANNOT_CLOSE_CHILD"),
            ("missing_compile", "MISSING_CONTROLLED_COMPILE"),
            ("missing_mutations", "MACRO_EXECUTION_COUNT_MISMATCH"),
            ("false_action_count", "MACRO_NATIVE_ACTION_COUNT_MISMATCH"),
            ("false_roundtrip", "MACRO_LOSSLESS_ROUNDTRIP_FAILED"),
            ("stale_markdown", "STALE_MACRO_MARKDOWN"),
        )
        for name, expected in faults:
            altered = copy.deepcopy(original)
            md = original_md
            if name == "stale_source":
                altered["sourceFingerprint"] = "0" * 64
            elif name in ("edited_observation", "false_action_count"):
                altered["compiles"][0]["nativeActionCount"] += 1
            elif name == "missing_child":
                altered["requirements"].pop()
            elif name == "premature_closure":
                altered["requirements"][0]["status"] = "DONE"
            elif name == "missing_compile":
                altered["compiles"].pop()
            elif name == "missing_mutations":
                altered["causalMutations"] = []
            elif name == "false_roundtrip":
                altered["compiles"][0]["decompile"]["nativeActionsExact"] = False
            else:
                md += b"\nInvalid appended output\n"
            if name != "edited_observation":
                altered["observationsSHA256"] = module.observations_digest(altered)
            store(altered, md)
            check(name, expected)
            store(original)
            check("restore_after_" + name)
        # The original nineteen actual-process controls above remain intact.
        # Each new fault changes copied real label evidence, rehashes that
        # observation, and must reach its specific guard before markdown checks.
        label_faults = (
            ("label_missing_component", "MACRO_LABEL_COMPONENT_MISSING",
             lambda e: e.pop("labelControls")),
            ("label_wrong_source_revision", "MACRO_LABEL_SOURCE_JOIN_MISMATCH",
             lambda e: e["labelControls"].__setitem__("implementationSha", "0" * 40)),
            ("label_wrong_clean_status", "MACRO_LABEL_SOURCE_JOIN_MISMATCH",
             lambda e: e["labelControls"].__setitem__("cleanCheckout", not e["labelControls"]["cleanCheckout"])),
            ("label_wrong_compiler", "MACRO_LABEL_COMPILER_JOIN_MISMATCH",
             lambda e: e["labelControls"].__setitem__("compilerSha256", "0" * 64)),
            ("label_wrong_helper", "MACRO_LABEL_HELPER_JOIN_MISMATCH",
             lambda e: e["labelControls"].__setitem__("helperSha256", "0" * 64)),
            ("label_missing_input_binding", "MACRO_LABEL_SOURCE_INPUT_MISMATCH",
             lambda e: e["labelControls"]["sourceInputs"].pop(module.LABEL_HELPER)),
            ("label_missing_compile", "MACRO_LABEL_CASE_INVENTORY_MISMATCH",
             lambda e: e["labelControls"]["fixtures"].pop()),
            ("label_duplicate_compile", "MACRO_LABEL_CASE_INVENTORY_MISMATCH",
             lambda e: e["labelControls"]["fixtures"].__setitem__(1, copy.deepcopy(e["labelControls"]["fixtures"][0]))),
            ("label_wrong_fixture_source", "MACRO_LABEL_SOURCE_MISMATCH",
             lambda e: e["labelControls"]["fixtures"][0]["source"].__setitem__("sourceSha256", "0" * 64)),
            ("label_missing_unsigned_hash", "MACRO_LABEL_UNSIGNED_HASH_MISSING",
             lambda e: e["labelControls"]["fixtures"][0].__setitem__("unsignedPlistSha256", "")),
            ("label_wrong_graph_source", "MACRO_LABEL_GRAPH_CONTRACT_MISMATCH",
             lambda e: e["labelControls"]["fixtures"][0]["graph"].__setitem__("sourceSha256", "0" * 64)),
            ("label_false_action_count", "MACRO_LABEL_GRAPH_CONTRACT_MISMATCH",
             lambda e: e["labelControls"]["fixtures"][0].__setitem__("nativeActionCount", e["labelControls"]["fixtures"][0]["nativeActionCount"] + 1)),
            ("label_missing_partition_index", "MACRO_LABEL_GRAPH_CONTRACT_MISMATCH",
             lambda e: e["labelControls"]["fixtures"][0]["graph"]["instrumentation"]["sourceIndices"].pop()),
            ("label_missing_mutation", "MACRO_LABEL_MUTATION_INVENTORY_MISMATCH",
             lambda e: e["labelControls"]["fixtures"][0]["causalMutations"]["rejected"].pop()),
            ("label_renamed_mutation", "MACRO_LABEL_MUTATION_INVENTORY_MISMATCH",
             lambda e: e["labelControls"]["fixtures"][0]["causalMutations"]["rejected"].__setitem__(0, "unexecuted_replacement")),
            ("label_false_total", "MACRO_LABEL_EXECUTION_COUNT_MISMATCH",
             lambda e: e["labelControls"].__setitem__("rejectedMutationCount", 313)),
            ("label_false_native_credit", "MACRO_LABEL_SCOPE_MISMATCH",
             lambda e: e["labelControls"].__setitem__("nativeExecution", "PASS")),
            ("label_false_formal_credit", "MACRO_LABEL_SCOPE_MISMATCH",
             lambda e: e["labelControls"].__setitem__("formalAcceptanceGranted", True)),
            ("label_orphan_case_link", "MACRO_LABEL_REQUIREMENT_LINK_MISMATCH",
             lambda e: next(r for r in e["requirements"] if r["feature"] == "label")["executedCases"].__setitem__(0, "TestDebugLabelsPublicLiteralOmissionContract")),
            ("label_stale_implementation_state", "MACRO_LABEL_REQUIREMENT_LINK_MISMATCH",
             lambda e: next(r for r in e["requirements"] if r["feature"] == "label").__setitem__("implementationState", "DECLARED_BUT_NOT_EXERCISED")),
            ("label_wrong_summary", "MACRO_LABEL_SUMMARY_MISMATCH",
             lambda e: e["summary"].__setitem__("labelCompileChecks", 11)),
        )
        for name, expected, mutate in label_faults:
            altered = copy.deepcopy(original)
            mutate(altered)
            altered["observationsSHA256"] = module.observations_digest(altered)
            store(altered)
            check(name, expected)
            store(original)
            check("restore_after_" + name)
    summary = {"checks":len(rows), "passed":sum(r["verdict"]=="PASS" for r in rows),
               "failed":sum(r["verdict"]=="FAIL" for r in rows)}
    report = {"sourceSha":original["sourceSha"], "sourceFingerprint":original["sourceFingerprint"],
              "ledgerSHA256":hashlib.sha256(evidence.read_bytes()).hexdigest(),
              "generatorSHA256":original["generatorSHA256"], "scope":"COPIED_REAL_EVIDENCE_ACTUAL_VERIFIER_EXITS",
              "results":rows, "summary":summary}
    options.report.parent.mkdir(parents=True,exist_ok=True)
    options.report.write_text(json.dumps(report,indent=2)+"\n",encoding="utf-8")
    print(json.dumps(summary),flush=True)
    return 0 if summary["failed"]==0 else 1

if __name__ == "__main__":
    raise SystemExit(main())
