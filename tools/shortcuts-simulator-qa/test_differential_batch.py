import copy
import importlib.util
from pathlib import Path
import unittest

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
                         "resultShape": {"utf8ByteCount": len(self.case["expected"].encode("utf-8"))}}
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
    def test_observation_without_oracle_never_passes(self):
        self.case["expected"] = None
        self.check(self.callback, "OBSERVED_ONLY")

if __name__ == "__main__":
    unittest.main()
