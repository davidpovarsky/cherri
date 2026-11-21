#!/usr/bin/env python3
"""Execute actual editor publication methods with controlled async adapters.

This is a portable Swift control-flow probe. It does not execute SwiftUI, the
Go compiler bridge, Apple Shortcuts, signing, or any real network/file adapter.
"""
from __future__ import annotations
import argparse
import hashlib
import json
import math
import os
from pathlib import Path
import shutil
import signal
import subprocess
import sys
import time

CONTROL_NAMES = {
    "current_success", "late_success_after_newer", "late_analysis_after_newer",
    "late_generic_error_after_newer", "canonical_unicode_is_not_same_snapshot",
    "cancelled_same_source_does_not_publish", "old_completion_cannot_clear_new_busy",
    "current_error_is_not_suppressed", "current_completion", "different_source",
    "different_caret", "canonical_unicode", "cancelled_completion",
    "selection_range_does_not_call_completion",
}

def sha(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()

def region(text: str, start: str, end: str) -> str:
    if text.count(start) != 1 or text.count(end) != 1:
        raise ValueError("ambiguous or missing actual method boundary")
    at, stop = text.index(start), text.index(end)
    if stop <= at:
        raise ValueError("reversed actual method boundary")
    result = text[at:stop].strip()
    if any(line.strip().startswith("#") for line in result.splitlines()):
        raise ValueError("conditional method ownership is not reviewed")
    return result

def generate(repo: Path) -> tuple[str, dict[str, bytes]]:
    paths = {
        "workspace": "ios/CherriApp/App/WorkspaceView.swift",
        "completion": "ios/CherriApp/Editor/CherriEditorView.swift",
        "request": "ios/CherriApp/Editor/CherriCompletionRequest.swift",
        "compiler_types": "ios/CherriApp/Compiler/CherriCompiler.swift",
        "template": "tools/p10r15/editor_publication_probe.swift",
    }
    files = {rel: (repo / rel).read_bytes() for rel in paths.values()}
    get = lambda name: files[paths[name]].decode("utf-8", errors="strict")
    build = region(get("workspace"), "    private func build(", "\n    @MainActor\n    private func refreshActionCatalog")
    completion = region(get("completion"), "    private func refreshContextualCompletions(", "\n    private func refreshDiagnostic")
    item = region(get("compiler_types"), "struct CherriCompletionItem:", "\nstruct CherriWirePosition:")
    template = get("template")
    for marker in ("__ACTUAL_BUILD_METHOD__", "__ACTUAL_COMPLETION_METHOD__"):
        if template.count(marker) != 1:
            raise ValueError("template must own exactly one method placeholder")
    generated = template.replace("__ACTUAL_BUILD_METHOD__", build.replace("private func ", "func ", 1))
    generated = generated.replace("__ACTUAL_COMPLETION_METHOD__", completion.replace("private func ", "func ", 1))
    generated = "import Foundation\n" + item + "\n" + generated
    owned = repo / "ios/CherriApp/Editor/CherriEditorBuildRequest.swift"
    if owned.exists():
        files[owned.relative_to(repo).as_posix()] = owned.read_bytes()
    return generated, files

def invoke(command: list[str], root: Path, output: Path, label: str, timeout: float) -> dict:
    began = time.monotonic()
    deadline = began + timeout
    timed_out = False
    cleanup = None
    with (output / (label + ".stdout")).open("wb") as stdout, (output / (label + ".stderr")).open("wb") as stderr:
        process = subprocess.Popen(command, cwd=root, stdout=stdout, stderr=stderr, start_new_session=os.name != "nt")
        try:
            process.wait(timeout=max(0.001, deadline - time.monotonic()))
        except subprocess.TimeoutExpired:
            timed_out = True
            if os.name == "nt":
                stopped = subprocess.run(["taskkill", "/PID", str(process.pid), "/T", "/F"], capture_output=True, timeout=15)
                cleanup = {"exit_code": stopped.returncode, "stdout_sha256": sha(stopped.stdout), "stderr_sha256": sha(stopped.stderr)}
            else:
                os.killpg(process.pid, signal.SIGKILL)
                cleanup = {"owned_process_group_killed": process.pid}
            process.wait(timeout=15)
    elapsed = time.monotonic() - began
    return {
        "command": command, "exit_code": process.returncode, "timed_out": timed_out,
        "timeout_seconds": timeout, "duration_seconds": elapsed, "cleanup": cleanup,
        "within_budget": math.isfinite(elapsed) and 0 <= elapsed <= timeout,
        "artifacts": [
            {"path": label + suffix, "bytes": (output / (label + suffix)).stat().st_size, "sha256": sha((output / (label + suffix)).read_bytes())}
            for suffix in (".stdout", ".stderr")
        ],
    }

def successful(process: dict) -> bool:
    elapsed, budget = process.get("duration_seconds"), process.get("timeout_seconds")
    return (
        type(process.get("exit_code")) is int and process["exit_code"] == 0
        and process.get("timed_out") is False and process.get("within_budget") is True
        and type(elapsed) in (int, float) and type(budget) in (int, float)
        and math.isfinite(elapsed) and math.isfinite(budget) and 0 <= elapsed <= budget and budget > 0
    )

def inspect_output(data: bytes) -> tuple[list[dict], dict]:
    rows = [json.loads(line) for line in data.decode("utf-8", errors="strict").splitlines() if line]
    controls = [row for row in rows if row.get("event") == "control"]
    summaries = [row for row in rows if row.get("event") == "summary"]
    if len(rows) != len(CONTROL_NAMES) + 1 or len(controls) != len(CONTROL_NAMES) or len(summaries) != 1:
        raise ValueError("missing, extra or duplicate actual probe events")
    names = [row.get("name") for row in controls]
    if len(set(names)) != len(names) or set(names) != CONTROL_NAMES:
        raise ValueError("actual control selection changed")
    for row in controls:
        if row.get("result") not in {"PASS", "FAIL"} or row.get("scope") != "controlled adapter; actual production method body" or not isinstance(row.get("observation"), dict):
            raise ValueError("invalid control event")
    summary = summaries[0]
    if type(summary.get("controls")) is not int or type(summary.get("failed")) is not int or summary.get("controls") != len(controls) or summary.get("failed") != sum(row["result"] == "FAIL" for row in controls) or summary.get("uiExecuted") is not False or summary.get("realCompilerExecuted") is not False:
        raise ValueError("actual summary does not match control evidence/scope")
    return controls, summary

def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--swiftc")
    args = parser.parse_args()
    repo, output = args.repo.resolve(), args.output.resolve()
    output.mkdir(parents=True, exist_ok=False)
    compiler = args.swiftc or shutil.which("swiftc")
    if not compiler:
        raise RuntimeError("supported Swift compiler is unavailable; no control executed")
    generated, files = generate(repo)
    source = output / "editor-publication-generated.swift"
    source.write_text(generated, encoding="utf-8", newline="\n")
    binary = output / ("editor-publication-probe.exe" if os.name == "nt" else "editor-publication-probe")
    command = [compiler, "-parse-as-library", str(source), str(repo / "ios/CherriApp/Editor/CherriCompletionRequest.swift")]
    optional = "ios/CherriApp/Editor/CherriEditorBuildRequest.swift"
    if optional in files:
        command.append(str(repo / optional))
    command += ["-o", str(binary)]
    compile_result = invoke(command, repo, output, "compile", 90)
    result = {
        "schema_version": 1, "scope": "portable Swift control-flow only, controlled backend adapters",
        "uiExecuted": False, "realCompilerExecuted": False, "formalRequirementsProved": [],
        "sources": [{"path": path, "bytes": len(data), "sha256": sha(data)} for path, data in sorted(files.items())],
        "generated": {"path": source.name, "bytes": len(source.read_bytes()), "sha256": sha(source.read_bytes())},
        "compile": compile_result, "execution": None, "controls": [], "summary": None,
    }
    if successful(compile_result):
        result["binary"] = {"bytes": binary.stat().st_size, "sha256": sha(binary.read_bytes())}
        result["execution"] = invoke([str(binary)], repo, output, "execute", 45)
        try:
            result["controls"], result["summary"] = inspect_output((output / "execute.stdout").read_bytes())
        except (ValueError, UnicodeError, json.JSONDecodeError) as error:
            result["intake_error"] = str(error)
    unchanged = all((repo / path).read_bytes() == data for path, data in files.items())
    result["sources_unchanged"] = unchanged
    result["passed"] = bool(
        successful(compile_result) and result["execution"] and successful(result["execution"])
        and result["summary"] and result["summary"]["failed"] == 0 and unchanged
    )
    (output / "receipt.json").write_text(json.dumps(result, indent=2) + "\n", encoding="utf-8", newline="\n")
    print(json.dumps({"passed": result["passed"], "compiled": successful(compile_result), "summary": result["summary"], "intake_error": result.get("intake_error"), "receipt": str(output / "receipt.json")}))
    return 0 if result["passed"] else 1

if __name__ == "__main__":
    sys.exit(main())
