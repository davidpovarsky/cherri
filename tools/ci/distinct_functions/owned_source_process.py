"""Bounded POSIX process ownership for fixed, source-pinned private controllers.

This module does not select a compiler, test, source, or acceptance criterion.
Its caller must supply a fixed reviewed command route.  Every command receives
the same whole-owner deadline, including preflight and post-source commands.
"""
from contextlib import contextmanager
from pathlib import Path
import datetime
import hashlib
import json
import math
import os
import signal
import subprocess
import threading
import time


class OwnerAbort(Exception):
    pass


def utc():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


def original(path, value):
    path = Path(path)
    raw = value if isinstance(value, bytes) else (json.dumps(value, sort_keys=True, indent=2) + "\n").encode()
    with path.open("xb") as stream:
        stream.write(raw)
    return {"bytes": len(raw), "sha256": hashlib.sha256(raw).hexdigest()}


class WholeBudget:
    """One main-thread wall budget; cancellation is deferred only while owning cleanup.

    In particular, handlers cannot throw between Popen returning and assignment
    of its child handle.  A pending signal is acted on immediately afterward.
    The alarm also interrupts synchronous pre/post work outside that window.
    """
    def __init__(self, seconds, cleanup_seconds=8, final_reserve=20):
        if not all(type(x) in (int, float) and math.isfinite(x) and x > 0
                   for x in (seconds, cleanup_seconds)):
            raise ValueError("FINITE_POSITIVE_BUDGET_REQUIRED")
        if not 0 <= final_reserve < seconds:
            raise ValueError("FINAL_RESERVE")
        self.started = time.monotonic()
        self.deadline = self.started + seconds
        self.seconds = seconds
        self.cleanup_seconds = cleanup_seconds
        self.final_reserve = final_reserve
        self.cancel_signal = None
        self.alarm_fired = False
        self._deferred = 0
        self._finalizing = 0
        self._previous = {}
        self._old_timer = None

    def _handler(self, signum, _frame):
        if signum == signal.SIGALRM:
            self.alarm_fired = True
        elif self.cancel_signal is None:
            self.cancel_signal = signum
        if self._deferred == 0 and (signum == signal.SIGALRM or not self._finalizing):
            raise OwnerAbort("WHOLE_OWNER_DEADLINE" if signum == signal.SIGALRM else "OWNER_CANCELLED")

    def __enter__(self):
        if threading.current_thread() is not threading.main_thread() or os.name != "posix":
            raise ValueError("MAIN_THREAD_POSIX_OWNER_REQUIRED")
        self._old_timer = signal.getitimer(signal.ITIMER_REAL)
        if self._old_timer != (0.0, 0.0):
            raise ValueError("EXISTING_ALARM_NOT_OWNED")
        for sig in (signal.SIGINT, signal.SIGTERM, signal.SIGALRM):
            self._previous[sig] = signal.getsignal(sig)
            signal.signal(sig, self._handler)
        signal.setitimer(signal.ITIMER_REAL, max(0.000001, self.deadline - time.monotonic()))
        return self

    def __exit__(self, _kind, _value, _trace):
        signal.setitimer(signal.ITIMER_REAL, 0)
        for sig, handler in self._previous.items():
            signal.signal(sig, handler)

    @contextmanager
    def defer(self):
        self._deferred += 1
        try:
            yield
        finally:
            self._deferred -= 1

    @contextmanager
    def finalizing(self):
        self._finalizing += 1
        try:
            yield
        finally:
            self._finalizing -= 1

    def check(self, reserve=0):
        if self.alarm_fired or time.monotonic() >= self.deadline - reserve:
            raise OwnerAbort("WHOLE_OWNER_DEADLINE")
        if self.cancel_signal is not None and not self._finalizing:
            raise OwnerAbort("OWNER_CANCELLED")

    def child_deadline(self, seconds, post_reserve=None):
        self.check()
        if type(seconds) not in (int, float) or not math.isfinite(seconds) or seconds <= 0:
            raise ValueError("FINITE_CHILD_LIMIT_REQUIRED")
        reserve = self.final_reserve if post_reserve is None else post_reserve
        stop = min(time.monotonic() + seconds, self.deadline - reserve - self.cleanup_seconds)
        if stop <= time.monotonic():
            raise OwnerAbort("NO_CHILD_BUDGET")
        return stop


