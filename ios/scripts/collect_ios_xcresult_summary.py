#!/usr/bin/env python3
"""Seal a bounded, source-owned iOS xcresult summary without interpreting issues."""
import argparse
import contextlib
import ctypes
import datetime
import hashlib
import io
import json
import math
import os
from pathlib import Path
import re
import signal
import stat
import subprocess
import sys
import time
import types
import zipfile

SCHEMA = "cherri-ios-xcresult-summary/1"
REPOSITORY = "davidpovarsky/cherri"
WORKFLOW = ".github/workflows/ios-build.yml"
WORKFLOW_NAME = "iOS Build"
ARTIFACT_NAME = "ios-xcresult-summary"
RECIPIENT = "tools/shortcuts-simulator-qa/xcode_environment_inventory_recipient.pem"
RECIPIENT_SHA256 = "55956e825597a2e60acca4045e4ea8040ae27f17caa7964e9b4493824fa0c1f1"
STREAM_LIMIT = 1024 * 1024
METADATA_LIMIT = 32 * 1024
ARCHIVE_LIMIT = 3 * 1024 * 1024
CIPHER_LIMIT = ARCHIVE_LIMIT + 64 * 1024
OUTER_ZIP_LIMIT = ARCHIVE_LIMIT + 128 * 1024
SOURCE_LIMIT = 1024 * 1024
BUDGETS = {"preflightMs": 15000, "summaryMs": 20000,
           "encryptionMs": 20000, "postTestMs": 60000}
HELPERS = (
    "simulator_session_recovery", "qa_terminal_receipts",
    "project_prerequisite", "project_prerequisite_observation",
)
HELPER_FILES = tuple("tools/shortcuts-simulator-qa/" + n + ".py" for n in HELPERS)
SOURCE_FILES = (
    "ios/scripts/collect_ios_xcresult_summary.py",
    "ios/scripts/verify_ios_xcresult_summary.py",
    "ios/scripts/test_ios_xcresult_summary.py", WORKFLOW,
    *HELPER_FILES, RECIPIENT,
    "ios/CherriUITests/CherriEditorUITests.swift",
    "ios/CherriTests/CherriEditorInputTests.swift",
    "ios/CherriUITests/EditorStartupDeadline.swift", "ios/project.yml",
)
ABSENT_SOURCE = "ios/CherriApp/Views/CodeEditorView.swift"
PAYLOAD = ("identity.json", "command.json", "xcresult-summary.stdout",
           "xcresult-summary.stderr", "xcodebuild.exitcode")
MEMBERS = (*PAYLOAD, "manifest.json")
PUBLIC_FILES = {"ios-xcresult-summary.zip.p7m", "receipt.json"}
RESULTS = {"PASS", "EXIT_NONZERO", "TIMEOUT", "DEADLINE_EXHAUSTED",
           "LAUNCH_FAILED", "OUTPUT_OVERFLOW", "CAPTURE_INCOMPLETE",
           "OWNED_GROUP_REMAINED"}
REJECTIONS = {"CAPTURE_OVERFLOW", "SOURCE_CHANGED", "NATIVE_EXIT_INVALID",
              "RESERVATION_INVALID", "RESULT_INVALID", "ENCRYPTION_REJECTED",
              "DIAGNOSTIC_DEADLINE", "DIAGNOSTIC_REJECTED"}
HEX40 = re.compile(r"[0-9a-f]{40}\Z")
HEX64 = re.compile(r"[0-9a-f]{64}\Z")


class Rejected(ValueError):
    pass


class EnvelopeExpired(BaseException):
    pass


class PrepareFailureObservation:
    """Closed metadata for a rejected prepare; never a source or native receipt."""
    def __init__(self):
        self.started = time.monotonic()
        self.envelope_started = None
        self.stage_started = self.started
        self.stage = "arguments"
        self.file_index = None
        self.commands = []
        self.active_command = None
        self.command_overflow = False

    def mark(self, stage, file_index=None):
        self.stage, self.file_index = stage, file_index
        self.stage_started = time.monotonic()

    def git_started(self, args, allowance):
        operations = {
            ("rev-parse", "--verify", "HEAD"): "head",
            ("rev-parse", "--show-toplevel"): "toplevel",
            ("rev-parse", "HEAD^{tree}"): "tree",
        }
        operation = operations.get(tuple(args),
            "entries" if args[:3] == ["ls-tree", "-r", "-z"] else
            "status" if args[:3] == ["status", "--porcelain", "--untracked-files=no"] else
            "unclassified")
        self.mark("git_" + operation)
        if len(self.commands) >= 8:
            self.command_overflow = True
            self.active_command = None
            return
        row = {"operation": operation, "requestedTimeoutMs": round(allowance * 1000),
               "startedElapsedMs": round((self.stage_started - self.started) * 1000),
               "state": "NOT_RETURNED", "elapsedMs": None, "returnCode": None,
               "stdoutBytes": None, "stderrBytes": None}
        self.commands.append(row)
        self.active_command = row

    def git_returned(self, process):
        if self.active_command is not None:
            self.active_command.update(state="RETURNED", returnCode=process.returncode,
                elapsedMs=round((time.monotonic() - self.stage_started) * 1000),
                stdoutBytes=len(process.stdout), stderrBytes=len(process.stderr))
            self.active_command = None

    def emit_failure(self, error):
        now = time.monotonic()
        if self.active_command is not None:
            self.active_command.update(state="INTERRUPTED",
                elapsedMs=round((now - self.stage_started) * 1000))
            if isinstance(error, subprocess.TimeoutExpired):
                for key, raw in (("stdoutBytes", error.output), ("stderrBytes", error.stderr)):
                    self.active_command[key] = len(raw) if isinstance(raw, bytes) else None
        classes = (Rejected, EnvelopeExpired, subprocess.TimeoutExpired,
                   FileNotFoundError, PermissionError, OSError, ValueError, KeyError)
        kind = next((cls.__name__ for cls in classes if isinstance(error, cls)), "OtherException")
        codes = {"ARGUMENTS_REJECTED", "CONTEXT_REJECTED", "PATH_REJECTED", "SOURCE_CHANGED",
                 "RESERVATION_INVALID", "NATIVE_EXIT_INVALID", "DIAGNOSTIC_DEADLINE",
                 "FILE_REJECTED", "RESULT_INVALID", "OUTPUT_LIMIT", "SCHEMA_REJECTED",
                 "DIGEST_REJECTED", "JSON_REJECTED"}
        code = str(error) if isinstance(error, Rejected) and str(error) in codes else None
        value = {"schema": "cherri-ios-xcresult-prepare-failure/1", "mode": "prepare",
                 "lastStage": self.stage, "sourceFileIndex": self.file_index,
                 "elapsedMs": round((now - self.started) * 1000),
                 "stageElapsedMs": round((now - self.stage_started) * 1000),
                 "envelopeStarted": self.envelope_started is not None,
                 "envelopeElapsedMs": None if self.envelope_started is None else
                     round((now - self.envelope_started) * 1000),
                 "prepareEnvelopeMs": BUDGETS["preflightMs"],
                 "exceptionType": kind, "rejectionCode": code,
                 "commands": self.commands, "commandOverflow": self.command_overflow,
                 "processCleanupObserved": False, "formalAcceptanceGranted": False}
        raw = canonical(value)
        require(len(raw) <= 8192, "OUTPUT_LIMIT")
        sys.stderr.write("IOS_XCRESULT_PREPARE_DIAGNOSTIC:" + raw.decode("utf-8"))


