"""New conservative, read-only Linux Go admission observer (not a recovered original).

The command line is closed and /proc is fixed. No command lines, environments,
credentials, subprocesses or signals to other processes are read or used.
Every discovered live Go process blocks admission, regardless of UID or cwd.
"""
from pathlib import Path
import datetime
import json
import os
import re
import signal
import sys
import time

SCHEMA = "cherri-exclusive-go-observation/1"
POLICY = "new-conservative-all-live-go-20261126"
MAX_SECONDS = 8
MAX_PROCESSES = 8192
MAX_OBSERVATION_BYTES = 3 * 1024 * 1024
# Verified in the host's Linux 7.0.0-1017 and 6.17.0-1020 sched.h originals.
PF_KTHREAD = 0x00200000
INACTIVE_STATES = frozenset(("Z", "X", "x"))
GO_NAMES = frozenset(("go", "compile", "link", "gofmt"))
TOOL_DIRECTORY = re.compile(r"/pkg/tool/[A-Za-z0-9]+_[A-Za-z0-9]+/[^/]+$")


class ObservationError(Exception):
    pass


def require(value, code):
    if not value:
        raise ObservationError(code)


def parse_stat(raw, pid):
    require(type(raw) is bytes and len(raw) <= 4096, "STAT_BOUND")
    opening, closing = raw.find(b" ("), raw.rfind(b") ")
    require(opening > 0 and closing > opening, "STAT_FORMAT")
    require(raw[:opening] == str(pid).encode(), "STAT_PID_IDENTITY")
    fields = raw[closing + 2:].split()
    require(len(fields) >= 20 and len(fields[0]) == 1 and fields[0].isalpha(), "STAT_FIELDS")
    try:
        flags, started = int(fields[6]), int(fields[19])
    except ValueError as error:
        raise ObservationError("STAT_NUMERIC_FIELDS") from error
    # PID1 and boot-time kernel threads really have start tick zero on this host.
    require(flags >= 0 and started >= 0, "STAT_IDENTITY_VALUES")
    return {"pid": pid, "startTimeTicks": started, "state": fields[0].decode("ascii"),
            "flags": flags, "comm": os.fsdecode(raw[opening + 2:closing])}


def parse_uids(raw):
    require(type(raw) is bytes and len(raw) <= 65536, "STATUS_BOUND")
    fields = [line.split()[1:] for line in raw.splitlines() if line.startswith(b"Uid:")]
    require(len(fields) == 1 and len(fields[0]) == 4 and all(x.isdigit() for x in fields[0]), "UID_FIELDS")
    return [int(x) for x in fields[0]]


class ProcReader:
    def pids(self):
        result = []
        with os.scandir("/proc") as entries:
            for entry in entries:
                if entry.name.isdecimal():
                    require(str(int(entry.name)) == entry.name and int(entry.name) > 0, "PROC_PID_NAME")
                    result.append(int(entry.name))
                    require(len(result) <= MAX_PROCESSES, "PROCESS_COUNT_BOUND")
        return sorted(result)

    def bounded(self, pid, name, limit):
        with open(f"/proc/{pid}/{name}", "rb", buffering=0) as stream:
            raw = stream.read(limit + 1)
        require(len(raw) <= limit, "PROC_READ_BOUND")
        return raw

    def state(self, pid):
        return parse_stat(self.bounded(pid, "stat", 4096), pid)

    def uids(self, pid):
        return parse_uids(self.bounded(pid, "status", 65536))

    def link(self, pid, name):
        require(name in ("exe", "cwd"), "FIXED_PROC_LINK")
        value = os.readlink(f"/proc/{pid}/{name}")
        require(value.startswith("/") and len(os.fsencode(value)) <= 4096, "PROC_LINK_BOUND_OR_KIND")
        return value

    def absent(self, pid):
        # A missing read alone does not establish exit: confirm the PID directory.
        try:
            os.stat(f"/proc/{pid}", follow_symlinks=False)
        except FileNotFoundError:
            return True
        return False


def go_reason(state, executable):
    path = executable.removesuffix(" (deleted)")
    if Path(path).name in GO_NAMES or state["comm"] in GO_NAMES:
        return "GO_EXECUTABLE_OR_COMM"
    if TOOL_DIRECTORY.search(path):
        return "GO_TOOL_DIRECTORY_EXECUTABLE"
    return None


def inactive(state, uids=None, reason=None):
    return {"pid": state["pid"], "startTimeTicks": state["startTimeTicks"],
            "state": state["state"], "uids": uids, "exe": None, "cwd": None,
            "classification": "INACTIVE", "inactiveReason": reason or "ZOMBIE_OR_DEAD"}


