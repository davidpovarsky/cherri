"""Negative controls for the recorded native inventory admission, never native evidence."""
import copy
import os
from pathlib import Path
import tempfile
import unittest
from unittest import mock

import runtime_poc_preflight as preflight


class RuntimePreflightControls(unittest.TestCase):
    def setUp(self):
        self.before = {"valid": True, "head": "a" * 40, "tree": "b" * 40}
        self.after = dict(self.before)
        self.context = {"expectedSourceSha": "a" * 40}
        self.inventory = {
            "result": "PASS", "exitCode": 0,
            "capture": {"returnCodeObserved": 0, "cleanupParentReaped": True,
                        "groupPresentAfterCleanup": False,
                        "stdout": {"eof": True, "overflow": False},
                        "stderr": {"eof": True, "overflow": False}},
        }
        self.stdout = b"== Runtimes ==\niOS 27.0 (27.0 - 24A434) - com.apple.CoreSimulator.SimRuntime.iOS-27-0\n"

    def classify(self, system="Darwin"):
        return preflight.classify(system, self.before, self.after, self.context,
                                  self.inventory, self.stdout)

    def test_exact_native_receipt_is_ready(self):
        self.assertEqual(self.classify(), "READY")

    def test_host_platform_cannot_be_inferred_from_inventory_text(self):
        self.assertEqual(self.classify("Linux"), "BLOCKED_PLATFORM")

    def test_nonzero_original_exit_cannot_be_hidden_by_ios27_output(self):
        self.inventory["exitCode"] = 17
        self.assertEqual(self.classify(), "INVENTORY_FAILED")

    def test_late_zero_exit_and_timeout_are_not_success(self):
        for result in ("DEADLINE_EXHAUSTED", "TIMEOUT", "OUTPUT_OVERFLOW", "OWNED_GROUP_REMAINED"):
            with self.subTest(result=result):
                self.inventory["result"] = result
                self.assertEqual(self.classify(), "INVENTORY_FAILED")

    def test_boolean_exit_is_not_process_returncode_zero(self):
        self.inventory["exitCode"] = False
        self.assertEqual(self.classify(), "INVENTORY_FAILED")

    def test_original_streams_and_cleanup_must_be_complete(self):
        for field, value in (("cleanupParentReaped", False), ("groupPresentAfterCleanup", True),
                             ("returnCodeObserved", 9), ("returnCodeObserved", False)):
            with self.subTest(field=field):
                original = copy.deepcopy(self.inventory)
                self.inventory["capture"][field] = value
                self.assertEqual(self.classify(), "INVENTORY_CAPTURE_INCOMPLETE")
                self.inventory = original
        for stream in ("stdout", "stderr"):
            for field, value in (("eof", False), ("overflow", True)):
                with self.subTest(stream=stream, field=field):
                    original = copy.deepcopy(self.inventory)
                    self.inventory["capture"][stream][field] = value
                    self.assertEqual(self.classify(), "INVENTORY_CAPTURE_INCOMPLETE")
                    self.inventory = original

    def test_same_source_and_clean_checkout_are_required(self):
        self.after["head"] = "c" * 40
        self.assertEqual(self.classify(), "SOURCE_CHANGED")
        self.after = dict(self.before)
        self.after["tree"] = "c" * 40
        self.assertEqual(self.classify(), "SOURCE_CHANGED")
        self.after = dict(self.before)
        self.before["valid"] = False
        self.assertEqual(self.classify(), "SOURCE_INVALID")

    def test_github_context_must_match_actual_head(self):
        self.context["expectedSourceSha"] = "c" * 40
        self.assertEqual(self.classify(), "SOURCE_CONTEXT_MISMATCH")

    def test_text_marker_and_ios26_cannot_replace_actual_ios27_inventory(self):
        for data in (b"IOS27_SHORTCUTS_RUNTIME_POC=PASS\n", b"iOS 26.0 (26.0 - old)\n",
                     b"prefix iOS 27.0 (27.0 - misleading)\n"):
            self.stdout = data
            self.assertEqual(self.classify(), "IOS27_UNAVAILABLE")

    def test_local_context_never_claims_a_github_run(self):
        with mock.patch.dict(os.environ, {}, clear=True):
            value = preflight.context()
        self.assertEqual(value["provider"], "local")
        self.assertIsNone(value["runId"])
        self.assertIsNone(value["expectedSourceSha"])

    def test_incomplete_github_context_rejected_before_capture(self):
        with mock.patch.dict(os.environ, {"GITHUB_ACTIONS": "true"}, clear=True):
            with self.assertRaises(ValueError):
                preflight.context()

    def test_existing_output_is_never_overwritten(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory)
            marker = path / "receipt.json"
            marker.write_bytes(b"original")
            with self.assertRaises(FileExistsError):
                preflight.collect(path, path)
            self.assertEqual(marker.read_bytes(), b"original")


if __name__ == "__main__":
    unittest.main()
