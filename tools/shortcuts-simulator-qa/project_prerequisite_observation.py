#!/usr/bin/env python3
"""Capture only the existing project prerequisite, then encrypt its originals.

This is a separate diagnostic transport, not a simulator or acceptance runner.
The existing prepare implementation and its 180/60/240 second limits are inputs.
"""
import argparse
import datetime
import errno
import hashlib
import io
import json
import math
import os
from pathlib import Path
import re
import selectors
import signal
import stat
import subprocess
import sys
import time
import zipfile

import project_prerequisite as prerequisite

SCHEMA = "cherri-project-prerequisite-observation/1"
QA_SCHEMA = "cherri-project-prerequisite-observation/2"
QA_SCOPE = "CURRENT_PREREQUISITE_CALL_ONLY"
QA_MANIFEST = "qa-compiler-manifest.original.json"
QA_MANIFEST_LIMIT = 4 * prerequisite.terminal.MAX_PARSE_BYTES
STREAM_LIMIT = 2 * 1024 * 1024
METADATA_LIMIT = 64 * 1024
ARCHIVE_LIMIT = 9 * 1024 * 1024
CIPHER_LIMIT = ARCHIVE_LIMIT + METADATA_LIMIT
PREFLIGHT_SECONDS = 30
ENCRYPT_SECONDS = 20
OUTER_SECONDS = 310
RECIPIENT_SHA256 = "55956e825597a2e60acca4045e4ea8040ae27f17caa7964e9b4493824fa0c1f1"
QA = "tools/shortcuts-simulator-qa/"
WORKFLOW = ".github/workflows/cherri-ios27-shortcuts-simulator-qa.yml"
RECIPIENT = QA + "xcode_environment_inventory_recipient.pem"
SOURCE_FILES = (
    WORKFLOW,
    QA + "project_prerequisite_observation.py",
    QA + "test_project_prerequisite_observation.py",
    QA + "project_prerequisite.py",
    QA + "simulator_session_recovery.py",
    QA + "qa_terminal_receipts.py",
    RECIPIENT,
    QA + "XcodeProject/project.yml",
    QA + "XcodeProject/ReceiverApp/Info.plist",
)
QA_SOURCE_FILES = tuple(dict.fromkeys((*SOURCE_FILES,
    *prerequisite.terminal.SOURCE_FILES, QA + "test_project_prerequisite_qa.py")))
SCRIPTS = (
    ("install", 'if ! command -v xcodegen >/dev/null 2>&1; then HOMEBREW_NO_AUTO_UPDATE=1 HOMEBREW_NO_INSTALL_CLEANUP=1 brew install xcodegen; fi'),
    ("generate", "xcodegen generate"),
)
STAGE_LIMITS = {"install": 180, "generate": 60}
EXPECTED_ENVIRONMENT = {
    "productVersion": "27.0.1",
    "buildVersion": "26A434",
    "xcodeVersion": "27.1",
    "xcodeBuild": "27A9275",
    "developerDirectory": "/Applications/Xcode_27.1.app/Contents/Developer",
}
PROCESS_RESULTS = {
    "PASS", "EXIT_NONZERO", "TIMEOUT", "DEADLINE_EXHAUSTED", "LAUNCH_FAILED",
    "OUTPUT_OVERFLOW", "CAPTURE_INCOMPLETE", "OWNED_GROUP_REMAINED",
}
PUBLIC_FILES = {"project-prerequisite.zip.p7m", "summary.json"}


class Rejected(ValueError):
    """Only fixed internal codes may reach the diagnostic control flow."""


class ObservationExpired(BaseException):
    """Do not absorb the whole-envelope signal into an ordinary stage error."""


def require(condition, code):
    if not condition:
        raise Rejected(code)


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def utc():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


def finite_number(value):
    return type(value) in (int, float) and math.isfinite(value)


def duplicate_free(pairs):
    result = {}
    for key, value in pairs:
        require(key not in result, "DUPLICATE_JSON_KEY")
        result[key] = value
    return result


def canonical(value):
    return (json.dumps(value, sort_keys=True, separators=(",", ":"),
                       allow_nan=False) + "\n").encode()


def fresh_ancestors(path):
    result = []
    for parent in path.parents:
        info = parent.lstat()
        require(stat.S_ISDIR(info.st_mode), "ANCESTOR_NOT_DIRECTORY")
        result.append((str(parent), info.st_dev, info.st_ino, info.st_mode))
    return result


def file_identity(info):
    return (info.st_dev, info.st_ino, info.st_mode, info.st_size,
            info.st_mtime_ns, info.st_ctime_ns)


def read_regular(path, limit):
    """Fresh bounded read; no metadata, path, content or identity cache."""
    path = Path(path).absolute()
    before = path.lstat()
    require(stat.S_ISREG(before.st_mode), "FILE_NOT_REGULAR")
    require(0 <= before.st_size <= limit, "FILE_LIMIT")
    parents = fresh_ancestors(path)
    with path.open("rb") as stream:
        opened = os.fstat(stream.fileno())
        require(file_identity(opened) == file_identity(before), "FILE_CHANGED_AT_OPEN")
        raw = stream.read(limit + 1)
        after_handle = os.fstat(stream.fileno())
    require(len(raw) <= limit and len(raw) == before.st_size, "FILE_LIMIT_OR_LENGTH")
    require(file_identity(before) == file_identity(after_handle) ==
            file_identity(path.lstat()), "FILE_CHANGED_DURING_READ")
    require(fresh_ancestors(path) == parents, "ANCESTOR_CHANGED")
    return raw


def write_json(path, value):
    raw = canonical(value)
    require(len(raw) <= METADATA_LIMIT, "METADATA_LIMIT")
    with Path(path).open("xb") as stream:
        stream.write(raw)
    return raw


def read_json(path):
    return parse_json(read_regular(path, METADATA_LIMIT))


def parse_json(raw):
    def nonfinite(value):
        raise Rejected("NONFINITE_JSON")
    def real(value):
        parsed = float(value)
        require(math.isfinite(parsed), "NONFINITE_JSON")
        return parsed
    return json.loads(raw, object_pairs_hook=duplicate_free, parse_constant=nonfinite,
                      parse_float=real)


def binding(path, limit, *, complete=True):
    raw = read_regular(path, limit)
    return {"bytes": len(raw), "sha256": sha(raw), "complete": complete}


def group_observation(pid):
    """Only ESRCH establishes absence; denied/failed probes remain unknown."""
    try:
        os.killpg(pid, 0)
        return {"state": "PRESENT", "present": True, "errno": None}
    except ProcessLookupError:
        return {"state": "ABSENT", "present": False, "errno": None}
    except PermissionError as error:
        return {"state": "PERMISSION_DENIED", "present": None, "errno": error.errno}
    except OSError as error:
        return {"state": "QUERY_FAILED", "present": None, "errno": error.errno}


