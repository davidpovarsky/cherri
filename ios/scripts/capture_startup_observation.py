#!/usr/bin/env python3
"""Capture fixed startup lifecycle observations; never grant acceptance credit.

Only fixed run-nonce-owned cache files are read from the selected Simulator app.
Private XCTest descriptions and container-query output remain quarantined.
"""
import argparse
import base64
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import time

SESSION = "[CHERRI_UI_OBSERVATION_SESSION] "
ISSUE = "[CHERRI_UI_OBSERVATION_ISSUE_PRIVATE] "
TESTS = {
    "CherriEditorUITests/testBRG27EditorRealWorldWorkflowUI": "brg27",
    "CherriEditorUITests/testD03CurrentDocumentationSearchUI": "d03",
    "CherriEditorUITests/testE01CompletionDisplayAndInsertionUI": "e01",
    "CherriEditorUITests/testE07VisualParameterEditingCanvasUI": "e07",
}
PHASES = {"before-launch", "after-launch", "readiness-start", "teardown-start", "teardown-finished"}
APP_PHASES = {
    "observer-opened", "heartbeat", "observer-expired", "observer-stopped", "event-limit-reached",
    "main-queue-enqueued", "main-queue-executed",
    "app-initialized", "document-initialized", "document-read-started", "document-read-finished",
    "document-write-started", "document-write-finished", "workspace-body", "workspace-appeared",
    "editor-host-body", "editor-host-appeared", "editor-container-created",
    "editor-container-attached", "editor-container-detached", "editor-container-document-changed",
}
DOMAINS = {"XCTestErrorDomain", "NSCocoaErrorDomain", "NSPOSIXErrorDomain", "AXErrorDomain",
           "com.apple.dt.xctest.error", "com.apple.dt.xctest.ui-testing", "DTXMessage"}
ISSUE_TYPES = {"assertionFailure", "performanceRegression", "system", "uncaughtException",
               "thrownError", "unmatchedExpectedFailure"}
NONCE = re.compile(r"[0-9a-f]{32}")
CASE = re.compile(r"Test Case '-\[CherriUITests\.CherriEditorUITests (test[A-Za-z0-9_]+)\]' (started\.|(?:passed|failed) \([0-9.]+ seconds\)\.)")
SOURCES = (
    "ios/CherriApp/Diagnostics/StartupLifecycleObserver.swift",
    "ios/CherriApp/Diagnostics/StartupContainerWitness.swift",
    "ios/CherriApp/App/CherriApp.swift",
    "ios/CherriApp/App/WorkspaceView.swift",
    "ios/CherriApp/Documents/CherriDocument.swift",
    "ios/CherriApp/Editor/CherriEditorView.swift",
    "ios/CherriUITests/CherriEditorUITests.swift",
    "ios/CherriUITests/EditorStartupDeadline.swift",
    "ios/scripts/capture_startup_observation.py",
    "ios/scripts/collect_ios_test_evidence.py",
    ".github/workflows/ios-build.yml",
)
MAX_FILE = 65_536


def require(condition, code):
    if not condition:
        raise ValueError(code)


def integer(value, minimum=0, maximum=9_000_000_000_000):
    return type(value) is int and minimum <= value <= maximum


def digest(data):
    return hashlib.sha256(data).hexdigest()


def file_digest(path):
    value = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(65_536), b""):
            value.update(chunk)
    return value.hexdigest()


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        require(key not in result, "DUPLICATE_JSON_KEY")
        result[key] = value
    return result


def decode(line):
    return json.loads(line, object_pairs_hook=unique_object)


def session(line, active):
    require(len(line.encode("utf-8")) <= 1_024 and line.startswith(SESSION), "SESSION_SIZE")
    row = decode(line[len(SESSION):])
    require(type(row) is dict, "SESSION_OBJECT")
    keys = {"schemaVersion", "nonce", "testId", "phase", "uptimeMs"}
    if row.get("phase") == "readiness-start":
        keys.add("readinessStartedAtMs")
    require(set(row) == keys and row["schemaVersion"] == "1", "SESSION_SCHEMA")
    require(row["testId"] == active and active in TESTS, "SESSION_TEST")
    require(isinstance(row["nonce"], str) and NONCE.fullmatch(row["nonce"]), "SESSION_NONCE")
    require(type(row["phase"]) is str and row["phase"] in PHASES and integer(row["uptimeMs"]), "SESSION_PHASE_CLOCK")
    if "readinessStartedAtMs" in row:
        require(integer(row["readinessStartedAtMs"])
                and 0 <= row["uptimeMs"] - row["readinessStartedAtMs"] <= 1_000, "READINESS_CLOCK")
    return row


