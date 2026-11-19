"""Process controls for the canonical JXA gate; no test here executes Apple code."""
import copy
import json
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[3]
SCRIPT = ROOT / "tools/shortcut-runtime/jxa/verify_encodings.js"
TEMPLATE = ROOT / "tools/shortcut-runtime/required-encoding-expectations.json"
FIXTURE = Path(__file__).with_name("encoding_engine_fixture.js")
PARAMETERS = ROOT / "docs/apple-runtime/parameter-encodings.json"
TABLE = ROOT / "docs/apple-runtime/encoding-table.json"
HISTORICAL = ROOT / "docs/apple-runtime/encoding-roundtrips.json"
TARGET = "plainString.WFStringParameterState.string"


class RequiredEncodingGateTests(unittest.TestCase):
    def invoke(self, mode="normal", override=None, template=None, empty_discovery=False, missing_input=False):
        node = shutil.which("node")
        self.assertIsNotNone(node, "Node is required for process plumbing tests; missing runtime is not a pass")
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            output = root / "observed.json"
            params = PARAMETERS
            if empty_discovery:
                params = root / "empty-parameters.json"
                params.write_text('{"parameterClasses": {}}', encoding="utf-8")
            if missing_input:
                params = root / "missing-parameters.json"
            expected = TEMPLATE
            if template is not None:
                expected = root / "template.json"
                expected.write_text(json.dumps(template), encoding="utf-8")
            args = [str(params), str(TABLE), str(output), str(expected)]
            if override is not None:
                isolated = root / "isolated-expectations.json"
                isolated.write_text(json.dumps(override), encoding="utf-8")
                args.append(str(isolated))
            command = [node, str(FIXTURE), str(SCRIPT), str(HISTORICAL), mode, *args]
            result = subprocess.run(command, capture_output=True, text=True, encoding="utf-8", timeout=20)
            report = json.loads(output.read_bytes()) if output.exists() else None
            return result, report

    def expectations(self):
        return json.loads(TEMPLATE.read_bytes())

    def test_correct_set_reports_every_required_form_and_expected_rejection(self):
        result, report = self.invoke()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(report["status"], "PASS")
        expected = self.expectations()["REQUIRED"]
        self.assertEqual(len(expected), 23)
        self.assertEqual(len(report["requiredResults"]), len(expected))
        self.assertEqual({row["id"] for row in report["requiredResults"]}, {row["id"] for row in expected})
        self.assertTrue(all(row["passed"] for row in report["requiredResults"]))
        rejection = next(row for row in report["requiredResults"] if row["expected"] == "reject")
        self.assertEqual(rejection["stateClass"], "WFVariableParameterState")
        self.assertEqual(rejection["form"], "wrappedSubject")
        self.assertEqual(rejection["observation"], "rejected")
        self.assertEqual(report["failures"], [])
        self.assertEqual(set(report["forms"]), {"string", "number", "bool", "attachment(variable)",
                                               "attachment(output)", "tokenString", "wrappedSubject"})

    def test_required_native_rejection_is_nonzero_and_preserves_observation(self):
        result, report = self.invoke(mode="native_rejection")
        self.assertNotEqual(result.returncode, 0, "A failed REQUIRED EXPECT must not be warning-only")
        self.assertEqual(report["status"], "FAIL")
        self.assertEqual(report["failures"], [{"code": "E_REQUIRED_ENCODING_EXPECTATION",
                                               "id": TARGET, "expected": "accept", "actual": "reject"}])
        self.assertEqual(report["results"]["WFStringParameterState"]["string"], "rejected")
        self.assertIn("E_REQUIRED_ENCODING_EXPECTATION", result.stderr)

    def test_one_corrupt_expectation_fails_then_original_set_passes(self):
        changed = self.expectations()
        target = next(row for row in changed["REQUIRED"] if row["id"] == TARGET)
        self.assertEqual(target["expect"], "accept")
        target["expect"] = "reject"
        result, report = self.invoke(override=changed)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(report["status"], "FAIL")
        self.assertEqual(report["failures"], [{"code": "E_REQUIRED_ENCODING_EXPECTATION",
                                               "id": TARGET, "expected": "reject", "actual": "accept"}])
        restored, correct = self.invoke()
        self.assertEqual(restored.returncode, 0, restored.stderr)
        self.assertEqual(correct["status"], "PASS")

    def test_engine_failure_class_absence_and_exception_are_not_rejections(self):
        for mode in ("engine_failure", "class_missing", "native_exception"):
            with self.subTest(mode=mode):
                result, report = self.invoke(mode=mode)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(report["status"], "NOT_RUN")
                self.assertTrue(any(row["code"] == "E_REQUIRED_ENCODING_ENGINE" for row in report["failures"]))
        changed = self.expectations()
        next(row for row in changed["REQUIRED"] if row["id"] == TARGET)["expect"] = "reject"
        result, report = self.invoke(mode="native_exception", override=changed)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(report["status"], "NOT_RUN")

    def test_missing_input_and_no_discovery_cannot_pass(self):
        for kwargs in ({"missing_input": True}, {"empty_discovery": True}):
            with self.subTest(kwargs=kwargs):
                result, report = self.invoke(**kwargs)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(report["status"], "NOT_RUN")
                self.assertFalse(report["engineLoaded"])
                self.assertEqual(report["requiredResults"], [])

    def test_malformed_or_incomplete_expectations_cannot_remove_required_cases(self):
        empty = {"schemaVersion": 1, "REQUIRED": []}
        missing = self.expectations()
        missing["REQUIRED"].pop()
        duplicate = self.expectations()
        duplicate["REQUIRED"][-1] = copy.deepcopy(duplicate["REQUIRED"][0])
        changed_identity = self.expectations()
        changed_identity["REQUIRED"][0]["stateClass"] = "AnotherClass"
        invalid_outcome = self.expectations()
        invalid_outcome["REQUIRED"][0]["expect"] = "anything"
        for value in (empty, missing, duplicate, changed_identity, invalid_outcome):
            with self.subTest(value=value):
                result, report = self.invoke(override=value)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(report["status"], "NOT_RUN")
                self.assertEqual(report["failures"][0]["code"], "E_REQUIRED_ENCODING_INPUT")
        result, report = self.invoke(template=empty)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(report["status"], "NOT_RUN")


if __name__ == "__main__":
    unittest.main()
