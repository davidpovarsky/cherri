#!/usr/bin/env python3
"""Bound the existing selected-device listapps prerequisite and seal originals.

This command neither boots nor imports nor runs a Shortcut. Successful raw
stdout is released to the existing caller only after source and sealing checks.
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
from simulator_session_recovery import require

SCHEMA = "cherri-simulator-listapps-observation/1"
PHASE_SECONDS = 90
OUTER_SECONDS = 120
STAGE_SECONDS = {"source-before": 10, "listapps": 60, "source-after": 10}
LIMIT = 1024 * 1024
ARCHIVE_LIMIT = 3 * 1024 * 1024
RECIPIENT = "tools/shortcuts-simulator-qa/xcode_environment_inventory_recipient.pem"
RESULTS = {"PASS", "EXIT_NONZERO", "TIMEOUT", "LAUNCH_FAILED",
           "DEADLINE_EXHAUSTED", "OUTPUT_OVERFLOW", "OWNED_GROUP_REMAINED",
           "CAPTURE_INCOMPLETE", "CAPTURE_INTERRUPTED"}


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def encoded(value):
    return (json.dumps(value, sort_keys=True, separators=(",", ":"),
                       allow_nan=False) + "\n").encode()


class ListAppsObservation:
    def __init__(self, repo, root, udid, identity, *,
                 runner=observation.capture, now=time.monotonic):
        self.repo, self.root = Path(repo), Path(root)
        require(self.repo.is_dir() and not self.repo.is_symlink(), "REPO")
        require(not self.root.exists() and not self.root.is_symlink(), "OUTPUT_EXISTS")
        require(re.fullmatch(r"[0-9A-Fa-f]{8}(?:-[0-9A-Fa-f]{4}){3}-[0-9A-Fa-f]{12}", udid),
                "DEVICE_ID")
        require(type(identity) is dict and set(identity) ==
                {"compilerCommitSha", "runId", "runAttempt", "fixtureId"}, "IDENTITY")
        for key, pattern in (("compilerCommitSha", r"[0-9a-f]{40}"),
                             ("runId", r"[A-Za-z0-9_-]{1,64}"),
                             ("runAttempt", r"[1-9][0-9]{0,3}"),
                             ("fixtureId", r"[A-Za-z0-9_-]{1,96}")):
            require(isinstance(identity[key], str) and re.fullmatch(pattern, identity[key]),
                    "IDENTITY_VALUE")
        self.root.mkdir(parents=True, mode=0o700)
        self.private, self.public = self.root / "private", self.root / "public"
        self.private.mkdir(mode=0o700)
        self.public.mkdir(mode=0o700)
        self.udid, self.identity = udid, identity
        self.runner, self.now = runner, now
        self.started = now()
        self.commands = []
        self.source_before = self.source_after = None
        self.status, self.reason = "BLOCKED", "NOT_COMPLETED"
        self.stdout = None
        self.write("intent.original.json", {
            "schema": SCHEMA, **identity, "udid": udid,
            "argv": ["xcrun", "simctl", "listapps", udid],
            "phaseBudgetMs": PHASE_SECONDS * 1000,
            "childBudgetsSeconds": STAGE_SECONDS,
            "deviceMutationAllowed": False, "shortcutExecutionAllowed": False,
        })

    def write(self, name, value):
        with (self.private / name).open("xb") as stream:
            stream.write(encoded(value))

    def remaining(self):
        elapsed = self.now() - self.started
        require(math.isfinite(elapsed) and elapsed >= 0, "CLOCK")
        return max(0, PHASE_SECONDS - elapsed)

    def run(self, stage, argv, *, deadline=None):
        require(stage in STAGE_SECONDS, "STAGE")
        hard_deadline = min(self.started + PHASE_SECONDS,
                            self.started + PHASE_SECONDS if deadline is None else deadline)
        allowance = min(STAGE_SECONDS[stage], self.remaining(),
                        max(0, hard_deadline - self.now()))
        number = len(self.commands) + 1
        prefix = f"command-{number:02d}"
        outpath, errpath = self.private / (prefix + ".stdout"), self.private / (prefix + ".stderr")
        journal_events = []

        def journal(event, detail):
            require(event in {"started", "finished"} and event not in journal_events,
                    "JOURNAL_EVENT")
            self.write(prefix + "." + event + ".original.json",
                       {"stage": stage, "argv": argv, "event": event, "capture": detail})
            journal_events.append(event)

        self.write(prefix + ".intent.original.json", {
            "stage": stage, "argv": argv, "allowanceMs": round(allowance * 1000),
            "phaseDeadlineIsMonotonic": True,
        })
        result = {"result": "CAPTURE_INTERRUPTED", "exitCode": None, "elapsedMs": 0}
        detail = {}
        start = self.now()
        try:
            with outpath.open("xb") as output, errpath.open("xb") as error:
                if allowance <= 0:
                    result = {"result": "DEADLINE_EXHAUSTED", "exitCode": None, "elapsedMs": 0}
                else:
                    result, detail = self.runner(
                        argv, allowance, output=output, error_output=error,
                        hard_deadline=hard_deadline,
                        stream_limit=LIMIT, journal=journal)
        finally:
            stdout, stderr = outpath.read_bytes(), errpath.read_bytes()
            require(len(stdout) <= LIMIT + 1 and len(stderr) <= LIMIT + 1, "STREAM_CAP")
            if result["result"] == "CAPTURE_INTERRUPTED":
                result["elapsedMs"] = round((self.now() - start) * 1000)
                terminal = self.private / (prefix + ".finished.original.json")
                if terminal.is_file():
                    detail = json.loads(terminal.read_bytes())["capture"]
            require(result.get("result") in RESULTS, "PROCESS_RESULT")
            if result["result"] == "PASS" and (
                    self.remaining() <= 0 or result.get("exitCode") != 0 or
                    result.get("elapsedMs", -1) > round(allowance * 1000)):
                result = {**result, "result": "DEADLINE_EXHAUSTED"}
            receipt = {
                "stage": stage, "argv": argv, "allowanceMs": round(allowance * 1000),
                **result, "capture": detail, "journalEvents": journal_events,
                "stdout": {"path": outpath.name, "bytes": len(stdout), "sha256": sha(stdout)},
                "stderr": {"path": errpath.name, "bytes": len(stderr), "sha256": sha(stderr)},
            }
            self.write(prefix + ".process.original.json", receipt)
            self.commands.append(receipt)
        return result, stdout

    def source(self, stage):
        deadline = min(self.started + PHASE_SECONDS, self.now() + STAGE_SECONDS[stage])
        expected = self.identity["compilerCommitSha"]
        result, raw = self.run(stage, ["git", "--no-replace-objects", "-C",
                                      str(self.repo), "rev-parse", "HEAD"], deadline=deadline)
        require(result["result"] == "PASS" and raw == (expected + "\n").encode(), "SOURCE_CHANGED")
        result, tree = self.run(
            stage, ["git", "--no-replace-objects", "-C", str(self.repo),
                    "ls-tree", "-rz", expected, "--", *observation.QA_SOURCE_FILES],
            deadline=deadline)
        require(result["result"] == "PASS", "SOURCE_TREE_PROCESS")
        blobs = {}
        for entry in tree.split(b"\0"):
            if not entry:
                continue
            metadata, relative = entry.split(b"\t", 1)
            mode, kind, digest = metadata.decode("ascii").split()
            name = relative.decode("utf-8")
            require(name not in blobs and kind == "blob" and mode in {"100644", "100755"},
                    "SOURCE_INVENTORY")
            blobs[name] = (mode, digest)
        require(set(blobs) == set(observation.QA_SOURCE_FILES), "SOURCE_INVENTORY")
        rows, matched = [], False
        try:
            for name in observation.QA_SOURCE_FILES:
                require(self.now() < deadline, "SOURCE_DEADLINE")
                data, identity = observation.bound_read(
                    self.repo / name, observation.source_limit(name, qa=True))
                actual = hashlib.sha1(b"blob " + str(len(data)).encode() + b"\0" + data).hexdigest()
                mode, expected_blob = blobs[name]
                rows.append({"path": name, "mode": mode, "expectedBlob": expected_blob,
                             "actualBlob": actual, "bytes": len(data), "sha256": sha(data),
                             "physicalIdentity": identity})
                require(actual == expected_blob, "SOURCE_BYTES")
            require(self.now() < deadline, "SOURCE_DEADLINE")
            matched = True
        finally:
            self.write(stage + ".source.original.json", {
                "sourceCommit": expected, "workingBytesMatched": matched, "files": rows})
        return expected

    def observe(self):
        try:
            self.source_before = self.source("source-before")
            result, raw = self.run("listapps", ["xcrun", "simctl", "listapps", self.udid])
            require(result["result"] == "PASS", "LISTAPPS_PROCESS")
            self.source_after = self.source("source-after")
            require(self.remaining() > 0, "PHASE_DEADLINE")
            self.stdout = raw
            self.status, self.reason = "READY", "EXACT_LISTAPPS_STDOUT_RETAINED"
        except (OSError, ValueError, KeyError, TypeError, UnicodeError, KeyboardInterrupt):
            self.status, self.reason = "BLOCKED", "LISTAPPS_OR_SOURCE_NOT_VERIFIED"
        return self.status == "READY"

    def seal(self):
        summary = {
            "schema": SCHEMA, **self.identity,
            "deviceIdSha256": sha(self.udid.upper().encode()),
            "executionProvider": "github-actions" if os.environ.get("GITHUB_RUN_ID") else "local-diagnostic",
            "status": self.status, "reason": self.reason,
            "phaseBudgetMs": PHASE_SECONDS * 1000,
            "elapsedMs": round((self.now() - self.started) * 1000),
            "sourceMatchedBefore": self.source_before == self.identity["compilerCommitSha"],
            "sourceMatchedAfter": self.source_after == self.identity["compilerCommitSha"],
            "commands": [{key: row[key] for key in
                          ("stage", "allowanceMs", "result", "exitCode", "elapsedMs")}
                         for row in self.commands],
            "deviceMutated": False, "shortcutExecuted": False, "formalAcceptanceGranted": False,
        }
        self.write("result.original.json", summary)
        paths = sorted(self.private.rglob("*"))
        require(not any(p.is_symlink() for p in paths), "ORIGINAL_SYMLINK")
        members, buffer = [], io.BytesIO()
        with zipfile.ZipFile(buffer, "w", compression=zipfile.ZIP_STORED) as archive:
            for path in paths:
                if not path.is_file():
                    continue
                raw = path.read_bytes()
                require(len(raw) <= LIMIT + 1, "ORIGINAL_LIMIT")
                name = path.relative_to(self.private).as_posix()
                members.append({"name": name, "bytes": len(raw), "sha256": sha(raw)})
                archive.writestr(name, raw)
        data = buffer.getvalue()
        require(0 < len(data) <= ARCHIVE_LIMIT, "ARCHIVE_LIMIT")
        archive_path = self.private / "session-originals.zip"
        with archive_path.open("xb") as stream:
            stream.write(data)
        binding = {"bytes": len(data), "sha256": sha(data)}
        cipher = observation.encrypt_archive(
            self.private, self.public, archive_path, binding,
            self.repo / RECIPIENT, self.started + OUTER_SECONDS)
        (self.public / "project-prerequisite.zip.p7m").rename(self.public / "session-originals.zip.p7m")
        summary.update(members=members, plaintextArchive=binding, ciphertext=cipher,
                       recipientSha256=observation.RECIPIENT_SHA256)
        with (self.public / "summary.json").open("xb") as stream:
            stream.write(encoded(summary))
        return summary

    def release_stdout(self, path):
        require(self.status == "READY" and self.stdout is not None, "NOT_READY")
        require((self.public / "summary.json").is_file() and
                (self.public / "session-originals.zip.p7m").is_file(), "NOT_SEALED")
        path = Path(path)
        require(not path.exists() and not path.is_symlink() and path.parent.is_dir(), "OUTPUT_EXISTS")
        with path.open("xb") as stream:
            stream.write(self.stdout)


def main():
    os.umask(0o077)
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("repo", "root", "udid", "source-sha", "run-id", "attempt", "fixture-id", "output"):
        parser.add_argument("--" + name, required=True)
    args = parser.parse_args()
    require(sys.platform == "darwin", "DARWIN_REQUIRED")
    keys = ("GITHUB_SHA", "GITHUB_RUN_ID", "GITHUB_RUN_ATTEMPT")
    if any(os.environ.get(key) for key in keys):
        require(os.environ.get("GITHUB_SHA") == args.source_sha and
                os.environ.get("GITHUB_RUN_ID") == args.run_id and
                os.environ.get("GITHUB_RUN_ATTEMPT") == args.attempt, "WORKFLOW_IDENTITY")
    probe = ListAppsObservation(args.repo, args.root, args.udid, {
        "compilerCommitSha": args.source_sha, "runId": args.run_id,
        "runAttempt": args.attempt, "fixtureId": args.fixture_id})
    ready = probe.observe()
    summary = probe.seal()
    if os.environ.get("GITHUB_OUTPUT"):
        with Path(os.environ["GITHUB_OUTPUT"]).open("a", encoding="ascii") as stream:
            stream.write("session_boundary_publication_ready=true\n")
    if ready:
        probe.release_stdout(args.output)
    print(json.dumps({"simulatorListApps": summary["status"], "formalAcceptanceGranted": False}))
    return 0 if ready else 70


if __name__ == "__main__":
    def interrupted(signum, frame):
        raise TimeoutError("LISTAPPS_OBSERVATION_INTERRUPTED")
    for kind in (signal.SIGTERM, signal.SIGINT, signal.SIGALRM):
        signal.signal(kind, interrupted)
    signal.setitimer(signal.ITIMER_REAL, OUTER_SECONDS)
    try:
        raise SystemExit(main())
    except (Exception, KeyboardInterrupt):
        print("SIMULATOR_LISTAPPS_OBSERVATION_BLOCKED", file=sys.stderr)
        raise SystemExit(70)
    finally:
        signal.setitimer(signal.ITIMER_REAL, 0)
