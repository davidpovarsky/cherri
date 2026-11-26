#!/usr/bin/env python3
"""One source-bound import for selected hosted QA fixtures, before runtime.

The existing test02 and its 900-second phase remain the UI owner. This module
observes the selected native library read-only, dispatches one original45s file
delivery, and requires the actual UI plus a new exact native record. It cannot
run a Shortcut, sign, create a simulator, replace a library item, or retry.
Raw process/AX/database originals stay under private builds, never public logs.
"""
import argparse
import contextlib
import datetime
import hashlib
import io
import json
import os
from pathlib import Path
import re
import signal
import sys
import time
import uuid
import zipfile

import qa_terminal_receipts as terminal
import shortcut_library_identity as library
from git_source_batch import verify_raw_sources
import project_prerequisite_observation as observation
from project_prerequisite_observation import capture

FIXTURES = frozenset(("qa-short-circuit-v2", "qa-nested-list-control-v2",
                     "qa-nested-list-candidate-v2", "qa-storage-delete-v2"))
TEST_ID = "CherriSimulatorQAUITests/test02_ShortcutsImportLibraryAndActionEditorCoverage"
SCHEMA = "cherri-hosted-unique-import/1"
PHASE_SECONDS = 900
DELIVERY_SECONDS = 45
MAX_JSON = 4 * 1024 * 1024
MAX_ORIGINAL_BYTES = 64 * 1024 * 1024
require, sha, canonical = library.require, library.sha, library.canonical


def safe(root, relative, exists=True):
    root = Path(root)
    require(root.is_absolute() and root.is_dir() and not root.is_symlink(), "HOSTED_ROOT")
    require(root.resolve(strict=True) == root, "HOSTED_ROOT_ALIAS")
    return terminal.contained(root, relative, exists=exists)


def read(root, relative, limit=MAX_JSON):
    return library.read(safe(root, relative), limit)


def write(root, relative, value):
    path = safe(root, relative, exists=False)
    require(path.parent.is_dir(), "HOSTED_ORIGINAL_PARENT")
    raw = value if isinstance(value, bytes) else canonical(value) + b"\n"
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(fd, "wb") as stream:
        stream.write(raw)
        stream.flush()
        os.fsync(stream.fileno())
    return {"file": relative, "bytes": len(raw), "sha256": sha(raw)}


def live_dispatch(root, phase, value):
    # Only this owner replaces its live projection. Immutable started/finished
    # process originals and the final projection are separately create-only.
    name = "dispatch-" + phase + ".json"
    temporary = ".dispatch-" + uuid.uuid4().hex + ".tmp"
    write(root, temporary, value)
    os.replace(safe(root, temporary), safe(root, name, exists=False))


def remaining(deadline):
    value = deadline - time.monotonic()
    require(value > 0, "HOSTED_PHASE_DEADLINE")
    return value


@contextlib.contextmanager
def interrupts(deadline):
    old = {s: signal.getsignal(s) for s in (signal.SIGALRM, signal.SIGTERM, signal.SIGINT)}
    timer = signal.getitimer(signal.ITIMER_REAL)
    started = time.monotonic()
    def cancel(signum, frame):
        raise TimeoutError("HOSTED_OWNER_INTERRUPTED_" + str(signum))
    try:
        for kind in old:
            signal.signal(kind, cancel)
        allowance = remaining(deadline)
        if timer[0] > 0:
            allowance = min(allowance, timer[0])
        signal.setitimer(signal.ITIMER_REAL, allowance)
        yield
    finally:
        signal.setitimer(signal.ITIMER_REAL, 0)
        for kind, handler in old.items():
            signal.signal(kind, handler)
        if timer[0] > 0:
            signal.setitimer(signal.ITIMER_REAL, max(.000001, timer[0] - (time.monotonic() - started)), timer[1])


def source(repo, root, commit, fixture, stage, deadline):
    repo = Path(repo)
    require(Path(__file__).resolve().parents[2] == repo, "HOSTED_SOURCE_OWNER")
    files = tuple(dict.fromkeys((*terminal.SOURCE_FILES,
        "tools/shortcuts-simulator-qa/fixtures/" + fixture + ".cherri")))
    originals = safe(root, "source-" + stage, exists=False)
    originals.mkdir(mode=0o700)
    return verify_raw_sources(repo, commit, files,
        deadline=min(deadline, time.monotonic() + 10), originals=originals)


