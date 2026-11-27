"""Actual caller-input transport through the existing signed import/XCTest owner.

The caller supplies the already source/product-bound owner used for native
imports. This module performs real library reads, one import, one runtime
dispatch, callback capture and raw clipboard observation. It never infers a
native List/Map/Void type from clipboard or callback formatting, and does not
grant an acceptance tier.
"""
from __future__ import annotations

import base64
import hashlib
import json
import plistlib
from pathlib import Path
import re
import sys
import time
import urllib.parse
import uuid

import native_contract_fixtures as fixtures
import project_prerequisite_observation as process_capture
import run_differential_batch as differential
import shortcut_library_identity as library

SCHEMA = "cherri-native-caller-input-observation/1"
PACKET_SCHEMA = "cherri-native-caller-input-packet/1"
PACKET_KEYS = frozenset((
    "schema", "caseId", "packetToken", "workflowName", "compileFilename",
    "canonicalTest", "canonicalSourceBase64", "canonicalSourceSha256",
    "sourceBase64", "sourceSha256", "sourceBytes", "unsignedBase64",
    "unsignedSha256", "unsignedBytes", "actionCount", "originalGraphOracle",
    "companionKind", "nativeExecuted", "formalAcceptanceGranted",
))
PREFIXES = {"nested_false": "CherriFN04Direct_", "nested_true": "CherriFN04Nested_",
            "without_functions": "CherriFN04Plain_"}
PLAIN_SOURCE = (b"let first=getVariable(system.input)\nlet second=getVariable(ShortcutInput)\n"
                b'setClipboard(f"{first}|{second}")')
PBCOPY_EXEC = (
    "import os,sys\n"
    "with open(sys.argv[2], 'rb') as original:\n"
    " os.dup2(original.fileno(), 0)\n"
    " os.execv('/usr/bin/xcrun', ['/usr/bin/xcrun','simctl','pbcopy',sys.argv[1]])\n"
)


def require(condition, code):
    if not condition:
        raise fixtures.FixtureRejected(code)


def original(path, raw):
    path = Path(path)
    with path.open("xb") as stream:
        stream.write(raw)
    return {"bytes": len(raw), "sha256": fixtures.sha(raw)}


def record(path, value):
    return original(path, library.canonical(value) + b"\n")


