#!/usr/bin/env python3
"""One fixed public value-block component; F04 remains an unaccepted observation.

Import is inert. Generic source/process/event and output-ownership helpers are
reused byte-for-byte from the reviewed index/AI component wrappers.
"""
import calendar
from collections import Counter
from contextlib import contextmanager
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import re
import signal
import stat
import subprocess
import time

EXPECTED_SHA = "17bf9cbc5c418ebeb049f9e037279fb3e0150968"
ORIGIN = "https://github.com/davidpovarsky/cherri.git"
PACKAGE = "github.com/electrikmilk/cherri/tools/language-acceptance"
NESTED_PACKAGE = "github.com/electrikmilk/cherri"
INVENTORY_PATH = "tools/language-acceptance/combined_value_block_inventory.json"
INVENTORY_SHA256 = "6e72a9c8f934ad9b0923e941232630d5f8bdb121823f7e50983dfc683edcb94d"
PATHS_SHA256 = "8815479e1809972ff46ad19856b708a6cef1f994137b1250e1914d18bb096f6c"
HARNESS_PATHS = ("tools/p10r15/run_valueblock_component_ci.py",
                 ".github/workflows/p10r15-valueblock-component.yml")
PARENTS = ("TestCombinedValueBlockInventoryAndUnprovedRuntime",
           "TestCombinedValueBlockActualProductionEvidence",
           "TestCombinedValueBlockIncompleteF04Scope",
           "TestCombinedValueBlockF04ActualObservation")
FAULTS = ("wrong_tier", "wrong_requirement", "cross_tier_producer", "partial_command",
          "missing_artifact", "dirty_source", "wrong_source_digest", "missing_source_file",
          "missing_child", "skipped_child", "foreign_package")
EXPECTED_EVENTS = tuple(sorted(PARENTS + tuple(PARENTS[0] + "/" + name for name in
    ("source_count_before_launch", "required_source_before_launch")) + tuple(
    PARENTS[1] + "/" + requirement + "/" + fault
    for requirement in ("RP23", "BRG14") for fault in FAULTS) + tuple(
    PARENTS[3] + "/actual_subset_cannot_supply_" + tier for tier in ("unit", "native-structure"))))
COMMAND = ("go", "test", "./tools/language-acceptance", "-run", "^(" + "|".join(PARENTS) + ")$",
           "-count=1", "-json", "-timeout=9m")
MAX_SOURCE = 4 * 1024 * 1024
MAX_SOURCES = 16 * 1024 * 1024
MAX_LOG = 64 * 1024 * 1024
MAX_OUTPUT_FILES = 256
MAX_OUTPUT_BYTES = 256 * 1024 * 1024
MAX_OUTPUT_DIRECTORIES = 64
SCENARIOS = {'F04': {'group': 'control', 'title': 'Value loop collects one semantic element per iteration', 'tests': ['TestP15F04ValueLoopBoundaryUnit', 'TestP15F04ValueLoopBoundaryNative'], 'scope': 'Bounded supported facets only: actual zero/multiple repeat and for-item sources retain typed Text/Number/Bool results, exact complete native graphs and causal corruption controls. Nested List/Optional target rejection is explicitly observed and is not cardinality fulfillment. No complete F04 tier credit or Apple execution.', 'required': ['unit', 'native-structure']}, 'RP23': {'group': 'repair', 'title': 'Value block and loop yields', 'tests': ['TestP15RP23ValueBlockLoopUnit', 'TestP15RP23ValueBlockLoopNative'], 'scope': 'Distinct local joined scenario: Number if yields, Bool menu yields, zero/multiple Text loop yields, two-item for results and one final consumer. Exact semantic types, all native branch/result edges and absence of overwritten outputs are independently checked; missing/illegal/overwritten yields have causal source-bound diagnostics and no emitted artifact. This local supported scenario supplies no F04 nested List/Optional or Apple result evidence.', 'required': ['unit', 'native-structure']}, 'BRG14': {'group': 'recovery_gate', 'title': 'Value blocks, loop results and return', 'tests': ['TestBackendRecovery14ValueBlockReturnUnit', 'TestBackendRecovery14ValueBlockReturnNative'], 'scope': 'Distinct joined supported value-block scenario plus actual complete main/child early-return, bare-return, both-branch return and zero-iteration return graphs. Typed values, complete dispatch/function/caller ancestry, exact output/Nothing boundaries and final unused Text are independently specified and causally corrupted. No M02/F04/FN evidence inheritance; nested List/Optional transport and actual Apple value/cardinality/frame outcomes remain unproved.', 'required': ['unit', 'native-structure', 'ios-runtime']}}
F04_GAP = 'The complete F04 scenario is NOT_RUN: nested List and Optional loop elements have no verified native one-element/no-flatten/presence transport. Supported scalar and for-item graphs plus current target rejection are bounded observations only.'


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


def utc():
    return datetime.now(timezone.utc).isoformat(timespec="microseconds")


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def encoded(value):
    return (json.dumps(value, ensure_ascii=True, indent=2, allow_nan=False) + "\n").encode()


def strict_json(raw):
    def pairs(items):
        result = {}
        for key, value in items:
            require(key not in result, "duplicate JSON key: " + key)
            result[key] = value
        return result

    def invalid(value):
        raise ValueError("nonfinite JSON value: " + value)

    return json.loads(raw, object_pairs_hook=pairs, parse_constant=invalid)


def utc_ns(value):
    require(isinstance(value, str), "missing UTC timestamp")
    match = re.fullmatch(r"(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})(?:\.(\d{1,9}))?(Z|\+00:00)", value)
    require(match is not None, "timestamp must be explicit UTC RFC3339")
    whole = datetime.strptime(match[1], "%Y-%m-%dT%H:%M:%S")
    return calendar.timegm(whole.timetuple()) * 1_000_000_000 + int((match[2] or "").ljust(9, "0"))