_prepare_observation = None
_owned_git = None
_first_interruption = None

# This bootstrap permits only these already-qualified process primitives before
# the first Git read. The normal immutable Git/blob/physical checks still follow.
BOOTSTRAP_PINS = {
    "tools/shortcuts-simulator-qa/simulator_session_recovery.py": {
        "bytes": 19628, "sha256": "99f0cb92dc9ec32b2727f671f851053d53ae8e204b02d9405d0af85f188c230d"},
    "tools/shortcuts-simulator-qa/qa_terminal_receipts.py": {
        "bytes": 41639, "sha256": "cdeaf46fdcb81073fb6267ff2db54c94782b57660d5b61e1630af8ba1f804aa5"},
    "tools/shortcuts-simulator-qa/project_prerequisite.py": {
        "bytes": 4717, "sha256": "6c0bed088dc53e73af138221ce72a6faff0dfa0bc0279cc254ae622dd54bd843"},
    "tools/shortcuts-simulator-qa/project_prerequisite_observation.py": {
        "bytes": 69826, "sha256": "65148ffb9d2df8fd83c2887d1e56060d96ff2e7a978b9718753b74ab6ffdc24e"},
}


class OwnedGitReads:
    """Opt-in diagnostic ownership; no source authority is granted by capture."""
    def __init__(self, repo, output):
        self.repo, self.output = Path(repo), Path(output)
        self.helper = load_primitives(repo, None)
        self.records = []
        self.output.mkdir(mode=0o700)

    @staticmethod
    def process_image(pid):
        # Inspect only the just-created owned child. Absence is not an exec claim.
        try:
            if sys.platform == "darwin":
                lib = ctypes.CDLL("/usr/lib/libproc.dylib")
                lib.proc_pidpath.argtypes = [ctypes.c_int, ctypes.c_void_p, ctypes.c_uint32]
                lib.proc_pidpath.restype = ctypes.c_int
                buffer = ctypes.create_string_buffer(4096)
                count = lib.proc_pidpath(pid, buffer, len(buffer))
                path = buffer.value.decode("utf-8", "replace") if count > 0 else None
            else:
                path = os.readlink("/proc/%d/exe" % pid)
            return {"state": "OBSERVED" if path else "NOT_OBSERVED", "path": path}
        except (OSError, ValueError):
            return {"state": "NOT_OBSERVED", "path": None}

    def run(self, args, env, deadline):
        require(_first_interruption is None and len(self.records) < 24,
                "DIAGNOSTIC_DEADLINE")
        now = time.monotonic()
        require(deadline - now > 2, "DIAGNOSTIC_DEADLINE")
        directory = self.output / ("command-%02d" % (len(self.records) + 1))
        directory.mkdir(mode=0o700)
        argv = ["/usr/bin/env", "-i", *[k + "=" + v for k, v in sorted(env.items())],
                "/usr/bin/git", "--no-replace-objects", "-C", str(self.repo), *args]
        row = {"argv": argv, "executionDeadlineMonotonic": deadline - 2,
               "hardDeadlineMonotonic": deadline, "observed": None, "capture": None,
               "processImage": None}
        self.records.append(row)
        write_json(directory / "intent.original.json", row)

        def journal(phase, detail):
            # A Popen-return observation is not an assertion that exec completed.
            if phase == "started":
                row["processImage"] = self.process_image(detail["pid"])
            image = row["processImage"]
            value = {"phase": phase, "popenReturned": detail["pid"] is not None,
                     "gitImageObserved": image is not None and image["path"] is not None
                         and image["path"].endswith("/git"),
                     "processImage": image, "capture": detail}
            write_json(directory / (phase + ".original.json"), value)
            row["capture"] = json.loads(canonical(detail))

        with (directory / "stdout.original").open("xb") as stdout, \
                (directory / "stderr.original").open("xb") as stderr:
            observed, detail = self.helper.capture(
                argv, deadline - now - 2, output=stdout, error_output=stderr,
                execution_deadline=deadline - 2, hard_deadline=deadline,
                stream_limit=METADATA_LIMIT, journal=journal)
        row.update(observed=observed, capture=detail)
        write_json(directory / "result.original.json", row)
        require(observed["result"] == "PASS" and detail["cleanupParentReaped"] is True
                and detail["groupAfterCleanup"]["state"] == "ABSENT",
                "SOURCE_CHANGED")
        return subprocess.CompletedProcess(argv, observed["exitCode"],
            read_regular(directory / "stdout.original", METADATA_LIMIT),
            read_regular(directory / "stderr.original", METADATA_LIMIT))

    def emit(self):
        rows = []
        for item in self.records:
            detail = item["capture"]
            image = item["processImage"]
            rows.append({"pid": None if detail is None else detail["pid"],
                         "popenReturned": detail is not None and detail["pid"] is not None,
                         "gitImageObserved": image is not None and image["path"] is not None
                             and image["path"].endswith("/git"),
                         "returnCode": None if detail is None else detail["returnCodeObserved"],
                         "parentReaped": None if detail is None or "finishedAt" not in detail
                             else detail["cleanupParentReaped"],
                         "groupAfterCleanup": None if detail is None or detail["groupAfterCleanup"] is None
                             else detail["groupAfterCleanup"]["state"],
                         "result": None if item["observed"] is None else item["observed"]["result"]})
        value = {"schema": "cherri-ios-owned-git-diagnostic/1", "commands": rows,
                 "formalAcceptanceGranted": False, "firstInterruption": _first_interruption}
        raw = canonical(value)
        require(len(raw) <= 16384, "OUTPUT_LIMIT")
        sys.stderr.write("IOS_XCRESULT_OWNED_GIT_DIAGNOSTIC:" + raw.decode())


