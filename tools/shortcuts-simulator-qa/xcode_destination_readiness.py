#!/usr/bin/env python3
"""Verify the owned Xcode destination before the unchanged test02 command.

The query and at most one non-reboot readiness refresh share the original
900-second test phase. This is infrastructure evidence, never an import or
Shortcut-runtime acceptance oracle. Raw destination names remain quarantined.
"""
import argparse
import hashlib
import json
import math
import os
from pathlib import Path
import re
import time

from simulator_session_recovery import integer, require, run_process
import xcode_destination_diagnostic as diagnostic

PROJECT = "tools/shortcuts-simulator-qa/XcodeProject/CherriSimulatorQA.xcodeproj"
SCHEME = "CherriSimulatorQAReceiver"
TEST = "CherriSimulatorQAUITests/CherriSimulatorQAUITests/test02_ShortcutsImportLibraryAndActionEditorCoverage"
PHASE_SECONDS = 900
GATE_SECONDS = 90
MAX_OUTPUT = 256 * 1024
MAX_OBSERVED_MS = 86_400_000
STAGES = {"destinations": 30, "open": 15, "bootstatus": 60, "test": PHASE_SECONDS}
RESULTS = {"PASS", "EXIT_NONZERO", "TIMEOUT", "LAUNCH_FAILED", "DEADLINE_EXHAUSTED"}
PARSE_RESULTS = {"AVAILABLE", "MISSING", "INELIGIBLE", "AMBIGUOUS", "UNPARSEABLE"}


def sha(data):
    return hashlib.sha256(data).hexdigest()


def destination_observation(data, udid):
    return _destination_observation(data, sha(udid.upper().encode()))


def _destination_observation(data, device_hash):
    """Parse only exact scheme sections and exact ID fields, not names.

    The two section spellings are Xcode's Available/Ineligible form and the
    Compatible/Incompatible form retained in the actual failed Xcode 27 run.
    A placeholder, another device, an error-bearing row or a matching name
    cannot establish that this owned simulator is an eligible destination.
    """
    result = {"outputBytes": len(data), "outputSha256": sha(data),
              "availableMatches": 0, "ineligibleMatches": 0,
              "targetDeviceIdSha256": device_hash, "status": "UNPARSEABLE"}
    if len(data) > MAX_OUTPUT:
        return result
    try:
        text = data.decode("utf-8")
    except UnicodeDecodeError:
        return result
    if "\x00" in text:
        return result
    available_headers = {
        f'Available destinations for the "{SCHEME}" scheme:',
        f'Destinations compatible with the "{SCHEME}" scheme:',
    }
    ineligible_headers = {
        f'Ineligible destinations for the "{SCHEME}" scheme:',
        f'Destinations incompatible with the "{SCHEME}" scheme:',
    }
    section, headers, invalid = None, 0, False
    for raw in text.splitlines():
        line = raw.strip()
        if line in available_headers:
            section, headers = "available", headers + 1
            continue
        if line in ineligible_headers:
            section = "ineligible"
            continue
        if re.search(r'(?:destinations|Destinations).*scheme:', line):
            section = None
            continue
        if not section or not line.startswith("{"):
            continue
        if not line.endswith("}") or len(line) > 4096:
            invalid = True
            continue
        ids = re.findall(r'(?:^|,)\s*id:\s*([^,{}]+)', line[1:-1])
        platforms = re.findall(r'(?:^|,)\s*platform:\s*([^,{}]+)', line[1:-1])
        if len(ids) != 1 or len(platforms) != 1:
            invalid = True
            continue
        device = ids[0].strip().upper()
        if not re.fullmatch(r"[0-9A-F]{8}(?:-[0-9A-F]{4}){3}-[0-9A-F]{12}", device) or sha(device.encode()) != device_hash:
            continue
        error = re.search(r'(?:^|,)\s*error:', line[1:-1]) is not None
        if section == "available" and platforms[0].strip() == "iOS Simulator" and not error:
            result["availableMatches"] += 1
        else:
            result["ineligibleMatches"] += 1
    if invalid or headers != 1:
        return result
    available, ineligible = result["availableMatches"], result["ineligibleMatches"]
    result["status"] = ("AMBIGUOUS" if available > 1 or (available and ineligible) else
                        "AVAILABLE" if available == 1 else
                        "INELIGIBLE" if ineligible else "MISSING")
    return result


