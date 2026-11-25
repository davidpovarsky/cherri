"""Causal controller tests: no Go, Apple, signing or external service call."""
import copy
import hashlib
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import tempfile
import time
import unittest
from unittest import mock

import capture as p


class StrictEvents(unittest.TestCase):
    def events(self):
        return [
            {"Action": "start", "Package": "p"},
            {"Action": "run", "Package": "p", "Test": "TestA"},
            {"Action": "run", "Package": "p", "Test": "TestA/child"},
            {"Action": "pass", "Package": "p", "Test": "TestA/child"},
            {"Action": "pass", "Package": "p", "Test": "TestA"},
            {"Action": "pass", "Package": "p"},
        ]

    def raw(self, rows):
        return b"".join(p.canonical(row) for row in rows)

    def test_complete_exact_inventory(self):
        result = p.exact_events(self.raw(self.events()), {"p": ["TestA", "TestA/child"]})
        self.assertEqual(result["namedPassEvents"], 2)

    def test_missing_duplicate_skip_failure_foreign_and_order(self):
        for fault in ("missing", "duplicate", "skip", "fail", "foreign_package",
                      "foreign_test", "pass_before_run", "missing_package"):
            with self.subTest(fault=fault):
                rows = self.events()
                if fault == "missing": del rows[2]
                elif fault == "duplicate": rows.insert(3, dict(rows[3]))
                elif fault in ("skip", "fail"): rows[3]["Action"] = fault
                elif fault == "foreign_package": rows[3]["Package"] = "other"
                elif fault == "foreign_test": rows[3]["Test"] = "Other"
                elif fault == "pass_before_run": rows[1], rows[4] = rows[4], rows[1]
                elif fault == "missing_package": rows.pop()
                with self.assertRaises(p.Rejected):
                    p.exact_events(self.raw(rows), {"p": ["TestA", "TestA/child"]})

    def test_raw_framing_and_duplicate_json(self):
        original = self.raw(self.events())
        cases = [original[:-1], original+b"\n", b"\n"+original,
                 original.replace(b'"Action":"run"', b'"Action":"run","Action":"run"', 1)]
        for raw in cases:
            with self.subTest(raw_sha=p.sha(raw)), self.assertRaises(ValueError):
                p.exact_events(raw, {"p": ["TestA", "TestA/child"]})

    def test_selection_source_pin_is_not_relabelled(self):
        raw = (p.HERE/"selection.original.json").read_bytes()
        self.assertEqual(p.sha(raw), p.SELECTION_SHA)
        changed = json.loads(raw)
        changed["sourceCommit"] = "a"*40
        with self.assertRaisesRegex(p.Rejected, "SELECTION_PIN"):
            p.verify_selection({}, p.canonical(changed), b"[]", b"{}")

    def test_write_new_and_original_bytes(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp)/"original.json"
            original = b'{"a": 1} \n'
            p.write_new(path, original)
            self.assertEqual(p.read_regular(path), original)
            with self.assertRaises(FileExistsError):
                p.write_new(path, b"changed")
            self.assertEqual(path.read_bytes(), original)


