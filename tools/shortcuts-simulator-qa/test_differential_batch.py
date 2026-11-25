import contextlib
import copy
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import tempfile
import unittest
from unittest import mock
import urllib.parse

spec = importlib.util.spec_from_file_location("differential_batch", Path(__file__).with_name("run_differential_batch.py"))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)

class CallbackEvidenceTests(unittest.TestCase):
    def setUp(self):
        self.case = {"id": "unicode", "name": "CHERRI_QA_D30_V_01", "expected": "CHERRI_QA_01|שלום 🚀"}
        self.callback = {"runId": "fresh-nonce", "fixtureId": self.case["id"],
                         "expectedShortcutName": self.case["name"], "receiverSelfTest": False,
                         "eventSource": "shortcuts-x-callback", "status": "SUCCESS",
                         "resultSummary": self.case["expected"],
                         "resultShape": {"utf8ByteCount": len(self.case["expected"].encode("utf-8")),
                                         "utf8SHA256": hashlib.sha256(self.case["expected"].encode("utf-8")).hexdigest()}}
    def check(self, callback, expected):
        self.assertEqual(module.classify_callback(callback, self.case, "fresh-nonce"), expected)
    def test_exact_unicode_callback(self):
        self.check(self.callback, "PASS")
    def test_absent_callback(self):
        self.check(None, "NO_CALLBACK")
    def test_identity_corruption(self):
        for key, value in [("runId", "stale"), ("fixtureId", "other"), ("expectedShortcutName", "other"),
                           ("receiverSelfTest", True), ("receiverSelfTest", None), ("eventSource", "self-test")]:
            with self.subTest(field=key, value=value):
                bad = copy.deepcopy(self.callback)
                bad[key] = value
                self.check(bad, "INVALID_CALLBACK_IDENTITY")
    def test_native_error_with_expected_text_cannot_pass(self):
        bad = copy.deepcopy(self.callback)
        bad["status"] = "ERROR"
        self.check(bad, "RUNTIME_ERROR")
    def test_changed_output_cannot_pass(self):
        bad = copy.deepcopy(self.callback)
        bad["resultSummary"] += "!"
        self.check(bad, "OUTPUT_MISMATCH")
    def test_trimmed_extra_whitespace_cannot_pass(self):
        bad = copy.deepcopy(self.callback)
        bad["resultShape"]["utf8ByteCount"] += 1
        self.check(bad, "RAW_OUTPUT_LENGTH_MISMATCH")
    def test_missing_raw_shape_cannot_pass(self):
        bad = copy.deepcopy(self.callback)
        del bad["resultShape"]
        self.check(bad, "RAW_OUTPUT_LENGTH_MISMATCH")
    def test_character_count_is_not_utf8_byte_count(self):
        bad = copy.deepcopy(self.callback)
        bad["resultShape"]["utf8ByteCount"] = len(self.case["expected"])
        self.check(bad, "RAW_OUTPUT_LENGTH_MISMATCH")
    def test_missing_raw_digest_cannot_pass(self):
        bad = copy.deepcopy(self.callback)
        del bad["resultShape"]["utf8SHA256"]
        self.check(bad, "RAW_OUTPUT_DIGEST_MISMATCH")
    def test_changed_raw_digest_cannot_pass(self):
        bad = copy.deepcopy(self.callback)
        bad["resultShape"]["utf8SHA256"] = "0" * 64
        self.check(bad, "RAW_OUTPUT_DIGEST_MISMATCH")
    def test_equal_byte_count_cr_lf_substitution_cannot_pass(self):
        self.case["expected"] = "CHERRI_QA_LINE1\nשלום 🚀"
        raw = self.case["expected"].replace("\n", "\r")
        self.callback["resultSummary"] = self.case["expected"]
        self.callback["resultShape"] = {
            "utf8ByteCount": len(raw.encode("utf-8")),
            "utf8SHA256": hashlib.sha256(raw.encode("utf-8")).hexdigest(),
        }
        self.assertEqual(len(raw.encode("utf-8")), len(self.case["expected"].encode("utf-8")))
        self.check(self.callback, "RAW_OUTPUT_DIGEST_MISMATCH")
    def test_exact_multiline_unicode_digest(self):
        self.case["expected"] = "CHERRI_QA_LINE1\nשלום 🚀"
        self.callback["resultSummary"] = self.case["expected"]
        self.callback["resultShape"] = {
            "utf8ByteCount": len(self.case["expected"].encode("utf-8")),
            "utf8SHA256": hashlib.sha256(self.case["expected"].encode("utf-8")).hexdigest(),
        }
        self.check(self.callback, "PASS")
    def test_observation_without_oracle_never_passes(self):
        self.case["expected"] = None
        self.check(self.callback, "OBSERVED_ONLY")

