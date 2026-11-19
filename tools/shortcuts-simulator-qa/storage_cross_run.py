"""Two external invocations of one reviewed synthetic Stored Content fixture.

This orchestrates existing simctl/Receiver behavior. It neither implements nor
simulates storage and never infers cache, TTL, deletion or arbitrary type support.
Raw plans, trigger URLs and command output stay outside the publication allowlist.
"""
from datetime import datetime, timezone
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import secrets
import shutil
import signal
import subprocess
import time
import urllib.parse
import uuid

FIXTURE = "qa-storage-text-cross-run-v2"
BUNDLE = "org.cherrilang.qa.Receiver"
PREFIX = "CHERRI_QA_STORAGE_CROSS_RUN_RESULT:"
KEY_PREFIX = "cherri.qa.p15.storage.cross-run."
PLAN_FILE = "storage-cross-run-plan.json"
DIAGNOSTIC_FILE = "storage-cross-run-diagnostic.json"


def digest(data):
    return hashlib.sha256(data).hexdigest()


def now():
    return datetime.now(timezone.utc).isoformat(timespec="milliseconds").replace("+00:00", "Z")


def write_json(path, data):
    Path(path).write_text(json.dumps(data, indent=2, ensure_ascii=True) + "\n", encoding="utf-8")


def read_json(path, default=None):
    try:
        return json.loads(Path(path).read_text(encoding="utf-8"))
    except (OSError, ValueError):
        return default


def marker(phase):
    if phase not in ("WRITE", "READ"):
        raise ValueError("UNREVIEWED_PHASE")
    return PREFIX + phase + ":TYPE_OK:VALUE_OK"


def manifest_entry(root, sha):
    manifest = read_json(Path(root) / "compiler-manifest.json")
    if (not isinstance(manifest, dict) or manifest.get("syntheticOnly") is not True
            or manifest.get("compilerCommitSha") != sha or not re.fullmatch(r"[0-9a-f]{40}", sha)):
        raise ValueError("SOURCE_MANIFEST_MISMATCH")
    entries = [f for f in manifest.get("fixtures", []) if isinstance(f, dict) and f.get("fixtureId") == FIXTURE]
    if len(entries) != 1:
        raise ValueError("FIXTURE_MANIFEST_MISMATCH")
    entry = entries[0]
    if (entry.get("fileName") != FIXTURE + ".cherri" or entry.get("buildProfile") != "debug"
            or entry.get("runtimeTraceExpectation") != "REQUIRED_NDJSON"
            or any(not re.fullmatch(r"[0-9a-f]{64}", entry.get(k, ""))
                   for k in ("sourceSha256", "unsignedPlistSha256"))):
        raise ValueError("DEBUG_SOURCE_BINDING_REQUIRED")
    return entry


def validate_plan(plan, root):
    if not isinstance(plan, dict) or set(plan) != {
            "schemaVersion", "fixtureId", "compilerCommitSha", "writerNonce", "readerNonce",
            "runToken", "sameSimulatorSha256"}:
        raise ValueError("INVALID_PLAN_FIELDS")
    if plan["schemaVersion"] != 1 or plan["fixtureId"] != FIXTURE:
        raise ValueError("UNREVIEWED_PLAN")
    for name in ("writerNonce", "readerNonce"):
        if not re.fullmatch(re.escape(FIXTURE) + r"-[A-Za-z0-9-]{1,90}-[0-9a-f]{32}", plan[name]):
            raise ValueError("INVALID_PHASE_NONCE")
    if plan["writerNonce"] == plan["readerNonce"]:
        raise ValueError("REUSED_PHASE_NONCE")
    if not re.fullmatch(r"[0-9a-f]{32}", plan["runToken"]):
        raise ValueError("INVALID_OWNED_KEY_TOKEN")
    if not re.fullmatch(r"[0-9a-f]{64}", plan["sameSimulatorSha256"]):
        raise ValueError("SIMULATOR_BINDING_REQUIRED")
    manifest_entry(root, plan["compilerCommitSha"])
    return plan


def load_plan(root):
    return validate_plan(read_json(Path(root) / "runtime" / PLAN_FILE), root)


