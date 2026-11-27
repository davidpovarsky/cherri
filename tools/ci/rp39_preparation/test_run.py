"""One owned-child observation/cleanup control; no Git, build or native test."""
import importlib.util
import json
import os
from pathlib import Path
import sys
import signal
import tempfile
import time
import unittest


HERE = Path(__file__).resolve().parent
REPO = HERE.parents[2]
spec = importlib.util.spec_from_file_location("preparation_controller", HERE / "run.py")
controller = importlib.util.module_from_spec(spec)
spec.loader.exec_module(controller)


class PreparationObservationTests(unittest.TestCase):
    def test_real_child_timeout_keeps_samples_and_owned_cleanup(self):
        source = REPO / "ios/scripts/collect_ios_xcresult_summary.py"
        base = controller.module_from_bytes("_control_base", source, source.read_bytes())
        helper = base.load_primitives(REPO, None)
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            deadline = time.monotonic() + 2
            journals = []
            with (root / "stdout").open("xb") as stdout, (root / "stderr").open("xb") as stderr:
                observed, detail = controller.observed_capture(helper.capture,
                    base.OwnedGitReads.process_image, root / "samples.json",
                    ["/bin/sleep", "2"], 0.5, output=stdout, error_output=stderr,
                    execution_deadline=deadline - 1.5, hard_deadline=deadline,
                    stream_limit=65536, journal=lambda phase, value: journals.append((phase, value["pid"])))
            samples = json.loads((root / "samples.json").read_bytes())
            self.assertEqual(observed["result"], "TIMEOUT")
            self.assertTrue(detail["cleanupParentReaped"])
            self.assertEqual(detail["groupAfterCleanup"]["state"], "ABSENT")
            self.assertEqual(journals, [("started", detail["pid"]), ("finished", detail["pid"])])
            self.assertTrue(samples["samplerJoined"])
            self.assertEqual(samples["samples"][0]["phase"], "started")
            self.assertEqual(samples["samples"][-1]["phase"], "finished")
            self.assertTrue(all(row["pid"] == detail["pid"] for row in samples["samples"]))
            self.assertEqual([row["elapsedMs"] for row in samples["samples"]],
                             sorted(row["elapsedMs"] for row in samples["samples"]))
            self.assertLessEqual(len(samples["samples"]), controller.MAX_SAMPLES)
            self.assertFalse(any(thread.name == "owned-process-image" and thread.is_alive()
                                 for thread in controller.threading.enumerate()))
            self.assertTrue(samples["initialImageIsNotResidencyProof"])

    def test_first_signal_during_cleanup_cannot_interrupt_reap(self):
        source = REPO / "ios/scripts/collect_ios_xcresult_summary.py"
        base = controller.module_from_bytes("_signal_control_base", source, source.read_bytes())
        helper = base.load_primitives(REPO, None)
        root = Path(os.environ["CHERRI_PREPARATION_CONTROL_ORIGINALS"]).resolve()
        root.mkdir(mode=0o700)
        original_kill = helper.kill_group
        owned = []
        journals = []
        first = True
        def signal_at_cleanup(process):
            nonlocal first
            owned.append(process)
            if first:
                first = False
                os.kill(os.getpid(), signal.SIGTERM)
            return original_kill(process)
        def journal(phase, value):
            journals.append(phase)
            (root / (phase + ".original.json")).write_text(json.dumps(value, sort_keys=True) + "\n")
        helper.kill_group = signal_at_cleanup
        interrupted = False
        before_rescue = None
        try:
            with base.envelope(3):
                deadline = time.monotonic() + 1.5
                try:
                    with (root / "stdout.original").open("xb") as stdout, \
                            (root / "stderr.original").open("xb") as stderr:
                        controller.observed_capture(helper.capture, base.OwnedGitReads.process_image,
                            root / "samples.original.json", ["/bin/sleep", "2"], 0.2,
                            output=stdout, error_output=stderr, execution_deadline=deadline - 1.3,
                            hard_deadline=deadline, stream_limit=65536, journal=journal)
                except base.EnvelopeExpired:
                    interrupted = True
            self.assertTrue(owned, "the injected signal must follow actual Popen and enter cleanup")
            process = owned[0]
            before_rescue = {"pid": process.pid, "returnCode": process.returncode,
                             "group": helper.group_observation(process.pid), "journals": journals,
                             "interrupted": interrupted, "injectedSignal": "SIGTERM_AT_KILL_GROUP"}
            (root / "before-rescue.original.json").write_text(json.dumps(before_rescue, sort_keys=True) + "\n")
            self.assertTrue(interrupted)
            self.assertEqual(journals, ["started", "finished"], "first cleanup signal must not erase finished journal")
            self.assertIsNotNone(process.returncode, "capture must reap before cancellation is delivered")
            self.assertEqual(before_rescue["group"]["state"], "ABSENT")
            self.assertTrue(json.loads((root / "samples.original.json").read_bytes())["samplerJoined"])
        finally:
            helper.kill_group = original_kill
            # The old-source causal failure is deliberately rescued by its test
            # owner, so a failing comparison never leaves the owned sleep live.
            rescue = []
            for process in owned:
                original_kill(process)
                process.wait(timeout=2)
                for pipe in (process.stdout, process.stderr):
                    if pipe is not None and not pipe.closed:
                        pipe.close()
                rescue.append({"pid": process.pid, "parentReaped": process.returncode is not None,
                               "group": helper.group_observation(process.pid)})
            (root / "rescue.original.json").write_text(json.dumps(rescue, sort_keys=True) + "\n")


if __name__ == "__main__":
    unittest.main()
