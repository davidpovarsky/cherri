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

    def verify(self, events):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "events.json").write_text(json.dumps(events), encoding="utf-8")
            result = subprocess.run([sys.executable, "-c", self.program, str(root / "events.json"),
                                     "fresh-run", "qa-unit", "CHERRI_QA_EXPECTED", str(root / "trace.jsonl")],
                                    capture_output=True, text=True, timeout=10)
            self.assertEqual(result.returncode, 0, result.stderr)
            return json.loads(result.stdout)

    def test_exact_fresh_success(self):
        self.assertEqual(self.verify([self.event()])["runGate"], "PASS")

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

    def report(self, events, run_status="PASS"):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "runtime").mkdir()
            (root / "runtime/callback-events.json").write_text(json.dumps(events), encoding="utf-8")
            result = subprocess.run([sys.executable, "-c", self.assertion_program, str(root), run_status,
                                     "PASS", "PASS", "synthetic diagnostic", "qa-unit", "CHERRI_QA_EXPECTED", "fresh-run"],
                                    capture_output=True, text=True, timeout=10)
            self.assertEqual(result.returncode, 0, result.stderr)
            return json.loads((root / "runtime/output-assertions.json").read_text(encoding="utf-8"))

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


if __name__ == "__main__":
    unittest.main()
