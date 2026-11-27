#!/usr/bin/env python3
"""Observe a complete immutable Git checkout with batched, journaled metadata.

The original critical-source metadata allowance remains ten seconds. One full
tree read replaces one `git show` per critical file; all tracked physical bytes
and modes are still verified. The previous separate fifteen-second tree-command
allowance is used for a closing HEAD/tree/cleanliness check. The complete
physical traversal additionally has a 120-second ceiling.
"""
import datetime
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import signal
import stat
import threading
import time

import project_prerequisite_observation as bounded

METADATA_SECONDS = 10
CLOSING_SECONDS = 15
WHOLE_SECONDS = 120
FAILURE_CLEANUP_SECONDS = 2
MAX_METADATA_BYTES = 2 * 1024 * 1024
MAX_CRITICAL_BYTES = 2 * 1024 * 1024


def require(value, code):
    if not value:
        raise ValueError("SOURCE_BINDING_" + code)


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def canonical(value):
    return (json.dumps(value, ensure_ascii=False, sort_keys=True, indent=2, allow_nan=False) + "\n").encode()


def utc():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


def original(path, raw):
    with Path(path).open("xb") as stream:
        os.chmod(path, 0o600)
        stream.write(raw)
        stream.flush()
        os.fsync(stream.fileno())


def record(path, value):
    original(path, canonical(value))


class WholeDeadline:
    """A main-thread ceiling that preserves an earlier caller alarm."""
    def __enter__(self):
        require(threading.current_thread() is threading.main_thread(), "MAIN_THREAD_REQUIRED")
        self.started = time.monotonic()
        self.handler = signal.getsignal(signal.SIGALRM)
        self.timer = signal.getitimer(signal.ITIMER_REAL)
        require(self.timer[1] == 0 and (self.timer[0] == 0 or callable(self.handler)), "OUTER_ALARM_CONTRACT")
        self.outer_fired = False
        self.outer_first = self.timer[0] > 0 and self.timer[0] <= WHOLE_SECONDS
        def expired(signum, frame):
            if self.outer_first and callable(self.handler):
                self.outer_fired = True
                self.handler(signum, frame)
            raise TimeoutError("SOURCE_BINDING_WHOLE_DEADLINE")
        signal.signal(signal.SIGALRM, expired)
        signal.setitimer(signal.ITIMER_REAL, min(self.timer[0], WHOLE_SECONDS) if self.timer[0] > 0 else WHOLE_SECONDS)
        self.deadline = self.started + (min(self.timer[0], WHOLE_SECONDS) if self.timer[0] > 0 else WHOLE_SECONDS)
        return self.deadline

    def __exit__(self, kind, value, traceback):
        signal.setitimer(signal.ITIMER_REAL, 0)
        signal.signal(signal.SIGALRM, self.handler)
        if self.timer[0] > 0 and not self.outer_fired:
            remaining = self.timer[0] - (time.monotonic() - self.started)
            if remaining > 0:
                signal.setitimer(signal.ITIMER_REAL, remaining)
            else:
                self.outer_fired = True
                self.handler(signal.SIGALRM, None)
                raise TimeoutError("SOURCE_BINDING_OUTER_DEADLINE")


def owned_directory(value):
    path = Path(value)
    require(path.is_absolute() and ".." not in path.parts and "\n" not in str(path) and "\r" not in str(path), "PATH")
    require(path.is_dir() and not path.is_symlink() and path.resolve(strict=True) == path, "DIRECTORY")
    require(path.stat().st_uid == os.getuid(), "DIRECTORY_OWNER")
    return path


def parse_tree(raw):
    tree = {}
    for row in raw.split(b"\0"):
        if not row:
            continue
        metadata, name = row.split(b"\t", 1)
        mode, kind, blob = metadata.decode("ascii").split()
        name = name.decode("utf-8", errors="strict")
        require(kind == "blob" and mode in ("100644", "100755", "120000")
                and re.fullmatch(r"[0-9a-f]{40}", blob) is not None, "TREE_KIND")
        require(name and not Path(name).is_absolute() and ".." not in Path(name).parts
                and "\n" not in name and "\r" not in name and name not in tree, "TREE_PATH")
        tree[name] = {"mode": mode, "gitBlob": blob}
    require(tree, "EMPTY_TREE")
    return tree


