#!/usr/bin/env python3
"""Bounded simulator liveness probes; never an import/runtime acceptance oracle."""
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

MAX_BUDGET = 240
MIN_DISPLAY_BYTES = 85000
STAGES = {"getenv", "shutdown", "boot", "bootstatus", "open", "screenshot"}
STAGE_MAX_MS = {"getenv": 15000, "shutdown": 30000, "boot": 30000,
                "bootstatus": 120000, "open": 15000, "screenshot": 25000}
RESULTS = {"PASS", "EXIT_NONZERO", "TIMEOUT", "LAUNCH_FAILED", "DEADLINE_EXHAUSTED"}
MAX_OBSERVED_MS = 86_400_000


def require(condition, message):
    if not condition:
        raise ValueError(message)


def integer(value, low, high):
    return type(value) is int and low <= value <= high


def receipt_json(text):
    def unique_object(pairs):
        value = {}
        for key, item in pairs:
            require(key not in value, "DUPLICATE_RECEIPT_KEY")
            value[key] = item
        return value
    return json.loads(text, object_pairs_hook=unique_object)


def run_process(command, timeout):
    """Same process-group kill strategy as the existing bounded Xcode runner."""
    started = time.monotonic()
    if timeout <= 0:
        return {"result": "DEADLINE_EXHAUSTED", "exitCode": None, "elapsedMs": 0}
    try:
        process = subprocess.Popen(command, stdin=subprocess.DEVNULL,
            stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
            start_new_session=(os.name == "posix"))
    except OSError:
        return {"result": "LAUNCH_FAILED", "exitCode": None,
                "elapsedMs": round((time.monotonic() - started) * 1000)}
    try:
        code = process.wait(timeout=timeout)
        result = "PASS" if code == 0 else "EXIT_NONZERO"
    except subprocess.TimeoutExpired:
        result, code = "TIMEOUT", None
        try:
            if os.name == "posix":
                os.killpg(process.pid, signal.SIGKILL)
            else:
                process.kill()
        except ProcessLookupError:
            pass
        try:
            process.wait(timeout=2)
        except subprocess.TimeoutExpired:
            pass
    elapsed = time.monotonic() - started
    if elapsed > timeout and result == "PASS":
        result = "DEADLINE_EXHAUSTED"
    return {"result": result, "exitCode": code, "elapsedMs": round(elapsed * 1000)}