def observe_prepare(stage, file_index=None):
    if _prepare_observation is not None:
        _prepare_observation.mark(stage, file_index)


def require(condition, code):
    if not condition:
        raise Rejected(code)


def integer(value, low=0, high=2**63 - 1):
    return type(value) is int and low <= value <= high


def sha(data):
    return hashlib.sha256(data).hexdigest()


def canonical(value):
    return (json.dumps(value, sort_keys=True, separators=(",", ":"),
                       allow_nan=False) + "\n").encode("utf-8")


def unique_object(pairs):
    value = {}
    for key, item in pairs:
        require(key not in value, "JSON_REJECTED")
        value[key] = item
    return value


def json_value(raw):
    def nonfinite(_):
        raise Rejected("JSON_REJECTED")

    def finite(raw_number):
        value = float(raw_number)
        require(math.isfinite(value), "JSON_REJECTED")
        return value
    try:
        return json.loads(raw, object_pairs_hook=unique_object,
                          parse_constant=nonfinite, parse_float=finite)
    except (ValueError, UnicodeError, RecursionError):
        raise Rejected("JSON_REJECTED") from None


def exact_keys(value, keys, code="SCHEMA_REJECTED"):
    require(type(value) is dict and set(value) == set(keys), code)


def file_identity(info):
    return (info.st_dev, info.st_ino, info.st_mode, info.st_size,
            info.st_mtime_ns, info.st_ctime_ns)


def ancestors(path):
    result = []
    for parent in path.parents:
        info = parent.lstat()
        require(stat.S_ISDIR(info.st_mode) and
                not (hasattr(parent, "is_junction") and parent.is_junction()),
                "PATH_REJECTED")
        result.append((str(parent), info.st_dev, info.st_ino, info.st_mode))
    return result


def read_regular(path, limit):
    path = Path(path).absolute()
    before = path.lstat()
    require(stat.S_ISREG(before.st_mode) and 0 <= before.st_size <= limit,
            "FILE_REJECTED")
    parents = ancestors(path)
    with path.open("rb") as handle:
        opened = os.fstat(handle.fileno())
        require(file_identity(opened) == file_identity(before), "FILE_REJECTED")
        raw = handle.read(limit + 1)
        after = os.fstat(handle.fileno())
    require(len(raw) == before.st_size and len(raw) <= limit and
            file_identity(before) == file_identity(after) ==
            file_identity(path.lstat()) and ancestors(path) == parents,
            "FILE_REJECTED")
    return raw


def write_new(path, data, limit=METADATA_LIMIT):
    require(type(data) is bytes and len(data) <= limit, "OUTPUT_LIMIT")
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, "wb") as handle:
        handle.write(data)


def write_json(path, value):
    raw = canonical(value)
    write_new(path, raw)
    return raw


def digest_record(raw, *, complete=None):
    value = {"bytes": len(raw), "sha256": sha(raw)}
    if complete is not None:
        value["complete"] = complete
    return value


def validate_digest(value, limit, *, complete=False):
    exact_keys(value, {"bytes", "sha256", "complete"} if complete else {"bytes", "sha256"})
    require(integer(value["bytes"], 0, limit) and
            type(value["sha256"]) is str and HEX64.fullmatch(value["sha256"]),
            "DIGEST_REJECTED")
    if complete:
        require(type(value["complete"]) is bool, "DIGEST_REJECTED")


def validate_context(value):
    exact_keys(value, {"repository", "workflowPath", "workflowName", "job",
                       "event", "sourceSha", "runId", "attempt"})
    require(value["repository"] == REPOSITORY and value["workflowPath"] == WORKFLOW
            and value["workflowName"] == WORKFLOW_NAME and value["job"] == "ios-build",
            "CONTEXT_REJECTED")
    require(type(value["event"]) is str and value["event"] in {"push", "workflow_dispatch"} and
            type(value["sourceSha"]) is str and HEX40.fullmatch(value["sourceSha"])
            and integer(value["runId"], 1) and integer(value["attempt"], 1, 10000),
            "CONTEXT_REJECTED")
    return value


