#!/usr/bin/env python3
"""Real Git source-boundary controls; simulator output is never executed here."""
import hashlib
import json
import os
from pathlib import Path
import signal
import shutil
import subprocess
import tempfile
import time
import threading
import unittest
from unittest import mock

import git_source_batch as batch
import simulator_listapps_observation as app

REAL_POPEN = subprocess.Popen
REAL_RUN = subprocess.run
UDID = "11111111-2222-3333-4444-555555555555"


class ListAppsGitBatchTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory(prefix="cherri-listapps-git-")
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name).resolve(strict=True)
        self.repo = self.root / "repo"
        self.repo.mkdir()
        self.files = app.observation.QA_SOURCE_FILES
        for name in self.files:
            path = self.repo / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(("FIXTURE_CONTENT_CANARY " + name + "\n").encode())
        self.git("init", "-q")
        self.git("add", ".")
        self.commit()
        self.calls, self.processes = [], []
        self.addCleanup(self.reap)

    def tearDown(self):
        retained = os.environ.get("P15_LISTAPPS_GIT_TEST_ORIGINALS")
        if retained:
            destination = Path(retained) / self._testMethodName
            destination.mkdir(mode=0o700)
            for path in sorted(self.root.iterdir()):
                if path == self.repo or not path.is_dir() or path.is_symlink():
                    continue
                shutil.copytree(path, destination / path.name)

    def reap(self):
        for child in self.processes:
            if child.poll() is None:
                try:
                    os.killpg(child.pid, signal.SIGKILL)
                except ProcessLookupError:
                    pass
            child.wait(timeout=2)

    def git(self, *args, input=None):
        return REAL_RUN(["git", "--no-replace-objects", "-C", str(self.repo), *args],
                        input=input, check=True, capture_output=True, timeout=5).stdout

    def commit(self):
        self.git("-c", "user.name=Synthetic source control", "-c",
                 "user.email=synthetic@invalid.example", "commit", "-qm", "Pin synthetic inputs")
        self.source = self.git("rev-parse", "HEAD").decode().strip()

    def record(self, argv, *args, **kwargs):
        self.calls.append(list(argv))
        child = REAL_POPEN(argv, *args, **kwargs)
        self.processes.append(child)
        return child

    def probe(self, name="packet", **kwargs):
        identity = {"compilerCommitSha": self.source, "runId": "12345", "runAttempt": "1",
                    "fixtureId": "qa-short-circuit-v2"}
        return app.ListAppsObservation(self.repo, self.root / name, UDID, identity, **kwargs)

    def verify(self, probe=None):
        probe = probe or self.probe()
        with mock.patch.object(subprocess, "Popen", side_effect=self.record):
            self.assertEqual(probe.source("source-before"), self.source)
        return probe

    def original(self, probe):
        return json.loads((probe.private / "command-01-git-batch/result.original.json").read_bytes())

    def test_actual_listapps_source_uses_one_git_and_retains_both_head_observations(self):
        probe = self.verify()
        value = self.original(probe)
        self.assertEqual(self.calls, [list(batch.COMMAND)])
        self.assertEqual(value["headBefore"], self.source)
        self.assertEqual(value["headAfter"], self.source)
        self.assertEqual(value["status"], "SOURCE_VERIFIED")
        self.assertEqual([r["path"] for r in value["sourceFiles"]], list(self.files))
        self.assertTrue(value["gitTreeModesVerified"])
        self.assertFalse(value["completeTrackedSource"])
        self.assertFalse(value["nativeExecution"])
        self.assertFalse(value["formalAcceptanceGranted"])
        process = value["process"]
        self.assertEqual(process["pid"], self.processes[0].pid)
        self.assertEqual(process["exitCode"], 0)
        self.assertTrue(process["parentReaped"])
        self.assertEqual(process["groupAfterCleanup"]["state"], "ABSENT")
        self.assertEqual(probe.commands[0]["journalEvents"], ["started", "finished"])
        retained = probe.private / "command-01-git-batch"
        requests = (retained / "stdin.original").read_bytes().splitlines()
        self.assertEqual(requests[0], b"HEAD")
        self.assertEqual(requests[-1], b"HEAD")
        self.assertEqual(requests.count(b"HEAD"), 2)
        self.assertTrue(all(len(v) == 40 for v in requests[1:-1]))
        self.assertNotIn(b"FIXTURE_CONTENT_CANARY", (retained / "stdout.original").read_bytes())
        for event in ("started", "finished"):
            journal = json.loads((probe.private / ("command-01." + event + ".original.json")).read_bytes())
            self.assertEqual(journal["capture"]["pid"], process["pid"])

    def test_original_ten_second_stream_archive_and_per_file_limits_are_preserved(self):
        observed = []
        original = batch.verify_raw_sources
        def inspect(*args, **kwargs):
            observed.append(dict(kwargs))
            self.assertGreater(kwargs["deadline"] - time.monotonic(), 9)
            self.assertLessEqual(kwargs["deadline"] - time.monotonic(), 10)
            return original(*args, **kwargs)
        with mock.patch.object(batch, "verify_raw_sources", side_effect=inspect):
            self.verify()
        self.assertEqual(len(observed), 1)
        self.assertEqual(observed[0]["stdout_limit"], 1024 * 1024)
        self.assertEqual(observed[0]["file_limits"],
                         {p: app.observation.source_limit(p, qa=True) for p in self.files})
        self.assertEqual(app.STAGE_SECONDS, {"source-before": 10, "listapps": 60, "source-after": 10})
        self.assertEqual((app.PHASE_SECONDS, app.OUTER_SECONDS, app.ARCHIVE_LIMIT), (90, 120, 3 * 1024 * 1024))

    def test_actual_regular_git_modes_and_physical_uid_identity_are_retained(self):
        path = self.repo / self.files[0]
        path.chmod(0o755)
        self.git("add", "--", self.files[0])
        self.commit()
        value = self.original(self.verify())
        rows = {r["path"]: r for r in value["sourceFiles"]}
        self.assertEqual(rows[self.files[0]]["gitMode"], "100755")
        self.assertTrue(rows[self.files[0]]["executable"])
        self.assertEqual(rows[self.files[1]]["gitMode"], "100644")
        for name, row in rows.items():
            self.assertEqual(row["physicalUid"], (self.repo / name).stat().st_uid)
            self.assertEqual(row["physicalIdentity"], list(app.observation.file_identity((self.repo / name).stat())))
            self.assertEqual(row["actualGitBlob"], row["gitBlob"])

    def test_export_attributes_cannot_substitute_or_omit_raw_selected_bytes(self):
        name = self.files[0]
        path = self.repo / name
        path.write_bytes(b"FIXTURE_CONTENT_CANARY $Format:%H$\n")
        (self.repo / ".gitattributes").write_text(name + " export-ignore export-subst\n")
        self.git("add", ".")
        self.commit()
        value = self.original(self.verify())
        row = next(r for r in value["sourceFiles"] if r["path"] == name)
        self.assertEqual(row["sha256"], hashlib.sha256(path.read_bytes()).hexdigest())
        self.assertEqual(len(value["sourceFiles"]), len(self.files))

    def test_git_symlink_and_submodule_modes_reject_even_when_physical_file_is_regular(self):
        original = self.source
        name = self.files[0]
        blob = self.git("rev-parse", self.source + ":" + name).decode().strip()
        for index, (mode, oid) in enumerate((("120000", blob), ("160000", original))):
            with self.subTest(mode=mode):
                self.git("update-index", "--cacheinfo", mode + "," + oid + "," + name)
                self.commit()
                self.assertTrue((self.repo / name).is_file())
                self.assertFalse((self.repo / name).is_symlink())
                probe = self.probe("bad-mode-" + str(index))
                with self.assertRaisesRegex(ValueError, "SOURCE_INVENTORY"):
                    probe.source("source-before")
                self.assertFalse(self.original(probe)["nativeExecution"])
                self.git("update-index", "--cacheinfo", "100644," + blob + "," + name)
                self.commit()

    def test_missing_git_entry_cannot_be_satisfied_by_an_untracked_physical_file(self):
        name = self.files[0]
        self.git("rm", "--cached", "--", name)
        self.commit()
        probe = self.probe()
        with self.assertRaisesRegex(ValueError, "SOURCE_INVENTORY"):
            probe.source("source-before")
        self.assertTrue((self.repo / name).is_file())
        self.assertEqual(self.original(probe)["status"], "SOURCE_UNVERIFIED")

    def test_physical_mutation_is_retained_with_expected_and_actual_blob_then_blocks_native(self):
        name = "tools/shortcuts-simulator-qa/git_source_batch.py"
        path = self.repo / name
        path.write_bytes(path.read_bytes() + b"mutated in working tree\n")
        native = mock.Mock(side_effect=AssertionError("No native call permitted"))
        probe = self.probe(runner=native)
        self.assertFalse(probe.observe())
        native.assert_not_called()
        value = self.original(probe)
        row = next(r for r in value["sourceFiles"] if r["path"] == name)
        self.assertNotEqual(row["actualGitBlob"], row["gitBlob"])
        self.assertEqual(value["error"], "SOURCE_BYTES")
        self.assertEqual(value["process"]["groupAfterCleanup"]["state"], "ABSENT")
        source = json.loads((probe.private / "source-before.source.original.json").read_bytes())
        self.assertFalse(source["workingBytesMatched"])
        self.assertEqual(source["files"][-1]["actualBlob"], row["actualGitBlob"])

    def test_same_invocation_physical_mutation_after_first_read_is_rejected(self):
        original = batch.selected_tree_entry
        path = self.repo / self.files[0]
        def mutate(*args):
            result = original(*args)
            if args[2] == self.files[-1]:
                path.write_bytes(path.read_bytes() + b"mutation before closing physical read\n")
            return result
        with mock.patch.object(batch, "selected_tree_entry", side_effect=mutate):
            with self.assertRaisesRegex(ValueError, "SOURCE_PHYSICAL_CHANGED"):
                self.verify()

    def test_same_invocation_head_change_is_observed_only_after_closing_physical_reads(self):
        old = self.source
        self.git("-c", "user.name=Synthetic control", "-c", "user.email=synthetic@invalid.example",
                 "commit", "--allow-empty", "-qm", "Future fixture HEAD")
        other = self.git("rev-parse", "HEAD").decode().strip()
        self.git("update-ref", "HEAD", old, other)
        original = app.observation.bound_read
        seen = []
        def change(path, limit):
            result = original(path, limit)
            seen.append(str(Path(path).relative_to(self.repo)))
            if len(seen) == len(self.files) * 2:
                self.git("update-ref", "HEAD", other, old)
            return result
        with mock.patch.object(app.observation, "bound_read", side_effect=change):
            with self.assertRaisesRegex(ValueError, "CHECKOUT_SOURCE_MISMATCH"):
                self.verify()
        self.assertEqual(seen, list(self.files) * 2)

    def test_same_invocation_physical_mode_change_is_rejected(self):
        original = batch.selected_tree_entry
        path = self.repo / self.files[0]
        def mutate(*args):
            result = original(*args)
            if args[2] == self.files[-1]:
                path.chmod(0o755)
            return result
        with mock.patch.object(batch, "selected_tree_entry", side_effect=mutate):
            with self.assertRaisesRegex(ValueError, "SOURCE_PHYSICAL_CHANGED"):
                self.verify()

    def test_existing_ancestor_symlink_and_file_limits_remain_fail_closed(self):
        name = self.files[0]
        path = self.repo / name
        original = path.read_bytes()
        for label in ("file_symlink", "ancestor_symlink", "too_large"):
            with self.subTest(label=label):
                if label == "file_symlink":
                    stored = path.with_name("retained-file")
                    path.rename(stored)
                    path.symlink_to(stored)
                elif label == "ancestor_symlink":
                    parent = path.parent
                    stored = parent.with_name(parent.name + "-retained")
                    parent.rename(stored)
                    parent.symlink_to(stored, target_is_directory=True)
                else:
                    path.write_bytes(b"x" * (app.observation.source_limit(name, qa=True) + 1))
                try:
                    with self.assertRaises(ValueError):
                        self.verify(self.probe(label))
                finally:
                    if label == "file_symlink":
                        path.unlink(); stored.rename(path)
                    elif label == "ancestor_symlink":
                        parent.unlink(); stored.rename(parent)
                    else:
                        path.write_bytes(original)

    def test_redirects_are_rejected_before_any_git_process(self):
        for index, variable in enumerate(batch.REDIRECT_ENV):
            with self.subTest(variable=variable):
                with mock.patch.dict(os.environ, {variable: "synthetic-redirect"}):
                    with self.assertRaisesRegex(ValueError, "SOURCE_GIT_REDIRECT"):
                        self.verify(self.probe("redirect-" + str(index)))
        self.assertEqual(self.calls, [])

    def test_failed_spawn_retains_original_failure_without_fabricated_pid_or_journal(self):
        probe = self.probe()
        with mock.patch.object(subprocess, "Popen", side_effect=OSError("synthetic launch failure")):
            with self.assertRaises(OSError):
                probe.source("source-before")
        value = self.original(probe)
        self.assertEqual(value["error"], "OSError")
        self.assertIsNone(value["process"]["pid"])
        self.assertEqual(probe.commands[0]["journalEvents"], [])
        self.assertFalse(probe.commands[0]["sourceVerified"])

    def test_cancellation_after_actual_spawn_keeps_owned_cleanup_and_original_journals(self):
        probe = self.probe()
        original = batch.Batch.object
        def interrupt(instance, request, *args):
            if request == "HEAD":
                os.kill(os.getpid(), signal.SIGTERM)
            return original(instance, request, *args)
        with mock.patch.object(batch.Batch, "object", interrupt):
            self.assertFalse(probe.observe())
        value = self.original(probe)
        self.assertEqual(value["error"], "SourceInterrupted")
        self.assertTrue(value["process"]["parentReaped"])
        self.assertEqual(value["process"]["groupAfterCleanup"]["state"], "ABSENT")
        self.assertEqual(probe.commands[0]["result"], "CAPTURE_INTERRUPTED")
        self.assertEqual(probe.commands[0]["journalEvents"], ["started", "finished"])
        self.assertEqual(len(probe.commands), 1)

    def test_started_journal_failure_still_retains_and_cleans_the_actual_child(self):
        probe = self.probe()
        original = probe.write
        def fail(name, value):
            if name == "command-01.started.original.json":
                raise OSError("synthetic journal write failure")
            return original(name, value)
        with mock.patch.object(probe, "write", side_effect=fail):
            with self.assertRaises(OSError):
                probe.source("source-before")
        value = self.original(probe)
        self.assertIsNotNone(value["process"]["pid"])
        self.assertTrue(value["process"]["parentReaped"])
        self.assertEqual(value["process"]["groupAfterCleanup"]["state"], "ABSENT")
        self.assertEqual(value["error"], "OSError")

    def test_expired_source_budget_cannot_start_git_or_native(self):
        ticks = iter([0])
        native = mock.Mock(side_effect=AssertionError("No native call permitted"))
        probe = self.probe(now=lambda: next(ticks, 91), runner=native)
        with mock.patch.object(subprocess, "Popen", side_effect=self.record):
            self.assertFalse(probe.observe())
        self.assertEqual(self.calls, [])
        native.assert_not_called()
        self.assertEqual(probe.commands[0]["result"], "DEADLINE_EXHAUSTED")

    def test_source_reads_reuse_directory_handles_with_fresh_file_checks(self):
        # The original failure completed Git object reads but exhausted its
        # deadline resolving absolute ancestors for the 40th physical file.
        ancestors = {p for name in self.files for p in (self.repo / name).parents}
        absolute_ancestor_reads, file_reads, directory_fds = [], [], []
        original_stat = Path.lstat
        original_read = app.observation.bound_read
        original_open = os.open
        def stat_path(path, *args, **kwargs):
            if path in ancestors:
                absolute_ancestor_reads.append(str(path))
            return original_stat(path, *args, **kwargs)
        def read_file(path, limit, **kwargs):
            result = original_read(path, limit, **kwargs)
            file_reads.append(str(Path(path).relative_to(self.repo)))
            return result
        def open_file(path, flags, *args, **kwargs):
            fd = original_open(path, flags, *args, **kwargs)
            if flags & os.O_DIRECTORY:
                directory_fds.append(fd)
            return fd
        probe = self.probe()
        with mock.patch.object(Path, "lstat", stat_path), \
                mock.patch.object(app.observation, "bound_read", side_effect=read_file), \
                mock.patch.object(os, "open", side_effect=open_file):
            self.verify(probe)
        probe.write("physical-read-control.original.json", {
            "selectedFiles": len(self.files),
            "absoluteAncestorReads": len(absolute_ancestor_reads),
            "distinctAncestors": len(ancestors),
            "physicalFileReads": file_reads,
            "directoryHandleCount": len(directory_fds),
            "nativeExecution": False,
        })
        self.assertEqual(file_reads, list(self.files) * 2)
        self.assertLessEqual(len(absolute_ancestor_reads), 2 * len(ancestors),
            "fresh checks must not repeatedly resolve every full ancestor pathname")
        self.assertEqual(len(directory_fds), len(ancestors))
        for fd in directory_fds:
            with self.assertRaises(OSError):
                os.fstat(fd)
        self.assertEqual(self.original(probe)["status"], "SOURCE_VERIFIED")

    def test_failed_source_deadline_reaps_own_parent_without_authentication(self):
        # A real child remains live when the source decision fails. Delay only
        # the synthetic signal delivery, making the zero-wait race causal.
        original_object = batch.Batch.object
        actual_kill = app.observation.kill_group
        deliveries, timers = [], []
        expired = False
        def expire_after_head(instance, request, *args):
            nonlocal expired
            result = original_object(instance, request, *args)
            if request == "HEAD" and not expired:
                expired = True
                instance.deadline = time.monotonic() - .001
            return result
        def deliver_later(process):
            def deliver():
                deliveries.append({"pid": process.pid, "signal": actual_kill(process)})
            timer = threading.Timer(.05, deliver)
            timers.append(timer)
            timer.start()
            return {"state": "SENT", "signalSent": True, "errno": None}
        native = mock.Mock(side_effect=AssertionError("No native call permitted"))
        probe = self.probe(runner=native)
        try:
            with mock.patch.object(batch.Batch, "object", expire_after_head), \
                    mock.patch.object(app.observation, "kill_group", side_effect=deliver_later), \
                    mock.patch.object(subprocess, "Popen", side_effect=self.record):
                self.assertFalse(probe.observe())
        finally:
            for timer in timers:
                timer.join(timeout=2)
                self.assertFalse(timer.is_alive())
        native.assert_not_called()
        value = self.original(probe)
        probe.write("failure-reap-control.original.json", {
            "injection": "Expire the batch decision deadline after actual HEAD; delay owned SIGKILL delivery by 50ms",
            "actualDeliveries": deliveries,
            "process": value["process"],
            "nativeExecution": False,
        })
        self.assertEqual(value["error"], "SOURCE_GUARD_DEADLINE")
        self.assertEqual(value["status"], "SOURCE_UNVERIFIED")
        self.assertEqual(len(deliveries), 1)
        self.assertEqual(deliveries[0]["pid"], self.processes[0].pid)
        self.assertTrue(value["process"]["parentReaped"],
            "a failed acceptance deadline must still allow bounded owned-parent reaping")
        self.assertEqual(value["process"]["returnCodeObserved"], -signal.SIGKILL)
        self.assertEqual(value["process"]["groupAfterCleanup"]["state"], "ABSENT")
        self.assertEqual(probe.commands[0]["journalEvents"], ["started", "finished"])
        self.assertLess(value["elapsedMs"], 2000)
        self.assertFalse(value["nativeExecution"])
        self.assertFalse(value["formalAcceptanceGranted"])