def read_original(path, maximum):
    before = path.lstat()
    require(stat.S_ISREG(before.st_mode) and before.st_nlink == 1 and before.st_ino != 0,
            "expected one regular, non-linked file: " + str(path))
    require(before.st_size <= maximum, "file exceeds bounded read: " + str(path))
    with path.open("rb") as stream:
        opened = os.fstat(stream.fileno())
        require((opened.st_dev, opened.st_ino) == (before.st_dev, before.st_ino), "file identity changed")
        raw = stream.read(maximum + 1)
    after = path.lstat()
    fields = ("st_dev", "st_ino", "st_size", "st_mtime_ns")
    require(len(raw) == before.st_size <= maximum and
            all(getattr(before, key) == getattr(after, key) for key in fields),
            "file changed during bounded read: " + str(path))
    return raw, {"bytes": len(raw), "sha256": sha(raw), "device": before.st_dev,
                 "inode": before.st_ino, "mtime_ns": before.st_mtime_ns}


class TerminationRequested(RuntimeError):
    def __init__(self, signum):
        self.signum = signum
        super().__init__("received " + signal.Signals(signum).name)


@contextmanager
def cancellation_signals(ignore=False):
    def received(signum, _frame):
        raise TerminationRequested(signum)
    previous = {sig: signal.getsignal(sig) for sig in (signal.SIGTERM, signal.SIGINT)}
    try:
        for sig in previous:
            signal.signal(sig, signal.SIG_IGN if ignore else received)
        yield
    finally:
        for sig, handler in previous.items():
            signal.signal(sig, handler)


def parse_owned_ps(raw, pgid):
    """Retain numeric state only for this exact group; no command/environment text."""
    require(len(raw) <= 64 * 1024, "owned process snapshot exceeded its bound")
    members, seen = [], set()
    for line in raw.decode("ascii").splitlines():
        fields = line.split()
        require(len(fields) == 5, "malformed owned process snapshot")
        pid, parent, group, session = map(int, fields[:4])
        state = fields[4]
        if group != pgid:
            continue
        require(pid > 0 and parent >= 0 and session == pgid and pid not in seen and state,
                "inconsistent owned process identity")
        seen.add(pid)
        members.append({"pid": pid, "parent_pid": parent, "pgid": group, "session": session,
                        "state": state, "zombie": state.startswith("Z"),
                        "live": not state.startswith(("Z", "X"))})
    return sorted(members, key=lambda item: item["pid"])


def inspect_owned_group(pgid, timeout=1.0):
    # start_new_session makes SID=PGID=the child PID. procps selects that session;
    # the parser retains only its exact original PGID. Other groups are untouched.
    result = subprocess.run(["ps", "--sid", str(pgid), "-o", "pid=,ppid=,pgid=,sid=,stat="],
                            stdin=subprocess.DEVNULL, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                            timeout=timeout, check=False)
    require(result.returncode in (0, 1) and not result.stderr and
            (result.returncode == 0 or not result.stdout.strip()), "owned ps inspection failed")
    return parse_owned_ps(result.stdout, pgid)


def cleanup_owned_group(process, timed_out, interrupted, term_budget=5.0, kill_budget=10.0):
    pgid = process.pid
    require(pgid > 1 and pgid != os.getpgrp(), "refusing a group not created by this Popen")
    result = {"owned_process_group": pgid, "term_budget_seconds": term_budget,
              "kill_budget_seconds": kill_budget, "signals": [], "inspection_errors": [],
              "initial_members": [], "final_members": [], "orphaned_live_group": False,
              "complete": False}

    def inspect(timeout=1.0):
        process.poll()  # Reap our direct child before classifying any zombies.
        try:
            return inspect_owned_group(pgid, timeout=max(0.001, timeout))
        except Exception as error:
            result["inspection_errors"].append(type(error).__name__ + ": " + str(error))
            return None

    current = inspect()
    result["initial_members"] = current
    result["orphaned_live_group"] = (process.returncode is not None and not timed_out and
                                     not interrupted and current is not None and
                                     any(member["live"] for member in current))
    for sig, budget in ((signal.SIGTERM, term_budget), (signal.SIGKILL, kill_budget)):
        if current is not None and not any(member["live"] for member in current):
            break
        deadline = time.monotonic() + budget
        sent = {"signal": signal.Signals(sig).name, "at": utc(), "sent": False}
        try:
            os.killpg(pgid, sig)
            sent["sent"] = True
        except ProcessLookupError:
            sent["already_absent"] = True
        except OSError as error:
            sent["error"] = type(error).__name__ + ": " + str(error)
        result["signals"].append(sent)
        while time.monotonic() < deadline:
            current = inspect(min(1.0, deadline - time.monotonic()))
            if current is not None and not any(member["live"] for member in current):
                break
            remaining = deadline - time.monotonic()
            if remaining > 0:
                time.sleep(min(0.025, remaining))
    process.poll()
    result["final_members"] = current
    result["root_returncode"] = process.returncode
    result["complete"] = (process.returncode is not None and current is not None and
                          not any(member["live"] for member in current) and
                          not result["inspection_errors"] and
                          not any("error" in item for item in result["signals"]))
    return result


