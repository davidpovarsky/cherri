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

    def test_native_editor_input_failure_keeps_only_location_and_test_identity(self):
        target = "CherriTests.CherriEditorInputTests"
        method = "testAuthoredCloserIsNeverConsumed"
        log = (f"Test Case '-[{target} {method}]' started.\n"
               f"/Users/runner/CherriEditorInputTests.swift:65: error: -[{target} {method}] : PRIVATE_MESSAGE\n"
               f"Test Case '-[{target} {method}]' failed (0.123 seconds).\n")
        diagnostic, records, outputs = self.collect(log, 65)
        self.assertEqual(records, [])
        self.assertEqual(diagnostic["testResults"][0]["observedStatus"], "failed")
        self.assertEqual(diagnostic["uiFailures"], [{"testId": "CherriEditorInputTests/" + method, "sourceLine": 65}])
        self.assertNotIn("PRIVATE_MESSAGE", "".join(outputs.values()))
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

    def startup_receipt(self, **updates):
        value = {"schemaVersion": "1.0", "testName": f"CherriEditorUITests/{self.ui_method}",
                 "stage": "document-wait", "waitCompleted": False,
                 "windowExists": True, "appRunningForeground": True,
                 "createDocumentExists": True, "createDocumentHittable": False,
                 "editorExists": False, "editorHittable": False}
        value.update(updates)
        return "[CHERRI_UI_STARTUP] " + json.dumps(value, ensure_ascii=False)

    def test_startup_receipts_capture_only_known_state_without_source(self):
        secret = "PRIVATE_STARTUP_CANARY document label and restored source"
        for stage in ("window-wait", "document-wait", "editor-wait"):
            with self.subTest(stage=stage):
                receipt = self.startup_receipt(stage=stage, actualSource=secret, expectedSource=secret,
                                               documentLabel=secret, accessibilityTree=secret,
                                               syntheticSourceResetVerified=True)
                diagnostic, records, outputs = self.collect(self.block(status="failed", body=receipt), 65)
                saved = diagnostic.get("startupReceipts", [])
                self.assertEqual(len(saved), 1)
                self.assertEqual(saved[0], {
                    "schemaVersion": "1.0", "testName": f"CherriEditorUITests/{self.ui_method}",
                    "stage": stage, "waitCompleted": False, "windowExists": True,
                    "appRunningForeground": True, "createDocumentExists": True,
                    "createDocumentHittable": False, "editorExists": False, "editorHittable": False})
                self.assertNotIn(secret, "".join(outputs.values()))
                self.assertEqual(records, [])

    def test_startup_receipts_require_active_case_exact_stage_and_boolean_types(self):
        diagnostic, _, _ = self.collect(self.block(status="failed", body=self.startup_receipt()), 65)
        self.assertEqual(len(diagnostic.get("startupReceipts", [])), 1)
        for changes in ({"schemaVersion": "2"}, {"testName": "CherriEditorUITests/testOther"},
                        {"stage": "PRIVATE_STAGE"}, {"stage": ["document-wait"]},
                        {"waitCompleted": 1}, {"windowExists": "true"}, {"editorExists": None},
                        {"appRunningForeground": 0}, {"createDocumentExists": "PRIVATE_VALUE"},
                        {"createDocumentHittable": 1}, {"editorHittable": {}}):
            with self.subTest(changes=changes):
                diagnostic, records, outputs = self.collect(
                    self.block(status="failed", body=self.startup_receipt(**changes)), 65)
                self.assertEqual(diagnostic.get("startupReceipts", []), [])
                self.assertEqual(records, [])
                self.assertNotIn("PRIVATE_", "".join(outputs.values()))
        diagnostic, _, _ = self.collect(self.startup_receipt(), 65)
        self.assertEqual(diagnostic.get("startupReceipts", []), [])

    def test_startup_receipts_have_a_separate_bound_from_synthetic_source_receipts(self):
        startup = "\n".join(self.startup_receipt() for _ in range(80))
        diagnostic, _, _ = self.collect(self.block(status="failed", body=startup + "\n" + self.receipt()), 65)
        self.assertEqual(len(diagnostic.get("startupReceipts", [])), 64)
        self.assertEqual(len(diagnostic["uiReceipts"]), 1)
        self.assertEqual(diagnostic["uiReceipts"][0]["actualSource"], 'text("שלום 🌍")\n')

    def test_startup_receipt_cannot_replace_behavioral_checkpoint(self):
        receipt = self.startup_receipt(stage="editor-wait", waitCompleted=True,
                                       editorExists=True, editorHittable=True,
                                       checkpoint=self.checkpoint)
        diagnostic, records, _ = self.collect(self.block(body=receipt))
        self.assertEqual(len(diagnostic.get("startupReceipts", [])), 1)
        self.assertEqual(records, [])

    def documentation_ui_state(self, **updates):
        value = {key: False for key in collector.docs.UI_STATE_BOOLS}
        value.update(schemaVersion="1.0", testName=collector.docs.UI,
                     stage="after-toggle", syntheticSourceResetVerified=True,
                     searchExists=True, queryIsExpected=True, switchExists=True,
                     switchHittable=True, switchOff=True, noResultsExists=True)
        value.update(updates)
        return collector.docs.UI_STATE_PREFIX + json.dumps(value)

    def test_documentation_ui_state_preserves_actual_failed_toggle_without_acceptance(self):
        method = collector.docs.UI.split("/")[1]
        body = self.documentation_ui_state()
        diagnostic, records, outputs = self.collect(self.block(method=method, status="failed", body=body), 65)
        self.assertEqual(records, [])
        self.assertEqual(diagnostic["runStatus"], "FAILED")
        self.assertEqual(len(diagnostic["documentationUIStates"]), 1)
        state = diagnostic["documentationUIStates"][0]
        self.assertTrue(state["switchOff"])
        self.assertFalse(state["switchOn"])
        self.assertFalse(state["migrationButtonExists"])
        self.assertEqual(set(state), {"schemaVersion", "testName", "stage", *collector.docs.UI_STATE_BOOLS})
        self.assertNotIn(collector.docs.CHECKPOINT, outputs["xcodetest.log"])
        # Successful diagnostic observations cannot substitute for the three
        # required app tests, original bundle receipt and behavioral checkpoint.
        diagnostic, records, _ = self.collect(self.block(method=method, body=self.documentation_ui_state(
            switchOn=True, switchOff=False, migrationButtonExists=True, migrationElementExists=True)))
        self.assertEqual(records, [])

    def test_documentation_ui_state_rejects_private_unbound_and_malformed_fields(self):
        method = collector.docs.UI.split("/")[1]
        cases = ({"stage": "PRIVATE_STAGE"}, {"stage": ["after-toggle"]},
                 {"query": "PRIVATE_QUERY"}, {"schemaVersion": "2"},
                 {"testName": "CherriEditorUITests/testOther"},
                 {"queryIsExpected": "PRIVATE_QUERY"}, {"switchOn": 1},
                 {"switchOff": None}, {"migrationButtonExists": {}},
                 {"syntheticSourceResetVerified": False})
        for changes in cases:
            with self.subTest(changes=changes):
                diagnostic, records, outputs = self.collect(self.block(
                    method=method, status="failed", body=self.documentation_ui_state(**changes)), 65)
                self.assertEqual(diagnostic["documentationUIStates"], [])
                self.assertEqual(records, [])
                self.assertNotIn("PRIVATE_", "".join(outputs.values()))
        duplicate = self.documentation_ui_state().replace('"switchOn": false', '"switchOn": false, "switchOn": true')
        for body in (duplicate, self.documentation_ui_state(query="x" * 4096)):
            self.assertEqual(self.collect(self.block(method=method, status="failed", body=body), 65)[0]["documentationUIStates"], [])
        for raw in (self.documentation_ui_state(), self.block(status="failed", body=self.documentation_ui_state())):
            self.assertEqual(self.collect(raw, 65)[0]["documentationUIStates"], [])

    def test_documentation_ui_state_has_an_independent_eight_record_bound(self):
        method = collector.docs.UI.split("/")[1]
        body = "\n".join(self.documentation_ui_state() for _ in range(20))
        body += "\n" + self.receipt(testName=collector.docs.UI)
        diagnostic, records, _ = self.collect(self.block(method=method, status="failed", body=body), 65)
        self.assertEqual(len(diagnostic["documentationUIStates"]), 8)
        self.assertEqual(len(diagnostic["uiReceipts"]), 1)
        self.assertEqual(records, [])

    def input_receipt(self, **updates):
        value = {"schemaVersion": "1.0", "testName": "CherriEditorInputTests/testNestedGeneratedClosersAndCompleteRealInput",
                 "isolatedSyntheticEditor": True, "fixtureId": "nested-call", "stage": "before-key",
                 "keyIndex": 2, "inputUTF16": [120], "actualSource": 'text("',
                 "actualUTF16Length": 6, "selectionLocation": 6, "selectionLength": 0}
        value.update(updates)
        return collector.INPUT_PREFIX + json.dumps(value)

    def input_block(self, body, complete=True):
        target = "CherriTests.CherriEditorInputTests testNestedGeneratedClosersAndCompleteRealInput"
        return (f"Test Case '-[{target}]' started.\n" + body + "\n"
                + (f"Test Case '-[{target}]' passed (0.123 seconds).\n" if complete else ""))

    def test_input_crash_keeps_fixed_category_and_last_real_key_without_private_reason(self):
        body = self.input_receipt() + "\n/Users/private/CodeStorageDelegate.swift:827: Fatal error: Index out of range PRIVATE_REASON"
        diagnostic, records, outputs = self.collect(self.input_block(body, complete=False), 65)
        self.assertEqual(records, [])
        self.assertEqual(diagnostic["testResults"][0]["observedStatus"], "incomplete")
        self.assertEqual(len(diagnostic["inputReceipts"]), 1)
        self.assertEqual(diagnostic["nativeRuntimeEvents"], [{
            "testId": "CherriEditorInputTests/testNestedGeneratedClosersAndCompleteRealInput",
            "code": "INDEX_OUT_OF_RANGE", "sourceFile": "CodeStorageDelegate.swift", "sourceLine": 827}])
        self.assertNotIn("PRIVATE_REASON", "".join(outputs.values()))
        self.assertNotIn("/Users/private", "".join(outputs.values()))

    def test_input_receipt_requires_active_case_and_exact_isolation_and_fixture(self):
        for changes in ({"isolatedSyntheticEditor": False}, {"isolatedSyntheticEditor": 1},
                        {"fixtureId": "private-source"}, {"testName": "CherriEditorInputTests/testOther"},
                        {"selectionLocation": -1}, {"inputUTF16": [True]}, {"inputUTF16": [65536]}):
            diagnostic, _, outputs = self.collect(self.input_block(self.input_receipt(**changes)), 65)
            self.assertEqual(diagnostic["inputReceipts"], [])
        diagnostic, _, _ = self.collect(self.input_receipt(), 65)
        self.assertEqual(diagnostic["inputReceipts"], [])

    def test_input_receipts_have_count_and_private_source_limits(self):
        body = "\n".join(self.input_receipt() for _ in range(600))
        diagnostic, _, _ = self.collect(self.input_block(body), 65)
        self.assertEqual(len(diagnostic["inputReceipts"]), 512)
        for source in ("ghp_PRIVATE_CANARY", "x" * 16385):
            diagnostic, _, outputs = self.collect(self.input_block(self.input_receipt(actualSource=source)), 65)
            self.assertEqual(diagnostic["inputReceipts"][0]["actualSource"], "[REDACTED_UNEXPECTED_CONTENT]")
            self.assertNotIn(source, "".join(outputs.values()))

    def test_paste_lifecycle_receipts_preserve_real_failure_without_promoting_evidence(self):
        method = "testSingleCharacterPasteDoesNotConsumeGeneratedText"
        target = "CherriEditorInputTests/" + method
        start = f"Test Case '-[CherriTests.CherriEditorInputTests {method}]' started.\n"
        finish = f"Test Case '-[CherriTests.CherriEditorInputTests {method}]' failed (3.123 seconds).\n"
        def receipt(**updates):
            data = dict(testName=target, fixtureId="single-character-paste", stage="after-paste-wait",
                        actualSource='text("x"))', pasteElapsedMilliseconds=3123,
                        viewIsFirstResponder=True, windowAttached=True, windowIsKey=True)
            data.update(updates)
            return self.input_receipt(**data)
        diagnostic, records, outputs = self.collect(start + receipt() + "\n" + finish, 65)
        self.assertEqual(records, [])
        self.assertEqual(diagnostic["testResults"][0]["observedStatus"], "failed")
        row = diagnostic["inputReceipts"][0]
        self.assertEqual(row["actualSource"], 'text("x"))')
        self.assertEqual(row["pasteElapsedMilliseconds"], 3123)
        for changes in ({"testName": "CherriEditorInputTests/testOther"}, {"fixtureId": "nested-call"},
                        {"stage": "fixture-complete"}, {"pasteElapsedMilliseconds": True},
                        {"pasteElapsedMilliseconds": -1}, {"pasteElapsedMilliseconds": 120001},
                        {"windowIsKey": "PRIVATE"}, {"windowAttached": None},
                        {"isolatedSyntheticEditor": False}):
            with self.subTest(changes=changes):
                d, _, _ = self.collect(start + receipt(**changes) + "\n" + finish, 65)
                self.assertEqual(d["inputReceipts"], [])
        d, _, outputs = self.collect(start + receipt(actualSource="a private document") + "\n" + finish, 65)
        self.assertEqual(d["inputReceipts"][0]["actualSource"], "[REDACTED_UNEXPECTED_CONTENT]")
        self.assertNotIn("a private document", "".join(outputs.values()))
        self.assertEqual(self.collect(receipt(), 65)[0]["inputReceipts"], [])

    def test_startup_deadline_receipt_is_bounded_and_cannot_authorize_source(self):
        method = "testE01CompletionDisplayAndInsertionUI"
        target = "CherriEditorUITests/" + method
        base = dict(schemaVersion="1.0", testName=target, stage="editor-wait",
                    waitCompleted=False, windowExists=True, appRunningForeground=True,
                    createDocumentExists=False, createDocumentHittable=False,
                    editorExists=True, editorHittable=True,
                    elapsedMilliseconds=29000, budgetMilliseconds=28000,
                    documentCreationIssued=True, actualSource="PRIVATE_RESTORED_SOURCE",
                    syntheticSourceResetVerified=True)
        def collect(changes):
            row = {**base, **changes}
            return self.collect(self.block(method=method, status="failed",
                body=collector.STARTUP_PREFIX + json.dumps(row)), 65)
        diagnostic, records, outputs = collect({})
        self.assertEqual(records, [])
        self.assertEqual(diagnostic["startupReceipts"][0]["elapsedMilliseconds"], 29000)
        self.assertFalse(diagnostic["startupReceipts"][0]["waitCompleted"])
        self.assertNotIn("PRIVATE_RESTORED_SOURCE", "".join(outputs.values()))
        for changes in ({"elapsedMilliseconds": True}, {"elapsedMilliseconds": -1},
                        {"elapsedMilliseconds": 120001}, {"budgetMilliseconds": 29000},
                        {"budgetMilliseconds": 28000.0}, {"documentCreationIssued": "true"}):
            with self.subTest(changes=changes):
                self.assertEqual(collect(changes)[0]["startupReceipts"], [])

    def test_candidate_identifiers_require_exact_synthetic_prefix_and_safe_shape(self):
        identifiers = ["completionItem_show", "suggestionItem_show", "private/path", "completionItem_" + "x" * 100]
        receipt = self.receipt(actualSource="sho", completionCandidateIdentifiers=identifiers)
        diagnostic, _, _ = self.collect(self.block(status="failed", body=receipt), 65)
        self.assertEqual(diagnostic["uiReceipts"][0]["completionCandidateIdentifiers"],
                         ["completionItem_show", "suggestionItem_show"])
        for changes in ({"actualSource": "other"}, {"syntheticSourceResetVerified": False},
                        {"testName": "CherriEditorUITests/testOther"}):
            value = {"actualSource": "sho", "completionCandidateIdentifiers": identifiers}
            value.update(changes)
            diagnostic, _, _ = self.collect(self.block(status="failed", body=self.receipt(**value)), 65)
            self.assertNotIn("completionCandidateIdentifiers", diagnostic["uiReceipts"][0])

    def test_arbitrary_runtime_messages_and_forged_runtime_marker_are_excluded(self):
        body = "A private NSRange text\n" + collector.RUNTIME_PREFIX + '{"code":"PRIVATE_SECRET"}'
        diagnostic, _, outputs = self.collect(self.input_block(body), 65)
        self.assertEqual(diagnostic["nativeRuntimeEvents"], [])
        self.assertNotIn("PRIVATE_SECRET", "".join(outputs.values()))

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
