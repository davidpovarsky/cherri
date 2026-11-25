#!/usr/bin/env python3
"""Reacquire the selected simulator after XCTest; retain sealed originals.

This establishes only a process/session precondition. It never imports or runs
a Shortcut and grants no runtime, List-type or formal acceptance.
"""
import argparse
import hashlib
import io
import json
import math
import os
from pathlib import Path
import re
import signal
import sys
import time
import zipfile

import project_prerequisite_observation as observation
from simulator_session_recovery import require, receipt_json
import xcode_destination_diagnostic as launch

SCHEMA = "cherri-simulator-phase-boundary/1"
PHASES = ("pre-import", "post-test-runtime")
PHASE_SECONDS = 240
OUTER_SECONDS = 310
LIMIT = 256 * 1024
ARCHIVE_LIMIT = 8 * 1024 * 1024
STAGE_SECONDS = {"source-before": 15, "source-after": 15, "state": 15,
                 "prepare": 15, "boot": 30, "bootstatus": 120, "getenv": 15}
RESULTS = {"PASS", "EXIT_NONZERO", "TIMEOUT", "LAUNCH_FAILED", "DEADLINE_EXHAUSTED",
           "OUTPUT_OVERFLOW", "OWNED_GROUP_REMAINED", "CAPTURE_INCOMPLETE"}
STATES = {"Booted", "Booting", "Shutdown", "Shutting Down"}
RECIPIENT = "tools/shortcuts-simulator-qa/xcode_environment_inventory_recipient.pem"


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def encoded(value):
    return (json.dumps(value, sort_keys=True, separators=(",", ":"), allow_nan=False) + "\n").encode()


def state_observation(raw, udid):
    """Accept exactly the selected available iOS 27 device, never its name."""
    require(isinstance(raw, bytes) and len(raw) <= LIMIT, "STATE_OUTPUT_SIZE")
    value = receipt_json(raw.decode("utf-8"))
    require(type(value) is dict and type(value.get("devices")) is dict, "STATE_SHAPE")
    matches = []
    for runtime, devices in value["devices"].items():
        require(isinstance(runtime, str) and type(devices) is list, "STATE_RUNTIME")
        for device in devices:
            require(type(device) is dict, "STATE_DEVICE")
            if isinstance(device.get("udid"), str) and device["udid"].upper() == udid.upper():
                matches.append((runtime, device))
    require(len(matches) == 1, "STATE_DEVICE_NOT_UNIQUE")
    runtime, device = matches[0]
    require(re.fullmatch(r"com\.apple\.CoreSimulator\.SimRuntime\.iOS-27-[0-9]{1,3}(?:-[0-9]{1,3}){0,2}", runtime),
            "STATE_PLATFORM")
    require(device.get("isAvailable") is True and device.get("state") in STATES,
            "STATE_UNAVAILABLE")
    return {"state": device["state"], "runtime": runtime,
            "deviceIdSha256": sha(udid.upper().encode()),
            "stdoutBytes": len(raw), "stdoutSha256": sha(raw)}


