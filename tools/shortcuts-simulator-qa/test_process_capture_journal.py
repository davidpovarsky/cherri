"""Actual owned-process controls for optional capture lifecycle journaling."""
import inspect
import io
import json
import os
from pathlib import Path
import signal
import sys
import tempfile
import time
import unittest

import project_prerequisite_observation as observation


class CaptureJournalTests(unittest.TestCase):
    def invoke(self, source, *, allowance=2, journal=None):
        output, error = io.BytesIO(), io.BytesIO()
        args = [sys.executable, "-c", source]
        kwargs = dict(output=output, error_output=error,
                      hard_deadline=time.monotonic() + 6, stream_limit=4096)
        # The fallback allows the original source to execute the real causal
        # interruption; absence of its lifecycle receipt is the RED assertion.
        if "journal" in inspect.signature(observation.capture).parameters:
            kwargs["journal"] = journal
        result, detail = observation.capture(args, allowance, **kwargs)
        return result, detail, output.getvalue(), error.getvalue()

    def test_external_interruption_retains_started_pid_and_terminal_cleanup(self):
        events = []
        def journal(event, detail):
            events.append((event, json.loads(json.dumps(detail))))
        def interrupted(signum, frame):
            raise TimeoutError("OWNED_CONTROL_INTERRUPTION")
        old = signal.signal(signal.SIGTERM, interrupted)
        try:
            with self.assertRaisesRegex(TimeoutError, "OWNED_CONTROL_INTERRUPTION"):
                self.invoke(
                    "import os,signal,time; print(os.getpid(),flush=True); "
                    "time.sleep(.08); os.kill(os.getppid(),signal.SIGTERM); time.sleep(30)",
                    journal=journal)
        finally:
            signal.signal(signal.SIGTERM, old)
        self.assertEqual([event for event, _ in events], ["started", "finished"])
        started, ended = events[0][1], events[1][1]
        self.assertIsInstance(started["pid"], int)
        self.assertGreater(started["pid"], 1)
        self.assertEqual(started["pid"], ended["pid"])
        self.assertIn("startedAt", started)
        self.assertIn("finishedAt", ended)
        self.assertTrue(ended["cleanupGroupSignalSent"])
        self.assertTrue(ended["cleanupParentReaped"])
        self.assertFalse(ended["groupPresentAfterCleanup"])
        self.assertIsNotNone(ended["returnCodeObserved"])
        with self.assertRaises(ProcessLookupError):
            os.kill(ended["pid"], 0)


    def test_cancellation_at_started_journal_is_not_a_launch_failure(self):
        events = []
        def interrupted(signum, frame):
            raise TimeoutError("OWNED_SPAWN_INTERRUPTION")
        def journal(event, detail):
            events.append((event, json.loads(json.dumps(detail))))
            if event == "started":
                os.kill(os.getpid(), signal.SIGTERM)
        old = signal.signal(signal.SIGTERM, interrupted)
        try:
            with self.assertRaisesRegex(TimeoutError, "OWNED_SPAWN_INTERRUPTION"):
                self.invoke("import time; time.sleep(30)", journal=journal)
        finally:
            signal.signal(signal.SIGTERM, old)
        self.assertEqual([event for event, _ in events], ["started", "finished"])
        self.assertIsInstance(events[0][1]["pid"], int)
        self.assertEqual(events[0][1]["pid"], events[1][1]["pid"])
        self.assertTrue(events[1][1]["cleanupParentReaped"])
        self.assertTrue(events[1][1]["cleanupGroupSignalSent"])
        with self.assertRaises(ProcessLookupError):
            os.kill(events[1][1]["pid"], 0)

    def test_normal_capture_without_journal_keeps_exact_output(self):
        result, detail, output, error = self.invoke(
            "import sys; sys.stdout.buffer.write(b'plist\\x00bytes\\n'); "
            "sys.stderr.write('native note\\n')")
        self.assertEqual(result["result"], "PASS")
        self.assertEqual(result["exitCode"], 0)
        self.assertEqual(output, b"plist\x00bytes\n")
        self.assertEqual(error, b"native note\n")
        self.assertFalse(detail["groupPresentAfterCleanup"])

    def test_journal_does_not_turn_actual_nonzero_into_pass(self):
        events = []
        result, detail, output, error = self.invoke(
            "import sys; print('original partial plist'); "
            "sys.stderr.write('original failure\\n'); sys.exit(37)",
            journal=lambda event, detail: events.append((event, dict(detail))))
        self.assertEqual(result["result"], "EXIT_NONZERO")
        self.assertEqual(result["exitCode"], 37)
        self.assertEqual([event for event, _ in events], ["started", "finished"])
        self.assertEqual(events[-1][1]["returnCodeObserved"], 37)
        self.assertEqual(error, b"original failure\n")

    def test_actual_timeout_reaps_parent_and_same_owned_group_descendant(self):
        events = []
        source = (
            "import os,subprocess,sys,time; "
            "child=subprocess.Popen([sys.executable,'-c','import time; time.sleep(30)']); "
            "print(str(os.getpid())+','+str(child.pid),flush=True); time.sleep(30)"
        )
        result, detail, output, error = self.invoke(
            source, allowance=.15,
            journal=lambda event, detail: events.append((event, json.loads(json.dumps(detail)))))
        self.assertEqual(result["result"], "TIMEOUT")
        self.assertEqual([event for event, _ in events], ["started", "finished"])
        parent, child = map(int, output.strip().split(b","))
        self.assertEqual(detail["pid"], parent)
        self.assertTrue(detail["cleanupParentReaped"])
        self.assertTrue(detail["cleanupGroupSignalSent"])
        self.assertEqual(events[-1][1]["pid"], parent)
        self.assertIsNotNone(events[-1][1]["returnCodeObserved"])
        # Preserve the actual group observation; absence is never inferred
        # from a successful signal or an already exited parent.
        self.assertEqual(events[-1][1]["groupPresentAfterCleanup"],
                         detail["groupPresentAfterCleanup"])

    def test_zero_exit_cannot_hide_descendant_holding_stdout(self):
        events = []
        result, detail, output, error = self.invoke(
            "import subprocess,sys; subprocess.Popen([sys.executable,'-c',"
            "'import time; time.sleep(30)']); print('parent exiting',flush=True)",
            allowance=.15,
            journal=lambda event, detail: events.append((event, dict(detail))))
        self.assertEqual(result["result"], "TIMEOUT")
        self.assertEqual(detail["returnCodeObserved"], 0)
        self.assertTrue(detail["cleanupGroupSignalSent"])
        self.assertEqual([event for event, _ in events], ["started", "finished"])
        self.assertEqual(events[-1][1]["groupPresentAfterCleanup"],
                         detail["groupPresentAfterCleanup"])


if __name__ == "__main__":
    unittest.main()