def packet(repo, artifacts, commit, run_id, fixture):
    require(fixture in FIXTURES, "HOSTED_FIXTURE_NOT_ENROLLED")
    expected = terminal.context(artifacts, commit, run_id, fixture)
    manifest_raw = read(artifacts, "compiler-manifest.json")
    manifest = library.strict_json(manifest_raw)
    rows = manifest["fixtures"]
    require(len({row["fixtureId"] for row in rows}) == len(rows), "HOSTED_MANIFEST_DUPLICATE")
    row = next(row for row in rows if row["fixtureId"] == fixture)
    require(row.get("fileName") == fixture + ".cherri" and row.get("dialect") == "v2"
            and row.get("buildProfile") == "debug" and row.get("runtimeTraceExpectation") == "REQUIRED_NDJSON",
            "HOSTED_COMPILE_PROFILE")
    paths = {"compiledSource": "builds/" + fixture + ".cherri",
             "unsigned": "builds/" + fixture + ".shortcut",
             "signed": "builds/signed/" + fixture + ".shortcut",
             "signing": "runtime/signing-provenance.json"}
    raw = {key: read(artifacts, path) for key, path in paths.items()}
    require(raw["compiledSource"] == read(repo, "tools/shortcuts-simulator-qa/fixtures/" + fixture + ".cherri")
            and sha(raw["compiledSource"]) == expected["compiledSourceSha256"], "HOSTED_COMPILED_SOURCE")
    actions, _ = library.action_input(raw["unsigned"])
    require(sha(raw["unsigned"]) == expected["unsignedPlistSha256"]
            and type(row.get("nativeActionCount")) is int and row["nativeActionCount"] == len(actions),
            "HOSTED_UNSIGNED")
    signed, signing = raw["signed"], library.strict_json(raw["signing"])
    require(signed.startswith(b"AEA1") and len(signed) > 4, "HOSTED_SIGNED_BYTES")
    require(signing.get("compilerCommitSha") == commit and signing.get("signedArtifactValidAEA1") is True
            and signing.get("signedArtifactSha256") == sha(signed)
            and type(signing.get("signedByteSize")) is int and signing["signedByteSize"] == len(signed),
            "HOSTED_SIGNING_SUMMARY")
    target = signing.get("targetProvenance")
    require(type(target) is dict, "HOSTED_SIGNING_TARGET")
    binding = {"compilerCommitSha": commit, "sourceSha256": sha(raw["compiledSource"]),
        "unsignedPlistSha256": sha(raw["unsigned"]), "unsignedByteSize": len(raw["unsigned"]),
        "nativeActionCount": len(actions), "signedArtifactSha256": sha(signed),
        "signedByteSize": len(signed), "headerMagic": "AEA1"}
    for key, value in binding.items():
        require(type(target.get(key)) is type(value) and target[key] == value, "HOSTED_SIGNING_" + key)
    return {"context": expected, "manifestSha256": sha(manifest_raw), "actionCount": len(actions),
        "files": {key: {"path": paths[key], "bytes": len(data), "sha256": sha(data)} for key, data in raw.items()}}


def configuration(args):
    require(args.fixture in FIXTURES and args.name == args.fixture, "HOSTED_EXACT_NAME")
    require(re.fullmatch(r"[0-9a-f]{40}", args.source) is not None, "HOSTED_SOURCE")
    require(re.fullmatch(r"[A-Za-z0-9_-]{1,64}", args.run_id) is not None, "HOSTED_RUN")
    require(re.fullmatch(r"[1-9][0-9]{0,3}", args.attempt) is not None, "HOSTED_ATTEMPT")
    require(args.nonce.startswith(args.fixture + "-" + args.run_id + "-")
            and re.fullmatch(r"[A-Za-z0-9_-]{1,128}", args.nonce) is not None, "HOSTED_NONCE")
    require(str(uuid.UUID(args.udid)).upper() == args.udid, "HOSTED_DEVICE")
    root, repo, artifacts = Path(args.root), Path(args.repo), Path(args.artifacts)
    require(root == artifacts / "builds/hosted-import" and artifacts.name == args.run_id, "HOSTED_PRIVATE_PATH")
    phase = "import-" + args.nonce
    require(len(phase) <= 128, "HOSTED_PHASE_ID")
    return repo, artifacts, root, phase


