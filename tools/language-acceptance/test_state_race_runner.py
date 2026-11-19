import copy
import json
import unittest

import run_state_race as runner


class StateRaceIntakeControls(unittest.TestCase):
    """Synthetic controls exercise intake logic; they never produce race receipts."""

    def fixture(self):
        events = []
        for name in runner.TESTS:
            events += [{"Action": "run", "Test": name}, {"Action": "pass", "Test": name}]
        observations = [
            {"race_instrumented": True, "sequential_rounds": 3, "concurrent_workers": 8, "concurrent_rounds": 8, "completed_cases": 64},
            {"race_instrumented": True, "sequential_rounds": 0, "concurrent_workers": 0, "concurrent_rounds": 0, "completed_cases": 0, "emitter_faults": ["emitter_error", "cancellation"]},
        ]
        for name, observation in zip(runner.TESTS, observations):
            text = runner.MARKER + json.dumps(observation) + "\n"
            # Real Go JSON can split a single t.Log message across Output events.
            events += [{"Action": "output", "Test": name, "Output": text[:20]}, {"Action": "output", "Test": name, "Output": text[20:]}]
        events.append({"Action": "pass", "Package": "github.com/electrikmilk/cherri"})
        return events

    def evaluate(self, events, **options):
        args = {"stderr": b"", "code": 0, "timed_out": False, "environment": {"GOOS": "linux", "CGO_ENABLED": "1"}, "clean_before": True, "clean_after": True}
        args.update(options)
        return runner.evaluate(b"\n".join(json.dumps(event).encode() for event in events), **args)

    def test_split_output_control(self):
        self.assertEqual(self.evaluate(self.fixture())["status"], "PASSED")

    def test_required_child_cannot_inherit_parent_success(self):
        events = [event for event in self.fixture() if event.get("Test") != runner.TESTS[3]]
        self.assertEqual(self.evaluate(events)["status"], "FAILED")

    def test_zero_matching_tests(self):
        self.assertEqual(self.evaluate(self.fixture()[-1:])["status"], "FAILED")

    def test_rejects_process_configuration_and_source_failures(self):
        for mutation in ({"code": 1}, {"timed_out": True}, {"clean_before": False}, {"clean_after": False}, {"environment": {"GOOS": "windows", "CGO_ENABLED": "0"}}, {"stderr": b"WARNING: DATA RACE"}):
            with self.subTest(mutation=mutation):
                self.assertEqual(self.evaluate(self.fixture(), **mutation)["status"], "FAILED")

    def test_rejects_skips_and_failures(self):
        for action in ("skip", "fail"):
            events = self.fixture() + [{"Action": action, "Test": runner.TESTS[3]}]
            self.assertEqual(self.evaluate(events)["status"], "FAILED")

    def test_rejects_uninstrumented_and_incomplete_witnesses(self):
        for before, after in [('"race_instrumented": true', '"race_instrumented": false'), ('"race_instrumented": true', '"race_instrumented": 1'), ('"completed_cases": 64', '"completed_cases": 64.0'), ('"completed_cases": 64', '"completed_cases": 63'), ('"cancellation"', '"not_run"')]:
            events = self.fixture()
            for event in events:
                if "Output" in event:
                    event["Output"] = event["Output"].replace(before, after)
            self.assertEqual(self.evaluate(events)["status"], "FAILED")

    def test_rejects_duplicate_completion(self):
        events = self.fixture()
        events += [copy.deepcopy(event) for event in events if event.get("Action") == "output" and event.get("Test") == runner.TESTS[0]]
        self.assertEqual(self.evaluate(events)["status"], "FAILED")


if __name__ == "__main__":
    unittest.main()