def bounded_description(value):
    require(type(value) is dict and set(value) == {"base64", "originalByteCount", "truncated"}, "DESCRIPTION_SCHEMA")
    require(type(value["base64"]) is str and len(value["base64"]) <= 5_464, "DESCRIPTION_SIZE")
    try:
        raw = base64.b64decode(value["base64"], validate=True)
    except (ValueError, TypeError) as error:
        raise ValueError("DESCRIPTION_BASE64") from error
    require(len(raw) <= 4_096 and integer(value["originalByteCount"], 0, 1_000_000_000)
            and type(value["truncated"]) is bool
            and len(raw) == min(value["originalByteCount"], 4_096)
            and value["truncated"] == (value["originalByteCount"] > 4_096), "DESCRIPTION_LENGTH")
    return raw, {"capturedBytes": len(raw), "originalBytes": value["originalByteCount"],
                 "truncated": value["truncated"], "capturedSha256": digest(raw)}


def issue(line, active, nonce):
    require(len(line.encode("utf-8")) <= 32_768 and line.startswith(ISSUE), "ISSUE_SIZE")
    row = decode(line[len(ISSUE):])
    require(set(row) == {"schemaVersion", "nonce", "testId", "uptimeMs", "issueIndex", "issueType",
                        "sourceFile", "sourceLine", "errorPresent", "errorDomain", "errorCode",
                        "compact", "detailed", "errorDescription"}
            and row["schemaVersion"] == "1", "ISSUE_SCHEMA")
    require(row["testId"] == active and active in TESTS and row["nonce"] == nonce, "ISSUE_IDENTITY")
    require(integer(row["uptimeMs"]) and integer(row["issueIndex"], 1, 8)
            and type(row["errorPresent"]) is bool and integer(row["sourceLine"], 0, 1_000_000)
            and integer(row["errorCode"], -(2**53), 2**53), "ISSUE_VALUES")
    require(type(row["sourceFile"]) is str and len(row["sourceFile"].encode()) <= 1_024
            and type(row["issueType"]) is str and len(row["issueType"]) <= 128, "ISSUE_METADATA")
    decoded = {key: bounded_description(row[key]) for key in ("compact", "detailed", "errorDescription", "errorDomain")}
    domain = decoded["errorDomain"][0].decode("utf-8", errors="replace")
    require(row["errorPresent"] or (not domain and row["errorCode"] == 0
                                  and not decoded["errorDescription"][0]), "ABSENT_ERROR_CONTENT")
    compact = decoded["compact"][0].decode("utf-8", errors="replace")
    # Fixed diagnostic phrases; never copy arbitrary descriptions, AX trees or paths.
    signatures = [label for label, phrase in (
        ("MATCHING_SNAPSHOT_FAILURE", "Failed to get matching snapshot"),
        ("ACCESSIBILITY_TIMEOUT", "Timed out while waiting for"),
        ("ACCESSIBILITY_ELEMENT_FAILURE", "Failed to get matching elements"),
        ("APPLICATION_NOT_RUNNING", "Application is not running"),
        ("APPLICATION_NOT_RESPONDING", "Application is not responding"),
        ("STARTUP_ASSERTION", "within the startup budget"),
    ) if phrase in compact]
    return {
        "schemaVersion": "1", "nonce": nonce, "testId": active, "uptimeMs": row["uptimeMs"],
        "issueIndex": row["issueIndex"], "issueType": row["issueType"] if row["issueType"] in ISSUE_TYPES else "UNREVIEWED",
        "sourceFile": "CherriEditorUITests.swift" if row["sourceFile"] == "CherriEditorUITests.swift" else "UNREVIEWED",
        "sourceLine": row["sourceLine"], "errorPresent": row["errorPresent"],
        "errorDomain": domain if domain in DOMAINS else ("UNREVIEWED" if domain else "ABSENT"),
        "errorCode": row["errorCode"], "diagnosticSignatures": signatures,
        "descriptions": {key: value[1] for key, value in decoded.items()},
    }


