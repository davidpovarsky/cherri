"""Exercise existing signed callback and shell selectors for collection probes."""
import json
import os
from pathlib import Path
import subprocess
import unittest
import test_runtime_callback_contract as callback
import test_workflow_input_contract as workflow

CASES = {'qa-collect-counts-v2': 'CHERRI_QA_COLLECT_COUNTS:empty=0:single=1:one:many=100:0:99', 'qa-collect-counts-legacy': 'CHERRI_QA_COLLECT_COUNTS:empty=0:single=1:one:many=100:0:99', 'qa-collect-dictionaries-v2': 'CHERRI_QA_COLLECT_DICTIONARIES:count=2:first=first:last=second:type=Dictionary:nested=2:a:b', 'qa-collect-dictionaries-legacy': 'CHERRI_QA_COLLECT_DICTIONARIES:count=2:first=first:last=second:type=Dictionary:nested=2:a:b'}

class CollectNativeContractTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        callback.CallbackContractTests.setUpClass()
        workflow.WorkflowInputContractTests.setUpClass()
        cls.callback = callback.CallbackContractTests()
        cls.workflow = workflow.WorkflowInputContractTests()
        cls.root = Path(__file__).resolve().parents[2]

    def test_exact_collection_targets_and_source_identity(self):
        script = (self.root / "scripts/ios27_shortcuts_simulator_qa.sh").read_text(encoding="utf-8")
        selector = script.split("# Select primary target fixture from SAMPLE_ID\n", 1)[1].split("LEGACY_COUNTERPART_SOURCE=", 1)[0]
        selector += '\nprintf "%s\\n" "$TARGET_FIXTURE_ID" "$TARGET_SOURCE_FILE" "$EXPECTED_SHORTCUT_NAME" "$EXPECTED_SEMANTIC_OUTPUT"\n'
        for fid, expected in CASES.items():
            with self.subTest(fixture=fid):
                self.assertEqual(self.workflow.validate(["runtime", "ipad-default", fid, "false", "1", "true"]).returncode, 0)
                result = subprocess.run([self.workflow.bash, "-s"], input=selector,
                    env={**os.environ, "SAMPLE_ID": fid, "ROOT_DIR": "/reviewed",
                         "TARGET_SOURCE_RELATIVE": "../../private.cherri"},
                    capture_output=True, text=True, timeout=10)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(result.stdout.splitlines(), [fid,
                    "/reviewed/tools/shortcuts-simulator-qa/fixtures/" + fid + ".cherri", fid, expected])
        for bad in ("qa-collect-private", "qa-collect-counts-v2-release", "qa-collect-counts-v2;exit 0"):
            self.assertEqual(self.workflow.validate(["runtime", "ipad-default", bad, "false", "1", "true"]).returncode, 2)

    def test_actual_result_counts_and_trace_are_required(self):
        for fid, expected in CASES.items():
            with self.subTest(fixture=fid):
                event = self.callback.event()
                event.update(fixtureId=fid, expectedShortcutName=fid, resultSummary=expected)
                event["tracePayload"] = event["tracePayload"].replace("qa-unit", fid)
                proof = self.callback.verify([event], fid, expected)
                self.assertEqual((proof["runGate"], proof["traceGate"]), ("PASS", "PASS"))
                for mutation in (
                    {**event, "resultSummary": expected.replace("=2", "=1").replace("=100", "=99")},
                    {**event, "resultSummary": expected.replace(":first=first:last=second", ":first=second:last=first").replace(":0:99", ":99:0")},
                    {**event, "resultSummary": expected.replace("type=Dictionary", "type=Text").replace("empty=0", "empty=1")},
                    {**event, "runId": "stale"},
                    {**event, "status": "ERROR"},
                ):
                    self.assertEqual(self.callback.verify([mutation], fid, expected)["runGate"], "FAIL_RUNTIME")
                self.assertEqual(self.callback.verify([{**event, "tracePayload": ""}], fid, expected)["traceGate"], "FAIL_RUNTIME")
