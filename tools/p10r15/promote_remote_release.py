#!/usr/bin/env python3
"""Promote one fully verified Remote Builder tuple, exercise rollback, then reapply.

Only individual replacements are atomic. This is not an all-files transaction,
a power-loss guarantee, or a concurrent-reader barrier. Old immutable runtimes,
jobs, outputs, Pure, tools, the Legacy default and registered relative link stay
in place. A durable journal supports explicit recovery after interruption.
"""
import argparse
from contextlib import contextmanager
import ctypes
import fcntl
import json
import os
from pathlib import Path
import signal
import stat
import subprocess
import sys
import tempfile
import time

from package_skill import expected_package, verify_archive
from remote_release_gates import (PROTECTED, load_evidence, read_json, require, sha,
                                   validate_reports, validate_smoke)
from verify_remote_smoke import probe, scrubbed_env, status_matches
from verify_remote_release import execute_owned

SWITCH_ORDER = (PROTECTED[1], PROTECTED[2], PROTECTED[3], PROTECTED[0])
RESTORE_ORDER = (PROTECTED[0], PROTECTED[1], PROTECTED[2], PROTECTED[3])
LINK = "bin/cherri-remote-builder"
LINK_TARGET = "cherri-remote-legacy.sh"
CANONICAL = "https://github.com/davidpovarsky/cherri.git"
DOCS = "https://github.com/davidpovarsky/cherrilang.org.git"


def fsync_directory(path):
    descriptor = os.open(path, os.O_RDONLY | os.O_DIRECTORY)
    try:
        os.fsync(descriptor)
    finally:
        os.close(descriptor)


def regular_state(path):
    info = path.lstat()
    require(stat.S_ISREG(info.st_mode) and info.st_nlink == 1, "Expected an unshared regular file: " + str(path))
    return {"sha256": sha(path.read_bytes()), "bytes": info.st_size,
            "mode": stat.S_IMODE(info.st_mode), "uid": info.st_uid, "gid": info.st_gid}


def link_state(path):
    info = path.lstat()
    require(stat.S_ISLNK(info.st_mode), "Registered entrypoint is not a symlink")
    return {"target": os.readlink(path), "mode": stat.S_IMODE(info.st_mode),
            "uid": info.st_uid, "gid": info.st_gid}


def assert_state(path, expected):
    require(regular_state(path) == expected, "Concurrent or stale file state: " + str(path))


def durable_new(path, data, mode=0o600):
    descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, mode)
    with os.fdopen(descriptor, "wb") as output:
        output.write(data)
        output.flush()
        os.fsync(output.fileno())
    fsync_directory(path.parent)


def atomic_replace(path, data, expected, desired, hook=lambda point: None):
    """Compare immediately before replacement; never follow a destination link."""
    assert_state(path, expected)
    hook("before_temp")
    descriptor, temporary = tempfile.mkstemp(prefix=".p15-replace-", dir=path.parent)
    temporary = Path(temporary)
    try:
        with os.fdopen(descriptor, "wb") as output:
            output.write(data)
            os.fchmod(output.fileno(), desired["mode"])
            os.fchown(output.fileno(), desired["uid"], desired["gid"])
            output.flush()
            os.fsync(output.fileno())
        require(regular_state(temporary) == desired, "Prepared bytes/mode do not match journal")
        hook("after_temp_fsync")
        assert_state(path, expected)
        hook("before_replace")
        assert_state(path, expected)
        os.replace(temporary, path)
        hook("after_replace")
        fsync_directory(path.parent)
        assert_state(path, desired)
        hook("after_directory_fsync")
    finally:
        if temporary.exists():
            temporary.unlink()
            fsync_directory(path.parent)


def rename_absent(source, destination):
    """Linux no-replace rename closes the destination-existence race."""
    require(source.stat().st_dev == destination.parent.stat().st_dev, "Release move crosses filesystems")
    libc = ctypes.CDLL(None, use_errno=True)
    rename = getattr(libc, "renameat2", None)
    require(rename is not None, "Linux renameat2 is required for no-replace installation")
    rename.argtypes = (ctypes.c_int, ctypes.c_char_p, ctypes.c_int, ctypes.c_char_p, ctypes.c_uint)
    rename.restype = ctypes.c_int
    if rename(-100, os.fsencode(source), -100, os.fsencode(destination), 1) != 0:
        error = ctypes.get_errno()
        raise OSError(error, os.strerror(error), str(destination))
    fsync_directory(source.parent)
    fsync_directory(destination.parent)