class RawTreeParsingTests(unittest.TestCase):
    def test_exact_regular_tree_entries_and_typed_child_entries(self):
        raw = b"".join(mode + b" " + name + b"\0" + bytes.fromhex(oid) for mode, name, oid in (
            (b"100644", b"one.txt", "1" * 40), (b"100755", b"run.sh", "2" * 40),
            (b"40000", b"nested", "3" * 40), (b"120000", b"link", "4" * 40),
            (b"160000", b"submodule", "5" * 40)))
        parsed = batch.tree_entries(raw)
        self.assertEqual(parsed[b"one.txt"], ("100644", "1" * 40))
        self.assertEqual(parsed[b"run.sh"], ("100755", "2" * 40))
        self.assertEqual(len(parsed), 5)

    def test_truncated_ambiguous_and_malformed_tree_bytes_are_rejected(self):
        good = b"100644 one\0" + b"x" * 20
        cases = {"no_nul": b"100644 one", "short_oid": good[:-1], "trailing": good + b"x",
                 "duplicate": good + good, "invalid_mode": b"100600 one\0" + b"x" * 20,
                 "no_mode": b"one\0" + b"x" * 20, "empty_name": b"100644 \0" + b"x" * 20,
                 "slash": b"100644 a/b\0" + b"x" * 20, "dot": b"100644 .\0" + b"x" * 20,
                 "parent": b"100644 ..\0" + b"x" * 20,
                 "oversize_name": b"100644 " + b"x" * 4097 + b"\0" + b"x" * 20,
                 "oversize_tree": b"x" * (batch.TREE_LIMIT + 1)}
        for label, raw in cases.items():
            with self.subTest(case=label):
                with self.assertRaises(ValueError):
                    batch.tree_entries(raw)



if __name__ == "__main__":
    unittest.main()
