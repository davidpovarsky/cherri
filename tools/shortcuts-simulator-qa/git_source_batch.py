#!/usr/bin/env python3
"""Read selected raw Git objects through one bounded, owned batch process.

No archive filters, replacement objects, shell command, or second Git startup
participates. The caller supplies one absolute deadline, including physical
reads, the closing HEAD query, pipe drain and owned-process cleanup.
"""
import contextlib
import datetime
import hashlib
import json
import os
from pathlib import Path
import re
import selectors
import signal
import stat
import subprocess
import threading
import time

from simulator_session_recovery import defer_cancellation_signals, require

COMMAND = ("git", "--no-replace-objects", "cat-file", "--batch")
FILE_LIMIT = 4 * 1024 * 1024
COMMIT_LIMIT = 1024 * 1024
TREE_LIMIT = 1024 * 1024
STDERR_LIMIT = 64 * 1024
PATH_PATTERN = re.compile(r"[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)*")
HEADER = re.compile(rb"([0-9a-f]{40}) (commit|tree|blob) (0|[1-9][0-9]{0,9})\n")
REDIRECT_ENV = ("GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_INDEX_FILE",
                "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES",
                "GIT_REPLACE_REF_BASE", "GIT_NAMESPACE", "GIT_SHALLOW_FILE",
                "GIT_CONFIG", "GIT_CONFIG_COUNT", "GIT_CONFIG_PARAMETERS")


def digest(data):
    return hashlib.sha256(data).hexdigest()


def utc():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


def remaining(deadline):
    value = deadline - time.monotonic()
    require(value > 0, "SOURCE_GUARD_DEADLINE")
    return value


def physical(root, relative, deadline):
    remaining(deadline)
    require(isinstance(relative, str) and len(relative) <= 4096
            and PATH_PATTERN.fullmatch(relative), "ORIGINAL_PATH_INVALID")
    parts = relative.split("/")
    require(all(p not in {".", ".."} for p in parts), "ORIGINAL_PATH_INVALID")
    path = root.joinpath(*parts)
    require(all(not root.joinpath(*parts[:i]).is_symlink()
                for i in range(1, len(parts) + 1)), "ORIGINAL_SYMLINK")
    require(path.is_file(), "ORIGINAL_UNAVAILABLE")
    with os.fdopen(os.open(path, os.O_RDONLY | os.O_NOFOLLOW), "rb") as stream:
        before = os.fstat(stream.fileno())
        require(stat.S_ISREG(before.st_mode) and before.st_size <= FILE_LIMIT,
                "SOURCE_FILE_SIZE")
        data = stream.read(FILE_LIMIT + 1)
        after = os.fstat(stream.fileno())
    remaining(deadline)
    require(len(data) <= FILE_LIMIT, "SOURCE_FILE_SIZE")
    def identity(item):
        return (item.st_dev, item.st_ino, item.st_mode, item.st_size,
                item.st_mtime_ns, item.st_ctime_ns)
    require(identity(before) == identity(after) == identity(path.lstat()),
            "SOURCE_PHYSICAL_CHANGED")
    require(all(not root.joinpath(*parts[:i]).is_symlink()
                for i in range(1, len(parts) + 1)), "ORIGINAL_SYMLINK")
    return data, identity(after)