class Session:
    def __init__(self, udid, scratch, budget=MAX_BUDGET, runner=run_process):
        require(isinstance(udid, str) and re.fullmatch(r"[0-9A-Fa-f]{8}(?:-[0-9A-Fa-f]{4}){3}-[0-9A-Fa-f]{12}", udid), "DEVICE_ID_INVALID")
        require(0 < budget <= MAX_BUDGET, "BUDGET_INVALID")
        self.udid, self.scratch, self.runner = udid, Path(scratch), runner
        require(self.scratch.is_dir() and not self.scratch.is_symlink(), "SCRATCH_INVALID")
        self.started = time.monotonic()
        self.deadline = self.started + budget
        self.budget = budget
        self.commands = []

    def remaining(self, deadline=None):
        return max(0, min(self.deadline, deadline or self.deadline) - time.monotonic())

    def command(self, stage, args, limit, deadline=None):
        require(stage in STAGES, "STAGE_INVALID")
        allowance = min(limit, self.remaining(deadline))
        result = (self.runner(args, allowance) if allowance > 0 else
                  {"result": "DEADLINE_EXHAUSTED", "exitCode": None, "elapsedMs": 0})
        if not self.remaining(deadline) and result["result"] == "PASS":
            result = {**result, "result": "DEADLINE_EXHAUSTED"}
        self.commands.append({"stage": stage, **result})
        return result

    def probe(self, deadline=None):
        return self.command("getenv", ["xcrun", "simctl", "getenv", self.udid, "HOME"], 15, deadline)["result"] == "PASS"

    def pause(self, duration, deadline=None):
        time.sleep(min(duration, self.remaining(deadline)))

    def display(self, budget=90):
        deadline = min(self.deadline, time.monotonic() + budget)
        last_size = 0
        # Raster exists only inside a unique owned scratch directory and is
        # deleted on every exit. Only a bounded byte-count witness is retained.
        with tempfile.TemporaryDirectory(prefix="cherri-session-display-", dir=self.scratch) as directory:
            path = Path(directory) / "probe.png"
            for _ in range(19):
                if not self.remaining(deadline):
                    break
                path.unlink(missing_ok=True)
                last_size = 0
                if self.probe(deadline):
                    observed = self.command("screenshot", ["xcrun", "simctl", "io", self.udid, "screenshot", str(path)], 25, deadline)
                    if path.is_file() and not path.is_symlink():
                        last_size = min(path.stat().st_size, 100_000_000)
                    if observed["result"] == "PASS" and last_size >= MIN_DISPLAY_BYTES and self.remaining(deadline):
                        return True, last_size
                self.pause(5, deadline)
        return False, last_size

    def recover(self, force=False):
        status, reason, display_bytes = "BLOCKED", "READINESS_UNAVAILABLE", 0
        if not force and self.probe():
            # Preserve the existing quick path: getenv alone is a liveness
            # check. This does not claim a displayed editor or target import.
            status, reason = "PROBE_READY", "INITIAL_GETENV_READY"
        else:
            commands = (
                ("shutdown", ["xcrun", "simctl", "shutdown", self.udid], 30),
                ("boot", ["xcrun", "simctl", "boot", self.udid], 30),
                ("bootstatus", ["xcrun", "simctl", "bootstatus", self.udid, "-b"], 120),
                ("open", ["open", "-g", "-a", "Simulator", "--args", "-CurrentDeviceUDID", self.udid], 15),
            )
            for stage, command, allowance in commands:
                observed = self.command(stage, command, allowance)
                if observed["result"] in {"TIMEOUT", "DEADLINE_EXHAUSTED"}:
                    reason = "RECOVERY_COMMAND_TIMEOUT"
                    break
                if stage == "shutdown":
                    self.pause(2)
                elif stage == "open":
                    self.pause(3)
            else:
                if self.probe():
                    displayed, display_bytes = self.display(90)
                    if displayed and self.remaining():
                        status, reason = "RECOVERED", "FRESH_POST_RECOVERY_PROBES"
        elapsed = round((time.monotonic() - self.started) * 1000)
        if not self.remaining() and status != "BLOCKED":
            status, reason = "BLOCKED", "RECOVERY_COMMAND_TIMEOUT"
        return {"forceReboot": bool(force), "budgetMs": round(self.budget * 1000),
                "elapsedMs": elapsed, "status": status, "reason": reason,
                "springboardBytes": display_bytes, "commands": self.commands}