def prepare(args):
    repo, artifacts, root, phase = configuration(args)
    require(args.build_exit_code == 0, "HOSTED_HARNESS_BUILD_FAILED")
    require(not root.exists(), "HOSTED_PREPARATION_CREATE_ONLY")
    safe(artifacts, "builds", exists=False)
    root.mkdir(mode=0o700)
    deadline = time.monotonic() + 30
    outcome = {"schema": SCHEMA, "stage": "prepare", "status": "FAILED",
               "nativeInvocationCount": 0, "formalAcceptanceGranted": False}
    try:
        with interrupts(deadline):
            source(repo, root, args.source, args.fixture, "prepare", deadline)
            observed = packet(repo, artifacts, args.source, args.run_id, args.fixture)
            context = library.context({"schema": library.CONTEXT_SCHEMA, "runId": args.run_id,
                "runNonce": args.nonce, "deviceUdid": args.udid, "phaseId": phase, "expectedName": args.name,
                "compilerCommitSha": args.source, "builtHarnessCommitSha": args.source,
                "controllerCommitSha": args.source, "unsignedSHA256": observed["files"]["unsigned"]["sha256"],
                "signedSHA256": observed["files"]["signed"]["sha256"]})
            data = Path(args.device_data)
            library.database_path(data, args.udid)
            build_log = terminal.file_binding(artifacts, "builds/xcodebuild-build.log")
            value = {"schema": SCHEMA, "fixtureId": args.fixture, "testId": TEST_ID,
                "repo": str(repo), "artifactRoot": str(artifacts), "privateRoot": str(root),
                "context": context, "workflowRunAttempt": args.attempt,
                "packet": observed, "deviceDataPath": str(data),
                "deviceDataPathBasis": "selected-udid-canonical-CoreSimulator-data-with-existing-database",
                "buildExitCode": args.build_exit_code,
                "buildLog": build_log, "preparedAt": library.now(), "deliverySeconds": DELIVERY_SECONDS,
                "phaseSeconds": PHASE_SECONDS, "runtimeExecuted": False, "formalAcceptanceGranted": False}
            write(root, "prepared.original.json", value)
            outcome.update(status="PREPARED", context=context)
    except BaseException as error:
        outcome.update(errorType=type(error).__name__, error=str(error))
        raise
    finally:
        write(root, "prepare-result.original.json", outcome)
    return value


def load(root):
    raw = read(root, "prepared.original.json")
    value = library.strict_json(raw)
    require(value.get("schema") == SCHEMA and value.get("testId") == TEST_ID
            and value.get("fixtureId") in FIXTURES, "HOSTED_PREPARED_CONTEXT")
    context = library.context(value["context"])
    require(context["expectedName"] == value["fixtureId"] and value["privateRoot"] == str(root)
            and Path(value["artifactRoot"]) / "builds/hosted-import" == root,
            "HOSTED_PREPARED_PATH")
    require(context["compilerCommitSha"] == context["builtHarnessCommitSha"] == context["controllerCommitSha"]
            and context["runId"] == Path(value["artifactRoot"]).name
            and context["runNonce"].startswith(value["fixtureId"] + "-" + context["runId"] + "-")
            and context["phaseId"] == "import-" + context["runNonce"], "HOSTED_PREPARED_IDENTITY")
    require(type(value.get("deliverySeconds")) is int and value["deliverySeconds"] == DELIVERY_SECONDS
            and type(value.get("phaseSeconds")) is int and value["phaseSeconds"] == PHASE_SECONDS
            and type(value.get("buildExitCode")) is int and value["buildExitCode"] == 0
            and value.get("formalAcceptanceGranted") is False,
            "HOSTED_PREPARED_SCOPE")
    return value, sha(raw)


def packet_again(value):
    actual = packet(Path(value["repo"]), Path(value["artifactRoot"]), value["context"]["compilerCommitSha"],
                    value["context"]["runId"], value["fixtureId"])
    require(actual == value["packet"], "HOSTED_PACKET_CHANGED")
    require(terminal.file_binding(value["artifactRoot"], "builds/xcodebuild-build.log") == value["buildLog"],
            "HOSTED_BUILD_LOG_CHANGED")


def dispatch_projection(argv, details, observed, published):
    okay = (observed.get("result") == "PASS" and type(observed.get("exitCode")) is int
        and observed["exitCode"] == 0
        and details.get("cleanupParentReaped") is True
        and type(details.get("returnCodeObserved")) is int and details["returnCodeObserved"] == 0
        and (details.get("groupBeforeCleanup") or {}).get("state") == "ABSENT"
        and (details.get("groupAfterCleanup") or {}).get("state") == "ABSENT")
    return {"schemaVersion": 1, "command": argv, "timeoutSeconds": DELIVERY_SECONDS,
        "startedUTC": details["startedAt"], "pid": details.get("pid"),
        "outcome": "EXITED" if okay else "FAILED", "timedOut": observed.get("result") == "TIMEOUT",
        "exitCode": observed.get("exitCode"), "phasePublishedUTC": published,
        "exitObservedUTC": details["finishedAt"], "completedUTC": library.now(),
        "elapsedSeconds": observed.get("elapsedMs", 0) / 1000,
        "processReturncode": details.get("returnCodeObserved"),
        "parentReaped": details.get("cleanupParentReaped") is True}