def prepare(root, writer_nonce, sha, udid):
    root = Path(root)
    simulator = str(uuid.UUID(udid))
    plan = {
        "schemaVersion": 1, "fixtureId": FIXTURE, "compilerCommitSha": sha,
        "writerNonce": writer_nonce, "readerNonce": FIXTURE + "-reader-" + secrets.token_hex(16),
        "runToken": secrets.token_hex(16), "sameSimulatorSha256": digest(simulator.encode()),
    }
    validate_plan(plan, root)
    path = root / "runtime" / PLAN_FILE
    if path.exists():
        raise ValueError("RUN_PLAN_ALREADY_EXISTS")
    write_json(path, plan)
    return plan


def input_text(plan, phase):
    marker(phase)
    return json.dumps({"phase": phase, "runToken": plan["runToken"]}, separators=(",", ":"))


def run_url(plan, phase):
    q = urllib.parse.quote
    nonce = plan["writerNonce" if phase == "WRITE" else "readerNonce"]
    callbacks = {}
    for status in ("success", "cancel", "error"):
        callbacks[status] = (f"cherriqa://{status}?runId={q(nonce)}&fixtureId={q(FIXTURE)}"
                             f"&shortcutName={q(FIXTURE)}&source=shortcuts-x-callback")
    return (f"shortcuts://x-callback-url/run-shortcut?name={q(FIXTURE)}&input=text"
            f"&text={q(input_text(plan, phase), safe='')}&x-source=CherriSimulatorQAReceiver"
            + "".join("&x-" + k + "=" + q(v, safe="") for k, v in callbacks.items()))


def phase_observation(root, plan, phase):
    """Bind the existing strict oracle to its actual callback and trace bytes."""
    root = Path(root)
    lower = "writer" if phase == "WRITE" else "reader"
    nonce = plan[lower + "Nonce"]
    oracle = read_json(root / "runtime" / ("storage-" + lower + "-oracle.json"), {})
    events = read_json(root / "runtime/callback-events.json", [])
    if not isinstance(events, list) or not all(isinstance(e, dict) for e in events):
        events = []
    matched = [e for e in events if e.get("receiverSelfTest") is False and e.get("runId") == nonce]
    ev = matched[0] if len(matched) == 1 else {}
    source = manifest_entry(root, plan["compilerCommitSha"])
    trace_path = root / "runtime" / ("storage-writer-trace.jsonl" if phase == "WRITE" else "runtime-trace.jsonl")
    trace = trace_path.read_bytes() if trace_path.is_file() else b""
    trace_bound = False
    try:
        payload = ev.get("tracePayload")
        if isinstance(payload, str):
            records = [json.loads(line) for line in payload.splitlines()
                       if line.strip() and line.strip() != "===CHERRI_QA_TRACE_NDJSON_V14==="]
            expected_trace = "".join(json.dumps(record, ensure_ascii=False) + "\n" for record in records).encode()
            # The canonical oracle writes platform text newlines; retain the
            # original byte hash while comparing its exact JSON records.
            trace_bound = bool(records) and trace.replace(b"\r\n", b"\n") == expected_trace
    except (ValueError, TypeError):
        pass
    timestamp = ev.get("timestampISO8601", "")
    if not isinstance(timestamp, str) or not re.fullmatch(
            r"\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d+)?Z", timestamp):
        timestamp = ""
    status = ev.get("status") if ev.get("status") in ("SUCCESS", "ERROR", "CANCELLED") else "UNKNOWN"
    exact = (len(matched) == 1 and ev.get("eventSource") == "shortcuts-x-callback"
             and ev.get("fixtureId") == FIXTURE and ev.get("expectedShortcutName") == FIXTURE
             and status == "SUCCESS" and isinstance(ev.get("resultSummary"), str)
             and ev["resultSummary"].strip() == marker(phase))
    oracle_bound = (isinstance(oracle, dict) and oracle.get("runGate") == "PASS"
                    and oracle.get("traceGate") == "PASS" and oracle.get("targetFixtureId") == FIXTURE
                    and oracle.get("compilerCommitSha") == plan["compilerCommitSha"]
                    and oracle.get("sourceSha256") == source["sourceSha256"]
                    and oracle.get("unsignedPlistSha256") == source["unsignedPlistSha256"]
                    and oracle.get("buildProfile") == "debug"
                    and oracle.get("traceExpectation") == "REQUIRED_NDJSON"
                    and oracle.get("releaseBuildVerified") is False)
    return {
        "runId": nonce, "phase": phase, "matchingCallbacks": len(matched),
        "observedStatus": status, "exactExpectedSummary": exact, "timestampISO8601": timestamp,
        "strictOracleVerified": oracle_bound, "traceMatchesActualCallback": trace_bound,
        "traceSha256": digest(trace) if trace else "",
        "verified": exact and oracle_bound and trace_bound and bool(timestamp),
    }


