#!/usr/bin/env python3
"""Run the exact public index component and preserve failure/cleanup evidence.

Import is inert. Production budgets remain 9m Go / 570s supervisor / 600s step.
Local Python controls can exercise ownership and parsing without Go or network.
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

EXPECTED_SHA = "1dcc7b3f97da992200fdf6b627f2c67090ea5298"
ORIGIN = "https://github.com/davidpovarsky/cherri.git"
PACKAGE = "github.com/electrikmilk/cherri/tools/language-acceptance"
INVENTORY_PATH = "tools/language-acceptance/combined_index_arithmetic_inventory.json"
PATHS_SHA256 = "0889e41841fc38f79de586a1e6c374e4a7bcde312916eb84971a5d02caa4c252"
HARNESS_PATHS = ("tools/p10r15/run_index_component_ci.py",
                 ".github/workflows/p10r15-index-component.yml")
PARENTS = ("TestCombinedIndexArithmeticInventoryAndUnprovedRuntime",
           "TestCombinedIndexArithmeticActualProductionEvidence")
FAULTS = ("wrong_tier", "wrong_requirement", "cross_tier_producer", "partial_command",
          "missing_artifact", "dirty_source", "wrong_source_digest", "missing_source_file",
          "missing_child", "skipped_child", "foreign_package")
EXPECTED_EVENTS = tuple(sorted(PARENTS + tuple(PARENTS[0] + "/" + name for name in
    ("source_count_before_launch", "required_source_before_launch")) + tuple(
    PARENTS[1] + "/" + requirement + "/" + fault
    for requirement in ("RP21", "BRG16") for fault in FAULTS)))
COMMAND = ("go", "test", "./tools/language-acceptance", "-run", "^(" + "|".join(PARENTS) + ")$",
           "-count=1", "-json", "-timeout=9m")
MAX_SOURCE = 4 * 1024 * 1024
MAX_SOURCES = 16 * 1024 * 1024
MAX_LOG = 64 * 1024 * 1024


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


def main():
    require(os.name == "posix", "this wrapper is limited to the reviewed Ubuntu process-group owner")
    root = Path(os.environ["CHERRI_COMPONENT_SOURCE"]).resolve(strict=True)
    dest = Path(os.environ["CHERRI_COMPONENT_OUT"]).resolve()
    script = Path(__file__).resolve(strict=True)
    workflow_root = script.parents[2]
    workflow_sha = os.environ["GITHUB_SHA"]
    require(script == workflow_root / HARNESS_PATHS[0], "wrapper is outside its exact tracked path")
    for other in (root, workflow_root):
        require(not dest.is_relative_to(other) and not other.is_relative_to(dest), "output must be outside checkouts")
    require(not root.is_relative_to(workflow_root) and not workflow_root.is_relative_to(root), "checkouts must be distinct")
    require(not dest.exists(), "exclusive evidence destination exists")
    dest.mkdir(parents=True)
    (dest / "producer").mkdir()
    metadata_dir = dest / "metadata"
    metadata_dir.mkdir()
    environment = dict(os.environ, GIT_NO_REPLACE_OBJECTS="1", PYTHONDONTWRITEBYTECODE="1")
    metadata = Metadata(metadata_dir, environment)
    summary = {"schema": "cherri.p15.index-arithmetic-clean-replay/v1", "source": EXPECTED_SHA,
               "workflow_source": workflow_sha, "execution_platform": "ubuntu-ci", "command": list(COMMAND),
               "actual_go_started": False, "passed_all": False, "errors": [],
               "formal_acceptance_granted": False, "apple_runtime_proved": False}
    before, after, process, names, inventory = {}, {}, None, None, None
    with cancellation_signals():
        try:
            original = metadata.git(root, "inventory-blob", "show", EXPECTED_SHA + ":" + INVENTORY_PATH)
            working = read_original(root / INVENTORY_PATH, MAX_SOURCE)[0]
            require(working == original, "inventory differs from exact subject Git blob")
            inventory = strict_json(original)
            names = inventory["source_paths"]
            require(isinstance(names, list) and len(names) == 109 and sorted(set(names)) == names,
                    "expected exactly 109 fixed source paths")
            require(sha(("\n".join(names) + "\n").encode()) == PATHS_SHA256, "wrong source witness inventory")
            require(len(EXPECTED_EVENTS) == 26 and len(inventory["events"]["RP21"]) == 232 and
                    len(inventory["events"]["BRG16"]) == 213, "wrong fixed event inventory")
            before["subject"] = repository_state(metadata, "before", "subject", root, EXPECTED_SHA, names)
            before["workflow"] = repository_state(metadata, "before", "workflow", workflow_root, workflow_sha, HARNESS_PATHS)
            launch = dict(summary, root=str(root), created_at=utc(), outer_go_timeout_seconds=540,
                driver_timeout_seconds=570, workflow_step_timeout_seconds=600, nested_go_timeout_seconds=180,
                nested_driver_timeout_seconds=240, cleanup_term_seconds=5, cleanup_kill_seconds=10,
                policy="One exact reviewed public component run; original budgets, no retry.",
                scope="Fixed RP21/BRG16 component only; no full179 or Apple runtime credit",
                expected_outer_events=EXPECTED_EVENTS, expected_nested=inventory["events"],
                source_before=before["subject"], workflow_before=before["workflow"])
            (dest / "launch.json").write_bytes(encoded(launch))
            environment["CHERRI_COMBINED_INDEX_ARITHMETIC_EVIDENCE_OUTPUT"] = str(dest / "producer")
            process = run_owned_process(COMMAND, root, environment, dest, "outer-go", 570)
        except BaseException as error:
            summary["errors"].append(type(error).__name__ + ": " + str(error))
        finally:
            # If cancellation occurs between a returned process receipt and the
            # caller assignment, retain the actual launch instead of saying NOT_RUN.
            if process is None and (dest / "outer-go.process.json").is_file():
                try:
                    process = strict_json(read_original(dest / "outer-go.process.json", MAX_SOURCE)[0])
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
                    raw, raw_receipt = read_original(dest / "outer-go.stdout.original", MAX_LOG)
                    _, stderr_receipt = read_original(dest / "outer-go.stderr.original", MAX_LOG)
                    parsed = parse_go_events(raw, process["started_at"], process["finished_at"])
                    summary.update(parsed, started_at=process["started_at"], finished_at=process["finished_at"],
                        elapsed_seconds=process["elapsed_seconds"], exit_code=process["exit_code"],
                        timed_out=process["timed_out"], error=process["error"],
                        raw_sha256=raw_receipt["sha256"], raw_bytes=raw_receipt["bytes"],
                        stderr_sha256=stderr_receipt["sha256"], stderr_bytes=stderr_receipt["bytes"])
                    summary["passed_all"] = (process.get("passed") is True and parsed["passed_all_events"] and
                        summary["source_files_unchanged"] and summary["workflow_files_unchanged"] and not summary["errors"])
                except BaseException as error:
                    summary["errors"].append(type(error).__name__ + ": " + str(error))
            (dest / "summary.json").write_bytes(encoded(summary))
    print(json.dumps({"directory": str(dest), "actual_go_started": summary["actual_go_started"],
                      "passed_all": summary["passed_all"], "errors": summary["errors"]}, ensure_ascii=True))
    return 0 if summary["passed_all"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