def dispatch(value, root, deadline):
    context = value["context"]
    phase, fixture, commit = context["phaseId"], value["fixtureId"], context["controllerCommitSha"]
    repo, artifacts = Path(value["repo"]), Path(value["artifactRoot"])
    write(root, "dispatch-claim.original.json", {"context": context, "claimedAt": library.now(), "attemptOrdinal": 1})
    source(repo, root, commit, fixture, "dispatch-before", deadline)
    packet_again(value)
    unsigned = safe(artifacts, value["packet"]["files"]["unsigned"]["path"])
    signed = safe(artifacts, value["packet"]["files"]["signed"]["path"])
    before = library.observe(value["deviceDataPath"], context, "before", unsigned, signed, root / "library-before")
    write(root, "absence.original.json", library.admit_absence(before, context))
    remaining(deadline)
    argv = ["xcrun", "simctl", "openurl", context["deviceUdid"], signed.as_uri()]
    published = None
    details = {"startedAt": library.now(), "finishedAt": library.now(), "pid": None}
    observed = {"result": "NOT_STARTED", "exitCode": None, "elapsedMs": 0}
    original_error = None
    def journal(stage, record):
        nonlocal published, details
        details = dict(record)
        write(root, "process-" + stage + ".original.json", {"argv": argv, "allowanceSeconds": DELIVERY_SECONDS,
              "stage": stage, "details": record})
        if stage == "started":
            published = library.now()
            projection = {"schemaVersion": 1, "command": argv,
                "timeoutSeconds": DELIVERY_SECONDS, "startedUTC": record["startedAt"], "pid": record["pid"],
                "outcome": "LAUNCHED", "timedOut": False, "exitCode": None, "phasePublishedUTC": published}
            write(root, "dispatch-started.original.json", projection)
            live_dispatch(root, phase, projection)
    try:
        with safe(root, "stdout.original", exists=False).open("xb") as output, \
             safe(root, "stderr.original", exists=False).open("xb") as errors:
            observed, details = capture(argv, DELIVERY_SECONDS, output=output, error_output=errors,
                hard_deadline=deadline, execution_deadline=min(deadline, time.monotonic() + DELIVERY_SECONDS),
                journal=journal)
    except BaseException as error:
        original_error = error
        observed = {"result": "OWNER_INTERRUPTED", "exitCode": None, "elapsedMs": 0}
    finally:
        raw = {"schema": SCHEMA, "context": context, "argv": argv, "process": observed, "details": details,
               "errorType": type(original_error).__name__ if original_error else None,
               "nativeInvocationCount": 0, "formalAcceptanceGranted": False}
        write(root, "process-result.original.json", raw)
        if original_error is None:
            try:
                source(repo, root, commit, fixture, "dispatch-after", deadline)
                packet_again(value)
            except BaseException as closing_error:
                write(root, "closing-error.original.json", {"errorType": type(closing_error).__name__, "error": str(closing_error)})
                original_error = closing_error
        else:
            # Capture's original finally owns the simctl group. Do not start a
            # new Git child after cancellation; retain the unverified close.
            write(root, "closing-error.original.json", {"status": "NOT_RUN",
                "reason": "OWNER_INTERRUPTED", "sourceMatchedAfter": False})
        projection = dispatch_projection(argv, details, observed, published)
        if original_error is not None:
            projection["outcome"] = "FAILED"
        write(root, "dispatch-result.original.json", projection)
        live_dispatch(root, phase, projection)
    if original_error is not None:
        raise original_error
    require(projection["outcome"] == "EXITED", "HOSTED_DELIVERY_FAILED")
    return projection