def repository_metadata(root, deadline):
    """Accept ordinary and linked checkouts without environment redirects."""
    remaining(deadline)
    require(not any(os.environ.get(key) for key in REDIRECT_ENV), "SOURCE_GIT_REDIRECT")
    pointer = root / ".git"
    require(not pointer.is_symlink(), "SOURCE_GIT_SYMLINK")
    witness = {}
    if pointer.is_dir():
        git_dir = pointer
    else:
        require(pointer.is_file() and pointer.stat().st_size <= 4096, "SOURCE_GIT_DIRECTORY")
        raw = pointer.read_bytes()
        require(raw.startswith(b"gitdir: ") and raw.endswith(b"\n") and raw.count(b"\n") == 1,
                "SOURCE_GIT_DIRECTORY")
        target = Path(os.fsdecode(raw[8:-1]))
        git_dir = (root / target).resolve(strict=True)
        witness["gitPointer"] = digest(raw)
    require(git_dir.is_dir() and not git_dir.is_symlink(), "SOURCE_GIT_DIRECTORY")
    common_pointer = git_dir / "commondir"
    common_dir = git_dir
    if common_pointer.exists() or common_pointer.is_symlink():
        require(common_pointer.is_file() and not common_pointer.is_symlink()
                and common_pointer.stat().st_size <= 4096, "SOURCE_GIT_DIRECTORY")
        raw = common_pointer.read_bytes()
        require(raw.endswith(b"\n") and raw.count(b"\n") == 1, "SOURCE_GIT_DIRECTORY")
        common_dir = (git_dir / os.fsdecode(raw[:-1])).resolve(strict=True)
        witness["commonPointer"] = digest(raw)
    require(common_dir.is_dir(), "SOURCE_GIT_DIRECTORY")
    for label, directory in (("git", git_dir), ("common", common_dir)):
        for relative in ("info/grafts", "objects/info/alternates"):
            path = directory / relative
            require(not path.is_symlink(), "SOURCE_GIT_REDIRECT")
            if path.exists():
                require(path.is_file() and path.stat().st_size == 0, "SOURCE_GIT_REDIRECT")
            witness[label + ":" + relative] = path.exists()
    remaining(deadline)
    return witness


class SourceInterrupted(BaseException):
    def __init__(self, signum):
        self.signum = signum


@contextlib.contextmanager
def cancellation_scope():
    # Preserve a caller's alarm timer. Only its handler is temporarily replaced;
    # Popen registration uses the already-qualified deferred-handler helper.
    require(threading.current_thread() is threading.main_thread(), "SOURCE_MAIN_THREAD")
    signals = [signal.SIGINT, signal.SIGTERM, signal.SIGALRM]
    old = {kind: signal.getsignal(kind) for kind in signals}
    def interrupted(kind, frame):
        raise SourceInterrupted(kind)
    try:
        for kind in signals:
            signal.signal(kind, interrupted)
        yield
    finally:
        for kind, handler in old.items():
            signal.signal(kind, handler)