def validate_packet(value, canonical_fixture, *, permit_unit_token=False):
    require(type(value) is dict and set(value) == PACKET_KEYS and
            value["schema"] == PACKET_SCHEMA, "CALLER_PACKET_SCHEMA")
    case_id, token = value["caseId"], value["packetToken"]
    require(type(case_id) is str and case_id in PREFIXES and isinstance(token, str) and
            re.fullmatch("[0-9a-f]{32}", token) is not None, "CALLER_PACKET_IDENTITY")
    require(permit_unit_token or token != "0" * 32, "NON_NATIVE_UNIT_PACKET_TOKEN")
    case = fixtures.CASE_BY_TEST[fixtures.EXPORT_PARENT + "/FN04/" + case_id]
    fixtures.validate_fixture(canonical_fixture, case)
    name = PREFIXES[case_id] + token
    require(value["workflowName"] == name and value["compileFilename"] == name + ".cherri"
            and value["canonicalTest"] == case.original_test, "CALLER_PACKET_NAME_BINDING")
    require(value["nativeExecuted"] is False and value["formalAcceptanceGranted"] is False,
            "CALLER_PACKET_OVERCLAIM")
    require(value["canonicalSourceBase64"] == canonical_fixture["sourceBase64"] and
            value["canonicalSourceSha256"] == canonical_fixture["sourceSha256"],
            "ORIGINAL_CANONICAL_PROGRAM_BINDING")
    canonical_raw = base64.b64decode(value["canonicalSourceBase64"], validate=True)
    require(base64.b64encode(canonical_raw).decode() == value["canonicalSourceBase64"] and
            fixtures.sha(canonical_raw) == value["canonicalSourceSha256"] ==
            canonical_fixture["sourceSha256"] and
            value["canonicalSourceBase64"] == canonical_fixture["sourceBase64"],
            "ORIGINAL_CANONICAL_PROGRAM_CHANGED")
    source = fixtures._decoded(value["sourceBase64"], value["sourceBytes"],
                               value["sourceSha256"], fixtures.MAX_SOURCE)
    native = fixtures._decoded(value["unsignedBase64"], value["unsignedBytes"],
                               value["unsignedSha256"], fixtures.MAX_NATIVE)
    if case_id == "without_functions":
        require(source == PLAIN_SOURCE and value["companionKind"] == "CAPTURE_ORIGINAL_INPUT_READ_RESULTS"
                and value["originalGraphOracle"] == "p15NativeCallerPlainGraph",
                "PLAIN_INPUT_COMPANION_SOURCE")
    else:
        require(source == canonical_raw and value["companionKind"] == "ORIGINAL_SOURCE_COMPILER_NAME"
                and value["originalGraphOracle"] == "p15CallerInputGraphForWorkflow",
                "FUNCTION_INPUT_COMPANION_SOURCE")
    # The actual Go producer separately checks every operation and edge through
    # the complete original graph oracle. This byte parser binds that export;
    # it cannot replace the source/process/test proof for those graph checks.
    try:
        document = plistlib.loads(native)
    except (plistlib.InvalidFileException, ValueError, TypeError, OverflowError, RecursionError) as exc:
        raise fixtures.FixtureRejected("CALLER_PACKET_PLIST") from exc
    require(type(document) is dict, "CALLER_PACKET_DOCUMENT")
    actions = document.get("WFWorkflowActions")
    require(type(actions) is list and type(value["actionCount"]) is int and
            0 < len(actions) == value["actionCount"] <= 5000, "CALLER_PACKET_ACTIONS")
    require(all(type(action) is dict for action in actions), "CALLER_PACKET_ACTION_ENVELOPES")
    calls = [action for action in actions
             if action.get("WFWorkflowActionIdentifier") == "is.workflow.actions.runworkflow"]
    require(len(calls) == (0 if case_id == "without_functions" else 2 if case_id == "nested_true" else 1),
            "CALLER_PACKET_SELF_CALL_COUNT")
    for call in calls:
        parameters = call.get("WFWorkflowActionParameters", {})
        require(type(parameters) is dict and parameters.get("WFWorkflowName") == name and
                parameters.get("WFWorkflow") == {"isSelf": True, "workflowName": name},
                "CALLER_PACKET_SELF_CALL_IDENTITY")
    return {"caseId": case_id, "name": name, "token": token, "source": source,
            "unsigned": native, "sourceSha256": value["sourceSha256"],
            "unsignedSha256": value["unsignedSha256"]}


PACKET_PARENT = "TestP15NativeCallerInputPackets"
PACKET_EVENTS = frozenset([PACKET_PARENT] + [PACKET_PARENT + "/" + name for name in PREFIXES])
PACKET_COMMAND = ("go", "test", "-json", "-count=1", "-timeout=3m", "-run", "^" + PACKET_PARENT + "$", ".")
PACKET_MARKER = re.compile(
    r"^\s*[A-Za-z0-9_./-]+\.go:[0-9]+: P15_NATIVE_CALLER_PACKET "
    r"sha256=([0-9a-f]{64}) bytes=([1-9][0-9]*) base64=([A-Za-z0-9+/]+={0,2})$"
)