def kill_group(process):
    """Always attempt the owned group; denial cannot bypass final pipe cleanup."""
    try:
        os.killpg(process.pid, signal.SIGKILL)
        return {"state": "SENT", "signalSent": True, "errno": None}
    except ProcessLookupError:
        return {"state": "ABSENT", "signalSent": False, "errno": None}
    except PermissionError as error:
        return {"state": "PERMISSION_DENIED", "signalSent": False, "errno": error.errno}
    except OSError as error:
        return {"state": "SIGNAL_FAILED", "signalSent": False, "errno": error.errno}


def capture(command, allowance, *, output, error_output, hard_deadline,
            stream_limit=STREAM_LIMIT, execution_deadline=None):
    """Bounded POSIX pipe capture, sharing one stage deadline with drain/wait.

    A zero parent exit cannot bypass held pipe descriptors or group cleanup.
    Only the caller's fixed command router can invoke this for prerequisite work.
    """
    require(os.name == "posix", "POSIX_PROCESS_GROUP_REQUIRED")
    require(finite_number(allowance) and allowance >= 0, "ALLOWANCE_TYPE")
    require(type(stream_limit) is int and 0 < stream_limit <= STREAM_LIMIT, "STREAM_LIMIT")
    start = time.monotonic()
    deadline = min(start + allowance, hard_deadline,
                   execution_deadline if execution_deadline is not None else hard_deadline)
    observed = {"result": "DEADLINE_EXHAUSTED", "exitCode": None, "elapsedMs": 0}
    details = {
        "startedAt": utc(), "stdin": "DEVNULL", "pid": None,
        "returnCodeObserved": None, "cleanupGroupSignalSent": False,
        "cleanupParentReaped": True, "groupPresentAfterCleanup": None,
        "groupBeforeCleanup": None, "groupAfterCleanup": None,
        "cleanupGroupSignal": None,
        "stdout": {"bytes": 0, "overflow": False, "eof": False, "sha256": sha(b"")},
        "stderr": {"bytes": 0, "overflow": False, "eof": False, "sha256": sha(b"")},
    }
    if deadline <= start:
        details["finishedAt"] = utc()
        return observed, details
    process, selector = None, selectors.DefaultSelector()
    result, exit_code = "CAPTURE_INCOMPLETE", None
    stream_targets = {"stdout": output, "stderr": error_output}
    hashes = {name: hashlib.sha256() for name in stream_targets}
    try:
        try:
            process = subprocess.Popen(command, stdin=subprocess.DEVNULL,
                stdout=subprocess.PIPE, stderr=subprocess.PIPE, start_new_session=True)
        except OSError:
            result = "LAUNCH_FAILED"
            return {"result": result, "exitCode": None,
                    "elapsedMs": round((time.monotonic() - start) * 1000)}, details
        details["pid"] = process.pid
        for name, pipe in (("stdout", process.stdout), ("stderr", process.stderr)):
            os.set_blocking(pipe.fileno(), False)
            selector.register(pipe, selectors.EVENT_READ, name)
        while selector.get_map():
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                result = "TIMEOUT"
                break
            for key, _ in selector.select(min(remaining, 0.1)):
                name = key.data
                record = details[name]
                take = min(65536, stream_limit + 1 - record["bytes"])
                try:
                    chunk = os.read(key.fileobj.fileno(), take)
                except BlockingIOError:
                    continue
                if not chunk:
                    record["eof"] = True
                    selector.unregister(key.fileobj)
                    continue
                stream_targets[name].write(chunk)
                hashes[name].update(chunk)
                record["bytes"] += len(chunk)
                if record["bytes"] > stream_limit:
                    record["overflow"] = True
                    result = "OUTPUT_OVERFLOW"
                    break
            if result == "OUTPUT_OVERFLOW":
                break
        else:
            try:
                exit_code = process.wait(timeout=max(0, deadline - time.monotonic()))
                result = "PASS" if exit_code == 0 else "EXIT_NONZERO"
            except subprocess.TimeoutExpired:
                result = "TIMEOUT"
        if result == "PASS" and time.monotonic() >= deadline:
            result = "DEADLINE_EXHAUSTED"
        if result == "PASS":
            details["groupBeforeCleanup"] = group_observation(process.pid)
            present = details["groupBeforeCleanup"]["present"]
            if present is not False:
                result = "OWNED_GROUP_REMAINED" if present is True else "CAPTURE_INCOMPLETE"
    finally:
        if process is not None:
            # No poll()!=None shortcut: descendants may outlive the parent.
            details["cleanupGroupSignal"] = kill_group(process)
            details["cleanupGroupSignalSent"] = details["cleanupGroupSignal"]["signalSent"]
            try:
                process.wait(timeout=max(0, min(2, hard_deadline - time.monotonic())))
                details["cleanupParentReaped"] = True
            except subprocess.TimeoutExpired:
                details["cleanupParentReaped"] = False
            details["returnCodeObserved"] = process.returncode
            details["groupAfterCleanup"] = group_observation(process.pid)
            details["groupPresentAfterCleanup"] = details["groupAfterCleanup"]["present"]
            # Retain only already available bytes after cancellation, bounded by
            # the same stream cap. Never block waiting for a surviving writer.
            for key in list(selector.get_map().values()):
                name, record = key.data, details[key.data]
                while record["bytes"] <= stream_limit and time.monotonic() < hard_deadline:
                    try:
                        chunk = os.read(key.fileobj.fileno(),
                                        min(65536, stream_limit + 1 - record["bytes"]))
                    except BlockingIOError:
                        break
                    if not chunk:
                        record["eof"] = True
                        break
                    stream_targets[name].write(chunk)
                    hashes[name].update(chunk)
                    record["bytes"] += len(chunk)
                    if record["bytes"] > stream_limit:
                        record["overflow"] = True
                        result = "OUTPUT_OVERFLOW"
                        break
            for pipe in (process.stdout, process.stderr):
                pipe.close()
        selector.close()
        output.flush()
        error_output.flush()
        for name in stream_targets:
            details[name]["sha256"] = hashes[name].hexdigest()
        details["finishedAt"] = utc()
    if result == "PASS" and (not details["cleanupParentReaped"] or
            details["cleanupGroupSignal"]["state"] not in {"SENT", "ABSENT"} or
            details["groupPresentAfterCleanup"] is not False or
            any(not details[name]["eof"] or details[name]["overflow"]
                for name in ("stdout", "stderr"))):
        result = "CAPTURE_INCOMPLETE"
    elapsed = time.monotonic() - start
    if result == "PASS" and elapsed >= allowance:
        result = "DEADLINE_EXHAUSTED"
    return {"result": result, "exitCode": exit_code,
            "elapsedMs": round(elapsed * 1000)}, details


class MonotonicClock:
    """Expose the exact monotonic samples consumed by unchanged prepare()."""
    def __init__(self):
        self.started = self.last = None

    def __call__(self):
        value = time.monotonic()
        if self.started is None:
            self.started = value
        self.last = value
        return value


def bound_read(path, limit):
    before = file_identity(Path(path).lstat())
    raw = read_regular(path, limit)
    require(file_identity(Path(path).lstat()) == before, "BOUND_IDENTITY_CHANGED")
    return raw, list(before)