class Batch:
    def __init__(self, root, deadline, stdout_limit):
        self.root, self.deadline = root, deadline
        self.limits = {"stdout": stdout_limit, "stderr": STDERR_LIMIT}
        self.streams = {"stdin": bytearray(), "stdout": bytearray(), "stderr": bytearray()}
        self.buffer = bytearray()
        self.eof = {"stdout": False, "stderr": False}
        self.process = None
        self.journal = None
        self.selector = selectors.DefaultSelector()
        self.record = {"argv": list(COMMAND), "startedAt": None, "pid": None,
                       "startNewSession": True, "exitCode": None,
                       "parentReaped": None, "groupBeforeCleanup": None,
                       "groupAfterCleanup": None, "cleanupSignal": None}

    def start(self):
        remaining(self.deadline)
        with defer_cancellation_signals():
            self.process = subprocess.Popen(list(COMMAND), cwd=self.root,
                stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                bufsize=0, start_new_session=True)
            self.record.update(pid=self.process.pid, startedAt=utc())
            if self.journal is not None:
                self.journal("started", dict(self.record))
        for name in ("stdin", "stdout", "stderr"):
            os.set_blocking(getattr(self.process, name).fileno(), False)
        for name in ("stdout", "stderr"):
            self.selector.register(getattr(self.process, name), selectors.EVENT_READ, name)

    def pump(self):
        wait = remaining(self.deadline)
        for key, _ in self.selector.select(min(wait, .1)):
            name = key.data
            if name == "stdin":
                continue
            take = min(65536, self.limits[name] + 1 - len(self.streams[name]))
            require(take > 0, "SOURCE_BATCH_OUTPUT_OVERFLOW")
            try:
                chunk = os.read(key.fileobj.fileno(), take)
            except BlockingIOError:
                continue
            if not chunk:
                self.eof[name] = True
                self.selector.unregister(key.fileobj)
                continue
            self.streams[name].extend(chunk)
            if name == "stdout":
                self.buffer.extend(chunk)
            require(len(self.streams[name]) <= self.limits[name], "SOURCE_BATCH_OUTPUT_OVERFLOW")
        remaining(self.deadline)

    def send(self, request):
        raw = request.encode("ascii") + b"\n"
        require(len(raw) <= 8192, "SOURCE_BATCH_REQUEST")
        offset = 0
        while offset < len(raw):
            remaining(self.deadline)
            try:
                count = os.write(self.process.stdin.fileno(), raw[offset:])
            except BlockingIOError:
                self.selector.register(self.process.stdin, selectors.EVENT_WRITE, "stdin")
                try:
                    self.pump()
                finally:
                    self.selector.unregister(self.process.stdin)
                continue
            require(count > 0, "SOURCE_BATCH_STDIN")
            self.streams["stdin"].extend(raw[offset:offset + count])
            offset += count

    def read(self, count):
        while len(self.buffer) < count:
            require(not self.eof["stdout"], "SOURCE_BATCH_TRUNCATED")
            self.pump()
        raw = bytes(self.buffer[:count])
        del self.buffer[:count]
        return raw

    def object(self, request, kind, maximum):
        require(not self.buffer, "SOURCE_BATCH_TRAILING")
        self.send(request)
        while b"\n" not in self.buffer:
            require(len(self.buffer) <= 128, "SOURCE_BATCH_HEADER")
            require(not self.eof["stdout"], "SOURCE_BATCH_TRUNCATED")
            self.pump()
        end = self.buffer.index(b"\n") + 1
        require(end <= 128, "SOURCE_BATCH_HEADER")
        header = self.read(end)
        match = HEADER.fullmatch(header)
        require(match is not None and match[2].decode() == kind, "SOURCE_BATCH_HEADER")
        size = int(match[3])
        require(size <= maximum, "SOURCE_BATCH_SIZE")
        raw = self.read(size)
        require(self.read(1) == b"\n", "SOURCE_BATCH_TERMINATOR")
        expected = hashlib.sha1(kind.encode() + b" " + str(size).encode() + b"\0" + raw).hexdigest()
        require(match[1].decode() == expected, "SOURCE_BATCH_OBJECT")
        return expected, raw

    def close_and_wait(self):
        self.process.stdin.close()
        while not all(self.eof.values()):
            self.pump()
            require(not self.buffer, "SOURCE_BATCH_TRAILING")
        require(not self.buffer, "SOURCE_BATCH_TRAILING")
        self.record["exitCode"] = self.process.wait(timeout=remaining(self.deadline))
        require(self.record["exitCode"] == 0, "SOURCE_BATCH_EXIT")
        remaining(self.deadline)

    def finish(self):
        # Reuse the existing qualified group observer; a reaped parent alone
        # cannot establish that its owned descendants are gone.
        if self.process is not None:
            from project_prerequisite_observation import group_observation, kill_group
            self.record["groupBeforeCleanup"] = group_observation(self.process.pid)
            self.record["cleanupSignal"] = kill_group(self.process)
            try:
                self.process.wait(timeout=max(0, self.deadline - time.monotonic()))
                self.record["parentReaped"] = True
            except subprocess.TimeoutExpired:
                self.record["parentReaped"] = False
            self.record["returnCodeObserved"] = self.process.returncode
            self.record["groupAfterCleanup"] = group_observation(self.process.pid)
            for name in ("stdin", "stdout", "stderr"):
                getattr(self.process, name).close()
        self.selector.close()
        self.record["finishedAt"] = utc()
        self.record["streams"] = {name: {"bytes": len(raw), "sha256": digest(raw),
            **({"eof": self.eof[name], "overflow": len(raw) > self.limits[name]} if name != "stdin" else {})}
            for name, raw in self.streams.items()}
        if self.journal is not None and self.process is not None:
            self.journal("finished", dict(self.record))

    def clean(self):
        return (self.record["parentReaped"] is True
                and self.record["groupBeforeCleanup"]["state"] == "ABSENT"
                and self.record["groupAfterCleanup"]["state"] == "ABSENT"
                and self.record["cleanupSignal"]["state"] in {"SENT", "ABSENT"})