def parse_packets(raw, *, expected_sha256, canonical_exports, packet_token):
    """Bind all three named packets to the original process-bound export stream.

    The closed execution owner must additionally validate exact real Go argv,
    source before/after, token environment, exit, streams and owned cleanup.
    Parsing these bytes does not prove a process or any Apple execution.
    """
    require(type(raw) is bytes and 0 < len(raw) <= fixtures.MAX_JSONL and raw.endswith(b"\n"),
            "CALLER_GO_STREAM_BOUND")
    require(type(expected_sha256) is str and fixtures.SHA.fullmatch(expected_sha256) is not None
            and fixtures.sha(raw) == expected_sha256, "CALLER_GO_STREAM_HASH")
    require(type(packet_token) is str and re.fullmatch("[0-9a-f]{32}", packet_token) is not None
            and packet_token != "0" * 32, "CALLER_PACKET_TOKEN")
    require(type(canonical_exports) in (tuple, list) and len(canonical_exports) == len(fixtures.CASES),
            "ORIGINAL_EXPORT_INVENTORY")
    canonical = {}
    for value, case in zip(canonical_exports, fixtures.CASES):
        fixtures.validate_fixture(value, case)
        if case.requirement == "FN04":
            canonical[case.case_id] = value
    require(set(canonical) == set(PREFIXES), "ORIGINAL_CALLER_CASES")
    states, buffers, packets = {}, {}, {}
    package_started = package_passed = False
    for line in raw.splitlines():
        require(line, "EMPTY_CALLER_GO_EVENT")
        event = fixtures._json(line)
        require(type(event) is dict and event.get("Package") == fixtures.PACKAGE, "CALLER_GO_PACKAGE")
        action, test = event.get("Action"), event.get("Test", "")
        require(type(action) is str and type(test) is str, "CALLER_GO_EVENT_SCHEMA")
        require(not package_passed, "CALLER_EVENT_AFTER_PACKAGE_PASS")
        if not test:
            if action == "start":
                require(not package_started, "CALLER_DUPLICATE_PACKAGE_START")
                package_started = True
            elif action == "pass":
                require(package_started and set(states) == PACKET_EVENTS and
                        all(state == "pass" for state in states.values()), "CALLER_PACKAGE_BEFORE_CASES")
                package_passed = True
            else:
                require(action == "output", "CALLER_PACKAGE_NOT_PASSED")
            continue
        require(package_started and test in PACKET_EVENTS, "CALLER_UNEXPECTED_TEST")
        if action == "run":
            require(test not in states, "CALLER_DUPLICATE_TEST")
            require(test == PACKET_PARENT or states.get(PACKET_PARENT) == "run", "CALLER_CHILD_OUTSIDE_PARENT")
            states[test] = "run"
        elif action == "output":
            require(states.get(test) == "run" and type(event.get("Output")) is str,
                    "CALLER_OUTPUT_OUTSIDE_TEST")
            text = buffers.get(test, "") + event["Output"]
            require(len(text.encode("utf-8")) <= 3 * fixtures.MAX_EXPORT, "CALLER_TEST_OUTPUT_BOUND")
            while "\n" in text:
                output, text = text.split("\n", 1)
                if "P15_NATIVE_CALLER_PACKET" not in output:
                    continue
                match = PACKET_MARKER.fullmatch(output)
                require(match is not None and test != PACKET_PARENT and test not in packets,
                        "CALLER_EXPORT_MARKER_BINDING")
                encoded = fixtures._decoded(match[3], int(match[2]), match[1], fixtures.MAX_EXPORT)
                value = fixtures._json(encoded)
                case_id = test.removeprefix(PACKET_PARENT + "/")
                validate_packet(value, canonical[case_id])
                require(value["caseId"] == case_id and value["packetToken"] == packet_token,
                        "CALLER_EXPORT_CASE_TOKEN_BINDING")
                packets[test] = value
            buffers[test] = text
        elif action == "pass":
            require(states.get(test) == "run" and not buffers.get(test), "CALLER_INVALID_TEST_PASS")
            require(all(state == "pass" for child, state in states.items() if child.startswith(test + "/")),
                    "CALLER_PARENT_BEFORE_CHILD")
            require(test == PACKET_PARENT or test in packets, "CALLER_MISSING_PACKET")
            states[test] = "pass"
        else:
            raise fixtures.FixtureRejected("CALLER_TEST_NOT_PASSED")
    require(package_passed and set(packets) == PACKET_EVENTS - {PACKET_PARENT}, "CALLER_INCOMPLETE_EXPORT")
    return tuple(packets[PACKET_PARENT + "/" + name] for name in PREFIXES)


