#!/usr/bin/env python3
"""Bound post-test02 inspection and retain explicitly incomplete acquisition.

The test command and its original 900-second phase are unchanged. Metadata
queries use only that phase's remaining allowance. Private originals retain
their exact bytes; no callback, trace, import or runtime success is invented.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import signal
import subprocess
import tempfile
import time

from simulator_session_recovery import require, run_process

PHASE_MS = 900_000
COMMAND_MS = 20_000
MAX_PARSE_BYTES = 1024 * 1024
STAGES = ("COMPILED", "TEST02_STARTED", "TEST02_COMPLETED")
INSPECTIONS = ("xcresult-part1", "xcresult-part2", "receiver-data")
GATES = ("RUNNER_AVAILABLE", "SIMULATOR_BOOTED", "SHORTCUTS_APP_INSTALLED",
         "SHORTCUTS_LAUNCHED", "EDITOR_OPENED", "FILE_IMPORT_ATTEMPTED",
         "COMPILER_ARTIFACT_IMPORTED", "SHORTCUT_EXISTS_IN_LIBRARY",
         "ACTION_EDITOR_CAPTURED", "SHORTCUT_RUN_ATTEMPTED",
         "SHORTCUT_RUN_COMPLETED", "CALLBACK_CAPTURED", "TRACE_CAPTURED",
         "PHYSICAL_DEVICE_TESTED")
CONTEXT_KEYS = {"compilerCommitSha", "runId", "targetFixtureId",
                "compiledSourceSha256", "unsignedPlistSha256", "buildProfile",
                "traceExpectation", "compilerManifestSha256"}
RESULTS = {"PASS", "EXIT_NONZERO", "TIMEOUT", "LAUNCH_FAILED",
           "DEADLINE_EXHAUSTED", "CANCELLED", "MISSING_INPUT", "INVALID_INPUT"}
PARSES = {"NOT_READ", "JSON_OBJECT", "RECEIVER_CONTAINER", "INVALID", "OVERSIZE"}
SHA = re.compile(r"[0-9a-f]{64}")
DEVICE = re.compile(r"[0-9A-Fa-f]{8}(?:-[0-9A-Fa-f]{4}){3}-[0-9A-Fa-f]{12}")


def sha(data):
    return hashlib.sha256(data).hexdigest()


def integer(value, low=0, high=10**18):
    return type(value) is int and low <= value <= high


def unique_json(data):
    def unique(pairs):
        value = {}
        for key, item in pairs:
            require(key not in value, "DUPLICATE_JSON_KEY")
            value[key] = item
        return value
    return json.loads(data, object_pairs_hook=unique)


def contained(root, relative, *, exists=True):
    root = Path(root)
    require(root.is_dir() and not root.is_symlink(), "ARTIFACT_ROOT_INVALID")
    require(isinstance(relative, str) and re.fullmatch(r"[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)*", relative),
            "ORIGINAL_PATH_INVALID")
    pieces = relative.split("/")
    require(all(x not in (".", "..") for x in pieces), "ORIGINAL_PATH_INVALID")
    path = root.joinpath(*pieces)
    require(all(not root.joinpath(*pieces[:i]).is_symlink() for i in range(1, len(pieces) + 1)),
            "ORIGINAL_SYMLINK")
    if exists:
        require(path.is_file(), "ORIGINAL_UNAVAILABLE")
    return path


def read_bytes(root, relative, maximum=MAX_PARSE_BYTES):
    path = contained(root, relative)
    require(path.stat().st_size <= maximum, "ORIGINAL_PARSE_SIZE")
    return path.read_bytes()


def file_binding(root, relative):
    path = contained(root, relative)
    digest, size = hashlib.sha256(), 0
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(65536), b""):
            size += len(block)
            digest.update(block)
    return {"path": relative, "bytes": size, "sha256": digest.hexdigest()}


def check_binding(root, value, relative):
    require(isinstance(value, dict) and set(value) == {"path", "bytes", "sha256"},
            "ORIGINAL_BINDING_FIELDS")
    require(value["path"] == relative, "ORIGINAL_BINDING_PATH")
    require(integer(value["bytes"]) and isinstance(value["sha256"], str)
            and SHA.fullmatch(value["sha256"]), "ORIGINAL_BINDING_VALUE")
    require(file_binding(root, relative) == value, "ORIGINAL_BINDING_MISMATCH")


def write_new(root, relative, value):
    path = contained(root, relative, exists=False)
    require(path.parent.is_dir(), "ORIGINAL_PARENT_REQUIRED")
    data = (json.dumps(value, ensure_ascii=True, indent=2) + "\n").encode("utf-8")
    # Link only a fully written, fsynced private file into its final name.
    # Hard-link creation is exclusive: an existing completed receipt wins.
    # A failed/interrupted write remains private instead of leaving a truncated
    # allowlisted file that would block all other truthful evidence.
    private = contained(root, "builds", exists=False)
    require(private.is_dir() and not private.is_symlink(), "PRIVATE_ORIGINAL_DIRECTORY")
    descriptor, temporary_name = tempfile.mkstemp(prefix=".qa-receipt-", suffix=".tmp", dir=private)
    temporary = Path(temporary_name)
    with os.fdopen(descriptor, "wb") as stream:
        stream.write(data)
        stream.flush()
        os.fsync(stream.fileno())
    os.link(temporary, path)
    temporary.unlink()
    return data


def context(root, source, run_id, fixture):
    require(isinstance(source, str) and re.fullmatch(r"[0-9a-f]{40}", source), "SOURCE_INVALID")
    require(isinstance(run_id, str) and re.fullmatch(r"[A-Za-z0-9_-]{1,64}", run_id), "RUN_INVALID")
    require(Path(root).name == run_id, "RUN_ROOT_MISMATCH")
    require(isinstance(fixture, str) and re.fullmatch(r"[A-Za-z0-9_-]{1,96}", fixture), "FIXTURE_INVALID")
    data = read_bytes(root, "compiler-manifest.json", 4 * MAX_PARSE_BYTES)
    manifest = unique_json(data)
    require(isinstance(manifest, dict) and manifest.get("syntheticOnly") is True
            and manifest.get("compilerCommitSha") == source, "COMPILER_CONTEXT_INVALID")
    fixtures = manifest.get("fixtures")
    require(isinstance(fixtures, list) and all(isinstance(f, dict) for f in fixtures),
            "FIXTURE_MANIFEST_INVALID")
    matching = [f for f in fixtures if f.get("fixtureId") == fixture]
    require(len(matching) == 1, "FIXTURE_CONTEXT_AMBIGUOUS")
    f = matching[0]
    for key in ("sourceSha256", "unsignedPlistSha256"):
        require(isinstance(f.get(key), str) and SHA.fullmatch(f[key]), "FIXTURE_DIGEST_INVALID")
    require((f.get("buildProfile"), f.get("runtimeTraceExpectation"))
            in (("debug", "REQUIRED_NDJSON"), ("release", "ABSENT_RELEASE")),
            "FIXTURE_PROFILE_INVALID")
    return {"compilerCommitSha": source, "runId": run_id, "targetFixtureId": fixture,
            "compiledSourceSha256": f["sourceSha256"], "unsignedPlistSha256": f["unsignedPlistSha256"],
            "buildProfile": f["buildProfile"], "traceExpectation": f["runtimeTraceExpectation"],
            "compilerManifestSha256": sha(data)}


def checkpoint_path(stage):
    require(stage in STAGES, "CHECKPOINT_STAGE_INVALID")
    return "builds/qa-acquisition-" + stage.lower() + ".json"


def check_checkpoint(value, expected, stage):
    require(isinstance(value, dict) and set(value) == CONTEXT_KEYS | {
        "schemaVersion", "stage", "monotonicNs", "deviceId", "gates",
        "test02PhaseExitCode", "expectedRunId", "requireRuntime", "testPhaseBudgetMs", "formalAcceptanceGranted"},
        "CHECKPOINT_FIELDS")
    require(all(value.get(k) == v for k, v in expected.items()), "CHECKPOINT_CONTEXT")
    require(value["schemaVersion"] == "1.0" and value["stage"] == stage, "CHECKPOINT_STAGE")
    require(integer(value["monotonicNs"], 1), "CHECKPOINT_CLOCK")
    require(value["testPhaseBudgetMs"] == PHASE_MS and type(value["testPhaseBudgetMs"]) is int,
            "CHECKPOINT_BUDGET")
    require(value["formalAcceptanceGranted"] is False and type(value["requireRuntime"]) is bool, "CHECKPOINT_ACCEPTANCE")
    require(isinstance(value["gates"], dict) and set(value["gates"]) == set(GATES)
            and all(isinstance(x, str) and re.fullmatch(r"[A-Z][A-Z0-9_]{0,63}", x)
                    for x in value["gates"].values()), "CHECKPOINT_GATES")
    require(isinstance(value["expectedRunId"], str)
            and (not value["expectedRunId"] or re.fullmatch(r"[A-Za-z0-9_-]{1,128}", value["expectedRunId"])),
            "CHECKPOINT_NONCE")
    if stage == "COMPILED":
        require(value["deviceId"] is None and value["test02PhaseExitCode"] is None, "COMPILED_CHECKPOINT_STATE")
    else:
        require(isinstance(value["deviceId"], str) and DEVICE.fullmatch(value["deviceId"]),
                "CHECKPOINT_DEVICE")
        require((stage == "TEST02_STARTED" and value["test02PhaseExitCode"] is None)
                or (stage == "TEST02_COMPLETED" and integer(value["test02PhaseExitCode"], 0, 255)),
                "CHECKPOINT_TEST_EXIT")


def read_checkpoints(root, expected):
    records, bindings = [], []
    missing = False
    for stage in STAGES:
        relative = checkpoint_path(stage)
        path = contained(root, relative, exists=False)
        if not path.exists():
            missing = True
            continue
        require(not missing, "CHECKPOINT_ORDER")
        data = read_bytes(root, relative)
        value = unique_json(data)
        check_checkpoint(value, expected, stage)
        if records:
            require(value["monotonicNs"] >= records[-1]["monotonicNs"]
                    and value["requireRuntime"] == records[-1]["requireRuntime"], "CHECKPOINT_CLOCK_ORDER")
        if stage == "TEST02_COMPLETED":
            require(value["deviceId"] == records[-1]["deviceId"]
                    and value["expectedRunId"] == records[-1]["expectedRunId"], "CHECKPOINT_DEVICE_OR_NONCE")
        records.append(value)
        bindings.append({"stage": stage, **file_binding(root, relative)})
    require(records, "ACQUISITION_CHECKPOINT_MISSING")
    return records, bindings


def checkpoint(root, expected, stage, gates, *, device=None, nonce="", code=None, require_runtime=False, now_ns=time.monotonic_ns):
    require(stage in STAGES, "CHECKPOINT_STAGE_INVALID")
    if stage != "COMPILED":
        records, _ = read_checkpoints(root, expected)
        require(tuple(x["stage"] for x in records) == STAGES[:STAGES.index(stage)], "CHECKPOINT_ORDER")
    value = {"schemaVersion": "1.0", **expected, "stage": stage, "monotonicNs": now_ns(),
             "deviceId": device, "gates": dict(gates), "test02PhaseExitCode": code,
             "expectedRunId": nonce, "requireRuntime": require_runtime, "testPhaseBudgetMs": PHASE_MS, "formalAcceptanceGranted": False}
    check_checkpoint(value, expected, stage)
    if stage != "COMPILED":
        require(value["monotonicNs"] >= records[-1]["monotonicNs"], "CHECKPOINT_CLOCK_ORDER")
        if stage == "TEST02_COMPLETED":
            require(device == records[-1]["deviceId"] and nonce == records[-1]["expectedRunId"],
                    "CHECKPOINT_DEVICE_OR_NONCE")
    write_new(root, checkpoint_path(stage), value)
    return value


class InspectionCancelled(BaseException):
    def __init__(self, signum):
        self.signum = signum


def _cancel(signum, frame):
    raise InspectionCancelled(signum)


def _parsed_output(stage, data, device):
    if len(data) > MAX_PARSE_BYTES:
        return "OVERSIZE"
    try:
        if stage.startswith("xcresult-"):
            return "JSON_OBJECT" if isinstance(unique_json(data), dict) else "INVALID"
        text = data.decode("utf-8").strip()
        if (not text or "\x00" in text or "\n" in text or "\r" in text or len(text) > 4096
                or not Path(text).is_absolute()):
            return "INVALID"
        return "RECEIVER_CONTAINER" if device.upper() in (x.upper() for x in Path(text).parts) else "INVALID"
    except (UnicodeError, ValueError):
        return "INVALID"


def collect(root, expected, scratch, device, *, runner=run_process, now_ns=time.monotonic_ns):
    root, scratch = Path(root), Path(scratch)
    records, bindings = read_checkpoints(root, expected)
    require(tuple(x["stage"] for x in records) == STAGES, "TEST02_COMPLETION_REQUIRED")
    start, completed = records[1], records[2]
    require(device == start["deviceId"] == completed["deviceId"], "INSPECTION_DEVICE")
    require(scratch.is_dir() and not scratch.is_symlink(), "SCRATCH_REQUIRED")
    for stage in INSPECTIONS[:2]:
        require(not contained(root, f"tests/{stage}.json", exists=False).exists(), "STALE_INSPECTION_OUTPUT")
    receipt_path = contained(root, "runtime/post-test-inspection.json", exists=False)
    marker = contained(root, "builds/post-test-inspection-attempt.json", exists=False)
    require(not receipt_path.exists() and not marker.exists(), "INSPECTION_ALREADY_ATTEMPTED")
    write_new(root, "builds/post-test-inspection-attempt.json",
              {"compilerCommitSha": expected["compilerCommitSha"], "runId": expected["runId"],
               "targetFixtureId": expected["targetFixtureId"], "checkpointSha256": bindings[1]["sha256"]})
    origin = start["monotonicNs"]
    require(now_ns() >= completed["monotonicNs"], "INSPECTION_CLOCK_BEFORE_TEST")
    commands, cancelled = [], None
    for stage in INSPECTIONS:
        before_ns = now_ns()
        require(before_ns >= origin, "INSPECTION_CLOCK_INVALID")
        before_ms = (before_ns - origin) // 1_000_000
        allowance_ms = max(0, min(COMMAND_MS, PHASE_MS - before_ms))
        paths = {key: f"builds/post-test-{stage}.{key}" for key in ("stdout", "stderr")}
        input_path = scratch / ("CherriSimulatorQA-Part1.xcresult" if stage == "xcresult-part1"
                                else "CherriSimulatorQA.xcresult")
        args = (["xcrun", "xcresulttool", "get", "test-results", "summary", "--path", str(input_path)]
                if stage.startswith("xcresult-") else
                ["xcrun", "simctl", "get_app_container", device, "org.cherrilang.qa.Receiver", "data"])
        outcome = None
        with contained(root, paths["stdout"], exists=False).open("xb") as stdout, \
             contained(root, paths["stderr"], exists=False).open("xb") as stderr:
            if allowance_ms <= 0:
                outcome = {"result": "DEADLINE_EXHAUSTED", "exitCode": None, "elapsedMs": 0}
            elif stage.startswith("xcresult-") and input_path.is_symlink():
                outcome = {"result": "INVALID_INPUT", "exitCode": None, "elapsedMs": 0}
            elif stage.startswith("xcresult-") and not input_path.is_dir():
                outcome = {"result": "MISSING_INPUT", "exitCode": None, "elapsedMs": 0}
            else:
                try:
                    outcome = runner(args, allowance_ms / 1000, output=stdout, error_output=stderr)
                except InspectionCancelled as exc:
                    cancelled = exc.signum
                    outcome = {"result": "CANCELLED", "exitCode": None,
                               "elapsedMs": max(0, (now_ns() - before_ns) // 1_000_000)}
                except KeyboardInterrupt:
                    cancelled = int(signal.SIGINT)
                    outcome = {"result": "CANCELLED", "exitCode": None,
                               "elapsedMs": max(0, (now_ns() - before_ns) // 1_000_000)}
        after_ns = now_ns()
        require(after_ns >= before_ns, "INSPECTION_CLOCK_REGRESSION")
        after_ms = (after_ns - origin) // 1_000_000
        if outcome["result"] == "PASS" and (after_ms >= PHASE_MS or after_ms - before_ms > allowance_ms):
            outcome = {**outcome, "result": "DEADLINE_EXHAUSTED"}
        raw_bindings = {key: file_binding(root, relative) for key, relative in paths.items()}
        parse = "NOT_READ"
        if outcome["result"] == "PASS":
            if raw_bindings["stdout"]["bytes"] > MAX_PARSE_BYTES:
                parse = "OVERSIZE"
            else:
                data = read_bytes(root, paths["stdout"])
                parse = _parsed_output(stage, data, device)
                if parse == "JSON_OBJECT":
                    destination = contained(root, f"tests/{stage}.json", exists=False)
                    with destination.open("xb") as stream:
                        stream.write(data)
        commands.append({"stage": stage, "phaseStartMs": before_ms, "phaseEndMs": after_ms,
                         "allowanceMs": allowance_ms, **outcome, "parseStatus": parse,
                         "stdout": raw_bindings["stdout"], "stderr": raw_bindings["stderr"]})
        if cancelled is not None:
            break
    elapsed = (now_ns() - origin) // 1_000_000
    value = {"schemaVersion": "1.0", **expected, "deviceIdSha256": sha(device.upper().encode()),
             "phaseBudgetMs": PHASE_MS, "commandBudgetMs": COMMAND_MS, "phaseElapsedMs": elapsed,
             "checkpointBindings": bindings, "commands": commands,
             "status": "COLLECTED" if len(commands) == 3 and all(x["result"] == "PASS"
                       and x["parseStatus"] in ("JSON_OBJECT", "RECEIVER_CONTAINER") for x in commands) else "INCOMPLETE",
             "cancellationSignal": cancelled, "formalAcceptanceGranted": False}
    validate_inspection(value, root, expected)
    write_new(root, "runtime/post-test-inspection.json", value)
    return (128 + cancelled if cancelled is not None else 0 if value["status"] == "COLLECTED" else 70), value


def validate_inspection(value, root, expected):
    require(isinstance(value, dict) and set(value) == CONTEXT_KEYS | {
        "schemaVersion", "deviceIdSha256", "phaseBudgetMs", "commandBudgetMs", "phaseElapsedMs",
        "checkpointBindings", "commands", "status", "cancellationSignal", "formalAcceptanceGranted"},
        "INSPECTION_FIELDS")
    require(all(value.get(k) == v for k, v in expected.items()), "INSPECTION_CONTEXT")
    records, bindings = read_checkpoints(root, expected)
    require(tuple(x["stage"] for x in records) == STAGES, "INSPECTION_CHECKPOINTS")
    require(value["checkpointBindings"] == bindings, "INSPECTION_CHECKPOINT_BINDING")
    require(value["schemaVersion"] == "1.0" and value["formalAcceptanceGranted"] is False,
            "INSPECTION_SCHEMA_OR_ACCEPTANCE")
    require(type(value["phaseBudgetMs"]) is int and value["phaseBudgetMs"] == PHASE_MS
            and type(value["commandBudgetMs"]) is int and value["commandBudgetMs"] == COMMAND_MS,
            "INSPECTION_BUDGET")
    require(integer(value["phaseElapsedMs"]), "INSPECTION_ELAPSED")
    device = records[1]["deviceId"]
    require(value["deviceIdSha256"] == sha(device.upper().encode()), "INSPECTION_DEVICE")
    commands = value["commands"]
    require(isinstance(commands, list) and 1 <= len(commands) <= 3, "INSPECTION_COMMAND_COUNT")
    previous = (records[2]["monotonicNs"] - records[1]["monotonicNs"]) // 1_000_000
    for i, command in enumerate(commands):
        require(isinstance(command, dict) and set(command) == {
            "stage", "phaseStartMs", "phaseEndMs", "allowanceMs", "result", "exitCode",
            "elapsedMs", "parseStatus", "stdout", "stderr"}, "INSPECTION_COMMAND_FIELDS")
        stage = INSPECTIONS[i]
        require(command["stage"] == stage, "INSPECTION_COMMAND_ORDER")
        for key in ("phaseStartMs", "phaseEndMs", "allowanceMs", "elapsedMs"):
            require(integer(command[key]), "INSPECTION_COMMAND_TIME")
        before, after = command["phaseStartMs"], command["phaseEndMs"]
        require(previous <= before <= after <= value["phaseElapsedMs"], "INSPECTION_TIME_ORDER")
        require(command["allowanceMs"] == max(0, min(COMMAND_MS, PHASE_MS - before)),
                "INSPECTION_ALLOWANCE")
        require(command["elapsedMs"] <= after - before + 3, "INSPECTION_UNDERSTATED_DURATION")
        result, code = command["result"], command["exitCode"]
        require(result in RESULTS and command["parseStatus"] in PARSES, "INSPECTION_COMMAND_RESULT")
        if result == "PASS":
            require(type(code) is int and code == 0 and command["allowanceMs"] > 0
                    and after < PHASE_MS and after - before <= command["allowanceMs"],
                    "INSPECTION_LATE_OR_FALSE_PASS")
        elif result == "EXIT_NONZERO":
            require(type(code) is int and code != 0, "INSPECTION_NONZERO_EXIT")
        else:
            require(code is None or (result == "DEADLINE_EXHAUSTED" and type(code) is int and code == 0),
                    "INSPECTION_FAILURE_EXIT")
        for key in ("stdout", "stderr"):
            check_binding(root, command[key], f"builds/post-test-{stage}.{key}")
        expected_parse = "NOT_READ"
        if result == "PASS":
            raw = command["stdout"]
            expected_parse = ("OVERSIZE" if raw["bytes"] > MAX_PARSE_BYTES
                              else _parsed_output(stage, read_bytes(root, raw["path"]), device))
        require(command["parseStatus"] == expected_parse, "INSPECTION_ORIGINAL_PARSE")
        previous = after
    cancelled = value["cancellationSignal"]
    require(cancelled is None or (integer(cancelled, 1, 64) and commands[-1]["result"] == "CANCELLED"),
            "INSPECTION_CANCELLATION")
    require(not any(c["result"] == "CANCELLED" for c in commands[:-1])
            and ((commands[-1]["result"] == "CANCELLED") == (cancelled is not None)),
            "INSPECTION_CANCELLATION_ORDER")
    expected_status = ("COLLECTED" if len(commands) == 3 and all(
        c["result"] == "PASS" and c["parseStatus"] in ("JSON_OBJECT", "RECEIVER_CONTAINER")
        for c in commands) else "INCOMPLETE")
    require(value["status"] == expected_status, "INSPECTION_FALSE_COMPLETE")


def receiver_path(root, expected):
    data = read_bytes(root, "runtime/post-test-inspection.json")
    value = unique_json(data)
    validate_inspection(value, root, expected)
    command = next((x for x in value["commands"] if x["stage"] == "receiver-data"), None)
    require(command and command["result"] == "PASS" and command["parseStatus"] == "RECEIVER_CONTAINER",
            "RECEIVER_CONTAINER_NOT_OBSERVED")
    path = Path(read_bytes(root, command["stdout"]["path"]).decode("utf-8").strip())
    require(path.is_dir() and not path.is_symlink(), "RECEIVER_CONTAINER_UNAVAILABLE")
    return str(path)


OUTPUT_CONTEXT_PATH = "builds/qa-output-context.json"


def check_output_context(value, expected, records, bindings):
    require(isinstance(value, dict) and set(value) == CONTEXT_KEYS | {
        "schemaVersion", "stage", "monotonicNs", "expectedRunId",
        "requireRuntime", "checkpointBindings", "formalAcceptanceGranted"},
        "OUTPUT_CONTEXT_FIELDS")
    require(all(value.get(k) == v for k, v in expected.items()), "OUTPUT_CONTEXT_IDENTITY")
    require(value["schemaVersion"] == "1.0" and value["stage"] == "ASSERTIONS_EMITTER_STARTED"
            and value["formalAcceptanceGranted"] is False, "OUTPUT_CONTEXT_STAGE")
    require(integer(value["monotonicNs"], records[-1]["monotonicNs"]), "OUTPUT_CONTEXT_CLOCK")
    require(type(value["requireRuntime"]) is bool
            and value["requireRuntime"] == records[-1]["requireRuntime"], "OUTPUT_CONTEXT_REQUIREMENT")
    require(value["checkpointBindings"] == bindings, "OUTPUT_CONTEXT_CHECKPOINTS")
    require(isinstance(value["expectedRunId"], str)
            and (not value["expectedRunId"] or re.fullmatch(r"[A-Za-z0-9_-]{1,128}", value["expectedRunId"])),
            "OUTPUT_CONTEXT_NONCE")


def read_output_context(root, expected, records, bindings):
    path = contained(root, OUTPUT_CONTEXT_PATH, exists=False)
    if not path.exists():
        return None, None
    value = unique_json(read_bytes(root, OUTPUT_CONTEXT_PATH))
    check_output_context(value, expected, records, bindings)
    return value, file_binding(root, OUTPUT_CONTEXT_PATH)


def output_context(root, expected, nonce, *, now_ns=time.monotonic_ns):
    records, bindings = read_checkpoints(root, expected)
    value = {"schemaVersion": "1.0", **expected, "stage": "ASSERTIONS_EMITTER_STARTED",
             "monotonicNs": now_ns(), "expectedRunId": nonce,
             "requireRuntime": records[-1]["requireRuntime"], "checkpointBindings": bindings,
             "formalAcceptanceGranted": False}
    check_output_context(value, expected, records, bindings)
    write_new(root, OUTPUT_CONTEXT_PATH, value)
    return value


def check_existing_assertions(value, expected, nonce):
    # Normal oracle output has no CI runId field; its current invocation nonce
    # is bound by the source/run/fixture-owned emitter context (or acquisition
    # checkpoint before the emitter is reached). Reject any supplied foreign
    # runId as well as a borrowed compiler/fixture/profile tuple.
    keys = ("compilerCommitSha", "targetFixtureId", "compiledSourceSha256",
            "unsignedPlistSha256", "buildProfile", "traceExpectation")
    require(isinstance(value, dict) and value.get("schemaVersion") == "1.5"
            and all(value.get(k) == expected[k] for k in keys)
            and value.get("expectedRunId") == nonce
            and ("runId" not in value or value["runId"] == expected["runId"]),
            "EXISTING_ASSERTION_IDENTITY")


def _retained_summary(root, expected, nonce):
    path = contained(root, "summary.json", exists=False)
    if not path.exists():
        return None
    value = unique_json(read_bytes(root, "summary.json"))
    keys = ("compilerCommitSha", "runId", "targetFixtureId", "buildProfile", "traceExpectation")
    require(isinstance(value, dict) and all(value.get(k) == expected[k] for k in keys),
            "EXISTING_SUMMARY_IDENTITY")
    check_existing_assertions(unique_json(read_bytes(root, "runtime/output-assertions.json")), expected, nonce)
    return {"status": "EXISTING_SUMMARY_RETAINED", "sha256": sha(path.read_bytes())}


def finalize(root, expected, outcome, *, now_ns=time.monotonic_ns, custom_server=""):
    require(outcome in ("success", "failure", "cancelled", "skipped"), "STEP_OUTCOME_INVALID")
    records, bindings = read_checkpoints(root, expected)
    latest = records[-1]
    emitter, emitter_binding = read_output_context(root, expected, records, bindings)
    nonce = emitter["expectedRunId"] if emitter else latest["expectedRunId"]
    now = now_ns()
    require(integer(now, emitter["monotonicNs"] if emitter else latest["monotonicNs"]),
            "FINALIZER_CLOCK_INVALID")
    prior = _retained_summary(root, expected, nonce)
    if prior:
        return prior
    phase_elapsed = ((now - records[1]["monotonicNs"]) // 1_000_000 if len(records) > 1 else None)
    assertion_path = contained(root, "runtime/output-assertions.json", exists=False)
    if assertion_path.exists():
        assertions = unique_json(read_bytes(root, "runtime/output-assertions.json"))
        check_existing_assertions(assertions, expected, nonce)
    else:
        assertions = {"schemaVersion": "1.5", **{k: expected[k] for k in CONTEXT_KEYS
                      if k != "compilerManifestSha256"},
                      "releaseBuildVerified": False, "runtimeContractVerified": False,
                      "traceObserved": "UNVERIFIED", "expectedRunId": nonce,
                      "expectedSemanticOutput": "", "shortcutRunCompletedGate": "NOT_RUN",
                      "callbackCapturedGate": "NOT_RUN", "traceCapturedGate": "NOT_RUN",
                      "receiverSelfCallbackVerified": False, "compiledShortcutCallbackVerified": False,
                      "compiledShortcutRuntimeExecuted": False,
                      "observedCorrelatedCallbackStatus": "UNKNOWN",
                      "typedValueCoverage": "UNKNOWN_AT_TRACE_EVENT", "perActionCoverageVerified": False,
                      "diagnosticReason": "INCOMPLETE_ACQUISITION_" + latest["stage"],
                      "acquisitionIncomplete": True,
                      "notes": "The normal runtime oracle did not finish. Existing callback/trace bytes, if any, remain unverified; no callback or trace was synthesized."}
        try:
            write_new(root, "runtime/output-assertions.json", assertions)
        except FileExistsError:
            assertions = unique_json(read_bytes(root, "runtime/output-assertions.json"))
            check_existing_assertions(assertions, expected, nonce)
    terminal = {"schemaVersion": "1.0", **expected, "status": "INCOMPLETE_COLLECTION",
                "stepOutcome": outcome, "lastAcquiredStage": latest["stage"],
                "checkpointBindings": bindings, "outputContextBinding": emitter_binding,
                "phaseElapsedMs": phase_elapsed,
                "phaseExpired": phase_elapsed is not None and phase_elapsed >= PHASE_MS,
                "test02PhaseExitCode": latest["test02PhaseExitCode"], "runtimeAcceptanceGranted": False,
                "formalAcceptanceGranted": False,
                "assertionBinding": file_binding(root, "runtime/output-assertions.json")}
    validate_terminal(terminal, root, expected)
    write_new(root, "runtime/terminal-acquisition.json", terminal)
    from prepare_public_artifacts import scan_public_privacy
    privacy = scan_public_privacy(root, latest["deviceId"] or "", custom_server)
    gates = dict(latest["gates"])
    # A test process exit and checkpoint never substitute for an import or
    # completed callback/trace oracle. Earlier observed setup states remain.
    for key in ("COMPILER_ARTIFACT_IMPORTED", "SHORTCUT_EXISTS_IN_LIBRARY", "EDITOR_OPENED",
                "ACTION_EDITOR_CAPTURED", "SHORTCUT_RUN_ATTEMPTED", "SHORTCUT_RUN_COMPLETED",
                "CALLBACK_CAPTURED", "TRACE_CAPTURED"):
        gates[key] = "NOT_RUN"
    summary = {"schemaVersion": "1.4", "runId": expected["runId"],
               "compilerCommitSha": expected["compilerCommitSha"], "requireRuntime": latest["requireRuntime"],
               "targetFixtureId": expected["targetFixtureId"], "buildProfile": expected["buildProfile"],
               "traceExpectation": expected["traceExpectation"], "traceRequirementMet": False,
               "runtimeContractVerified": False, "releaseReadiness": "PARTIAL_READY",
               "privacyGatePassed": privacy["passed"], "privacyScope": "EXPLICIT_SYNTHETIC_TEXT_PATHS_ONLY",
               "pixelRedactionVerified": False, "editorInspectorStatus": "NOT_RUN",
               "privacyViolations": privacy["violations"], "gates": gates,
               "acquisitionIncomplete": True, "terminalAcquisitionStatus": "INCOMPLETE_COLLECTION"}
    try:
        write_new(root, "summary.json", summary)
    except FileExistsError:
        return _retained_summary(root, expected, nonce)
    return {"status": "INCOMPLETE_SUMMARY_RETAINED", "privacyGatePassed": privacy["passed"],
            "runtimeAcceptanceGranted": False}


def validate_terminal(value, root, expected):
    require(isinstance(value, dict) and set(value) == CONTEXT_KEYS | {
        "schemaVersion", "status", "stepOutcome", "lastAcquiredStage", "checkpointBindings",
        "phaseElapsedMs", "phaseExpired", "test02PhaseExitCode", "runtimeAcceptanceGranted",
        "formalAcceptanceGranted", "assertionBinding", "outputContextBinding"}, "TERMINAL_FIELDS")
    require(all(value.get(k) == v for k, v in expected.items()), "TERMINAL_CONTEXT")
    records, bindings = read_checkpoints(root, expected)
    require(value["checkpointBindings"] == bindings and value["lastAcquiredStage"] == records[-1]["stage"],
            "TERMINAL_CHECKPOINT_BINDING")
    require(value["schemaVersion"] == "1.0" and value["status"] == "INCOMPLETE_COLLECTION"
            and value["stepOutcome"] in ("success", "failure", "cancelled", "skipped")
            and value["runtimeAcceptanceGranted"] is False and value["formalAcceptanceGranted"] is False,
            "TERMINAL_FALSE_ACCEPTANCE")
    emitter, emitter_binding = read_output_context(root, expected, records, bindings)
    require(value["outputContextBinding"] == emitter_binding, "TERMINAL_OUTPUT_CONTEXT_BINDING")
    last_clock = emitter["monotonicNs"] if emitter else records[-1]["monotonicNs"]
    elapsed = value["phaseElapsedMs"]
    require((len(records) == 1 and elapsed is None)
            or (len(records) > 1 and integer(elapsed)
                and elapsed >= (last_clock - records[1]["monotonicNs"]) // 1_000_000),
            "TERMINAL_ELAPSED")
    require(type(value["phaseExpired"]) is bool
            and value["phaseExpired"] == (elapsed is not None and elapsed >= PHASE_MS),
            "TERMINAL_EXPIRY")
    require(value["test02PhaseExitCode"] == records[-1]["test02PhaseExitCode"]
            and type(value["test02PhaseExitCode"]) is type(records[-1]["test02PhaseExitCode"]), "TERMINAL_TEST_EXIT")
    check_binding(root, value["assertionBinding"], "runtime/output-assertions.json")
    nonce = emitter["expectedRunId"] if emitter else records[-1]["expectedRunId"]
    check_existing_assertions(unique_json(read_bytes(root, "runtime/output-assertions.json")), expected, nonce)


SOURCE_FILES = (
    "tools/shortcuts-simulator-qa/simulator_listapps_observation.py",
    "tools/shortcuts-simulator-qa/test_simulator_listapps_observation.py",
    "tools/shortcuts-simulator-qa/project_prerequisite_observation.py",
    "tools/shortcuts-simulator-qa/test_process_capture_journal.py",
    "tools/shortcuts-simulator-qa/simulator_phase_boundary.py",
    "tools/shortcuts-simulator-qa/test_simulator_phase_boundary.py",
    "tools/shortcuts-simulator-qa/nested_list_runtime.py",
    "tools/shortcuts-simulator-qa/nested_list_probe.py",
    "tools/shortcuts-simulator-qa/fixtures/qa-nested-list-control-v2.cherri",
    "tools/shortcuts-simulator-qa/fixtures/qa-nested-list-control-v2.expected.json",
    "tools/shortcuts-simulator-qa/fixtures/qa-nested-list-candidate-v2.cherri",
    "tools/shortcuts-simulator-qa/fixtures/qa-nested-list-candidate-v2.expected.json",
    "tools/shortcuts-simulator-qa/XcodeProject/ReceiverApp/ReceiverApp.swift",
    "tools/shortcuts-simulator-qa/xcode_destination_diagnostic.py",
    "tools/shortcuts-simulator-qa/project_prerequisite.py",
    "tools/shortcuts-simulator-qa/qa_terminal_receipts.py",
    "tools/shortcuts-simulator-qa/prepare_public_artifacts.py",
    "tools/shortcuts-simulator-qa/simulator_session_recovery.py",
    "tools/shortcuts-simulator-qa/xcode_destination_readiness.py",
    "scripts/ios27_shortcuts_simulator_qa.sh",
    ".github/workflows/cherri-ios27-shortcuts-simulator-qa.yml",
)


def verify_checkout(repo, source):
    repo = Path(repo).resolve()
    require(Path(__file__).resolve().parents[2] == repo, "ACQUISITION_SOURCE_OWNER")
    require(isinstance(source, str) and re.fullmatch(r"[0-9a-f]{40}", source), "SOURCE_INVALID")
    deadline = time.monotonic() + 10
    def git(*args):
        remaining = deadline - time.monotonic()
        require(remaining > 0, "SOURCE_GUARD_DEADLINE")
        return subprocess.run(["git", "--no-replace-objects", *args], cwd=repo,
                              capture_output=True, timeout=remaining, check=True).stdout
    require(git("rev-parse", "HEAD").decode().strip() == source, "CHECKOUT_SOURCE_MISMATCH")
    for relative in SOURCE_FILES:
        path = contained(repo, relative)
        require(path.stat().st_size <= 4 * MAX_PARSE_BYTES, "SOURCE_FILE_SIZE")
        require(path.read_bytes() == git("show", source + ":" + relative), "ACQUISITION_SOURCE_BYTES")


def _cli_context(args):
    repo, root = Path(args.repo), Path(args.root)
    verify_checkout(repo, args.source_sha)
    from prepare_public_artifacts import trusted_fixture_sources, verify_synthetic_sources
    verify_synthetic_sources(root, trusted_fixture_sources(repo / "tools/shortcuts-simulator-qa/fixtures"))
    fixture = args.fixture_id
    if not fixture:
        initial = unique_json(read_bytes(root, checkpoint_path("COMPILED")))
        fixture = initial.get("targetFixtureId")
    expected = context(root, args.source_sha, args.run_id, fixture)
    manifest = unique_json(read_bytes(root, "compiler-manifest.json", 4 * MAX_PARSE_BYTES))
    selected = next(f for f in manifest["fixtures"] if f["fixtureId"] == fixture)
    relative = "tools/shortcuts-simulator-qa/fixtures/" + selected["fileName"]
    # Compare the reviewed source against actual Git objects, not replacement
    # views or another checkout carrying a copied manifest.
    deadline = time.monotonic() + 5
    data = subprocess.run(["git", "--no-replace-objects", "show", args.source_sha + ":" + relative],
                          cwd=repo, capture_output=True, timeout=max(.001, deadline-time.monotonic()),
                          check=True).stdout
    require(contained(repo, relative).read_bytes() == data, "FIXTURE_GIT_SOURCE_BYTES")
    return expected


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("mode", choices=("checkpoint", "output-context", "collect", "receiver-path", "finalize"))
    parser.add_argument("--repo", required=True)
    parser.add_argument("--root", required=True)
    parser.add_argument("--source-sha", required=True)
    parser.add_argument("--run-id", required=True)
    parser.add_argument("--fixture-id")
    parser.add_argument("--stage", choices=STAGES)
    parser.add_argument("--gates", nargs=len(GATES))
    parser.add_argument("--udid")
    parser.add_argument("--nonce", default="")
    parser.add_argument("--require-runtime", choices=("true", "false"))
    parser.add_argument("--test-exit-code", type=int)
    parser.add_argument("--scratch")
    parser.add_argument("--step-outcome", choices=("success", "failure", "cancelled", "skipped"))
    args = parser.parse_args()
    expected = _cli_context(args)
    if args.mode == "checkpoint":
        require(args.gates is not None and args.require_runtime is not None, "GATE_SNAPSHOT_REQUIRED")
        checkpoint(args.root, expected, args.stage, dict(zip(GATES, args.gates)),
                   device=args.udid, nonce=args.nonce, code=args.test_exit_code,
                   require_runtime=args.require_runtime == "true")
        return 0
    if args.mode == "output-context":
        output_context(args.root, expected, args.nonce)
        return 0
    if args.mode == "receiver-path":
        print(receiver_path(args.root, expected))
        return 0
    if args.mode == "finalize":
        result = finalize(args.root, expected, args.step_outcome,
                          custom_server=os.environ.get("CHERRI_SIGNING_SERVER", "").strip())
        print(json.dumps(result, sort_keys=True))
        return 0
    require(args.scratch is not None and args.udid is not None, "COLLECTION_INPUTS_REQUIRED")
    old = {s: signal.getsignal(s) for s in (signal.SIGINT, signal.SIGTERM)}
    try:
        for signum in old:
            signal.signal(signum, _cancel)
        code, receipt = collect(args.root, expected, args.scratch, args.udid)
        print(json.dumps({"status": receipt["status"], "commandResults": [
            {"stage": x["stage"], "result": x["result"], "parseStatus": x["parseStatus"]}
            for x in receipt["commands"]], "formalAcceptanceGranted": False}, sort_keys=True))
        return code
    finally:
        for signum, handler in old.items():
            signal.signal(signum, handler)


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (OSError, ValueError, subprocess.SubprocessError):
        # Raw command originals are preserved separately. Do not echo private
        # paths, device identity, input bytes or exception text to public logs.
        raise SystemExit("QA acquisition receipt rejected; original evidence remains quarantined.")