def verify_shell(shell, observed):
    supplied = Path(shell)
    require(list(file_identity(supplied.lstat())) == observed["suppliedIdentity"] and
            str(supplied.resolve(strict=True)) == observed["physical"], "BASH_CHANGED")
    raw, identity = bound_read(Path(observed["physical"]), 32 * 1024 * 1024)
    require(identity == observed["physicalIdentity"] and sha(raw) == observed["sha256"] and
            os.access(Path(observed["physical"]), os.X_OK), "BASH_CHANGED")


class StageAdapter:
    def __init__(self, repo, private, shell, clock, outer_deadline, shell_observation):
        self.repo, self.private = Path(repo), Path(private)
        self.shell, self.clock, self.outer_deadline = shell, clock, outer_deadline
        self.shell_observation = shell_observation
        self.records = []

    def __call__(self, command, allowance, *, output, error_output):
        index = len(self.records)
        require(index < 2, "REPEATED_STAGE")
        stage, script = SCRIPTS[index]
        require(command == [self.shell, "-c", script], "UNAPPROVED_COMMAND_OR_ORDER")
        require(Path.cwd() == self.repo / prerequisite.PROJECT, "STAGE_CWD")
        require(self.clock.started is not None and self.clock.last is not None, "CLOCK_NOT_STARTED")
        expected = max(0, min(STAGE_LIMITS[stage],
                           prerequisite.BUDGET_SECONDS - (self.clock.last - self.clock.started)))
        require(finite_number(allowance) and allowance == expected, "ALLOWANCE_CHANGED")
        for stream, suffix in ((output, ".stdout"), (error_output, ".stderr")):
            wanted = self.private / "builds/project-prerequisite" / (stage + suffix)
            require(Path(stream.name) == wanted and
                    stat.S_ISREG(os.fstat(stream.fileno()).st_mode) and
                    file_identity(os.fstat(stream.fileno())) ==
                    file_identity(wanted.lstat()), "STAGE_OUTPUT_HANDLE")
        require(output.fileno() != error_output.fileno(), "SHARED_STAGE_HANDLE")
        verify_shell(self.shell, self.shell_observation)
        intent = {"stage": stage, "argv": command, "cwd": str(Path.cwd()),
                  "allowanceSeconds": allowance, "startedAt": utc(), "stdin": "DEVNULL"}
        write_json(self.private / (stage + ".intent.json"), intent)
        observed, detail = capture(command, allowance, output=output, error_output=error_output,
            hard_deadline=self.outer_deadline,
            execution_deadline=min(self.clock.last + allowance,
                                   self.clock.started + prerequisite.BUDGET_SECONDS))
        for stream, handle in (("stdout", output), ("stderr", error_output)):
            detail[stream]["outputIdentity"] = list(file_identity(os.fstat(handle.fileno())))
        record = {**intent, **observed, "capture": detail}
        write_json(self.private / (stage + ".process.json"), record)
        self.records.append(record)
        return observed


def small_command(command, deadline, maximum):
    stdout, stderr = io.BytesIO(), io.BytesIO()
    allowance = max(0, min(maximum, deadline - time.monotonic()))
    observed, detail = capture(command, allowance, output=stdout, error_output=stderr,
                              hard_deadline=deadline, stream_limit=METADATA_LIMIT)
    out, err = stdout.getvalue(), stderr.getvalue()
    record = {"argv": command, **observed,
              "stdout": {"bytes": len(out), "sha256": sha(out)},
              "stderr": {"bytes": len(err), "sha256": sha(err)}, "capture": detail}
    require(observed["result"] == "PASS" and not err, "IDENTITY_COMMAND_FAILED")
    return out, record


def source_limit(name, *, qa=False):
    # The original native CLI already bounds these larger script/Swift inputs
    # to 4 MiB. Standalone observation and metadata retain their 64 KiB limit.
    if qa and name in prerequisite.terminal.SOURCE_FILES:
        return 4 * prerequisite.terminal.MAX_PARSE_BYTES
    return METADATA_LIMIT


def source_binding(repo, expected, deadline, *, clean, qa=False):
    names = QA_SOURCE_FILES if qa else SOURCE_FILES
    head_raw, head_record = small_command(
        ["git", "--no-replace-objects", "rev-parse", "HEAD"], deadline, 3)
    require(head_raw.decode("ascii").strip() == expected, "SOURCE_HEAD")
    commands = [head_record]
    if clean:
        status, status_record = small_command(
            ["git", "--no-replace-objects", "status", "--porcelain=v1"], deadline, 3)
        commands.append(status_record)
        require(status == b"", "SOURCE_DIRTY_BEFORE")
    tree, tree_record = small_command(
        ["git", "--no-replace-objects", "ls-tree", "-rz", expected, "--", *names],
        deadline, 5)
    commands.append(tree_record)
    blobs = {}
    for line in tree.split(b"\0"):
        if not line:
            continue
        meta, relative = line.split(b"\t", 1)
        mode, kind, digest = meta.decode("ascii").split()
        name = relative.decode("utf-8")
        require(name not in blobs and kind == "blob" and
                mode in {"100644", "100755"}, "SOURCE_TREE_ENTRY")
        blobs[name] = digest
    require(set(blobs) == set(names), "SOURCE_INVENTORY")
    rows = []
    for name in names:
        raw, identity = bound_read(repo / name, source_limit(name, qa=qa))
        blob = hashlib.sha1(b"blob " + str(len(raw)).encode() + b"\0" + raw).hexdigest()
        require(blob == blobs[name], "SOURCE_BYTES")
        rows.append({"path": name, "gitBlob": blob, "sha256": sha(raw), "bytes": len(raw),
                     "identity": identity})
    require(sha(read_regular(repo / RECIPIENT, METADATA_LIMIT)) == RECIPIENT_SHA256,
            "RECIPIENT_CHANGED")
    return {"sourceSha": expected, "cleanRequiredBeforeOnly": clean,
            "files": rows, "commands": commands}


