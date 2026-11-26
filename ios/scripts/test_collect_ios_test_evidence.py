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

    def test_partial_brg25_brg27_probes_preserve_results_without_formal_credit(self):
        cases = (
            ("testBRG25SharedAnalysisBridgeContract", True, ""),
            ("testBRG27EditorRealWorldWorkflowUI", False, "[CHECKPOINT:BRG27_WORKFLOW_VERIFIED]"),
        )
        for method, service, checkpoint in cases:
            for status, exit_code in (("passed", 0), ("failed", 65)):
                with self.subTest(method=method, status=status):
                    diagnostic, records, outputs = self.collect(
                        self.block(method=method, status=status, body=checkpoint, service=service),
                        exit_code)
                    suite = "CherriCoreIntegrationTests" if service else "CherriEditorUITests"
                    self.assertEqual(diagnostic["runStatus"], status.upper())
                    self.assertEqual(diagnostic["testResults"], [
                        {"testId": suite + "/" + method, "observedStatus": status}])
                    self.assertEqual(records, [])
                    self.assertIn(method, outputs["xcodetest.log"])
                    if checkpoint:
                        self.assertIn(checkpoint, outputs["xcodetest.log"])

    def test_partial_gate_probes_do_not_change_existing_e01_credit(self):
        log = (
            self.block(method="testBRG25SharedAnalysisBridgeContract", service=True)
            + self.block(method="testBRG27EditorRealWorldWorkflowUI",
                         body="[CHECKPOINT:BRG27_WORKFLOW_VERIFIED]")
            + self.block(body=self.checkpoint + "\n" + self.receipt())
            + self.block(service=True)
        )
        diagnostic, records, outputs = self.collect(log)
        self.assertEqual(diagnostic["runStatus"], "PASSED")
        self.assertEqual(len(diagnostic["testResults"]), 4)
        self.assertEqual(len(records), 2)
        self.assertTrue(all(record["requirements"] == ["E01"] for record in records))
        self.assertEqual({record["tier"] for record in records}, {"service", "ios-ui"})
        self.assertIn("[CHECKPOINT:BRG27_WORKFLOW_VERIFIED]", outputs["xcodetest.log"])

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
        self.assertEqual(diagnostic["uiFailures"], [{"testId": "CherriEditorInputTests/" + method,
                                                   "sourceLine": 65, "failureCategory": "UNCLASSIFIED"}])
        self.assertNotIn("PRIVATE_MESSAGE", "".join(outputs.values()))
        self.assertNotIn("/Users/runner", "".join(outputs.values()))

    def test_fixed_failure_categories_never_publish_private_failure_details(self):
        cases = {
            "Failed to get matching snapshot: ": "SNAPSHOT_QUERY_FAILURE",
            "Failed to get matching snapshots: ": "SNAPSHOT_QUERY_FAILURE",
            "Failed to get attributes for element: ": "ATTRIBUTE_QUERY_FAILURE",
            "Multiple matching elements found for query: ": "AMBIGUOUS_ELEMENT",
            "No matches found for query: ": "ELEMENT_NOT_FOUND",
            "Application is not running: ": "APPLICATION_NOT_RUNNING",
            "XCTAssertTrue failed - ": "ASSERTION_FAILURE",
            "Unhandled future XCTest error: ": "UNCLASSIFIED",
        }
        secret = "PRIVATE_RESTORED_DOCUMENT /Users/Private/document.cherri ghp_FAKE_SECRET_TOKEN"
        for prefix, expected in cases.items():
            with self.subTest(prefix=prefix):
                failure = (f"/Users/runner/project/CherriEditorUITests.swift:120: error: "
                           f"-[CherriUITests.CherriEditorUITests {self.ui_method}] : {prefix}{secret}")
                diagnostic, records, outputs = self.collect(self.block(status="failed", body=failure), 65)
                self.assertEqual(records, [])
                self.assertEqual(diagnostic["uiFailures"], [{"testId": "CherriEditorUITests/" + self.ui_method,
                    "sourceLine": 120, "failureCategory": expected}])
                for private in (secret, "PRIVATE_RESTORED_DOCUMENT", "/Users/Private", "ghp_FAKE_SECRET_TOKEN"):
                    self.assertNotIn(private, "".join(outputs.values()))

    def test_failure_category_only_uses_immediate_bounded_prefix(self):
        cases = (
            "asserted private text: Multiple matching elements found for query",
            "Unknown error: Failed to get matching snapshot",
            "x" * 200 + "No matches found for query",
            'XCTAssertEqual failed: ("No matches found for private value")',
        )
        expected = ("UNCLASSIFIED", "UNCLASSIFIED", "UNCLASSIFIED", "ASSERTION_FAILURE")
        for message, category in zip(cases, expected):
            with self.subTest(message=message[:40]):
                self.assertEqual(collector.failure_category(message), category)

    def test_preview_lifecycle_tests_remain_diagnostics_without_formal_membership(self):
        target = "CherriTests.CherriPreviewLifecycleTests"
        method = "testCompiledPreviewAppearsAndIsRemovedWhenSourceClears"
        for status, exit_code in (("passed", 0), ("failed", 65)):
            with self.subTest(status=status):
                failure = (f"/Users/runner/CherriPreviewLifecycleTests.swift:89: error: "
                           f"-[{target} {method}] : XCTAssertTrue failed - PRIVATE_CONTENT")
                log = (f"Test Case '-[{target} {method}]' started.\n"
                       + (failure + "\n" if status == "failed" else "")
                       + f"Test Case '-[{target} {method}]' {status} (0.123 seconds).\n")
                diagnostic, records, outputs = self.collect(log, exit_code)
                self.assertEqual(records, [])
                self.assertEqual(diagnostic["testResults"], [{"testId": "CherriPreviewLifecycleTests/" + method,
                                                             "observedStatus": status}])
                self.assertNotIn("PRIVATE_CONTENT", "".join(outputs.values()))

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


    def rp40_collect(self, text, exit_code=0, *, run_id="1234", diagnostic_only=False):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "raw.log").write_text(text, encoding="utf-8")
            if exit_code is not None:
                (root / "exit").write_text(str(exit_code), encoding="utf-8")
            with contextlib.redirect_stdout(io.StringIO()):
                diagnostic, records = collector.collect(
                    root / "raw.log", root / "output", "a" * 40, run_id, root / "exit",
                    diagnostic_only=diagnostic_only)
            outputs = {p.name: p.read_text(encoding="utf-8") for p in (root / "output").iterdir()}
            return diagnostic, records, outputs

    def rp40_block(self, **updates):
        values = {"method": collector.RP40_APP_TEST.split("/")[1],
                  "service": True, "body": collector.RP40_APP_CHECKPOINT}
        values.update(updates)
        return self.block(**values)

    def test_rp40_joined_app_record_binds_two_originals_and_never_grants_ui(self):
        diagnostic, records, outputs = self.rp40_collect(self.rp40_block())
        self.assertEqual(len(records), 1)
        record = records[0]
        self.assertEqual(record["requirements"], ["RP40"])
        self.assertEqual(record["tier"], "app-integration")
        self.assertEqual(record["test_id"], collector.RP40_APP_TEST)
        self.assertEqual(record["assertions"], collector.RP40_APP_ASSERTIONS)
        self.assertEqual(record["detail"], collector.RP40_APP_SCOPE)
        self.assertEqual(record["implementation_sha"], "a" * 40)
        self.assertEqual(record["command"], ["xcodebuild", "test", "-scheme", "Cherri"])
        self.assertEqual(diagnostic["appIntegrationEvidence"]["status"], "VERIFIED_APP_INTEGRATION")
        self.assertIs(diagnostic["appIntegrationEvidence"]["uiTierGranted"], False)
        fixture = SCRIPT.parents[2] / collector.RP40_APP_FIXTURE
        self.assertEqual(record["fixture_sha256"], hashlib.sha256(fixture.read_bytes()).hexdigest())
        self.assertEqual(diagnostic["appIntegrationEvidence"]["fixtureSHA256"], record["fixture_sha256"])
        self.assertEqual({Path(a["path"]).name for a in record["artifacts"]},
                         {"xcodetest.log", "ios-test-diagnostics.json"})
        for artifact in record["artifacts"]:
            self.assertEqual(artifact["run_id"], "1234")
            self.assertEqual(artifact["sha256"],
                             hashlib.sha256(outputs[Path(artifact["path"]).name].encode()).hexdigest())

    def test_rp40_old_service_and_completion_only_ui_are_not_combined_proof(self):
        log = (self.block(method="testRP40SwiftIntegrationService", service=True)
               + self.block(body=self.checkpoint) + self.rp40_block())
        _, records, _ = self.rp40_collect(log)
        rp40 = [r for r in records if "RP40" in r["requirements"]]
        self.assertEqual(len(rp40), 1)
        self.assertEqual(rp40[0]["test_id"], collector.RP40_APP_TEST)
        self.assertEqual(rp40[0]["tier"], "app-integration")
        self.assertEqual([r["requirements"] for r in records if r["tier"] == "ios-ui"], [["E01"]])
        _, old_records, _ = self.rp40_collect(
            self.block(method="testRP40SwiftIntegrationService", service=True))
        self.assertEqual(old_records, [])

    def test_rp40_checkpoint_and_test_boundary_corruptions_are_rejected(self):
        good = self.rp40_block()
        cases = {
            "missing": self.rp40_block(body=""),
            "outside": collector.RP40_APP_CHECKPOINT + "\n" + self.rp40_block(body=""),
            "another_case": self.block(service=True, body=collector.RP40_APP_CHECKPOINT)
                            + self.rp40_block(body=""),
            "duplicate_marker": self.rp40_block(body=(collector.RP40_APP_CHECKPOINT + "\n") * 2),
            "marker_suffix": self.rp40_block(body=collector.RP40_APP_CHECKPOINT + " private payload"),
            "duplicate_case": good + good,
            "incomplete": good.split("passed (", 1)[0],
            "wrong_target": good.replace("CherriTests.CherriCoreIntegrationTests",
                                         "OtherTarget.CherriCoreIntegrationTests"),
            "failed_case": self.rp40_block(status="failed"),
            "other_failed_case": good + self.block(service=True, status="failed"),
        }
        for name, text in cases.items():
            with self.subTest(name=name):
                _, records, _ = self.rp40_collect(text)
                self.assertFalse(any("RP40" in r["requirements"] for r in records))

    def test_rp40_whole_process_and_diagnostic_only_guards_are_preserved(self):
        for options in ({"exit_code": 65}, {"exit_code": None},
                        {"run_id": "UNKNOWN"}, {"diagnostic_only": True}):
            with self.subTest(options=options):
                diagnostic, records, _ = self.rp40_collect(self.rp40_block(), **options)
                self.assertEqual(records, [])
                self.assertNotEqual(diagnostic["appIntegrationEvidence"]["status"],
                                    "VERIFIED_APP_INTEGRATION")

    def test_rp40_missing_source_fixture_never_becomes_app_evidence(self):
        original = collector.RP40_APP_FIXTURE
        try:
            collector.RP40_APP_FIXTURE = "ios/CherriTests/RP40MissingFixture.swift"
            diagnostic, records, _ = self.rp40_collect(self.rp40_block())
        finally:
            collector.RP40_APP_FIXTURE = original
        self.assertEqual(records, [])
        self.assertEqual(diagnostic["appIntegrationEvidence"]["status"], "SOURCE_FIXTURE_UNAVAILABLE")

    def test_rp40_checkpoint_does_not_publish_arbitrary_text_or_private_paths(self):
        secret = "PRIVATE_RP40_DOCUMENT /Users/Private/source.cherri ghp_FAKE_SECRET_TOKEN"
        _, records, outputs = self.rp40_collect(self.rp40_block(body=secret + "\n" + collector.RP40_APP_CHECKPOINT))
        self.assertEqual(len(records), 1)
        self.assertNotIn(secret, "".join(outputs.values()))
        _, records, outputs = self.rp40_collect(
            self.rp40_block(body=collector.RP40_APP_CHECKPOINT + " " + secret))
        self.assertEqual(records, [])
        self.assertNotIn(secret, "".join(outputs.values()))

    def startup_receipt(self, **updates):
        value = {"schemaVersion": "1.0", "testName": f"CherriEditorUITests/{self.ui_method}",
                 "stage": "document-wait", "waitCompleted": False,
                 "windowExists": True, "appRunningForeground": True,
                 "createDocumentExists": True, "createDocumentHittable": False,
                 "editorExists": False, "editorHittable": False}
        value.update(updates)
        return "[CHERRI_UI_STARTUP] " + json.dumps(value, ensure_ascii=False)

    def startup_timing(self, **updates):
        value = {"schemaVersion": "1.0", "testName": f"CherriEditorUITests/{self.ui_method}",
                 "budgetMilliseconds": 28000, "elapsedMilliseconds": 31000,
                 "droppedEntries": 0, "inFlightObservations": 0, "entries": [
                     {"sequence": 0, "phase": "editor-wait", "operation": "editor-exists",
                      "startedMilliseconds": 19000, "completedMilliseconds": 30000,
                      "value": True, "mainThread": False},
                     {"sequence": 1, "phase": "editor-wait", "operation": "returned",
                      "startedMilliseconds": 31000, "completedMilliseconds": 31000,
                      "value": False, "mainThread": True}]}
        value.update(updates)
        return collector.STARTUP_TIMING_PREFIX + json.dumps(value)

    def test_startup_query_timing_preserves_late_observations_without_acceptance(self):
        line = self.startup_timing()
        diagnostic, records, outputs = self.collect(self.block(status="failed", body=line), 65)
        self.assertEqual(records, [])
        self.assertEqual(diagnostic["runStatus"], "FAILED")
        trace = diagnostic["startupTimingTraces"][0]
        self.assertTrue(trace["entries"][0]["value"])
        self.assertFalse(trace["entries"][1]["value"])
        self.assertEqual(trace["entries"][0]["completedMilliseconds"], 30000)
        self.assertEqual(trace["budgetMilliseconds"], 28000)
        self.assertIn("startupTimingTraces", outputs["ios-test-diagnostics.json"])
        # Even a passed test boundary and a timing trace are not a UI checkpoint.
        self.assertEqual(self.collect(self.block(body=line))[1], [])

    def test_startup_query_timing_rejects_impossible_or_private_receipts(self):
        original = json.loads(self.startup_timing()[len(collector.STARTUP_TIMING_PREFIX):])
        mutations = []
        for key, value in (("budgetMilliseconds", 29000), ("elapsedMilliseconds", True),
                           ("elapsedMilliseconds", -1), ("droppedEntries", 1),
                           ("inFlightObservations", False), ("privateTitle", "PRIVATE_TIMING_CANARY")):
            changed = json.loads(json.dumps(original)); changed[key] = value; mutations.append(changed)
        for key, value in (("phase", "private-document"), ("operation", "title"),
                           ("value", 1), ("mainThread", 1), ("sequence", True),
                           ("startedMilliseconds", 30001), ("completedMilliseconds", 31001),
                           ("source", "PRIVATE_TIMING_CANARY")):
            changed = json.loads(json.dumps(original)); changed["entries"][0][key] = value; mutations.append(changed)
        changed = json.loads(json.dumps(original)); changed["entries"][1]["startedMilliseconds"] = 30001; mutations.append(changed)
        for changed in mutations:
            with self.subTest(changed=changed):
                diagnostic, records, outputs = self.collect(self.block(status="failed",
                    body=collector.STARTUP_TIMING_PREFIX + json.dumps(changed)), 65)
                self.assertEqual(diagnostic["startupTimingTraces"], [])
                self.assertEqual(records, [])
                self.assertNotIn("PRIVATE_TIMING_CANARY", "".join(outputs.values()))

    def test_filtered_startup_query_timing_remains_bound_to_known_phases(self):
        original = json.loads(self.startup_timing()[len(collector.STARTUP_TIMING_PREFIX):])
        for phase in ("window-snapshot", "document-snapshot", "final-snapshot", "document-wait", "editor-wait"):
            changed = json.loads(json.dumps(original))
            changed["entries"][0].update(phase=phase, operation="known-controls")
            line = collector.STARTUP_TIMING_PREFIX + json.dumps(changed)
            diagnostic, records, _ = self.collect(self.block(status="failed", body=line), 65)
            self.assertEqual(diagnostic["startupTimingTraces"], [changed])
            self.assertEqual(records, [])
        for phase in ("document-tap", "window-wait", "private-document"):
            changed["entries"][0]["phase"] = phase
            line = collector.STARTUP_TIMING_PREFIX + json.dumps(changed)
            self.assertEqual(self.collect(self.block(status="failed", body=line), 65)[0]["startupTimingTraces"], [])

    def test_startup_timing_can_retain_overlap_and_an_inflight_query(self):
        original = json.loads(self.startup_timing()[len(collector.STARTUP_TIMING_PREFIX):])
        original["entries"].reverse()
        for index, entry in enumerate(original["entries"]): entry["sequence"] = index
        original["inFlightObservations"] = 1
        diagnostic, records, _ = self.collect(self.block(status="failed",
            body=collector.STARTUP_TIMING_PREFIX + json.dumps(original)), 65)
        self.assertEqual(diagnostic["startupTimingTraces"], [original])
        self.assertEqual(records, [])

    def test_startup_timing_requires_its_active_test_and_one_bounded_trace(self):
        line = self.startup_timing()
        self.assertEqual(self.collect(line, 65)[0]["startupTimingTraces"], [])
        self.assertEqual(self.collect(self.block(method="testOther", body=line), 65)[0]["startupTimingTraces"], [])
        log = self.block(status="failed", body="\n".join([line]*20))
        self.assertEqual(len(self.collect(log, 65)[0]["startupTimingTraces"]), 1)
        log = "".join(self.block(method=f"testTiming{index}", status="failed",
            body=self.startup_timing(testName=f"CherriEditorUITests/testTiming{index}")) for index in range(20))
        self.assertEqual(len(self.collect(log, 65)[0]["startupTimingTraces"]), 16)

    def test_startup_timing_explicitly_reports_capacity_and_rejects_duplicate_keys(self):
        row = json.loads(self.startup_timing()[len(collector.STARTUP_TIMING_PREFIX):])
        first = row["entries"][0]
        row["entries"] = [dict(first, sequence=index) for index in range(128)]
        row["droppedEntries"] = 2
        line = collector.STARTUP_TIMING_PREFIX + json.dumps(row)
        self.assertEqual(self.collect(self.block(status="failed", body=line), 65)[0]["startupTimingTraces"], [row])
        duplicate = line.replace('"schemaVersion": "1.0"', '"schemaVersion": "private", "schemaVersion": "1.0"')
        self.assertEqual(self.collect(self.block(status="failed", body=duplicate), 65)[0]["startupTimingTraces"], [])
        row["entries"].append(dict(first, sequence=128))
        self.assertEqual(self.collect(self.block(status="failed", body=collector.STARTUP_TIMING_PREFIX + json.dumps(row)), 65)[0]["startupTimingTraces"], [])


    def test_scoped_editor_startup_receipt_retains_explicit_unobserved_creation_without_acceptance(self):
        receipt = self.startup_receipt(schemaVersion="1.1", stage="editor-wait",
            controlScope="editor-only", waitCompleted=True, windowExists=True,
            appRunningForeground=True, createDocumentExists=False, createDocumentHittable=False,
            editorExists=True, editorHittable=True, documentCreationIssued=True,
            elapsedMilliseconds=27000, budgetMilliseconds=28000,
            actualSource="PRIVATE_SCOPE_CANARY", documentLabel="PRIVATE_SCOPE_CANARY")
        diagnostic, records, outputs = self.collect(self.block(status="failed", body=receipt), 65)
        saved = diagnostic["startupReceipts"]
        self.assertEqual(len(saved), 1)
        self.assertEqual(saved[0]["schemaVersion"], "1.1")
        self.assertEqual(saved[0]["controlScope"], "editor-only")
        self.assertFalse(saved[0]["createDocumentExists"])
        self.assertEqual(records, [])
        self.assertNotIn("PRIVATE_SCOPE_CANARY", "".join(outputs.values()))

    def test_scoped_startup_receipt_cannot_hide_creation_or_change_stage_budget_and_scope(self):
        raw = dict(schemaVersion="1.1", stage="editor-wait", controlScope="editor-only",
            createDocumentExists=False, createDocumentHittable=False, documentCreationIssued=True,
            elapsedMilliseconds=27000, budgetMilliseconds=28000)
        controls = (
            {"controlScope": "PRIVATE_SCOPE"}, {"controlScope": ["editor-only"]},
            {"stage": "window-wait"}, {"stage": "document-wait"}, {"stage": "document-check"},
            {"documentCreationIssued": False}, {"documentCreationIssued": 1},
            {"createDocumentExists": True}, {"createDocumentHittable": True},
            {"budgetMilliseconds": 28001}, {"elapsedMilliseconds": -1}, {"schemaVersion": "1.0"},
        )
        for mutation in controls:
            with self.subTest(mutation=mutation):
                diagnostic, records, outputs = self.collect(
                    self.block(status="failed", body=self.startup_receipt(**{**raw, **mutation})), 65)
                self.assertEqual(diagnostic.get("startupReceipts", []), [])
                self.assertEqual(records, [])
                self.assertNotIn("PRIVATE_SCOPE", "".join(outputs.values()))
        for missing in ("controlScope", "elapsedMilliseconds", "budgetMilliseconds", "documentCreationIssued"):
            changed = dict(raw)
            changed.pop(missing)
            diagnostic, records, _ = self.collect(
                self.block(status="failed", body=self.startup_receipt(**changed)), 65)
            self.assertEqual(diagnostic.get("startupReceipts", []), [])
            self.assertEqual(records, [])

    def test_current_full_startup_scope_and_original_legacy_receipts_are_preserved(self):
        for extra in ({}, {"schemaVersion": "1.1", "controlScope": "document-and-editor",
                          "elapsedMilliseconds": 5000, "budgetMilliseconds": 28000,
                          "documentCreationIssued": False}):
            diagnostic, records, _ = self.collect(
                self.block(status="failed", body=self.startup_receipt(**extra)), 65)
            self.assertEqual(len(diagnostic.get("startupReceipts", [])), 1)
            self.assertEqual(records, [])

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

    def test_direct_document_check_retains_actual_observation_without_inventing_wait(self):
        receipt = self.startup_receipt(stage="document-check", waitCompleted=False,
            createDocumentHittable=True, elapsedMilliseconds=4743,
            budgetMilliseconds=28000, documentCreationIssued=False)
        diagnostic, records, _ = self.collect(self.block(status="failed", body=receipt), 65)
        self.assertEqual(len(diagnostic["startupReceipts"]), 1)
        self.assertFalse(diagnostic["startupReceipts"][0]["waitCompleted"])
        self.assertTrue(diagnostic["startupReceipts"][0]["createDocumentHittable"])
        self.assertEqual(records, [])
        for changes in ({"waitCompleted": True}, {"budgetMilliseconds": 28001},
                        {"documentCreationIssued": 1}, {"elapsedMilliseconds": -1}):
            with self.subTest(changes=changes):
                changed = json.loads(receipt[len(collector.STARTUP_PREFIX):])
                changed.update(changes)
                diagnostic, records, _ = self.collect(self.block(status="failed",
                    body=collector.STARTUP_PREFIX + json.dumps(changed)), 65)
                self.assertEqual(diagnostic["startupReceipts"], [])
                self.assertEqual(records, [])
        missing_time = self.startup_receipt(stage="document-check", waitCompleted=False)
        diagnostic, records, _ = self.collect(self.block(status="failed", body=missing_time), 65)
        self.assertEqual(diagnostic["startupReceipts"], [])
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

    def test_paste_native_notification_and_predicate_match_remain_diagnostics(self):
        method = "testSingleCharacterPasteDoesNotConsumeGeneratedText"
        target = "CherriEditorInputTests/" + method
        start = f"Test Case '-[CherriTests.CherriEditorInputTests {method}]' started.\n"
        finish = f"Test Case '-[CherriTests.CherriEditorInputTests {method}]' failed (7.03 seconds).\n"
        for stage, index in (("paste-native-text-change", 1), ("paste-predicate-match", 0)):
            with self.subTest(stage=stage):
                data = dict(testName=target, fixtureId="single-character-paste", stage=stage,
                            keyIndex=index, actualSource='text("x"))', pasteElapsedMilliseconds=2890,
                            viewIsFirstResponder=True, windowAttached=True, windowIsKey=True)
                line = self.input_receipt(**data)
                diagnostic, records, _ = self.collect(start + line + "\n" + finish, 65)
                self.assertEqual(records, [])
                self.assertEqual(diagnostic["testResults"][0]["observedStatus"], "failed")
                self.assertEqual(diagnostic["inputReceipts"][0]["pasteElapsedMilliseconds"], 2890)
                self.assertEqual(diagnostic["inputReceipts"][0]["actualSource"], 'text("x"))')
                self.assertEqual(self.collect(line, 65)[0]["inputReceipts"], [])
                bad = self.input_receipt(**dict(data, keyIndex=5))
                self.assertEqual(self.collect(start + bad + "\n" + finish, 65)[0]["inputReceipts"], [])
                private = self.input_receipt(**dict(data, actualSource="PRIVATE_PASTE_CONTENT"))
                _, _, outputs = self.collect(start + private + "\n" + finish, 65)
                self.assertNotIn("PRIVATE_PASTE_CONTENT", "".join(outputs.values()))

    def stock_paste_receipt(self, **updates):
        row = dict(schemaVersion="1.0",
                   testName="CherriEditorInputTests/testSingleCharacterPasteDoesNotConsumeGeneratedText",
                   fixtureId="stock-ui-text-view-after-code-view", stage="after-paste-wait",
                   ordering="AFTER_ORIGINAL_CODEVIEW_ASSERTION", scope="DIAGNOSTIC_ONLY_WARM_PROCESS",
                   isolatedSyntheticEditor=True, budgetMilliseconds=3000, elapsedMilliseconds=3500,
                   actualSource='text("x"))', actualUTF16Length=10, selectionLocation=9, selectionLength=0,
                   viewIsFirstResponder=True, windowAttached=True, windowIsKey=True,
                   syntheticClipboardVerified=True, pasteIssued=True, waitResult="TIMED_OUT", exactExpectedText=True)
        row.update(updates)
        return collector.STOCK_PASTE_PREFIX + json.dumps(row)

    def stock_paste_log(self, body, *, original_wait=True, status="failed"):
        method = "testSingleCharacterPasteDoesNotConsumeGeneratedText"
        start = f"Test Case '-[CherriTests.CherriEditorInputTests {method}]' started.\n"
        finish = f"Test Case '-[CherriTests.CherriEditorInputTests {method}]' {status} (7.03 seconds).\n"
        prior = self.input_receipt(testName="CherriEditorInputTests/" + method,
            fixtureId="single-character-paste", stage="after-paste-wait", actualSource='text("x"))',
            actualUTF16Length=10, pasteElapsedMilliseconds=3592, viewIsFirstResponder=True,
            windowAttached=True, windowIsKey=True) if original_wait else ""
        return start + prior + "\n" + body + "\n" + finish

    def test_stock_paste_is_separate_and_never_promotes_the_original_failure(self):
        for result in ("COMPLETED", "TIMED_OUT"):
            line = self.stock_paste_receipt(waitResult=result)
            diagnostic, records, _ = self.collect(self.stock_paste_log(line), 65)
            self.assertEqual(records, [])
            self.assertEqual(diagnostic["runStatus"], "FAILED")
            self.assertEqual(diagnostic["testResults"][0]["observedStatus"], "failed")
            self.assertEqual(diagnostic["stockPasteDiagnostics"][0]["waitResult"], result)
            self.assertEqual(diagnostic["stockPasteDiagnostics"][0]["elapsedMilliseconds"], 3500)
            self.assertEqual(diagnostic["stockPasteDiagnostics"][0]["scope"], "DIAGNOSTIC_ONLY_WARM_PROCESS")
        self.assertEqual(self.collect(self.stock_paste_log(line, status="passed"))[1], [])

    def test_stock_paste_requires_the_original_wait_and_same_active_test(self):
        line = self.stock_paste_receipt()
        for log in (line, self.stock_paste_log(line, original_wait=False),
                    self.block(body=line), self.stock_paste_log(line).replace(
                        "testSingleCharacterPasteDoesNotConsumeGeneratedText", "testOther")):
            self.assertEqual(self.collect(log, 65)[0]["stockPasteDiagnostics"], [])

    def test_stock_paste_rejects_unknown_fields_malformed_types_and_scope_changes(self):
        mutations = [
            {"budgetMilliseconds": 3001}, {"elapsedMilliseconds": True}, {"elapsedMilliseconds": 120001},
            {"isolatedSyntheticEditor": False}, {"ordering": "BEFORE_ORIGINAL"}, {"scope": "ACCEPTANCE"},
            {"fixtureId": "private-view"}, {"stage": "private-stage"}, {"waitResult": "PASS"},
            {"pasteIssued": False}, {"syntheticClipboardVerified": False}, {"windowIsKey": 1},
            {"actualUTF16Length": 9}, {"selectionLocation": -1}, {"selectionLength": 2},
            {"exactExpectedText": False}, {"privateTitle": "PRIVATE_STOCK_CANARY"},
        ]
        for mutation in mutations:
            with self.subTest(mutation=mutation):
                diagnostic, records, outputs = self.collect(self.stock_paste_log(self.stock_paste_receipt(**mutation)), 65)
                self.assertEqual(diagnostic["stockPasteDiagnostics"], [])
                self.assertEqual(records, [])
                self.assertNotIn("PRIVATE_STOCK_CANARY", "".join(outputs.values()))
        duplicate = self.stock_paste_receipt().replace('"schemaVersion": "1.0"',
            '"schemaVersion": "private", "schemaVersion": "1.0"')
        self.assertEqual(self.collect(self.stock_paste_log(duplicate), 65)[0]["stockPasteDiagnostics"], [])

    def test_stock_paste_retains_only_fixed_values_and_bounded_events(self):
        secret = "PRIVATE_STOCK_CANARY"
        line = self.stock_paste_receipt(actualSource=secret, actualUTF16Length=len(secret), exactExpectedText=False)
        diagnostic, records, outputs = self.collect(self.stock_paste_log(line), 65)
        self.assertEqual(records, [])
        self.assertEqual(diagnostic["stockPasteDiagnostics"][0]["actualSource"], "[REDACTED_UNEXPECTED_CONTENT]")
        self.assertNotIn(secret, "".join(outputs.values()))
        log = self.stock_paste_log("\n".join(self.stock_paste_receipt() for _ in range(12)))
        self.assertEqual(len(self.collect(log, 65)[0]["stockPasteDiagnostics"]), 8)

    def test_stock_paste_stages_retain_real_setup_and_native_event_boundaries(self):
        for stage in ("setup-failed", "before-paste", "after-paste-call", "native-text-change"):
            issued = stage not in {"setup-failed", "before-paste"}
            row = self.stock_paste_receipt(stage=stage, waitResult="NOT_WAITED", pasteIssued=issued)
            diagnostic, records, _ = self.collect(self.stock_paste_log(row), 65)
            self.assertEqual(records, [])
            self.assertEqual(diagnostic["stockPasteDiagnostics"][0]["stage"], stage)
            bad = self.stock_paste_receipt(stage=stage, waitResult="COMPLETED", pasteIssued=issued)
            self.assertEqual(self.collect(self.stock_paste_log(bad), 65)[0]["stockPasteDiagnostics"], [])

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


    def rp40_ui_block(self, *, body=None, status="passed", target="CherriUITests.CherriRP40UITests"):
        method = collector.RP40_UI_TEST.split("/")[1]
        if body is None:
            body = "\n".join(collector.RP40_UI_STAGE_PREFIX + stage for stage in collector.RP40_UI_STAGES)
            body += "\n" + collector.RP40_UI_CHECKPOINT
        return (f"Test Case '-[{target} {method}]' started.\n{body}\n"
                f"Test Case '-[{target} {method}]' {status} (0.123 seconds).\n")

    def test_rp40_ui_is_dedicated_and_source_bound(self):
        diagnostic, records, outputs = self.collect(self.rp40_ui_block())
        self.assertEqual(len(records), 1)
        record = records[0]
        self.assertEqual(record["requirements"], ["RP40"])
        self.assertEqual(record["tier"], "ios-ui")
        self.assertEqual(record["test_id"], collector.RP40_UI_TEST)
        self.assertEqual(record["assertions"], collector.RP40_UI_ASSERTIONS)
        self.assertEqual(record["detail"], collector.RP40_UI_SCOPE)
        self.assertEqual({Path(a["path"]).name for a in record["artifacts"]},
                         {"xcodetest.log", "ios-test-diagnostics.json"})
        for artifact in record["artifacts"]:
            self.assertEqual(artifact["sha256"],
                             hashlib.sha256(outputs[Path(artifact["path"]).name].encode()).hexdigest())
        self.assertEqual(record["fixture_sha256"],
                         hashlib.sha256((SCRIPT.parents[2] / collector.RP40_UI_FIXTURE).read_bytes()).hexdigest())
        self.assertEqual(diagnostic["appIntegrationUIEvidence"]["status"], "VERIFIED_APP_UI")
        self.assertIs(diagnostic["appIntegrationUIEvidence"]["appIntegrationTierGranted"], False)
        self.assertEqual(diagnostic["appIntegrationEvidence"]["status"], "NOT_OBSERVED")

    def test_rp40_ui_and_hosted_app_remain_separate_records(self):
        for log in (self.rp40_block() + self.rp40_ui_block(),
                    self.rp40_ui_block() + self.rp40_block()):
            with self.subTest(order=log[:90]):
                diagnostic, records, _ = self.collect(log)
                self.assertEqual(len(records), 2)
                self.assertEqual({(r["tier"], r["test_id"]) for r in records},
                                 {("app-integration", collector.RP40_APP_TEST),
                                  ("ios-ui", collector.RP40_UI_TEST)})
                self.assertTrue(all(row["observedStatus"] == "passed" for row in diagnostic["testResults"]))

    def test_rp40_ui_requires_all_ordered_stages_and_exact_marker_in_its_case(self):
        stage_lines = [collector.RP40_UI_STAGE_PREFIX + stage for stage in collector.RP40_UI_STAGES]
        good = "\n".join(stage_lines + [collector.RP40_UI_CHECKPOINT])
        mutations = {
            "missing_stage": self.rp40_ui_block(body="\n".join(stage_lines[1:] + [collector.RP40_UI_CHECKPOINT])),
            "duplicate_stage": self.rp40_ui_block(body="\n".join(stage_lines[:1] + stage_lines + [collector.RP40_UI_CHECKPOINT])),
            "reordered_stage": self.rp40_ui_block(body="\n".join(list(reversed(stage_lines)) + [collector.RP40_UI_CHECKPOINT])),
            "unknown_stage": self.rp40_ui_block(body=good.replace("semantic-rename", "UNREVIEWED_STAGE")),
            "stage_suffix": self.rp40_ui_block(body=good.replace("semantic-rename", "semantic-rename PRIVATE_TOKEN")),
            "missing_marker": self.rp40_ui_block(body="\n".join(stage_lines)),
            "duplicate_marker": self.rp40_ui_block(body=good + "\n" + collector.RP40_UI_CHECKPOINT),
            "marker_suffix": self.rp40_ui_block(body=good + " PRIVATE_TOKEN"),
            "marker_outside": collector.RP40_UI_CHECKPOINT + "\n" + self.rp40_ui_block(body="\n".join(stage_lines)),
            "marker_before_stages": self.rp40_ui_block(body=collector.RP40_UI_CHECKPOINT + "\n" + "\n".join(stage_lines)),
            "other_target": self.rp40_ui_block(target="CherriTests.CherriRP40UITests"),
            "other_class": self.rp40_ui_block(target="CherriUITests.CherriEditorUITests"),
            "borrowed_completion": self.block(body=self.checkpoint + "\n" + good),
            "duplicate_case": self.rp40_ui_block() * 2,
            "failed_case": self.rp40_ui_block(status="failed"),
            "incomplete_case": self.rp40_ui_block().split(" passed (")[0],
        }
        for name, log in mutations.items():
            with self.subTest(name=name):
                self.assertEqual(len(self.collect(self.rp40_ui_block())[1]), 1)
                _, records, outputs = self.collect(log)
                self.assertFalse(any(r["tier"] == "ios-ui" and "RP40" in r["requirements"] for r in records))
                self.assertNotIn("PRIVATE_TOKEN", "".join(outputs.values()))

    def test_rp40_ui_process_failure_and_diagnostic_only_never_grant_credit(self):
        for exit_code in (65, None):
            diagnostic, records, _ = self.collect(self.rp40_ui_block(), exit_code)
            self.assertEqual(records, [])
            self.assertNotEqual(diagnostic["appIntegrationUIEvidence"]["status"], "VERIFIED_APP_UI")
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "raw").write_text(self.rp40_ui_block())
            (root / "exit").write_text("0")
            with contextlib.redirect_stdout(io.StringIO()):
                diagnostic, records = collector.collect(root / "raw", root / "out", "a" * 40,
                                                        "1234", root / "exit", diagnostic_only=True)
            self.assertEqual(records, [])
            self.assertTrue(diagnostic["diagnosticOnly"])

    def test_rp40_ui_missing_source_and_unknown_source_do_not_receive_credit(self):
        original = collector.RP40_UI_FIXTURE
        try:
            collector.RP40_UI_FIXTURE = "ios/CherriUITests/AbsentRP40.swift"
            diagnostic, records, _ = self.collect(self.rp40_ui_block())
        finally:
            collector.RP40_UI_FIXTURE = original
        self.assertEqual(records, [])
        self.assertEqual(diagnostic["appIntegrationUIEvidence"]["status"], "SOURCE_FIXTURE_UNAVAILABLE")
        self.assertEqual(self.collect(self.rp40_ui_block(), sha="UNKNOWN")[1], [])

    def test_rp40_ui_publishes_only_fixed_stage_tokens_and_failure_metadata(self):
        secret = "PRIVATE_RP40_UI_VALUE /Users/Private/source.cherri ghp_FAKE_TOKEN"
        body = secret + "\n" + "\n".join(collector.RP40_UI_STAGE_PREFIX + x for x in collector.RP40_UI_STAGES)
        body += "\n" + collector.RP40_UI_CHECKPOINT
        _, records, outputs = self.collect(self.rp40_ui_block(body=body))
        self.assertEqual(len(records), 1)
        self.assertNotIn(secret, "".join(outputs.values()))
        failure = ("CherriRP40UITests.swift:246: error: -[CherriUITests.CherriRP40UITests "
                   "testRP40ActualAppDocumentWorkflowUI] : XCTAssertTrue failed - " + secret)
        diagnostic, records, outputs = self.collect(self.rp40_ui_block(body=failure, status="failed"), 65)
        self.assertEqual(records, [])
        self.assertEqual(diagnostic["uiFailures"], [{
            "testId": collector.RP40_UI_TEST, "sourceLine": 246, "failureCategory": "ASSERTION_FAILURE"}])
        self.assertNotIn(secret, "".join(outputs.values()))