def expected_clipboard(case_id, input_text):
    require(re.fullmatch("CHERRI-CALLER-[0-9a-f]{32}", input_text) is not None,
            "FRESH_CALLER_INPUT_NONCE")
    if case_id == "without_functions":
        return (input_text + "|" + input_text).encode()
    return fixtures.caller_input_expected(case_id, input_text).encode()


def validate_runtime_bytes(callback, case_id, fixture_id, name, nonce, raw, input_text):
    require(type(nonce) is str and re.fullmatch("[0-9a-f]{32}", nonce) is not None and
            nonce != "0" * 32 and input_text == "CHERRI-CALLER-" + nonce, "CALLER_INPUT_RUN_BINDING")
    require(type(callback) is dict and callback.get("receiverSelfTest") is False and
            callback.get("eventSource") == "shortcuts-x-callback" and
            callback.get("status") == "SUCCESS" and callback.get("runId") == nonce and
            callback.get("fixtureId") == fixture_id and callback.get("expectedShortcutName") == name,
            "ACTUAL_RUNTIME_CALLBACK_IDENTITY")
    original_classification = differential.classify_callback(callback, {"id": fixture_id, "name": name, "expected": None}, nonce)
    require(original_classification == "OBSERVED_ONLY", "ORIGINAL_CALLBACK_CLASSIFICATION")
    expected = expected_clipboard(case_id, input_text)
    require(type(raw) is bytes and raw == expected, "ORIGINAL_CALLER_INPUT_VALUE_MISMATCH")
    return {"caseId": case_id, "rawClipboardBytes": len(raw),
            "rawClipboardSha256": fixtures.sha(raw), "inputSha256": fixtures.sha(input_text.encode()),
            "fieldCount": len(raw.split(b"|")), "actualCallerInputMatched": True,
            "originalCallbackClassification": original_classification,
            "nativeRawTypeObserved": False, "formalAcceptanceGranted": False}


def dispatch_ui(owner, phase, operation, name, argv):
    """Reuse the qualified capture; publish only after its real owned Popen."""
    require(operation in ("import", "runtime") and
            re.fullmatch("[A-Za-z0-9_-]{1,128}", phase) is not None, "UI_DISPATCH_IDENTITY")
    require(owner.deadline - time.monotonic() > 65, "DISPATCH_EXECUTION_AND_CLEANUP_BUDGET")
    folder = owner.folder / ("dispatch-" + phase)
    folder.mkdir(mode=0o700)
    dispatch_path = owner.folder / ("dispatch-" + phase + ".json")
    receipt = {"schemaVersion": 1, "command": argv, "timeoutSeconds": 60,
               "startedUTC": library.now(), "outcome": "STARTING", "exitCode": None,
               "timedOut": False, "parentReaped": False}
    command = {"id": phase, "operation": operation, "shortcutName": name}
    if operation == "import":
        command["requiresAbsentLibraryRecord"] = True

    last_details = {}

    def journal(event, details):
        nonlocal last_details
        last_details = dict(details)
        record(folder / (event + ".original.json"), details)
        if event == "started":
            receipt.update(pid=details["pid"], outcome="LAUNCHED")
            differential.atomic_json(dispatch_path, receipt)
            differential.atomic_json(owner.folder / "command.json", command)
            receipt["phasePublishedUTC"] = library.now()
            differential.atomic_json(dispatch_path, receipt)

    observed = None
    interrupted = None
    try:
        with (folder / "stdout.original").open("xb") as stdout, (folder / "stderr.original").open("xb") as stderr:
            observed, details = process_capture.capture(argv, 60, output=stdout, error_output=stderr,
                hard_deadline=min(owner.deadline, time.monotonic() + 65), journal=journal)
        last_details = details
    except BaseException as error:
        interrupted = type(error).__name__
        raise
    finally:
        # The unchanged capture finalizes its actual owned child before calling
        # its finished journal, including interruption in the publication hook.
        details = last_details
        receipt.update(processReturncode=details.get("returnCodeObserved"),
            outcome="INTERRUPTED", parentReaped=details.get("cleanupParentReaped") is True,
            completedUTC=details.get("finishedAt", library.now()))
        if observed is not None:
            receipt.update(exitCode=observed["exitCode"],
                outcome="EXITED" if observed["result"] == "PASS" and observed["exitCode"] == 0 else observed["result"],
                timedOut=observed["result"] in ("TIMEOUT", "DEADLINE_EXHAUSTED"),
                exitObservedUTC=details["finishedAt"], elapsedSeconds=observed["elapsedMs"] / 1000)
        differential.atomic_json(dispatch_path, receipt)
        record(folder / "result.original.json", {"argv": argv, "observation": observed, "details": details,
            "interruptionType": interrupted})
    require(observed["result"] == "PASS" and observed["exitCode"] == 0 and
            details["cleanupParentReaped"] is True and details["groupPresentAfterCleanup"] is False,
            "OWNED_NATIVE_DISPATCH_PROCESS")
    return receipt