class Boundary:
    def __init__(self, repo, root, udid, identity, *, runner=observation.capture,
                 prepare=launch.prepare, now=time.monotonic):
        self.repo, self.root = Path(repo), Path(root)
        require(self.repo.is_dir() and not self.repo.is_symlink(), "REPO")
        require(not self.root.exists() and not self.root.is_symlink(), "OUTPUT_EXISTS")
        require(re.fullmatch(r"[0-9A-Fa-f]{8}(?:-[0-9A-Fa-f]{4}){3}-[0-9A-Fa-f]{12}", udid),
                "DEVICE_ID")
        require(type(identity) is dict and set(identity) ==
                {"compilerCommitSha", "runId", "runAttempt", "fixtureId", "phase"}, "IDENTITY")
        for key, pattern in (("compilerCommitSha", r"[0-9a-f]{40}"),
                             ("runId", r"[A-Za-z0-9_-]{1,64}"), ("runAttempt", r"[1-9][0-9]{0,3}"),
                             ("fixtureId", r"[A-Za-z0-9_-]{1,96}")):
            require(isinstance(identity[key], str) and re.fullmatch(pattern, identity[key]),
                    "IDENTITY_VALUE")
        require(identity["phase"] in PHASES, "PHASE")
        self.root.mkdir(parents=True)
        self.private, self.public = self.root / "private", self.root / "public"
        self.private.mkdir()
        (self.private / "builds").mkdir()
        self.public.mkdir()
        self.udid, self.identity = udid, identity
        self.runner, self.prepare, self.now = runner, prepare, now
        self.started = now()
        self.commands, self.states = [], []
        self.prepare_receipt = None
        self.boot_issued = False
        self.status, self.reason = "BLOCKED", "NOT_COMPLETED"
        self.source_before = self.source_after = None
        (self.private / "intent.original.json").write_bytes(encoded({
            "schema": SCHEMA, **identity, "udid": udid,
            "phaseBudgetMs": PHASE_SECONDS * 1000, "childBudgetsSeconds": STAGE_SECONDS,
            "eraseOrCreateAllowed": False, "shortcutExecutionAllowed": False,
        }))

    def remaining(self):
        elapsed = self.now() - self.started
        require(math.isfinite(elapsed) and elapsed >= 0, "CLOCK")
        return max(0, PHASE_SECONDS - elapsed)

    def run(self, stage, argv, allowance=None, *, output=None, error_output=None):
        require(stage in STAGE_SECONDS, "STAGE")
        limit = min(STAGE_SECONDS[stage], self.remaining(),
                    STAGE_SECONDS[stage] if allowance is None else max(0, allowance))
        number = len(self.commands) + 1
        stdout_path = self.private / (f"command-{number:02d}.stdout")
        stderr_path = self.private / (f"command-{number:02d}.stderr")
        owns_output = output is None
        if owns_output:
            output = stdout_path.open("xb")
        else:
            stdout_path = Path(output.name)
            require(stdout_path.resolve().is_relative_to(self.private.resolve()),
                    "OUTPUT_OUTSIDE_PRIVATE")
        # The launch resolver historically joins stderr into stdout. Keep its
        # original combined stream while retaining a separate exact empty file.
        owns_error = error_output is None
        if owns_error:
            error_output = stderr_path.open("xb")
        else:
            stderr_path = Path(error_output.name)
            require(stderr_path.resolve().is_relative_to(self.private.resolve()),
                    "ERROR_OUTSIDE_PRIVATE")
        try:
            if limit <= 0:
                result, detail = {"result": "DEADLINE_EXHAUSTED", "exitCode": None, "elapsedMs": 0}, {}
            else:
                result, detail = self.runner(
                    argv, limit, output=output, error_output=error_output,
                    hard_deadline=self.started + PHASE_SECONDS, stream_limit=LIMIT)
            output.flush()
            error_output.flush()
        finally:
            if owns_output:
                output.close()
            if owns_error:
                error_output.close()
        require(result.get("result") in RESULTS, "PROCESS_RESULT")
        if result["result"] == "PASS" and (self.remaining() <= 0 or
                result.get("exitCode") != 0 or result.get("elapsedMs", -1) > round(limit * 1000)):
            result = {**result, "result": "DEADLINE_EXHAUSTED"}
        stdout, stderr = stdout_path.read_bytes(), stderr_path.read_bytes()
        require(len(stdout) <= LIMIT + 1 and len(stderr) <= LIMIT + 1, "STREAM_CAP")
        receipt = {"stage": stage, "argv": argv, "allowanceMs": round(limit * 1000),
                   **result, "capture": detail,
                   "stdout": {"path": str(stdout_path.relative_to(self.private)),
                              "bytes": len(stdout), "sha256": sha(stdout)},
                   "stderr": {"path": str(stderr_path.relative_to(self.private)),
                              "bytes": len(stderr), "sha256": sha(stderr)}}
        (self.private / f"command-{number:02d}.process.json").write_bytes(encoded(receipt))
        self.commands.append(receipt)
        return result, stdout

    def source(self, stage):
        result, raw = self.run(stage, ["git", "--no-replace-objects", "-C", str(self.repo),
                                      "rev-parse", "HEAD"])
        require(result["result"] == "PASS" and raw == (self.identity["compilerCommitSha"] + "\n").encode(),
                "SOURCE_CHANGED")
        return raw.decode().strip()

    def state(self):
        result, raw = self.run("state", ["xcrun", "simctl", "list", "devices", "--json"])
        require(result["result"] == "PASS", "STATE_PROCESS")
        observed = state_observation(raw, self.udid)
        self.states.append(observed)
        return observed["state"]

    def boot_if_stopped(self, state):
        if state == "Shutdown":
            require(not self.boot_issued, "SECOND_BOOT_FORBIDDEN")
            self.boot_issued = True
            result, _ = self.run("boot", ["xcrun", "simctl", "boot", self.udid])
            require(result["result"] == "PASS", "BOOT_PROCESS")
        else:
            require(state in {"Booted", "Booting"}, "DEVICE_STILL_STOPPING")

    def run_session(self):
        try:
            self.source_before = self.source("source-before")
            self.state()
            # Resolve and open only the active Xcode's Simulator using the
            # existing reviewed resolver, including its explicit headless case.
            result, self.prepare_receipt = self.prepare(
                self.private, self.udid, min(15, self.remaining()), now=self.now)
            # Persist the original resolver/open result even when preparation fails.
            (self.private / "prepare.original.json").write_bytes(encoded(self.prepare_receipt))
            require(result["result"] == "PASS", "SESSION_PREPARE")
            state = self.state()
            self.boot_if_stopped(state)
            result, _ = self.run("bootstatus", ["xcrun", "simctl", "bootstatus", self.udid, "-b"])
            require(result["result"] == "PASS", "BOOTSTATUS_PROCESS")
            require(self.state() == "Booted", "POST_BOOT_STATE")
            result, _ = self.run("getenv", ["xcrun", "simctl", "getenv", self.udid, "HOME"])
            require(result["result"] == "PASS", "POST_BOOT_LIVENESS")
            require(self.state() == "Booted", "FINAL_DEVICE_STATE")
            self.source_after = self.source("source-after")
            require(self.remaining() > 0, "PHASE_DEADLINE")
            self.status, self.reason = "READY", "FRESH_SELECTED_DEVICE_AFTER_XCTEST"
        except (OSError, ValueError, KeyError, TypeError, UnicodeError):
            self.status, self.reason = "BLOCKED", "SESSION_OR_SOURCE_NOT_VERIFIED"
        return self.status == "READY"

    def seal(self):
        elapsed = round((self.now() - self.started) * 1000)
        summary = {"schema": SCHEMA, **self.identity,
                   "deviceIdSha256": sha(self.udid.upper().encode()),
                   "executionProvider": "github-actions" if os.environ.get("GITHUB_RUN_ID") else "local-diagnostic",
                   "status": self.status, "reason": self.reason, "elapsedMs": elapsed,
                   "phaseBudgetMs": PHASE_SECONDS * 1000,
                   "sourceMatchedBefore": self.source_before == self.identity["compilerCommitSha"],
                   "sourceMatchedAfter": self.source_after == self.identity["compilerCommitSha"],
                   "bootIssued": self.boot_issued, "states": self.states,
                   "commands": [{key: row[key] for key in ("stage", "allowanceMs", "result", "exitCode", "elapsedMs")}
                                for row in self.commands],
                   "deviceErased": False, "deviceCreated": False,
                   "shortcutExecuted": False, "formalAcceptanceGranted": False}
        (self.private / "result.original.json").write_bytes(encoded(summary))
        files = sorted(p for p in self.private.rglob("*") if p.is_file())
        require(files and not any(p.is_symlink() for p in self.private.rglob("*")), "ORIGINAL_SYMLINK")
        members = []
        buffer = io.BytesIO()
        with zipfile.ZipFile(buffer, "w", compression=zipfile.ZIP_STORED) as archive:
            for path in files:
                raw = path.read_bytes()
                require(len(raw) <= LIMIT + 1, "ORIGINAL_LIMIT")
                name = path.relative_to(self.private).as_posix()
                members.append({"name": name, "bytes": len(raw), "sha256": sha(raw)})
                archive.writestr(name, raw)
        data = buffer.getvalue()
        require(0 < len(data) <= ARCHIVE_LIMIT, "ARCHIVE_LIMIT")
        archive_path = self.private / "session-originals.zip"
        archive_path.write_bytes(data)
        binding = {"bytes": len(data), "sha256": sha(data)}
        cipher = observation.encrypt_archive(
            self.private, self.public, archive_path, binding,
            self.repo / RECIPIENT, self.started + OUTER_SECONDS)
        (self.public / "project-prerequisite.zip.p7m").rename(self.public / "session-originals.zip.p7m")
        summary.update({"members": members, "plaintextArchive": binding, "ciphertext": cipher,
                        "recipientSha256": observation.RECIPIENT_SHA256})
        (self.public / "summary.json").write_bytes(encoded(summary))
        return summary


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("repo", "root", "udid", "source-sha", "run-id", "attempt", "fixture-id", "phase"):
        parser.add_argument("--" + name, required=True)
    args = parser.parse_args()
    require(sys.platform == "darwin", "DARWIN_REQUIRED")
    require(args.phase in PHASES, "PHASE")
    github_keys = ("GITHUB_SHA", "GITHUB_RUN_ID", "GITHUB_RUN_ATTEMPT")
    if any(os.environ.get(key) for key in github_keys):
        require(os.environ.get("GITHUB_SHA") == args.source_sha and
                os.environ.get("GITHUB_RUN_ID") == args.run_id and
                os.environ.get("GITHUB_RUN_ATTEMPT") == args.attempt, "WORKFLOW_IDENTITY")
    boundary = Boundary(args.repo, args.root, args.udid, {
        "compilerCommitSha": args.source_sha, "runId": args.run_id, "runAttempt": args.attempt,
        "fixtureId": args.fixture_id, "phase": args.phase})
    ready = boundary.run_session()
    summary = boundary.seal()
    if os.environ.get("GITHUB_OUTPUT"):
        with Path(os.environ["GITHUB_OUTPUT"]).open("a", encoding="ascii") as stream:
            stream.write("session_boundary_publication_ready=true\n")
    print(json.dumps({"simulatorPhaseBoundary": summary["status"], "phase": args.phase,
                      "bootIssued": summary["bootIssued"], "formalAcceptanceGranted": False}))
    return 0 if ready else 70


if __name__ == "__main__":
    def interrupted(signum, frame):
        raise TimeoutError("SESSION_BOUNDARY_INTERRUPTED")
    signal.signal(signal.SIGTERM, interrupted)
    signal.signal(signal.SIGINT, interrupted)
    signal.signal(signal.SIGALRM, interrupted)
    signal.setitimer(signal.ITIMER_REAL, OUTER_SECONDS)
    try:
        raise SystemExit(main())
    except (Exception, KeyboardInterrupt):
        print("SIMULATOR_PHASE_BOUNDARY_BLOCKED", file=sys.stderr)
        raise SystemExit(70)
    finally:
        signal.setitimer(signal.ITIMER_REAL, 0)