def run_owned_process(command, cwd, environment, output, label, budget,
                      term_budget=5.0, kill_budget=10.0):
    require(os.name == "posix", "reviewed process ownership requires POSIX")
    require(re.fullmatch(r"[a-z0-9-]+", label) is not None, "invalid process label")
    stdout_path, stderr_path = (output / (label + suffix) for suffix in (".stdout.original", ".stderr.original"))
    receipt_path = output / (label + ".process.json")
    require(not receipt_path.exists(), "process receipt already exists")
    row = {"command": list(command), "cwd": str(cwd), "budget_seconds": budget,
           "started_at": utc(), "finished_at": None, "pid": None, "exit_code": None,
           "timed_out": False, "error": None, "state": "STARTING", "cleanup": None}
    receipt_path.write_bytes(encoded(row))
    process = None
    begin = time.monotonic()
    with cancellation_signals():
        try:
            with stdout_path.open("xb", buffering=0) as stdout, stderr_path.open("xb", buffering=0) as stderr:
                try:
                    process = subprocess.Popen(command, cwd=cwd, env=environment, stdin=subprocess.DEVNULL,
                                               stdout=stdout, stderr=stderr, start_new_session=True)
                    row.update(pid=process.pid, state="RUNNING")
                    receipt_path.write_bytes(encoded(row))
                    try:
                        process.wait(timeout=max(0.0, budget - (time.monotonic() - begin)))
                        row["timed_out"] = time.monotonic() - begin > budget
                    except subprocess.TimeoutExpired:
                        row["timed_out"] = True
                except BaseException as error:
                    row["error"] = type(error).__name__ + ": " + str(error)
                finally:
                    # Success, timeout, exception and SIGTERM share this path.
                    # A second cancellation cannot interrupt bounded cleanup.
                    with cancellation_signals(ignore=True):
                        if process is not None:
                            row["cleanup"] = cleanup_owned_group(process, row["timed_out"],
                                row["error"] is not None, term_budget, kill_budget)
                            row["exit_code"] = process.returncode
                        row.update(finished_at=utc(), elapsed_seconds=round(time.monotonic() - begin, 6),
                                   state="TERMINAL")
                        receipt_path.write_bytes(encoded(row))
        except BaseException as error:
            row["error"] = row["error"] or type(error).__name__ + ": " + str(error)
        finally:
            with cancellation_signals(ignore=True):
                if row["finished_at"] is None:
                    row.update(finished_at=utc(), elapsed_seconds=round(time.monotonic() - begin, 6),
                               state="TERMINAL")
                for key, path in (("stdout", stdout_path), ("stderr", stderr_path)):
                    try:
                        _, row[key] = read_original(path, MAX_LOG)
                    except Exception as error:
                        row["error"] = row["error"] or type(error).__name__ + ": " + str(error)
                row["passed"] = (process is not None and row["exit_code"] == 0 and not row["timed_out"] and
                                 row["error"] is None and row["cleanup"] is not None and
                                 row["cleanup"]["complete"] and not row["cleanup"]["orphaned_live_group"])
                receipt_path.write_bytes(encoded(row))
    return row


class Metadata:
    def __init__(self, output, environment):
        self.output, self.environment = output, environment

    def git(self, repo, label, *arguments):
        command = ["git", "--no-replace-objects", "-c", "core.fsmonitor=false", *arguments]
        row = run_owned_process(command, repo, self.environment, self.output, label, 30)
        require(row["passed"], "Git metadata command failed: " + label)
        return read_original(self.output / (label + ".stdout.original"), MAX_LOG)[0]


def valid_origin(role, value):
    if role == "subject":
        return value == ORIGIN
    if role == "workflow":
        return value in (ORIGIN, "https://github.com/davidpovarsky/cherri")
    return False


def repository_state(metadata, phase, role, repo, expected_sha, names):
    def git(label, *args):
        return metadata.git(repo, phase + "-" + role + "-" + label, *args)

    require(re.fullmatch(r"[0-9a-f]{40}", expected_sha) is not None, "invalid exact source SHA")
    head = git("head", "rev-parse", "HEAD").decode().strip()
    require(head == expected_sha, "wrong " + role + " HEAD")
    top = Path(git("root", "rev-parse", "--show-toplevel").decode().strip()).resolve(strict=True)
    require(top == repo, "wrong " + role + " repository root")
    origin = git("origin", "remote", "get-url", "origin").decode().removesuffix("\n")
    require(valid_origin(role, origin), "wrong " + role + " origin")
    branch = git("branch", "branch", "--show-current").decode().strip()
    status = git("status", "status", "--porcelain=v1", "-z", "--untracked-files=all", "--ignored=matching")
    require(not status, role + " has tracked/untracked/ignored changes")
    flags = git("flags", "ls-files", "-v", "-z")
    flag_entries = flags.rstrip(b"\0").split(b"\0") if flags else []
    require(all(item.startswith(b"H ") for item in flag_entries), role + " has hidden index flags")
    staged = git("index", "ls-files", "--stage", "-z")
    indexed = {}
    for item in staged.split(b"\0"):
        if not item:
            continue
        fields, path = item.split(b"\t", 1)
        mode, blob, stage = fields.decode().split()
        name = path.decode("utf-8")
        require(stage == "0" and name not in indexed, "unmerged or duplicate index entry")
        indexed[name] = (mode, blob)
    tree_raw = git("tree", "ls-tree", "-r", "-z", "--full-tree", expected_sha)
    tree = {}
    for item in tree_raw.split(b"\0"):
        if not item:
            continue
        fields, path = item.split(b"\t", 1)
        mode, _kind, blob = fields.decode().split()
        name = path.decode("utf-8")
        require(name not in tree, "duplicate committed tree entry")
        tree[name] = (mode, blob)
    require(indexed == tree and len(flag_entries) == len(indexed) and
            {item[2:].decode("utf-8") for item in flag_entries} == set(indexed),
            role + " index differs from the exact committed tree")
    bindings = []
    for name in names:
        require(isinstance(name, str) and not Path(name).is_absolute() and ".." not in Path(name).parts,
                "invalid witness path")
        path = repo / name
        require(path.resolve(strict=True) == path and name in tree and tree[name][0] in ("100644", "100755"),
                "source witness must be a tracked regular path: " + name)
        raw, identity = read_original(path, MAX_SOURCE)
        blob = hashlib.sha1(b"blob " + str(len(raw)).encode() + b"\0" + raw).hexdigest()
        require(blob == tree[name][1], "source witness differs from exact Git blob: " + name)
        bindings.append(dict(identity, path=name, git_blob=blob, git_mode=tree[name][0]))
    require(sum(item["bytes"] for item in bindings) <= MAX_SOURCES, "source witness aggregate exceeds cap")
    return {"root": str(repo), "head": head, "origin": origin, "branch": branch, "status": "",
            "ignored_checked": True, "tracked_files": len(indexed), "index_flags_sha256": sha(flags),
            "index_stage_sha256": sha(staged), "git_tree_sha256": sha(tree_raw), "source_files": bindings}