@contextmanager
def deployment_lock(live):
    path = live.parent / (".cherri-deploy-" + sha(str(live).encode())[:16] + ".lock")
    descriptor = os.open(path, os.O_RDWR | os.O_CREAT | os.O_NOFOLLOW, 0o600)
    try:
        require(stat.S_ISREG(os.fstat(descriptor).st_mode), "Deployment lock is not a file")
        try:
            fcntl.flock(descriptor, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError as error:
            raise ValueError("Another deployment owns this live directory") from error
        yield
    finally:
        os.close(descriptor)


@contextmanager
def interruption_handlers():
    previous = {sig: signal.getsignal(sig) for sig in (signal.SIGINT, signal.SIGTERM)}
    def interrupted(signum, frame):
        raise InterruptedError("Promotion interrupted by signal " + str(signum))
    try:
        for sig in previous:
            signal.signal(sig, interrupted)
        yield
    finally:
        for sig, handler in previous.items():
            signal.signal(sig, handler)


def git(repo, *args):
    return subprocess.check_output(["git", "-C", str(repo), *args], text=True).strip()


def verify_checkout(repo, revision, origin):
    require((repo / ".git").is_dir() and not (repo / ".git").is_symlink(), "Release must contain an ordinary Git clone")
    require(not (repo / ".git/objects/info/alternates").exists(), "Borrowed Git object store is not an immutable release")
    require(git(repo, "rev-parse", "HEAD") == revision and not git(repo, "status", "--porcelain"), "Release source is dirty or stale")
    require(git(repo, "remote", "get-url", "origin") == origin, "Release origin is not the canonical repository")


def verify_runtime(home, release):
    label = release["label"]
    verify_checkout(home / "source" / label, release["skillSourceHead"], CANONICAL)
    runtime = home / label / "runtime"
    verify_checkout(runtime / "src", release["compilerCommit"], CANONICAL)
    verify_checkout(runtime / "docs", release["docsCommit"], DOCS)
    require(sha((runtime / "bin/cherri").read_bytes()) == release["compilerSHA256"], "Installed compiler bytes differ")
    require((runtime / "bin/cherri.commit").read_text().strip() == release["compilerCommit"], "Compiler commit sidecar differs")
    require((runtime / "bin/cherri.sha256").read_text().strip() == release["compilerSHA256"], "Compiler hash sidecar differs")


def preflight(stage, live, evidence_path):
    manifest, paths, reports = load_evidence(evidence_path, stage)
    candidate = stage / "remote-home"
    for one, other in ((stage, live), (candidate, live)):
        require(one != other and one not in other.parents and other not in one.parents, "Stage and live overlap")
    release = reports["stage"]["tuple"]
    skill = candidate / "source" / release["label"]
    skill_manifest = read_json(skill / "skills/cherri-shortcuts/compatibility-manifest.json")
    build_hash = sha((skill / "skills/cherri-shortcuts/scripts/build.sh").read_bytes())
    validate_reports(reports, candidate, skill_manifest, build_hash)
    require(read_json(candidate / "active.json") == release, "Candidate active tuple differs")
    require(link_state(live / LINK)["target"] == link_state(candidate / LINK)["target"] == LINK_TARGET,
            "Registered relative route differs")
    require((live / PROTECTED[4]).read_bytes().strip() == b"legacy"
            and (candidate / PROTECTED[4]).read_bytes() == (live / PROTECTED[4]).read_bytes(), "Legacy preference differs")
    require((candidate / "tools").is_symlink()
            and (candidate / "tools").resolve() == (live / "tools").resolve(), "Candidate toolchain link differs")
    for home, expected in ((live, reports["stage"]["liveHashesBefore"]),
                           (candidate, reports["stage"]["candidateHashes"])):
        require(all(sha((home / name).read_bytes()) == value for name, value in expected.items()), "Live/candidate evidence is stale")
    verify_runtime(candidate, release)
    for kind, archive_key, report_key, count in (("source", "sourceArchive", "sourceIntegrity", 20),
                                               ("runtime", "runtimeArchive", "runtimeIntegrity", 23)):
        files, receipt = expected_package(skill, release["skillSourceHead"], kind,
                    candidate / release["label"] / "runtime/bin/cherri", str(live / "tools/go/bin/go"))
        require(len(files) == count, "Unexpected committed package inventory")
        verify_archive(paths[archive_key], files)
        require(sha(paths[archive_key].read_bytes()) == reports[report_key]["archiveSha256"], "Archive receipt differs")
        require(receipt["runtime"] == reports[report_key]["runtime"], "Binary package provenance differs")
    require(not (live / "source" / release["label"]).exists() and not (live / release["label"]).exists(), "Release destination already exists")
    require(candidate.stat().st_dev == live.stat().st_dev, "Staged release is on another filesystem")
    return manifest, paths, reports


class Promotion:
    def __init__(self, journal, document, hook=None):
        self.journal, self.document = Path(journal), document
        self.live = Path(document["liveHome"])
        self.hook = hook or (lambda point: None)
        self.recovery_logging = False
        self.journal_error = None

    @classmethod
    def prepare(cls, stage, live, journal, evidence, hook=None):
        stage, live, journal = (Path(p).resolve() for p in (stage, live, journal))
        source_root = Path(__file__).resolve().parents[2]
        require(not journal.exists(), "Journal already exists; use explicit rollback/reapply")
        for protected in (stage, live, source_root):
            require(journal != protected and journal not in protected.parents and protected not in journal.parents,
                    "Journal must be outside source, stage and live")
        manifest, paths, reports = preflight(stage, live, evidence)
        require(all(sha(path.read_bytes()) == manifest["files"][name]["sha256"] for name, path in paths.items()),
                "Evidence changed during material verification")
        candidate = stage / "remote-home"
        before = {name: regular_state(live / name) for name in reports["stage"]["liveHashesBefore"]}
        after = {name: regular_state(candidate / name) for name in SWITCH_ORDER}
        require(all(record["sha256"] == reports["stage"]["liveHashesBefore"][name] for name, record in before.items())
                and all(record["sha256"] == reports["stage"]["candidateHashes"][name] for name, record in after.items()),
                "Live or candidate changed after gate verification")
        require(all(before[name]["uid"] == os.geteuid() for name in SWITCH_ORDER), "Live files have another owner")
        require(before[PROTECTED[4]]["mode"] == 0o600, "Unexpected Legacy preference permissions")
        require(all(after[name]["mode"] == (0o700 if name in SWITCH_ORDER[:2] else 0o600) for name in SWITCH_ORDER),
                "Unexpected candidate script/active permissions")
        # Replacement preserves ownership, while using reviewed candidate modes.
        for name in after:
            after[name].update(uid=before[name]["uid"], gid=before[name]["gid"])
        journal.mkdir(mode=0o700)
        fsync_directory(journal.parent)
        for group, home in (("before", live), ("after", candidate)):
            (journal / group).mkdir(mode=0o700)
            for index, name in enumerate(SWITCH_ORDER):
                raw = (home / name).read_bytes()
                expected = before[name] if group == "before" else after[name]
                require(sha(raw) == expected["sha256"], "Source changed while taking journal snapshot")
                durable_new(journal / group / str(index), raw)
        document = {"formatVersion": 1, "liveHome": str(live), "stage": str(stage),
                    "tuple": reports["stage"]["tuple"], "oldTuple": read_json(live / "active.json"),
                    "before": before, "after": after, "registeredLink": link_state(live / LINK),
                    "evidenceManifest": manifest, "evidencePaths": {k: str(v) for k, v in paths.items()},
                    "state": "PREPARED", "limitations": "Per-file atomic replacements only; no whole-transaction, power-loss or concurrent-reader guarantee."}
        durable_new(journal / "plan.json", (json.dumps(document, indent=2, sort_keys=True) + "\n").encode())
        transaction = cls(journal, document, hook)
        transaction.event("PREPARED")
        transaction.check_guard_files()
        for name in SWITCH_ORDER:
            assert_state(live / name, before[name])
        return transaction

    @classmethod
    def load(cls, journal, live, hook=None):
        journal, live = Path(journal).resolve(), Path(live).resolve()
        document = read_json(journal / "plan.json")
        require(document["formatVersion"] == 1 and Path(document["liveHome"]) == live, "Journal belongs to another live home")
        require(set(document["after"]) == set(SWITCH_ORDER) and set(PROTECTED) <= set(document["before"]), "Journal file inventory differs")
        require(document["registeredLink"]["target"] == LINK_TARGET, "Journal registered link differs")
        for group in ("before", "after"):
            for i, name in enumerate(SWITCH_ORDER):
                raw = (journal / group / str(i)).read_bytes()
                require(sha(raw) == document[group][name]["sha256"] and len(raw) == document[group][name]["bytes"], "Journal snapshot was modified")
        return cls(journal, document, hook)

    def event(self, event, **fields):
        row = {"event": event, "timeUtc": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()), **fields}
        path = self.journal / "events.jsonl"
        try:
            descriptor = os.open(path, os.O_WRONLY | os.O_APPEND | os.O_CREAT | os.O_NOFOLLOW, 0o600)
            with os.fdopen(descriptor, "ab") as output:
                output.write((json.dumps(row, sort_keys=True) + "\n").encode())
                output.flush(); os.fsync(output.fileno())
            fsync_directory(self.journal)
        except OSError as error:
            if not self.recovery_logging:
                raise
            # Logging failure must never prevent an attempted restoration from
            # the already durable snapshots. It still prevents success.
            self.journal_error = type(error).__name__ + ": " + str(error)

    def check_guard_files(self):
        require(link_state(self.live / LINK) == self.document["registeredLink"], "Registered relative link changed")
        for name, record in self.document["before"].items():
            if name not in SWITCH_ORDER:
                assert_state(self.live / name, record)

    def install_directories(self):
        candidate = Path(self.document["stage"]) / "remote-home"
        label = self.document["tuple"]["label"]
        self.check_guard_files()
        self.event("BEFORE_RELEASE_DIRECTORY")
        self.hook("install:before_directory")
        (self.live / label).mkdir(mode=0o700)
        fsync_directory(self.live)
        self.hook("install:after_directory")
        for relative in (Path("source") / label, Path(label) / "runtime"):
            self.event("BEFORE_MOVE", path=str(relative))
            self.hook("install:before_move:" + str(relative))
            rename_absent(candidate / relative, self.live / relative)
            self.hook("install:after_move:" + str(relative))
            self.event("AFTER_MOVE", path=str(relative))
        verify_runtime(self.live, self.document["tuple"])

    def switch(self, direction, phase):
        self.check_guard_files()
        order = SWITCH_ORDER if direction == "after" else RESTORE_ORDER
        for name in order:
            current = regular_state(self.live / name)
            old, new = self.document["before"][name], self.document["after"][name]
            require(current == old or current == new, "Unknown concurrent state; refusing to overwrite " + name)
            desired = self.document[direction][name]
            if current == desired:
                continue
            index = SWITCH_ORDER.index(name)
            data = (self.journal / direction / str(index)).read_bytes()
            require(sha(data) == desired["sha256"], "Journal bytes changed")
            self.event("BEFORE_FILE", phase=phase, path=name, direction=direction)
            atomic_replace(self.live / name, data, current, desired,
                           lambda point: self.hook(phase + ":" + name + ":" + point))
            self.event("AFTER_FILE", phase=phase, path=name, direction=direction)
            self.check_guard_files()
        require(all(regular_state(self.live / name) == self.document[direction][name] for name in SWITCH_ORDER), "Incomplete release switch")
        self.event("SWITCHED", phase=phase, direction=direction)

    def old_status(self, name):
        rows = []
        for dialect, route in (("v2", self.live / PROTECTED[1]), ("legacy", self.live / LINK)):
            rc, stdout, stderr = execute_owned([route, "status"], scrubbed_env(self.live, True), self.live, 75)
            ok = rc == 0 and status_matches(stdout, self.document["oldTuple"], dialect)
            rows.append({"route": dialect, "exitCode": rc, "passed": ok,
                         "stdoutSha256": sha(stdout.encode()), "stderrSha256": sha(stderr.encode())})
        durable_new(self.journal / (name + ".json"), (json.dumps(rows, indent=2) + "\n").encode())
        require(all(row["passed"] for row in rows), "Old registered/V2 status did not match exact old tuple")
        self.check_guard_files()
        self.event("OLD_STATUS_PASSED", report=name)

    def smoke(self, name):
        self.hook(name + ":before")
        report = probe(self.live, self.document["tuple"], self.journal / (name + ".json"), True)
        hashes = {key: value["sha256"] for key, value in self.document["before"].items()}
        hashes.update({key: value["sha256"] for key, value in self.document["after"].items()})
        validate_smoke(report, self.document["tuple"], self.live, hashes, True)
        self.check_guard_files()
        verify_runtime(self.live, self.document["tuple"])
        self.hook(name + ":after")
        self.event("SMOKE_PASSED", report=name)

    def rollback(self, status_name="rollback-status", phase="rollback"):
        self.switch("before", phase)
        self.old_status(status_name)
        self.event("ROLLED_BACK", retainedInactiveRuntime=True)

    def cycle(self):
        try:
            self.old_status("baseline-old-status")
            self.install_directories()
            self.switch("after", "activate")
            self.smoke("first-smoke")
            self.rollback()
            self.switch("after", "reapply")
            self.smoke("reapplied-smoke")
            self.event("COMPLETED", passed=True, actualRollbackAndReapply=True)
        except BaseException as error:
            self.recovery_logging = True
            self.event("FAILED", error=type(error).__name__ + ": " + str(error))
            self.hook = lambda point: None
            try:
                self.rollback("failure-rollback-status", "failure-rollback")
            except BaseException as restore_error:
                self.event("ROLLBACK_FAILED", error=type(restore_error).__name__ + ": " + str(restore_error), passed=False)
                raise RuntimeError("Promotion failed and exact rollback requires journal recovery") from restore_error
            self.event("FAILED_RESTORED", passed=False)
            raise


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="operation", required=True)
    for name in ("cycle", "rollback", "reapply"):
        command = sub.add_parser(name)
        command.add_argument("--live-home", type=Path, required=True)
        command.add_argument("--journal", type=Path, required=True)
        if name == "cycle":
            command.add_argument("--stage", type=Path, required=True)
            command.add_argument("--evidence-manifest", type=Path, required=True)
    args = parser.parse_args()
    live = args.live_home.resolve()
    try:
        with deployment_lock(live), interruption_handlers():
            if args.operation == "cycle":
                tx = Promotion.prepare(args.stage, live, args.journal, args.evidence_manifest)
                tx.cycle()
            else:
                tx = Promotion.load(args.journal, live)
                if args.operation == "rollback":
                    tx.recovery_logging = True
                    tx.rollback("manual-rollback-" + str(time.time_ns()), "manual-rollback")
                    require(tx.journal_error is None, "Old files restored, but journal recording failed: " + str(tx.journal_error))
                else:
                    for key, record in tx.document["evidenceManifest"]["files"].items():
                        require(sha(Path(tx.document["evidencePaths"][key]).read_bytes()) == record["sha256"], "Original evidence changed")
                    verify_runtime(live, tx.document["tuple"])
                    tx.old_status("before-manual-reapply-" + str(time.time_ns()))
                    try:
                        tx.switch("after", "manual-reapply")
                        tx.smoke("manual-reapply-smoke-" + str(time.time_ns()))
                        tx.event("REAPPLIED", passed=True, doesNotImplyFullCycle=True)
                    except BaseException:
                        tx.recovery_logging = True
                        tx.rollback("failed-manual-reapply-" + str(time.time_ns()), "failure-rollback")
                        raise
        print(json.dumps({"operation": args.operation, "passed": True, "journal": str(args.journal.resolve())}))
        return 0
    except BaseException as error:
        print(json.dumps({"operation": args.operation, "passed": False, "error": type(error).__name__ + ": " + str(error)}))
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
