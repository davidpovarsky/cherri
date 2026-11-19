"""Protocol controls only: synthetic Node reports never mint Apple-engine evidence."""
import copy
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

import test_required_encoding_gate as fixture_tests

ROOT = Path(__file__).resolve().parents[3]
PRODUCER = ROOT / "tools/shortcut-runtime/required_encoding_gate.py"
spec = importlib.util.spec_from_file_location("required_encoding_gate", PRODUCER)
gate = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gate)


class RequiredEncodingProducerTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        fixture = fixture_tests.RequiredEncodingGateTests()
        cls.expected = fixture.expectations()["REQUIRED"]
        positive_process, cls.positive = fixture.invoke()
        if positive_process.returncode != 0:
            raise AssertionError(positive_process.stderr)
        cls.changed = copy.deepcopy(cls.expected)
        next(row for row in cls.changed if row["id"] == gate.NEGATIVE_ID)["expect"] = "reject"
        negative_process, cls.negative = fixture.invoke(override={"schemaVersion": 1, "REQUIRED": cls.changed})
        if negative_process.returncode != 1:
            raise AssertionError("The actual gate child process did not reject the isolated EXPECT")
        cls.rejected_process = {"exitCode": 1, "negativeDiagnosticObserved": True}

    def test_complete_triplet_protocol_accepts_the_exact_control(self):
        gate.validate_phase({"exitCode": 0}, self.positive, self.expected)
        gate.validate_phase(self.rejected_process, self.negative, self.changed, negative=True)
        gate.validate_phase({"exitCode": 0}, self.positive, self.expected)

    def test_missing_or_contradictory_actual_observations_are_rejected(self):
        cases = {}
        value = copy.deepcopy(self.positive)
        value["forms"].pop()
        cases["missing_form"] = value
        value = copy.deepcopy(self.positive)
        value["requiredResults"].pop()
        cases["missing_required_case"] = value
        value = copy.deepcopy(self.positive)
        value["requiredResults"][-1]["id"] = value["requiredResults"][0]["id"]
        cases["duplicate_identity"] = value
        value = copy.deepcopy(self.positive)
        value["requiredResults"][0]["passed"] = False
        cases["contradictory_verdict"] = value
        value = copy.deepcopy(self.positive)
        value["results"]["WFStringParameterState"]["string"] = "rejected"
        cases["matrix_verdict_disagreement"] = value
        value = copy.deepcopy(self.positive)
        value["results"]["WFStringParameterState"].pop("bool")
        cases["missing_nonrequired_per_form_observation"] = value
        value = copy.deepcopy(self.positive)
        value["requiredExpectations"][0]["expect"] = "reject"
        cases["wrong_applied_expectation"] = value
        value = copy.deepcopy(self.positive)
        value["discovery"]["uniqueStateClasses"] += 1
        cases["wrong_discovery_count"] = value
        value = copy.deepcopy(self.positive)
        value["discovery"]["parameterStateClasses"] = 0
        cases["zero_discovery"] = value
        value = copy.deepcopy(self.positive)
        value["engineLoaded"] = False
        cases["no_engine"] = value
        value = copy.deepcopy(self.positive)
        value["failures"] = [{"code": "hidden failure"}]
        cases["contradictory_failures"] = value
        value = copy.deepcopy(self.positive)
        value["status"] = "NOT_RUN"
        cases["wrong_status"] = value
        for name, value in cases.items():
            with self.subTest(name=name), self.assertRaises(gate.GateFailure):
                gate.validate_phase({"exitCode": 0}, value, self.expected)

    def test_generic_failure_timeout_signal_or_missing_diagnostic_is_not_negative_pass(self):
        for exit_code in (0, 2, 124, -9, None, True):
            with self.subTest(exit_code=exit_code), self.assertRaises(gate.GateFailure):
                gate.validate_phase({"exitCode": exit_code, "negativeDiagnosticObserved": True},
                                    self.negative, self.changed, negative=True)
        with self.assertRaises(gate.GateFailure):
            gate.validate_phase({"exitCode": 1, "negativeDiagnosticObserved": False},
                                self.negative, self.changed, negative=True)
        value = copy.deepcopy(self.negative)
        value["requiredResults"][0]["actual"] = "unavailable"
        with self.assertRaises(gate.GateFailure):
            gate.validate_phase(self.rejected_process, value, self.changed, negative=True)
        value = copy.deepcopy(self.negative)
        value["status"] = "NOT_RUN"
        with self.assertRaises(gate.GateFailure):
            gate.validate_phase(self.rejected_process, value, self.changed, negative=True)

    def test_unavailable_cli_never_emits_formal_evidence_even_with_ci_environment(self):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "fresh"
            env = dict(os.environ, GITHUB_ACTIONS="true", GITHUB_SHA="not-a-source-sha", GITHUB_RUN_ID="123456")
            result = subprocess.run([sys.executable, "-B", str(PRODUCER),
                                     "--prelude", str(Path(directory) / "missing-prelude.js"),
                                     "--output", str(output), "--source-sha", "not-a-source-sha",
                                     "--run-id", "123456"], capture_output=True, timeout=25, env=env)
            self.assertEqual(result.returncode, 3, result.stderr)
            summary = json.loads((output / "required-encoding-gate.json").read_bytes())
            evidence = json.loads((output / "required-encoding-evidence.json").read_bytes())
            self.assertEqual(summary["status"], "NOT_RUN")
            self.assertFalse(summary["appleEngineExecuted"])
            self.assertFalse(summary["mockEngineAllowed"])
            self.assertEqual(evidence, {"records": []})

    def test_cli_refuses_to_replace_existing_evidence(self):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory)
            receipt = output / "required-encoding-gate.json"
            original = b'{"preserve": "prior actual receipt"}\n'
            receipt.write_bytes(original)
            result = subprocess.run([sys.executable, "-B", str(PRODUCER), "--prelude", "missing.js",
                                     "--output", str(output), "--source-sha", "not-a-source-sha"],
                                    capture_output=True, timeout=25)
            self.assertEqual(result.returncode, 3, result.stderr)
            self.assertIn(b"E_ENCODING_OUTPUT_EXISTS", result.stdout)
            self.assertEqual(receipt.read_bytes(), original)
            self.assertFalse((output / "required-encoding-evidence.json").exists())

    def test_duplicate_input_keys_are_rejected_before_evidence_processing(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "duplicate.json"
            path.write_bytes(b'{"records": [], "records": [{"result": "passed"}]}')
            with self.assertRaises(ValueError):
                gate.read_json(path)


if __name__ == "__main__":
    unittest.main()