def physical_pin(repo, name, entry, deadline, critical=False):
    path = repo / name
    require(path.parent.resolve(strict=True) == path.parent, "PHYSICAL_ANCESTOR")
    before = path.lstat()
    mode = entry["mode"]
    require(stat.S_ISLNK(before.st_mode) if mode == "120000" else stat.S_ISREG(before.st_mode), "PHYSICAL_KIND")
    require(not critical or mode != "120000", "CRITICAL_REGULAR_FILE")
    require(mode == "120000" or bool(before.st_mode & 0o111) == (mode == "100755"), "PHYSICAL_MODE")
    if mode == "120000":
        data = os.fsencode(os.readlink(path))
        size = len(data)
        digest = hashlib.sha1(b"blob " + str(size).encode() + b"\0" + data).hexdigest()
        checksum = sha(data)
    else:
        size = before.st_size
        require(not critical or size <= MAX_CRITICAL_BYTES, "CRITICAL_SIZE")
        blob_hash = hashlib.sha1(b"blob " + str(size).encode() + b"\0")
        content_hash = hashlib.sha256()
        count = 0
        descriptor = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
        with os.fdopen(descriptor, "rb") as stream:
            opened = os.fstat(stream.fileno())
            require((opened.st_dev, opened.st_ino, opened.st_mode, opened.st_size) ==
                    (before.st_dev, before.st_ino, before.st_mode, before.st_size), "PHYSICAL_OPEN_CHANGED")
            while True:
                require(time.monotonic() < deadline, "PHYSICAL_DEADLINE")
                chunk = stream.read(1024 * 1024)
                if not chunk:
                    break
                blob_hash.update(chunk)
                content_hash.update(chunk)
                count += len(chunk)
                require(count <= size, "PHYSICAL_SIZE_CHANGED")
        require(count == size, "PHYSICAL_SIZE_CHANGED")
        digest, checksum = blob_hash.hexdigest(), content_hash.hexdigest()
    after = path.lstat()
    require((before.st_dev, before.st_ino, before.st_mode, before.st_size, before.st_mtime_ns, before.st_ctime_ns) ==
            (after.st_dev, after.st_ino, after.st_mode, after.st_size, after.st_mtime_ns, after.st_ctime_ns),
            "PHYSICAL_IDENTITY_CHANGED")
    require(digest == entry["gitBlob"], "PHYSICAL_GIT_BLOB")
    return {"mode": mode, "gitBlob": digest, "bytes": size, "sha256": checksum}