def ci_context():
    require(sys.platform == "darwin" and os.environ.get("RUNNER_OS") == "macOS",
            "CONTEXT_REJECTED")
    run, attempt = os.environ.get("GITHUB_RUN_ID", ""), os.environ.get("GITHUB_RUN_ATTEMPT", "")
    require(re.fullmatch(r"[1-9][0-9]{0,18}", run) and
            re.fullmatch(r"[1-9][0-9]{0,3}", attempt), "CONTEXT_REJECTED")
    source = os.environ.get("GITHUB_SHA", "")
    value = validate_context({
        "repository": os.environ.get("GITHUB_REPOSITORY"),
        "workflowPath": WORKFLOW, "workflowName": os.environ.get("GITHUB_WORKFLOW"),
        "job": os.environ.get("GITHUB_JOB"), "event": os.environ.get("GITHUB_EVENT_NAME"),
        "sourceSha": source, "runId": int(run), "attempt": int(attempt),
    })
    require(os.environ.get("GITHUB_WORKFLOW_SHA") == source and
            os.environ.get("GITHUB_REF", "").startswith("refs/heads/") and
            os.environ.get("GITHUB_WORKFLOW_REF") ==
                REPOSITORY + "/" + WORKFLOW + "@" + os.environ.get("GITHUB_REF", ""),
            "CONTEXT_REJECTED")
    return value


def root_name(context):
    return "cherri-ios-xcresult-{}-{}-{}".format(
        context["runId"], context["attempt"], context["sourceSha"])


def owned_root(context):
    # RUNNER_TEMP is an explicitly trusted runner root; payload paths are never resolved.
    temporary = Path(os.environ["RUNNER_TEMP"]).resolve(strict=True)
    require(temporary.is_dir(), "PATH_REJECTED")
    ancestors(temporary / "child")
    return temporary / root_name(context)


def directory_identity(path):
    info = Path(path).lstat()
    require(stat.S_ISDIR(info.st_mode) and info.st_uid == os.getuid(),
            "RESULT_INVALID")
    ancestors(Path(path).absolute())
    return {"device": info.st_dev, "inode": info.st_ino, "owner": info.st_uid,
            "mode": stat.S_IMODE(info.st_mode)}


def local_git(repo, args, deadline):
    remaining = deadline - time.monotonic()
    require(remaining > 0, "DIAGNOSTIC_DEADLINE")
    env = {k: os.environ[k] for k in ("PATH", "HOME", "LANG", "LC_ALL", "TMPDIR", "DEVELOPER_DIR") if k in os.environ}
    env.update({"GIT_NO_REPLACE_OBJECTS": "1", "GIT_OPTIONAL_LOCKS": "0",
                "GIT_TERMINAL_PROMPT": "0"})
    if _prepare_observation is not None:
        _prepare_observation.git_started(args, remaining)
    if _owned_git is None:
        process = subprocess.run(["/usr/bin/git", "--no-replace-objects", "-C", str(repo), *args],
                                 stdin=subprocess.DEVNULL, stdout=subprocess.PIPE,
                                 stderr=subprocess.PIPE, timeout=remaining, env=env)
    else:
        require(Path(repo) == _owned_git.repo, "SOURCE_CHANGED")
        process = _owned_git.run(args, env, deadline)
    if _prepare_observation is not None:
        _prepare_observation.git_returned(process)
    require(process.returncode == 0 and len(process.stdout) <= METADATA_LIMIT and
            len(process.stderr) <= METADATA_LIMIT and time.monotonic() < deadline,
            "SOURCE_CHANGED")
    return process.stdout


def validate_sources(value, source=None):
    exact_keys(value, {"sourceSha", "tree", "files", "absent"})
    require(type(value["sourceSha"]) is str and HEX40.fullmatch(value["sourceSha"])
            and (source is None or value["sourceSha"] == source) and
            type(value["tree"]) is str and HEX40.fullmatch(value["tree"]),
            "SOURCE_CHANGED")
    exact_keys(value["files"], SOURCE_FILES)
    for record in value["files"].values():
        exact_keys(record, {"bytes", "sha256", "blob", "mode"})
        require(integer(record["bytes"], 1, SOURCE_LIMIT) and
                type(record["sha256"]) is str and HEX64.fullmatch(record["sha256"]) and
                type(record["blob"]) is str and HEX40.fullmatch(record["blob"]) and
                record["mode"] in {"100644", "100755"}, "SOURCE_CHANGED")
    require(value["files"][RECIPIENT]["sha256"] == RECIPIENT_SHA256 and
            value["absent"] == [ABSENT_SOURCE], "SOURCE_CHANGED")
    return value