def parse_go_events(raw, started_at, finished_at, expected_events=EXPECTED_EVENTS, package=PACKAGE):
    expected = set(expected_events)
    require(len(expected) == len(expected_events), "duplicate expected event")
    start, finish = utc_ns(started_at), utc_ns(finished_at)
    require(start <= finish, "reversed actual process interval")
    started, terminal, package_terminal = [], {}, []
    seen_names, packages = set(), set()
    errors, parse_errors = [], []
    package_started, package_finished = 0, False
    paused = set()
    previous_time = start

    def problem(line, message):
        errors.append({"line": line, "error": message})

    for number, line in enumerate(raw.splitlines(), 1):
        try:
            event = strict_json(line)
            require(isinstance(event, dict), "Go event must be an object")
            action, name, event_package = event.get("Action"), event.get("Test", ""), event.get("Package")
            require(isinstance(name, str) and isinstance(event_package, str), "invalid Go test/package field")
            require(action in ("start", "run", "pause", "cont", "output", "pass", "fail", "skip"),
                    "unexpected Go action")
        except Exception as error:
            parse_errors.append({"line": number, "error": type(error).__name__ + ": " + str(error)})
            continue
        # Retain actual fail/skip/unstarted accounting even when timing is invalid.
        try:
            event_time = utc_ns(event.get("Time"))
            require(start <= event_time <= finish, "Go event outside actual process interval")
            require(event_time >= previous_time, "Go event time moved backwards")
            previous_time = event_time
            if action in ("pass", "fail", "skip"):
                require("Elapsed" in event, "terminal Go event lacks Elapsed")
            if "Elapsed" in event:
                require(type(event["Elapsed"]) in (int, float) and event["Elapsed"] >= 0 and
                        event["Elapsed"] < float("inf"), "invalid Go elapsed value")
        except Exception as error:
            problem(number, type(error).__name__ + ": " + str(error))
        packages.add(event_package)
        if event_package != package:
            problem(number, "foreign package")
        if name:
            seen_names.add(name)
        if package_finished:
            problem(number, "event after package terminal")
        if action == "start":
            if name or package_started or started:
                problem(number, "invalid/duplicate package start")
            package_started += 1
            continue
        if package_started != 1:
            problem(number, "event before unique package start")
        if action == "run":
            if not name or name not in expected:
                problem(number, "unexpected test run")
            if name in started or name in terminal:
                problem(number, "duplicate/late test run")
            for ancestor in expected:
                if name.startswith(ancestor + "/") and (ancestor not in started or ancestor in terminal):
                    problem(number, "child ran outside active parent")
            started.append(name)
        elif action in ("pause", "cont"):
            if not name or name not in started or name in terminal:
                problem(number, "inactive test pause/cont")
            if (action == "pause") == (name in paused):
                problem(number, "invalid test pause/cont order")
            if action == "pause":
                paused.add(name)
            else:
                paused.discard(name)
        elif action in ("pass", "fail", "skip") and name:
            if started.count(name) != 1 or name in terminal:
                problem(number, "test terminal without exactly one prior run")
            if any(child.startswith(name + "/") and child not in terminal for child in started):
                problem(number, "parent terminal before active child")
            if action == "pass" and any(terminal.get(child) != ["pass"] for child in expected
                                        if child.startswith(name + "/")):
                problem(number, "parent passed before all required children passed")
            terminal.setdefault(name, []).append(action)
            paused.discard(name)
        elif action in ("pass", "fail", "skip"):
            if package_finished or any(name not in terminal for name in started):
                problem(number, "package terminal before test terminals or duplicate")
            if action == "pass" and (set(terminal) != expected or any(v != ["pass"] for v in terminal.values())):
                problem(number, "package passed before complete fixed test inventory")
            package_terminal.append(event)
            package_finished = True
        elif action == "output" and name:
            if name not in expected or started.count(name) != 1 or name in terminal:
                problem(number, "output for inactive or unexpected test")
        elif action != "output":
            problem(number, "test action missing its test name")
    counts = Counter(started)
    inventory_ok = (set(counts) == expected and all(value == 1 for value in counts.values()) and
                    set(terminal) == expected and all(value == ["pass"] for value in terminal.values()))
    unexpected_packages = sorted(packages - {package})
    result = {"expected_events": len(expected), "started": len(started),
              "passed": sum(value == ["pass"] for value in terminal.values()),
              "failed": sum("fail" in value for value in terminal.values()),
              "skipped": sum("skip" in value for value in terminal.values()),
              "unstarted": sorted(expected - set(started)), "unterminated": sorted(set(started) - set(terminal)),
              "unexpected": sorted(seen_names - expected), "unexpected_packages": unexpected_packages,
              "parse_errors": parse_errors, "event_order_errors": errors, "package_started": package_started,
              "package_terminal": package_terminal, "terminal_by_test": terminal,
              "run_count_by_test": dict(sorted(counts.items()))}
    result["passed_all_events"] = (inventory_ok and not parse_errors and not errors and not result["unexpected"] and
        not unexpected_packages and package_started == 1 and len(package_terminal) == 1 and
        package_terminal[0]["Action"] == "pass")
    return result