class SourceWitness(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.root = Path(self.tmp.name).resolve()
        self.git("init", "--quiet")
        (self.root/"source.txt").write_bytes(b"original\n")
        self.git("add", "source.txt")
        self.git("-c", "user.name=Control", "-c", "user.email=control@example.invalid",
                 "commit", "--quiet", "-m", "synthetic controller source")
        self.head = self.git("rev-parse", "HEAD").decode().strip()

    def tearDown(self):
        self.tmp.cleanup()

    def git(self, *args):
        return subprocess.check_output(["git", "--no-replace-objects", "-C", str(self.root), *args],
                                       env=p.safe_env(), stderr=subprocess.DEVNULL)

    def test_complete_git_working_bytes(self):
        value = p.snapshot(self.root, self.head)
        self.assertTrue(value["sourceClean"])
        self.assertEqual(value["files"][0]["sha256"], hashlib.sha256(b"original\n").hexdigest())

    def test_wrong_commit_dirty_and_hidden_index_bytes(self):
        with self.assertRaisesRegex(p.Rejected, "SOURCE_HEAD"):
            p.snapshot(self.root, "f"*40)
        (self.root/"source.txt").write_bytes(b"changed\n")
        with self.assertRaisesRegex(p.Rejected, "SOURCE_DIRTY"):
            p.snapshot(self.root, self.head)
        self.git("update-index", "--assume-unchanged", "source.txt")
        with self.assertRaisesRegex(p.Rejected, "SOURCE_WORKING_BLOB"):
            p.snapshot(self.root, self.head)

    def test_replacements_and_alternates_rejected(self):
        self.git("update-ref", "refs/replace/"+self.head, self.head)
        with self.assertRaisesRegex(p.Rejected, "SOURCE_REPLACEMENTS"):
            p.snapshot(self.root, self.head)
        self.git("update-ref", "-d", "refs/replace/"+self.head)
        (self.root/".git/objects/info/alternates").write_text("/untrusted\n")
        with self.assertRaisesRegex(p.Rejected, "SOURCE_OBJECT_REDIRECTION"):
            p.snapshot(self.root, self.head)


@unittest.skipUnless(sys.platform == "linux", "Linux process-owner causal controls")
class OwnedProcess(unittest.TestCase):
    def capture(self, command, *, seconds=40, journal_failure=False):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        out = Path(tmp.name).resolve()
        real_record = p.record
        def journal(path, value):
            if journal_failure and path.name == "process-start.original.json":
                raise OSError("synthetic journal failure")
            return real_record(path, value)
        with mock.patch.object(p, "record", side_effect=journal):
            receipt = p.capture(command, out, dict(os.environ), out, time.monotonic()+seconds)
        return receipt, out

    def test_original_stdout_and_exit(self):
        receipt, out = self.capture([sys.executable, "-c", "import sys;sys.stdout.buffer.write(b'original\\n')"])
        self.assertEqual(receipt["exitCode"], 0)
        self.assertIsNone(receipt["reason"])
        self.assertTrue(receipt["ownedGroupAbsent"])
        self.assertEqual((out/"go-events.original.jsonl").read_bytes(), b"original\n")

    def test_timeout_cleans_only_own_group(self):
        foreign = subprocess.Popen([sys.executable, "-c", "import time;time.sleep(30)"],
                                   start_new_session=True)
        def finish_foreign():
            if foreign.poll() is None:
                foreign.kill()
            foreign.wait(timeout=3)
        self.addCleanup(finish_foreign)
        receipt, out = self.capture([sys.executable, "-c", "import time;time.sleep(60)"], seconds=25.5)
        self.assertEqual(receipt["reason"], "OWNER_PROCESS_DEADLINE")
        self.assertTrue(receipt["ownedGroupAbsent"])
        self.assertIsNone(foreign.poll())
        self.assertNotIn(foreign.pid, [r["pgid"] for r in receipt["ownedGroupsSignalled"]])
        foreign.kill()
        foreign.wait(timeout=3)

    def test_adopted_detached_descendant_is_owned_and_cleaned(self):
        code = ("import subprocess,sys;"
                "subprocess.Popen([sys.executable,'-c','import time;time.sleep(60)'],start_new_session=True)")
        receipt, out = self.capture([sys.executable, "-c", code])
        self.assertEqual(receipt["reason"], "OWNED_DESCENDANTS_REMAINED")
        self.assertTrue(receipt["ownedGroupAbsent"])
        self.assertGreaterEqual(len(receipt["observedOwnedProcesses"]), 2)

    def test_journal_failure_after_spawn_keeps_cleanup(self):
        receipt, out = self.capture([sys.executable, "-c", "import time;time.sleep(60)"],
                                    journal_failure=True)
        self.assertEqual(receipt["reason"], "CAPTURE_ERROR:OSError")
        self.assertTrue(receipt["ownedGroupAbsent"])
        self.assertTrue((out/"process-result.original.json").is_file())


if __name__ == "__main__":
    unittest.main(verbosity=2)
