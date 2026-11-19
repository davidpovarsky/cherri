#!/usr/bin/env python3
"""Execute the existing WorkflowKit REQUIRED gate with an isolated negative control.

Only /usr/bin/osascript on Darwin can execute this producer. Unit fixtures validate
protocol and exit handling separately and cannot emit native acceptance records.
"""
import argparse
import copy
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import subprocess
import sys

REPO = Path(__file__).resolve().parents[2]
GATE = REPO / "tools/shortcut-runtime/jxa/verify_encodings.js"
EXPECTATIONS = REPO / "tools/shortcut-runtime/required-encoding-expectations.json"
PIN = "ee52c32937053268e4a035eba4b372fe77ec6411"
NEGATIVE_ID = "plainString.WFStringParameterState.string"
FORMS = {"string", "number", "bool", "attachment(variable)", "attachment(output)", "tokenString", "wrappedSubject"}
PUBLIC_ROOT = "artifacts/backend-recovery/ci-artifacts/required-encoding-gate"


class GateFailure(Exception):
    def __init__(self, code, detail, status="NOT_RUN"):
        super().__init__(detail)
        self.code, self.status = code, status


def digest(data):
    return hashlib.sha256(data).hexdigest()


def read_json(path):
    def unique(pairs):
        value = {}
        for key, item in pairs:
            if key in value:
                raise ValueError("duplicate JSON key")
            value[key] = item
        return value
    return json.loads(path.read_bytes(), object_pairs_hook=unique)


def write_json(path, value):
    path.write_bytes((json.dumps(value, indent=2, ensure_ascii=False) + "\n").encode("utf-8"))


def git(*args, cwd=REPO):
    result = subprocess.run(["git", "-C", str(cwd), *args], capture_output=True, timeout=30)
    if result.returncode:
        raise GateFailure("E_ENCODING_PROVENANCE", "Git source provenance could not be verified")
    return result.stdout


def expectation_rows(value):
    if not isinstance(value, dict) or value.get("schemaVersion") != 1 or not isinstance(value.get("REQUIRED"), list):
        raise GateFailure("E_ENCODING_INPUT", "Missing schemaVersion 1 REQUIRED expectation list")
    rows = value["REQUIRED"]
    if not rows or len(rows) > 256:
        raise GateFailure("E_ENCODING_INPUT", "Empty or oversized REQUIRED expectation list")
    seen, pairs = set(), set()
    for row in rows:
        if (not isinstance(row, dict) or set(row) != {"id", "kind", "stateClass", "form", "expect"}
                or not all(isinstance(v, str) for v in row.values())
                or not re.fullmatch(r"[A-Za-z][A-Za-z0-9_.()-]{0,191}", row["id"])
                or not re.fullmatch(r"[A-Za-z][A-Za-z0-9_]{0,31}", row["kind"])
                or not re.fullmatch(r"[A-Za-z_][A-Za-z0-9_]{0,127}", row["stateClass"])
                or row["form"] not in FORMS or row["expect"] not in {"accept", "reject"}
                or row["id"] in seen or (row["stateClass"], row["form"]) in pairs):
            raise GateFailure("E_ENCODING_INPUT", "Malformed or duplicate REQUIRED expectation")
        seen.add(row["id"])
        pairs.add((row["stateClass"], row["form"]))
    return rows


