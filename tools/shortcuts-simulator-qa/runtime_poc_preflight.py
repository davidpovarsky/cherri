#!/usr/bin/env python3
"""Retain the original bounded Darwin/iOS runtime inventory before the sole PoC."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import sys
import time

from project_prerequisite_observation import capture

SCHEMA = "cherri-ios-runtime-preflight/1"
STREAM_LIMIT = 256 * 1024
SHA = re.compile(r"[0-9a-f]{40}")
IOS27 = re.compile(r"(?m)^iOS 27(?:\.\d+)*\s+\(")


def encoded(value):
    return (json.dumps(value, indent=2, sort_keys=True) + "\n").encode("utf-8")


def context():
    if os.environ.get("GITHUB_ACTIONS") != "true":
        return {"provider": "local", "runId": None, "attempt": None,
                "repository": None, "ref": None, "expectedSourceSha": None}
    sha = os.environ.get("GITHUB_SHA", "")
    run = os.environ.get("GITHUB_RUN_ID", "")
    attempt = os.environ.get("GITHUB_RUN_ATTEMPT", "")
    ref = os.environ.get("GITHUB_REF", "")
    repository = os.environ.get("GITHUB_REPOSITORY", "")
    if (not SHA.fullmatch(sha) or not run.isdecimal() or int(run) < 1
            or not attempt.isdecimal() or int(attempt) < 1
            or not ref.startswith("refs/") or repository != "davidpovarsky/cherri"):
        raise ValueError("INVALID_GITHUB_CONTEXT")
    return {"provider": "github-actions", "runId": run, "attempt": int(attempt),
            "repository": repository, "ref": ref, "expectedSourceSha": sha}


def run_command(root, stage, argv, allowance, deadline):
    stdout = root / (stage + ".stdout")
    stderr = root / (stage + ".stderr")
    with stdout.open("xb") as out, stderr.open("xb") as err:
        result, details = capture(
            argv, min(allowance, max(0, deadline - time.monotonic())),
            output=out, error_output=err, hard_deadline=deadline,
            stream_limit=STREAM_LIMIT,
        )
    record = {"argv": argv, "allowanceMs": round(allowance * 1000),
              **result, "capture": details,
              "stdoutPath": stdout.name, "stderrPath": stderr.name}
    (root / (stage + ".process.json")).write_bytes(encoded(record))
    return record


def source_snapshot(repo, root, stage, deadline):
    head = run_command(root, stage + "-identity",
                       ["git", "-C", str(repo), "rev-parse", "HEAD", "HEAD^{tree}"],
                       5, deadline)
    status = run_command(root, stage + "-status",
                         ["git", "-C", str(repo), "status", "--porcelain", "--untracked-files=no"],
                         5, deadline)
    raw = (root / head["stdoutPath"]).read_bytes()
    rows = raw.decode("ascii", errors="replace").splitlines()
    valid = (head["result"] == status["result"] == "PASS"
             and head["exitCode"] == status["exitCode"] == 0
             and len(rows) == 2 and all(SHA.fullmatch(x) for x in rows)
             and not (root / status["stdoutPath"]).read_bytes())
    return {"valid": bool(valid), "head": rows[0] if len(rows) == 2 else None,
            "tree": rows[1] if len(rows) == 2 else None,
            "identityProcess": head, "statusProcess": status}


def classify(system, before, after, run_context, inventory, stdout):
    """No native success follows from marker text, a late exit, or stale source."""
    if system != "Darwin":
        return "BLOCKED_PLATFORM"
    if not before["valid"] or not after["valid"]:
        return "SOURCE_INVALID"
    if (before["head"], before["tree"]) != (after["head"], after["tree"]):
        return "SOURCE_CHANGED"
    expected = run_context["expectedSourceSha"]
    if expected is not None and before["head"] != expected:
        return "SOURCE_CONTEXT_MISMATCH"
    if (inventory is None or inventory["result"] != "PASS"
            or type(inventory["exitCode"]) is not int or inventory["exitCode"] != 0):
        return "INVENTORY_FAILED"
    capture_info = inventory["capture"]
    if (type(capture_info["returnCodeObserved"]) is not int
            or capture_info["returnCodeObserved"] != 0
            or not capture_info["cleanupParentReaped"]
            or capture_info["groupPresentAfterCleanup"] is not False
            or any(not capture_info[n]["eof"] or capture_info[n]["overflow"]
                   for n in ("stdout", "stderr"))):
        return "INVENTORY_CAPTURE_INCOMPLETE"
    if not IOS27.search(stdout.decode("utf-8", errors="strict")):
        return "IOS27_UNAVAILABLE"
    return "READY"


def collect(repo, output):
    repo = repo.resolve(strict=True)
    output.mkdir(mode=0o700, parents=False, exist_ok=False)
    started = time.monotonic()
    deadline = started + 52
    system = platform.system()
    runtime = None
    receipt = {"schema": SCHEMA, "sysPlatform": sys.platform, "system": system,
               "machine": platform.machine(), "release": platform.release(),
               "nativePoCExecutedByCollector": False, "formalAcceptanceGranted": False}
    try:
        run_context = context()
        before = source_snapshot(repo, output, "source-before", deadline)
        if system == "Darwin":
            runtime = run_command(output, "runtime-inventory",
                                  ["xcrun", "simctl", "list", "runtimes"], 30, deadline)
        after = source_snapshot(repo, output, "source-after", deadline)
        raw = (output / runtime["stdoutPath"]).read_bytes() if runtime else b""
        receipt.update(context=run_context, sourceBefore=before, sourceAfter=after,
                       runtimeInventory=runtime,
                       inventoryStdoutSha256=hashlib.sha256(raw).hexdigest(),
                       status=classify(system, before, after, run_context, runtime, raw))
    except (OSError, ValueError, KeyError) as error:
        receipt.update(status="PREFLIGHT_REJECTED", rejectionType=type(error).__name__)
    receipt["elapsedMs"] = round((time.monotonic() - started) * 1000)
    receipt["files"] = [
        {"path": p.name, "bytes": p.stat().st_size,
         "sha256": hashlib.sha256(p.read_bytes()).hexdigest()}
        for p in sorted(output.iterdir()) if p.is_file()
    ]
    (output / "receipt.json").write_bytes(encoded(receipt))
    print(json.dumps({"status": receipt["status"], "system": system,
                      "inventoryExitCode": runtime["exitCode"] if runtime else None,
                      "nativePoCExecutedByCollector": False}))
    return 0 if receipt["status"] == "READY" else 1


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args(argv)
    return collect(args.repo, args.output)


if __name__ == "__main__":
    raise SystemExit(main())