class E07PaneObservationTests(unittest.TestCase):
    prefix = "[CHERRI_E07_PANE_TRANSITION] "
    rejected = "[CHERRI_E07_PANE_REJECTED] INVALID_TRANSITION"
    test_id = "CherriEditorUITests/testE07VisualParameterEditingCanvasUI"
    method = "testE07VisualParameterEditingCanvasUI"
    stages = (("initial-preview", "Preview"), ("edited-code", "Code"),
              ("invalid-preview", "Preview"), ("rejected-code", "Code"),
              ("reopened-preview", "Preview"), ("reopened-code", "Code"))
    checkpoint = "[CHECKPOINT:E07_PREVIEW_EDIT_VERIFIED]"

    def row(self, index, phase, dual=False):
        stage, target = self.stages[index]
        return {
            "schema": "cherri-e07-pane-transition/1", "testId": self.test_id,
            "stage": stage, "target": target, "phase": phase,
            "selectorCount": 0 if dual else 1, "codeCount": 0 if dual else 1,
            "previewCount": 0 if dual else 1, "targetHittable": not dual,
            "targetSelected": phase == "selected", "editorExists": True,
            "previewExists": True, "dualPaneVisible": dual,
            "tapIssued": phase in ("selected", "not-selected"),
            "budgetMilliseconds": 3000, "elapsedMilliseconds": 100 if phase == "ready" else 200,
        }

    def lines(self, dual=False):
        return [self.prefix + json.dumps(self.row(i, phase, dual), sort_keys=True)
                for i in range(6) for phase in ("ready", "dual-pane" if dual else "selected")]

    def block(self, lines, status="passed", target="CherriUITests.CherriEditorUITests", method=None):
        method = method or self.method
        return (f"Test Case '-[{target} {method}]' started.\n"
                + "\n".join(lines) + "\n" + self.checkpoint + "\n"
                + f"Test Case '-[{target} {method}]' {status} (0.123 seconds).\n")

    def retained(self, raw):
        return collector.bounded_log(raw)[0]

    def observation_lines(self, retained):
        return [line for line in retained.splitlines() if line.startswith(self.prefix)]

    def test_exact_segmented_observations_are_retained(self):
        lines = self.lines()
        actual = self.retained(self.block(lines))
        self.assertEqual(self.observation_lines(actual), lines)
        self.assertNotIn(self.rejected, actual)
        self.assertIn(self.checkpoint, actual)

    def test_proved_dual_pane_observations_are_retained(self):
        lines = self.lines(dual=True)
        actual = self.retained(self.block(lines))
        self.assertEqual(self.observation_lines(actual), lines)
        self.assertTrue(all(not json.loads(line[len(self.prefix):])["tapIssued"] for line in lines))

    def test_accepted_observation_bytes_are_not_reconstructed(self):
        lines = self.lines()
        lines[0] = self.prefix + json.dumps(self.row(0, "ready"), separators=(", ", ": "))
        self.assertEqual(self.observation_lines(self.retained(self.block(lines))), lines)

    def test_old_E07_log_without_new_observations_is_unchanged(self):
        raw = self.block([])
        self.assertEqual(self.retained(raw), raw.replace("\n\n", "\n"))

    def test_failed_readiness_is_retained_without_success_credit(self):
        row = self.row(0, "not-ready")
        row.update(selectorCount=2, codeCount=0, previewCount=0,
                   targetHittable=False, elapsedMilliseconds=3100)
        line = self.prefix + json.dumps(row)
        diagnostic, records, outputs = IOSTestEvidenceTests().collect(self.block([line], "failed"), 65)
        self.assertEqual(self.observation_lines(outputs["xcodetest.log"]), [line])
        self.assertEqual(diagnostic["runStatus"], "FAILED")
        self.assertEqual(records, [])

    def test_failed_selection_retains_the_actual_tap_and_elapsed(self):
        rows = [self.row(0, "ready"), self.row(0, "not-selected")]
        rows[1]["elapsedMilliseconds"] = 3200
        lines = [self.prefix + json.dumps(row) for row in rows]
        diagnostic, records, outputs = IOSTestEvidenceTests().collect(self.block(lines, "failed"), 65)
        self.assertEqual(self.observation_lines(outputs["xcodetest.log"]), lines)
        self.assertEqual(diagnostic["testResults"], [{"testId": self.test_id, "observedStatus": "failed"}])
        self.assertEqual(records, [])

    def test_fixed_schema_causal_corruptions_are_excluded(self):
        faults = {
            "schema": {"schema": "private"}, "test": {"testId": "Other/test"},
            "stage": {"stage": "PRIVATE_CANARY"}, "target": {"target": "Other"},
            "phase": {"phase": "OTHER"}, "extra_source": {"source": "PRIVATE_CANARY"},
            "extra_AX": {"axTree": "PRIVATE_CANARY"}, "count_boolean": {"selectorCount": True},
            "count_negative": {"codeCount": -1}, "count_large": {"previewCount": 33},
            "string_boolean": {"targetHittable": "PRIVATE_CANARY"},
            "budget_boolean": {"budgetMilliseconds": True}, "budget_drift": {"budgetMilliseconds": 3001},
            "elapsed_negative": {"elapsedMilliseconds": -1}, "elapsed_float": {"elapsedMilliseconds": 1.0},
            "elapsed_large": {"elapsedMilliseconds": 60001},
        }
        for name, change in faults.items():
            with self.subTest(fault=name):
                row = self.row(0, "ready"); row.update(change)
                actual = self.retained(self.block([self.prefix + json.dumps(row)]))
                self.assertEqual(self.observation_lines(actual), [])
                self.assertEqual(actual.count(self.rejected), 1)
                self.assertNotIn("PRIVATE_CANARY", actual)
                self.assertIn("passed (0.123 seconds).", actual)

    def test_duplicate_keys_malformed_and_oversized_JSON_are_excluded(self):
        valid = json.dumps(self.row(0, "ready"))
        cases = ["{invalid", "null", "[]", '{"schema":"wrong",' + valid[1:],
                 valid[:-1] + ',"private":"' + "PRIVATE_CANARY" * 200 + '"}',
                 "[" * 1000 + "]" * 1000]
        for raw in cases:
            with self.subTest(shape=raw[:20]):
                actual = self.retained(self.block([self.prefix + raw]))
                self.assertEqual(self.observation_lines(actual), [])
                self.assertEqual(actual.count(self.rejected), 1)
                self.assertNotIn("PRIVATE_CANARY", actual)

    def test_wrong_test_or_target_never_admits_observations(self):
        for target, method in (("CherriTests.CherriEditorUITests", self.method),
                               ("CherriUITests.CherriEditorUITests", "testE01CompletionDisplayAndInsertionUI")):
            with self.subTest(target=target, method=method):
                actual = self.retained(self.block(self.lines(), target=target, method=method))
                self.assertEqual(self.observation_lines(actual), [])
                self.assertIn(self.rejected, actual)

    def test_outside_test_and_forged_rejection_retain_no_payload(self):
        raw = self.lines()[0] + "\n[CHERRI_E07_PANE_REJECTED] PRIVATE_CANARY\n"
        actual = self.retained(raw)
        self.assertEqual(actual, self.rejected + "\n")
        self.assertNotIn("PRIVATE_CANARY", actual)

    def test_transition_state_faults_are_rejected(self):
        ready, selected = self.row(0, "ready"), self.row(0, "selected")
        faults = {
            "no_ready": [selected],
            "duplicate_ready": [ready, ready],
            "wrong_order": [self.row(1, "ready")],
            "missing_unique_control": [dict(ready, codeCount=2)],
            "early_tap": [dict(ready, tapIssued=True)],
            "selected_without_tap": [ready, dict(selected, tapIssued=False)],
            "selected_without_selected": [ready, dict(selected, targetSelected=False)],
            "selected_wrong_control": [ready, dict(selected, previewCount=0)],
            "success_after_budget": [ready, dict(selected, elapsedMilliseconds=3001)],
            "reversed_time": [ready, dict(selected, elapsedMilliseconds=99)],
            "unproved_dual": [dict(self.row(0, "ready", True), editorExists=False)],
            "wrong_terminal_branch": [ready, self.row(0, "dual-pane", True)],
        }
        for name, rows in faults.items():
            with self.subTest(fault=name):
                actual = self.retained(self.block([self.prefix + json.dumps(row) for row in rows]))
                self.assertIn(self.rejected, actual)
                self.assertLessEqual(len(self.observation_lines(actual)), 1)

    def test_duplicate_stage_and_extra_events_are_bounded(self):
        lines = self.lines()
        actual = self.retained(self.block(lines + [lines[0]] * 64))
        self.assertEqual(self.observation_lines(actual), lines)
        self.assertEqual(actual.count(self.rejected), 1)
        self.assertLess(len(actual.encode()), 20000)

    def test_stage_cannot_continue_across_test_boundaries(self):
        raw = self.block([self.lines()[0]], "failed") + self.block([self.lines()[1]])
        actual = self.retained(raw)
        self.assertEqual(self.observation_lines(actual), [self.lines()[0]])
        self.assertIn(self.rejected, actual)
        self.assertIn("failed (0.123 seconds).", actual)
        self.assertIn("passed (0.123 seconds).", actual)

    def test_missing_stage_on_a_passed_test_is_diagnostic_rejection(self):
        actual = self.retained(self.block(self.lines()[:-2]))
        self.assertEqual(self.observation_lines(actual), self.lines()[:-2])
        self.assertIn(self.rejected, actual)
        self.assertIn("passed (0.123 seconds).", actual)

    def test_original_failure_and_terminal_are_never_erased(self):
        failure = ("/Users/runner/project/ios/CherriUITests/CherriEditorUITests.swift:895: error: "
                   "-[CherriUITests.CherriEditorUITests " + self.method + "] : XCTAssertTrue failed")
        raw = self.block([self.prefix + "{invalid", failure], "failed")
        diagnostic, records, outputs = IOSTestEvidenceTests().collect(raw, 65)
        self.assertEqual(diagnostic["testResults"], [{"testId": self.test_id, "observedStatus": "failed"}])
        self.assertEqual(diagnostic["uiFailures"][0]["sourceLine"], 895)
        self.assertEqual(records, [])
        self.assertIn(self.rejected, outputs["xcodetest.log"])


    def test_padded_valid_observation_cannot_bypass_raw_byte_bound(self):
        valid = self.lines()[0]
        for raw in (" " * 2200 + valid, valid + "\t" * 2200):
            with self.subTest(padding="leading" if raw.startswith(" ") else "trailing"):
                actual = self.retained(self.block([raw], "failed"))
                self.assertEqual(self.observation_lines(actual), [])
                self.assertNotIn(raw, actual)
                self.assertIn(self.rejected, actual)