def validate_library_rows(ui, dispatch, rows, *, phase, name):
    """Join the two actual fresh observations emitted by the qualified selector.

    Node matching remains the existing Swift selector's responsibility. These
    rows are accepted only from that source/product-bound XCTest run, never as
    a replacement for its selector or the separate native database identity.
    """
    require(type(rows) is list and len(rows) == 2, "TWO_FRESH_LIBRARY_OBSERVATIONS")
    completed = library.instant(dispatch["completedUTC"])
    matched = library.instant(ui["importTiming"]["libraryMatchedAfterCaptureUTC"])
    previous = None
    for row in rows:
        require(type(row) is dict and row.get("schema") == "cherri-current-library-selection-observation/1"
                and row.get("phase") == phase and row.get("expectedName") == name,
                "LIBRARY_ROW_CONTEXT")
        require(row.get("canConfirm") is True and row.get("foreground") is True
                and row.get("dispatchConfirmed") is True
                and type(row.get("currentNamedCellCount")) is int
                and row["currentNamedCellCount"] == 1
                and type(row.get("ordinal")) is int and row["ordinal"] > 0,
                "LIBRARY_ROW_UNIQUE_CURRENT")
        instant = library.instant(row["observedUTC"])
        require(completed <= instant <= matched < library.instant(row["deadlineUTC"]),
                "LIBRARY_ROW_TIMELINE")
        if previous is not None:
            require(row["ordinal"] == previous["ordinal"] + 1 and
                    library.instant(previous["observedUTC"]) <= instant,
                    "LIBRARY_ROWS_NOT_FRESH_AND_SEQUENTIAL")
        previous = row
    return {"schema": "cherri-two-fresh-library-observations/1", "phase": phase,
            "expectedName": name, "ordinals": [row["ordinal"] for row in rows],
            "observedUTC": [row["observedUTC"] for row in rows],
            "formalAcceptanceGranted": False}


def retain_library_rows(owner, ui, dispatch, phase, name):
    paths = sorted(owner.folder.glob(phase + "-library-observation-*.json"))
    require(2 <= len(paths) <= 256, "LIBRARY_OBSERVATION_FILE_COUNT")
    original_rows, pins = [], []
    for path in paths[-2:]:
        raw = library.read(path, 2 * 1024 * 1024)
        row = library.strict_json(raw)
        require(path.name == phase + "-library-observation-" + ("%04d" % row["ordinal"]) + ".json",
                "LIBRARY_OBSERVATION_FILENAME")
        original_rows.append(row)
        pins.append({"path": path.name, "bytes": len(raw), "sha256": fixtures.sha(raw)})
    value = validate_library_rows(ui, dispatch, original_rows, phase=phase, name=name)
    value["originals"] = pins
    record(owner.folder / (phase + "-fresh-library-proof.original.json"), value)
    return value


