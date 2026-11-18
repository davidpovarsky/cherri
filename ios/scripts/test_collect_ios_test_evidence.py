import contextlib
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest

SCRIPT = Path(__file__).with_name("collect_ios_test_evidence.py")
spec = importlib.util.spec_from_file_location("ios_evidence", SCRIPT)
collector = importlib.util.module_from_spec(spec)
spec.loader.exec_module(collector)


class IOSTestEvidenceTests(unittest.TestCase):
    ui_method = "testE01CompletionDisplayAndInsertionUI"
    service_method = "testE01ContextualCompletionService"
    checkpoint = "[CHECKPOINT:E01_COMPLETION_INSERTED]"

    def block(self, method=None, status="passed", body="", service=False):
        method = method or (self.service_method if service else self.ui_method)
        target = "CherriTests.CherriCoreIntegrationTests" if service else "CherriUITests.CherriEditorUITests"
        return (f"Test Case '-[{target} {method}]' started.\n{body}\n"
                f"Test Case '-[{target} {method}]' {status} (0.123 seconds).\n")

    def receipt(self, **updates):
        value = {"schemaVersion": "1.0", "testName": f"CherriEditorUITests/{self.ui_method}",
                 "stage": "typed-source", "syntheticSourceResetVerified": True,
                 "actualSource": 'text("שלום 🌍")\n', "expectedSource": 'text("שלום 🌍")\n',
                 "exactUTF16Match": True, "editorExists": True}
        value.update(updates)
        return collector.RECEIPT_PREFIX + json.dumps(value, ensure_ascii=False)

    def collect(self, text, exit_code=0, sha="a" * 40):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            if text is not None:
                (root / "raw.log").write_text(text, encoding="utf-8")
            if exit_code is not None:
                (root / "exit").write_text(str(exit_code), encoding="utf-8")
            with contextlib.redirect_stdout(io.StringIO()):
                diagnostic, records = collector.collect(root / "raw.log", root / "output", sha, "1234", root / "exit")
            outputs = {path.name: path.read_text(encoding="utf-8") for path in (root / "output").iterdir()}
            return diagnostic, records, outputs

    def test_success_requires_real_test_boundary_checkpoint_and_process_success(self):
        log = self.block(body=self.checkpoint + "\n" + self.receipt()) + self.block(service=True)
        diagnostic, records, outputs = self.collect(log)
        self.assertEqual(diagnostic["runStatus"], "PASSED")
        self.assertEqual(len(records), 2)
        self.assertEqual(len(diagnostic["uiReceipts"]), 1)
        digest = hashlib.sha256(outputs["xcodetest.log"].encode()).hexdigest()
        self.assertTrue(all(record["artifacts"][0]["sha256"] == digest for record in records))

    def test_failed_run_keeps_observed_outcomes_without_acceptance_credit(self):
        failure = f"/Users/runner/project/ios/CherriUITests/CherriEditorUITests.swift:98: error: -[CherriUITests.CherriEditorUITests {self.ui_method}] : XCTAssertTrue failed"
        log = self.block(service=True) + self.block(status="failed", body=self.receipt(actualSource='text(")x")') + "\n" + failure)
        diagnostic, records, outputs = self.collect(log, 65)
        self.assertEqual(diagnostic["runStatus"], "FAILED")
        self.assertEqual(records, [])
        self.assertEqual({row["observedStatus"] for row in diagnostic["testResults"]}, {"passed", "failed"})
        self.assertEqual(diagnostic["uiFailures"][0]["sourceLine"], 98)
        self.assertNotIn("/Users/runner", "".join(outputs.values()))

    def test_checkpoint_outside_case_or_inside_another_case_cannot_pass(self):
        log = self.checkpoint + "\n" + self.block() + self.block(method="testUnrelated", body=self.checkpoint)
        self.assertEqual(self.collect(log)[1], [])

    def test_duplicate_or_incomplete_case_is_not_verified(self):
        duplicate = self.block(body=self.checkpoint) * 2
        diagnostic, records, _ = self.collect(duplicate)
        self.assertEqual(records, [])
        self.assertEqual(diagnostic["testResults"][0]["observedStatus"], "ambiguous")
        incomplete = f"Test Case '-[CherriUITests.CherriEditorUITests {self.ui_method}]' started.\n{self.checkpoint}\n"
        diagnostic, records, _ = self.collect(incomplete)
        self.assertEqual(records, [])
        self.assertEqual(diagnostic["testResults"][0]["observedStatus"], "incomplete")

    def test_unverified_reset_never_publishes_source_or_arbitrary_ax_data(self):
        secret = "Private document from before reset מידע פרטי"
        for verified in (False, None, "true", 1):
            receipt = self.receipt(syntheticSourceResetVerified=verified, actualSource=secret,
                                   expectedSource=secret, diagnosticLabel=secret, editorExists=secret)
            diagnostic, _, outputs = self.collect(self.block(status="failed", body=receipt), 65)
            self.assertNotIn(secret, "".join(outputs.values()))
            saved = diagnostic["uiReceipts"][0]
            self.assertEqual(saved["sourceOmittedReason"], "EMPTY_RESET_NOT_VERIFIED")
            self.assertNotIn("diagnosticLabel", saved)

    def test_unexpected_secret_canaries_are_redacted_but_synthetic_unicode_is_exact(self):
        for secret in ("ghp_FAKE_SECRET_TOKEN", "person@example.com", r"C:\Users\Personal\private.txt", "/Users/Private/Documents"):
            _, _, outputs = self.collect(self.block(status="failed", body=self.receipt(actualSource=secret)), 65)
            self.assertNotIn(secret, "".join(outputs.values()))
            self.assertIn("REDACTED_UNEXPECTED_CONTENT", outputs["ios-test-diagnostics.json"])
        diagnostic, _, _ = self.collect(self.block(body=self.receipt() + "\n" + self.checkpoint))
        self.assertEqual(diagnostic["uiReceipts"][0]["actualSource"], 'text("שלום 🌍")\n')

    def test_missing_log_exit_or_revision_never_generates_acceptance(self):
        for text, code, sha in ((None, 0, "a" * 40), (self.block(body=self.checkpoint), None, "a" * 40),
                                (self.block(body=self.checkpoint), 0, "unknown"), ("unrelated text", 0, "a" * 40)):
            diagnostic, records, _ = self.collect(text, code, sha)
            self.assertEqual(records, [])
            self.assertEqual(diagnostic["runStatus"], "NOT_RUN_OR_INCOMPLETE")

    def test_raw_logs_and_fake_receipt_fields_are_excluded(self):
        raw = "arbitrary app UI and file names must not be uploaded\n"
        raw += collector.RECEIPT_PREFIX + '{"schemaVersion":"bogus"}\n'
        raw += self.block(body=self.receipt(untrusted={"secret": "never-publish"}) + "\n" + self.checkpoint)
        diagnostic, _, outputs = self.collect(raw)
        self.assertEqual(len(diagnostic["uiReceipts"]), 1)
        self.assertNotIn("never-publish", "".join(outputs.values()))
        self.assertNotIn("arbitrary app UI", "".join(outputs.values()))

    def test_receipts_and_source_have_hard_bounds(self):
        diagnostic, _, outputs = self.collect(self.block(status="failed", body="\n".join(self.receipt() for _ in range(100))), 65)
        self.assertEqual(len(diagnostic["uiReceipts"]), 64)
        _, _, outputs = self.collect(self.block(status="failed", body=self.receipt(actualSource="x" * 16385)), 65)
        self.assertNotIn("x" * 16385, "".join(outputs.values()))

    def test_existing_evidence_is_never_overwritten(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "output").mkdir()
            marker = root / "output/preserved"
            marker.write_text("original")
            with self.assertRaises(ValueError):
                collector.collect(root / "missing", root / "output", "a" * 40, "1234", root / "missing-exit")
            self.assertEqual(marker.read_text(), "original")

    def test_real_cli_collects_failed_run_and_writes_only_allowlisted_files(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "raw").write_text(self.block(status="failed", body=self.receipt()), encoding="utf-8")
            (root / "exit").write_text("65")
            result = subprocess.run([sys.executable, str(SCRIPT), str(root / "raw"), str(root / "output"),
                                     "a" * 40, "1234", str(root / "exit")], capture_output=True, text=True, timeout=10)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(json.loads(result.stdout)["runStatus"], "FAILED")
            self.assertEqual({p.name for p in (root / "output").iterdir()},
                             {"xcodetest.log", "ios-test-evidence.json", "ios-test-diagnostics.json"})

    def test_workflow_preserves_test_exit_and_collects_even_after_failure(self):
        workflow = (SCRIPT.parents[2] / ".github/workflows/ios-build.yml").read_text(encoding="utf-8")
        collect_step = workflow.split("      - name: Collect bounded iOS test evidence\n", 1)[1].split("      - name:", 1)[0]
        upload_step = workflow.split("      - name: Upload iOS test evidence\n", 1)[1].split("      - name:", 1)[0]
        self.assertIn("if: always()", collect_step)
        self.assertIn("if: always() && steps.ios_evidence.outcome == 'success'", upload_step)
        self.assertNotIn("ios/xcodetest.log", upload_step)
        bash = str(Path(os.environ.get("ProgramFiles", "C:/Program Files")) / "Git/bin/bash.exe") if os.name == "nt" else shutil.which("bash")
        if not bash:
            self.skipTest("bash unavailable; workflow exit preservation not executed")
        with tempfile.TemporaryDirectory() as directory:
            step = workflow.split("      - name: Run CherriCore in iOS Simulator\n", 1)[1].split("      - name:", 1)[0]
            after_test = step[step.index("          status=$?\n"):]
            script = "set +e\n(exit 65)\n" + "\n".join(line[10:] if line.startswith("          ") else line for line in after_test.splitlines())
            result = subprocess.run([bash, "-c", script], cwd=directory, capture_output=True, text=True, timeout=10)
            self.assertEqual(result.returncode, 65, result.stderr)
            self.assertEqual((Path(directory) / "xcodetest.exitcode").read_text().strip(), "65")


if __name__ == "__main__":
    unittest.main()