def group_observation(pgid):
    """Linux live-process observation; never conceal retained zombie identities."""
    try:
        os.killpg(pgid, 0)
    except ProcessLookupError:
        return {"state": "ABSENT", "kernelGroupExists": False, "live": False, "members": []}
    except PermissionError:
        return {"state": "UNKNOWN", "kernelGroupExists": None, "live": None, "members": []}
    members = []
    try:
        for item in Path("/proc").iterdir():
            if not item.name.isdecimal():
                continue
            try:
                raw = (item / "stat").read_text()
                fields = raw[raw.rfind(")") + 2:].split()
                if int(fields[2]) == pgid:
                    members.append({"pid": int(item.name), "state": fields[0],
                                    "pgid": int(fields[2]), "sid": int(fields[3]),
                                    "startTicks": int(fields[19])})
            except (FileNotFoundError, ProcessLookupError, PermissionError):
                continue
    except (OSError, ValueError, IndexError):
        return {"state": "UNKNOWN", "kernelGroupExists": True, "live": None, "members": members}
    members.sort(key=lambda item: item["pid"])
    if any(item["sid"] != pgid for item in members):
        return {"state": "UNKNOWN", "kernelGroupExists": True, "live": None, "members": members}
    live = [item for item in members if item["state"] not in ("Z", "X")]
    if not members:
        try:
            os.killpg(pgid, 0)
        except ProcessLookupError:
            return {"state": "ABSENT", "kernelGroupExists": False, "live": False, "members": []}
        return {"state": "UNKNOWN", "kernelGroupExists": True, "live": None, "members": []}
    return {"state": "LIVE" if live else "ZOMBIE_ONLY", "kernelGroupExists": True,
            "live": bool(live), "members": members}


def _cleanup(process, pgid, authorized, deadline):
    result = {"groupSignalAuthorized": authorized, "signals": [], "errors": []}
    if not authorized:
        try:
            process.kill()
            result["directChildSignal"] = "SIGKILL"
        except ProcessLookupError:
            pass
    else:
        result["before"] = group_observation(pgid)
        for sig, allowance in ((signal.SIGTERM, 2.0), (signal.SIGKILL, 3.0)):
            observed = group_observation(pgid)
            if observed["live"] is False:
                break
            try:
                os.killpg(pgid, sig)
                result["signals"].append(signal.Signals(sig).name)
            except ProcessLookupError:
                break
            remaining = max(0, deadline - time.monotonic())
            grace = min(allowance, remaining / 3) if sig == signal.SIGTERM else allowance
            stop = min(deadline, time.monotonic() + grace)
            while time.monotonic() < stop:
                process.poll()
                observed = group_observation(pgid)
                if observed["live"] is False:
                    break
                time.sleep(min(.02, max(0, stop - time.monotonic())))
    try:
        process.wait(timeout=max(0, min(2, deadline - time.monotonic())))
    except subprocess.TimeoutExpired:
        result["errors"].append("PARENT_NOT_REAPED")
    result["parentReaped"] = process.returncode is not None
    result["after"] = group_observation(pgid) if authorized else {"state": "UNAUTHORIZED_GROUP", "live": None}
    result["noLiveGroupMembers"] = authorized and result["after"]["live"] is False
    return result