def imported_unique(owner, expected, unsigned, signed):
    """Original absence, unique fresh AX and one new SQLite record precede run."""
    before_dir = owner.folder / (expected["phaseId"] + "-library-before")
    before = library.observe(owner.device_data_path, expected, "before", unsigned, signed, before_dir)
    library.admit_absence(before, expected)
    dispatch = dispatch_ui(owner, expected["phaseId"], "import", expected["expectedName"],
        ["/usr/bin/xcrun", "simctl", "openurl", owner.device, Path(signed).as_uri()])
    ui = differential.wait_file(owner.folder / ("ack-" + expected["phaseId"] + ".json"), 90, owner.xctest)
    require(ui.get("id") == expected["phaseId"] and ui.get("operation") == "import" and
            ui.get("name") == expected["expectedName"] and ui.get("outcome") == "IMPORTED" and
            ui.get("identityVerified") is True and ui.get("libraryIdentityVerified") is True and
            ui.get("libraryUniqueCurrentCellVerified") is True and
            ui.get("requiresAbsentLibraryRecord") is True and ui.get("replaceTapped") is False,
            "UNIQUE_SIGNED_IMPORT_UI")
    timing = ui.get("importTiming", {})
    require(type(timing.get("sheetMatchedCandidateCount")) is int and
            timing["sheetMatchedCandidateCount"] == 1 and
            type(timing.get("libraryCurrentNamedCellCount")) is int and
            timing["libraryCurrentNamedCellCount"] == 1 and
            isinstance(timing.get("sheetMatchedAfterCaptureUTC"), str) and
            isinstance(timing.get("libraryMatchedAfterCaptureUTC"), str), "FRESH_IMPORT_UI_OBSERVATIONS")
    retain_library_rows(owner, ui, dispatch, expected["phaseId"], expected["expectedName"])
    after_dir = owner.folder / (expected["phaseId"] + "-library-after")
    after = library.observe(owner.device_data_path, expected, "after", unsigned, signed, after_dir)
    binding = library.bind_new_record(before, after, expected, dispatch)
    record(owner.folder / (expected["phaseId"] + "-new-record.original.json"), binding)
    return binding