def serve(root):
    value, prepared_sha = load(root)
    deadline = time.monotonic() + PHASE_SECONDS
    result = {"schema": SCHEMA, "stage": "serve", "status": "FAILED", "preparedSha256": prepared_sha,
              "ownerPid": os.getpid(), "startedAt": library.now(), "nativeInvocationCount": 0,
              "formalAcceptanceGranted": False}
    write(root, "owner-started.original.json", result)
    try:
        with interrupts(deadline):
            while not safe(root, "request-import.txt", exists=False).exists():
                require(not safe(root, "stop-request.txt", exists=False).exists(), "HOSTED_STOPPED_BEFORE_IMPORT")
                time.sleep(min(.1, remaining(deadline)))
            require(read(root, "request-import.txt", 4096) == value["context"]["expectedName"].encode("utf-8"),
                    "HOSTED_REQUEST_IDENTITY")
            dispatch(value, root, deadline)
            result["status"] = "DELIVERY_COMPLETED"
    except BaseException as error:
        result.update(errorType=type(error).__name__, error=str(error))
    finally:
        result["finishedAt"] = library.now()
        write(root, "owner-result.original.json", result)
    return result["status"] == "DELIVERY_COMPLETED"


def validate_import(dispatch, receipt, argv, phase, expected_name):
    require(isinstance(dispatch, dict) and dispatch.get("command") == argv
            and dispatch.get("outcome") == "EXITED" and dispatch.get("timedOut") is False
            and type(dispatch.get("exitCode")) is int and dispatch["exitCode"] == 0
            and type(dispatch.get("processReturncode")) is int and dispatch["processReturncode"] == 0
            and dispatch.get("parentReaped") is True and dispatch.get("timeoutSeconds") == DELIVERY_SECONDS,
            "IMPORT_PROCESS")
    require(isinstance(receipt, dict) and receipt.get("id") == phase and receipt.get("operation") == "import"
            and receipt.get("outcome") == "IMPORTED" and receipt.get("name") == expected_name
            and receipt.get("identityVerified") is True and receipt.get("libraryIdentityVerified") is True,
            "IMPORT_IDENTITY")
    timing = receipt.get("importTiming")
    require(isinstance(timing, dict) and timing.get("contract") == "dispatch-exit-anchored-add-v1"
            and timing.get("dispatchState") == "EXITED"
            and timing.get("dispatchReceipt") == "dispatch-" + phase + ".json", "IMPORT_TIMING")
    require(timing.get("sheetContract") == "requested-identity-and-add-v1"
            and all(timing.get(name) is True for name in ("sheetButtonExists", "sheetButtonHittable",
                "sheetIdentityMatched", "sheetCanTap")), "IMPORT_SHEET_READINESS")
    require(timing.get("sheetSelectionContract") == "current-container-bound-add-v1"
            and timing.get("sheetSelectionReason") == "UNIQUE_CURRENT_NAMED_CONTAINER",
            "IMPORT_CURRENT_CONTAINER_CONTRACT")
    for name, low, high in (("sheetQueryCandidateCount", 1, 32), ("sheetCurrentContainerCount", 1, 8192),
                            ("sheetMatchedCandidateCount", 1, 1), ("sheetSelectedQueryIndex", 0, 31)):
        value = timing.get(name)
        require(type(value) is int and low <= value <= high, "IMPORT_CURRENT_CONTAINER_COUNTS")
    require(timing["sheetSelectedQueryIndex"] < timing["sheetQueryCandidateCount"],
            "IMPORT_CURRENT_CONTAINER_INDEX")
    paths = []
    for name in ("sheetSelectedContainerPath", "sheetSelectedTitlePath", "sheetSelectedButtonPath"):
        path = timing.get(name)
        require(isinstance(path, list) and 0 < len(path) <= 64
                and all(type(index) is int and 0 <= index < 8192 for index in path),
                "IMPORT_CURRENT_CONTAINER_PATH")
        paths.append(path)
    container, title, button = paths
    require(all(len(path) > len(container) and path[:len(container)] == container
                for path in (title, button))
            and title[len(container)] != button[len(container)], "IMPORT_CURRENT_CONTAINER_ANCESTRY")
    instants = []
    for name in ("addObservedUTC", "lastSheetCheckUTC", "sheetMatchedAfterCaptureUTC"):
        raw = timing.get(name)
        require(isinstance(raw, str) and raw.endswith("Z"), "IMPORT_SHEET_CLOCK")
        instant = datetime.datetime.fromisoformat(raw[:-1] + "+00:00")
        require(instant.tzinfo is not None, "IMPORT_SHEET_CLOCK")
        instants.append(instant)
    require(instants[0] <= instants[1] <= instants[2], "IMPORT_SHEET_CLOCK_ORDER")