def source_inventory(repo, commit, output, critical_paths):
    repo = owned_directory(repo)
    require(isinstance(commit, str) and re.fullmatch(r"[0-9a-f]{40}", commit), "COMMIT")
    require(type(critical_paths) in (tuple, list) and critical_paths
            and all(type(path) is str for path in critical_paths) and len(set(critical_paths)) == len(critical_paths),
            "CRITICAL_PATHS")
    output = Path(output)
    require(output.is_absolute() and ".." not in output.parts and "\n" not in str(output) and "\r" not in str(output)
            and not output.exists()
            and not output.is_symlink() and repo not in output.parents, "OUTPUT_CREATE_ONLY")
    owned_directory(output.parent)
    output.mkdir(mode=0o700)
    report = {"schema": "cherri-complete-source-binding-observation/1", "status": "SOURCE_UNVERIFIED",
              "commit": commit, "repo": str(repo), "startedUTC": utc(), "metadataSeconds": METADATA_SECONDS,
              "closingSeconds": CLOSING_SECONDS, "wholeSeconds": WHOLE_SECONDS,
              "failureCleanupSeconds": FAILURE_CLEANUP_SECONDS,
              "sourceMutation": False, "nativeInvocation": False, "processes": []}
    git = shutil.which("git")
    env_prefix = ["/usr/bin/env"]
    for name in sorted(k for k in os.environ if k.startswith("GIT_")):
        env_prefix.extend(["-u", name])
    env_prefix.extend(["GIT_NO_REPLACE_OBJECTS=1", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null",
                       "GIT_TERMINAL_PROMPT=0", git, "--no-replace-objects", "-C", str(repo)])
    def command(label, args, deadline):
        folder = output / label
        folder.mkdir(mode=0o700)
        argv = env_prefix + list(args)
        # Execution retains the original phase cutoff. Only failed cleanup may
        # use two further seconds, still capped by the original whole deadline.
        cleanup_deadline = min(whole_deadline, deadline + FAILURE_CLEANUP_SECONDS)
        receipt = {"argv": argv, "observation": None, "process": None,
                   "executionDeadlineMonotonic": deadline,
                   "cleanupDeadlineMonotonic": cleanup_deadline}
        def journal(event, value):
            record(folder / ("process-" + event + ".original.json"), value)
            receipt["process"] = json.loads(canonical(value))
            receipt["lastJournalEvent"] = event
        try:
            with (folder / "stdout.original").open("xb") as stdout, (folder / "stderr.original").open("xb") as stderr:
                observation, process = bounded.capture(argv, max(0, deadline - time.monotonic()), output=stdout,
                    error_output=stderr, execution_deadline=deadline, hard_deadline=cleanup_deadline,
                    stream_limit=MAX_METADATA_BYTES, journal=journal)
            receipt.update(observation=observation, process=process)
        except BaseException as error:
            receipt.update(errorType=type(error).__name__, error=str(error)[:2048])
            raise
        finally:
            record(folder / "process.original.json", receipt)
            report["processes"].append({"label": label, "receipt": label + "/process.original.json", **receipt})
        require(process.get("cleanupParentReaped") is True and process.get("groupPresentAfterCleanup") is False,
                "PROCESS_CLEANUP")
        require(observation.get("result") == "PASS" and type(observation.get("exitCode")) is int
                and observation["exitCode"] == 0, "PROCESS_" + label)
        return (folder / "stdout.original").read_bytes()
    try:
        require(git is not None and Path(git).is_absolute(), "GIT_EXECUTABLE")
        with WholeDeadline() as whole_deadline:
            metadata_deadline = min(whole_deadline, time.monotonic() + METADATA_SECONDS)
            metadata = command("identity", ["rev-parse", "HEAD", "HEAD^{tree}", "--show-toplevel",
                "--git-path", "info/grafts", "--git-path", "objects/info/alternates"], metadata_deadline)
            lines = metadata.decode("utf-8", errors="strict").splitlines()
            require(len(lines) == 5 and lines[0] == commit and re.fullmatch(r"[0-9a-f]{40}", lines[1])
                    and Path(lines[2]) == repo, "IDENTITY")
            tree_id = lines[1]
            redirect_paths = []
            for name in lines[3:]:
                path = Path(name)
                if not path.is_absolute():
                    path = repo / path
                require(not path.exists() and not path.is_symlink(), "OBJECT_REDIRECT")
                redirect_paths.append(path)
            require(not command("clean", ["status", "--porcelain=v1", "--untracked-files=all"], metadata_deadline), "CLEAN")
            require(not command("replacements", ["for-each-ref", "--format=%(refname)", "refs/replace"], metadata_deadline),
                    "REPLACEMENT")
            flags = command("index-flags", ["ls-files", "-v", "-z"], metadata_deadline)
            require(all(row.startswith(b"H ") for row in flags.split(b"\0") if row), "INDEX_FLAGS")
            tree = parse_tree(command("tree", ["ls-tree", "-rz", "--full-tree", commit], metadata_deadline))
            critical = []
            for name in critical_paths:
                require(name in tree, "CRITICAL_TRACKED_PATH")
                pin = physical_pin(repo, name, tree[name], metadata_deadline, critical=True)
                critical.append({"path": name, "bytes": pin["bytes"], "sha256": pin["sha256"]})
            require(time.monotonic() < metadata_deadline, "METADATA_DEADLINE")
            report["metadataCompletedUTC"] = utc()
            pins = {name: physical_pin(repo, name, entry, whole_deadline) for name, entry in tree.items()}
            for row in critical:
                require(pins[row["path"]]["bytes"] == row["bytes"] and pins[row["path"]]["sha256"] == row["sha256"],
                        "CRITICAL_CHANGED_DURING_FULL_SCAN")
            closing_deadline = min(whole_deadline, time.monotonic() + CLOSING_SECONDS)
            require(command("identity-after", ["rev-parse", "HEAD", "HEAD^{tree}"], closing_deadline).decode().splitlines()
                    == [commit, tree_id], "IDENTITY_CHANGED")
            require(not command("clean-after", ["status", "--porcelain=v1", "--untracked-files=all"], closing_deadline),
                    "CLEAN_CHANGED")
            require(not command("replacements-after", ["for-each-ref", "--format=%(refname)", "refs/replace"],
                                closing_deadline), "REPLACEMENT_CHANGED")
            require(command("index-flags-after", ["ls-files", "-v", "-z"], closing_deadline) == flags,
                    "INDEX_FLAGS_CHANGED")
            require(all(not path.exists() and not path.is_symlink() for path in redirect_paths), "OBJECT_REDIRECT_CHANGED")
            snapshot = {"commit": commit, "tree": tree_id, "completeTrackedSource": True, "files": pins,
                "criticalSource": {"commit": commit, "tree": tree_id, "sourceClean": True, "files": critical}}
            raw = canonical(snapshot)
            original(output / "snapshot.original.json", raw)
            report.update(status="SOURCE_VERIFIED", sourceFiles=len(pins), criticalFiles=len(critical),
                          sourceCleanBeforeAndAfter=True, snapshotSHA256=sha(raw),
                          completeTrackedSource=True, metadataProcessCount=5, closingProcessCount=4)
            return snapshot
    except BaseException as error:
        report.update(errorType=type(error).__name__, error=str(error)[:2048])
        raise
    finally:
        report["completedUTC"] = utc()
        record(output / "result.original.json", report)
