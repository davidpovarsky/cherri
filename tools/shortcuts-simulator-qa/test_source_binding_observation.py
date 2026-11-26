#!/usr/bin/env python3
"""Causal controls use owned tiny real Git repositories and actual child capture."""
import hashlib
import json
import os
from pathlib import Path
import signal
import stat
import subprocess
import tempfile
import time
import unittest
from unittest import mock

import source_binding_observation as subject


class SourceBindingObservationTests(unittest.TestCase):
    def setUp(self):
        retention = os.environ.get("CHERRI_SOURCE_BINDING_CONTROL_ARTIFACTS")
        if retention:
            base = Path(retention)
            self.assertTrue(base.is_absolute() and base.resolve(strict=True) == base and base.is_dir())
            self.root = base / self._testMethodName
            self.root.mkdir(mode=0o700)
        else:
            self.temporary = tempfile.TemporaryDirectory(prefix="cherri-source-binding-")
            self.addCleanup(self.temporary.cleanup)
            self.root = Path(self.temporary.name).resolve(strict=True)
        self.repo = self.root / "repo"
        self.repo.mkdir()
        self.git("init", "-q")
        self.git("config", "user.name", "Owned Fixture")
        self.git("config", "user.email", "owned-fixture@example.invalid")
        (self.repo / "source.py").write_bytes(b"print('original')\n")
        (self.repo / "binary").write_bytes(bytes(range(256)))
        (self.repo / "executable").write_bytes(b"#!/bin/sh\nexit 0\n")
        (self.repo / "executable").chmod(0o755)
        (self.repo / "link").symlink_to("source.py")
        self.git("add", ".")
        self.git("commit", "-qm", "owned initial fixture")
        self.commit = self.git("rev-parse", "HEAD").decode().strip()
        self.evidence = self.root / "originals"

    def git(self, *args):
        env = {k: v for k, v in os.environ.items() if not k.startswith("GIT_")}
        env.update(GIT_CONFIG_NOSYSTEM="1", GIT_CONFIG_GLOBAL="/dev/null", GIT_TERMINAL_PROMPT="0")
        return subprocess.check_output(["git", "--no-replace-objects", "-C", str(self.repo), *args],
            env=env, stderr=subprocess.PIPE, timeout=5)

    def observe(self, critical=("source.py",), output=None):
        return subject.source_inventory(self.repo, self.commit, output or self.evidence, critical)

    def receipt(self):
        return json.loads((self.evidence / "result.original.json").read_bytes())

    def rejected(self, code, critical=("source.py",)):
        with self.assertRaisesRegex(ValueError, code):
            self.observe(critical)
        result = self.receipt()
        self.assertEqual(result["status"], "SOURCE_UNVERIFIED")
        self.assertIs(result["nativeInvocation"], False)
        self.assertFalse((self.evidence / "snapshot.original.json").exists())
        return result

    def after_capture(self, event, action):
        original = subject.bounded.capture
        def run(*args, **kwargs):
            observed = original(*args, **kwargs)
            label = Path(kwargs["output"].name).parent.name
            if label == event:
                action()
            return observed
        return mock.patch.object(subject.bounded, "capture", run)

    def test_complete_real_git_tree_binary_symlink_and_executable(self):
        snapshot = self.observe()
        self.assertEqual(set(snapshot["files"]), {"source.py", "binary", "executable", "link"})
        self.assertEqual(snapshot["tree"], self.git("rev-parse", "HEAD^{tree}").decode().strip())
        self.assertTrue(snapshot["completeTrackedSource"])
        for name, pin in snapshot["files"].items():
            raw = self.git("show", self.commit + ":" + name)
            self.assertEqual(pin["bytes"], len(raw))
            self.assertEqual(pin["sha256"], hashlib.sha256(raw).hexdigest())
            self.assertEqual(pin["gitBlob"], self.git("rev-parse", self.commit + ":" + name).decode().strip())
        self.assertEqual(snapshot["files"]["executable"]["mode"], "100755")
        self.assertEqual(snapshot["files"]["link"]["mode"], "120000")
        result = self.receipt()
        self.assertEqual(result["status"], "SOURCE_VERIFIED")
        self.assertEqual((result["metadataProcessCount"], result["closingProcessCount"]), (5, 4))
        self.assertEqual((result["metadataSeconds"], result["closingSeconds"], result["wholeSeconds"]), (10, 15, 120))
        for row in result["processes"]:
            with self.subTest(process=row["label"]):
                process = row["process"]
                self.assertEqual(row["observation"]["exitCode"], 0)
                self.assertIs(process["cleanupParentReaped"], True)
                self.assertIs(process["groupPresentAfterCleanup"], False)
                for stream in ("stdout", "stderr"):
                    raw = (self.evidence / row["label"] / (stream + ".original")).read_bytes()
                    self.assertEqual(hashlib.sha256(raw).hexdigest(), process[stream]["sha256"])
                self.assertTrue((self.evidence / row["label"] / "process-started.original.json").is_file())
                self.assertTrue((self.evidence / row["label"] / "process-finished.original.json").is_file())

    def test_source_commit_mismatch_is_not_authenticated(self):
        self.commit = "0" * 40
        self.rejected("IDENTITY")

    def test_tracked_mutation_is_rejected(self):
        (self.repo / "source.py").write_bytes(b"print('mutated!')\n")
        self.rejected("CLEAN")

    def test_untracked_file_is_rejected(self):
        (self.repo / "unexpected").write_bytes(b"untracked")
        self.rejected("CLEAN")

    def test_assume_unchanged_cannot_hide_bytes(self):
        self.git("update-index", "--assume-unchanged", "binary")
        (self.repo / "binary").write_bytes(b"x" * 256)
        self.assertEqual(self.git("status", "--porcelain=v1", "--untracked-files=all"), b"")
        self.rejected("INDEX_FLAGS")

    def test_skip_worktree_cannot_hide_bytes(self):
        self.git("update-index", "--skip-worktree", "binary")
        (self.repo / "binary").write_bytes(b"x" * 256)
        self.assertEqual(self.git("status", "--porcelain=v1", "--untracked-files=all"), b"")
        self.rejected("INDEX_FLAGS")

    def test_replacement_ref_is_rejected(self):
        self.git("update-ref", "refs/replace/" + "0" * 40, self.commit)
        self.rejected("REPLACEMENT")

    def test_graft_file_even_empty_is_rejected(self):
        (self.repo / ".git/info/grafts").write_bytes(b"")
        self.rejected("OBJECT_REDIRECT")

    def test_alternate_file_even_empty_is_rejected(self):
        (self.repo / ".git/objects/info/alternates").write_bytes(b"")
        self.rejected("OBJECT_REDIRECT")

    def test_dangling_redirect_symlink_is_rejected(self):
        (self.repo / ".git/objects/info/alternates").symlink_to(self.root / "missing")
        self.rejected("OBJECT_REDIRECT")

    def test_executable_mode_ignoring_git_config_is_still_rejected(self):
        self.git("config", "core.filemode", "false")
        (self.repo / "executable").chmod(0o644)
        self.assertEqual(self.git("status", "--porcelain=v1", "--untracked-files=all"), b"")
        self.rejected("PHYSICAL_MODE")

    def test_critical_file_must_be_a_regular_tracked_file(self):
        self.rejected("CRITICAL_REGULAR_FILE", critical=("link",))

    def test_missing_critical_path_is_rejected(self):
        self.rejected("CRITICAL_TRACKED_PATH", critical=("absent",))

    def test_critical_size_bound_is_preserved(self):
        self.assertEqual(subject.MAX_CRITICAL_BYTES, 2 * 1024 * 1024)
        with mock.patch.object(subject, "MAX_CRITICAL_BYTES", 4):
            self.rejected("CRITICAL_SIZE")

    def test_actual_nonzero_git_child_keeps_original_failure(self):
        (self.repo / ".git/HEAD").unlink()
        result = self.rejected("PROCESS_identity")
        row = result["processes"][0]
        self.assertEqual(row["observation"]["result"], "EXIT_NONZERO")
        self.assertNotEqual(row["observation"]["exitCode"], 0)
        self.assertTrue((self.evidence / "identity/stderr.original").read_bytes())
        self.assertIs(row["process"]["cleanupParentReaped"], True)
        self.assertIs(row["process"]["groupPresentAfterCleanup"], False)

    def test_timeout_child_keeps_original_process_and_no_snapshot(self):
        original = subject.bounded.capture
        def delay_journal(*args, **kwargs):
            original_journal = kwargs["journal"]
            def journal(event, value):
                original_journal(event, value)
                if event == "started":
                    time.sleep(0.04)
            kwargs["journal"] = journal
            return original(*args, **kwargs)
        with mock.patch.object(subject, "METADATA_SECONDS", 0.02), mock.patch.object(subject.bounded, "capture", delay_journal):
            with self.assertRaises(ValueError):
                self.observe()
        result = self.receipt()
        row = result["processes"][0]
        self.assertEqual(result["status"], "SOURCE_UNVERIFIED")
        self.assertNotEqual(row["observation"]["result"], "PASS")
        self.assertIsInstance(row["process"]["pid"], int)
        self.assertTrue((self.evidence / "identity/process-finished.original.json").is_file())
        self.assertFalse((self.evidence / "snapshot.original.json").exists())

    def test_cancellation_at_real_spawn_keeps_finished_and_aggregate(self):
        original = subject.bounded.capture
        def interrupt_journal(*args, **kwargs):
            original_journal = kwargs["journal"]
            def journal(event, value):
                original_journal(event, value)
                if event == "started":
                    raise KeyboardInterrupt("authored cancellation after actual Git spawn")
            kwargs["journal"] = journal
            return original(*args, **kwargs)
        with mock.patch.object(subject.bounded, "capture", interrupt_journal):
            with self.assertRaisesRegex(KeyboardInterrupt, "authored cancellation"):
                self.observe()
        result = self.receipt()
        self.assertEqual(result["errorType"], "KeyboardInterrupt")
        row = result["processes"][0]
        self.assertIsNone(row["observation"])
        self.assertEqual(row["lastJournalEvent"], "finished")
        self.assertEqual(row["errorType"], "KeyboardInterrupt")
        finished = json.loads((self.evidence / "identity/process-finished.original.json").read_bytes())
        self.assertEqual(row["process"], finished)
        self.assertIsInstance(finished["pid"], int)
        self.assertIs(finished["cleanupParentReaped"], True)
        self.assertIs(finished["groupPresentAfterCleanup"], False)
        self.assertFalse((self.evidence / "snapshot.original.json").exists())

    def test_full_tree_bytes_after_clean_check_cannot_be_substituted(self):
        original_stat = (self.repo / "binary").stat()
        def mutate():
            (self.repo / "binary").write_bytes(b"x" * 256)
            os.utime(self.repo / "binary", ns=(original_stat.st_atime_ns, original_stat.st_mtime_ns))
        with self.after_capture("tree", mutate):
            self.rejected("PHYSICAL_GIT_BLOB")

    def test_symlink_parent_cannot_redirect_full_scan(self):
        (self.repo / "directory").mkdir()
        (self.repo / "directory/file").write_bytes(b"original child")
        self.git("add", ".")
        self.git("commit", "-qm", "owned directory fixture")
        self.commit = self.git("rev-parse", "HEAD").decode().strip()
        def redirect():
            (self.repo / "directory").rename(self.root / "outside")
            (self.repo / "directory").symlink_to(self.root / "outside", target_is_directory=True)
        with self.after_capture("tree", redirect):
            self.rejected("PHYSICAL_ANCESTOR")

    def test_closing_head_change_is_rejected(self):
        with self.after_capture("tree", lambda: self.git("commit", "--allow-empty", "-qm", "changed identity")):
            self.rejected("IDENTITY_CHANGED")

    def test_closing_untracked_change_is_rejected(self):
        with self.after_capture("identity-after", lambda: (self.repo / "late-file").write_bytes(b"late")):
            self.rejected("CLEAN_CHANGED")

    def test_closing_index_flags_cannot_hide_late_mutation(self):
        def hide():
            self.git("update-index", "--skip-worktree", "binary")
            (self.repo / "binary").write_bytes(b"x" * 256)
        with self.after_capture("identity-after", hide):
            self.rejected("INDEX_FLAGS_CHANGED")

    def test_closing_replacement_ref_is_rejected(self):
        with self.after_capture("identity-after", lambda: self.git("update-ref", "refs/replace/" + "0" * 40, self.commit)):
            self.rejected("REPLACEMENT_CHANGED")

    def test_closing_object_redirect_is_rejected(self):
        with self.after_capture("identity-after", lambda: (self.repo / ".git/info/grafts").write_bytes(b"")):
            self.rejected("OBJECT_REDIRECT_CHANGED")

    def test_source_mutation_between_critical_and_complete_scan_is_rejected(self):
        original = subject.physical_pin
        def change_after_pin(*args, **kwargs):
            value = original(*args, **kwargs)
            if kwargs.get("critical"):
                (self.repo / "source.py").write_bytes(b"print('modified')\n")
            return value
        with mock.patch.object(subject, "physical_pin", change_after_pin):
            self.rejected("PHYSICAL_GIT_BLOB")

    def test_physical_traversal_has_real_whole_deadline_and_receipt(self):
        original = subject.physical_pin
        physical_entered = []
        def slow(*args, **kwargs):
            if not kwargs.get("critical"):
                physical_entered.append(True)
                signal.setitimer(signal.ITIMER_REAL, 0.2)
                time.sleep(0.5)
            return original(*args, **kwargs)
        previous_handler = signal.getsignal(signal.SIGALRM)
        with mock.patch.object(subject, "physical_pin", slow):
            with self.assertRaisesRegex(TimeoutError, "WHOLE_DEADLINE"):
                self.observe()
        self.assertEqual(physical_entered, [True])
        self.assertIs(signal.getsignal(signal.SIGALRM), previous_handler)
        self.assertEqual(signal.getitimer(signal.ITIMER_REAL), (0.0, 0.0))
        self.assertEqual(self.receipt()["status"], "SOURCE_UNVERIFIED")

    def test_earlier_outer_alarm_runs_once_and_is_not_rearmed(self):
        previous_handler = signal.getsignal(signal.SIGALRM)
        previous_timer = signal.getitimer(signal.ITIMER_REAL)
        self.assertEqual(previous_timer, (0.0, 0.0))
        calls = []
        def outer(signum, frame):
            calls.append(signum)
            raise TimeoutError("AUTHORED_OUTER_DEADLINE")
        signal.signal(signal.SIGALRM, outer)
        signal.setitimer(signal.ITIMER_REAL, 0.02)
        try:
            with self.assertRaisesRegex(TimeoutError, "AUTHORED_OUTER_DEADLINE"):
                with subject.WholeDeadline():
                    time.sleep(0.2)
            time.sleep(0.005)
            self.assertEqual(calls, [signal.SIGALRM])
            self.assertEqual(signal.getitimer(signal.ITIMER_REAL), (0.0, 0.0))
            self.assertIs(signal.getsignal(signal.SIGALRM), outer)
        finally:
            signal.setitimer(signal.ITIMER_REAL, 0)
            signal.signal(signal.SIGALRM, previous_handler)

    def test_later_outer_alarm_restores_only_remaining_time(self):
        previous_handler = signal.getsignal(signal.SIGALRM)
        def outer(signum, frame):
            raise TimeoutError("not expected")
        signal.signal(signal.SIGALRM, outer)
        signal.setitimer(signal.ITIMER_REAL, 3)
        try:
            with mock.patch.object(subject, "WHOLE_SECONDS", 1):
                with subject.WholeDeadline():
                    time.sleep(0.01)
            remaining, interval = signal.getitimer(signal.ITIMER_REAL)
            self.assertGreater(remaining, 2)
            self.assertLess(remaining, 3)
            self.assertEqual(interval, 0)
            self.assertIs(signal.getsignal(signal.SIGALRM), outer)
        finally:
            signal.setitimer(signal.ITIMER_REAL, 0)
            signal.signal(signal.SIGALRM, previous_handler)

    def test_periodic_outer_alarm_is_rejected_without_modifying_it(self):
        previous_handler = signal.getsignal(signal.SIGALRM)
        def outer(signum, frame):
            raise TimeoutError("not expected")
        signal.signal(signal.SIGALRM, outer)
        signal.setitimer(signal.ITIMER_REAL, 3, 2)
        try:
            with self.assertRaisesRegex(ValueError, "OUTER_ALARM_CONTRACT"):
                with subject.WholeDeadline():
                    self.fail("unsupported periodic timer entered")
            remaining, interval = signal.getitimer(signal.ITIMER_REAL)
            self.assertGreater(remaining, 2)
            self.assertEqual(interval, 2)
            self.assertIs(signal.getsignal(signal.SIGALRM), outer)
        finally:
            signal.setitimer(signal.ITIMER_REAL, 0)
            signal.signal(signal.SIGALRM, previous_handler)

    def test_external_git_environment_is_stripped_without_values_in_receipts(self):
        secret = "synthetic-must-not-appear"
        with mock.patch.dict(os.environ, {"GIT_DIR": secret, "GIT_WORK_TREE": secret,
            "GIT_CONFIG_COUNT": "1", "GIT_CONFIG_KEY_0": "core.bare", "GIT_CONFIG_VALUE_0": "true"}):
            self.observe()
        result = (self.evidence / "result.original.json").read_text()
        self.assertNotIn(secret, result)
        self.assertIn("GIT_DIR", result)
        self.assertEqual(self.receipt()["status"], "SOURCE_VERIFIED")

    def test_before_after_originals_are_disjoint_and_collision_is_rejected(self):
        first = self.observe()
        original = (self.evidence / "result.original.json").read_bytes()
        with self.assertRaisesRegex(ValueError, "OUTPUT_CREATE_ONLY"):
            self.observe()
        self.assertEqual((self.evidence / "result.original.json").read_bytes(), original)
        second = self.observe(output=self.root / "after-originals")
        self.assertEqual(first, second)

    def test_symlink_output_and_repo_are_rejected_before_process(self):
        self.evidence.symlink_to(self.root / "missing")
        with self.assertRaisesRegex(ValueError, "OUTPUT_CREATE_ONLY"):
            self.observe()
        self.assertFalse((self.root / "missing").exists())
        alias = self.root / "alias"
        alias.symlink_to(self.repo, target_is_directory=True)
        with self.assertRaisesRegex(ValueError, "DIRECTORY"):
            subject.source_inventory(alias, self.commit, self.root / "no-output", ["source.py"])
        self.assertFalse((self.root / "no-output").exists())

    def test_tree_parser_rejects_ambiguous_or_foreign_shapes(self):
        blob = "a" * 40
        cases = {
            "duplicate": f"100644 blob {blob}\ta\0" * 2,
            "absolute": f"100644 blob {blob}\t/a\0",
            "escape": f"100644 blob {blob}\t../a\0",
            "submodule": f"160000 commit {blob}\tsubmodule\0",
            "unknown_mode": f"100600 blob {blob}\ta\0",
            "newline": f"100644 blob {blob}\ta\nb\0",
        }
        for name, raw in cases.items():
            with self.subTest(shape=name), self.assertRaises(ValueError):
                subject.parse_tree(raw.encode())


if __name__ == "__main__":
    unittest.main()