class PreviewTransactionRetentionTests(unittest.TestCase):
    collect = IOSTestEvidenceTests.collect
    suite = "CherriPreviewEditTransactionTests"
    methods = (
        "testSuccessfulEditPublishesOnlyAfterOneRebuildOfCapturedRequest",
        "testRejectedMappingAndFailedRebuildPreserveCommittedState",
        "testEachSnapshotChangeDuringEitherAwaitRejectsPublication",
        "testCancellationAfterMappingDoesNotRebuildOrPublish",
        "testLegacyDecompileFailureDoesNotPublishLocalPlistMutation",
        "testLiveTaskIdentityResumesChangedSourceAfterPendingEdit",
        "testQueuedDOMMessageCannotAcquireNewRenderIdentity",
        "testRealBridgeMapsExpandedNativeWaitAndPreservesExactSource",
        "testShowResultConvenienceSelectionUsesUniqueCompilerEntry",
        "testAutoLegacyRoutesByActualModeAndKeepsRequestedAutoContext",
        "testMultilineUnicodePreviewValueUsesCherriLiteralEscapes",
    )

    def transaction(self, method=None, status="passed", body="", target=None):
        method = method or self.methods[0]
        target = target or "CherriTests." + self.suite
        return (f"Test Case '-[{target} {method}]' started.\n{body}\n"
                f"Test Case '-[{target} {method}]' {status} (0.123 seconds).\n")

    def test_all_eleven_original_transaction_methods_keep_outcomes_without_credit(self):
        import re
        source = (SCRIPT.parents[1] / "CherriTests" / (self.suite + ".swift")).read_text()
        self.assertEqual(re.findall(r"func (test[A-Za-z0-9_]+)\(", source), list(self.methods))
        diagnostic, records, _ = self.collect("".join(self.transaction(name) for name in self.methods))
        self.assertEqual(diagnostic["runStatus"], "PASSED")
        self.assertEqual(diagnostic["testResults"], [
            {"testId": self.suite + "/" + name, "observedStatus": "passed"}
            for name in sorted(self.methods)])
        self.assertEqual(records, [])

    def test_transaction_failure_keeps_location_without_private_payload(self):
        method = self.methods[0]
        failure = (f"/Users/runner/{self.suite}.swift:89: error: "
                   f"-[CherriTests.{self.suite} {method}] : XCTAssertTrue failed - PRIVATE_CANARY")
        diagnostic, records, outputs = self.collect(
            self.transaction(status="failed", body=failure), 65)
        self.assertEqual(diagnostic["testResults"], [
            {"testId": self.suite + "/" + method, "observedStatus": "failed"}])
        self.assertEqual(diagnostic["uiFailures"], [{
            "testId": self.suite + "/" + method, "sourceLine": 89,
            "failureCategory": "ASSERTION_FAILURE"}])
        self.assertEqual(records, [])
        self.assertNotIn("PRIVATE_CANARY", "".join(outputs.values()))
        self.assertNotIn("/Users/runner", "".join(outputs.values()))

    def test_new_transaction_failure_requires_exact_bundle_class_and_active_case(self):
        method = self.methods[0]
        original = (f"/Users/runner/{self.suite}.swift:89: error: "
                    f"-[CherriTests.{self.suite} {method}] : PRIVATE_CANARY")
        variants = [
            ("wrong-bundle", self.transaction(target="CherriUITests." + self.suite,
                                              body=original.replace("CherriTests.", "CherriUITests."))),
            ("class-suffix", self.transaction(target="CherriTests." + self.suite + "Other",
                                              body=original.replace(self.suite, self.suite + "Other"))),
            ("outside-case", original),
            ("other-active-case", self.transaction(self.methods[1], body=original)),
        ]
        for label, log in variants:
            with self.subTest(label=label):
                diagnostic, records, outputs = self.collect(log, 65)
                self.assertEqual(diagnostic["uiFailures"], [])
                self.assertEqual(records, [])
                self.assertNotIn("PRIVATE_CANARY", "".join(outputs.values()))
                if label in {"wrong-bundle", "class-suffix", "outside-case"}:
                    self.assertEqual(diagnostic["testResults"], [])

