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

    def event(self):
        records = [
            {"kind": "COLLECTOR_EVENT", "stage": "collector_begin", "runId": "qa-unit"},
            {"kind": "INLINE_DEBUG_ACTION", "stage": "before", "timestamp": "synthetic-time", "stepSequence": 1},
            {"kind": "COLLECTOR_EVENT", "stage": "collector_end", "runId": "qa-unit"},
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


if __name__ == "__main__":
    unittest.main()