def retain(originals, value, batch):
    if originals is None:
        return
    root = Path(originals)
    require(root.is_dir() and not root.is_symlink(), "SOURCE_ORIGINAL_DIRECTORY")
    for name, raw in batch.streams.items():
        descriptor = os.open(root / (name + ".original"), os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
        with os.fdopen(descriptor, "wb") as stream:
            stream.write(raw)
            stream.flush()
            os.fsync(stream.fileno())
    descriptor = os.open(root / "result.original.json", os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(descriptor, "wb") as stream:
        stream.write((json.dumps(value, sort_keys=True, indent=2) + "\n").encode())
        stream.flush()
        os.fsync(stream.fileno())


def tree_entries(raw):
    """Decode the exact bounded Git tree object, without archive attributes."""
    require(len(raw) <= TREE_LIMIT, "SOURCE_TREE_SIZE")
    offset, entries = 0, {}
    while offset < len(raw):
        end = raw.find(b"\0", offset)
        require(end >= 0 and end + 21 <= len(raw), "SOURCE_TREE_TRUNCATED")
        header = raw[offset:end]
        require(b" " in header, "SOURCE_TREE_ENTRY")
        mode, name = header.split(b" ", 1)
        require(mode in {b"40000", b"100644", b"100755", b"120000", b"160000"}
                and 0 < len(name) <= 4096 and b"/" not in name and name not in {b".", b".."}
                and name not in entries, "SOURCE_TREE_ENTRY")
        entries[name] = (mode.decode("ascii"), raw[end + 1:end + 21].hex())
        offset = end + 21
    return entries


def selected_tree_entry(batch, root_tree, relative, cache):
    require(PATH_PATTERN.fullmatch(relative) and all(x not in {".", ".."} for x in relative.split("/")),
            "ORIGINAL_PATH_INVALID")
    parts, tree = relative.split("/"), root_tree
    for index, part in enumerate(parts):
        if tree not in cache:
            require(len(cache) < 512, "SOURCE_TREE_COUNT")
            actual, raw = batch.object(tree, "tree", TREE_LIMIT)
            require(actual == tree, "SOURCE_TREE_OBJECT")
            cache[tree] = tree_entries(raw)
        entry = cache[tree].get(part.encode("ascii"))
        require(entry is not None, "SOURCE_INVENTORY")
        mode, digest = entry
        if index == len(parts) - 1:
            require(mode in {"100644", "100755"}, "SOURCE_INVENTORY")
            return mode, digest
        require(mode == "40000", "SOURCE_INVENTORY")
        tree = digest
    raise ValueError("SOURCE_INVENTORY")


def verify_raw_sources(repo, source, files, *, deadline, originals=None,
                       tree_inventory=False, file_limits=None, stdout_limit=None, journal=None):
    root = Path(repo).resolve(strict=True)
    require(os.name == "posix", "SOURCE_POSIX_REQUIRED")
    require(isinstance(source, str) and re.fullmatch(r"[0-9a-f]{40}", source), "SOURCE_INVALID")
    require(isinstance(files, (tuple, list)) and 0 < len(files) <= 512
            and len(set(files)) == len(files), "SOURCE_PATH_SET")
    require(type(tree_inventory) is bool, "SOURCE_TREE_MODE")
    require(stdout_limit is None or type(stdout_limit) is int and 0 < stdout_limit <= 4 * FILE_LIMIT,
            "SOURCE_BATCH_LIMIT")
    if tree_inventory:
        require(Path(repo).absolute() == root, "SOURCE_ROOT_ALIAS")
        require(type(file_limits) is dict and set(file_limits) == set(files)
                and all(type(x) is int and 0 < x <= FILE_LIMIT for x in file_limits.values()), "SOURCE_FILE_LIMITS")
        # Preserve listapps' original strict ancestor/read and per-file limits.
        from project_prerequisite_observation import bound_read
        def read_selected(relative):
            remaining(deadline)
            require(isinstance(relative, str) and len(relative) <= 4096 and PATH_PATTERN.fullmatch(relative)
                    and all(x not in {".", ".."} for x in relative.split("/")), "ORIGINAL_PATH_INVALID")
            raw, identity = bound_read(root / relative, file_limits[relative])
            remaining(deadline)
            return raw, tuple(identity)
    else:
        require(file_limits is None, "SOURCE_FILE_LIMITS")
        def read_selected(relative):
            return physical(root, relative, deadline)
    started = time.monotonic()
    # Upper bound preserves the previous per-file 4 MiB allowance. A header
    # may still reject an oversized individual object before reading its body.
    batch = Batch(root, deadline, stdout_limit if stdout_limit is not None else
                  len(files) * (FILE_LIMIT + 128) + 2 * (COMMIT_LIMIT + 128))
    batch.journal = journal
    value = {"schema": "cherri-selected-git-batch/1", "sourceCommitSha": source,
             "status": "SOURCE_UNVERIFIED", "completeTrackedSource": False,
             "sourceFiles": [], "error": None, "cleanupError": None, "process": batch.record,
             "nativeExecution": False, "formalAcceptanceGranted": False}
    error = None
    with cancellation_scope():
        try:
            metadata = repository_metadata(root, deadline)
            batch.start()
            initial, commit = batch.object("HEAD", "commit", COMMIT_LIMIT)
            require(initial == source, "CHECKOUT_SOURCE_MISMATCH")
            root_tree, trees = None, {}
            if tree_inventory:
                match = re.match(rb"tree ([0-9a-f]{40})\n", commit)
                require(match is not None, "SOURCE_COMMIT_TREE")
                root_tree = match[1].decode("ascii")
            observations = {}
            for relative in files:
                before, identity = read_selected(relative)
                mode = None
                if tree_inventory:
                    mode, blob = selected_tree_entry(batch, root_tree, relative, trees)
                    actual_blob = hashlib.sha1(b"blob " + str(len(before)).encode() + b"\0" + before).hexdigest()
                    value["sourceFiles"].append({"path": relative, "bytes": len(before),
                        "sha256": digest(before), "gitBlob": blob, "actualGitBlob": actual_blob,
                        "executable": bool(identity[2] & 0o111), "gitMode": mode,
                        "physicalIdentity": list(identity), "physicalUid": (root / relative).lstat().st_uid})
                    require(actual_blob == blob, "SOURCE_BYTES")
                    raw = before
                else:
                    blob, raw = batch.object(source + ":" + relative, "blob", FILE_LIMIT)
                    require(raw == before, "ACQUISITION_SOURCE_BYTES")
                    value["sourceFiles"].append({"path": relative, "bytes": len(raw),
                        "sha256": digest(raw), "gitBlob": blob, "executable": bool(identity[2] & 0o111)})
                observations[relative] = (digest(before), identity)
            for relative in files:
                after, identity = read_selected(relative)
                require((digest(after), identity) == observations[relative], "SOURCE_PHYSICAL_CHANGED")
            require(repository_metadata(root, deadline) == metadata, "SOURCE_GIT_METADATA_CHANGED")
            # Send this only after every physical closing read. Preloading all
            # stdin would allow Git to answer the closing HEAD too early.
            closing, _ = batch.object("HEAD", "commit", COMMIT_LIMIT)
            require(closing == initial == source, "CHECKOUT_SOURCE_MISMATCH")
            batch.close_and_wait()
            value.update(headBefore=initial, headAfter=closing)
            if tree_inventory:
                value.update(gitTreeModesVerified=True, physicalBinding="git-tree-blob-sha1",
                             treeObjectCount=len(trees))
        except BaseException as caught:
            error = caught
            value["error"] = (str(caught) if isinstance(caught, ValueError) else type(caught).__name__)
        finally:
            try:
                batch.finish()
            except BaseException as caught:
                # Cleanup observation is separate from the original setup,
                # launch or source failure. Keep both even if no child exists.
                value["cleanupError"] = (str(caught) if isinstance(caught, ValueError)
                                         else type(caught).__name__)
                if error is None:
                    error = caught
                    value["error"] = value["cleanupError"]
            if error is None:
                try:
                    require(batch.clean(), "SOURCE_BATCH_PROCESS_CLEANUP")
                    remaining(deadline)
                except BaseException as caught:
                    error = caught
                    value["error"] = str(caught) if isinstance(caught, ValueError) else type(caught).__name__
            if error is None:
                value["status"] = "SOURCE_VERIFIED"
            value["elapsedMs"] = round((time.monotonic() - started) * 1000)
            retain(originals, value, batch)
        if error is not None:
            raise error
        return value