def directory_identity(path):
    info = path.lstat()
    require(path.resolve(strict=True) == path and stat.S_ISDIR(info.st_mode) and info.st_ino != 0,
            "owned output directory is linked or not a directory")
    return {"path": str(path), "device": info.st_dev, "inode": info.st_ino}


def create_owned_output(value, roots):
    given = Path(value)
    require(given.is_absolute() and str(given) == os.path.normpath(value), "output must be absolute and clean")
    destination = given.parent.resolve(strict=True) / given.name
    for root in roots:
        require(not destination.is_relative_to(root) and not root.is_relative_to(destination),
                "output must be physically outside both checkouts")
    require(not destination.exists() and not destination.is_symlink(), "exclusive output already exists")
    destination.mkdir(mode=0o700)
    (destination / "producer").mkdir(mode=0o700)
    (destination / "metadata").mkdir(mode=0o700)
    identities = {name: directory_identity(destination / name) for name in (".", "producer", "metadata")}
    return destination, identities


def check_owned_output(destination, identities):
    require({name: directory_identity(destination / name) for name in identities} == identities,
            "owned output directory identity changed")


def read_owned_original(path, parent, maximum=MAX_SOURCE):
    require(path.is_absolute() and path.is_relative_to(parent) and path.resolve(strict=True) == path,
            "original artifact escaped its physical owned directory")
    return read_original(path, maximum)


def check_nested_process(directory, prefix, requirement, command, tests, exit_code, budget, inventory, outer):
    paths = {key: directory / (prefix + suffix) for key, suffix in
             (("process", "-process.json"), ("raw", "-go.jsonl"), ("stderr", "-stderr.log"))}
    originals = {key: read_owned_original(path, directory) for key, path in paths.items()}
    receipt = strict_json(originals["process"][0])
    fields = {"format_version", "requirement", "source_commit", "source_clean_before", "source_clean_after",
              "source_files", "command", "required_tests", "discovered_tests", "started_utc", "finished_utc",
              "elapsed_seconds", "exit_code", "timed_out", "raw_sha256", "raw_bytes", "stderr_sha256",
              "stderr_bytes", "benchmark_sha256", "benchmark_bytes"}
    require(isinstance(receipt, dict) and set(receipt) == fields, "unexpected nested process receipt shape")
    require(type(receipt["format_version"]) is int and receipt["format_version"] == 1 and
            receipt["requirement"] == requirement and receipt["source_commit"] == EXPECTED_SHA and
            receipt["source_clean_before"] is True and receipt["source_clean_after"] is True and
            receipt["command"] == list(command) and receipt["required_tests"] == tests and
            receipt["discovered_tests"] == tests, "nested process command/source/inventory mismatch")
    require(type(receipt["exit_code"]) is int and receipt["exit_code"] == exit_code and
            receipt["timed_out"] is False and type(receipt["benchmark_bytes"]) is int and
            receipt["benchmark_bytes"] == 0 and receipt["benchmark_sha256"] == "",
            "unexpected nested process result")
    expected_sources = [{"path": row["path"], "sha256": row["sha256"]}
                        for row in inventory["production_source_witnesses"]]
    require(receipt["source_files"] == expected_sources, "nested process lost an exact production source witness")
    started, finished = utc_ns(receipt["started_utc"]), utc_ns(receipt["finished_utc"])
    elapsed = receipt["elapsed_seconds"]
    require(utc_ns(outer["started_at"]) <= started < finished <= utc_ns(outer["finished_at"]) and
            type(elapsed) in (int, float) and 0 < elapsed <= budget and
            abs((finished - started) / 1e9 - elapsed) <= 1,
            "nested process is outside its actual interval or original budget")
    for key, stem in (("raw", "raw"), ("stderr", "stderr")):
        raw, binding = originals[key]
        require(type(receipt[stem + "_bytes"]) is int and receipt[stem + "_bytes"] == len(raw) and
                receipt[stem + "_sha256"] == binding["sha256"], "nested original digest/size mismatch")
    return receipt, originals, paths


def collect_original_inventory(root):
    result, total, directories = [], 0, 0

    def visit(directory):
        nonlocal total, directories
        directories += 1
        require(directories <= MAX_OUTPUT_DIRECTORIES and len(directory.relative_to(root).parts) <= 8,
                "retained original directory inventory exceeds cap")
        directory_identity(directory)
        entries = []
        for path in directory.iterdir():
            require(len(entries) < MAX_OUTPUT_FILES + MAX_OUTPUT_DIRECTORIES,
                    "retained original directory entry count exceeds cap")
            entries.append(path)
        for path in sorted(entries, key=lambda item: item.name):
            info = path.lstat()
            if stat.S_ISDIR(info.st_mode):
                visit(path)
                continue
            require(len(result) < MAX_OUTPUT_FILES, "retained original file count exceeds cap")
            _, identity = read_owned_original(path, root, MAX_LOG)
            total += identity["bytes"]
            require(total <= MAX_OUTPUT_BYTES, "retained original bytes exceed aggregate cap")
            result.append(dict(identity, path=path.relative_to(root).as_posix()))
    visit(root)
    return sorted(result, key=lambda item: item["path"])