def test_observations(text):
    active = None
    tests = {test: {"sessions": [], "issues": [], "caseStarts": 0, "caseConclusions": [],
                    "rejectedMetadataCount": 0} for test in TESTS}
    seen_nonces = set()
    private_issues = []
    for raw in text.splitlines():
        line = raw.strip()
        if line.startswith("Test Case '"):
            active = None
            if match := CASE.fullmatch(line):
                test = "CherriEditorUITests/" + match.group(1)
                if test in tests:
                    if match.group(2) == "started.":
                        tests[test]["caseStarts"] += 1
                        active = test
                    else:
                        tests[test]["caseConclusions"].append(match.group(2).split(" ", 1)[0])
            continue
        if not active or not line.startswith((SESSION, ISSUE)):
            continue
        target = tests[active]
        try:
            require(target["caseStarts"] == 1, "REPEATED_TEST")
            if line.startswith(SESSION):
                row = session(line, active)
                previous = target["sessions"]
                require(len(previous) < 5, "SESSION_COUNT")
                if not previous:
                    require(row["phase"] == "before-launch" and row["nonce"] not in seen_nonces, "SESSION_START")
                    seen_nonces.add(row["nonce"])
                else:
                    require(row["nonce"] == previous[0]["nonce"]
                            and row["uptimeMs"] >= previous[-1]["uptimeMs"], "SESSION_IDENTITY_ORDER")
                    order = ["before-launch", "after-launch", "readiness-start", "teardown-start", "teardown-finished"]
                    require(order.index(row["phase"]) > order.index(previous[-1]["phase"]), "SESSION_PHASE_ORDER")
                previous.append(row)
            else:
                require(bool(target["sessions"]), "ISSUE_BEFORE_SESSION")
                row = issue(line, active, target["sessions"][0]["nonce"])
                require(row["issueIndex"] == len(target["issues"]) + 1, "ISSUE_SEQUENCE")
                target["issues"].append(row)
                private_issues.append(raw)
        except (ValueError, TypeError, KeyError):
            target["rejectedMetadataCount"] += 1
    for value in tests.values():
        value["uniqueTestInvocation"] = value["caseStarts"] == 1 and len(value["caseConclusions"]) <= 1
    return tests, private_issues