def source_binding(repo, source, deadline):
    observe_prepare("source_path")
    repo = Path(repo).absolute()
    require(repo == repo.resolve(strict=True), "SOURCE_CHANGED")
    require(local_git(repo, ["rev-parse", "--verify", "HEAD"], deadline).strip().decode() == source,
            "SOURCE_CHANGED")
    require(Path(local_git(repo, ["rev-parse", "--show-toplevel"], deadline).strip().decode()) == repo,
            "SOURCE_CHANGED")
    tree = local_git(repo, ["rev-parse", "HEAD^{tree}"], deadline).strip().decode()
    rows = local_git(repo, ["ls-tree", "-r", "-z", source, "--",
                           *SOURCE_FILES, ABSENT_SOURCE], deadline)
    entries = {}
    for raw in rows.split(b"\0"):
        if not raw:
            continue
        metadata, name = raw.split(b"\t", 1)
        mode, kind, blob = metadata.decode().split()
        name = name.decode()
        require(kind == "blob" and mode in {"100644", "100755"} and name not in entries,
                "SOURCE_CHANGED")
        entries[name] = (mode, blob)
    require(set(entries) == set(SOURCE_FILES) and
            not (repo / ABSENT_SOURCE).exists() and not (repo / ABSENT_SOURCE).is_symlink(),
            "SOURCE_CHANGED")
    files = {}
    for name in SOURCE_FILES:
        observe_prepare("source_file", SOURCE_FILES.index(name))
        raw = read_regular(repo / name, SOURCE_LIMIT)
        mode, blob = entries[name]
        info = (repo / name).lstat()
        working_mode = "100755" if info.st_mode & 0o111 else "100644"
        actual_blob = hashlib.sha1(b"blob " + str(len(raw)).encode() + b"\0" + raw).hexdigest()
        require(mode == working_mode and blob == actual_blob, "SOURCE_CHANGED")
        files[name] = {**digest_record(raw), "blob": blob, "mode": mode}
    require(not local_git(repo, ["status", "--porcelain", "--untracked-files=no",
                                 "--", *SOURCE_FILES, ABSENT_SOURCE], deadline).strip(),
            "SOURCE_CHANGED")
    observe_prepare("source_validation")
    return validate_sources({"sourceSha": source, "tree": tree, "files": files,
                             "absent": [ABSENT_SOURCE]}, source)


def load_primitives(repo, sources):
    # Execute only already authenticated bytes; do not race a second filesystem import.
    originals, modules, verified = {}, {}, {}
    for name, relative in zip(HELPERS, HELPER_FILES):
        require(sys.modules.get(name) is None, "SOURCE_CHANGED")
        raw = read_regular(Path(repo) / relative, SOURCE_LIMIT)
        expected = BOOTSTRAP_PINS if sources is None else sources["files"]
        require(digest_record(raw) == {k: expected[relative][k]
                                      for k in ("bytes", "sha256")}, "SOURCE_CHANGED")
        verified[name] = raw
    try:
        for name, relative in zip(HELPERS, HELPER_FILES):
            existing = sys.modules.get(name)
            require(existing is None, "SOURCE_CHANGED")
            raw = verified[name]
            module = types.ModuleType(name)
            module.__file__ = str(Path(repo) / relative)
            module.__package__ = ""
            originals[name] = existing
            sys.modules[name] = module
            exec(compile(raw, module.__file__, "exec"), module.__dict__)
            modules[name] = module
        helper = modules[HELPERS[-1]]
        require(helper.RECIPIENT_SHA256 == RECIPIENT_SHA256 and
                helper.ENCRYPT_SECONDS == 20, "SOURCE_CHANGED")
        return helper
    finally:
        for name, original in originals.items():
            if original is None:
                sys.modules.pop(name, None)
            else:
                sys.modules[name] = original


def native_exit(raw):
    require(type(raw) is bytes and re.fullmatch(rb"(0|[1-9][0-9]{0,2})\n", raw)
            and int(raw) <= 255, "NATIVE_EXIT_INVALID")
    return int(raw)


def publish_step_output(value):
    require(HEX64.fullmatch(value), "RESERVATION_INVALID")
    path = Path(os.environ["GITHUB_OUTPUT"]).absolute()
    before = path.lstat()
    require(stat.S_ISREG(before.st_mode), "RESERVATION_INVALID")
    ancestors(path)
    flags = os.O_WRONLY | os.O_APPEND | getattr(os, "O_NOFOLLOW", 0)
    fd = os.open(path, flags)
    with os.fdopen(fd, "ab") as handle:
        require(file_identity(os.fstat(handle.fileno())) == file_identity(before),
                "RESERVATION_INVALID")
        handle.write(("xcresult_reservation_sha256=" + value + "\n").encode())


