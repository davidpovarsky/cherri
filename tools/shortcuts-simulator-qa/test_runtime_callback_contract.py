"""Execute the real orchestrator's callback verifier, not a duplicate model."""
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


class CallbackContractTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        root = Path(__file__).resolve().parents[2]
        script = (root / "scripts/ios27_shortcuts_simulator_qa.sh").read_text(encoding="utf-8")
        start = script.index('      RUNTIME_EVAL="$(python3 - ')
        start = script.index("<<'PY'\n", start) + len("<<'PY'\n")
        end = script.index("\nPY\n", start)
        cls.program = script[start:end]
        start = script.index("# Write output-assertions.json and runtime-trace.jsonl")
        start = script.index("<<'PY'\n", start) + len("<<'PY'\n")
        end = script.index("\nPY\n", start)
        cls.assertion_program = script[start:end]
        start = script.index("# Write summary.json and docs/report.md")
        start = script.index("<<'PY'\n", start) + len("<<'PY'\n")
        end = script.index("\nPY\n", start)
        cls.summary_program = script[start:end]
        start = script.index('if [ "$REQUIRE_RUNTIME" = "true" ]; then')
        start = script.index("<<'PY'\n", start) + len("<<'PY'\n")
        end = script.index("\nPY\n", start)
        cls.required_program = script[start:end]

    def event(self):
        records = [
            {"schemaVersion": "1.4", "kind": "COLLECTOR_EVENT", "stage": "collector_begin", "runId": "qa-unit"},
            {"schemaVersion": "1.4", "kind": "INLINE_DEBUG_ACTION", "stage": "before", "timestamp": "synthetic-time",
             "stepSequence": 1, "actionIdentifier": "is.workflow.actions.gettext", "valueType": "UNKNOWN_AT_TRACE_EVENT"},
            {"schemaVersion": "1.4", "kind": "COLLECTOR_EVENT", "stage": "collector_end", "runId": "qa-unit"},
        ]
        return {"eventSource": "shortcuts-x-callback", "receiverSelfTest": False, "fixtureId": "qa-unit",
                "expectedShortcutName": "qa-unit", "runId": "fresh-run", "status": "SUCCESS",
                "resultSummary": "CHERRI_QA_EXPECTED", "tracePayload": "\n".join(map(json.dumps, records))}

    def manifest(self, fid="qa-unit"):
        release = fid == "qa-measure-release-v2"
        return {"syntheticOnly": True, "compilerCommitSha": "a" * 40, "fixtures": [{
            "fixtureId": fid, "buildProfile": "release" if release else "debug",
            "runtimeTraceExpectation": "ABSENT_RELEASE" if release else "REQUIRED_NDJSON",
            "releaseDiagnosticActionsAbsent": release, "releaseBodyShapeVerified": release,
            "sourceSha256": "b" * 64, "unsignedPlistSha256": "c" * 64,
        }]}

    def verify(self, events, fid="qa-unit", expected="CHERRI_QA_EXPECTED", manifest=None):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "events.json").write_text(json.dumps(events), encoding="utf-8")
            (root / "manifest.json").write_text(json.dumps(self.manifest(fid) if manifest is None else manifest), encoding="utf-8")
            result = subprocess.run([sys.executable, "-c", self.program, str(root / "events.json"),
                                     "fresh-run", fid, expected, str(root / "trace.jsonl"),
                                     str(root / "manifest.json"), "a" * 40],
                                    capture_output=True, text=True, timeout=10)
            self.assertEqual(result.returncode, 0, result.stderr)
            return json.loads(result.stdout)

    def test_exact_fresh_success(self):
        self.assertEqual(self.verify([self.event()])["runGate"], "PASS")

    def test_self_contained_short_circuit_requires_exact_count_and_normal_trace(self):
        fid = "qa-short-circuit-local-v2"
        expected = "CHERRI_QA_F01_LOCAL_RESULT:P15_F01_LOCAL_OK:calls=1"
        event = self.event()
        event.update(fixtureId=fid, expectedShortcutName=fid, resultSummary=expected)
        event["tracePayload"] = event["tracePayload"].replace("qa-unit", fid)
        result = self.verify([event], fid, expected)
        self.assertEqual((result["runGate"], result["traceGate"]), ("PASS", "PASS"))
        for summary in (expected.replace("calls=1", "calls=0"), expected.replace("calls=1", "calls=2"),
                        expected.replace("P15_F01_LOCAL_OK", "P15_F01_LOCAL_EAGER")):
            self.assertEqual(self.verify([{**event, "resultSummary": summary}], fid, expected)["runGate"], "FAIL_RUNTIME")
        self.assertEqual(self.verify([{**event, "tracePayload": ""}], fid, expected)["traceGate"], "FAIL_RUNTIME")
        manifest = self.manifest(fid)
        manifest["fixtures"][0].update(buildProfile="release", runtimeTraceExpectation="ABSENT_RELEASE")
        self.assertEqual(self.verify([event], fid, expected, manifest)["runGate"], "FAIL_RUNTIME")

    def test_f01_diagnostic_success_needs_exact_fresh_result_and_trace(self):
        cases = {
            "qa-f01-diag-counter-v2": "CHERRI_QA_F01_DIAG_COUNTER:calls=1",
            "qa-f01-diag-predicate-v2": "CHERRI_QA_F01_DIAG_PREDICATE:P15_F01_PREDICATE_OK",
            "qa-f01-diag-control-v2": "CHERRI_QA_F01_DIAG_CONTROL:P15_F01_CONTROL_OK:calls=1",
            "qa-f01-diag-logic-v2": "CHERRI_QA_F01_DIAG_LOGIC:P15_F01_LOGIC_OK"
        }
        for fid, expected in cases.items():
            with self.subTest(fixture=fid):
                event = self.event()
                event.update(fixtureId=fid, expectedShortcutName=fid, resultSummary=expected)
                event["tracePayload"] = event["tracePayload"].replace("qa-unit", fid)
                observed = self.verify([event], fid, expected)
                self.assertEqual((observed["runGate"], observed["traceGate"]), ("PASS", "PASS"))
                for mutation in (
                    {**event, "status": "ERROR", "errorCode": "1"},
                    {**event, "resultSummary": expected + "_WRONG"},
                    {**event, "resultSummary": expected.replace("calls=1", "calls=2")} if "calls=1" in expected else {**event, "resultSummary": expected.replace("_OK", "_WRONG")},
                    {**event, "runId": "stale-run"},
                    {**event, "fixtureId": "qa-short-circuit-local-v2"},
                ):
                    self.assertEqual(self.verify([mutation], fid, expected)["runGate"], "FAIL_RUNTIME")
                self.assertEqual(self.verify([{**event, "tracePayload": ""}], fid, expected)["traceGate"], "FAIL_RUNTIME")

    def test_label_export_probes_require_exact_fresh_summary_and_normal_trace(self):
        cases = {
            "qa-label-file-carrier-v2": "CHERRI_QA_LABEL_PROBE_RESULT:P15_LABEL_FILE_CARRIER_OK",
            "qa-label-raw-empty-v2": "CHERRI_QA_LABEL_PROBE_RESULT:P15_LABEL_RAW_EMPTY_OK",
            "qa-label-raw-grouping-v2": "CHERRI_QA_LABEL_PROBE_RESULT:P15_LABEL_RAW_GROUPING_OK",
            "qa-export-unicode-v2": "CHERRI_QA_EXPORT_PROBE_RESULT:P15_EXPORT_UNICODE_OK",
            "qa-export-empty-v2": "CHERRI_QA_EXPORT_PROBE_RESULT:P15_EXPORT_EMPTY_OK",
            "qa-export-thousand-v2": "CHERRI_QA_EXPORT_PROBE_RESULT:P15_EXPORT_THOUSAND_OK",
            "qa-export-redacted-v2": "CHERRI_QA_EXPORT_PROBE_RESULT:P15_EXPORT_REDACTED_OK",
            "qa-export-unsupported-file-v2": "CHERRI_QA_EXPORT_PROBE_RESULT:P15_EXPORT_UNSUPPORTED_FILE_OK",
            "qa-export-text-file-v2": "CHERRI_QA_EXPORT_PROBE_RESULT:P15_EXPORT_TEXT_FILE_OK",
            "qa-paginate-ordered-v2": "CHERRI_QA_PAGINATE_PROBE_RESULT:P15_PAGINATE_ORDERED_OK",
            "qa-paginate-cycle-v2": "CHERRI_QA_PAGINATE_PROBE_RESULT:P15_PAGINATE_CYCLE_OK",
            "qa-paginate-page-cap-v2": "CHERRI_QA_PAGINATE_PROBE_RESULT:P15_PAGINATE_PAGE_CAP_OK",
            "qa-paginate-item-cap-v2": "CHERRI_QA_PAGINATE_PROBE_RESULT:P15_PAGINATE_ITEM_CAP_OK",
            "qa-paginate-empty-v2": "CHERRI_QA_PAGINATE_PROBE_RESULT:P15_PAGINATE_EMPTY_OK",
        }
        for fid, expected in cases.items():
            with self.subTest(fixture=fid):
                event = self.event()
                event.update(fixtureId=fid, expectedShortcutName=fid, resultSummary=expected)
                event["tracePayload"] = event["tracePayload"].replace("qa-unit", fid)
                result = self.verify([event], fid, expected)
                self.assertEqual((result["runGate"], result["traceGate"]), ("PASS", "PASS"))
                for key, value in (("resultSummary", expected + "_WRONG"), ("status", "ERROR"),
                                   ("status", "CANCELLED"), ("runId", "stale"),
                                   ("eventSource", "receiver-self-test"), ("fixtureId", "qa-other"),
                                   ("expectedShortcutName", "qa-other"), ("receiverSelfTest", True)):
                    result = self.verify([{**event, key: value}], fid, expected)
                    self.assertEqual(result["runGate"], "FAIL_RUNTIME", (fid, key))
                for payload in ("", event["tracePayload"].replace(fid, "stale-fixture")):
                    self.assertEqual(self.verify([{**event, "tracePayload": payload}], fid, expected)["traceGate"], "FAIL_RUNTIME")
                self.assertEqual(self.verify([event, event], fid, expected)["runGate"], "FAIL_RUNTIME")
                manifest = self.manifest(fid)
                manifest["fixtures"][0].update(buildProfile="release", runtimeTraceExpectation="ABSENT_RELEASE",
                                               releaseDiagnosticActionsAbsent=True, releaseBodyShapeVerified=True)
                self.assertEqual(self.verify([event], fid, expected, manifest)["runGate"], "FAIL_RUNTIME")

    def test_retry_probes_keep_exact_fresh_result_and_normal_trace_contract(self):
        import retry_http_probe as probe
        for fid in probe.FIXTURES:
            expected = probe.expected_marker(fid)
            event = self.event()
            event.update(fixtureId=fid, expectedShortcutName=fid, resultSummary=expected)
            event["tracePayload"] = event["tracePayload"].replace("qa-unit", fid)
            with self.subTest(fixture=fid):
                result = self.verify([event], fid, expected)
                self.assertEqual((result["runGate"], result["traceGate"]), ("PASS", "PASS"))
                for key, value in (("status", "ERROR"), ("status", "CANCELLED"),
                                   ("runId", "stale"), ("resultSummary", expected + "_WRONG"),
                                   ("tracePayload", "")):
                    result = self.verify([{**event, key: value}], fid, expected)
                    self.assertEqual(result["traceGate" if key == "tracePayload" else "runGate"], "FAIL_RUNTIME", key)
                manifest = self.manifest(fid)
                manifest["fixtures"][0].update(buildProfile="release", runtimeTraceExpectation="ABSENT_RELEASE",
                                               releaseDiagnosticActionsAbsent=True, releaseBodyShapeVerified=True)
                self.assertEqual(self.verify([event], fid, expected, manifest)["runGate"], "FAIL_RUNTIME")

    def test_source_fixture_and_self_test_mutations(self):
        for key, value in (("eventSource", "receiver-self-test"), ("fixtureId", "other"),
                           ("expectedShortcutName", "other"), ("receiverSelfTest", True),
                           ("receiverSelfTest", None), ("runId", "stale")):
            event = self.event()
            event[key] = value
            self.assertEqual(self.verify([event])["runGate"], "FAIL_RUNTIME", key)

    def test_duplicate_replay_rejected(self):
        event = self.event()
        self.assertEqual(self.verify([event, event])["runGate"], "FAIL_RUNTIME")

    def test_output_substring_or_trace_only_cannot_pass(self):
        for summary in ("unexpected CHERRI_QA_EXPECTED suffix", "different"):
            event = self.event()
            event["resultSummary"] = summary
            event["tracePayload"] += "\nCHERRI_QA_EXPECTED"
            self.assertEqual(self.verify([event])["runGate"], "FAIL_RUNTIME")

    def test_error_cancel_and_missing_callback_not_success(self):
        for status in ("ERROR", "CANCELLED", "DENIED", "TIMEOUT"):
            event = self.event()
            event["status"] = status
            self.assertEqual(self.verify([event])["runGate"], "FAIL_RUNTIME", status)
        self.assertEqual(self.verify([])["runGate"], "FAIL_RUNTIME")

    def test_wrong_trace_identity_and_no_events_not_trace_success(self):
        for payload in ("", self.event()["tracePayload"].replace("qa-unit", "stale-fixture")):
            event = self.event()
            event["tracePayload"] = payload
            self.assertEqual(self.verify([event])["traceGate"], "FAIL_RUNTIME")

    def test_callback_store_and_field_types_fail_closed(self):
        for store in ({}, None, [None], ["event"]):
            self.assertEqual(self.verify(store)["runGate"], "FAIL_RUNTIME")
        for key, value in (("resultSummary", {}), ("resultSummary", 7), ("tracePayload", ["line"])):
            event = self.event()
            event[key] = value
            self.assertEqual(self.verify([event])["runGate"], "FAIL_RUNTIME", key)

    def test_trace_schema_order_and_completeness(self):
        records = [json.loads(line) for line in self.event()["tracePayload"].splitlines()]
        begin, inline, end = records
        invalid_streams = [records[::-1], [inline, begin, end], [begin, end],
                           [begin, begin, inline, end], [begin, inline, end, inline],
                           [begin, inline, inline, end], [begin, inline, end, end],
                           [begin, inline, {**inline, "stepSequence": 0}, end],
                           [{**begin, "runId": ""}, inline, end],
                           [begin, {**inline, "kind": "UNKNOWN_KIND"}, end],
                           [begin, {**inline, "schemaVersion": "unverified"}, end],
                           [begin, 3, end]]
        for stream in invalid_streams:
            event = self.event()
            event["tracePayload"] = "\n".join(map(json.dumps, stream))
            self.assertEqual(self.verify([event])["traceGate"], "FAIL_RUNTIME", stream)
        event = self.event()
        event["tracePayload"] += "\nmalformed trailing text"
        self.assertEqual(self.verify([event])["traceGate"], "FAIL_RUNTIME")

    def test_inline_field_types_are_validated_without_inventing_value_types(self):
        for key, value in (("stepSequence", True), ("stepSequence", "1"), ("stepSequence", -1),
                           ("timestamp", 123), ("timestamp", ""), ("actionIdentifier", None),
                           ("valueType", None)):
            event = self.event()
            records = [json.loads(line) for line in event["tracePayload"].splitlines()]
            records[1][key] = value
            event["tracePayload"] = "\n".join(map(json.dumps, records))
            self.assertEqual(self.verify([event])["traceGate"], "FAIL_RUNTIME", (key, value))
        self.assertEqual(self.verify([self.event()])["traceGate"], "PASS")

    def report(self, events, run_status="PASS", fid="qa-unit", expected="CHERRI_QA_EXPECTED",
               trace_status="PASS", oracle=None):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "runtime").mkdir()
            (root / "runtime/callback-events.json").write_text(json.dumps(events), encoding="utf-8")
            result = subprocess.run([sys.executable, "-c", self.assertion_program, str(root), run_status,
                                     "PASS", trace_status, "synthetic diagnostic", fid, expected, "fresh-run",
                                     json.dumps(self.verify(events, fid, expected) if oracle is None else oracle)],
                                    capture_output=True, text=True, timeout=10)
            self.assertEqual(result.returncode, 0, result.stderr)
            report = json.loads((root / "runtime/output-assertions.json").read_text(encoding="utf-8"))
            report["_traceFileText"] = (root / "runtime/runtime-trace.jsonl").read_text(encoding="utf-8")
            return report

    def test_report_requires_same_callback_identity_and_oracle_acceptance(self):
        report = self.report([self.event()])
        self.assertTrue(report["compiledShortcutCallbackVerified"])
        self.assertEqual(report["typedValueCoverage"], "UNKNOWN_AT_TRACE_EVENT")
        self.assertFalse(report["perActionCoverageVerified"])
        self.assertFalse(self.report([self.event()], "FAIL_RUNTIME")["compiledShortcutCallbackVerified"])
        for key, value in (("runId", "stale"), ("fixtureId", "other"), ("eventSource", "receiver-self-test"),
                           ("expectedShortcutName", "other"), ("receiverSelfTest", None),
                           ("resultSummary", "wrong"), ("status", "ERROR"), ("status", "CANCELLED")):
            event = self.event()
            event[key] = value
            report = self.report([event])
            self.assertFalse(report["compiledShortcutCallbackVerified"], (key, value))
            self.assertFalse(report["compiledShortcutRuntimeExecuted"], (key, value))
        self.assertFalse(self.report([self.event(), self.event()])["compiledShortcutCallbackVerified"])

    def test_report_preserves_observed_error_cancel_without_promoting_success(self):
        for status in ("ERROR", "CANCELLED"):
            event = self.event()
            event["status"] = status
            report = self.report([event], "FAIL_RUNTIME")
            self.assertEqual(report["observedCorrelatedCallbackStatus"], status)
            self.assertFalse(report["compiledShortcutCallbackVerified"])

    release_id = "qa-measure-release-v2"
    release_expected = "CHERRI_QA_MEASURE_RELEASE_RESULT:P15_MEASURE_RELEASE_OK"

    def release_event(self):
        event = self.event()
        event.update(fixtureId=self.release_id, expectedShortcutName=self.release_id,
                     resultSummary=self.release_expected, tracePayload=None)
        return event

    def verify_release(self, events, manifest=None):
        return self.verify(events, self.release_id, self.release_expected, manifest)

    def test_release_absence_is_explicit_and_not_a_trace_pass(self):
        for payload in (None, ""):
            event = self.release_event()
            event["tracePayload"] = payload
            result = self.verify_release([event])
            self.assertEqual(result["runGate"], "PASS")
            self.assertEqual(result["traceGate"], "NOT_APPLICABLE_RELEASE")
            self.assertIs(result["releaseBuildVerified"], True)
            report = self.report([event], fid=self.release_id, expected=self.release_expected,
                                 trace_status="NOT_APPLICABLE_RELEASE")
            self.assertIs(report["runtimeContractVerified"], True)
            self.assertEqual(report["traceObserved"], "ABSENT")
            self.assertEqual(report["_traceFileText"], "")
        event = self.event()
        event["tracePayload"] = None
        self.assertEqual(self.verify([event])["traceGate"], "FAIL_RUNTIME")

    def test_release_unexpected_trace_and_nontext_values_fail(self):
        for payload in (" ", "garbage", self.event()["tracePayload"], False, 0, [], {}):
            event = self.release_event()
            event["tracePayload"] = payload
            self.assertEqual(self.verify_release([event])["runGate"], "FAIL_RUNTIME", payload)

    def test_release_exact_callback_identity_output_and_no_replay_still_required(self):
        for key, value in (("eventSource", "receiver-self-test"), ("receiverSelfTest", True),
                           ("runId", "stale"), ("fixtureId", "qa-measure-v2"),
                           ("expectedShortcutName", "qa-measure-v2"), ("status", "ERROR"),
                           ("status", "CANCELLED"), ("resultSummary", "P15_MEASURE_RELEASE_OK")):
            event = self.release_event()
            event[key] = value
            self.assertEqual(self.verify_release([event])["runGate"], "FAIL_RUNTIME", key)
        self.assertEqual(self.verify_release([self.release_event(), self.release_event()])["runGate"], "FAIL_RUNTIME")

    def test_release_requires_exact_compiler_profile_manifest_proof(self):
        for key, value in (("buildProfile", "debug"), ("runtimeTraceExpectation", "REQUIRED_NDJSON"),
                           ("releaseDiagnosticActionsAbsent", False), ("releaseBodyShapeVerified", False),
                           ("sourceSha256", ""), ("unsignedPlistSha256", "wrong"),
                           ("fixtureId", "qa-measure-v2")):
            manifest = self.manifest(self.release_id)
            manifest["fixtures"][0][key] = value
            self.assertEqual(self.verify_release([self.release_event()], manifest)["runGate"], "FAIL_RUNTIME", key)
        for key, value in (("compilerCommitSha", "stale"), ("syntheticOnly", False), ("fixtures", [])):
            manifest = self.manifest(self.release_id)
            manifest[key] = value
            self.assertEqual(self.verify_release([self.release_event()], manifest)["runGate"], "FAIL_RUNTIME", key)
        manifest = self.manifest(self.release_id)
        manifest["fixtures"].append(manifest["fixtures"][0].copy())
        self.assertEqual(self.verify_release([self.release_event()], manifest)["runGate"], "FAIL_RUNTIME")
        manifest = self.manifest()
        manifest["fixtures"][0].update(buildProfile="release", runtimeTraceExpectation="ABSENT_RELEASE",
                                        releaseDiagnosticActionsAbsent=True, releaseBodyShapeVerified=True)
        event = self.event()
        event["tracePayload"] = None
        self.assertEqual(self.verify([event], manifest=manifest)["runGate"], "FAIL_RUNTIME")

    def test_release_report_cannot_promote_missing_or_mismatched_oracle(self):
        oracle = self.verify_release([self.release_event()])
        for key, value in (("releaseBuildVerified", False), ("buildProfile", "debug"),
                           ("traceGate", "PASS"), ("targetFixtureId", "qa-unit"), ("runGate", "FAIL_RUNTIME")):
            wrong = {**oracle, key: value}
            report = self.report([self.release_event()], fid=self.release_id, expected=self.release_expected,
                                 trace_status="NOT_APPLICABLE_RELEASE", oracle=wrong)
            self.assertFalse(report["runtimeContractVerified"], key)
            self.assertFalse(report["compiledShortcutCallbackVerified"], key)

    def summarize(self, report, trace_status="NOT_APPLICABLE_RELEASE", run_status="PASS"):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "runtime").mkdir()
            (root / "docs").mkdir()
            (root / "runtime/output-assertions.json").write_text(json.dumps(report), encoding="utf-8")
            gates = ["PASS"] * 14
            gates[8] = "EDITOR_CAPTURE_COMPLETE"
            gates[10] = run_status
            gates[12] = trace_status
            gates[13] = "BLOCKED_DEVICE"
            result = subprocess.run([sys.executable, "-c", self.summary_program, str(root), "synthetic-run", "a" * 40,
                                     "true", *gates, ""], capture_output=True, text=True, timeout=10)
            self.assertEqual(result.returncode, 0, result.stderr)
            summary_path = root / "summary.json"
            summary = json.loads(summary_path.read_text(encoding="utf-8"))
            required = subprocess.run([sys.executable, "-c", self.required_program, str(summary_path)],
                                      capture_output=True, text=True, timeout=10)
            return summary, required.returncode

    def test_summary_and_required_gate_only_accept_verified_release_absence(self):
        report = self.report([self.release_event()], fid=self.release_id, expected=self.release_expected,
                             trace_status="NOT_APPLICABLE_RELEASE")
        summary, code = self.summarize(report)
        self.assertEqual(code, 0)
        self.assertEqual(summary["gates"]["TRACE_CAPTURED"], "NOT_APPLICABLE_RELEASE")
        self.assertEqual(summary["releaseReadiness"], "RUNTIME_VERIFIED_EDITOR_NEEDS_REVIEW")
        for key, value in (("runtimeContractVerified", False), ("targetFixtureId", "qa-unit"),
                           ("compilerCommitSha", "stale"), ("buildProfile", "debug"),
                           ("traceObserved", "NDJSON_VERIFIED"), ("releaseBuildVerified", False)):
            summary, code = self.summarize({**report, key: value})
            self.assertEqual(code, 1, key)
            self.assertEqual(summary["gates"]["TRACE_CAPTURED"], "FAIL_RUNTIME", key)
        self.assertEqual(self.summarize(report, run_status="FAIL_RUNTIME")[1], 1)
        self.assertEqual(self.summarize(report, trace_status="PASS")[1], 1)
        normal_report = self.report([self.event()])
        self.assertEqual(self.summarize(normal_report, trace_status="PASS")[1], 0)
        self.assertEqual(self.summarize({**normal_report, "compilerCommitSha": "stale"}, trace_status="PASS")[1], 1)
        self.assertEqual(self.summarize(normal_report)[1], 1)


if __name__ == "__main__":
    unittest.main()