def validate_receipt(value, source_sha=None, run_id=None, fixture_id=None):
    require(isinstance(value, dict) and set(value) == {"schemaVersion", "compilerCommitSha", "runId", "fixtureId", "deviceIdSha256", "attempts"}, "RECEIPT_FIELDS")
    require(value["schemaVersion"] == "1.0", "RECEIPT_VERSION")
    require(isinstance(value["compilerCommitSha"], str) and re.fullmatch(r"[0-9a-f]{40}", value["compilerCommitSha"]), "RECEIPT_SOURCE")
    require(isinstance(value["runId"], str) and re.fullmatch(r"[A-Za-z0-9_-]{1,64}", value["runId"]), "RECEIPT_RUN")
    require(isinstance(value["fixtureId"], str) and re.fullmatch(r"[A-Za-z0-9_-]{1,96}", value["fixtureId"]), "RECEIPT_FIXTURE")
    require(isinstance(value["deviceIdSha256"], str) and re.fullmatch(r"[0-9a-f]{64}", value["deviceIdSha256"]), "RECEIPT_DEVICE")
    for key, expected in (("compilerCommitSha", source_sha), ("runId", run_id), ("fixtureId", fixture_id)):
        require(expected is None or value[key] == expected, "RECEIPT_IDENTITY")
    require(isinstance(value["attempts"], list) and 1 <= len(value["attempts"]) <= 8, "RECEIPT_ATTEMPTS")
    for index, attempt in enumerate(value["attempts"], 1):
        require(isinstance(attempt, dict) and set(attempt) == {"attempt", "forceReboot", "budgetMs", "elapsedMs", "status", "reason", "springboardBytes", "commands"}, "ATTEMPT_FIELDS")
        require(type(attempt["attempt"]) is int and attempt["attempt"] == index and type(attempt["forceReboot"]) is bool, "ATTEMPT_ID")
        # Preserve actual late observations; only successful readiness must fit
        # the budget. Process creation/OS scheduling are not hard real-time.
        require(integer(attempt["budgetMs"], 1, MAX_BUDGET * 1000) and integer(attempt["elapsedMs"], 0, MAX_OBSERVED_MS), "ATTEMPT_TIME")
        require(integer(attempt["springboardBytes"], 0, 100_000_000), "ATTEMPT_DISPLAY")
        require(attempt["status"] in {"PROBE_READY", "RECOVERED", "BLOCKED"} and attempt["reason"] in {"INITIAL_GETENV_READY", "FRESH_POST_RECOVERY_PROBES", "RECOVERY_COMMAND_TIMEOUT", "READINESS_UNAVAILABLE"}, "ATTEMPT_STATUS")
        commands = attempt["commands"]
        require(isinstance(commands, list) and 1 <= len(commands) <= 45, "ATTEMPT_COMMANDS")
        for command in commands:
            require(isinstance(command, dict) and set(command) == {"stage", "result", "exitCode", "elapsedMs"}, "COMMAND_FIELDS")
            require(command["stage"] in STAGES and command["result"] in RESULTS and integer(command["elapsedMs"], 0, MAX_OBSERVED_MS), "COMMAND_VALUE")
            require(command["exitCode"] is None or integer(command["exitCode"], -255, 255), "COMMAND_EXIT")
            require(command["result"] != "PASS" or command["exitCode"] == 0, "COMMAND_FALSE_PASS")
            require(command["result"] != "PASS" or command["elapsedMs"] <= STAGE_MAX_MS[command["stage"]], "COMMAND_PASS_AFTER_LIMIT")
            require(command["result"] != "EXIT_NONZERO" or command["exitCode"] not in (0, None), "COMMAND_FALSE_FAILURE")
            require(command["result"] not in {"TIMEOUT", "LAUNCH_FAILED"} or command["exitCode"] is None, "COMMAND_FALSE_TIMEOUT")
        # Command and attempt clocks are independently rounded milliseconds.
        # Permit at most one rounding millisecond per command, never an
        # understated aggregate or an out-of-budget successful command.
        require(sum(x["elapsedMs"] for x in commands) <= attempt["elapsedMs"] + len(commands), "ATTEMPT_COMMAND_TIME_MISMATCH")
        if attempt["status"] == "PROBE_READY":
            require(not attempt["forceReboot"] and attempt["reason"] == "INITIAL_GETENV_READY" and attempt["springboardBytes"] == 0 and len(commands) == 1 and commands[0]["stage"] == "getenv" and commands[0]["result"] == "PASS", "INITIAL_READY_FALSE")
        elif attempt["status"] == "RECOVERED":
            require(attempt["reason"] == "FRESH_POST_RECOVERY_PROBES" and attempt["springboardBytes"] >= MIN_DISPLAY_BYTES and len(commands) >= 7, "RECOVERY_FALSE")
            require([x["stage"] for x in commands[-2:]] == ["getenv", "screenshot"] and all(x["result"] == "PASS" for x in commands[-2:]), "RECOVERY_PROBES_MISSING")
        else:
            require(attempt["reason"] in {"RECOVERY_COMMAND_TIMEOUT", "READINESS_UNAVAILABLE"}, "BLOCK_REASON")
        if attempt["status"] != "PROBE_READY":
            offset = 0 if attempt["forceReboot"] else 1
            if offset:
                require(commands[0]["stage"] == "getenv" and commands[0]["result"] != "PASS", "RECOVERY_INITIAL_PROBE")
            sequence = commands[offset:]
            expected = ["shutdown", "boot", "bootstatus", "open"]
            require(sequence and [x["stage"] for x in sequence[:4]] == expected[:min(len(sequence), 4)], "RECOVERY_COMMAND_ORDER")
            for position, command in enumerate(sequence[:4]):
                if command["result"] in {"TIMEOUT", "DEADLINE_EXHAUSTED"}:
                    require(attempt["status"] == "BLOCKED" and attempt["reason"] == "RECOVERY_COMMAND_TIMEOUT" and len(sequence) == position + 1, "RECOVERY_AFTER_TIMEOUT")
                    break
            else:
                require(len(sequence) >= 5 and sequence[4]["stage"] == "getenv", "RECOVERY_POST_PROBE")
                if sequence[4]["result"] != "PASS":
                    require(len(sequence) == 5 and attempt["status"] == "BLOCKED", "RECOVERY_WITHOUT_POST_PROBE")
                else:
                    tail = sequence[5:]
                    for position, command in enumerate(tail):
                        previous = tail[position - 1] if position else None
                        if command["stage"] == "screenshot":
                            require(previous is not None and previous["stage"] == "getenv" and previous["result"] == "PASS", "SCREENSHOT_WITHOUT_PROBE")
                        else:
                            require(command["stage"] == "getenv" and not (previous and previous["stage"] == "getenv" and previous["result"] == "PASS"), "DISPLAY_COMMAND_ORDER")
        if attempt["status"] != "BLOCKED":
            require(attempt["elapsedMs"] <= attempt["budgetMs"], "READY_AFTER_DEADLINE")
    return value


