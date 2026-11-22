#!/usr/bin/env python3
"""Run every Go package once with a separate cumulative acceptance budget."""
from __future__ import annotations
import argparse
from collections import Counter
from datetime import datetime, timezone
import json
import os
from pathlib import Path
import subprocess
import sys
import time

ACCEPTANCE_PACKAGE = "github.com/electrikmilk/cherri/tools/language-acceptance"
SUBPROCESS_PARALLEL_ENV = "CHERRI_FULL_SUITE_SUBPROCESS_PARALLEL"

def execution_plan(packages):
    if not packages or any(not p or p.strip() != p for p in packages):
        raise ValueError("empty or malformed Go package inventory")
    if len(set(packages)) != len(packages) or packages.count(ACCEPTANCE_PACKAGE) != 1:
        raise ValueError("the exact acceptance package must occur once in a unique inventory")
    ordinary = [p for p in packages if p != ACCEPTANCE_PACKAGE]
    if not ordinary:
        raise ValueError("ordinary Go package inventory is empty")
    plan = [{"packages": group, "timeout": budget,
             "argv": ["go", "test", "-count=1", "-timeout=" + budget,
                      *(["-parallel=2"] if group == ordinary else []), "-v", *group]}
            for group, budget in [(ordinary, "15m"), ([ACCEPTANCE_PACKAGE], "35m")]]
    if Counter(p for group in plan for p in group["packages"]) != Counter(packages):
        raise ValueError("Go test plan lost or duplicated a package")
    return plan

def execution_environment(group):
    environment = os.environ.copy()
    environment.pop(SUBPROCESS_PARALLEL_ENV, None)
    limits = [arg for arg in group["argv"]
              if arg in ("-parallel", "-test.parallel")
              or arg.startswith(("-parallel=", "-test.parallel="))]
    if group["packages"] == [ACCEPTANCE_PACKAGE]:
        if limits:
            raise ValueError("acceptance execution must retain sequential producer policy")
    else:
        if ACCEPTANCE_PACKAGE in group["packages"] or limits != ["-parallel=2"]:
            raise ValueError("ordinary execution requires exactly -parallel=2")
        environment[SUBPROCESS_PARALLEL_ENV] = "2"
    return environment

def execute_plan(plan, repo, invoke=subprocess.run):
    results = []
    for group in plan:
        print("GO_SUITE_EXECUTE " + json.dumps(group), flush=True)
        started = time.monotonic()
        result = invoke(group["argv"], cwd=repo, check=False,
                        env=execution_environment(group))
        results.append({**group, "exit_code": result.returncode,
                        "elapsed_seconds": round(time.monotonic() - started, 3)})
    return results

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--report", type=Path, required=True)
    args = parser.parse_args()
    repo = Path(__file__).resolve().parents[2]
    def capture(argv):
        return subprocess.run(argv, cwd=repo, check=True, capture_output=True, text=True).stdout
    source = capture(["git", "rev-parse", "HEAD"]).strip()
    before = capture(["git", "status", "--porcelain", "--untracked-files=normal"])
    packages = capture(["go", "list", "./..."]).splitlines()
    results = execute_plan(execution_plan(packages), repo)
    after = capture(["git", "status", "--porcelain", "--untracked-files=normal"])
    exact = Counter(p for result in results for p in result["packages"]) == Counter(packages)
    success = exact and all(result["exit_code"] == 0 for result in results)
    report = {
        "schema_version": 1, "observed_utc": datetime.now(timezone.utc).isoformat(),
        "source_sha": source, "source_status_before": before, "source_status_after": after,
        "all_packages": packages, "invocations": results,
        "all_packages_executed_once": exact, "success": success,
        "scope": "Actual complete Go package suite. Ordinary packages retain15m and opt in only reviewed top-level subprocess tests with CHERRI_FULL_SUITE_SUBPROCESS_PARALLEL=2 and -parallel=2. Acceptance removes that marker and retains35m cumulative; selected producers stay sequential. Test selection, assertions and child budgets are unchanged.",
    }
    args.report.parent.mkdir(parents=True, exist_ok=True)
    args.report.write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
    return 0 if success else 1

if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (ValueError, subprocess.CalledProcessError) as exc:
        print("GO_SUITE_ERROR: " + str(exc), file=sys.stderr)
        if isinstance(exc, subprocess.CalledProcessError) and exc.stderr:
            print(exc.stderr, file=sys.stderr)
        raise SystemExit(1)