def writer_ready(root, plan):
    observed = phase_observation(root, plan, "WRITE")
    if not observed["verified"]:
        raise ValueError("WRITER_CALLBACK_AND_TRACE_NOT_VERIFIED")
    return observed


def run_bounded_native(command, *, timeout, capture_output=True, env=None):
    process = subprocess.Popen(command, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                               env=env, start_new_session=os.name != "nt")
    try:
        stdout, stderr = process.communicate(timeout=timeout)
        return subprocess.CompletedProcess(command, process.returncode, stdout, stderr)
    except subprocess.TimeoutExpired:
        try:
            if os.name == "posix":
                os.killpg(process.pid, signal.SIGKILL)
            else:
                process.kill()
        except ProcessLookupError:
            pass
        process.communicate()
        raise


def reader_confirmation_config():
    names = ("CHERRI_QA_READER_CONFIRM_PROJECT", "CHERRI_QA_READER_CONFIRM_DERIVED_DATA",
             "CHERRI_QA_READER_CONFIRM_RESULT")
    values = [os.environ.get(name, "") for name in names]
    if not any(values):
        return None
    if not all(values) or not Path(values[0]).is_dir() or not Path(values[1]).is_dir():
        raise ValueError("READER_CONSENT_CONFIGURATION_INVALID")
    return values


def confirm_reader_dialog(root, udid, plan, config, runner):
    project, derived, result = config
    output = root / "editor"
    output.mkdir(exist_ok=True)
    sentinel = output / "confirm-dialog-mode.txt"
    command = ["xcodebuild", "test-without-building", "-project", project,
               "-scheme", "CherriSimulatorQAReceiver", "-derivedDataPath", derived,
               "-destination", "platform=iOS Simulator,id=" + udid,
               "-only-testing:CherriSimulatorQAUITests/CherriSimulatorQAUITests/test04ConfirmPendingRuntimeDialog",
               "-resultBundlePath", result, "CODE_SIGNING_ALLOWED=NO"]
    # Reuse the writer's UI consent handler without issuing another READ request.
    # A green XCTest result never replaces the subsequent callback/trace oracle.
    record = {"phase": "READ", "startedAt": now(), "finishedAt": "", "exitCode": None}
    sentinel.write_text("1", encoding="utf-8")
    try:
        result = runner(command, capture_output=True, timeout=120,
                        env={**os.environ, "TEST_RUNNER_CHERRI_QA_DIALOG_CONFIRM_ONLY": "1",
                             "TEST_RUNNER_CHERRI_QA_EXEC_RUN_NONCE": plan["readerNonce"],
                             "TEST_RUNNER_CHERRI_QA_SHARED_OUTPUT_DIR": str(output)})
        record["exitCode"] = result.returncode
        log = (result.stdout or b"") + (result.stderr or b"")
        (root / "runtime/storage-reader-consent.log").write_bytes(log)
    except subprocess.TimeoutExpired:
        record["exitCode"] = 124
    except OSError:
        record["exitCode"] = 127
    finally:
        sentinel.unlink(missing_ok=True)
        record["finishedAt"] = now()
        write_json(root / "runtime/storage-reader-consent.json", record)