def environment_binding(shell, deadline):
    observations, commands = {}, []
    for name, argv, maximum in (
        ("os", ["/usr/bin/sw_vers"], 3),
        ("developer", ["/usr/bin/xcode-select", "-p"], 3),
        ("xcode", ["/usr/bin/xcodebuild", "-version"], 10),
    ):
        raw, record = small_command(argv, deadline, maximum)
        commands.append(record)
        observations[name] = raw.decode("utf-8", errors="strict")
    fields = {}
    for line in observations["os"].splitlines():
        key, separator, value = line.partition(":")
        require(separator and key not in fields, "OS_IDENTITY_FORMAT")
        fields[key] = value.strip()
    require(set(fields) == {"ProductName", "ProductVersion", "BuildVersion"} and
            fields["ProductName"] == "macOS", "OS_IDENTITY_FORMAT")
    match = re.fullmatch(r"Xcode ([0-9.]+)\nBuild version ([A-Za-z0-9]+)\n?",
                         observations["xcode"])
    require(match is not None, "XCODE_IDENTITY_FORMAT")
    actual = {
        "productVersion": fields["ProductVersion"], "buildVersion": fields["BuildVersion"],
        "xcodeVersion": match[1], "xcodeBuild": match[2],
        "developerDirectory": observations["developer"].strip(),
    }
    physical_shell = Path(shell).resolve(strict=True)
    require(os.access(physical_shell, os.X_OK), "BASH_NOT_EXECUTABLE")
    shell_raw, shell_identity = bound_read(physical_shell, 32 * 1024 * 1024)
    require(physical_shell.name == "bash", "BASH_NAME")
    return {"expected": EXPECTED_ENVIRONMENT, "actual": actual,
            "matched": actual == EXPECTED_ENVIRONMENT, "commands": commands,
            "bash": {"supplied": shell, "physical": str(physical_shell),
                     "sha256": sha(shell_raw), "bytes": len(shell_raw),
                     "suppliedIdentity": list(file_identity(Path(shell).lstat())),
                     "physicalIdentity": shell_identity},
            "python": {"executable": sys.executable, "version": sys.version}}


def validate_context(source, run_id, attempt):
    require(type(source) is str and re.fullmatch(r"[0-9a-f]{40}", source), "SOURCE_ID")
    require(type(run_id) is str and re.fullmatch(r"[1-9][0-9]{0,19}", run_id), "RUN_ID")
    require(type(attempt) is str and re.fullmatch(r"[1-9][0-9]{0,3}", attempt), "RUN_ATTEMPT")
    return {"diagnosticSourceSha": source, "workflowRunId": run_id,
            "workflowRunAttempt": attempt, "prerequisiteOnly": True,
            "formalAcceptanceGranted": False}


def qa_context_binding(repo, root, source, run_id, fixture):
    """Retain the original native CLI context; never trust a PREPARED token."""
    repo, root = Path(repo).absolute(), Path(root).absolute()
    require(root == repo / "artifacts/cherri-ios27-simulator-qa" / run_id,
            "QA_ARTIFACT_ROOT")
    fresh_ancestors(root / "probe")
    manifest = prerequisite.terminal.contained(root, "compiler-manifest.json")
    raw, identity = bound_read(manifest, 4 * prerequisite.terminal.MAX_PARSE_BYTES)
    expected = prerequisite.terminal.context(root, source, run_id, fixture)
    require(expected["compilerManifestSha256"] == sha(raw), "QA_MANIFEST_CONTEXT_CHANGED")
    after, after_identity = bound_read(manifest, 4 * prerequisite.terminal.MAX_PARSE_BYTES)
    require(after == raw and after_identity == identity, "QA_MANIFEST_CONTEXT_CHANGED")
    return {"context": expected, "compilerManifest": {
        "bytes": len(raw), "sha256": sha(raw), "identity": identity}}


def retain_qa_manifest(private, root, observed):
    original = prerequisite.terminal.contained(root, "compiler-manifest.json")
    raw, identity = bound_read(original, QA_MANIFEST_LIMIT)
    require({"bytes": len(raw), "sha256": sha(raw), "identity": identity} ==
            observed["compilerManifest"], "QA_MANIFEST_CHANGED_BEFORE_COPY")
    target = private / QA_MANIFEST
    with target.open("xb") as stream:
        stream.write(raw)
    copied, copied_identity = bound_read(target, QA_MANIFEST_LIMIT)
    require(copied == raw, "QA_MANIFEST_COPY_CHANGED")
    return {"bytes": len(raw), "sha256": sha(raw), "identity": copied_identity}


def qa_packet_context(context, observation):
    before = observation["before"]
    return {**context, "executionScope": QA_SCOPE,
            "qaContextSha256": sha(canonical(before["context"])) if before else None,
            "qaContextMatchedBefore": observation["matchedBefore"],
            "qaContextMatchedAfter": observation["matchedAfter"]}


def execution_fields(*, qa=False):
    if qa:
        # These fields describe only this collector. The surrounding QA run
        # has already compiled a fixture and may have started its simulator.
        return {"collectorActionsStarted": {
            "compiler": False, "simulator": False, "nativeTest": False,
            "modelRequest": False}, "priorWorkflowPhasesAssessed": False}
    return {"simulatorStarted": False, "nativeTestStarted": False,
            "compilerStarted": False, "modelRequestStarted": False}


def stage_summary(private, records):
    result = []
    for index, (name, _) in enumerate(SCRIPTS):
        if index >= len(records):
            result.append({"stage": name, "status": "NOT_STARTED"})
            continue
        record = records[index]
        require(record["stage"] == name and
                canonical(read_json(private / (name + ".process.json"))) == canonical(record),
                "PROCESS_RECEIPT_CHANGED")
        status, code, elapsed = record["result"], record["exitCode"], record["elapsedMs"]
        require(status in PROCESS_RESULTS, "PROCESS_RESULT")
        require(code is None or type(code) is int and -(2 ** 31) <= code < 2 ** 31, "EXIT_CODE_TYPE")
        require(type(elapsed) is int and 0 <= elapsed <= OUTER_SECONDS * 1000, "ELAPSED_TYPE")
        allowance = record["allowanceSeconds"]
        require(finite_number(allowance) and 0 <= allowance <= STAGE_LIMITS[name],
                "ALLOWANCE_TYPE")
        streams = {}
        for stream in ("stdout", "stderr"):
            captured = record["capture"][stream]
            require(type(captured["bytes"]) is int and
                    0 <= captured["bytes"] <= STREAM_LIMIT + 1 and
                    type(captured["eof"]) is bool and type(captured["overflow"]) is bool,
                    "STREAM_RECEIPT_TYPE")
            item = binding(private / "builds/project-prerequisite" / (name + "." + stream),
                           STREAM_LIMIT + 1,
                           complete=captured["eof"] and not captured["overflow"])
            require(item["bytes"] == captured["bytes"], "STREAM_SIZE_CHANGED")
            raw, identity = bound_read(private / "builds/project-prerequisite" /
                                       (name + "." + stream), STREAM_LIMIT + 1)
            require(identity == captured["outputIdentity"] and
                    sha(raw) == captured["sha256"] == item["sha256"], "STREAM_IDENTITY_OR_BYTES_CHANGED")
            streams[stream] = item
        result.append({"stage": name, "status": status, "exitCode": code,
                       "allowanceMs": round(allowance * 1000), "elapsedMs": elapsed,
                       "streams": streams})
    return result


