#!/usr/bin/env python3
"""Export only the immutable179 runtime clauses exercised by the reviewed PoC.

The complete historical diagnostic set remains a separate, unchanged artifact.
Its old requirement labels are diagnostic metadata, never formal authority.
This exporter does not run Shortcuts and cannot upgrade a failed observation.
"""
import argparse
import copy
import json
import os
from pathlib import Path
import subprocess
import sys

from runtime_poc_publication import (
    CI_PREFIX, FORMAL_FILES, SOURCE_HASHES, SOURCE_RECEIPT, PublicationBlocked,
    read_contained, read_text_data, require, sha, verify_sources,
)

RAW_IDENTITIES = {
    "ev-runtime-smoke": "smoke",
    "ev-runtime-variables": "variables",
    "ev-runtime-unicode": "unicode",
    "ev-runtime-indexing": "indexing",
    "ev-runtime-numbers": "numbers",
    "ev-runtime-conditionals": "conditionals",
    "ev-runtime-loops": "nested-loops",
    "ev-runtime-functions": "functions",
    "ev-runtime-dict-approach-a": "dict-approach-a",
    "ev-runtime-dict-approach-b-let": "dict-approach-b-let",
    "ev-runtime-dict-approach-b-var": "dict-approach-b-var",
    "ev-runtime-dict-approach-c": "dict-approach-c",
    "ev-runtime-dict-approach-d": "dict-approach-d",
    "ev-runtime-menu-structural": "menu-structural",
    "ev-runtime-menu-alpha": "menu-alpha",
    "ev-runtime-menu-beta": "menu-beta",
    "ev-runtime-legacy-if": "legacy-if",
}

# These are runtime facets only. All other contract tiers remain mandatory.
# In particular, recursion does not replace FN03's illegal-capture checks.
FORMAL_SCENARIOS = {
    "ev-runtime-variables": (
        ("V01", "V03"),
        {"variable_result": ("VAR", "updated"),
         "captured_before": ("CAPTURED_BEFORE", "initial"),
         "num_captured": ("NUM_CAPTURED", "1"),
         "clip_captured": ("CLIP_CAPTURED", "clip_initial")},
        "Observed mutable Number/Text capture and clipboard capture before later writes.",
    ),
    "ev-runtime-indexing": (
        ("L02", "L04"),
        {"index_trace": ("IDX", "alpha:gamma:::::beta")},
        "Observed first/last/empty/negative/out-of-range and guarded dynamic List indexing.",
    ),
    "ev-runtime-numbers": (
        ("N01",),
        {"math_trace": ("MATH", "2.5:1:11:0:1:0")},
        "Observed fractional division 5/2=2.5 in the computed numeric trace.",
    ),
    "ev-runtime-functions": (
        ("FN01", "FN02", "FN03"),
        {"function_result": ("FUNCTION", "21"),
         "caller_continuation": ("FN_CONT", "22"),
         "defaults_explicit": ("DEFAULTS_EXPLICIT", "0:0::"),
         "defaults_omitted": ("DEFAULTS_OMITTED", "1:9:def:"),
         "recursion_fact3": ("FACT", "6"),
         "void_output": ("VOID_OUT", ""),
         "partial_envelope_safe": ("ENV_SAFE", "multiply")},
        "Observed function return and caller continuation, false/zero/empty Text presence defaults, and terminating factorial recursion. Additional recorded values do not establish caller-input, structured-default, or main-return requirements.",
    ),
}


def _clipboard_values(root):
    data = read_contained(root, "clipboard-result.txt")
    text = read_text_data(data, "clipboard-result.txt")
    lines = text.splitlines()
    values = {}
    for line in lines[1:]:
        require("=" in line, "POC_CLIPBOARD_ROW_INVALID")
        key, value = line.split("=", 1)
        require(key and key not in values, "POC_CLIPBOARD_KEY_DUPLICATE")
        values[key] = value
    return lines[0] if lines else "", values


def _artifacts(root, record, run_id):
    artifacts = record.get("artifacts")
    require(isinstance(artifacts, list) and len(artifacts) == len(FORMAL_FILES),
            "POC_DIAGNOSTIC_ARTIFACT_SET_INCOMPLETE")
    seen = set()
    for artifact in artifacts:
        require(isinstance(artifact, dict), "POC_DIAGNOSTIC_ARTIFACT_INVALID")
        path = artifact.get("path")
        require(isinstance(path, str) and path.startswith(CI_PREFIX),
                "POC_DIAGNOSTIC_ARTIFACT_PREFIX")
        name = path[len(CI_PREFIX):]
        require(name in FORMAL_FILES and name not in seen,
                "POC_DIAGNOSTIC_ARTIFACT_UNKNOWN_OR_DUPLICATE")
        seen.add(name)
        data = read_contained(root, name)
        require(artifact.get("run_id") == run_id and artifact.get("sha256") == sha(data),
                "POC_DIAGNOSTIC_ARTIFACT_IDENTITY")
        if name.endswith(".cherri"):
            require(sha(data) == SOURCE_HASHES[Path(name).stem], "POC_DIAGNOSTIC_SOURCE_COPY")
        if name.endswith(".shortcut"):
            require(data.startswith(b"AEA1"), "POC_DIAGNOSTIC_UNSIGNED_ARTIFACT")
        else:
            read_text_data(data, name)
    require(seen == FORMAL_FILES, "POC_DIAGNOSTIC_ARTIFACT_SET_INCOMPLETE")