def validate_unique_library_import(receipt, folder, phase, expected_name):
    """Join the actual fresh XCTest observations; do not substitute a title for a cell."""
    require(receipt.get("libraryUniqueCurrentCellVerified") is True
            and receipt.get("requiresAbsentLibraryRecord") is True
            and receipt.get("replaceTapped") is False, "IMPORT_FRESH_UNIQUE_LIBRARY")
    timing = receipt["importTiming"]
    require(timing.get("libraryContract") == "dispatch-exit-anchored-library-v1"
            and timing.get("libraryCanConfirm") is True
            and timing.get("librarySelectionContract") == "unique-current-library-cell-v1"
            and timing.get("librarySelectionReason") == "UNIQUE_CURRENT_NAMED_LIBRARY_CELL",
            "IMPORT_LIBRARY_SELECTION_CONTRACT")
    for key in ("libraryCurrentNamedCellCount", "libraryQueryCandidateCount"):
        require(type(timing.get(key)) is int and timing[key] == 1, "IMPORT_LIBRARY_UNIQUE_COUNTS")
    index = timing.get("librarySelectedQueryIndex")
    path = timing.get("librarySelectedCellPath")
    require(type(index) is int and index == 0 and type(path) is list and 0 < len(path) <= 64
            and all(type(i) is int and 0 <= i < 8192 for i in path), "IMPORT_LIBRARY_BINDING_PATH")
    names = sorted(folder.glob(phase + "-library-observation-*.json"))
    require(2 <= len(names) <= 512 and all(p.name == phase + "-library-observation-%04d.json" % (i + 1)
            for i, p in enumerate(names)), "IMPORT_LIBRARY_ORIGINAL_SEQUENCE")
    observations = []
    bindings = []
    for ordinal, file in ((len(names) - 1, names[-2]), (len(names), names[-1])):
        raw = library.read(file, 2 * 1024 * 1024)
        value = library.strict_json(raw)
        require(value.get("schema") == "cherri-current-library-selection-observation/1"
                and value.get("phase") == phase and type(value.get("ordinal")) is int
                and value["ordinal"] == ordinal and value.get("expectedName") == expected_name
                and value.get("foreground") is True and value.get("dispatchConfirmed") is True
                and value.get("decision") == "UNIQUE_CURRENT_NAMED_LIBRARY_CELL"
                and value.get("canConfirm") is True and type(value.get("currentNamedCellCount")) is int
                and value["currentNamedCellCount"] == 1 and type(value.get("queryCandidateCount")) is int
                and value["queryCandidateCount"] == 1, "IMPORT_LIBRARY_ORIGINAL_IDENTITY")
        candidates = value.get("candidates")
        require(type(candidates) is list and len(candidates) == 1 and candidates[0].get("exists") is True
                and candidates[0].get("hittable") is True and type(candidates[0].get("queryIndex")) is int
                and candidates[0]["queryIndex"] == index, "IMPORT_LIBRARY_ORIGINAL_CANDIDATE")
        node = value["current"]
        for part in path:
            require(type(node.get("children")) is list and part < len(node["children"]),
                    "IMPORT_LIBRARY_ORIGINAL_PATH")
            node = node["children"][part]
        require(node == candidates[0].get("snapshot") and node.get("kind") == "cell"
                and node.get("enabled") is True and node.get("label") == timing.get("librarySelectedCellLabel")
                and node.get("identifier") == timing.get("librarySelectedCellIdentifier"),
                "IMPORT_LIBRARY_ORIGINAL_CELL")
        observed, deadline = library.instant(value["observedUTC"]), library.instant(value["deadlineUTC"])
        require(observed <= deadline and value["deadlineUTC"] == timing["libraryDeadlineUTC"],
                "IMPORT_LIBRARY_ORIGINAL_DEADLINE")
        observations.append({"file": file.name, "sha256": sha(raw), "observedUTC": value["observedUTC"]})
        bindings.append(node)
    require(bindings[0] == bindings[1] and observations[-1]["observedUTC"] == timing["librarySelectionObservedUTC"]
            and library.instant(observations[0]["observedUTC"]) <= library.instant(observations[1]["observedUTC"])
            <= library.instant(timing["libraryMatchedAfterCaptureUTC"])
            <= library.instant(timing["libraryDeadlineUTC"]), "IMPORT_LIBRARY_FRESH_CAPTURE_JOIN")
    return {"schema": "cherri-current-library-xctest-original-join/1", "phase": phase,
            "fixtureId": expected_name, "observations": observations, "uniqueCurrentCell": True,
            "freshAfterCapture": True, "formalAcceptanceGranted": False}