def fixed_members(private, records, expected_bindings, *, qa=False):
    names = ["source.json", "identity.json", "diagnostic.json"]
    if qa and QA_MANIFEST in expected_bindings:
        names.append(QA_MANIFEST)
    receipt = private / "builds/project-prerequisite/receipt.json"
    if receipt.exists():
        names.append("builds/project-prerequisite/receipt.json")
    for row in records:
        stage = row["stage"]
        require(stage in STAGE_LIMITS, "MEMBER_STAGE")
        names.extend([stage + ".intent.json", stage + ".process.json",
                      "builds/project-prerequisite/" + stage + ".stdout",
                      "builds/project-prerequisite/" + stage + ".stderr"])
    require(len(names) == len(set(names)) and len(names) <= (13 if qa else 12), "MEMBER_INVENTORY")
    require(set(names) == set(expected_bindings), "MEMBER_INVENTORY")
    members, raw_members = [], {}
    for name in names:
        is_stream = name.endswith((".stdout", ".stderr"))
        limit = QA_MANIFEST_LIMIT if qa and name == QA_MANIFEST else \
            STREAM_LIMIT + 1 if is_stream else METADATA_LIMIT
        raw = read_regular(private / name, limit)
        require(name in expected_bindings and
                expected_bindings[name] == {"bytes": len(raw), "sha256": sha(raw),
                    "identity": list(file_identity((private / name).lstat()))},
                "ORIGINAL_CHANGED_BEFORE_ARCHIVE")
        complete = True
        if is_stream:
            stage, stream = Path(name).name.split(".")
            row = next(item for item in records if item["stage"] == stage)
            captured = row["capture"][stream]
            complete = captured["eof"] and not captured["overflow"]
        else:
            parse_json(raw)
        members.append({"name": name, "bytes": len(raw), "sha256": sha(raw),
                        "complete": complete})
        raw_members[name] = raw
    return members, raw_members


def make_archive(private, context, records, expected_bindings, *, qa=False):
    members, originals = fixed_members(private, records, expected_bindings, qa=qa)
    manifest = canonical({"schema": (QA_SCHEMA if qa else SCHEMA) + "/originals",
                          **context, "members": members})
    require(len(manifest) <= METADATA_LIMIT and
            sum(len(value) for value in originals.values()) + len(manifest) + 8192 <= ARCHIVE_LIMIT,
            "ARCHIVE_LIMIT")
    buffer = io.BytesIO()
    with zipfile.ZipFile(buffer, "w", compression=zipfile.ZIP_STORED) as archive:
        for name, raw in [*originals.items(), ("manifest.json", manifest)]:
            info = zipfile.ZipInfo(name)
            info.external_attr = 0o600 << 16
            archive.writestr(info, raw)
    encoded = buffer.getvalue()
    require(len(encoded) <= ARCHIVE_LIMIT, "ARCHIVE_LIMIT")
    # Read the created archive, not merely the prospective member inventory.
    with zipfile.ZipFile(io.BytesIO(encoded), "r") as check:
        require(check.namelist() == [*originals, "manifest.json"], "ARCHIVE_MEMBER_CHANGED")
        for row in members:
            raw = check.read(row["name"])
            require(len(raw) == row["bytes"] and sha(raw) == row["sha256"],
                    "ARCHIVE_MEMBER_CHANGED")
        require(check.read("manifest.json") == manifest, "ARCHIVE_MANIFEST_CHANGED")
    path = private / "project-prerequisite.zip"
    with path.open("xb") as stream:
        stream.write(encoded)
    require(read_regular(path, ARCHIVE_LIMIT) == encoded, "ARCHIVE_WRITE_CHANGED")
    return path, {"bytes": len(encoded), "sha256": sha(encoded)}, members


def encrypt_archive(private, public, archive, archive_binding, recipient, deadline):
    deadline = min(deadline, time.monotonic() + ENCRYPT_SECONDS)
    recipient_raw, recipient_identity = bound_read(recipient, METADATA_LIMIT)
    require(sha(recipient_raw) == RECIPIENT_SHA256,
            "RECIPIENT_CHANGED")
    _, archive_identity = bound_read(archive, ARCHIVE_LIMIT)
    require(binding(archive, ARCHIVE_LIMIT) ==
            {**archive_binding, "complete": True}, "ARCHIVE_CHANGED")
    cipher = public / "project-prerequisite.zip.p7m"
    require(not cipher.exists() and not cipher.is_symlink(), "CIPHERTEXT_EXISTS")
    command = ["/usr/bin/openssl", "smime", "-encrypt", "-binary", "-aes256",
               "-in", str(archive), "-out", str(cipher), "-outform", "DER", str(recipient)]
    with (private / "encryption.stdout").open("xb") as stdout, \
         (private / "encryption.stderr").open("xb") as stderr:
        observed, detail = capture(command, min(ENCRYPT_SECONDS, max(0, deadline - time.monotonic())),
            output=stdout, error_output=stderr, hard_deadline=deadline, stream_limit=METADATA_LIMIT)
    write_json(private / "encryption.process.json",
               {"argv": command, **observed, "capture": detail, "stdin": "DEVNULL"})
    require(observed["result"] == "PASS" and time.monotonic() < deadline and
            not read_regular(private / "encryption.stdout", METADATA_LIMIT) and
            not read_regular(private / "encryption.stderr", METADATA_LIMIT), "ENCRYPTION_FAILED")
    recipient_after, recipient_after_identity = bound_read(recipient, METADATA_LIMIT)
    require(sha(recipient_after) == RECIPIENT_SHA256 and recipient_after_identity == recipient_identity,
            "RECIPIENT_CHANGED")
    _, archive_after_identity = bound_read(archive, ARCHIVE_LIMIT)
    require(archive_after_identity == archive_identity, "ARCHIVE_CHANGED")
    require(binding(archive, ARCHIVE_LIMIT) ==
            {**archive_binding, "complete": True}, "ARCHIVE_CHANGED")
    encoded = read_regular(cipher, CIPHER_LIMIT)
    require(0 < len(encoded) <= CIPHER_LIMIT and encoded.startswith(b"\x30"), "CIPHERTEXT_INVALID")
    return {"bytes": len(encoded), "sha256": sha(encoded)}


def verify_public(public, summary):
    validate_public_summary(summary)
    require({p.name for p in public.iterdir()} == PUBLIC_FILES, "PUBLICATION_MEMBER")
    require(read_regular(public / "summary.json", METADATA_LIMIT) == canonical(summary),
            "PUBLIC_SUMMARY_CHANGED")
    require(binding(public / "project-prerequisite.zip.p7m", CIPHER_LIMIT) ==
            {**summary["ciphertext"], "complete": True}, "PUBLIC_CIPHERTEXT_CHANGED")


