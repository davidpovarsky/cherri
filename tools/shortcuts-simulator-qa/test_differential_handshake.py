"""Owned host-process controls; these do not execute Apple software."""
import contextlib
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import time
import unittest
from unittest import mock
import urllib.parse

spec = importlib.util.spec_from_file_location("differential_handshake_subject", Path(__file__).with_name("run_differential_batch.py"))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)

WAIT_FOR_PHASE_CHILD = r'''import json, os, pathlib, sys, time
root, operation, behavior = pathlib.Path(sys.argv[1]), sys.argv[2], sys.argv[3]
callback = json.loads(sys.argv[4])
(root / "owned-child-started.json").write_text(json.dumps({"pid": os.getpid(), "operation": operation}))
end = time.monotonic() + 10
while time.monotonic() < end:
    try:
        command = json.loads((root / "command.json").read_text())
    except (FileNotFoundError, json.JSONDecodeError):
        command = {}
    if command.get("operation") == operation:
        assert command.get("shortcutName") == "OWNED_HANDSHAKE_FIXTURE"
        receipt = {"id": command["id"], "operation": operation, "shortcutName": command["shortcutName"], "observedMonotonic": time.monotonic()}
        (root / "owned-child-observed-phase.json").write_text(json.dumps(receipt))
        (root / ("phase-started-" + command["id"] + ".json")).write_text(json.dumps(dict(receipt, outcome="STARTED")))
        outcome = "IMPORTED" if operation == "import" else "CALLBACK_OBSERVED_BY_HOST"
        (root / ("ack-" + command["id"] + ".json")).write_text(json.dumps(dict(receipt, outcome=outcome)))
        if operation == "runtime":
            destination = root.parent / "receiver/Documents/callback-events.json"
            destination.parent.mkdir(parents=True, exist_ok=True)
            destination.write_text(json.dumps([callback]))
        print("OWNED_CHILD_SAW_MATCHING_PHASE", flush=True)
        if behavior == "timeout":
            time.sleep(10)
        raise SystemExit(7 if behavior == "nonzero" else 0)
    time.sleep(0.01)
raise SystemExit("matching UI phase was never published")
'''