def observe_pid(reader, pid):
    before = None
    uids = None
    executable = cwd = None
    try:
        before = reader.state(pid)
        uids = reader.uids(pid)
        if before["state"] in INACTIVE_STATES or before["flags"] & PF_KTHREAD:
            after = reader.state(pid)
            require(before["startTimeTicks"] == after["startTimeTicks"], "PID_REUSED")
            if before["state"] in INACTIVE_STATES:
                require(after["state"] in INACTIVE_STATES, "INACTIVE_IDENTITY_CHANGED")
                return inactive(after, uids)
            require(all(x == 0 for x in uids) and after["flags"] & PF_KTHREAD, "KERNEL_IDENTITY")
            return {"pid": pid, "startTimeTicks": after["startTimeTicks"], "state": after["state"],
                    "uids": uids, "exe": None, "cwd": None, "classification": "KERNEL_THREAD",
                    "kernelFlag": PF_KTHREAD}
        executable, cwd = reader.link(pid, "exe"), reader.link(pid, "cwd")
        repeated_exe, repeated_cwd = reader.link(pid, "exe"), reader.link(pid, "cwd")
        repeated_uids, after = reader.uids(pid), reader.state(pid)
        require(before["startTimeTicks"] == after["startTimeTicks"], "PID_REUSED")
        if after["state"] in INACTIVE_STATES:
            return inactive(after, repeated_uids)
        require(not after["flags"] & PF_KTHREAD and before["comm"] == after["comm"] and
                executable == repeated_exe and cwd == repeated_cwd and uids == repeated_uids,
                "LIVE_IDENTITY_CHANGED")
        reason = go_reason(after, executable)
        return {"pid": pid, "startTimeTicks": after["startTimeTicks"], "state": after["state"],
                "uids": uids, "exe": executable, "cwd": cwd,
                "classification": "BLOCKED_GO" if reason else "OTHER_LIVE", "reason": reason}
    except OSError as original:
        # Resolve only an observed exit; a live unreadable identity is never admitted.
        try:
            current = reader.state(pid)
        except FileNotFoundError:
            require(reader.absent(pid), "VANISHED_PID_NOT_CONFIRMED")
            return {"pid": pid, "startTimeTicks": None if before is None else before["startTimeTicks"],
                    "state": None, "uids": uids, "exe": None, "cwd": None,
                    "classification": "VANISHED_CONFIRMED"}
        if before is not None:
            require(before["startTimeTicks"] == current["startTimeTicks"], "PID_REUSED")
        if current["state"] in INACTIVE_STATES:
            return inactive(current, uids, "EXIT_OBSERVED_AFTER_READ_ERROR")
        error = ObservationError("LIVE_PROC_READ_ERROR:" + str(original.errno))
        error.identity = {"startTimeTicks": current["startTimeTicks"], "state": current["state"],
                          "uids": uids, "exe": executable, "cwd": cwd}
        raise error from original
    except ObservationError as error:
        error.identity = {"startTimeTicks": None if before is None else before["startTimeTicks"],
                          "state": None if before is None else before["state"],
                          "uids": uids, "exe": executable, "cwd": cwd}
        raise


def observe(reader=None):
    reader = ProcReader() if reader is None else reader
    started = time.monotonic()
    report = {"schema": SCHEMA, "policy": POLICY, "createdAt": datetime.datetime.now(datetime.timezone.utc).isoformat(),
              "observerUid": os.getuid(), "observations": [], "errors": [], "blockedPids": [],
              "admitted": False, "historicalObserverRecovered": False,
              "historicalOwnerAllowlistUsed": False, "commandsOrEnvironmentsRead": False,
              "maxSeconds": MAX_SECONDS, "maxProcesses": MAX_PROCESSES}
    retained = 0
    seen = set()
    try:
        require(os.getuid() == 0, "ROOT_OBSERVER_REQUIRED")
        # One initial census plus one bounded census of newly visible PIDs.
        # This is an admission observation, never a claim about future processes.
        for _ in range(2):
            pids = reader.pids()
            require(len(pids) <= MAX_PROCESSES and len(set(pids)) == len(pids), "PROCESS_INVENTORY_BOUND")
            for pid in pids:
                if pid in seen:
                    continue
                require(time.monotonic() - started < MAX_SECONDS, "OBSERVER_DEADLINE")
                require(len(seen) < MAX_PROCESSES, "PROCESS_COUNT_BOUND")
                seen.add(pid)
                try:
                    row = observe_pid(reader, pid)
                except (ObservationError, OSError) as error:
                    row = {"pid": pid, "startTimeTicks": None, "state": None,
                           "uids": None, "exe": None, "cwd": None,
                           **getattr(error, "identity", {}), "classification": "UNRESOLVED_LIVE_IDENTITY",
                           "error": str(error) if isinstance(error, ObservationError) else "PROC_OS_ERROR:" + str(error.errno)}
                    report["errors"].append({"pid": pid, "code": row["error"]})
                retained += len(json.dumps(row, ensure_ascii=True))
                require(retained <= MAX_OBSERVATION_BYTES, "OBSERVATION_OUTPUT_BOUND")
                report["observations"].append(row)
                if row["classification"] == "BLOCKED_GO":
                    report["blockedPids"].append(pid)
        require(time.monotonic() - started < MAX_SECONDS, "OBSERVER_DEADLINE")
    except (ObservationError, OSError) as error:
        report["errors"].append({"pid": None, "code": str(error) if isinstance(error, ObservationError) else "PROC_OS_ERROR:" + str(error.errno)})
    report["admitted"] = not report["errors"] and not report["blockedPids"]
    report["elapsedSeconds"] = time.monotonic() - started
    report["observedPidCount"] = len(report["observations"])
    return report


def main():
    def deadline(_signum, _frame):
        raise ObservationError("OBSERVER_DEADLINE")
    if len(sys.argv) != 1 or os.getuid() != 0:
        print(json.dumps({"schema": SCHEMA, "policy": POLICY, "admitted": False,
                          "blockedPids": [], "errors": [{"code": "FIXED_ROOT_NO_ARGUMENT_ENTRY"}]}))
        return 2
    signal.signal(signal.SIGALRM, deadline)
    signal.setitimer(signal.ITIMER_REAL, MAX_SECONDS)
    try:
        report = observe()
    finally:
        signal.setitimer(signal.ITIMER_REAL, 0)
    print(json.dumps(report, sort_keys=True, ensure_ascii=True), flush=True)
    return 0 if report["admitted"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