def verify(root):
    value, prepared_sha = load(root)
    context = value["context"]
    phase, fixture, commit = context["phaseId"], value["fixtureId"], context["controllerCommitSha"]
    artifacts, repo = Path(value["artifactRoot"]), Path(value["repo"])
    result = {"schema": SCHEMA, "stage": "admission", "status": "FAILED", "context": context,
              "importIdentityAuthenticated": False, "runtimeExecuted": False, "formalAcceptanceGranted": False}
    try:
        deadline = time.monotonic() + 30
        with interrupts(deadline):
            source(repo, root, commit, fixture, "admission", deadline)
            packet_again(value)
            owner = library.strict_json(read(root, "owner-result.original.json"))
            require(owner.get("status") == "DELIVERY_COMPLETED" and owner.get("preparedSha256") == prepared_sha,
                    "HOSTED_DELIVERY_OWNER")
            original = library.strict_json(read(root, "process-result.original.json"))
            require(original.get("context") == context and original.get("errorType") is None,
                    "HOSTED_PROCESS_CONTEXT")
            signed = safe(artifacts, value["packet"]["files"]["signed"]["path"])
            unsigned = safe(artifacts, value["packet"]["files"]["unsigned"]["path"])
            argv = ["xcrun", "simctl", "openurl", context["deviceUdid"], signed.as_uri()]
            require(original.get("argv") == argv, "HOSTED_PROCESS_ARGV")
            started = library.strict_json(read(root, "dispatch-started.original.json"))
            completed = library.strict_json(read(root, "dispatch-result.original.json"))
            require(library.strict_json(read(root, "dispatch-" + phase + ".json")) == completed,
                    "HOSTED_LIVE_PROJECTION_CHANGED")
            projected = dispatch_projection(argv, original["details"], original["process"], started["phasePublishedUTC"])
            require(projected["outcome"] == "EXITED" and all(type(completed.get(key)) is type(item)
                    and completed[key] == item
                    for key, item in projected.items() if key != "completedUTC"), "HOSTED_PROCESS_PROJECTION")
            require(started.get("outcome") == "LAUNCHED" and started.get("timedOut") is False
                    and all(started.get(key) == completed.get(key) for key in
                            ("pid", "command", "timeoutSeconds", "startedUTC", "phasePublishedUTC")),
                    "HOSTED_PROCESS_START_JOIN")
            require(library.instant(completed["startedUTC"]) <= library.instant(completed["phasePublishedUTC"])
                    <= library.instant(completed["exitObservedUTC"]) <= library.instant(completed["completedUTC"]),
                    "HOSTED_PROCESS_CLOCK")
            for name in ("stdout", "stderr"):
                raw = read(root, name + ".original", 1024 * 1024 + 1)
                pin = original["details"][name]
                require(pin.get("eof") is True and pin.get("overflow") is False
                        and type(pin.get("bytes")) is int and pin["bytes"] == len(raw)
                        and pin.get("sha256") == sha(raw), "HOSTED_PROCESS_STREAM")
            ui = library.strict_json(read(root, "ui-receipt.original.json"))
            require(ui.get("testId") == TEST_ID and ui.get("preparedSha256") == prepared_sha
                    and ui.get("context") == context and ui.get("workflowRunAttempt") == value["workflowRunAttempt"],
                    "HOSTED_UI_CONTEXT")
            validate_import(completed, ui, argv, phase, fixture)
            ui_originals = validate_unique_library_import(ui, root, phase, fixture)
            before = library.strict_json(read(root, "library-before/result.original.json"))
            after = library.observe(value["deviceDataPath"], context, "after", unsigned, signed, root / "library-after")
            database = library.bind_new_record(before, after, context, completed)
            write(root, "native-record.original.json", database)
            write(root, "ui-original-join.original.json", ui_originals)
            remaining(deadline)
            result.update(status="IMPORT_IDENTITY_VERIFIED", importIdentityAuthenticated=True,
                          beforeExactNameAbsent=True, afterNewExactNameUnique=True,
                          storedActionsMatchUnsigned=True,
                          nativeRecordUUID=database["nativeRecord"]["workflowUUID"],
                          actionCount=database["nativeRecord"]["actionCount"])
    except BaseException as error:
        result.update(errorType=type(error).__name__, error=str(error))
    finally:
        result["completedUTC"] = library.now()
        write(root, "admission.original.json", result)
    return result["status"] == "IMPORT_IDENTITY_VERIFIED"