def fixed_inventory(raw):
    require(sha(raw) == INVENTORY_SHA256, "fixed value-block inventory bytes changed")
    data = strict_json(raw)
    require(isinstance(data, dict) and set(data) == {"events", "source_paths"},
            "unexpected fixed value-block inventory shape")
    names = data["source_paths"]
    require(isinstance(names, list) and len(names) == 111 and names == sorted(set(names)) and
            sha(("\n".join(names) + "\n").encode()) == PATHS_SHA256,
            "changed exact 111 source witness set")
    require(len(EXPECTED_EVENTS) == len(set(EXPECTED_EVENTS)) == 30 and
            isinstance(data["events"], dict) and set(data["events"]) == set(SCENARIOS),
            "changed fixed outer or nested scenario set")
    for identity, count in (("F04", 47), ("RP23", 58), ("BRG14", 98)):
        events = data["events"][identity]
        require(isinstance(events, list) and len(events) == count and events == sorted(set(events)),
                "changed fixed nested event inventory: " + identity)
        parents = SCENARIOS[identity]["tests"]
        require(all(parent in events for parent in parents) and
                all(any(event == parent or event.startswith(parent + "/") for parent in parents) for event in events),
                "foreign nested test identity: " + identity)
    return data


def nested_command(identity):
    return ("go", "test", "-count=1", "-json", "-timeout", "3m0s", ".", "-run",
            "^(" + "|".join(SCENARIOS[identity]["tests"]) + ")$")


def validate_artifact_tuple(artifacts, paths, originals):
    require(isinstance(artifacts, list) and len(artifacts) == 3, "missing original artifact tuple")
    declared = [Path(item["path"]) for item in artifacts]
    require(len(set(declared)) == 3 and set(declared) == set(paths.values()),
            "artifact paths do not identify this actual process")
    for item in artifacts:
        key = next(key for key, path in paths.items() if path == Path(item["path"]))
        require(not item.get("run_id") and item["sha256"] == originals[key][1]["sha256"],
                "artifact digest or run identity differs from the actual original")


def validate_valueblock_records(producer, inventory, processes, outer):
    proof_path = producer / "combined-value-block-evidence.json"
    proof_raw, proof_binding = read_owned_original(proof_path, producer)
    proof = strict_json(proof_raw)
    require(isinstance(proof, dict) and set(proof) == {"source", "report", "results", "records"} and
            proof["source"] == EXPECTED_SHA, "combined local proof is detached")
    require(isinstance(proof["records"], list) and len(proof["records"]) == 4 and
            set(proof["results"]) == {"RP23", "BRG14"}, "combined proof changed its requirement scope")
    for identity, passed, status in (("RP23", True, "PASSED"), ("BRG14", False, "PENDING_EXTERNAL")):
        result = proof["results"][identity]
        require(result["id"] == identity and result["status"] == status and result["passed"] is passed and
                result["executed_levels"] == ["unit", "native-structure"] and
                result["group"] == SCENARIOS[identity]["group"] and
                result["requirement"] == SCENARIOS[identity]["title"],
                "combined result promoted an unexecuted scope: " + identity)
    found = set()
    for record in proof["records"]:
        require(record["requirements"] in (["RP23"], ["BRG14"]), "foreign or inherited requirement record")
        identity, tier = record["requirements"][0], record["tier"]
        require(tier in ("unit", "native-structure") and (identity, tier) not in found,
                "duplicate or external value-block tier")
        found.add((identity, tier))
        spec = SCENARIOS[identity]
        test = spec["tests"][0 if tier == "unit" else 1]
        require(record["implementation_sha"] == EXPECTED_SHA and record["result"] == "passed" and
                type(record["exit_code"]) is int and record["exit_code"] == 0 and
                record["test_id"] == NESTED_PACKAGE + ":" + test and
                record["command"] == list(nested_command(identity)), "detached value-block producer record")
        assertions = record["assertions"]
        for key, value in {"scope": spec["scope"], "required_production_tests": "\n".join(spec["tests"]),
                           "required_go_events": "\n".join(inventory["events"][identity])}.items():
            require(assertions.get(key) == value, "combined assertion changed: " + key)
        _, originals, paths = processes[identity]
        validate_artifact_tuple(record["artifacts"], paths, originals)
    require(found == {(identity, tier) for identity in ("RP23", "BRG14") for tier in ("unit", "native-structure")},
            "missing actual local value-block tier")
    report = proof["report"]
    require(report["phase"] == "local" and report["closure_complete"] is False and
            all(type(report[key]) is int and report[key] == value for key, value in
                (("total_cases", 2), ("passed_cases", 1), ("failed_cases", 0), ("skipped_cases", 1))),
            "local report changed immutable scoped accounting")
    require(utc_ns(outer["started_at"]) <= utc_ns(report["timestamp"]) <= utc_ns(outer["finished_at"]),
            "local report timestamp is outside the actual outer interval")
    require(isinstance(report["results"], list) and len(report["results"]) == 2 and
            {row["id"] for row in report["results"]} == {"RP23", "BRG14"}, "local report inherited a requirement")
    for row in report["results"]:
        identity = row["id"]
        require(row["executed_levels"] == ["unit", "native-structure"] and
                row["passed"] is (identity == "RP23") and
                row["status"] == ("PASSED" if identity == "RP23" else "NOT_RUN"),
                "local validation report expanded completion scope")

    observation_path = producer / "combined-value-block-f04-observation.json"
    observation_raw, observation_binding = read_owned_original(observation_path, producer)
    saved = strict_json(observation_raw)
    require(isinstance(saved, dict) and set(saved) == {"source", "result", "observation"} and
            saved["source"] == EXPECTED_SHA, "F04 observation is detached")
    result, observation = saved["result"], saved["observation"]
    require(result["id"] == "F04" and result["status"] == "NOT_RUN" and result["passed"] is False and
            not result.get("executed_levels") and result["detail"].startswith(F04_GAP),
            "supported F04 facets acquired complete requirement credit")
    require(set(observation) == {"schema", "implementation_sha", "requirement", "result", "tests", "scope",
                                   "artifacts", "complete_requirement_executed", "formal_acceptance_granted"} and
            observation["schema"] == "cherri.value-loop-facets/v1" and
            observation["implementation_sha"] == EXPECTED_SHA and observation["requirement"] == "F04" and
            observation["result"] == "DIAGNOSTIC_OBSERVATION" and observation["tests"] == SCENARIOS["F04"]["tests"] and
            observation["scope"] == SCENARIOS["F04"]["scope"] and
            observation["complete_requirement_executed"] is False and observation["formal_acceptance_granted"] is False,
            "F04 diagnostic observation overclaims its supported subset")
    _, originals, paths = processes["F04"]
    validate_artifact_tuple(observation["artifacts"], paths, originals)
    return {"combined_proof_sha256": proof_binding["sha256"],
            "F04_observation_sha256": observation_binding["sha256"],
            "RP23": {"status": "PASSED", "scope": "unit/native local contract only"},
            "BRG14": {"status": "PENDING_EXTERNAL", "executed_levels": ["unit", "native-structure"]},
            "F04": {"status": "NOT_RUN", "records": 0, "complete_requirement_executed": False,
                    "formal_acceptance_granted": False, "coverage_gap": F04_GAP}}, {proof_path, observation_path}


