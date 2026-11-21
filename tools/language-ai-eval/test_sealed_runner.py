"""Offline outcome-retention controls; mocked adapters are never AI evidence."""
import argparse
from pathlib import Path
import tempfile
import unittest
from unittest import mock

import runner


class OutcomeRetentionControls(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        guide = self.root / "guide.md"
        guide.write_text("synthetic control only")
        adapter = self.root / "adapter.py"
        adapter.write_text("# synthetic control, never invoked")
        validator = self.root / "validator.py"
        validator.write_text("# synthetic control, never invoked")
        self.args = argparse.Namespace(suite=Path(runner.__file__).with_name("tasks.json"), fake=False,
            adapter=str(adapter), validator=str(validator), model="SYNTHETIC_CONTROL_ONLY", allow_external=True,
            input_tokens=12000, output_tokens=2000, timeout=120, seed=0,
            legacy_guide=guide, v2_guide=guide, output=self.root / "report.json", sealed_bundle=None)

    def test_one_provider_error_retains_every_outcome_and_no_score(self):
        seen = []
        def process(command, request, timeout, originals=None):
            if "source" in request:
                reply = {k: request[k] for k in ("schema", "request_id", "task_id", "arm", "source_sha256")}
                reply.update(synthetic=False, checks=[{"name": name, "passed": True} for name in request["required_checks"]])
                return reply, {"exit_code": 0}
            seen.append(request["request_id"])
            if len(seen) == 1:
                raise ValueError("synthetic provider failure")
            return {"schema": runner.SCHEMA, "request_id": request["request_id"], "model": request["model"],
                    "synthetic": False, "provider_request_id": "SYNTHETIC_CONTROL_ONLY", "code": 'show("control")',
                    "usage": {"input_tokens": 1, "output_tokens": 1}}, {"exit_code": 0}
        with mock.patch.object(runner, "run_process", side_effect=process):
            report, status = runner.evaluate(self.args)
        self.assertEqual(len(seen), 12)
        self.assertEqual(len(set(seen)), 12)
        self.assertEqual(len(report["records"]), 12)
        self.assertEqual(sum(r["status"] == "PROVIDER_ERROR" for r in report["records"]), 1)
        self.assertEqual(sum(r["status"] == "MEASURED" for r in report["records"]), 11)
        self.assertEqual(status, 1)
        self.assertEqual(report["status"], "EVALUATION_INCOMPLETE")
        self.assertFalse(report["ai_quality_measured"])
        self.assertNotIn("measured_results", report)
        self.assertTrue(all(item["total"] == 6 for item in report["outcome_accounting"].values()))

    def test_per_process_failure_preserves_originals_and_secret_is_not_reported(self):
        import json
        import sys
        script = self.root / "failure.py"
        script.write_text('import sys\nprint("SECRET_CANARY",file=sys.stderr)\nraise SystemExit(7)\n')
        originals = self.root / "process"
        with self.assertRaises(ValueError) as error:
            runner.run_process([sys.executable, str(script)], {"synthetic": True}, 2, originals)
        self.assertNotIn("SECRET_CANARY", str(error.exception))
        receipt = json.loads((originals / "process.json").read_text())
        self.assertEqual(receipt["exit_code"], 7)
        self.assertEqual(receipt["stderr_sha256"], runner.digest((originals / "stderr.captured").read_bytes()))
        self.assertEqual((originals / "request.json").read_bytes(), b'{"synthetic": true}')


if __name__ == "__main__":
    unittest.main()