class DestinationTestPhase:
    def __init__(self, repo, scratch, artifacts, udid, identity,
                 runner=run_process, now=time.monotonic):
        self.repo, self.scratch, self.artifacts = map(Path, (repo, scratch, artifacts))
        require(isinstance(identity, dict) and set(identity) == {"compilerCommitSha", "runId", "fixtureId"}, "IDENTITY_FIELDS")
        for key, pattern in (("compilerCommitSha", r"[0-9a-f]{40}"),
                             ("runId", r"[A-Za-z0-9_-]{1,64}"),
                             ("fixtureId", r"[A-Za-z0-9_-]{1,96}")):
            require(isinstance(identity[key], str) and re.fullmatch(pattern, identity[key]), "IDENTITY_INVALID")
        require(re.fullmatch(r"[0-9A-Fa-f]{8}(?:-[0-9A-Fa-f]{4}){3}-[0-9A-Fa-f]{12}", udid), "DEVICE_ID_INVALID")
        for path in (self.repo, self.scratch, self.artifacts,
                     self.artifacts / "builds", self.artifacts / "runtime"):
            require(path.is_dir() and not path.is_symlink(), "OWNED_DIRECTORY_REQUIRED")
        require((self.repo / PROJECT).is_dir() and not (self.repo / PROJECT).is_symlink(), "PROJECT_REQUIRED")
        self.udid, self.runner, self.now = udid, runner, now
        self.started = now()
        self.commands, self.queries = [], []
        self.launch_receipt = None
        self.value = {"schemaVersion": "1.1", **identity,
                      "deviceIdSha256": sha(udid.upper().encode()),
                      "project": PROJECT, "scheme": SCHEME, "testIdentifier": TEST,
                      "phaseBudgetMs": PHASE_SECONDS * 1000,
                      "gateBudgetMs": GATE_SECONDS * 1000,
                      "status": "BLOCKED_DESTINATION", "elapsedMs": 0, "gateElapsedMs": 0,
                      "refreshAttempted": False, "testStarted": False,
                      "commands": self.commands, "queries": self.queries,
                      "formalAcceptanceGranted": False}

    def elapsed(self):
        value = self.now() - self.started
        return value if math.isfinite(value) and value >= 0 else float("inf")

    def command(self, stage, args, *, output=None, gate=True):
        phase_limit = GATE_SECONDS if gate else PHASE_SECONDS
        before = self.elapsed()
        allowance = max(0, min(STAGES[stage], phase_limit - before))
        if stage == "open":
            result, self.launch_receipt = diagnostic.launch(self.artifacts, self.udid, allowance,
                                                           runner=self.runner, now=self.now)
        else:
            result = (self.runner(args, allowance, output=output) if allowance else
                      {"result": "DEADLINE_EXHAUSTED", "exitCode": None, "elapsedMs": 0})
        after = self.elapsed()
        if result["result"] == "PASS" and (after >= phase_limit or after - before > allowance):
            result = {**result, "result": "DEADLINE_EXHAUSTED"}
        self.commands.append({"stage": stage, "allowanceMs": round(allowance * 1000), **result})
        return result

    def query(self):
        relative = "builds/xcode-destination-query-" + str(len(self.queries) + 1) + ".log"
        path = self.artifacts / relative
        require(not path.is_symlink(), "QUERY_LOG_SYMLINK")
        with path.open("xb") as stream:
            result = self.command("destinations", ["xcodebuild", "-showdestinations",
                "-project", str(self.repo / PROJECT), "-scheme", SCHEME], output=stream)
        # Preserve the one original in the existing private/quarantined area.
        # Publication binds its bytes but never copies this file to public output.
        # Oversized output remains in place and is hashed without loading it.
        if path.stat().st_size <= MAX_OUTPUT:
            observed = destination_observation(path.read_bytes(), self.udid)
        else:
            digest_state = hashlib.sha256()
            with path.open("rb") as stream:
                for block in iter(lambda: stream.read(65536), b""):
                    digest_state.update(block)
            digest = digest_state.hexdigest()
            observed = {"outputBytes": path.stat().st_size, "outputSha256": digest,
                        "availableMatches": 0, "ineligibleMatches": 0,
                        "targetDeviceIdSha256": self.value["deviceIdSha256"], "status": "UNPARSEABLE"}
        self.queries.append({"commandIndex": len(self.commands) - 1, "rawRelativePath": relative, **observed})
        return result["result"] == "PASS" and observed["status"] == "AVAILABLE"

    def run(self):
        marker = self.artifacts / "runtime" / "xcode-destination-attempt.json"
        require(not marker.exists() and not marker.is_symlink(), "DESTINATION_ATTEMPT_ALREADY_RESERVED")
        ready = self.query()
        if not ready and self.commands[-1]["result"] == "PASS" and self.queries[-1]["status"] == "MISSING":
            # No reboot, shutdown, alternate device, import or callback.
            # The same pre-issued import request remains untouched.
            self.value["refreshAttempted"] = True
            for stage, args in (
                # The launch helper binds Simulator.app to the active Xcode.
                ("open", None),
                ("bootstatus", ["xcrun", "simctl", "bootstatus", self.udid, "-b"]),
            ):
                if self.command(stage, args)["result"] != "PASS":
                    break
            else:
                ready = self.query()
        gate_elapsed = self.elapsed()
        self.value["gateElapsedMs"] = min(MAX_OBSERVED_MS, round(gate_elapsed * 1000)) if math.isfinite(gate_elapsed) else MAX_OBSERVED_MS
        code = 70
        if ready and gate_elapsed < GATE_SECONDS:
            log = self.artifacts / "builds" / "xcodebuild-test.log"
            require(not log.is_symlink(), "TEST_LOG_SYMLINK")
            reservation = {key: self.value[key] for key in ("compilerCommitSha", "runId", "fixtureId", "deviceIdSha256", "project", "scheme", "testIdentifier")}
            reservation.update(schemaVersion="1.0", state="ATTEMPT_RESERVED")
            # Reserve the actual invocation durably before any Xcode test can
            # start. Validator/output-write failures cannot erase this marker.
            with marker.open("xb") as stream:
                stream.write((json.dumps(reservation, sort_keys=True) + "\n").encode())
                stream.flush()
                os.fsync(stream.fileno())
            gate_elapsed = self.elapsed()
            self.value["gateElapsedMs"] = min(MAX_OBSERVED_MS, round(gate_elapsed * 1000)) if math.isfinite(gate_elapsed) else MAX_OBSERVED_MS
            if gate_elapsed < GATE_SECONDS:
                self.value["testStarted"] = True
                self.value["status"] = "DESTINATION_READY_TEST_ATTEMPTED"
                with log.open("wb") as stream:
                    result = self.command("test", ["xcodebuild", "test-without-building",
                        "-project", str(self.repo / PROJECT), "-scheme", SCHEME,
                        "-derivedDataPath", str(self.scratch / "DerivedData"),
                        "-destination", "platform=iOS Simulator,id=" + self.udid,
                        "-only-testing:" + TEST,
                        "-resultBundlePath", str(self.scratch / "CherriSimulatorQA.xcresult"),
                        "CODE_SIGNING_ALLOWED=NO"], output=stream, gate=False)
                code = result["exitCode"] if result["result"] in {"PASS", "EXIT_NONZERO"} else 124
        elapsed = self.elapsed()
        self.value["elapsedMs"] = min(MAX_OBSERVED_MS, round(elapsed * 1000)) if math.isfinite(elapsed) else MAX_OBSERVED_MS
        validate_receipt(self.value)
        value = diagnostic.document(self.artifacts, self.value, self.launch_receipt)
        diagnostic.validate(value, self.artifacts, self.value)
        with (self.artifacts / "runtime/xcode-destination-command-diagnostic.json").open("xb") as stream:
            stream.write(diagnostic.encoded(value))
        return code, self.value


