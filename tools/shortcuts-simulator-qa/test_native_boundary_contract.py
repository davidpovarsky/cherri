"""Exercise the production selector and callback gates; no Apple execution."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

import native_boundary_diagnostic as probe
import test_runtime_callback_contract as callback
import test_workflow_input_contract as workflow


class NativeBoundaryContractTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        callback.CallbackContractTests.setUpClass()
        workflow.WorkflowInputContractTests.setUpClass()
        cls.callback = callback.CallbackContractTests()
        cls.workflow = workflow.WorkflowInputContractTests()
        cls.root = Path(__file__).resolve().parents[2]

    def test_reviewed_source_preserves_original_body_and_oracles(self):
        raw = (self.root / "tools/shortcuts-simulator-qa/fixtures" / (probe.FIXTURE_ID + ".cherri")).read_bytes()
        observed = probe.verify_source(self.root, probe.FIXTURE_ID, raw)
        self.assertFalse(observed["formalAcceptance"])
        self.assertTrue(observed["bindingBodyByteIdentical"])
        with self.assertRaisesRegex(ValueError, "NATIVE_BOUNDARY_SOURCE_CONTRACT"):
            probe.verify_source(self.root, probe.FIXTURE_ID, raw.replace(b"getClipboard()", b"getVariable(Clipboard)"))
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for name in probe.ORIGINALS:
                path = root / name
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_bytes((self.root / name).read_bytes())
            changed = root / next(iter(probe.ORIGINALS))
            changed.write_bytes(changed.read_bytes() + b"// changed\n")
            with self.assertRaisesRegex(ValueError, "NATIVE_BOUNDARY_ORIGINAL_CHANGED"):
                probe.verify_source(root, probe.FIXTURE_ID, raw)

    def test_exact_selector_rejects_injected_or_unreviewed_choices(self):
        script = (self.root / "scripts/ios27_shortcuts_simulator_qa.sh").read_text(encoding="utf-8")
        selector = script.split("# Select primary target fixture from SAMPLE_ID\n", 1)[1].split("LEGACY_COUNTERPART_SOURCE=", 1)[0]
        selector += '\nprintf "%s\\n" "$TARGET_FIXTURE_ID" "$TARGET_SOURCE_FILE" "$EXPECTED_SHORTCUT_NAME" "$EXPECTED_SEMANTIC_OUTPUT"\n'
        self.assertEqual(self.workflow.validate(["runtime", "ipad-default", probe.SAMPLE_ID, "false", "1", "true"]).returncode, 0)
        proc = subprocess.run([self.workflow.bash, "-s"], input=selector,
                              env={**os.environ, "SAMPLE_ID": probe.SAMPLE_ID, "ROOT_DIR": "/reviewed",
                                   "TARGET_SOURCE_RELATIVE": "../../unreviewed.cherri"},
                              capture_output=True, text=True, timeout=10)
        self.assertEqual(proc.returncode, 0, proc.stderr)
        self.assertEqual(proc.stdout.splitlines(), [
            probe.FIXTURE_ID, "/reviewed/tools/shortcuts-simulator-qa/fixtures/" + probe.FIXTURE_ID + ".cherri",
            probe.FIXTURE_ID, probe.EXPECTED])
        for sample in (probe.FIXTURE_ID, probe.SAMPLE_ID + ";exit 0", probe.SAMPLE_ID + "-private"):
            self.assertEqual(self.workflow.validate(["runtime", "ipad-default", sample, "false", "1", "true"]).returncode, 2)

    def test_production_callback_gate_keeps_every_independent_observation_mandatory(self):
        event = self.callback.event()
        event.update(fixtureId=probe.FIXTURE_ID, expectedShortcutName=probe.FIXTURE_ID, resultSummary=probe.EXPECTED)
        event["tracePayload"] = event["tracePayload"].replace("qa-unit", probe.FIXTURE_ID)
        proof = self.callback.verify([event], probe.FIXTURE_ID, probe.EXPECTED)
        self.assertEqual((proof["runGate"], proof["traceGate"]), ("PASS", "PASS"))
        for label, _, value in probe.FIELDS:
            wrong = "N" if value == "Y" else "Y" if value == "N" else "0"
            changed = probe.EXPECTED.replace(":" + label + "=" + value, ":" + label + "=" + wrong)
            with self.subTest(field=label):
                self.assertNotEqual(changed, probe.EXPECTED)
                self.assertEqual(self.callback.verify([{**event, "resultSummary": changed}],
                                                      probe.FIXTURE_ID, probe.EXPECTED)["runGate"], "FAIL_RUNTIME")
        for changed in ({**event, "runId": "stale"}, {**event, "status": "ERROR"}):
            self.assertEqual(self.callback.verify([changed], probe.FIXTURE_ID, probe.EXPECTED)["runGate"], "FAIL_RUNTIME")
        self.assertEqual(self.callback.verify([{**event, "tracePayload": ""}],
                                              probe.FIXTURE_ID, probe.EXPECTED)["traceGate"], "FAIL_RUNTIME")


if __name__ == "__main__":
    unittest.main()
