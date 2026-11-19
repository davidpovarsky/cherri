#!/usr/bin/env python3
"""Run the bounded production state/race scenarios and retain their raw evidence."""
from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import signal
import subprocess
import sys
import time
from datetime import datetime, timezone

TESTS = [
    "TestBackendRecovery06MixedEntryPoints",
    "TestBackendRecovery06EmitterErrorAndCancellation",
    "TestBackendRecovery06EmitterErrorAndCancellation/emitter_error",
    "TestBackendRecovery06EmitterErrorAndCancellation/cancellation",
]
MARKER = "P15_STATE_EXERCISE="
COMMAND = ["go", "test", "-race", "-count=1", "-json", "-timeout=8m", "-run", "^(TestBackendRecovery06MixedEntryPoints|TestBackendRecovery06EmitterErrorAndCancellation)$", "."]
SOURCE_PATHS = [
    "compiler_state.go", "language_backend_session.go", "language_backend_adapter.go",
    "language_emit_adapter.go", "language_registry_adapter.go", "mobile_compiler.go", "ios_bridge.go",
    "internal/language/service/service.go", "internal/language/service/registry_lifetime.go",
    "internal/language/schema/registry_clone.go", "internal/language/assets/cache.go",
    "p15_backend_state_concurrency_test.go", "p15_state_race_enabled_test.go", "p15_state_race_disabled_test.go",
    "tools/language-acceptance/run_state_race.py", ".github/workflows/p10r15-state-race.yml",
]