if __name__ == "__main__":
    unittest.main()


class UIReadinessObservationTests(unittest.TestCase):
    collect = IOSTestEvidenceTests.collect
    prefix = "[CHERRI_UI_READINESS] "
    rejected = "[CHERRI_UI_READINESS_REJECTED] "
    sign_test = "CherriJoinedContractUITests/testBRG25ActualAppRejectedAndLiteralRoutesUI"
    field_test = "CherriPreviewDocumentUITests/testBRG28ActualWebKitFieldEditAndNumericRejectionUI"

    def row(self, boundary="brg25-malformed-sign", completed=False):
        sign = boundary.startswith("brg25-")
        return {
            "schema": "cherri-ui-readiness-observation/1",
            "testId": self.sign_test if sign else self.field_test,
            "boundary": boundary,
            "query": "button:signButton" if sign else "firstWebView:Text",
            "budgetMilliseconds": 5000 if sign else 8000,
            "elapsedMilliseconds": 5001 if sign else 8001,
            "observations": 5, "observationsClamped": False,
            "waitCompleted": completed,
            "state": ({"exists": True, "hittable": completed, "enabled": True if completed else None}
                      if sign else {"webExists": True, "candidateCount": 2,
                                    "matchingCount": 1 if completed else 0, "countsClamped": False}),
            **({} if sign else {"pane": {"target": "Preview", "exists": True,
                                         "hittable": True, "tapIssued": True,
                                         "selectionObserved": False}}),
        }

    def line(self, row):
        return self.prefix + json.dumps(row, sort_keys=True)

    def block(self, row, body=None, status="failed", test_id=None):
        suite, method = (test_id or row["testId"]).split("/")
        return (f"Test Case '-[CherriUITests.{suite} {method}]' started.\n"
                + (self.line(row) if body is None else body) + "\n"
                + f"Test Case '-[CherriUITests.{suite} {method}]' {status} (0.123 seconds).\n")

    def admitted(self, output):
        return [line for line in output.splitlines() if line.startswith(self.prefix)]

    def test_four_fixed_boundaries_preserve_exact_rows_and_original_terminal_only(self):
        for boundary in ("brg25-type-sign", "brg25-malformed-sign", "brg28-first-text", "brg28-kept-text"):
            for completed in (False, True):
                with self.subTest(boundary=boundary, completed=completed):
                    row = self.row(boundary, completed)
                    status = "passed" if completed else "failed"
                    diagnostic, records, outputs = self.collect(
                        self.block(row, status=status), 0 if completed else 65)
                    self.assertEqual(self.admitted(outputs["xcodetest.log"]), [self.line(row)])
                    self.assertEqual(diagnostic["testResults"], [
                        {"testId": row["testId"], "observedStatus": status}])
                    self.assertEqual(records, [])

    def test_short_circuit_sign_states_and_unobserved_predicate_are_distinct(self):
        states = [
            {"exists": False, "hittable": None, "enabled": None},
            {"exists": True, "hittable": False, "enabled": None},
            {"exists": True, "hittable": True, "enabled": False},
            {"exists": True, "hittable": True, "enabled": True},
            {},
        ]
        for state in states:
            with self.subTest(state=state):
                row = self.row(); row["state"] = state
                if not state:
                    row["observations"] = 0
                _, records, outputs = self.collect(self.block(row), 65)
                self.assertEqual(self.admitted(outputs["xcodetest.log"]), [self.line(row)])
                self.assertEqual(records, [])

    def test_web_missing_wrong_value_ambiguous_match_and_clamped_counts_are_distinct(self):
        cases = [(False, 0, 0, False), (True, 0, 0, False), (True, 2, 0, False),
                 (True, 2, 2, False), (True, 255, 1, True)]
        for present, candidates, matches, clamped in cases:
            with self.subTest(case=(present, candidates, matches, clamped)):
                row = self.row("brg28-first-text")
                row["state"] = dict(webExists=present, candidateCount=candidates,
                                    matchingCount=matches, countsClamped=clamped)
                _, _, outputs = self.collect(self.block(row), 65)
                self.assertEqual(self.admitted(outputs["xcodetest.log"]), [self.line(row)])

    def test_closed_schema_rejects_wrong_context_types_identity_and_impossible_state(self):
        import copy
        mutations = [
            ("schema", lambda r: r.update(schema="other")),
            ("test", lambda r: r.update(testId=self.field_test)),
            ("boundary", lambda r: r.update(boundary="private")),
            ("query", lambda r: r.update(query="button:private")),
            ("budget", lambda r: r.update(budgetMilliseconds=5001)),
            ("boolean-budget", lambda r: r.update(budgetMilliseconds=True)),
            ("elapsed-type", lambda r: r.update(elapsedMilliseconds=1.5)),
            ("elapsed-bound", lambda r: r.update(elapsedMilliseconds=120001)),
            ("observations-bool", lambda r: r.update(observations=True)),
            ("observations-bound", lambda r: r.update(observations=129)),
            ("overflow", lambda r: r.update(observationsClamped=True)),
            ("unobserved-ready", lambda r: r.update(observations=0, state={}, waitCompleted=True)),
            ("state-type", lambda r: r.update(state=[])),
            ("fake-hittable", lambda r: r["state"].update(exists=False, hittable=True)),
            ("fake-enabled", lambda r: r["state"].update(hittable=False, enabled=True)),
            ("ready-false-state", lambda r: r.update(waitCompleted=True)),
            ("extra-source", lambda r: r.update(source="PRIVATE_CANARY")),
            ("extra-ax", lambda r: r["state"].update(axTree="PRIVATE_CANARY")),
        ]
        for label, change in mutations:
            with self.subTest(label=label):
                row = copy.deepcopy(self.row()); change(row)
                _, records, outputs = self.collect(self.block(row, test_id=self.sign_test), 65)
                self.assertEqual(self.admitted(outputs["xcodetest.log"]), [])
                self.assertIn(self.rejected, outputs["xcodetest.log"])
                self.assertNotIn("PRIVATE_CANARY", "".join(outputs.values()))
                self.assertEqual(records, [])

    def test_field_bounds_and_unobserved_selection_do_not_invent_native_state(self):
        import copy
        for label, change in [
            ("boolean-count", lambda r: r["state"].update(candidateCount=True)),
            ("negative-count", lambda r: r["state"].update(matchingCount=-1)),
            ("too-many-matches", lambda r: r["state"].update(matchingCount=3)),
            ("missing-web-with-fields", lambda r: r["state"].update(webExists=False)),
            ("wrong-pane", lambda r: r["pane"].update(target="Code")),
            ("invented-selection", lambda r: r["pane"].update(selectionObserved=True)),
            ("tap-without-match", lambda r: r["pane"].update(exists=False, hittable=None)),
            ("private-pane", lambda r: r["pane"].update(label="PRIVATE_CANARY")),
        ]:
            with self.subTest(label=label):
                row = copy.deepcopy(self.row("brg28-kept-text")); change(row)
                _, records, outputs = self.collect(self.block(row), 65)
                self.assertEqual(self.admitted(outputs["xcodetest.log"]), [])
                self.assertIn(self.rejected, outputs["xcodetest.log"])
                self.assertNotIn("PRIVATE_CANARY", "".join(outputs.values()))
                self.assertEqual(records, [])

    def test_raw_padding_duplicate_json_keys_and_foreign_test_are_rejected(self):
        row = self.row()
        line = self.line(row)
        invalid = [" " * 2100 + line, line + " " * 2100,
                   line.replace('"waitCompleted": false', '"waitCompleted": false, "waitCompleted": true')]
        for raw in invalid:
            _, _, outputs = self.collect(self.block(row, body=raw), 65)
            self.assertEqual(self.admitted(outputs["xcodetest.log"]), [])
            self.assertIn(self.rejected, outputs["xcodetest.log"])
        _, records, outputs = self.collect(self.block(row, test_id=self.field_test), 65)
        self.assertEqual(self.admitted(outputs["xcodetest.log"]), [])
        self.assertIn(self.rejected, outputs["xcodetest.log"])
        self.assertEqual(records, [])

    def test_duplicate_boundary_and_rows_after_case_cannot_supply_evidence(self):
        row = self.row()
        raw = self.block(row, body=self.line(row) + "\n" + self.line(row)) + self.line(row)
        diagnostic, records, outputs = self.collect(raw, 65)
        self.assertEqual(self.admitted(outputs["xcodetest.log"]), [self.line(row)])
        self.assertEqual(outputs["xcodetest.log"].count(self.rejected), 2)
        self.assertEqual(diagnostic["testResults"], [{"testId": self.sign_test, "observedStatus": "failed"}])
        self.assertEqual(records, [])

    def test_rejection_volume_is_bounded_without_erasing_failed_terminal(self):
        row = self.row()
        diagnostic, records, outputs = self.collect(
            self.block(row, body="\n".join([self.line(row)] * 20)), 65)
        self.assertEqual(self.admitted(outputs["xcodetest.log"]), [self.line(row)])
        self.assertLessEqual(outputs["xcodetest.log"].count(self.rejected), 8)
        self.assertEqual(diagnostic["testResults"], [{"testId": self.sign_test, "observedStatus": "failed"}])
        self.assertEqual(records, [])