def export_records(repo, root, diagnostics, commit, run_id, attempt):
    root = Path(root)
    source_receipt = verify_sources(repo, root, commit, run_id, attempt)
    committed_exporter = subprocess.check_output([
        "git", "-C", str(repo), "show",
        commit + ":tools/shortcuts-simulator-qa/runtime_poc_formal_evidence.py",
    ])
    require(committed_exporter == Path(__file__).read_bytes(), "POC_FORMAL_EXPORTER_SOURCE_MISMATCH")
    source_raw = read_contained(root, SOURCE_RECEIPT)
    require(read_text_data(source_raw, SOURCE_RECEIPT) == source_receipt,
            "POC_SOURCE_RECEIPT_MISMATCH")
    raw = read_contained(root, diagnostics)
    records = read_text_data(raw, diagnostics)
    require(isinstance(records, list) and len(records) == len(RAW_IDENTITIES)
            and all(isinstance(record, dict) for record in records),
            "POC_DIAGNOSTIC_SET_INCOMPLETE")
    by_id = {}
    for record in records:
        evidence_id = record.get("evidence_id")
        require(isinstance(evidence_id, str) and evidence_id in RAW_IDENTITIES
                and evidence_id not in by_id, "POC_DIAGNOSTIC_UNKNOWN_OR_DUPLICATE")
        require(record.get("test_id") == "ios27-runtime-poc:" + RAW_IDENTITIES[evidence_id]
                and record.get("implementation_sha") == commit
                and record.get("tier") == "ios-runtime", "POC_DIAGNOSTIC_IDENTITY")
        require(type(record.get("exit_code")) is int
                and type(record.get("runtime_pass")) is bool
                and type(record.get("structural_pass")) is bool
                and record.get("result") in ("passed", "failed"), "POC_DIAGNOSTIC_RESULT_INVALID")
        if record["result"] == "passed":
            require(record["exit_code"] == 0 and record["runtime_pass"]
                    and record["structural_pass"], "POC_DIAGNOSTIC_FALSE_SUCCESS")
        else:
            require(record["exit_code"] != 0 and not record["runtime_pass"],
                    "POC_DIAGNOSTIC_FAILED_RESULT_CONTRADICTORY")
        _artifacts(root, record, run_id)
        by_id[evidence_id] = record

    sentinel, values = _clipboard_values(root)
    formal = []
    for evidence_id, (requirements, expected, detail) in FORMAL_SCENARIOS.items():
        record = by_id[evidence_id]
        assertions = record.get("assertions")
        require(isinstance(assertions, dict) and set(assertions) == set(expected)
                and all(isinstance(value, str) for value in assertions.values()),
                "POC_FORMAL_ASSERTION_SCHEMA")
        for key, (clipboard_key, golden) in expected.items():
            require(assertions[key] == values.get(clipboard_key, ""),
                    "POC_FORMAL_ASSERTION_DETACHED_FROM_CLIPBOARD")
            if record["result"] == "passed":
                require(clipboard_key in values and sentinel == "CHERRI_IOS27_RUNTIME_OK"
                        and assertions[key] == golden,
                        "POC_FORMAL_SUCCESS_ORACLE_MISMATCH")
        output = copy.deepcopy(record)
        output["requirements"] = list(requirements)
        output["fixture_sha256"] = SOURCE_HASHES["CherriRuntimePOC"]
        output["detail"] = detail + " Runtime facet only; all immutable contract tiers remain required."
        output["assertions"]["diagnostics_sha256"] = sha(raw)
        output["assertions"]["source_receipt_sha256"] = sha(source_raw)
        output["assertions"]["run_id"] = run_id
        output["assertions"]["run_attempt"] = attempt
        formal.append(output)
    return formal


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo", type=Path, required=True)
    parser.add_argument("--root", type=Path, required=True)
    parser.add_argument("--diagnostics", default="runtime-diagnostics.json")
    parser.add_argument("--output", default="runtime-evidence.json")
    parser.add_argument("--commit", default=os.environ.get("GITHUB_SHA", ""))
    parser.add_argument("--run-id", default=os.environ.get("GITHUB_RUN_ID", ""))
    parser.add_argument("--attempt", default=os.environ.get("GITHUB_RUN_ATTEMPT", "1"))
    args = parser.parse_args(argv)
    try:
        require(args.output == "runtime-evidence.json"
                and args.diagnostics == "runtime-diagnostics.json", "POC_FORMAL_OUTPUT_PATH_INVALID")
        output = args.root / args.output
        require(not output.exists() and not output.is_symlink(), "POC_FORMAL_OUTPUT_MUST_BE_NEW")
        records = export_records(args.repo, args.root, args.diagnostics,
                                 args.commit, args.run_id, args.attempt)
        # Exclusive creation prevents a previous success from being silently reused.
        with output.open("xb") as stream:
            stream.write((json.dumps(records, indent=2) + "\n").encode("utf-8"))
        print(json.dumps({"formalRecords": len(records), "formalRequirements": 8,
                          "rawDiagnosticRecords": len(RAW_IDENTITIES),
                          "closureClaimed": False}))
    except (PublicationBlocked, OSError, ValueError, TypeError, KeyError, subprocess.SubprocessError):
        print("POC_FORMAL_EXPORT_BLOCKED", file=sys.stderr)
        return 2
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