def validate_app_events(data, test, nonce):
    require(0 < len(data) <= MAX_FILE and data.endswith(b"\n"), "APP_FILE_BOUNDS")
    try:
        rows = [decode(line) for line in data.decode("utf-8").splitlines()]
    except (ValueError, UnicodeError) as error:
        raise ValueError("APP_JSON") from error
    require(1 <= len(rows) <= 96, "APP_EVENT_COUNT")
    keys = {"schemaVersion", "nonce", "testId", "sequence", "phase", "documentId",
            "observedUptimeMs", "writtenUptimeMs", "mainThread", "processId"}
    last_written = None
    process = None
    for index, row in enumerate(rows):
        require(type(row) is dict and set(row) == keys and row["schemaVersion"] == "1", "APP_SCHEMA")
        require(row["nonce"] == nonce and row["testId"] == test and test in TESTS, "APP_IDENTITY")
        require(integer(row["sequence"], 0, 95) and row["sequence"] == index
                and type(row["phase"]) is str and row["phase"] in APP_PHASES, "APP_SEQUENCE")
        require(row["documentId"] is None or (isinstance(row["documentId"], str)
                    and NONCE.fullmatch(row["documentId"])), "APP_DOCUMENT_ID")
        require(integer(row["observedUptimeMs"]) and integer(row["writtenUptimeMs"])
                and row["observedUptimeMs"] <= row["writtenUptimeMs"]
                and type(row["mainThread"]) is bool and integer(row["processId"], 1, 2**31-1), "APP_VALUES")
        if index == 0:
            require(row["phase"] == "observer-opened" and row["documentId"] is None, "APP_OPEN")
            process = row["processId"]
        else:
            require(row["phase"] != "observer-opened" and row["processId"] == process
                    and row["writtenUptimeMs"] >= last_written, "APP_PROCESS_CLOCK")
            require(rows[index-1]["phase"] not in {"observer-expired", "observer-stopped", "event-limit-reached"}, "APP_AFTER_CLOSE")
        last_written = row["writtenUptimeMs"]
    heartbeat_rows = [row for row in rows if row["phase"] == "heartbeat"]
    require(all(row["documentId"] is None and row["mainThread"] is False for row in heartbeat_rows), "HEARTBEAT_IDENTITY")
    heartbeats = [row["observedUptimeMs"] for row in heartbeat_rows]
    require(all(b >= a for a, b in zip(heartbeats, heartbeats[1:])), "HEARTBEAT_CLOCK")
    enqueued = [row for row in rows if row["phase"] == "main-queue-enqueued"]
    executed = [row for row in rows if row["phase"] == "main-queue-executed"]
    require(len(enqueued) <= 1 and len(executed) <= 1, "MAIN_QUEUE_SINGLE_PROBE")
    require(all(row["documentId"] is None for row in enqueued + executed), "MAIN_QUEUE_DOCUMENT")
    require(not enqueued or enqueued[0]["mainThread"] is False, "MAIN_QUEUE_ENQUEUE_THREAD")
    require(not executed or (enqueued and enqueued[0]["sequence"] < executed[0]["sequence"]
            and enqueued[0]["observedUptimeMs"] <= executed[0]["observedUptimeMs"]), "MAIN_QUEUE_ORDER")
    return rows, {
        "status": "HEARTBEATS_OBSERVED" if heartbeats else "NO_HEARTBEAT_OBSERVED",
        "heartbeatCount": len(heartbeats),
        "heartbeatSpanMs": heartbeats[-1] - heartbeats[0] if heartbeats else None,
        "largestObservedHeartbeatGapMs": max((b-a for a, b in zip(heartbeats, heartbeats[1:])), default=None),
        "terminatedBy": rows[-1]["phase"] if rows[-1]["phase"] in {"observer-expired", "observer-stopped", "event-limit-reached"} else "NO_TERMINAL_EVENT",
        "absenceProvesMissingLifecycle": False,
        "mainQueueProbeEnqueued": bool(enqueued),
        "mainQueueExecutionObserved": bool(executed),
        "executionWasOnMainThread": executed[0]["mainThread"] if executed else None,
        "mainQueueResponseMs": executed[0]["observedUptimeMs"] - enqueued[0]["observedUptimeMs"] if executed else None,
        "absenceProvesMainThreadBlocked": False,
    }


def contained_file(container, nonce):
    path = container / "Library" / "Caches" / "CherriStartupObservation" / (nonce + ".jsonl")
    require(NONCE.fullmatch(nonce), "APP_PATH_NONCE")
    for parent in [path] + list(path.parents)[:4]:
        require(not parent.is_symlink(), "APP_PATH_SYMLINK")
    require(path.resolve().is_relative_to(container.resolve()), "APP_PATH_ESCAPE")
    require(path.is_file() and stat.S_ISREG(path.stat().st_mode), "APP_FILE_MISSING")
    require(path.stat().st_size <= MAX_FILE, "APP_FILE_OVERSIZE")
    return path


def source_binding(repo, source):
    require(re.fullmatch(r"[0-9a-f]{40}", source), "SOURCE_SHA")
    head = subprocess.check_output(["git", "--no-replace-objects", "rev-parse", "HEAD"], cwd=repo, timeout=15).decode().strip()
    require(head == source, "SOURCE_HEAD")
    result = {}
    for name in SOURCES:
        path = repo / name
        require(path.is_file() and not path.is_symlink(), "SOURCE_FILE")
        raw = path.read_bytes()
        original = subprocess.check_output(["git", "--no-replace-objects", "show", source + ":" + name], cwd=repo, timeout=15)
        require(raw == original, "SOURCE_BYTES")
        result[name] = digest(raw)
    return result