def seal(root, sealed_root):
    """Retain raw observations through the existing encrypted session upload.

    Only ciphertext and a fixed summary leave the private directory. This can
    retain a failed prepare/dispatch/admission and never grants runtime credit.
    """
    root, sealed_root = Path(root), Path(sealed_root)
    started = time.monotonic()
    deadline = started + 45
    require(root.is_dir() and root.resolve(strict=True) == root, "HOSTED_SEAL_ROOT")
    require(sealed_root.is_absolute() and sealed_root.name == "hosted-import"
            and sealed_root.parent.is_dir()
            and sealed_root.parent.resolve(strict=True) == sealed_root.parent
            and not sealed_root.exists() and not sealed_root.is_symlink(), "HOSTED_SEAL_DESTINATION")
    sealed_root.mkdir(mode=0o700)
    private, public = sealed_root / "private", sealed_root / "public"
    private.mkdir(mode=0o700)
    public.mkdir(mode=0o700)
    with interrupts(deadline):
        paths = sorted(root.rglob("*"))
        require(len(paths) <= 2048 and not any(p.is_symlink() for p in paths), "HOSTED_SEAL_MEMBERS")
        members, originals, total = [], {}, 0
        for path in paths:
            remaining(deadline)
            if path.is_dir():
                continue
            name = path.relative_to(root).as_posix()
            raw = read(root, name)
            total += len(raw)
            require(total <= MAX_ORIGINAL_BYTES, "HOSTED_SEAL_TOTAL")
            originals[name] = raw
            members.append({"path": name, "bytes": len(raw), "sha256": sha(raw)})
        require(members, "HOSTED_SEAL_EMPTY")
        manifest = canonical({"schema": SCHEMA + "/originals", "members": members})
        buffer = io.BytesIO()
        with zipfile.ZipFile(buffer, "w", compression=zipfile.ZIP_DEFLATED, compresslevel=6) as archive:
            for name, raw in [*originals.items(), ("manifest.original.json", manifest)]:
                info = zipfile.ZipInfo(name)
                info.external_attr = 0o600 << 16
                info.compress_type = zipfile.ZIP_DEFLATED
                archive.writestr(info, raw)
        encoded = buffer.getvalue()
        require(0 < len(encoded) <= observation.ARCHIVE_LIMIT, "HOSTED_SEAL_ARCHIVE_LIMIT")
        with zipfile.ZipFile(io.BytesIO(encoded)) as archive:
            require(archive.namelist() == [*originals, "manifest.original.json"], "HOSTED_SEAL_NAMES")
            for name, raw in originals.items():
                require(archive.read(name) == raw and read(root, name) == raw, "HOSTED_SEAL_CHANGED")
            require(archive.read("manifest.original.json") == manifest, "HOSTED_SEAL_MANIFEST")
        write(private, "session-originals.zip", encoded)
        archive_binding = {"bytes": len(encoded), "sha256": sha(encoded)}
        recipient = Path(__file__).parent / "xcode_environment_inventory_recipient.pem"
        cipher = observation.encrypt_archive(private, public, private / "session-originals.zip",
            archive_binding, recipient, deadline)
        (public / "project-prerequisite.zip.p7m").rename(public / "session-originals.zip.p7m")
        # No native names, raw paths, DB fields, callbacks or failure strings.
        summary = {"schema": SCHEMA + "/sealed", "status": "ORIGINALS_RETAINED",
            "memberCount": len(members), "plaintextArchive": archive_binding,
            "ciphertext": cipher, "recipientSha256": observation.RECIPIENT_SHA256,
            "runtimeExecuted": False, "formalAcceptanceGranted": False}
        write(public, "summary.json", summary)
        require({p.name for p in public.iterdir()} == {"session-originals.zip.p7m", "summary.json"},
                "HOSTED_PUBLIC_MEMBERS")
        if os.environ.get("GITHUB_OUTPUT"):
            with Path(os.environ["GITHUB_OUTPUT"]).open("a", encoding="ascii") as stream:
                stream.write("session_boundary_publication_ready=true\n")
    return summary


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("mode", choices=("prepare", "serve", "verify", "seal"))
    parser.add_argument("--root", required=True, type=Path)
    for name in ("repo", "artifacts", "source", "run-id", "attempt", "fixture", "name", "nonce", "udid", "device-data"):
        parser.add_argument("--" + name)
    parser.add_argument("--build-exit-code", type=int)
    parser.add_argument("--sealed-root", type=Path)
    args = parser.parse_args()
    os.umask(0o077)
    if args.mode == "prepare":
        prepare(args)
        return 0
    if args.mode == "serve":
        return 0 if serve(args.root) else 1
    if args.mode == "seal":
        seal(args.root, args.sealed_root)
        return 0
    return 0 if verify(args.root) else 1


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (OSError, ValueError, TimeoutError):
        raise SystemExit("Hosted unique import rejected; private originals retained.")