def sha(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def write_json(path: Path, value: object) -> None:
    path.write_text(json.dumps(value, ensure_ascii=True, indent=2) + "\n", encoding="utf-8", newline="\n")


def captured(command: list[str], root: Path) -> str:
    result = subprocess.run(command, cwd=root, check=True, capture_output=True, text=True, encoding="utf-8", timeout=30)
    return result.stdout.strip()


def source_state(root: Path) -> tuple[str, bool]:
    return captured(["git", "rev-parse", "HEAD"], root), not captured(["git", "status", "--porcelain", "--untracked-files=all"], root)


def exact_value(actual: object, expected: object) -> bool:
    if type(actual) is not type(expected):
        return False
    if isinstance(expected, dict):
        return actual.keys() == expected.keys() and all(exact_value(actual[key], value) for key, value in expected.items())
    if isinstance(expected, list):
        return len(actual) == len(expected) and all(exact_value(a, b) for a, b in zip(actual, expected))
    return actual == expected


def evaluate(raw: bytes, stderr: bytes, code: int | None, timed_out: bool, environment: dict, clean_before: bool, clean_after: bool) -> dict:
    issues: list[str] = []
    events: list[dict] = []
    outputs: dict[str, str] = {}
    try:
        for line in raw.decode("utf-8").splitlines():
            event = json.loads(line)
            if not isinstance(event, dict):
                raise ValueError("event is not an object")
            events.append(event)
            if event.get("Action") == "output" and isinstance(event.get("Test"), str):
                outputs[event["Test"]] = outputs.get(event["Test"], "") + event.get("Output", "")
    except (ValueError, TypeError, UnicodeError):
        issues.append("malformed Go JSON events")
    if code != 0 or timed_out:
        issues.append("race process failed or timed out")
    if not clean_before or not clean_after:
        issues.append("source was not clean and unchanged throughout execution")
    if environment.get("GOOS") != "linux" or environment.get("CGO_ENABLED") != "1":
        issues.append("required Linux CGO race configuration missing")
    if b"WARNING: DATA RACE" in raw or b"WARNING: DATA RACE" in stderr:
        issues.append("race detector reported a data race")
    for action in ("fail", "skip"):
        if any(event.get("Action") == action for event in events):
            issues.append("Go reported " + action)
    for name in TESTS:
        for action in ("run", "pass"):
            if sum(event.get("Action") == action and event.get("Test") == name for event in events) != 1:
                issues.append(f"required exact test {name} lacks one {action} event")
    package_passes = [event for event in events if event.get("Action") == "pass" and not event.get("Test")]
    if len(package_passes) != 1 or package_passes[0].get("Package") != "github.com/electrikmilk/cherri":
        issues.append("terminal production package PASS missing")
    observations = {}
    expected = {
        TESTS[0]: {"race_instrumented": True, "sequential_rounds": 3, "concurrent_workers": 8, "concurrent_rounds": 8, "completed_cases": 64},
        TESTS[1]: {"race_instrumented": True, "sequential_rounds": 0, "concurrent_workers": 0, "concurrent_rounds": 0, "completed_cases": 0, "emitter_faults": ["emitter_error", "cancellation"]},
    }
    for name, want in expected.items():
        found = []
        for line in outputs.get(name, "").splitlines():
            if MARKER not in line:
                continue
            try:
                found.append(json.loads(line.split(MARKER, 1)[1]))
            except ValueError:
                issues.append("malformed completion witness for " + name)
        if len(found) != 1 or not exact_value(found[0], want):
            issues.append("missing, duplicated, uninstrumented or incomplete execution witness for " + name)
        observations[name] = found
    return {"status": "PASSED" if not issues else "FAILED", "issues": issues, "required_tests": TESTS, "observations": observations}


def run(root: Path, output: Path) -> int:
    if output.is_relative_to(root):
        raise ValueError("evidence output must be outside the source checkout")
    output.mkdir(parents=True, exist_ok=False)
    revision, clean_before = source_state(root)
    if revision != os.environ.get("GITHUB_SHA") or not clean_before:
        raise ValueError("checkout does not match the clean GitHub source SHA")
    environment = json.loads(captured(["go", "env", "-json", "GOOS", "GOARCH", "CGO_ENABLED", "CC", "GOVERSION"], root))
    environment.update({"platform": platform.platform(), "python": platform.python_version(), "go_version": captured(["go", "version"], root), "gcc_version": captured(["gcc", "-dumpfullversion"], root), "GOMAXPROCS": os.environ.get("GOMAXPROCS", ""), "GOFLAGS": os.environ.get("GOFLAGS", ""), "GORACE": os.environ.get("GORACE", "")})
    write_json(output / "environment.json", environment)
    source_files = [{"path": path, "sha256": sha((root / path).read_bytes())} for path in SOURCE_PATHS]
    write_json(output / "source-files.json", source_files)
    started_utc = datetime.now(timezone.utc).isoformat()
    started = time.monotonic()
    timed_out = False
    with (output / "go-test.jsonl").open("wb") as stdout, (output / "go-test.stderr").open("wb") as stderr:
        process = subprocess.Popen(COMMAND, cwd=root, stdout=stdout, stderr=stderr, start_new_session=True)
        try:
            process.wait(timeout=600)
        except subprocess.TimeoutExpired:
            timed_out = True
            os.killpg(process.pid, signal.SIGTERM)
            try:
                process.wait(timeout=3)
            except subprocess.TimeoutExpired:
                os.killpg(process.pid, signal.SIGKILL)
                process.wait()
    elapsed = time.monotonic() - started
    final_revision, clean_after = source_state(root)
    clean_after = clean_after and final_revision == revision
    raw = (output / "go-test.jsonl").read_bytes()
    errors = (output / "go-test.stderr").read_bytes()
    result = evaluate(raw, errors, process.returncode, timed_out, environment, clean_before, clean_after)
    receipt = {
        "format_version": 1, "source_commit": revision, "source_clean_before": clean_before, "source_clean_after": clean_after,
        "repository": os.environ.get("GITHUB_REPOSITORY", ""), "run_id": os.environ.get("GITHUB_RUN_ID", ""), "run_attempt": os.environ.get("GITHUB_RUN_ATTEMPT", ""),
        "command": COMMAND, "started_utc": started_utc, "finished_utc": datetime.now(timezone.utc).isoformat(), "elapsed_seconds": elapsed,
        "exit_code": process.returncode, "timed_out": timed_out,
        "raw_sha256": sha(raw), "raw_bytes": len(raw), "stderr_sha256": sha(errors), "stderr_bytes": len(errors),
        "environment_sha256": sha((output / "environment.json").read_bytes()), "source_files_sha256": sha((output / "source-files.json").read_bytes()),
        **result,
        "formal_requirements_proved": [],
        "scope": "Actual bounded race scenarios only. Formal BRG06/RP51 intake and RP51 benchmark evidence remain separate required gates.",
    }
    write_json(output / "race-receipt.json", receipt)
    print(json.dumps({"status": result["status"], "source_commit": revision, "exit_code": process.returncode, "elapsed_seconds": elapsed, "issues": result["issues"]}))
    return 0 if result["status"] == "PASSED" else 1


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    root = Path(__file__).resolve().parents[2]
    output = args.output.resolve()
    try:
        return run(root, output)
    except Exception as error:
        if output.exists() and output.is_dir() and not output.is_relative_to(root):
            prior = output / "race-receipt.json"
            if prior.is_file():
                previous = prior.read_bytes()
                backup = output / ("prior-race-receipt-" + sha(previous) + ".json")
                if not backup.exists():
                    backup.write_bytes(previous)
            write_json(prior, {"status": "HARNESS_FAILED", "type": type(error).__name__, "error": str(error), "formal_requirements_proved": []})
        print("State/race harness failed: " + str(error), file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