def run_owned(argv, *, cwd, env, output, name, budget, seconds,
              monitor=None, journal=None, post_reserve=None):
    """Execute one caller-selected command, retaining failures before returning.

    File outputs are exclusive, never pipes that can deadlock on descendants.
    Parent exit does not waive group observation.  Unexpected journal/monitor
    exceptions follow the exact same finally cleanup as cancellation/timeouts.
    """
    if not isinstance(budget, WholeBudget):
        raise ValueError("SHARED_WHOLE_BUDGET_REQUIRED")
    if not name or any(char not in "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_." for char in name):
        raise ValueError("PROCESS_NAME")
    output = Path(output)
    tick = time.monotonic()
    stop = budget.child_deadline(seconds, post_reserve)
    process = None
    authorized = False
    row = {"name": name, "argv": [str(x) for x in argv], "startedAt": utc(),
           "pid": None, "pgid": None, "sid": None, "exitCode": None,
           "commandSeconds": seconds, "wholeOwnerSeconds": budget.seconds,
           "effectiveCommandSeconds": max(0, stop - tick), "status": "FAILED",
           "timedOut": False, "cancelled": False, "error": None}
    try:
        with (output / (name + ".stdout.original")).open("xb") as stdout, (output / (name + ".stderr.original")).open("xb") as stderr:
            with budget.defer():
                process = subprocess.Popen(argv, cwd=cwd, env=env, stdin=subprocess.DEVNULL,
                                           stdout=stdout, stderr=stderr, start_new_session=True)
                row.update(pid=process.pid, pgid=os.getpgid(process.pid), sid=os.getsid(process.pid))
                authorized = row["pgid"] == process.pid and row["sid"] == process.pid
            if not authorized:
                raise OwnerAbort("PRIVATE_SESSION_NOT_ESTABLISHED")
            original(output / (name + ".start.original.json"), {**row, "ownershipVerified": True})
            if journal is not None:
                journal("started", dict(row))
            while True:
                budget.check()
                if time.monotonic() >= stop:
                    row["timedOut"] = True
                    raise OwnerAbort("COMMAND_OR_WHOLE_BUDGET_LIMIT")
                if monitor is not None:
                    monitor()
                code = process.poll()
                if code is not None:
                    row["exitCode"] = code
                    observed = group_observation(process.pid)
                    row["groupAtParentExit"] = observed
                    if observed["live"] is not False:
                        raise OwnerAbort("GROUP_REMAINED_AFTER_PARENT_EXIT")
                    row["status"] = "PASS" if code == 0 else "EXIT_NONZERO"
                    break
                time.sleep(min(.05, max(0, stop - time.monotonic())))
    except BaseException as error:
        row["error"] = {"type": type(error).__name__, "message": str(error)}
        row["cancelled"] = budget.cancel_signal is not None or isinstance(error, KeyboardInterrupt)
        row["timedOut"] = row["timedOut"] or budget.alarm_fired or time.monotonic() >= budget.deadline
    finally:
        with budget.defer():
            if process is not None:
                row["cleanup"] = _cleanup(process, process.pid, authorized,
                    min(budget.deadline, time.monotonic() + budget.cleanup_seconds))
                row["exitCode"] = process.returncode
            else:
                row["cleanup"] = {"childStarted": False, "parentReaped": True,
                                  "noLiveGroupMembers": True, "signals": []}
            row["cancelSignal"] = budget.cancel_signal
            row["cancelled"] = row["cancelled"] or budget.cancel_signal is not None
            row["finishedAt"] = utc()
            row["elapsedSeconds"] = time.monotonic() - tick
            row["ownerElapsedSeconds"] = time.monotonic() - budget.started
            row["wholeDeadlineExceeded"] = row["ownerElapsedSeconds"] >= budget.seconds
            if (row["cancelled"] or row["wholeDeadlineExceeded"] or
                not row["cleanup"]["parentReaped"] or not row["cleanup"]["noLiveGroupMembers"]):
                row["status"] = "FAILED"
            for key in ("stdout", "stderr"):
                path = output / (name + "." + key + ".original")
                if path.is_file():
                    digest = hashlib.sha256()
                    size = 0
                    with path.open("rb") as stream:
                        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
                            digest.update(chunk)
                            size += len(chunk)
                            if time.monotonic() >= budget.deadline:
                                row["streamHashIncomplete"] = True
                                break
                    row[key] = {"bytes": size, "sha256": digest.hexdigest(),
                                "complete": size == path.stat().st_size}
            row["elapsedSeconds"] = time.monotonic() - tick
            row["ownerElapsedSeconds"] = time.monotonic() - budget.started
            row["wholeDeadlineExceeded"] = row["ownerElapsedSeconds"] >= budget.seconds
            if row["wholeDeadlineExceeded"] or row.get("streamHashIncomplete"):
                row["status"] = "FAILED"
            original(output / (name + ".process.original.json"), row)
    return row


def require_success(row):
    if row["status"] != "PASS":
        raise OwnerAbort("PROCESS_FAILED:" + row["name"])
    return row