def record_attempt(path, identity, attempt):
    path = Path(path)
    require(path.parent.is_dir() and not path.parent.is_symlink() and not path.is_symlink(), "RECEIPT_PATH")
    lock = path.with_name(path.name + ".lock")
    descriptor = os.open(lock, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600)
    os.close(descriptor)
    try:
        value = {**identity, "schemaVersion": "1.0", "attempts": []}
        if path.exists():
            require(path.is_file() and path.stat().st_size <= 64 * 1024, "RECEIPT_SIZE")
            value = validate_receipt(receipt_json(path.read_text(encoding="utf-8")), identity["compilerCommitSha"], identity["runId"], identity["fixtureId"])
            require(value["deviceIdSha256"] == identity["deviceIdSha256"], "DEVICE_CHANGED")
        attempt = {"attempt": len(value["attempts"]) + 1, **attempt}
        value["attempts"].append(attempt)
        validate_receipt(value)
        data = (json.dumps(value, sort_keys=True, indent=2) + "\n").encode()
        require(len(data) <= 64 * 1024, "RECEIPT_SIZE")
        fd, temporary = tempfile.mkstemp(prefix="session-receipt-", dir=path.parent)
        try:
            with os.fdopen(fd, "wb") as stream:
                stream.write(data)
            os.replace(temporary, path)
        finally:
            Path(temporary).unlink(missing_ok=True)
    finally:
        lock.unlink(missing_ok=True)
    return value


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("operation", choices=("probe", "display", "recover"))
    parser.add_argument("--udid", required=True)
    parser.add_argument("--scratch", required=True)
    parser.add_argument("--display-budget", type=int, choices=(60, 90), default=90)
    parser.add_argument("--force-reboot", choices=("0", "1"), default="0")
    parser.add_argument("--receipt")
    parser.add_argument("--source-sha")
    parser.add_argument("--run-id")
    parser.add_argument("--fixture-id")
    args = parser.parse_args(argv)
    session = Session(args.udid, args.scratch)
    if args.operation == "probe":
        return 0 if session.probe() else 1
    if args.operation == "display":
        ready, _ = session.display(args.display_budget)
        return 0 if ready else 1
    require(all((args.receipt, args.source_sha, args.run_id, args.fixture_id)), "RECOVERY_IDENTITY_MISSING")
    identity = {"compilerCommitSha": args.source_sha, "runId": args.run_id,
                "fixtureId": args.fixture_id, "deviceIdSha256": hashlib.sha256(args.udid.upper().encode()).hexdigest()}
    attempt = session.recover(args.force_reboot == "1")
    record_attempt(args.receipt, identity, attempt)
    print(json.dumps({"sessionRecovery": attempt["status"], "reason": attempt["reason"],
                      "elapsedMs": attempt["elapsedMs"]}, sort_keys=True))
    return 0 if attempt["status"] != "BLOCKED" else 1


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (OSError, ValueError) as error:
        # Fixed diagnostic; private command output/path never enters receipts.
        print("SIMULATOR_SESSION_RECOVERY_FAILED")
        raise SystemExit(2)
