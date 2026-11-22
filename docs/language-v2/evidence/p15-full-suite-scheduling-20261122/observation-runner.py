#!/usr/bin/env python3
"""Diagnostic-only replay of four unchanged, fixed cumulative-suite parents."""
from __future__ import annotations
import argparse
from collections import Counter
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import time

PARENTS = {
    "TestP10_IsolatedCompilerMutationGate": [
        "mutation-1-run-makeParams-drops-WFInput",
        "mutation-2-statistic-emits-Input-instead-of-WFInput",
        "mutation-3a-parser-greedy-inline-var-regex-on-json-braces",
        "mutation-3b-shortcutgen-greedy-inline-var-regex-on-quantifiers",
        "mutation-4-lower-array-equality-conditional-accepted"],
    "TestP15CanonicalBehaviorSourcePropagation": ["static", "callback", "codec"],
    "TestP15RP45ExtractedInstallAndRollback": [
        "offline_compatible_cache", "corrupt_download_compiler", "corrupt_download_migration",
        "explicit_migration_override_missing", "explicit_migration_override_wrong_role",
        "interrupt_migration_backup", "interrupt_migration_binary",
        "interrupt_migration_source_receipt", "interrupt_migration_checksum_receipt",
        "interrupt_documentation", "successful_complete_update"],
    "TestP15RP42ExtractedInstallPublicCLI": ["help_and_schema", "check_and_alias",
        "format_and_aliases", "compile_and_file_alias", "import_and_alias",
        "installed_migration_tool"],
}
EXPECTED = sorted([name for name in PARENTS] +
                  [parent + "/" + child for parent, children in PARENTS.items() for child in children])

def sha(raw):
    return hashlib.sha256(raw).hexdigest()

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo", required=True, type=Path)
    parser.add_argument("--source", required=True)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    repo, output = args.repo.resolve(), args.output.resolve()
    if repo == output or repo in output.parents:
        raise ValueError("diagnostic output must be outside the source checkout")
    output.mkdir(parents=True, exist_ok=False)
    phases = output / "phases"
    phases.mkdir()
    os.chdir(repo)
    sys.dont_write_bytecode = True
    os.environ["PYTHONDONTWRITEBYTECODE"] = "1"
    sys.path.insert(0, str(repo / "tools/shortcuts-simulator-qa"))
    import project_prerequisite_observation as bounded
    def git(*argv):
        return subprocess.run(["git", "--no-replace-objects", *argv], check=True,
                              capture_output=True, text=True, timeout=30).stdout.strip()
    source, before = git("rev-parse", "HEAD"), git("status", "--porcelain", "--untracked-files=normal")
    if source != args.source or before:
        raise ValueError("exact clean diagnostic source is required")
    os.environ["CHERRI_P15_PHASE_SOURCE"] = source
    os.environ["CHERRI_P15_PHASE_OUTPUT"] = str(phases)
    command = ["go", "test", "-count=1", "-json", "-timeout=15m", "-run",
               "^(" + "|".join(PARENTS) + ")$", "."]
    inventory = {"source": source, "tree": git("rev-parse", "HEAD^{tree}"),
                 "command": command, "expected_tests": EXPECTED,
                 "diagnostic_only": True, "formal_acceptance_granted": False}
    (output / "inventory.json").write_text(json.dumps(inventory, indent=2) + "\n")
    started = time.monotonic()
    with (output / "go.stdout.jsonl").open("xb") as out, (output / "go.stderr.log").open("xb") as err:
        process, capture = bounded.capture(command, 930, output=out, error_output=err,
                                           hard_deadline=started + 930)
    after_source, after = git("rev-parse", "HEAD"), git("status", "--porcelain", "--untracked-files=normal")
    (output / "process.json").write_text(json.dumps({"process": process, "capture": capture}, indent=2) + "\n")
    errors, events = [], []
    for line in (output / "go.stdout.jsonl").read_bytes().splitlines():
        try:
            events.append(json.loads(line))
        except (ValueError, UnicodeError):
            errors.append("malformed Go event")
    runs = Counter(row.get("Test") for row in events if row.get("Action") == "run")
    passes = Counter(row.get("Test") for row in events if row.get("Action") == "pass" and row.get("Test"))
    failures = [row for row in events if row.get("Action") in {"fail", "skip"}]
    if runs != Counter(EXPECTED) or passes != Counter(EXPECTED) or failures:
        errors.append("fixed parent/child execution inventory is incomplete")
    if sum(row.get("Action") == "pass" and not row.get("Test") for row in events) != 1:
        errors.append("missing root package success")
    if any(row.get("Package") != "github.com/electrikmilk/cherri" for row in events):
        errors.append("foreign package event")
    if source != after_source or before or after:
        errors.append("source was not preserved clean")
    starts, ends = {}, {}
    for path in phases.glob("*.json"):
        row = json.loads(path.read_bytes())
        key = (row["pid"], row["id"])
        target = starts if path.name.endswith(".start.json") else ends
        if key in target:
            errors.append("duplicate phase identity")
        target[key] = row
        if row.get("source_commit") != source or row.get("test", "").split("/")[0] not in PARENTS:
            errors.append("foreign phase identity")
        for raw in row.get("originals", {}).values():
            name = raw["path"]
            if Path(name).name != name:
                errors.append("nonlocal original")
                continue
            data = (phases / name).read_bytes()
            if len(data) != raw["bytes"] or sha(data) != raw["sha256"]:
                errors.append("changed original bytes")
    if starts.keys() != ends.keys():
        errors.append("unfinished phase records")
    phase_counts = Counter(row["phase"] for row in ends.values())
    if phase_counts["p10_actual_mutation_command"] != 6 or phase_counts["canonical_rebuilt_probe"] != 7:
        errors.append("actual mutation/restoration process count changed")
    cache = [row for row in ends.values() if row["phase"] == "actual_tool_cache_lookup"]
    report = {"schema": "cherri.p15.four-parent-phase-run/v1", **inventory,
              "observed_utc": datetime.now(timezone.utc).isoformat(),
              "source_status_before": before, "source_status_after": after,
              "process": process, "capture": capture,
              "test_runs": dict(runs), "test_passes": dict(passes),
              "failures_or_skips": failures, "phase_counts": dict(phase_counts),
              "cache_observations": cache, "integrity_errors": errors,
              "success": process["result"] == "PASS" and not errors}
    (output / "report.json").write_text(json.dumps(report, indent=2) + "\n")
    print(json.dumps({k: report[k] for k in ["source", "process", "integrity_errors", "success"]}), flush=True)
    return 0 if report["success"] else 1

if __name__ == "__main__":
    raise SystemExit(main())