def inspect_valueblock_originals(destination, inventory, outer, source_before):
    producer = destination / "producer"
    retained = collect_original_inventory(producer)
    observed_files = {producer / row["path"] for row in retained}
    result = {"scope": "local value-block scenarios and separate incomplete F04 diagnostic",
              "source": EXPECTED_SHA, "nested": {}, "errors": [], "passed_all": False,
              "formal_acceptance_granted": False, "apple_runtime_proved": False,
              "F04_complete_requirement_executed": False, "nested_cleanup_proved": False,
              "nested_process_terminal_receipts_complete": False,
              "cleanup_scope": "outer PGID only; nested original terminal receipts bind execution, not descendant cleanup"}
    source_inventory = {"production_source_witnesses": source_before["source_files"]}
    processes, expected_files = {}, set()
    for identity in ("F04", "RP23", "BRG14"):
        prefix = "combined-value-block-" + identity.lower()
        entry = {"verified": False, "expected_events": len(inventory["events"][identity]), "errors": []}
        result["nested"][identity] = entry
        try:
            candidates = [path for path in observed_files if path.name == prefix + "-process.json"]
            require(len(candidates) == 1, "missing or duplicate nested process receipt: " + identity)
            directory = candidates[0].parent
            require(directory.parent == producer and re.fullmatch(prefix + r"-[A-Za-z0-9]+", directory.name),
                    "nested original escaped its exact owned child: " + identity)
            directory_identity(directory)
            # Keep actual failures and partial discovery visible even if strict
            # intake rejects the exit, source, timing or evidence tuple below.
            receipt = strict_json(read_owned_original(candidates[0], directory)[0])
            entry["process"] = receipt
            raw, raw_binding = read_owned_original(directory / (prefix + "-go.jsonl"), directory)
            entry["raw"] = raw_binding
            entry["events"] = parse_go_events(raw, receipt["started_utc"], receipt["finished_utc"],
                expected_events=tuple(inventory["events"][identity]), package=NESTED_PACKAGE)
            actual = check_nested_process(directory, prefix, identity, nested_command(identity),
                inventory["events"][identity], 0, 240, source_inventory, outer)
            require(entry["events"]["passed_all_events"], "incomplete fixed nested Go inventory: " + identity)
            processes[identity] = actual
            expected_files.update(actual[2].values())
            entry["verified"] = True
        except BaseException as error:
            detail = type(error).__name__ + ": " + str(error)
            entry["errors"].append(detail)
            result["errors"].append(identity + ": " + detail)
    result["nested_process_terminal_receipts_complete"] = len(processes) == 3
    try:
        require(len(processes) == 3, "cannot validate scoped reports without all three original process tuples")
        result["scoped_records"], report_paths = validate_valueblock_records(producer, inventory, processes, outer)
        expected_files.update(report_paths)
        require(len(expected_files) == 11 and observed_files == expected_files,
                "unexpected or missing retained value-block original")
    except BaseException as error:
        result["errors"].append(type(error).__name__ + ": " + str(error))
    result["retained_producer_files"] = len(observed_files)
    result["passed_all"] = not result["errors"]
    return result