class DifferentialHandshakeTests(unittest.TestCase):
    def run_case(self, operation, behavior):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            output = root / "out"
            signed = root / "owned-synthetic.shortcut"
            signed.write_bytes(b"AEA1owned-host-control-only")
            case = {"id": "owned", "lane": "host_control", "name": "OWNED_HANDSHAKE_FIXTURE",
                    "expected": "OWNED_CALLBACK", "signedPath": str(signed),
                    "signedSHA256": hashlib.sha256(signed.read_bytes()).hexdigest()}
            manifest = root / "manifest.json"
            manifest.write_text(json.dumps({"cases": [case]}))
            callbacks = root / "receiver/Documents/callback-events.json"
            actual_command = module.command
            actual_popen = subprocess.Popen
            child_processes = []
            requests = []

            class XCTestProcess:
                pid = 123456
                returncode = None
                def poll(self):
                    return self.returncode
                def wait(self, timeout):
                    self.returncode = 0
                    return 0

            def launch(argv, *args, **kwargs):
                if argv[0] == "xcodebuild":
                    return XCTestProcess()
                self.assertEqual(argv[0], sys.executable, "Only the owned Python producer may execute")
                process = actual_popen(argv, *args, **kwargs)
                child_processes.append(process)
                return process

            def callback_for(trigger):
                query = urllib.parse.parse_qs(urllib.parse.urlsplit(trigger).query)
                target = urllib.parse.parse_qs(urllib.parse.urlsplit(query["target"][0]).query)
                success = urllib.parse.parse_qs(urllib.parse.urlsplit(target["x-success"][0]).query)
                return {"runId": success["runId"][0], "fixtureId": case["id"],
                        "expectedShortcutName": case["name"], "receiverSelfTest": False,
                        "eventSource": "shortcuts-x-callback", "status": "SUCCESS",
                        "resultSummary": case["expected"],
                        "resultShape": {"utf8ByteCount": len(case["expected"].encode()),
                                        "utf8SHA256": hashlib.sha256(case["expected"].encode()).hexdigest()}}

            def invoke(argv, timeout=25, **kwargs):
                requests.append((argv, timeout))
                if argv[2] == "get_app_container":
                    return str(root / ("installed.app" if argv[-1] == "app" else "receiver"))
                if argv[2] != "openurl":
                    return ""
                self.assertEqual(timeout, 60, "The production openurl deadline changed")
                phase = "runtime" if argv[-1].startswith("cherriqa://trigger?") else "import"
                callback = callback_for(argv[-1]) if phase == "runtime" else {}
                if phase == operation:
                    # Exercise the real command implementation with an actual child.
                    # Only this owned producer's supervision is shortened; run()
                    # must still request the unchanged production60s deadline.
                    return actual_command([sys.executable, "-c", WAIT_FOR_PHASE_CHILD,
                                           str(output), phase, behavior, json.dumps(callback)],
                                          timeout=0.8, **kwargs)
                if kwargs.get("on_started") is not None:
                    kwargs["on_started"]()
                if phase == "runtime":
                    callbacks.parent.mkdir(parents=True, exist_ok=True)
                    callbacks.write_text(json.dumps([callback]))
                return ""

            def wait(path, seconds, process):
                if path.name == "ready.json":
                    return {"ready": True}
                other_phase = "runtime" if operation == "import" else "import"
                if path.name.startswith("ack-" + other_phase + "-"):
                    return {"outcome": "IMPORTED" if other_phase == "import" else "CALLBACK_OBSERVED_BY_HOST"}
                end = time.monotonic() + 1
                while time.monotonic() < end:
                    value = module.load_json(path)
                    if value is not None:
                        return value
                    time.sleep(0.01)
                raise TimeoutError(path.name)

            with mock.patch.object(module.subprocess, "Popen", side_effect=launch), \
                 mock.patch.object(module, "command", side_effect=invoke), \
                 mock.patch.object(module, "wait_file", side_effect=wait), \
                 mock.patch.object(module, "receiver_image_identity", return_value={"outcome": "MATCH", "ownedHostControl": True}), \
                 contextlib.redirect_stdout(io.StringIO()):
                code = module.run(manifest, output, "owned-no-simulator", root / "harness", root / "derived")
            self.assertEqual(len(child_processes), 1)
            self.assertIsNotNone(child_processes[0].poll(), "The owned URL producer was not reaped")
            return {"code": code, "summary": module.load_json(output / "summary.json"),
                    "rows": module.load_json(output / "results.json", []),
                    "observed": module.load_json(output / "owned-child-observed-phase.json"),
                    "acks": [module.load_json(p) for p in output.glob("ack-" + operation + "-*.json")],
                    "dispatch": [module.load_json(p) for p in output.glob("dispatch-" + operation + "-*.json")],
                    "callbacks": module.load_json(callbacks, []),
                    "childReturncode": child_processes[0].returncode}

    def test_actual_child_can_wait_for_each_phase_before_exit(self):
        for operation in ("import", "runtime"):
            with self.subTest(operation=operation):
                result = self.run_case(operation, "success")
                self.assertEqual(result["code"], 0, result)
                self.assertEqual(result["summary"]["counts"], {"PASS": 1})
                self.assertEqual(result["observed"]["operation"], operation)
                self.assertEqual(result["childReturncode"], 0)
                self.assertEqual(len(result["dispatch"]), 1)
                receipt = result["dispatch"][0]
                self.assertEqual(receipt["outcome"], "EXITED")
                self.assertEqual(receipt["exitCode"], 0)
                self.assertFalse(receipt["timedOut"])
                self.assertLessEqual(receipt["startedUTC"], receipt["phasePublishedUTC"])
                self.assertLessEqual(receipt["phasePublishedUTC"], receipt["completedUTC"])

    def test_nonzero_dispatch_cannot_pass_even_after_ui_ack(self):
        for operation in ("import", "runtime"):
            with self.subTest(operation=operation):
                result = self.run_case(operation, "nonzero")
                self.assertEqual(result["code"], 1)
                self.assertEqual(result["summary"]["casesObserved"], 0)
                self.assertTrue(result["acks"], result)
                self.assertEqual(result["childReturncode"], 7)
                self.assertEqual(result["dispatch"][0]["outcome"], "EXIT_NONZERO")
                self.assertEqual(result["dispatch"][0]["exitCode"], 7)
                if operation == "runtime":
                    self.assertEqual(result["callbacks"][0]["resultSummary"], "OWNED_CALLBACK")

    def test_timeout_dispatch_cannot_pass_even_after_ui_ack(self):
        for operation in ("import", "runtime"):
            with self.subTest(operation=operation):
                result = self.run_case(operation, "timeout")
                self.assertEqual(result["code"], 1)
                self.assertEqual(result["summary"]["casesObserved"], 0)
                self.assertTrue(result["acks"], result)
                self.assertNotEqual(result["childReturncode"], 0)
                self.assertEqual(result["dispatch"][0]["outcome"], "TIMED_OUT")
                self.assertTrue(result["dispatch"][0]["timedOut"])
                if operation == "runtime":
                    self.assertEqual(result["callbacks"][0]["resultSummary"], "OWNED_CALLBACK")

if __name__ == "__main__":
    unittest.main()