class ImportRestartTests(unittest.TestCase):
    def run_case(self, restart=False, import_outcome="IMPORTED", restart_error=False):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            signed = root / "synthetic.shortcut"
            signed.write_bytes(b"AEA1synthetic-host-control-input")
            case = {"id": "unicode", "lane": "fork_v2", "name": "CHERRI_QA_RESTART",
                    "expected": "CHERRI_QA_01|שלום 🚀", "signedPath": str(signed),
                    "signedSHA256": hashlib.sha256(signed.read_bytes()).hexdigest()}
            manifest_path = root / "manifest.json"
            manifest_path.write_text(json.dumps({"cases": [case]}))
            callbacks = root / "receiver" / "Documents" / "callback-events.json"
            observed = []
            calls = []
            real_load_json = module.load_json

            class Process:
                pid = 123456
                returncode = None
                def poll(self):
                    return self.returncode
                def wait(self, timeout):
                    self.returncode = 0
                    return 0

            def invoke(argv, timeout=25):
                calls.append((argv, timeout))
                if argv[2] == "get_app_container":
                    return str(root / "receiver")
                if argv[2] == "terminate":
                    if restart_error:
                        raise RuntimeError("synthetic termination failure")
                    return ""
                if argv[2] == "openurl" and argv[-1].startswith("cherriqa://trigger?"):
                    trigger = urllib.parse.parse_qs(urllib.parse.urlsplit(argv[-1]).query)
                    target = urllib.parse.parse_qs(urllib.parse.urlsplit(trigger["target"][0]).query)
                    success = urllib.parse.parse_qs(urllib.parse.urlsplit(target["x-success"][0]).query)
                    observed.append({"runId": success["runId"][0], "fixtureId": case["id"],
                                     "expectedShortcutName": case["name"], "receiverSelfTest": False,
                                     "eventSource": "shortcuts-x-callback", "status": "SUCCESS",
                                     "resultSummary": case["expected"],
                                     "resultShape": {"utf8ByteCount": len(case["expected"].encode("utf-8")),
                                                     "utf8SHA256": hashlib.sha256(case["expected"].encode("utf-8")).hexdigest()}})
                return ""

            def load(path, default=None):
                return observed if path == callbacks else real_load_json(path, default)

            def wait(path, seconds, process):
                if path.name.startswith("ack-import-"):
                    return {"outcome": import_outcome, "shortcutName": case["name"]}
                return {"outcome": "READY"}

            with mock.patch.object(module.subprocess, "Popen", return_value=Process()), \
                 mock.patch.object(module, "command", side_effect=invoke), \
                 mock.patch.object(module, "load_json", side_effect=load), \
                 mock.patch.object(module, "wait_file", side_effect=wait), \
                 contextlib.redirect_stdout(io.StringIO()):
                result = module.run(manifest_path, root / "out", "synthetic-simulator",
                                    root / "harness", root / "derived",
                                    restart_shortcuts_after_import=restart)
            return {"exit": result, "calls": calls,
                    "summary": json.loads((root / "out/summary.json").read_text()),
                    "rows": json.loads((root / "out/results.json").read_text()),
                    "restart_receipts": [json.loads(p.read_text()) for p in (root / "out").glob("restart-*.json")]}

    def test_restart_is_explicit_and_targets_only_selected_shortcuts_app(self):
        result = self.run_case(restart=True)
        self.assertEqual(result["exit"], 0)
        self.assertEqual(result["calls"][1][0], ["xcrun", "simctl", "launch", "--terminate-running-process",
                                                "synthetic-simulator", "org.cherrilang.qa.Receiver"])
        operations = [argv[2] for argv, _ in result["calls"]]
        self.assertEqual(operations, ["install", "launch", "get_app_container", "openurl", "terminate", "openurl"])
        self.assertEqual(result["calls"][4][0],
                         ["xcrun", "simctl", "terminate", "synthetic-simulator", "com.apple.shortcuts"])
        self.assertEqual(result["calls"][-1][1], 60)
        self.assertTrue(result["summary"]["restartShortcutsAfterImport"])
        self.assertTrue(result["rows"][0]["restartShortcutsAfterImport"])
        self.assertEqual(result["rows"][0]["restartReceipt"]["outcome"], "TERMINATED")
        self.assertEqual(len(result["restart_receipts"]), 1)
        self.assertEqual(result["restart_receipts"][0], result["rows"][0]["restartReceipt"])

    def test_default_never_terminates_shortcuts_app(self):
        result = self.run_case()
        self.assertEqual(result["exit"], 0)
        self.assertNotIn("terminate", [argv[2] for argv, _ in result["calls"]])
        self.assertFalse(result["rows"][0]["restartShortcutsAfterImport"])
        self.assertEqual(result["restart_receipts"], [])

    def test_unverified_import_never_terminates_or_runs(self):
        result = self.run_case(restart=True, import_outcome="IMPORT_NOT_FOUND")
        self.assertEqual(result["exit"], 1)
        self.assertEqual([argv[2] for argv, _ in result["calls"]], ["install", "launch", "get_app_container", "openurl"])
        self.assertEqual(result["rows"][0]["status"], "IMPORT_FAILED")
        self.assertEqual(result["restart_receipts"], [])

    def test_failed_termination_is_retained_without_runtime_fallback(self):
        result = self.run_case(restart=True, restart_error=True)
        self.assertEqual(result["exit"], 1)
        self.assertEqual([argv[2] for argv, _ in result["calls"]], ["install", "launch", "get_app_container", "openurl", "terminate"])
        self.assertEqual(result["rows"][0]["status"], "SHORTCUTS_RESTART_FAILED")
        self.assertEqual(result["summary"]["fatal"], "synthetic termination failure")
        self.assertEqual(result["restart_receipts"][0]["outcome"], "FAILED")

if __name__ == "__main__":
    unittest.main()