def validate_receipt(value, source_sha=None, run_id=None, fixture_id=None, artifact_root=None):
    fields = {"schemaVersion", "compilerCommitSha", "runId", "fixtureId", "deviceIdSha256",
              "project", "scheme", "testIdentifier", "phaseBudgetMs", "gateBudgetMs", "status",
              "elapsedMs", "gateElapsedMs", "refreshAttempted", "testStarted", "commands", "queries", "formalAcceptanceGranted"}
    require(isinstance(value, dict) and set(value) == fields, "DESTINATION_RECEIPT_FIELDS")
    require(value["schemaVersion"] in ("1.0", "1.1") and value["formalAcceptanceGranted"] is False, "DESTINATION_RECEIPT_SCOPE")
    require(isinstance(value["compilerCommitSha"], str) and re.fullmatch(r"[0-9a-f]{40}", value["compilerCommitSha"]), "DESTINATION_SOURCE")
    require(isinstance(value["deviceIdSha256"], str) and re.fullmatch(r"[0-9a-f]{64}", value["deviceIdSha256"]), "DESTINATION_DEVICE")
    for key, limit, expected in (("runId", 64, run_id), ("fixtureId", 96, fixture_id)):
        require(isinstance(value[key], str) and re.fullmatch(r"[A-Za-z0-9_-]{1," + str(limit) + r"}", value[key]), "DESTINATION_ID")
        require(expected is None or value[key] == expected, "DESTINATION_IDENTITY")
    require(source_sha is None or value["compilerCommitSha"] == source_sha, "DESTINATION_IDENTITY")
    require(value["project"] == PROJECT and value["scheme"] == SCHEME and value["testIdentifier"] == TEST, "DESTINATION_TARGET")
    require(value["phaseBudgetMs"] == 900000 and value["gateBudgetMs"] == 90000 and integer(value["elapsedMs"], 0, MAX_OBSERVED_MS), "DESTINATION_BUDGET")
    require(integer(value["gateElapsedMs"], 0, value["elapsedMs"] + 1), "DESTINATION_GATE_TIME")
    require(type(value["refreshAttempted"]) is bool and type(value["testStarted"]) is bool, "DESTINATION_FLAGS")
    commands, queries = value["commands"], value["queries"]
    require(isinstance(commands, list) and 1 <= len(commands) <= 5 and isinstance(queries, list) and 1 <= len(queries) <= 2, "DESTINATION_COUNTS")
    for command in commands:
        require(isinstance(command, dict) and set(command) == {"stage", "allowanceMs", "result", "exitCode", "elapsedMs"}, "DESTINATION_COMMAND_FIELDS")
        stage = command["stage"]
        require(stage in STAGES and command["result"] in RESULTS and integer(command["allowanceMs"], 0, STAGES[stage] * 1000) and integer(command["elapsedMs"], 0, MAX_OBSERVED_MS), "DESTINATION_COMMAND")
        require(command["exitCode"] is None or integer(command["exitCode"], -255, 255), "DESTINATION_EXIT")
        require(command["result"] != "PASS" or (command["exitCode"] == 0 and command["elapsedMs"] <= command["allowanceMs"] + 1), "DESTINATION_LATE_PASS")
        require(command["result"] != "EXIT_NONZERO" or command["exitCode"] not in (0, None), "DESTINATION_FALSE_FAILURE")
        require(command["result"] not in {"TIMEOUT", "LAUNCH_FAILED"} or command["exitCode"] is None, "DESTINATION_FALSE_TIMEOUT")
    require(sum(x["elapsedMs"] for x in commands) <= value["elapsedMs"] + len(commands), "DESTINATION_TIME_SUM")
    indices = [n for n, x in enumerate(commands) if x["stage"] == "destinations"]
    require(len(indices) == len(queries), "DESTINATION_QUERY_COUNT")
    for number, (index, query) in enumerate(zip(indices, queries), 1):
        require(isinstance(query, dict) and set(query) == {"commandIndex", "rawRelativePath", "outputBytes", "outputSha256", "availableMatches", "ineligibleMatches", "targetDeviceIdSha256", "status"}, "DESTINATION_QUERY_FIELDS")
        require(query["rawRelativePath"] == "builds/xcode-destination-query-" + str(number) + ".log", "DESTINATION_QUERY_PATH")
        require(type(query["commandIndex"]) is int and query["commandIndex"] == index and integer(query["outputBytes"], 0, 100000000) and isinstance(query["outputSha256"], str) and re.fullmatch(r"[0-9a-f]{64}", query["outputSha256"]), "DESTINATION_QUERY_OUTPUT")
        require(query["targetDeviceIdSha256"] == value["deviceIdSha256"] and query["status"] in PARSE_RESULTS and integer(query["availableMatches"], 0, 1000) and integer(query["ineligibleMatches"], 0, 1000), "DESTINATION_QUERY_VALUES")
        if query["status"] == "AVAILABLE":
            require(query["outputBytes"] <= MAX_OUTPUT and query["availableMatches"] == 1 and query["ineligibleMatches"] == 0, "DESTINATION_FALSE_AVAILABLE")
        if query["status"] == "MISSING":
            require(query["outputBytes"] <= MAX_OUTPUT and query["availableMatches"] == 0 and query["ineligibleMatches"] == 0, "DESTINATION_FALSE_MISSING")
        if query["status"] == "INELIGIBLE":
            require(query["outputBytes"] <= MAX_OUTPUT and query["availableMatches"] == 0 and query["ineligibleMatches"] >= 1, "DESTINATION_FALSE_INELIGIBLE")
        if query["status"] == "AMBIGUOUS":
            require(query["outputBytes"] <= MAX_OUTPUT and (query["availableMatches"] > 1 or (query["availableMatches"] >= 1 and query["ineligibleMatches"] >= 1)), "DESTINATION_FALSE_AMBIGUOUS")
        if artifact_root is not None:
            root = Path(artifact_root)
            raw = root / query["rawRelativePath"]
            require(root.is_dir() and not root.is_symlink() and raw.is_file() and not raw.is_symlink() and not raw.parent.is_symlink(), "DESTINATION_ORIGINAL_PATH")
            require(raw.resolve().is_relative_to(root.resolve()) and raw.stat().st_size == query["outputBytes"], "DESTINATION_ORIGINAL_SIZE")
            digest = hashlib.sha256()
            with raw.open("rb") as stream:
                for block in iter(lambda: stream.read(65536), b""):
                    digest.update(block)
            require(digest.hexdigest() == query["outputSha256"], "DESTINATION_ORIGINAL_HASH")
            if raw.stat().st_size <= MAX_OUTPUT:
                observed = _destination_observation(raw.read_bytes(), value["deviceIdSha256"])
                require(all(query[key] == field for key, field in observed.items()), "DESTINATION_ORIGINAL_OBSERVATION")
            else:
                require(query["status"] == "UNPARSEABLE" and query["availableMatches"] == 0 and query["ineligibleMatches"] == 0, "DESTINATION_ORIGINAL_OVERSIZE")
    stages = [x["stage"] for x in commands]
    expected = ["destinations", "open", "bootstatus", "destinations"] if value["refreshAttempted"] else ["destinations"]
    if value["refreshAttempted"]:
        require(len(commands) >= 2 and queries[0]["status"] == "MISSING" and commands[0]["result"] == "PASS", "DESTINATION_REFRESH_REASON")
    gate_commands = commands[:-1] if value["testStarted"] else commands
    require([x["stage"] for x in gate_commands] == expected[:len(gate_commands)], "DESTINATION_COMMAND_ORDER")
    require(sum(x["elapsedMs"] for x in gate_commands) <= value["gateElapsedMs"] + len(gate_commands), "DESTINATION_GATE_TIME_SUM")
    for n, command in enumerate(gate_commands):
        if command["result"] != "PASS":
            require(n == len(gate_commands) - 1 and not value["testStarted"], "DESTINATION_CONTINUED_AFTER_FAILURE")
    if value["testStarted"]:
        require(stages == expected + ["test"] and value["status"] == "DESTINATION_READY_TEST_ATTEMPTED" and queries[-1]["status"] == "AVAILABLE" and all(x["result"] == "PASS" for x in gate_commands), "DESTINATION_TEST_WITHOUT_READY")
        require(value["gateElapsedMs"] <= 90000 and commands[-1]["allowanceMs"] + value["gateElapsedMs"] <= 900001, "DESTINATION_BUDGET_EXTENSION")
        require(commands[-1]["elapsedMs"] <= value["elapsedMs"] - value["gateElapsedMs"] + 1, "DESTINATION_TEST_TIME_SUM")
        require(commands[-1]["result"] != "PASS" or value["elapsedMs"] <= 900001, "DESTINATION_PHASE_LATE_PASS")
    else:
        require(value["status"] == "BLOCKED_DESTINATION" and "test" not in stages, "DESTINATION_FALSE_BLOCK")
    if artifact_root is not None and value["schemaVersion"] == "1.1":
        path = Path(artifact_root) / "runtime/xcode-destination-command-diagnostic.json"
        require(path.is_file() and not path.is_symlink() and not path.parent.is_symlink()
                and path.stat().st_size <= MAX_OUTPUT, "DESTINATION_DIAGNOSTIC_REQUIRED")
        from qa_terminal_receipts import unique_json
        diagnostic.validate(unique_json(path.read_bytes()), artifact_root, value)
    return value


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("repo", "scratch", "artifacts", "udid", "source-sha", "run-id", "fixture-id"):
        parser.add_argument("--" + name, required=True)
    args = parser.parse_args()
    path = Path(args.artifacts) / "runtime" / "xcode-destination-readiness.json"
    require(not path.exists() and not path.is_symlink(), "DESTINATION_RECEIPT_ALREADY_EXISTS")
    marker = path.with_name("xcode-destination-attempt.json")
    require(not marker.exists() and not marker.is_symlink(), "DESTINATION_ATTEMPT_ALREADY_RESERVED")
    phase = DestinationTestPhase(args.repo, args.scratch, args.artifacts, args.udid,
        {"compilerCommitSha": args.source_sha, "runId": args.run_id, "fixtureId": args.fixture_id})
    lock = path.with_name(path.name + ".lock")
    descriptor = os.open(lock, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600)
    os.close(descriptor)
    try:
        code, receipt = phase.run()
        with path.open("xb") as stream:
            stream.write((json.dumps(receipt, sort_keys=True, indent=2) + "\n").encode())
    finally:
        lock.unlink(missing_ok=True)
    print(json.dumps({"xcodeDestinationStatus": receipt["status"], "testStarted": receipt["testStarted"], "elapsedMs": receipt["elapsedMs"]}, sort_keys=True))
    # Preserve Python's existing exit-code mapping for a signal-terminated
    # Xcode child, as well as ordinary failures and the timeout result.
    return code


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (OSError, ValueError, TypeError, KeyError):
        print("XCODE_DESTINATION_READINESS_FAILED")
        raise SystemExit(70)