def validate_public_summary(value):
    require(type(value) is dict, "PUBLIC_SUMMARY_FIELDS")
    qa = value.get("schema") == QA_SCHEMA
    fields = {"schema", "diagnosticSourceSha", "workflowRunId", "workflowRunAttempt",
              "prerequisiteOnly", "formalAcceptanceGranted", "sourceMatchedBefore",
              "sourceMatchedAfter", "environmentMatched", "reason", "prerequisiteStatus",
              "captureComplete", "stages", "members", "plaintextArchive", "ciphertext",
              "recipientSha256", "elapsedMs", "budgetsSeconds", "simulatorStarted",
              "nativeTestStarted", "compilerStarted", "modelRequestStarted"}
    if qa:
        fields.difference_update({"simulatorStarted", "nativeTestStarted",
                                  "compilerStarted", "modelRequestStarted"})
        fields.update({"executionScope", "qaContextSha256", "qaContextMatchedBefore",
                       "qaContextMatchedAfter", "collectorActionsStarted",
                       "priorWorkflowPhasesAssessed"})
    require(type(value) is dict and set(value) == fields, "PUBLIC_SUMMARY_FIELDS")
    require(value["schema"] == (QA_SCHEMA if qa else SCHEMA), "PUBLIC_SCHEMA")
    validate_context(value["diagnosticSourceSha"], value["workflowRunId"], value["workflowRunAttempt"])
    require(value["prerequisiteOnly"] is True and value["formalAcceptanceGranted"] is False,
            "PUBLIC_ACCEPTANCE")
    if qa:
        require(value["executionScope"] == QA_SCOPE and
                value["priorWorkflowPhasesAssessed"] is False and
                type(value["collectorActionsStarted"]) is dict and
                set(value["collectorActionsStarted"]) == {"compiler", "simulator", "nativeTest", "modelRequest"} and
                all(item is False for item in value["collectorActionsStarted"].values()),
                "PUBLIC_QA_EXECUTION_SCOPE")
        require(type(value["qaContextMatchedBefore"]) is bool and
                type(value["qaContextMatchedAfter"]) is bool and
                (not value["qaContextMatchedAfter"] or value["qaContextMatchedBefore"]),
                "PUBLIC_QA_CONTEXT_STATE")
        context_sha = value["qaContextSha256"]
        require((value["qaContextMatchedBefore"] and type(context_sha) is str and
                 re.fullmatch(r"[0-9a-f]{64}", context_sha)) or
                (not value["qaContextMatchedBefore"] and context_sha is None),
                "PUBLIC_QA_CONTEXT_DIGEST")
        require(value["prerequisiteStatus"] != "PREPARED" or value["qaContextMatchedAfter"],
                "PUBLIC_QA_CONTEXT_UNCONFIRMED")
    else:
        require(all(value[key] is False for key in
            ("simulatorStarted", "nativeTestStarted", "compilerStarted", "modelRequestStarted")),
            "PUBLIC_ACCEPTANCE")
    require(all(type(value[key]) is bool for key in
        ("sourceMatchedBefore", "sourceMatchedAfter", "environmentMatched", "captureComplete")),
        "PUBLIC_BOOLEAN_TYPE")
    require(value["reason"] in {"NONE", "SOURCE_REJECTED", "ENVIRONMENT_CHANGED",
        "PREREQUISITE_FAILED", "SOURCE_OR_CAPTURE_REJECTED"}, "PUBLIC_REASON")
    require(value["prerequisiteStatus"] in {"PREPARED", "FAILED", "NOT_STARTED"},
            "PUBLIC_PREREQUISITE")
    require(type(value["elapsedMs"]) is int and 0 <= value["elapsedMs"] < 310000,
            "PUBLIC_ELAPSED")
    require(type(value["budgetsSeconds"]) is dict and
            all(type(item) is int for item in value["budgetsSeconds"].values()) and
            value["budgetsSeconds"] == {"install": 180, "generate": 60, "aggregate": 240,
                                      "preflight": 30, "encryption": 20, "outer": 310},
            "PUBLIC_BUDGET")
    require(value["recipientSha256"] == RECIPIENT_SHA256, "PUBLIC_RECIPIENT")
    def digest(record, limit, complete=False):
        keys = {"bytes", "sha256", "complete"} if complete else {"bytes", "sha256"}
        require(type(record) is dict and set(record) == keys and
                type(record["bytes"]) is int and 0 <= record["bytes"] <= limit and
                type(record["sha256"]) is str and re.fullmatch(r"[0-9a-f]{64}", record["sha256"]),
                "PUBLIC_BINDING")
        if complete:
            require(type(record["complete"]) is bool, "PUBLIC_COMPLETE_TYPE")
    digest(value["plaintextArchive"], ARCHIVE_LIMIT)
    digest(value["ciphertext"], CIPHER_LIMIT)
    require(type(value["stages"]) is list and len(value["stages"]) == 2, "PUBLIC_STAGES")
    names = {"source.json", "identity.json", "diagnostic.json"}
    if qa and value["qaContextMatchedBefore"]:
        names.add(QA_MANIFEST)
    any_started = False
    for index, item in enumerate(value["stages"]):
        name = SCRIPTS[index][0]
        require(type(item) is dict and item.get("stage") == name, "PUBLIC_STAGE_ORDER")
        if item.get("status") == "NOT_STARTED":
            require(set(item) == {"stage", "status"}, "PUBLIC_UNSTARTED_FIELDS")
            continue
        any_started = True
        require(set(item) == {"stage", "status", "exitCode", "allowanceMs", "elapsedMs", "streams"} and
                item["status"] in PROCESS_RESULTS, "PUBLIC_STAGE_FIELDS")
        require(item["exitCode"] is None or type(item["exitCode"]) is int and
                -(2 ** 31) <= item["exitCode"] < 2 ** 31, "PUBLIC_EXIT_TYPE")
        require(type(item["allowanceMs"]) is int and 0 <= item["allowanceMs"] <= STAGE_LIMITS[name] * 1000 and
                type(item["elapsedMs"]) is int and 0 <= item["elapsedMs"] <= 310000,
                "PUBLIC_STAGE_TIMING")
        require(type(item["streams"]) is dict and set(item["streams"]) == {"stdout", "stderr"},
                "PUBLIC_STREAMS")
        for stream in item["streams"].values():
            digest(stream, STREAM_LIMIT + 1, True)
        names.update({name + ".intent.json", name + ".process.json",
                      "builds/project-prerequisite/" + name + ".stdout",
                      "builds/project-prerequisite/" + name + ".stderr"})
    if any_started or value["prerequisiteStatus"] != "NOT_STARTED":
        names.add("builds/project-prerequisite/receipt.json")
    require(type(value["members"]) is list and 3 <= len(value["members"]) <= (13 if qa else 12), "PUBLIC_MEMBERS")
    observed_names = []
    for member in value["members"]:
        require(type(member) is dict and set(member) == {"name", "bytes", "sha256", "complete"} and
                member["name"] in names, "PUBLIC_MEMBER_NAME")
        digest({key: member[key] for key in ("bytes", "sha256", "complete")},
               QA_MANIFEST_LIMIT if qa and member["name"] == QA_MANIFEST else
               STREAM_LIMIT + 1 if member["name"].endswith((".stdout", ".stderr")) else METADATA_LIMIT, True)
        observed_names.append(member["name"])
    require(len(set(observed_names)) == len(observed_names) and set(observed_names) == names,
            "PUBLIC_MEMBER_INVENTORY")


def confirm_sources(repo, before, *, qa=False):
    names = QA_SOURCE_FILES if qa else SOURCE_FILES
    require({row["path"] for row in before["files"]} == set(names), "SOURCE_INVENTORY")
    for row in before["files"]:
        raw, identity = bound_read(repo / row["path"], source_limit(row["path"], qa=qa))
        require(identity == row["identity"] and len(raw) == row["bytes"] and
                sha(raw) == row["sha256"], "SOURCE_CHANGED_AT_SEAL")