def main():
    require(os.name == "posix", "this wrapper requires the reviewed Ubuntu process-group owner")
    root = Path(os.environ["CHERRI_COMPONENT_SOURCE"]).resolve(strict=True)
    script = Path(__file__).resolve(strict=True)
    workflow_root, workflow_sha = script.parents[2], os.environ["GITHUB_SHA"]
    require(script == workflow_root / HARNESS_PATHS[0], "wrapper is outside its exact tracked path")
    require(not root.is_relative_to(workflow_root) and not workflow_root.is_relative_to(root), "checkouts must be distinct")
    destination, ownership = create_owned_output(os.environ["CHERRI_COMPONENT_OUT"], (root, workflow_root))
    environment = dict(os.environ, GIT_NO_REPLACE_OBJECTS="1", PYTHONDONTWRITEBYTECODE="1")
    environment["CHERRI_COMBINED_VALUE_BLOCK_EVIDENCE_OUTPUT"] = str(destination / "producer")
    metadata = Metadata(destination / "metadata", environment)
    summary = {"schema": "cherri.valueblock-focused-replay/v1", "source": EXPECTED_SHA,
               "workflow_source": workflow_sha, "execution_platform": "ubuntu-ci", "command": list(COMMAND),
               "actual_go_started": False, "passed_all": False, "errors": [],
               "formal_acceptance_granted": False, "apple_runtime_proved": False,
               "nested_process_terminal_receipts_complete": False, "nested_cleanup_proved": False,
               "cleanup_evidence_status": "INCOMPLETE",
               "cleanup_scope": "outer owned PGID only; each nested original process must also be terminal"}
    before, after, process, inventory, names = {}, {}, None, None, None
    with cancellation_signals():
        try:
            original = metadata.git(root, "fixed-inventory-blob", "show", EXPECTED_SHA + ":" + INVENTORY_PATH)
            require(read_original(root / INVENTORY_PATH, MAX_SOURCE)[0] == original,
                    "fixed inventory differs from exact subject Git blob")
            inventory = fixed_inventory(original)
            names = inventory["source_paths"]
            before["subject"] = repository_state(metadata, "before", "subject", root, EXPECTED_SHA, names)
            before["workflow"] = repository_state(metadata, "before", "workflow", workflow_root, workflow_sha, HARNESS_PATHS)
            check_owned_output(destination, ownership)
            launch = dict(summary, root=str(root), created_at=utc(), outer_go_timeout_seconds=540,
                driver_timeout_seconds=570, workflow_step_timeout_seconds=600, nested_go_timeout_seconds=180,
                nested_driver_timeout_seconds=240, cleanup_term_seconds=5, cleanup_kill_seconds=10,
                policy="First fixed value-block component launch; original inner budgets and no retry.",
                expected_outer_events=EXPECTED_EVENTS, source_before=before["subject"],
                workflow_before=before["workflow"], owned_output=ownership,
                expected_nested_events=inventory["events"],
                retained_evidence_environment="CHERRI_COMBINED_VALUE_BLOCK_EVIDENCE_OUTPUT")
            (destination / "launch.json").write_bytes(encoded(launch))
            process = run_owned_process(COMMAND, root, environment, destination, "outer-go", 570)
        except BaseException as error:
            summary["errors"].append(type(error).__name__ + ": " + str(error))
        finally:
            if process is None and (destination / "outer-go.process.json").is_file():
                try:
                    process = strict_json(read_original(destination / "outer-go.process.json", MAX_SOURCE)[0])
                except Exception as error:
                    summary["errors"].append("process receipt: " + str(error))
            summary["actual_go_started"] = process is not None and process.get("pid") is not None
            for role, repo, revision, files in (("subject", root, EXPECTED_SHA, names),
                                               ("workflow", workflow_root, workflow_sha, HARNESS_PATHS)):
                if files is None:
                    continue
                try:
                    after[role] = repository_state(metadata, "after", role, repo, revision, files)
                except BaseException as error:
                    after[role] = {"error": type(error).__name__ + ": " + str(error)}
            summary.update(source_before=before.get("subject"), source_after=after.get("subject"),
                           workflow_before=before.get("workflow"), workflow_after=after.get("workflow"), process=process)
            summary["source_files_unchanged"] = "subject" in before and before["subject"] == after.get("subject")
            summary["workflow_files_unchanged"] = "workflow" in before and before["workflow"] == after.get("workflow")
            if process is not None:
                try:
                    raw, raw_receipt = read_original(destination / "outer-go.stdout.original", MAX_LOG)
                    _, stderr_receipt = read_original(destination / "outer-go.stderr.original", MAX_LOG)
                    require(raw_receipt == process.get("stdout") and stderr_receipt == process.get("stderr"),
                            "Original outer streams changed after process receipt")
                    parsed = parse_go_events(raw, process["started_at"], process["finished_at"])
                    summary.update(parsed, started_at=process["started_at"], finished_at=process["finished_at"],
                        elapsed_seconds=process["elapsed_seconds"], exit_code=process["exit_code"],
                        timed_out=process["timed_out"], error=process["error"],
                        raw_sha256=raw_receipt["sha256"], raw_bytes=raw_receipt["bytes"],
                        stderr_sha256=stderr_receipt["sha256"], stderr_bytes=stderr_receipt["bytes"])
                    check_owned_output(destination, ownership)
                    summary["valueblock_originals"] = inspect_valueblock_originals(destination, inventory, process, before["subject"])
                    intake = summary["valueblock_originals"]
                    summary["errors"].extend(intake["errors"])
                    summary["nested_process_terminal_receipts_complete"] = intake["nested_process_terminal_receipts_complete"]
                    if process.get("passed") is True and intake["passed_all"]:
                        summary["cleanup_evidence_status"] = "OUTER_PGID_COMPLETE_NESTED_TERMINALS_BOUND"
                    summary["passed_all"] = (process.get("passed") is True and parsed["passed_all_events"] and
                        summary["source_files_unchanged"] and summary["workflow_files_unchanged"] and not summary["errors"])
                except BaseException as error:
                    summary["errors"].append(type(error).__name__ + ": " + str(error))
            members = []
            try:
                check_owned_output(destination, ownership)
                members = collect_original_inventory(destination)
                require(len(members) + 2 <= MAX_OUTPUT_FILES, "summary/manifest exceed output file cap")
            except BaseException as error:
                summary["errors"].append(type(error).__name__ + ": " + str(error))
            summary["passed_all"] = summary["passed_all"] and not summary["errors"]
            summary_raw = encoded(summary)
            (destination / "summary.json").write_bytes(summary_raw)
            members.append(dict(path="summary.json", **read_original(destination / "summary.json", MAX_LOG)[1]))
            artifact_manifest = {"schema": "cherri.valueblock-component-original-inventory/v1", "subject": EXPECTED_SHA,
                                 "members": members, "excludes_self": "original-inventory.json",
                                 "formal_acceptance_granted": False}
            manifest_raw = encoded(artifact_manifest)
            require(sum(row["bytes"] for row in members) + len(manifest_raw) <= MAX_OUTPUT_BYTES,
                    "summary/manifest exceed output byte cap")
            (destination / "original-inventory.json").write_bytes(manifest_raw)
    print(json.dumps({"directory": str(destination), "actual_go_started": summary["actual_go_started"],
                      "passed_all": summary["passed_all"], "errors": summary["errors"]}, ensure_ascii=True))
    return 0 if summary["passed_all"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