def capture(repo, log, udid_file, output, quarantine, source, run_id):
    require(re.fullmatch(r"[0-9]+", run_id), "RUN_ID")
    require(not output.exists() and not quarantine.exists(), "OUTPUT_EXISTS")
    bindings = source_binding(repo, source)
    require(log.is_file() and log.stat().st_size <= 32*1024*1024, "TEST_LOG_BOUNDS")
    raw = log.read_bytes()
    tests, private_issues = test_observations(raw.decode("utf-8", errors="replace"))
    require(udid_file.is_file() and udid_file.stat().st_size <= 100, "UDID_FILE")
    udid = udid_file.read_text(encoding="ascii").strip()
    require(re.fullmatch(r"[0-9A-Fa-f]{8}(?:-[0-9A-Fa-f]{4}){3}-[0-9A-Fa-f]{12}", udid), "UDID")
    output.mkdir(parents=True)
    quarantine.mkdir(parents=True, mode=0o700)
    (quarantine / "issues-original.jsonl").write_bytes(("\n".join(private_issues) + "\n").encode())
    command = ["xcrun", "simctl", "get_app_container", udid, "org.cherrilang.Cherri", "data"]
    start = time.monotonic()
    with (quarantine / "container-query.stdout").open("wb") as stdout, (quarantine / "container-query.stderr").open("wb") as stderr:
        try:
            completed = subprocess.run(command, stdout=stdout, stderr=stderr, timeout=20)
            query = {"exitCode": completed.returncode, "timedOut": False}
        except subprocess.TimeoutExpired:
            query = {"exitCode": None, "timedOut": True}
    query["elapsedMs"] = int((time.monotonic() - start)*1000)
    query["originals"] = {suffix: {"bytes": (quarantine / ("container-query."+suffix)).stat().st_size,
                                 "sha256": file_digest(quarantine / ("container-query."+suffix))}
                          for suffix in ("stdout", "stderr")}
    container = None
    if query["exitCode"] == 0 and not query["timedOut"]:
        answer = b""
        if (quarantine / "container-query.stdout").stat().st_size <= 4_096:
            answer = (quarantine / "container-query.stdout").read_bytes()
        if answer:
            candidate = Path(answer.decode("utf-8").strip())
            if candidate.is_absolute() and candidate.is_dir():
                container = candidate
    for test, observation in tests.items():
        observation.update(appStatus="NOT_OBSERVED", appError=None, appEvents=[], observerHealth=None, appArtifact=None)
        if not container:
            observation["appError"] = "CONTAINER_UNAVAILABLE"
            continue
        if not observation["uniqueTestInvocation"] or not observation["sessions"] or observation["rejectedMetadataCount"]:
            observation["appError"] = "TEST_CONTEXT_UNVERIFIED"
            continue
        nonce = observation["sessions"][0]["nonce"]
        try:
            path = contained_file(container, nonce)
            data = path.read_bytes()
            (quarantine / ("app-" + TESTS[test] + ".jsonl")).write_bytes(data)
            events, health = validate_app_events(data, test, nonce)
            require(events[0]["observedUptimeMs"] >= observation["sessions"][0]["uptimeMs"], "APP_BEFORE_LAUNCH")
            filename = "startup-events-" + TESTS[test] + ".jsonl"
            (output / filename).write_bytes(data)
            observation.update(appStatus="OBSERVED", appEvents=events, observerHealth=health,
                               appArtifact={"path": filename, "sha256": digest(data), "bytes": len(data)})
        except (ValueError, OSError) as error:
            observation["appError"] = str(error) if isinstance(error, ValueError) else "APP_READ_ERROR"
    report = {
        "schemaVersion": "1", "scope": "STARTUP_OBSERVATION_ONLY", "implementationSha": source,
        "runId": run_id, "simulatorUdid": udid, "sourceFiles": bindings,
        "rawTestLogSha256": digest(raw), "containerQuery": query, "tests": tests,
        "readinessBudgetMilliseconds": 28_000, "pasteBudgetMilliseconds": 3_000,
        "acceptanceRecords": [], "formalAcceptanceGranted": False,
        "privacy": {"rawDescriptionsPublished": False, "containerPathPublished": False,
                    "arbitraryAppFilesRead": False, "sourceTextRead": False, "rasterRead": False},
    }
    (output / "startup-observation.json").write_bytes((json.dumps(report, indent=2, sort_keys=True)+"\n").encode())
    return report


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    for flag in ("repo", "log", "udid-file", "output", "quarantine"):
        parser.add_argument("--"+flag, required=True, type=Path)
    parser.add_argument("--source", required=True)
    parser.add_argument("--run-id", required=True)
    args = parser.parse_args()
    report = capture(args.repo, args.log, args.udid_file, args.output, args.quarantine, args.source, args.run_id)
    print(json.dumps({"scope": report["scope"], "observedAppCount": sum(v["appStatus"] == "OBSERVED" for v in report["tests"].values()),
                      "formalAcceptanceGranted": False}))