def observe_caller_input(owner, packet, canonical_fixture, unsigned, signed, expected):
    """One real imported FN04 case; the external closed owner binds source/build.

    owner is the existing native owner with repo/product/source admission,
    capture(), deadline, folder, device, device_data_path, documents and xctest.
    It retains all raw XCTest/AX/private originals and closes the owned group.
    This function never constructs or assumes a successful owner admission.
    """
    require(sys.platform == "darwin" and sys.version_info >= (3, 10), "DARWIN_SHARED_MONOTONIC_CLOCK")
    checked = validate_packet(packet, canonical_fixture)
    expected = library.context(expected)
    require(expected["expectedName"] == checked["name"] and
            expected["unsignedSHA256"] == checked["unsignedSha256"] and
            expected["deviceUdid"] == owner.device, "NATIVE_CASE_CONTEXT")
    unsigned, signed = Path(unsigned), Path(signed)
    require(library.read(unsigned) == checked["unsigned"], "UNSIGNED_PACKET_CHANGED")
    signed_raw = library.read(signed)
    require(signed_raw.startswith(b"AEA1") and fixtures.sha(signed_raw) == expected["signedSHA256"],
            "BOUND_SIGNED_PACKET")
    require(owner.admission_verified is True and owner.xctest.poll() is None, "EXISTING_OWNER_ADMISSION")
    binding = imported_unique(owner, expected, unsigned, signed)
    nonce = expected["runNonce"]
    require(re.fullmatch("[0-9a-f]{32}", nonce) is not None and nonce != "0" * 32,
            "FRESH_NATIVE_RUN_NONCE")
    input_text = "CHERRI-CALLER-" + nonce
    fixture_id = "fn04-" + checked["caseId"] + "-" + checked["token"]
    reset = ("CHERRI-CALLER-RESET-" + uuid.uuid4().hex).encode()
    reset_path = owner.folder / (nonce + "-clipboard-reset.original.txt")
    original(reset_path, reset)
    owner.capture(nonce + "-clipboard-reset",
        [sys.executable, "-I", "-B", "-c", PBCOPY_EXEC, owner.device, str(reset_path)], 25)
    _, observed_reset, _ = owner.capture(nonce + "-clipboard-reset-readback",
        ["/usr/bin/xcrun", "simctl", "pbpaste", owner.device], 25)
    require(observed_reset == reset, "CLIPBOARD_RESET_NOT_OBSERVED")
    query = {"name": checked["name"], "input": "text", "text": input_text,
             "x-source": "CherriSimulatorQAReceiver"}
    for status in ("success", "error", "cancel"):
        query["x-" + status] = "cherriqa://" + status + "?" + urllib.parse.urlencode({
            "runId": nonce, "fixtureId": fixture_id, "shortcutName": checked["name"],
            "source": "shortcuts-x-callback"})
    target = "shortcuts://x-callback-url/run-shortcut?" + urllib.parse.urlencode(query)
    trigger = "cherriqa://trigger?" + urllib.parse.urlencode({"target": target})
    phase = "runtime-" + nonce
    claim = owner.folder / (checked["caseId"] + "-runtime-claim.original.json")
    record(claim, {"schema": SCHEMA, "caseId": checked["caseId"], "runNonce": nonce,
        "inputText": input_text, "targetURL": target, "dispatchOrdinal": 1,
        "unsignedSha256": checked["unsignedSha256"], "signedSha256": expected["signedSHA256"],
        "libraryNativeUUID": binding["nativeRecord"]["workflowUUID"],
        "formalAcceptanceGranted": False})
    dispatch = dispatch_ui(owner, phase, "runtime", checked["name"],
        ["/usr/bin/xcrun", "simctl", "openurl", owner.device, trigger])
    callback = None
    deadline = min(owner.deadline, time.monotonic() + 48)
    while time.monotonic() < deadline:
        path = owner.documents / "callback-events.json"
        if path.is_file():
            raw = library.read(path)
            events = library.strict_json(raw)
            require(type(events) is list and all(type(e) is dict for e in events), "CALLBACK_EVENT_ARRAY")
            matched = [e for e in events if e.get("runId") == nonce]
            require(len(matched) <= 1, "DUPLICATE_NATIVE_CALLBACK")
            if matched:
                callback = matched[0]
                original(owner.folder / (nonce + "-callback-events.original.json"), raw)
                differential.atomic_json(owner.folder / ("completion-" + phase + ".json"), callback)
                break
        require(owner.xctest.poll() is None, "XCTEST_ENDED_BEFORE_CALLBACK")
        owner.check_capacity()
        time.sleep(0.25)
    ui = differential.wait_file(owner.folder / ("ack-" + phase + ".json"), 60, owner.xctest)
    require(ui.get("id") == phase and ui.get("operation") == "runtime" and
            ui.get("outcome") == "CALLBACK_OBSERVED_BY_HOST", "NATIVE_RUNTIME_UI")
    require(callback is not None, "NATIVE_CALLBACK_NOT_OBSERVED")
    _, raw, _ = owner.capture(nonce + "-actual-clipboard",
        ["/usr/bin/xcrun", "simctl", "pbpaste", owner.device], 25)
    original(owner.folder / (nonce + "-clipboard-result.original.txt"), raw)
    result = validate_runtime_bytes(callback, checked["caseId"], fixture_id, checked["name"],
                                    nonce, raw, input_text)
    require(library.read(unsigned) == checked["unsigned"] and library.read(signed) == signed_raw,
            "PACKET_CHANGED_DURING_NATIVE_EXECUTION")
    result.update(schema=SCHEMA, status="OBSERVED_EXACT_CALLER_INPUT", context=expected,
        runNonce=nonce, signedSha256=expected["signedSHA256"], unsignedSha256=checked["unsignedSha256"],
        nativeLibraryUUID=binding["nativeRecord"]["workflowUUID"], actualRuntimeDispatches=1,
        callbackSHA256=fixtures.sha(library.canonical(callback)), nativeExecuted=True,
        dispatchReceiptSHA256=fixtures.sha(library.canonical(dispatch)))
    record(owner.folder / (checked["caseId"] + "-result.original.json"), result)
    return result