def dispatch_reader(root, udid, receiver_data, runner=run_bounded_native,
                    monotonic=time.monotonic, sleeper=time.sleep):
    root = Path(root)
    plan = load_plan(root)
    if digest(str(uuid.UUID(udid)).encode()) != plan["sameSimulatorSha256"]:
        raise ValueError("SIMULATOR_CHANGED_BETWEEN_PHASES")
    writer = writer_ready(root, plan)
    confirmation = reader_confirmation_config()
    dispatch_path = root / "runtime/storage-reader-dispatch.json"
    dispatch = {
        "schemaVersion": 1, "sameSimulatorSha256": plan["sameSimulatorSha256"],
        "writerVerifiedBeforeReaderLaunch": True, "writerCallbackTimestamp": writer["timestampISO8601"],
        "readerLaunchStartedAt": now(), "readerLaunchFinishedAt": "",
        "readerLaunched": False, "readerCallbackObserved": False, "result": "NOT_LAUNCHED",
    }
    # Exclusive creation makes accidental re-entry fail before any native launch.
    with dispatch_path.open("x", encoding="utf-8") as handle:
        json.dump(dispatch, handle)
    try:
        runner(["xcrun", "simctl", "terminate", udid, BUNDLE], capture_output=True, timeout=15)
        result = runner(["xcrun", "simctl", "launch", udid, BUNDLE],
                        env={**os.environ, "SIMCTL_CHILD_CHERRI_TRIGGER_SHORTCUT_URL": run_url(plan, "READ")},
                        capture_output=True, timeout=25)
        dispatch["readerLaunchFinishedAt"] = now()
        if result.returncode != 0:
            dispatch["result"] = "RECEIVER_LAUNCH_FAILED"
            return False
        dispatch["readerLaunched"] = True
        dispatch["result"] = "CALLBACK_TIMEOUT"
        source = Path(receiver_data) / "Documents/callback-events.json"
        def wait_for_callback(seconds):
            deadline = monotonic() + seconds
            while monotonic() < deadline:
                if source.is_file() and source.stat().st_size <= 2 * 1024 * 1024:
                    events = read_json(source, [])
                    if isinstance(events, list) and all(isinstance(e, dict) for e in events):
                        shutil.copyfile(source, root / "runtime/callback-events.json")
                        if any(e.get("receiverSelfTest") is False and e.get("runId") == plan["readerNonce"] for e in events):
                            dispatch["readerCallbackObserved"] = True
                            dispatch["result"] = "CALLBACK_OBSERVED"
                            return True
                sleeper(1)
            return False
        if wait_for_callback(10 if confirmation else 45):
            return True
        if confirmation:
            confirm_reader_dialog(root, udid, plan, confirmation, runner)
            # XCTest may reinstall its host app; use the actual current container.
            refreshed = runner(["xcrun", "simctl", "get_app_container", udid, BUNDLE, "data"],
                               capture_output=True, timeout=15)
            if refreshed.returncode == 0:
                candidate = Path(refreshed.stdout.decode("utf-8", errors="replace").strip())
                if candidate.is_absolute() and candidate.is_dir():
                    source = candidate / "Documents/callback-events.json"
            return wait_for_callback(45)
        return False
    except subprocess.TimeoutExpired:
        dispatch["result"] = "RECEIVER_COMMAND_TIMEOUT"
        return False
    except OSError:
        dispatch["result"] = "RECEIVER_COMMAND_UNAVAILABLE"
        return False
    finally:
        write_json(dispatch_path, dispatch)