def validate_private(private, metadata, records, receipt, *, qa_manifest=None):
    expected = {}
    objects = dict(metadata)
    if receipt is not None:
        objects["builds/project-prerequisite/receipt.json"] = receipt
    for row in records:
        stage = row["stage"]
        objects[stage + ".process.json"] = row
        objects[stage + ".intent.json"] = {key: row[key] for key in
            ("stage", "argv", "cwd", "allowanceSeconds", "startedAt", "stdin")}
    for name, obj in objects.items():
        raw, identity = bound_read(private / name, METADATA_LIMIT)
        require(canonical(parse_json(raw)) == canonical(obj), "METADATA_CHANGED")
        expected[name] = {"bytes": len(raw), "sha256": sha(raw), "identity": identity}
    for row in records:
        for stream in ("stdout", "stderr"):
            name = "builds/project-prerequisite/" + row["stage"] + "." + stream
            raw, identity = bound_read(private / name, STREAM_LIMIT + 1)
            capture_record = row["capture"][stream]
            require(identity == capture_record["outputIdentity"] and
                    len(raw) == capture_record["bytes"] and
                    sha(raw) == capture_record["sha256"], "STREAM_IDENTITY_OR_BYTES_CHANGED")
            expected[name] = {"bytes": len(raw), "sha256": sha(raw), "identity": identity}
    if qa_manifest is not None:
        raw, identity = bound_read(private / QA_MANIFEST, QA_MANIFEST_LIMIT)
        require({"bytes": len(raw), "sha256": sha(raw), "identity": identity} == qa_manifest,
                "QA_MANIFEST_COPY_CHANGED")
        parse_json(raw)
        expected[QA_MANIFEST] = qa_manifest
    # A partial started intent or unknown stream is never silently omitted.
    actual = {p.relative_to(private).as_posix() for p in private.rglob("*") if p.is_file()}
    require(actual == set(expected), "UNACCOUNTED_PRIVATE_MEMBER")
    return expected


def ci_context(source, run_id, attempt):
    expected = {"GITHUB_SHA": source, "GITHUB_RUN_ID": run_id,
                "GITHUB_RUN_ATTEMPT": attempt, "GITHUB_REPOSITORY": "davidpovarsky/cherri",
                "GITHUB_EVENT_NAME": "workflow_dispatch"}
    require(all(os.environ.get(k) == v for k, v in expected.items()), "CI_CONTEXT_MISMATCH")
    require(os.environ.get("RUNNER_OS") == "macOS", "RUNNER_OS")
    return expected