def validate_phase(process, report, expected_rows, negative=False):
    """Validate every required observation against the independently authored EXPECT."""
    if not isinstance(report, dict) or report.get("schemaVersion") != 2 or report.get("engineLoaded") is not True:
        raise GateFailure("E_ENCODING_PROTOCOL", "No complete loaded-engine report")
    forms = report.get("forms")
    if (not isinstance(forms, list) or not all(isinstance(item, str) for item in forms)
            or len(forms) != len(FORMS) or set(forms) != FORMS or report.get("requiredExpectations") != expected_rows):
        raise GateFailure("E_ENCODING_PROTOCOL", "Report forms or applied EXPECT do not match the requested input")
    discovery = report.get("discovery", {})
    if (not isinstance(discovery, dict) or any(type(discovery.get(k)) is not int or discovery[k] <= 0
           for k in ("parameterStateClasses", "appIntentStateClasses", "uniqueStateClasses"))):
        raise GateFailure("E_ENCODING_PROTOCOL", "Engine report has no actual discovery counts")
    results = report.get("results")
    if not isinstance(results, dict) or len(results) != discovery["uniqueStateClasses"]:
        raise GateFailure("E_ENCODING_PROTOCOL", "Discovered classes and per-form matrix differ")
    for state_class, matrix in results.items():
        if not isinstance(state_class, str) or not re.fullmatch(r"[A-Za-z_][A-Za-z0-9_]*", state_class):
            raise GateFailure("E_ENCODING_PROTOCOL", "Invalid observed state-class identity")
        if matrix == "class not present":
            continue
        if (not isinstance(matrix, dict) or set(matrix) != FORMS
                or any(not isinstance(value, str) or not (value in {"roundtrip", "rejected", "threw: native parameter-state exception"}
                    or value.startswith("accepted, reserialized as ")) for value in matrix.values())):
            raise GateFailure("E_ENCODING_PROTOCOL", "Incomplete or invalid actual per-form matrix")
    observations = report.get("requiredResults")
    if not isinstance(observations, list) or len(observations) != len(expected_rows):
        raise GateFailure("E_ENCODING_PROTOCOL", "Incomplete REQUIRED observations")
    failures = []
    for expected, observed in zip(expected_rows, observations):
        if not isinstance(observed, dict) or any(observed.get(k) != expected[k] for k in ("id", "kind", "stateClass", "form")):
            raise GateFailure("E_ENCODING_PROTOCOL", "Required case identity changed")
        matrix = results.get(expected["stateClass"])
        outcome = matrix.get(expected["form"]) if isinstance(matrix, dict) else matrix
        actual = ("accept" if isinstance(outcome, str) and
                  (outcome == "roundtrip" or outcome.startswith("accepted, reserialized as "))
                  else "reject" if outcome == "rejected" else "unavailable")
        passed = actual == expected["expect"]
        if (observed.get("expected") != expected["expect"] or observed.get("actual") != actual
                or observed.get("observation") != outcome or observed.get("passed") is not passed):
            raise GateFailure("E_ENCODING_PROTOCOL", "Required verdict differs from its actual per-form observation")
        if not passed:
            failures.append({"code": "E_REQUIRED_ENCODING_ENGINE" if actual == "unavailable" else "E_REQUIRED_ENCODING_EXPECTATION",
                             "id": expected["id"], "expected": expected["expect"], "actual": actual})
    if report.get("failures") != failures:
        raise GateFailure("E_ENCODING_PROTOCOL", "Failure list is not the observed required-case result")
    if negative:
        wanted = [{"code": "E_REQUIRED_ENCODING_EXPECTATION", "id": NEGATIVE_ID, "expected": "reject", "actual": "accept"}]
        if (type(process.get("exitCode")) is not int or process["exitCode"] != 1
                or process.get("negativeDiagnosticObserved") is not True
                or report.get("status") != "FAIL" or failures != wanted):
            raise GateFailure("E_ENCODING_NEGATIVE_CONTROL", "Corrupt EXPECT did not fail solely at its exact required case", "FAIL")
    elif type(process.get("exitCode")) is not int or process["exitCode"] != 0 or report.get("status") != "PASS" or failures:
        raise GateFailure("E_ENCODING_REQUIRED", "Correct REQUIRED set did not pass the actual engine", "FAIL")


def run_phase(out, name, runner, parameters, table, override=None):
    output = out / (name + ".json")
    command = ["/usr/bin/osascript", "-l", "JavaScript", str(runner), str(parameters), str(table),
               str(output), str(EXPECTATIONS)]
    if override is not None:
        command.append(str(override))
    try:
        result = subprocess.run(command, capture_output=True, timeout=180)
        code, stdout, stderr = result.returncode, result.stdout, result.stderr
    except subprocess.TimeoutExpired as error:
        code, stdout, stderr = 124, error.stdout or b"", error.stderr or b""
    except OSError:
        code, stdout, stderr = None, b"", b"E_ENCODING_ENGINE_EXECUTION\n"
    (out / (name + ".stdout.log")).write_bytes(stdout)
    (out / (name + ".stderr.log")).write_bytes(stderr)
    process = {"phase": name, "command": command, "exitCode": code,
               "stdoutSha256": digest(stdout), "stderrSha256": digest(stderr),
               "negativeDiagnosticObserved": b"E_REQUIRED_ENCODING_EXPECTATION" in stderr and NEGATIVE_ID.encode() in stderr,
               "reportSha256": digest(output.read_bytes()) if output.is_file() else None}
    write_json(out / (name + ".process.json"), process)
    try:
        report = read_json(output)
    except (OSError, ValueError):
        report = None
    return process, report