def build_report(root):
    root = Path(root)
    plan = load_plan(root)
    source = manifest_entry(root, plan["compilerCommitSha"])
    writer, reader = (phase_observation(root, plan, phase) for phase in ("WRITE", "READ"))
    raw_dispatch = read_json(root / "runtime/storage-reader-dispatch.json", {})
    dispatch_keys = {
        "schemaVersion", "sameSimulatorSha256", "writerVerifiedBeforeReaderLaunch",
        "writerCallbackTimestamp", "readerLaunchStartedAt", "readerLaunchFinishedAt",
        "readerLaunched", "readerCallbackObserved", "result",
    }
    known_results = {"NOT_LAUNCHED", "RECEIVER_LAUNCH_FAILED", "CALLBACK_TIMEOUT", "CALLBACK_OBSERVED",
                     "RECEIVER_COMMAND_TIMEOUT", "RECEIVER_COMMAND_UNAVAILABLE"}
    dispatch_valid = (isinstance(raw_dispatch, dict) and set(raw_dispatch) == dispatch_keys
                      and raw_dispatch.get("schemaVersion") == 1
                      and raw_dispatch.get("result") in known_results
                      and raw_dispatch.get("sameSimulatorSha256") == plan["sameSimulatorSha256"]
                      and all(type(raw_dispatch.get(k)) is bool for k in (
                          "writerVerifiedBeforeReaderLaunch", "readerLaunched", "readerCallbackObserved")))
    order = False
    if dispatch_valid:
        try:
            stamp = lambda value: datetime.fromisoformat(value.replace("Z", "+00:00"))
            order = (stamp(writer["timestampISO8601"]) <= stamp(raw_dispatch["readerLaunchStartedAt"])
                     <= stamp(reader["timestampISO8601"])
                     and stamp(raw_dispatch["readerLaunchStartedAt"]) <= stamp(raw_dispatch["readerLaunchFinishedAt"]))
        except (ValueError, TypeError, AttributeError):
            pass
    verified = (writer["verified"] and reader["verified"] and dispatch_valid and order
                and raw_dispatch["writerVerifiedBeforeReaderLaunch"] is True
                and raw_dispatch["readerLaunched"] is True and raw_dispatch["readerCallbackObserved"] is True
                and raw_dispatch["result"] == "CALLBACK_OBSERVED"
                and raw_dispatch["writerCallbackTimestamp"] == writer["timestampISO8601"])
    # No raw URL, input object, storage value, key, simulator ID or command output.
    return {
        "schemaVersion": 1, "fixtureId": FIXTURE, "compilerCommitSha": plan["compilerCommitSha"],
        "sourceSha256": source["sourceSha256"], "unsignedPlistSha256": source["unsignedPlistSha256"],
        "scenario": "TWO_EXTERNAL_INVOCATIONS_SAME_SIMULATOR",
        "sameSimulatorSha256": plan["sameSimulatorSha256"],
        "runScopedKeySha256": digest((KEY_PREFIX + plan["runToken"]).encode()),
        "globalFlag": True, "writer": writer, "reader": reader,
        "dispatchResult": raw_dispatch["result"] if dispatch_valid else "NOT_OBSERVED",
        "readerDispatchedAfterVerifiedWriter": bool(order and dispatch_valid
              and raw_dispatch.get("writerVerifiedBeforeReaderLaunch") is True),
        "crossRunTextVerified": bool(verified),
        "cleanup": "READ_BRANCH_REQUESTED_DELETE_ONLY" if reader["verified"] else "NOT_OBSERVED",
        "cacheImplemented": False, "typedArchiveProven": False, "ttlProven": False,
        "crossShortcutIsolationProven": False, "physicalDeviceProven": False,
    }


def validate_public_diagnostic(data, root):
    if data != build_report(root):
        raise ValueError("CROSS_RUN_DIAGNOSTIC_NOT_BOUND_TO_ACTUAL_PHASE_EVIDENCE")
    assertions = read_json(Path(root) / "runtime/output-assertions.json", {})
    if data["crossRunTextVerified"] and (
            assertions.get("targetFixtureId") != FIXTURE
            or assertions.get("expectedRunId") != data["reader"]["runId"]
            or assertions.get("expectedSemanticOutput") != marker("READ")
            or assertions.get("runtimeContractVerified") is not True
            or assertions.get("compilerCommitSha") != data["compilerCommitSha"]
            or assertions.get("compiledSourceSha256") != data["sourceSha256"]
            or assertions.get("unsignedPlistSha256") != data["unsignedPlistSha256"]):
        raise ValueError("CROSS_RUN_FINAL_ORACLE_MISMATCH")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("prepare", "input", "reader-nonce", "writer-ready", "dispatch-reader", "report"))
    parser.add_argument("--root", type=Path, required=True)
    parser.add_argument("--nonce")
    parser.add_argument("--commit")
    parser.add_argument("--udid")
    parser.add_argument("--receiver-data")
    parser.add_argument("--phase", choices=("WRITE", "READ"))
    args = parser.parse_args()
    if args.command == "prepare":
        prepare(args.root, args.nonce, args.commit, args.udid)
        return 0
    plan = load_plan(args.root)
    if args.command == "input":
        print(input_text(plan, args.phase))
    elif args.command == "reader-nonce":
        print(plan["readerNonce"])
    elif args.command == "writer-ready":
        writer_ready(args.root, plan)
    elif args.command == "dispatch-reader":
        return 0 if dispatch_reader(args.root, args.udid, args.receiver_data) else 1
    else:
        report = build_report(args.root)
        write_json(args.root / "runtime" / DIAGNOSTIC_FILE, report)
        print(json.dumps({"crossRunTextVerified": report["crossRunTextVerified"],
                          "dispatchResult": report["dispatchResult"]}))
        return 0 if report["crossRunTextVerified"] else 1
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (OSError, ValueError, TypeError, KeyError):
        # Keep raw input/URL/native errors out of public stdout.
        print("STORAGE_CROSS_RUN_CONTRACT_REJECTED")
        raise SystemExit(2)