def prepare(repo, context, root, *, deadline):
    observe_prepare("reservation_preconditions")
    repo, root = Path(repo).absolute(), Path(root).absolute()
    validate_context(context)
    require(root.name == root_name(context) and not root.exists() and not root.is_symlink(),
            "RESERVATION_INVALID")
    ancestors(root)
    exit_path = repo / "ios/xcodetest.exitcode"
    require(not exit_path.exists() and not exit_path.is_symlink(), "NATIVE_EXIT_INVALID")
    sources = source_binding(repo, context["sourceSha"], deadline)
    observe_prepare("root_creation")
    root.mkdir(mode=0o700)
    created = directory_identity(root)
    observe_prepare("root_permissions")
    # A setgid parent can add that bit to mkdir's requested 0700 mode.
    descriptor = os.open(root, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    try:
        opened = os.fstat(descriptor)
        require((opened.st_dev, opened.st_ino, opened.st_uid) ==
                (created["device"], created["inode"], created["owner"]),
                "RESERVATION_INVALID")
        os.fchmod(descriptor, 0o700)
    finally:
        os.close(descriptor)
    require(directory_identity(root) == {**created, "mode": 0o700},
            "RESERVATION_INVALID")
    observe_prepare("private_creation")
    private = root / "private"
    private.mkdir(mode=0o700)
    bundle = root / "tests.xcresult"
    require(not bundle.exists() and not bundle.is_symlink(), "RESERVATION_INVALID")
    marker = {"schema": SCHEMA, "context": context, "sources": sources,
              "root": str(root), "rootIdentity": directory_identity(root),
              "resultPath": str(bundle), "resultAbsent": True,
              "exitPath": str(exit_path), "exitAbsent": True}
    observe_prepare("reservation_write")
    raw = write_json(private / "reservation.json", marker)
    require(time.monotonic() < deadline, "DIAGNOSTIC_DEADLINE")
    return sha(raw)


def reservation(repo, context, root, expected):
    require(type(expected) is str and HEX64.fullmatch(expected), "RESERVATION_INVALID")
    raw = read_regular(root / "private/reservation.json", METADATA_LIMIT)
    require(sha(raw) == expected, "RESERVATION_INVALID")
    value = json_value(raw)
    exact_keys(value, {"schema", "context", "sources", "root", "rootIdentity",
                       "resultPath", "resultAbsent", "exitPath", "exitAbsent"},
               "RESERVATION_INVALID")
    require(value["schema"] == SCHEMA and value["context"] == context and
            value["root"] == str(root) and value["rootIdentity"] == directory_identity(root)
            and value["rootIdentity"]["mode"] == 0o700 and
            value["resultPath"] == str(root / "tests.xcresult") and
            value["exitPath"] == str(repo / "ios/xcodetest.exitcode") and
            value["resultAbsent"] is True and value["exitAbsent"] is True,
            "RESERVATION_INVALID")
    validate_sources(value["sources"], context["sourceSha"])
    return value


def summary_command(bundle):
    return ["xcrun", "xcresulttool", "get", "test-results", "summary", "--path", str(bundle)]


def make_archive(payload, complete):
    require(set(payload) == set(PAYLOAD), "ARCHIVE_REJECTED")
    rows = []
    for name in PAYLOAD:
        limit = STREAM_LIMIT if name.endswith((".stdout", ".stderr")) else METADATA_LIMIT
        require(type(payload[name]) is bytes and len(payload[name]) <= limit, "ARCHIVE_REJECTED")
        rows.append({"name": name, **digest_record(payload[name], complete=complete[name])})
    manifest = canonical({"schema": SCHEMA, "members": rows})
    require(len(manifest) <= METADATA_LIMIT, "ARCHIVE_REJECTED")
    all_payload = {**payload, "manifest.json": manifest}
    stream = io.BytesIO()
    with zipfile.ZipFile(stream, "w", compression=zipfile.ZIP_STORED) as archive:
        for name in MEMBERS:
            info = zipfile.ZipInfo(name, (1980, 1, 1, 0, 0, 0))
            info.create_system = 3
            info.external_attr = (stat.S_IFREG | 0o600) << 16
            archive.writestr(info, all_payload[name])
    encoded = stream.getvalue()
    require(len(encoded) <= ARCHIVE_LIMIT, "ARCHIVE_REJECTED")
    return encoded, [*rows, {"name": "manifest.json", **digest_record(manifest, complete=True)}]


def public_receipt(context, sources, reservation_sha, *, elapsed, native=None,
                   rejection=None, result=None, parse="NOT_CHECKED", streams=None,
                   archive=None, cipher=None, members=None):
    disposition = "REJECTED" if rejection else (
        "SEALED_COMPLETE" if result == "PASS" and parse == "JSON_OBJECT" else "SEALED_INCOMPLETE")
    value = {"schema": SCHEMA, "disposition": disposition, "context": context,
             "sources": sources, "reservationSha256": reservation_sha,
             "nativeExitCode": native, "captureResult": result, "jsonStatus": parse,
             "rejectionCode": rejection, "streams": streams, "members": members or [],
             "plaintextArchive": archive, "ciphertext": cipher,
             "recipientSha256": RECIPIENT_SHA256, "budgets": BUDGETS,
             "elapsedMs": elapsed, "formalAcceptanceGranted": False}
    validate_receipt(value)
    require(len(canonical(value)) <= METADATA_LIMIT, "OUTPUT_LIMIT")
    return value


def validate_receipt(value):
    exact_keys(value, {"schema", "disposition", "context", "sources", "reservationSha256",
                       "nativeExitCode", "captureResult", "jsonStatus", "rejectionCode",
                       "streams", "members", "plaintextArchive", "ciphertext",
                       "recipientSha256", "budgets", "elapsedMs", "formalAcceptanceGranted"})
    require(value["schema"] == SCHEMA and value["formalAcceptanceGranted"] is False and
            value["recipientSha256"] == RECIPIENT_SHA256 and value["budgets"] == BUDGETS
            and type(value["budgets"]) is dict and
            all(type(v) is int for v in value["budgets"].values()) and
            integer(value["elapsedMs"], 0, 60000), "RECEIPT_REJECTED")
    validate_context(value["context"])
    validate_sources(value["sources"], value["context"]["sourceSha"])
    require(type(value["reservationSha256"]) is str and HEX64.fullmatch(value["reservationSha256"])
            and (value["nativeExitCode"] is None or integer(value["nativeExitCode"], 0, 255)),
            "RECEIPT_REJECTED")
    if value["disposition"] == "REJECTED":
        require(value["rejectionCode"] in REJECTIONS and value["plaintextArchive"] is None
                and value["ciphertext"] is None and value["members"] == []
                and value["streams"] is None and value["captureResult"] is None
                and value["jsonStatus"] == "NOT_CHECKED", "RECEIPT_REJECTED")
        return value
    require(value["disposition"] in {"SEALED_COMPLETE", "SEALED_INCOMPLETE"} and
            value["rejectionCode"] is None and value["captureResult"] in RESULTS - {"OUTPUT_OVERFLOW"}
            and value["jsonStatus"] in {"JSON_OBJECT", "INVALID_JSON", "NOT_CHECKED"}
            and integer(value["nativeExitCode"], 0, 255), "RECEIPT_REJECTED")
    validate_digest(value["plaintextArchive"], ARCHIVE_LIMIT)
    validate_digest(value["ciphertext"], CIPHER_LIMIT)
    require(value["plaintextArchive"]["bytes"] > 0 and value["ciphertext"]["bytes"] > 0,
            "RECEIPT_REJECTED")
    exact_keys(value["streams"], {"stdout", "stderr"})
    for stream in value["streams"].values():
        validate_digest(stream, STREAM_LIMIT, complete=True)
    require(type(value["members"]) is list and
            [x.get("name") for x in value["members"] if type(x) is dict] == list(MEMBERS),
            "RECEIPT_REJECTED")
    for row in value["members"]:
        exact_keys(row, {"name", "bytes", "sha256", "complete"})
        limit = STREAM_LIMIT if row["name"].endswith((".stdout", ".stderr")) else METADATA_LIMIT
        validate_digest({k: row[k] for k in ("bytes", "sha256", "complete")}, limit, complete=True)
        if not row["name"].endswith((".stdout", ".stderr")):
            require(row["complete"] is True, "RECEIPT_REJECTED")
    for name in ("stdout", "stderr"):
        row = next(x for x in value["members"] if x["name"] == "xcresult-summary." + name)
        require({k: row[k] for k in ("bytes", "sha256", "complete")} == value["streams"][name],
                "RECEIPT_REJECTED")
    full = (value["captureResult"] == "PASS" and value["jsonStatus"] == "JSON_OBJECT"
            and all(x["complete"] for x in value["streams"].values()))
    require((value["disposition"] == "SEALED_COMPLETE") == full, "RECEIPT_REJECTED")
    require(value["jsonStatus"] != "JSON_OBJECT" or full, "RECEIPT_REJECTED")
    return value


def publish_rejection(root, marker, reservation_sha, start, code, native=None):
    # Only a previously authenticated reservation may authorize this fixed receipt.
    require(code in REJECTIONS, "DIAGNOSTIC_REJECTED")
    elapsed = round((time.monotonic() - start) * 1000)
    require(elapsed <= 60000, "DIAGNOSTIC_DEADLINE")
    receipt = public_receipt(marker["context"], marker["sources"], reservation_sha,
                             elapsed=elapsed, native=native, rejection=code)
    stage = root / "private/rejected-public"
    stage.mkdir(mode=0o700)
    write_json(stage / "receipt.json", receipt)
    require(not (root / "public").exists() and not (root / "public").is_symlink(),
            "RESERVATION_INVALID")
    stage.rename(root / "public")
    return receipt


def collect(repo, context, root, expected_reservation, *, started=None):
    start = time.monotonic() if started is None else started
    deadline = start + 60
    repo, root = Path(repo).absolute(), Path(root).absolute()
    validate_context(context)
    require(root.name == root_name(context), "RESERVATION_INVALID")
    marker = reservation(repo, context, root, expected_reservation)
    write_new(root / "private/collect.started", b"COLLECT_STARTED\n")
    private = root / "private"
    native = None
    try:
        sources = source_binding(repo, context["sourceSha"], min(deadline, start + 15))
        require(sources == marker["sources"], "SOURCE_CHANGED")
        helper = load_primitives(repo, sources)
        bundle = root / "tests.xcresult"
        bundle_before = directory_identity(bundle)
        try:
            exit_raw = read_regular(repo / "ios/xcodetest.exitcode", METADATA_LIMIT)
            native = native_exit(exit_raw)
        except Exception:
            raise Rejected("NATIVE_EXIT_INVALID") from None
        require(time.monotonic() < min(deadline, start + 15), "DIAGNOSTIC_DEADLINE")
        summary_start = time.monotonic()
        summary_deadline = min(deadline, summary_start + 20)
        with (private / "xcresult-summary.stdout").open("xb") as stdout, \
                (private / "xcresult-summary.stderr").open("xb") as stderr:
            observed, details = helper.capture(
                summary_command(bundle), max(0, summary_deadline - summary_start),
                output=stdout, error_output=stderr, hard_deadline=summary_deadline,
                stream_limit=STREAM_LIMIT)
        require(directory_identity(root) == marker["rootIdentity"] and
                directory_identity(bundle) == bundle_before, "RESULT_INVALID")
        if any(details[x]["overflow"] or details[x]["bytes"] > STREAM_LIMIT for x in ("stdout", "stderr")):
            raise Rejected("CAPTURE_OVERFLOW")
        streams, raw_streams = {}, {}
        for name in ("stdout", "stderr"):
            raw = read_regular(private / ("xcresult-summary." + name), STREAM_LIMIT)
            require(len(raw) == details[name]["bytes"] and sha(raw) == details[name]["sha256"],
                    "DIAGNOSTIC_REJECTED")
            raw_streams[name] = raw
            streams[name] = digest_record(raw, complete=bool(details[name]["eof"] and
                                                           not details[name]["overflow"]))
        parse = "NOT_CHECKED"
        if observed["result"] == "PASS" and all(x["complete"] for x in streams.values()):
            try:
                require(type(json_value(raw_streams["stdout"])) is dict, "JSON_REJECTED")
                parse = "JSON_OBJECT"
            except Rejected:
                parse = "INVALID_JSON"
        after = source_binding(repo, context["sourceSha"], deadline)
        require(after == sources, "SOURCE_CHANGED")
        command = {"schema": SCHEMA, "argv": summary_command(bundle),
                   "allowanceMs": 20000, **observed, "capture": details}
        identity = {"schema": SCHEMA, "reservation": marker,
                    "reservationSha256": expected_reservation, "sourcesAfter": after}
        payload = {"identity.json": canonical(identity), "command.json": canonical(command),
                   "xcresult-summary.stdout": raw_streams["stdout"],
                   "xcresult-summary.stderr": raw_streams["stderr"],
                   "xcodebuild.exitcode": exit_raw}
        completion = {name: True for name in PAYLOAD}
        completion.update({"xcresult-summary." + n: v["complete"] for n, v in streams.items()})
        encoded, members = make_archive(payload, completion)
        archive = private / "ios-xcresult-summary.zip"
        write_new(archive, encoded, ARCHIVE_LIMIT)
        stage = private / "encryption-public"
        stage.mkdir(mode=0o700)
        try:
            cipher = helper.encrypt_archive(private, stage, archive, digest_record(encoded),
                                             repo / RECIPIENT, min(deadline, time.monotonic() + 20))
        except (Exception, helper.ObservationExpired):
            raise Rejected("ENCRYPTION_REJECTED") from None
        old_cipher = stage / "project-prerequisite.zip.p7m"
        raw_cipher = read_regular(old_cipher, CIPHER_LIMIT)
        require(cipher == digest_record(raw_cipher) and raw_cipher.startswith(b"\x30"),
                "ENCRYPTION_REJECTED")
        require(source_binding(repo, context["sourceSha"], deadline) == sources and
                directory_identity(root) == marker["rootIdentity"] and
                directory_identity(bundle) == bundle_before and
                read_regular(repo / "ios/xcodetest.exitcode", METADATA_LIMIT) == exit_raw,
                "SOURCE_CHANGED")
        receipt = public_receipt(
            context, sources, expected_reservation, elapsed=round((time.monotonic() - start) * 1000),
            native=native, result=observed["result"], parse=parse, streams=streams,
            archive=digest_record(encoded), cipher=cipher, members=members)
        old_cipher.rename(stage / "ios-xcresult-summary.zip.p7m")
        write_json(stage / "receipt.json", receipt)
        require({p.name for p in stage.iterdir()} == PUBLIC_FILES and
                not (root / "public").exists() and not (root / "public").is_symlink()
                and time.monotonic() < deadline, "DIAGNOSTIC_REJECTED")
        stage.rename(root / "public")
        return (0 if receipt["disposition"] == "SEALED_COMPLETE" else 70), receipt
    except Exception as error:
        code = str(error) if isinstance(error, Rejected) and str(error) in REJECTIONS else "DIAGNOSTIC_REJECTED"
        return 70, publish_rejection(root, marker, expected_reservation, start, code, native)


class FixedParser(argparse.ArgumentParser):
    def error(self, message):
        raise Rejected("ARGUMENTS_REJECTED")


@contextlib.contextmanager
def envelope(seconds):
    global _first_interruption
    _first_interruption = None
    def expired(signum, frame):
        global _first_interruption
        if _first_interruption is None:
            _first_interruption = signum
            signal.setitimer(signal.ITIMER_REAL, 0)
            raise EnvelopeExpired()
    previous = {kind: signal.signal(kind, expired)
                for kind in (signal.SIGALRM, signal.SIGTERM, signal.SIGINT)}
    signal.setitimer(signal.ITIMER_REAL, seconds)
    try:
        yield
    finally:
        signal.setitimer(signal.ITIMER_REAL, 0)
        for kind, handler in previous.items():
            signal.signal(kind, handler)


def main(argv=None):
    global _owned_git
    observe_prepare("arguments")
    parser = FixedParser(prog="collect_ios_xcresult_summary.py", description=__doc__)
    parser.add_argument("mode", choices=("prepare", "collect"))
    parser.add_argument("--repo", required=True)
    parser.add_argument("--reservation-sha")
    parser.add_argument("--owned-git-diagnostic", action="store_true")
    args = parser.parse_args(argv)
    require(sys.flags.isolated == 1, "ARGUMENTS_REJECTED")
    observe_prepare("context")
    context = ci_context()
    observe_prepare("repo_path")
    repo = Path(args.repo).resolve(strict=True)
    observe_prepare("workspace_path")
    require(repo == Path(os.environ["GITHUB_WORKSPACE"]).resolve(strict=True), "SOURCE_CHANGED")
    observe_prepare("owned_root")
    root = owned_root(context)
    observe_prepare("envelope")
    with envelope(15 if args.mode == "prepare" else 60):
        start = time.monotonic()
        if args.owned_git_diagnostic:
            require(os.environ.get("RP39_DIAGNOSTIC_ONLY") == "true" and
                    context["event"] == "workflow_dispatch", "CONTEXT_REJECTED")
            _owned_git = OwnedGitReads(repo, root.parent / (root.name + "-git-" + args.mode))
        if args.mode == "prepare":
            if _prepare_observation is not None:
                _prepare_observation.envelope_started = start
            require(args.reservation_sha is None, "ARGUMENTS_REJECTED")
            reservation_sha = prepare(repo, context, root, deadline=start + 15)
            observe_prepare("publish_step_output")
            publish_step_output(reservation_sha)
            print("IOS_XCRESULT_SUMMARY_RESERVED")
            return 0
        code, receipt = collect(repo, context, root, args.reservation_sha, started=start)
        print("IOS_XCRESULT_SUMMARY_" + receipt["disposition"])
        return code


if __name__ == "__main__":
    if sys.argv[1:2] == ["prepare"]:
        _prepare_observation = PrepareFailureObservation()
    try:
        raise SystemExit(main())
    except (Exception, EnvelopeExpired) as error:
        print("IOS_XCRESULT_SUMMARY_REJECTED", file=sys.stderr)
        if _prepare_observation is not None:
            try:
                _prepare_observation.emit_failure(error)
            except (Exception, EnvelopeExpired):
                print("IOS_XCRESULT_PREPARE_DIAGNOSTIC_UNAVAILABLE", file=sys.stderr)
        raise SystemExit(70)
    finally:
        if _owned_git is not None:
            _owned_git.emit()
