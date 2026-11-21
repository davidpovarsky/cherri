#!/usr/bin/env python3
"""Fail closed before Xcode when receiver project preparation fails."""
import argparse
import json
import os
from pathlib import Path
import plistlib
import subprocess
import time

from simulator_session_recovery import require, run_process
import qa_terminal_receipts as terminal

PROJECT = "tools/shortcuts-simulator-qa/XcodeProject"
GENERATED = "CherriSimulatorQA.xcodeproj/project.pbxproj"
BUDGET_SECONDS = 240
STAGE_SECONDS = {"install": 180, "generate": 60}


def prepare(repo, root, expected, shell, *, runner=run_process, now=time.monotonic):
    repo, root = Path(repo), Path(root)
    project = terminal.contained(repo, PROJECT, exists=False)
    require(project.is_dir(), "PROJECT_DIRECTORY_REQUIRED")
    target = terminal.contained(project, GENERATED, exists=False)
    require(not target.parent.exists(), "GENERATED_PROJECT_ALREADY_PRESENT")
    private = terminal.contained(root, "builds/project-prerequisite", exists=False)
    private.mkdir()
    started = now()
    receipt = {"schemaVersion": "1.0", **expected, "budgetMs": BUDGET_SECONDS * 1000,
               "status": "FAILED", "commands": [], "projectSha256": None,
               "formalAcceptanceGranted": False}
    old_cwd = Path.cwd()
    try:
        os.chdir(project)
        for stage, script in (
            ("install", 'if ! command -v xcodegen >/dev/null 2>&1; then HOMEBREW_NO_AUTO_UPDATE=1 HOMEBREW_NO_INSTALL_CLEANUP=1 brew install xcodegen; fi'),
            ("generate", "xcodegen generate"),
        ):
            allowance = max(0, min(STAGE_SECONDS[stage], BUDGET_SECONDS - (now() - started)))
            out, err = private / (stage + ".stdout"), private / (stage + ".stderr")
            with out.open("xb") as stdout, err.open("xb") as stderr:
                observed = runner([shell, "-c", script], allowance, output=stdout, error_output=stderr)
            record = {"stage": stage, "allowanceMs": round(allowance * 1000), **observed,
                      "stdout": terminal.file_binding(root, out.relative_to(root).as_posix()),
                      "stderr": terminal.file_binding(root, err.relative_to(root).as_posix())}
            receipt["commands"].append(record)
            if observed["result"] != "PASS" or now() - started >= BUDGET_SECONDS:
                return 70, receipt
        target = terminal.contained(project, GENERATED)
        require(target.is_file() and not target.is_symlink() and target.stat().st_size > 0,
                "FRESH_PROJECT_REQUIRED")
        with terminal.contained(project, "ReceiverApp/Info.plist").open("rb") as stream:
            info = plistlib.load(stream)
        schemes = [s for group in info.get("CFBundleURLTypes", [])
                   for s in group.get("CFBundleURLSchemes", [])]
        require("cherriqa" in schemes, "RECEIVER_SCHEME_REQUIRED")
        receipt["projectSha256"] = terminal.sha(target.read_bytes())
        receipt["status"] = "PREPARED"
        return 0, receipt
    except (OSError, ValueError, TypeError, AttributeError):
        return 70, receipt
    finally:
        os.chdir(old_cwd)
        receipt["elapsedMs"] = round((now() - started) * 1000)
        with (private / "receipt.json").open("xb") as stream:
            stream.write((json.dumps(receipt, sort_keys=True, indent=2) + "\n").encode())


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("repo", "root", "source-sha", "run-id", "fixture-id", "bash"):
        parser.add_argument("--" + name, required=True)
    args = parser.parse_args()
    terminal.verify_checkout(args.repo, args.source_sha)
    expected = terminal.context(args.root, args.source_sha, args.run_id, args.fixture_id)
    # The generated project must come from this exact reviewed spec.
    relative = PROJECT + "/project.yml"
    original = subprocess.run(["git", "--no-replace-objects", "show", args.source_sha + ":" + relative],
                              cwd=args.repo, capture_output=True, timeout=10, check=True).stdout
    require(terminal.contained(args.repo, relative).read_bytes() == original, "PROJECT_SPEC_SOURCE")
    code, receipt = prepare(args.repo, args.root, expected, args.bash)
    print(json.dumps({"projectPrerequisiteStatus": receipt["status"],
                      "stages": [{"stage": x["stage"], "result": x["result"],
                                  "exitCode": x["exitCode"]} for x in receipt["commands"]],
                      "formalAcceptanceGranted": False}, sort_keys=True))
    return code


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (OSError, ValueError, subprocess.SubprocessError):
        print("PROJECT_PREREQUISITE_REJECTED")
        raise SystemExit(70)
