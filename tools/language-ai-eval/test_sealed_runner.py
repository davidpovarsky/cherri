"""Offline outcome-retention controls; mocked adapters are never AI evidence."""
import argparse
import contextlib
import io
import json
from pathlib import Path
import tempfile
import unittest
from unittest import mock

import runner
import evaluation_bundle as bundle_api


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

    def sealed_arguments(self):
        (self.root / "suite.json").write_bytes(Path(runner.__file__).with_name("tasks.json").read_bytes())
        for arm in runner.ARMS:
            (self.root / (arm+"-guide.md")).write_text("synthetic control only", encoding="utf-8")
        bundle = {"root": self.root, "anchor": {"synthetic_control": True}, "manifest": {
            "provider": {"model": "SYNTHETIC_CONTROL_ONLY"},
            "budget": {"max_input_tokens": 12000, "max_output_tokens": 2000},
            "order_seed": 0, "process_timeout_seconds": 120,
            "provenance_scope": "synthetic runner control; never AI evidence"}}
        arguments = ["--suite", str(self.root / "suite.json"), "--output", str(self.root / "report.json"),
            "--sealed-bundle", str(self.root), "--model", "SYNTHETIC_CONTROL_ONLY", "--allow-external",
            "--legacy-guide", str(self.root / "legacy-guide.md"), "--v2-guide", str(self.root / "v2-guide.md"),
            "--adapter", str(Path(runner.__file__).with_name("anthropic_adapter.py")),
            "--validator", str(Path(runner.__file__).with_name("graph_validator.py"))]
        return arguments, bundle

    def sealed_controls(self, bundle, *, seal_error=None):
        stack = contextlib.ExitStack()
        stack.enter_context(mock.patch.dict(runner.os.environ, {"CHERRI_AI_EVAL_BUNDLE": str(self.root)}))
        stack.enter_context(mock.patch.object(bundle_api, "load_bundle", return_value=bundle))
        stack.enter_context(mock.patch.object(bundle_api, "validate_model_request", return_value=None))
        stack.enter_context(mock.patch.object(bundle_api, "seal_outcome", return_value={}, side_effect=seal_error))
        calls = stack.enter_context(mock.patch.object(runner, "run_process", side_effect=ValueError("synthetic provider failure")))
        stack.enter_context(contextlib.redirect_stdout(io.StringIO()))
        return stack, calls

    def test_second_sealed_invocation_preserves_original_report_and_never_calls_provider(self):
        arguments, bundle = self.sealed_arguments()
        stack, calls = self.sealed_controls(bundle)
        with stack:
            self.assertEqual(runner.main(arguments), 1)
            self.assertEqual(calls.call_count, 12)
            original = (self.root / "report.json").read_bytes()
            self.assertEqual(json.loads(original)["status"], "EVALUATION_INCOMPLETE")
            self.assertEqual(runner.main(arguments), 1)
            self.assertEqual(calls.call_count, 12)
            self.assertEqual((self.root / "report.json").read_bytes(), original)

    def test_first_sealed_attempt_keeps_started_outcome_when_sealing_fails(self):
        arguments, bundle = self.sealed_arguments()
        stack, calls = self.sealed_controls(bundle, seal_error=ValueError("synthetic seal failure"))
        with stack:
            self.assertEqual(runner.main(arguments), 1)
        report = json.loads((self.root / "report.json").read_bytes())
        self.assertEqual(calls.call_count, 1)
        self.assertEqual(report["status"], "HARNESS_FAILED")
        self.assertFalse(report["ai_quality_measured"])
        self.assertEqual(len(report["records"]), 12)
        self.assertEqual(report["records"][0]["status"], "PROVIDER_ERROR")
        self.assertEqual(sum(row["status"] == "NOT_STARTED" for row in report["records"]), 11)

    def test_preexisting_report_without_intent_is_preserved_before_provider_call(self):
        arguments, bundle = self.sealed_arguments()
        original = b"preserved earlier report bytes\n"
        (self.root / "report.json").write_bytes(original)
        stack, calls = self.sealed_controls(bundle)
        with stack:
            self.assertEqual(runner.main(arguments), 1)
        calls.assert_not_called()
        self.assertEqual((self.root / "report.json").read_bytes(), original)

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