def produce(args):
    out = Path(args.output).resolve()
    if out.exists():
        raise GateFailure("E_ENCODING_OUTPUT_EXISTS", "Choose a fresh output directory; existing evidence is preserved")
    out.mkdir(parents=True)
    summary = {"schemaVersion": 1, "status": "NOT_RUN", "implementationSha": args.source_sha,
               "runId": args.run_id, "appleEngineExecuted": False, "shortcutRuntimeExecuted": False,
               "mockEngineAllowed": False, "phases": [], "failures": [], "records": []}
    before = {}
    try:
        if platform.system() != "Darwin" or not Path("/usr/bin/osascript").is_file():
            raise GateFailure("E_ENCODING_PLATFORM", "A real macOS osascript WorkflowKit engine is required")
        if not re.fullmatch(r"[0-9a-f]{40}", args.source_sha) or git("rev-parse", "HEAD").decode().strip() != args.source_sha:
            raise GateFailure("E_ENCODING_PROVENANCE", "Exact source SHA does not match the checkout")
        source_paths = [GATE, EXPECTATIONS, Path(__file__).resolve()]
        if git("status", "--porcelain", "--", *[str(p.relative_to(REPO)) for p in source_paths]).strip():
            raise GateFailure("E_ENCODING_PROVENANCE", "Gate source must be committed and unchanged")
        for path in source_paths:
            if path.read_bytes() != git("show", args.source_sha + ":" + path.relative_to(REPO).as_posix()):
                raise GateFailure("E_ENCODING_PROVENANCE", "Gate bytes differ from their committed source blob")
        prelude = Path(args.prelude).resolve(strict=True)
        if git("rev-parse", "HEAD", cwd=prelude.parent).decode().strip() != PIN:
            raise GateFailure("E_ENCODING_PROVENANCE", "ShortcutKit checkout does not match its pinned commit")
        pinned = git("show", PIN + ":tools/jxa-prelude.js", cwd=prelude.parent)
        if prelude.read_bytes() != pinned:
            raise GateFailure("E_ENCODING_PROVENANCE", "Prelude differs from the pinned Git blob")
        parameters, table = Path(args.parameter_encodings).resolve(strict=True), Path(args.encoding_table).resolve(strict=True)
        inputs = source_paths + [prelude, parameters, table]
        if any(out == p or out in p.parents for p in inputs):
            raise GateFailure("E_ENCODING_OUTPUT_OVERLAP", "Output must not contain a source or input path")
        before = {str(p): digest(p.read_bytes()) for p in inputs}
        summary["inputs"] = before
        summary["preludeCommit"] = PIN
        summary["host"] = {"system": platform.system(), "machine": platform.machine(),
                           "macOSVersion": subprocess.check_output(["/usr/bin/sw_vers", "-productVersion"], text=True).strip()}
        template = read_json(EXPECTATIONS)
        required = expectation_rows(template)
        target = next((row for row in required if row["id"] == NEGATIVE_ID), None)
        if target is None or target["expect"] != "accept" or not any(row["expect"] == "reject" for row in required):
            raise GateFailure("E_ENCODING_INPUT", "Independent positive target and expected rejection are both required")
        runner = out / "verify.js"
        runner.write_bytes(pinned + b"\n" + GATE.read_bytes())
        baseline_process, baseline = run_phase(out, "positive", runner, parameters, table)
        summary["phases"].append(baseline_process)
        summary["appleEngineExecuted"] = bool(baseline and baseline.get("engineLoaded") is True)
        validate_phase(baseline_process, baseline, required)

        corrupt = copy.deepcopy(template)
        next(row for row in corrupt["REQUIRED"] if row["id"] == NEGATIVE_ID)["expect"] = "reject"
        override = out / "isolated-corrupt-expectations.json"
        write_json(override, corrupt)
        summary["isolatedMutation"] = {"id": NEGATIVE_ID, "field": "expect", "before": "accept", "after": "reject"}
        negative_process, negative = run_phase(out, "corrupt", runner, parameters, table, override)
        summary["phases"].append(negative_process)
        restored_process, restored = run_phase(out, "restored", runner, parameters, table)
        summary["phases"].append(restored_process)
        validate_phase(negative_process, negative, corrupt["REQUIRED"], negative=True)
        validate_phase(restored_process, restored, required)
        if any(digest(Path(path).read_bytes()) != value for path, value in before.items()):
            raise GateFailure("E_ENCODING_INPUT_CHANGED", "Source or input changed during isolated verification")
        summary.update(status="PASS", requiredCases=len(required), expectedRejections=sum(row["expect"] == "reject" for row in required),
                       inputBytesUnchanged=True, negativeControlExit=negative_process["exitCode"])
    except GateFailure as error:
        summary["status"] = error.status
        summary["failures"].append({"code": error.code, "detail": str(error)})
    except (OSError, ValueError, subprocess.SubprocessError) as error:
        summary["status"] = "NOT_RUN"
        summary["failures"].append({"code": "E_ENCODING_INFRASTRUCTURE", "detail": type(error).__name__})
    write_json(out / "required-encoding-gate.json", summary)
    records = []
    # A locally simulated result can never satisfy this condition. Actual CI run
    # identity and source/artifact binding are validated again by the aggregator.
    ci = (os.environ.get("GITHUB_ACTIONS") == "true" and os.environ.get("GITHUB_SHA") == args.source_sha
          and os.environ.get("GITHUB_RUN_ID") == args.run_id and re.fullmatch(r"[0-9]+", args.run_id))
    if summary["status"] == "PASS" and ci:
        artifact_names = ["required-encoding-gate.json"]
        for phase in ("positive", "corrupt", "restored"):
            artifact_names.extend(phase + suffix for suffix in (".json", ".process.json", ".stdout.log", ".stderr.log"))
        artifacts = [{"path": PUBLIC_ROOT + "/" + name, "sha256": digest((out / name).read_bytes()), "run_id": args.run_id}
                     for name in artifact_names]
        for requirement in ("CI01", "RP47"):
            records.append({
                "evidence_id": "ev-required-encoding-" + requirement.lower(),
                "requirements": [requirement], "tier": "negative-ci", "implementation_sha": args.source_sha,
                "test_id": "tools/shortcut-runtime:required-encoding-" + requirement.lower(),
                "exit_code": 0, "result": "passed", "fixture_sha256": digest(EXPECTATIONS.read_bytes()),
                "command": ["python3", "tools/shortcut-runtime/required_encoding_gate.py", "--source-sha", args.source_sha, "--run-id", args.run_id],
                "assertions": {"required_expect_nonzero": "true", "exact_corruption_rejected": "true",
                               "expected_rejection_passed": "true", "per_form_outcomes_preserved": "true",
                               "restored_required_set_passed": "true", "apple_engine_executed": "true"},
                "artifacts": artifacts,
                "detail": "Real WorkflowKit parameter-state process: correct set, one corrupted REQUIRED EXPECT, restored set; no Shortcut runtime execution",
            })
    write_json(out / "required-encoding-evidence.json", {"records": records})
    print(json.dumps({"status": summary["status"], "appleEngineExecuted": summary["appleEngineExecuted"],
                      "phases": len(summary["phases"]), "requiredCases": summary.get("requiredCases", 0),
                      "formalRecords": len(records), "failures": summary["failures"]}))
    return 0 if summary["status"] == "PASS" else 2 if summary["status"] == "FAIL" else 3


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--prelude", required=True)
    parser.add_argument("--parameter-encodings", default=str(REPO / "docs/apple-runtime/parameter-encodings.json"))
    parser.add_argument("--encoding-table", default=str(REPO / "docs/apple-runtime/encoding-table.json"))
    parser.add_argument("--output", required=True)
    parser.add_argument("--source-sha", required=True)
    parser.add_argument("--run-id", default=os.environ.get("GITHUB_RUN_ID", "LOCAL_NOT_CI"))
    args = parser.parse_args()
    try:
        return produce(args)
    except (GateFailure, OSError) as error:
        print(json.dumps({"status": "NOT_RUN", "code": getattr(error, "code", "E_ENCODING_OUTPUT"), "records": []}))
        return 3


if __name__ == "__main__":
    sys.exit(main())
