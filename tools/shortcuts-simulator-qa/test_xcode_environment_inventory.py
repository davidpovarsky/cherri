#!/usr/bin/env python3
"""Offline controls: no Apple command, provider call, compiler, or native test."""

import importlib.util
import io
import json
from pathlib import Path
import plistlib
import subprocess
import sys
import tempfile
import time
import unittest
from unittest import mock
import zipfile

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("inventory", HERE / "xcode_environment_inventory.py")
inventory = importlib.util.module_from_spec(spec)
spec.loader.exec_module(inventory)


class InventoryControls(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="cherri-inventory-control-")
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.raw = self.root / "raw"
        self.raw.mkdir()

    def test_fixed_plan_contains_only_read_only_commands(self):
        plan = inventory.command_plan()
        self.assertEqual(len(plan), 8)
        self.assertEqual(len({row[0] for row in plan}), 8)
        self.assertTrue(all(0 < row[2] <= 25 for row in plan))
        self.assertIn(("/usr/bin/xcode-select", "-p"), [row[1] for row in plan])
        self.assertEqual([row[1] for row in plan if row[0].startswith("simulator-")], [
            ("/usr/bin/xcrun", "simctl", "list", "devices", "--json"),
            ("/usr/bin/xcrun", "simctl", "list", "runtimes", "--json")])
        forbidden = {"boot", "bootstatus", "open", "install", "delete", "erase", "shutdown", "build", "test", "-switch", "--switch"}
        self.assertFalse(any(forbidden.intersection(argv) for _, argv, _ in plan))

    def test_process_originals_and_nonzero_status_preserved(self):
        row = inventory.run_command("synthetic", [sys.executable, "-c", "import sys;print('CONTROL_STDOUT');print('CONTROL_STDERR',file=sys.stderr);sys.exit(7)"], 5, self.raw, time.monotonic() + 10)
        self.assertEqual((row["status"], row["exit_code"]), ("EXIT_NONZERO", 7))
        self.assertTrue(row["streams_complete"])
        self.assertIn(b"CONTROL_STDOUT", (self.raw / "synthetic.stdout.original").read_bytes())
        self.assertIn(b"CONTROL_STDERR", (self.raw / "synthetic.stderr.original").read_bytes())
        self.assertEqual(json.loads((self.raw / "synthetic.process.json").read_bytes()), row)

    def test_actual_timeout_is_incomplete_and_not_a_success(self):
        row = inventory.run_command("timeout", [sys.executable, "-c", "import time;time.sleep(3)"], 0.1, self.raw, time.monotonic() + 5)
        self.assertEqual(row["status"], "TIMEOUT")
        self.assertTrue(row["timed_out"])
        self.assertFalse(row["streams_complete"])
        self.assertLess(row["elapsed_seconds"], 2)

    def test_actual_output_limit_preserves_prefix_and_marks_incomplete(self):
        with mock.patch.object(inventory, "MAX_STREAM_BYTES", 32):
            row = inventory.run_command("overflow", [sys.executable, "-c", "import sys;sys.stdout.write('x'*10000)"], 5, self.raw, time.monotonic() + 10)
        self.assertEqual(row["status"], "OUTPUT_LIMIT")
        self.assertFalse(row["streams_complete"])
        self.assertEqual((self.raw / "overflow.stdout.original").read_bytes(), b"x" * 33)

    def test_exhausted_aggregate_budget_never_launches_process(self):
        with mock.patch.object(inventory.subprocess, "Popen") as popen:
            row = inventory.run_command("unstarted", ["not-executed"], 10, self.raw, 1, clock=lambda: 2)
        popen.assert_not_called()
        self.assertEqual(row["status"], "NOT_STARTED")
        self.assertFalse(row["streams_complete"])

    def test_read_size_is_bounded_even_when_file_grows(self):
        source = self.root / "file"
        source.write_bytes(b"abcd")
        sizes = []
        class Growing(io.BytesIO):
            def fileno(self):
                return 123
            def read(self, size=-1):
                sizes.append(size)
                return super().read(size)
        snapshot = source.stat()
        with mock.patch.object(Path, "open", return_value=Growing(b"abcdef")), mock.patch.object(inventory.os, "fstat", return_value=snapshot):
            with self.assertRaisesRegex(inventory.InventoryError, "INPUT_CHANGED_OR_LIMIT"):
                inventory.read_regular(source, 4)
        self.assertEqual(sizes, [5])

    def test_metadata_captures_actual_missing_simulator_separately(self):
        apps = self.root / "Applications"
        xcode = apps / "Xcode_27.1.app"
        developer_apps = xcode / "Contents/Developer/Applications"
        developer_apps.mkdir(parents=True)
        (xcode / "Contents/Info.plist").write_bytes(plistlib.dumps({"CFBundleIdentifier": "com.apple.dt.Xcode", "CFBundleShortVersionString": "27.1"}))
        rows = [{"id": key, "status": "NOT_STARTED"} for key in ("selected-developer", "resolved-simctl", "resolved-xcodebuild")]
        result = inventory.capture_layout(self.raw, rows, time.monotonic() + 5, applications=apps)
        self.assertEqual(len(result["xcodes"]), 1)
        self.assertFalse(result["xcodes"][0]["simulator"]["is_directory"])
        self.assertEqual(result["xcodes"][0]["xcode_metadata"]["status"], "PARSED")
        self.assertTrue((self.raw / "bundle-00-xcode.Info.plist.original").is_file())

    def test_layout_bounds_xcode_count_without_recursive_walk(self):
        apps = self.root / "Applications"
        apps.mkdir()
        for index in range(3):
            (apps / ("Xcode" + str(index) + ".app")).mkdir()
        rows = [{"id": key, "status": "NOT_STARTED"} for key in ("selected-developer", "resolved-simctl", "resolved-xcodebuild")]
        with mock.patch.object(inventory, "MAX_XCODES", 2):
            result = inventory.capture_layout(self.raw, rows, time.monotonic() + 5, applications=apps)
        self.assertTrue(result["xcode_candidates_truncated"])
        self.assertEqual(len(result["xcodes"]), 2)

    def simulator_state(self):
        rows = []
        bodies = {
            "devices": {"devices": {"PRIVATE_RUNTIME_ID": [{"name": "PRIVATE_DEVICE_NAME", "udid": "PRIVATE_DEVICE_UDID", "state": "Booted", "isAvailable": True}]}},
            "runtimes": {"runtimes": [{"name": "PRIVATE_RUNTIME_NAME", "identifier": "PRIVATE_RUNTIME_ID", "isAvailable": True}]},
        }
        for kind, body in bodies.items():
            rows.append({"id": "simulator-" + kind, "status": "SUCCESS", "streams_complete": True})
            (self.raw / ("simulator-" + kind + ".stdout.original")).write_bytes(inventory.canonical(body))
        rows.append({"id": "receiver-project-tracked", "status": "SUCCESS"})
        (self.raw / "receiver-project-tracked.stdout.original").write_bytes(b"")
        return inventory.capture_simulator_state(self.raw, rows, repo=self.root)

    def test_simctl_state_survives_without_gui_or_receiver_project(self):
        state = self.simulator_state()
        self.assertEqual(state["devices"]["entries"][0]["state"], "Booted")
        self.assertTrue(state["runtimes"]["entries"][0]["isAvailable"])
        self.assertEqual(state["receiver_project"]["showdestinations_status"], "NOT_STARTED_SOURCE_PROJECT_ABSENT")
        self.assertFalse(state["receiver_project"]["project_generated"])

    def test_public_projection_never_copies_paths_device_names_or_udids(self):
        simulator = self.simulator_state()
        archive, ciphertext = self.root / "private.zip", self.root / "public.p7m"
        archive.write_bytes(b"synthetic private archive")
        ciphertext.write_bytes(b"synthetic ciphertext")
        row = {"id": "simulator-devices", "status": "SUCCESS", "exit_code": 0, "timed_out": False,
               "output_limit": False, "streams_complete": True, "budget_seconds": 25, "elapsed_seconds": 0.1,
               "argv": ["PRIVATE_EXECUTABLE_PATH"], "stderr": "PRIVATE_DIAGNOSTIC"}
        layout = {"xcodes": [{"simulator": {"is_directory": False, "path": "PRIVATE_SIMULATOR_PATH"}}]}
        result = inventory.public_summary("a" * 40, "123", [row], layout, simulator, {}, archive, ciphertext,
                                         {"recipient_sha256": "b" * 64, "cipher": "SMIME-DER-AES256"})
        self.assertNotIn("PRIVATE_", inventory.canonical(result).decode())
        self.assertEqual(result["device_state_counts"]["Booted"], 1)
        self.assertEqual(result["standard_simulator_directory_count"], 0)
        for key in ("formal_acceptance_granted", "headless_xctest_verified", "gui_absence_proves_runtime_unavailable", "boot_attempted", "build_attempted", "apple_test_executed"):
            self.assertIs(result[key], False)

    def test_archive_original_inventory_is_exact_and_contains_failure_bytes(self):
        (self.raw / "failure.stderr.original").write_bytes(b"private synthetic failure")
        archive = self.root / "raw.zip"
        entries = inventory.make_archive(self.raw, archive)
        self.assertEqual(set(entries), {"failure.stderr.original"})
        with zipfile.ZipFile(archive) as zipped:
            self.assertEqual(set(zipped.namelist()), {"failure.stderr.original", "originals-manifest.json"})
            self.assertEqual(zipped.read("failure.stderr.original"), b"private synthetic failure")
            self.assertEqual(json.loads(zipped.read("originals-manifest.json"))["originals"], entries)

    def test_changed_recipient_is_rejected_before_encryption(self):
        cert = self.root / "recipient.pem"
        cert.write_bytes(b"wrong certificate")
        with mock.patch.object(inventory.subprocess, "run") as run:
            with self.assertRaisesRegex(inventory.InventoryError, "RECIPIENT_CHANGED"):
                inventory.encrypt_archive(self.root / "raw.zip", cert, self.root / "out.p7m", time.monotonic() + 10)
        run.assert_not_called()

    def test_encryption_command_is_fixed_binary_der_aes256_and_deadlined(self):
        cert = self.root / "recipient.pem"
        cert.write_bytes(b"synthetic public certificate")
        archive, cipher = self.root / "raw.zip", self.root / "out.p7m"
        archive.write_bytes(b"synthetic plaintext")
        observed = []
        def run(argv, **kwargs):
            observed.append((argv, kwargs))
            cipher.write_bytes(b"ciphertext")
            return subprocess.CompletedProcess(argv, 0, b"", b"")
        with mock.patch.object(inventory, "CERTIFICATE_SHA256", inventory.digest(cert.read_bytes())), mock.patch.object(inventory.subprocess, "run", side_effect=run):
            result = inventory.encrypt_archive(archive, cert, cipher, 25, clock=lambda: 10)
        self.assertEqual(observed[0][0], ["/usr/bin/openssl", "smime", "-encrypt", "-binary", "-aes256", "-outform", "DER", "-in", str(archive), "-out", str(cipher), str(cert)])
        self.assertEqual(observed[0][1]["timeout"], 15)
        self.assertEqual(result["status"], "ENCRYPTED")

    def test_failed_encryption_does_not_produce_success_receipt(self):
        cert = self.root / "recipient.pem"
        cert.write_bytes(b"synthetic public certificate")
        with mock.patch.object(inventory, "CERTIFICATE_SHA256", inventory.digest(cert.read_bytes())), mock.patch.object(inventory.subprocess, "run", return_value=subprocess.CompletedProcess([], 1, b"", b"private error")):
            with self.assertRaisesRegex(inventory.InventoryError, "ENCRYPTION_FAILED"):
                inventory.encrypt_archive(self.root / "raw.zip", cert, self.root / "out.p7m", time.monotonic() + 10)


if __name__ == "__main__":
    unittest.main()