def collect(repo, root, source, run_id, attempt, shell, github_output, *, started=None,
            qa_root=None, fixture_id=None):
    require(sys.platform == "darwin", "APPLE_DIAGNOSTIC_ONLY")
    qa_mode = qa_root is not None or fixture_id is not None
    require(not qa_mode or isinstance(qa_root, (str, Path)) and type(fixture_id) is str,
            "QA_ARGUMENT_PAIR")
    start = time.monotonic() if started is None else started
    started_at = utc()
    deadline = start + OUTER_SECONDS
    context = validate_context(source, run_id, attempt)
    observed_ci = ci_context(source, run_id, attempt)
    require(prerequisite.BUDGET_SECONDS == 240 and prerequisite.STAGE_SECONDS == STAGE_LIMITS,
            "PREREQUISITE_BUDGET_CHANGED")
    repo, root = Path(repo).absolute(), Path(root).absolute()
    require(Path.cwd() == repo, "COLLECTOR_CWD")
    require(repo != root and repo not in root.parents, "OUTPUT_INSIDE_SOURCE")
    require(stat.S_ISDIR(repo.lstat().st_mode), "REPO_DIRECTORY")
    fresh_ancestors(repo / "probe")
    if qa_mode:
        runner_temp = Path(os.environ["RUNNER_TEMP"]).absolute()
        require(root == runner_temp / ("cherri-qa-project-prerequisite-" + run_id + "-" + attempt),
                "QA_OBSERVATION_ROOT")
        fresh_ancestors(root)
    root.mkdir(mode=0o700, exist_ok=False)
    private, public = root / "private", root / "public"
    private.mkdir(mode=0o700)
    public.mkdir(mode=0o700)
    (private / "builds").mkdir()
    for module, name in (
        (sys.modules[__name__], "project_prerequisite_observation.py"),
        (prerequisite, "project_prerequisite.py"),
        (sys.modules["simulator_session_recovery"], "simulator_session_recovery.py"),
        (sys.modules["qa_terminal_receipts"], "qa_terminal_receipts.py"),
    ):
        require(Path(module.__file__).absolute() == repo / QA / name, "IMPORTED_SOURCE_PATH")

    preflight_deadline = min(deadline, start + PREFLIGHT_SECONDS)
    source_record = {"before": None, "after": None, "sourceMatchedBefore": False,
                     "sourceMatchedAfter": False}
    qa_record = {"before": None, "after": None, "matchedBefore": False, "matchedAfter": False,
                 "manifestOriginal": None}
    if qa_mode:
        source_record["qaContext"] = qa_record
    identity = {"ci": observed_ci, "matched": False}
    receipt, records, generated = None, [], None
    returncode, reason = 70, "SOURCE_REJECTED"
    try:
        before = source_binding(repo, source, preflight_deadline, clean=True, qa=qa_mode)
        source_record.update(before=before, sourceMatchedBefore=True)
        if qa_mode:
            # Preserve the native CLI's complete source guard and actual
            # compiler/fixture context. No public summary can replace them.
            prerequisite.terminal.verify_checkout(repo, source)
            require(time.monotonic() < preflight_deadline, "PREFLIGHT_DEADLINE")
            qa_record["before"] = qa_context_binding(repo, qa_root, source, run_id, fixture_id)
            qa_record["manifestOriginal"] = retain_qa_manifest(private, qa_root, qa_record["before"])
            qa_record["matchedBefore"] = True
        identity = {"ci": observed_ci, **environment_binding(shell, preflight_deadline)}
        if not identity["matched"]:
            reason = "ENVIRONMENT_CHANGED"
        else:
            reason = "PREREQUISITE_FAILED"
            clock = MonotonicClock()
            adapter = StageAdapter(repo, private, shell, clock, deadline, identity["bash"])
            returncode, receipt = prerequisite.prepare(
                repo, private, qa_record["before"]["context"] if qa_mode else context,
                shell, runner=adapter, now=clock)
            records = adapter.records
            # This records expected generated output; it does not call the
            # mutated checkout clean, reset it, or accept unrelated changes.
            target = repo / prerequisite.PROJECT / prerequisite.GENERATED
            if target.exists():
                raw, generated_identity = bound_read(target, 8 * 1024 * 1024)
                generated = {"path": prerequisite.PROJECT + "/" + prerequisite.GENERATED,
                             "bytes": len(raw), "sha256": sha(raw), "identity": generated_identity}
            metadata_deadline = min(deadline - ENCRYPT_SECONDS, time.monotonic() + 15)
            after = source_binding(repo, source, metadata_deadline, clean=False, qa=qa_mode)
            source_record["after"] = after
            require(after["files"] == before["files"], "SOURCE_CHANGED_AFTER_STAGE")
            status, status_receipt = small_command(
                ["git", "--no-replace-objects", "status", "--porcelain=v1", "--untracked-files=all"],
                metadata_deadline, 3)
            allowed_prefix = "?? " + prerequisite.PROJECT + "/CherriSimulatorQA.xcodeproj/"
            require(all(line.startswith(allowed_prefix) for line in status.decode("utf-8").splitlines()),
                    "UNEXPECTED_CHECKOUT_CHANGE")
            source_record["postStatus"] = status_receipt
            source_record["sourceMatchedAfter"] = True
            if qa_mode:
                qa_record["after"] = qa_context_binding(repo, qa_root, source, run_id, fixture_id)
                require(canonical(qa_record["after"]) == canonical(qa_record["before"]),
                        "QA_CONTEXT_CHANGED_AFTER_STAGE")
                qa_record["matchedAfter"] = True
            reason = "NONE" if returncode == 0 else "PREREQUISITE_FAILED"
    except (OSError, ValueError, UnicodeError, TypeError, KeyError):
        # Raw exception messages may contain paths or service output; only this
        # fixed diagnostic state is public. Existing originals remain private.
        returncode = 70
        reason = "SOURCE_OR_CAPTURE_REJECTED" if source_record["sourceMatchedBefore"] else "SOURCE_REJECTED"
    stages = stage_summary(private, records)
    complete = all(stage["status"] == "NOT_STARTED" or
                   all(stream["complete"] for stream in stage["streams"].values())
                   for stage in stages)
    prerequisite_status = receipt["status"] if receipt is not None else "NOT_STARTED"
    require(prerequisite_status in {"PREPARED", "FAILED", "NOT_STARTED"}, "PREREQUISITE_STATUS")
    if prerequisite_status == "PREPARED":
        require(returncode == 0 and len(records) == 2 and
                all(row["result"] == "PASS" for row in records) and
                type(receipt["elapsedMs"]) is int and receipt["elapsedMs"] < 240000 and
                source_record["sourceMatchedAfter"] and generated is not None and
                generated["sha256"] == receipt["projectSha256"], "LATE_OR_UNBOUND_PREPARED")
        require(not qa_mode or qa_record["matchedAfter"], "QA_CONTEXT_NOT_CONFIRMED")
    if qa_mode:
        context = qa_packet_context(context, qa_record)
    packet_schema = QA_SCHEMA if qa_mode else SCHEMA
    diagnostic = {"schema": packet_schema, **context, "startedAt": started_at,
                  "prerequisiteReturnCode": returncode, "reason": reason,
                  "generatedProject": generated, "captureComplete": complete,
                  "prerequisiteStatus": prerequisite_status,
                  "budgets": {"install": 180, "generate": 60, "aggregate": 240,
                              "preflight": 30, "encryption": 20, "outer": 310},
                  **execution_fields(qa=qa_mode)}
    metadata = {"source.json": source_record, "identity.json": identity,
                "diagnostic.json": diagnostic}
    for name, value in metadata.items():
        write_json(private / name, value)
    expected_members = validate_private(private, metadata, records, receipt,
                                        qa_manifest=qa_record["manifestOriginal"] if qa_mode else None)
    archive, archive_record, members = make_archive(private, context, records, expected_members, qa=qa_mode)
    if source_record["before"] is not None:
        confirm_sources(repo, source_record["before"], qa=qa_mode)
    if qa_mode and qa_record["before"] is not None:
        require(canonical(qa_context_binding(repo, qa_root, source, run_id, fixture_id)) ==
                canonical(qa_record["before"]), "QA_CONTEXT_CHANGED_AT_SEAL")
    cipher = encrypt_archive(private, public, archive, archive_record,
                             repo / RECIPIENT, deadline)
    if source_record["before"] is not None:
        confirm_sources(repo, source_record["before"], qa=qa_mode)
    if qa_mode and qa_record["before"] is not None:
        require(canonical(qa_context_binding(repo, qa_root, source, run_id, fixture_id)) ==
                canonical(qa_record["before"]), "QA_CONTEXT_CHANGED_AT_SEAL")
    elapsed = round((time.monotonic() - start) * 1000)
    require(elapsed < OUTER_SECONDS * 1000, "DIAGNOSTIC_DEADLINE")
    summary = {
        "schema": packet_schema, **context,
        "sourceMatchedBefore": source_record["sourceMatchedBefore"],
        "sourceMatchedAfter": source_record["sourceMatchedAfter"],
        "environmentMatched": identity["matched"], "reason": reason,
        "prerequisiteStatus": prerequisite_status, "captureComplete": complete,
        "stages": stages, "members": members, "plaintextArchive": archive_record,
        "ciphertext": cipher, "recipientSha256": RECIPIENT_SHA256, "elapsedMs": elapsed,
        "budgetsSeconds": diagnostic["budgets"], **execution_fields(qa=qa_mode),
    }
    write_json(public / "summary.json", summary)
    verify_public(public, summary)
    require(time.monotonic() < deadline, "DIAGNOSTIC_DEADLINE")
    # Only the fixed safe status is printed, never raw stdout/stderr or errors.
    print(json.dumps({"diagnosticPacket": "SEALED", "prerequisiteStatus": prerequisite_status,
                      "captureComplete": complete, "formalAcceptanceGranted": False}))
    with Path(github_output).open("a", encoding="ascii") as stream:
        stream.write("prerequisite_publication_ready=true\n" if qa_mode else "publication_ready=true\n")
    return returncode if complete else 70


def main():
    started = time.monotonic()
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("repo", "root", "source-sha", "run-id", "attempt", "bash", "github-output"):
        parser.add_argument("--" + name, required=True)
    parser.add_argument("--qa-root")
    parser.add_argument("--fixture-id")
    args = parser.parse_args()
    require(sys.platform == "darwin", "APPLE_DIAGNOSTIC_ONLY")
    def interrupted(signum, frame):
        raise ObservationExpired("DIAGNOSTIC_DEADLINE_OR_INTERRUPTION")
    previous = {kind: signal.signal(kind, interrupted)
                for kind in (signal.SIGALRM, signal.SIGTERM, signal.SIGINT)}
    signal.setitimer(signal.ITIMER_REAL, max(0.001, OUTER_SECONDS - (time.monotonic() - started)))
    try:
        return collect(args.repo, args.root, args.source_sha, args.run_id,
                       args.attempt, args.bash, args.github_output, started=started,
                       qa_root=args.qa_root, fixture_id=args.fixture_id)
    finally:
        signal.setitimer(signal.ITIMER_REAL, 0)
        for kind, handler in previous.items():
            signal.signal(kind, handler)


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (Exception, ObservationExpired):
        print("PROJECT_PREREQUISITE_OBSERVATION_REJECTED")
        raise SystemExit(70)
