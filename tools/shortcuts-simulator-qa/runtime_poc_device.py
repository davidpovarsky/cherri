#!/usr/bin/env python3
"""Create and clean up only the iOS Simulator owned by one formal PoC run."""
import argparse
import json
import os
from pathlib import Path
import re
import subprocess
import uuid

from prepare_public_artifacts import PublicationBlocked, public_json

RECEIPT = "runtime-poc-device.json"
UDID = re.compile(r"^[A-Fa-f0-9]{8}(?:-[A-Fa-f0-9]{4}){3}-[A-Fa-f0-9]{12}$")
DEVICE_TYPE = "com.apple.CoreSimulator.SimDeviceType.iPhone-16"


def require(ok, code):
    if not ok:
        raise PublicationBlocked(code)


def identity(commit, run_id, attempt):
    require(re.fullmatch(r"[0-9a-f]{40}", commit or "") is not None, "POC_COMMIT_INVALID")
    require(re.fullmatch(r"[1-9][0-9]*", run_id or "") is not None, "POC_RUN_INVALID")
    require(re.fullmatch(r"[1-9][0-9]*", attempt or "") is not None, "POC_ATTEMPT_INVALID")


def write_receipt(path, data):
    require(not path.is_symlink(), "POC_DEVICE_RECEIPT_SYMLINK")
    temporary = path.with_suffix(".tmp")
    require(not temporary.exists() and not temporary.is_symlink(), "POC_DEVICE_TEMP_EXISTS")
    temporary.write_bytes((json.dumps(data, indent=2) + "\n").encode("utf-8"))
    temporary.replace(path)


def create(root, commit, run_id, attempt, run=subprocess.run):
    identity(commit, run_id, attempt)
    root = Path(root)
    require(root.is_dir() and not root.is_symlink(), "POC_RAW_ROOT_INVALID")
    path = root / RECEIPT
    require(not path.exists() and not path.is_symlink(), "POC_DEVICE_RECEIPT_EXISTS")
    result = run(["xcrun", "simctl", "list", "runtimes", "-j"], check=True,
                 capture_output=True, text=True)
    runtimes = public_json(result.stdout).get("runtimes", [])
    candidates = [r for r in runtimes if r.get("isAvailable") is True
                  and re.fullmatch(r"com\.apple\.CoreSimulator\.SimRuntime\.iOS-27(?:-[0-9]+)+",
                                   r.get("identifier", ""))]
    require(bool(candidates), "POC_IOS27_RUNTIME_UNAVAILABLE")
    runtime = sorted(candidates, key=lambda r: tuple(int(v) for v in r["identifier"].split("iOS-")[1].split("-")))[-1]["identifier"]
    name = "Cherri-PoC-" + run_id + "-" + attempt + "-" + uuid.uuid4().hex[:12]
    result = run(["xcrun", "simctl", "create", name, DEVICE_TYPE, runtime],
                 check=True, capture_output=True, text=True)
    udid = result.stdout.strip()
    require(UDID.fullmatch(udid) is not None, "POC_CREATE_DID_NOT_RETURN_UDID")
    receipt = {"schemaVersion": "1", "implementationSha": commit, "runId": run_id,
               "runAttempt": attempt, "createdByRun": True, "udid": udid,
               "deviceName": name, "deviceType": DEVICE_TYPE,
               "runtimeIdentifier": runtime, "cleanupStatus": "PENDING"}
    try:
        write_receipt(path, receipt)
    except (OSError, PublicationBlocked):
        # Creation returned this exact UDID in this call; never discover a substitute.
        run(["xcrun", "simctl", "delete", udid], capture_output=True, text=True)
        raise
    return receipt


def cleanup(root, commit, run_id, attempt, udid, run=subprocess.run):
    identity(commit, run_id, attempt)
    root = Path(root)
    require(root.is_dir() and not root.is_symlink(), "POC_RAW_ROOT_INVALID")
    path = root / RECEIPT
    require(path.is_file() and not path.is_symlink(), "POC_DEVICE_RECEIPT_INVALID")
    receipt = public_json(path.read_text(encoding="utf-8"))
    require(receipt.get("createdByRun") is True and receipt.get("implementationSha") == commit
            and receipt.get("runId") == run_id and receipt.get("runAttempt") == attempt
            and receipt.get("udid") == udid and UDID.fullmatch(udid or "") is not None
            and receipt.get("deviceName", "").startswith("Cherri-PoC-" + run_id + "-" + attempt + "-"),
            "POC_DEVICE_OWNERSHIP_MISMATCH")
    if receipt.get("cleanupStatus") == "DELETED":
        return receipt
    # No discovery, wildcard, erase, or deletion of any existing unrelated device.
    shutdown = run(["xcrun", "simctl", "shutdown", udid], capture_output=True, text=True)
    deleted = run(["xcrun", "simctl", "delete", udid], capture_output=True, text=True)
    receipt["shutdownExitCode"] = shutdown.returncode
    receipt["deleteExitCode"] = deleted.returncode
    receipt["cleanupStatus"] = "DELETED" if deleted.returncode == 0 else "DELETE_FAILED"
    write_receipt(path, receipt)
    require(deleted.returncode == 0, "POC_OWNED_DEVICE_CLEANUP_FAILED")
    return receipt


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("create", "cleanup"))
    parser.add_argument("--root", type=Path, required=True)
    parser.add_argument("--commit", default=os.environ.get("GITHUB_SHA", ""))
    parser.add_argument("--run-id", default=os.environ.get("GITHUB_RUN_ID", ""))
    parser.add_argument("--attempt", default=os.environ.get("GITHUB_RUN_ATTEMPT", "1"))
    parser.add_argument("--udid")
    args = parser.parse_args()
    try:
        if args.command == "create":
            receipt = create(args.root, args.commit, args.run_id, args.attempt)
            print(receipt["udid"])
        else:
            receipt = cleanup(args.root, args.commit, args.run_id, args.attempt, args.udid)
            print(json.dumps({"ownedSimulatorCleanup": receipt["cleanupStatus"]}))
    except (PublicationBlocked, OSError, ValueError, subprocess.SubprocessError):
        # Host exception text can contain private paths; retain only bounded status.
        print("POC_OWNED_SIMULATOR_OPERATION_FAILED", file=__import__("sys").stderr)
        return 2
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
