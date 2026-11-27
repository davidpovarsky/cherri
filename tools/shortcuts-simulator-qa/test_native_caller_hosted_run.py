"""Focused FN04 hosted adapter controls; no Apple process or signer is invoked."""
import copy
import os
import json
import shutil
import signal
import threading
from pathlib import Path
import stat
import tempfile
import time
import unittest
from unittest import mock
import zipfile

import native_caller_execution_context as context
import native_caller_hosted_run as subject
import test_native_caller_execution_context as context_controls
import test_native_caller_input_owner as original_controls

ROOT = Path(__file__).resolve().parents[2]


class HostedCallerControllerTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.root = Path(self.temporary.name).resolve()

    def tearDown(self):
        self.temporary.cleanup()

    def test_authentic_packet_and_signature_zips_join_without_signing(self):
        packets = self.root / "packets"
        subject.extract_original_bundle(ROOT / subject.PACKET_ZIP[0],
                                        subject.PACKET_ZIP[1], packets)
        for profile in ("initial", "retry"):
            row = subject.SIGNING_ZIPS[profile]
            destination = self.root / profile
            subject.extract_original_bundle(ROOT / row[0], row[1], destination)
            self.assertEqual(subject.sha((destination / "intake.original.json").read_bytes()), row[2])
        for case in subject.execution.CASES:
            with self.subTest(case=case):
                plan = {"caseId": case, "compilerCommitSha": subject.PACKET_SOURCE,
                        "bundleRoot": str(packets), "bundleIntakeSHA256": subject.PACKET_INTAKE_SHA}
                _, _, checked, _ = subject.base.bind_packet_bundle(plan)
                profile = "retry" if case == "without_functions" else "initial"
                selection = {"bundleRoot": str(self.root / profile),
                             "bundleIntakeSHA256": subject.SIGNING_ZIPS[profile][2]}
                raw, binding, _, _, _ = subject.signed.bind_signed_bundle(selection, plan, checked)
                self.assertTrue(raw.startswith(b"AEA1"))
                self.assertFalse(binding["nativeExecuted"])
                self.assertFalse(binding["appleSignatureValidated"])
                self.assertFalse(binding["formalAcceptanceGranted"])

    def test_fixture_extraction_rejects_changed_hash_paths_links_duplicates_and_size(self):
        genuine = (ROOT / subject.PACKET_ZIP[0]).read_bytes()
        changed = self.root / "changed.zip"
        changed.write_bytes(genuine + b"changed")
        with self.assertRaisesRegex(ValueError, "AUTHENTIC_FIXTURE_ZIP"):
            subject.extract_original_bundle(changed, subject.PACKET_ZIP[1], self.root / "no-changed")
        self.assertFalse((self.root / "no-changed").exists())
        cases = [
            ("traversal", [("../outside", b"x", stat.S_IFREG | 0o600)]),
            ("absolute", [("/outside", b"x", stat.S_IFREG | 0o600)]),
            ("link", [("link", b"target", stat.S_IFLNK | 0o777)]),
            ("duplicate", [("same", b"x", stat.S_IFREG | 0o600)]*2),
            ("count", [(str(i), b"", stat.S_IFREG | 0o600) for i in range(65)]),
        ]
        for label, members in cases:
            with self.subTest(case=label):
                path = self.root / (label + ".zip")
                with zipfile.ZipFile(path, "x") as archive:
                    for name, raw, mode in members:
                        info = zipfile.ZipInfo(name)
                        info.external_attr = mode << 16
                        archive.writestr(info, raw)
                with self.assertRaises(ValueError):
                    subject.extract_original_bundle(path, subject.sha(path.read_bytes()), self.root / label)
                self.assertFalse((self.root / label).exists())
        with mock.patch.object(subject, "MAX_EXPANDED_BYTES", 100):
            with self.assertRaisesRegex(ValueError, "FIXTURE_ZIP_BOUND"):
                subject.extract_original_bundle(ROOT / subject.PACKET_ZIP[0],
                    subject.PACKET_ZIP[1], self.root / "small-bound")
        self.assertFalse((self.root / "small-bound").exists())

    @staticmethod
    def inventories():
        return ({"runtimes": [{"identifier": subject.RUNTIME_ID, "isAvailable": True,
                              "name": "iOS 27.0", "version": "27.0"}]},
                {"devicetypes": [{"identifier": subject.DEVICE_TYPE, "name": "iPhone 16 Pro"}]},
                {"devices": {subject.RUNTIME_ID: []}})

    def test_device_creation_requires_exact_available_runtime_and_unused_name(self):
        runtimes, types, devices = self.inventories()
        value = {"sourceCommitSha": "a"*40, "runId": "1234567"}
        record = subject.choose_new_device(runtimes, types, devices, value)
        self.assertEqual(record["deviceName"], "Cherri-FN04-1234567-1")
        self.assertIsNone(record["udid"])
        mutations = []
        changed = copy.deepcopy(runtimes); changed["runtimes"][0]["isAvailable"] = False
        mutations.append((changed, types, devices))
        changed = copy.deepcopy(runtimes); changed["runtimes"] *= 2
        mutations.append((changed, types, devices))
        changed = copy.deepcopy(runtimes); changed["runtimes"][0]["identifier"] = "other"
        mutations.append((changed, types, devices))
        changed = copy.deepcopy(types); changed["devicetypes"][0]["identifier"] = "other"
        mutations.append((runtimes, changed, devices))
        changed = copy.deepcopy(devices); changed["devices"][subject.RUNTIME_ID] = [{"name": record["deviceName"]}]
        mutations.append((runtimes, types, changed))
        for index, triple in enumerate(mutations):
            with self.subTest(mutation=index):
                with self.assertRaises(ValueError):
                    subject.choose_new_device(*triple, value)

    def test_native_result_cannot_inherit_green_from_wrong_context_or_incomplete_owner(self):
        plan, ctx = {"runNonce": "a"*32}, {"runId": "1234567"}
        value = {"schema": subject.base.SCHEMA, "plan": plan, "hostedContext": ctx,
            "executionLocation": "GITHUB_ACTIONS_MACOS", "status": "OBSERVATION_VERIFIED",
            "runtimeObservationAuthenticated": True, "actualRuntimeDispatches": 1,
            "sourceUnchanged": True, "firstInterruption": None,
            "wholeOwnerSeconds": 1200, "finalReserveSeconds": 300,
            "originalLiveFloorBytes": 4*1024**3, "formalAcceptanceGranted": False,
            "rawNativeTypesObserved": False, "compilerRebuilt": False, "harnessRebuilt": False}
        self.assertEqual(subject.validate_native_result(value, plan, ctx), value)
        for key, changed in (("hostedContext", {"runId": "other"}), ("plan", {}),
                ("executionLocation", "EXISTING_LOCAL_MAC"), ("status", "OBSERVATION_ACQUIRED"),
                ("runtimeObservationAuthenticated", False), ("actualRuntimeDispatches", True),
                ("actualRuntimeDispatches", 2), ("sourceUnchanged", False),
                ("firstInterruption", "SIGTERM"), ("finalizationErrors", ["source"]),
                ("error", {"type": "ValueError"}), ("wholeOwnerSeconds", 1300),
                ("finalReserveSeconds", 400), ("originalLiveFloorBytes", 3*1024**3),
                ("rawNativeTypesObserved", True), ("formalAcceptanceGranted", True)):
            with self.subTest(field=key, value=changed):
                with self.assertRaises(ValueError):
                    subject.validate_native_result(dict(value, **{key: changed}), plan, ctx)

    def test_build_origin_rejects_local_receipt_and_other_hosted_run(self):
        fixture = context_controls.HostedCallerContextTests()
        fixture.setUp()
        try:
            with mock.patch.dict(os.environ, fixture.environment, clear=True):
                ctx, _ = fixture.model()
                value = {"hostedContext": ctx, "executionLocation": "GITHUB_ACTIONS_MACOS"}
                context.validate_build_origin(value, ctx)
                for bad in ({}, {"executionLocation": "EXISTING_LOCAL_MAC", "hostedContext": ctx},
                            dict(value, hostedContext=dict(ctx, runId="other"))):
                    with self.subTest(value=bad):
                        with self.assertRaisesRegex(ValueError, "ACTUAL_HOSTED_BUILD_CONTEXT"):
                            context.validate_build_origin(bad, ctx)
        finally:
            fixture.tearDown()

    def test_actual_auxiliary_child_is_reaped_without_native_invocation(self):
        current = subject.HostedCaller.__new__(subject.HostedCaller)
        current.private = self.root
        (self.root / "processes").mkdir()
        current.hard_deadline = time.monotonic() + 20
        current.execution_deadline = current.hard_deadline
        current.latch = subject.base.InterruptionLatch()
        current.state = {"nativeChildInvocations": 0, "nativeOwnerInvocations": 0, "processes": []}
        with mock.patch.object(current, "capacity", return_value=8*1024**3):
            observed, _ = current.command("metadata-control", ["/bin/true"], 2)
        self.assertEqual(observed["result"], "PASS")
        self.assertEqual(current.state["nativeChildInvocations"], 0)
        process = current.state["processes"][0]["process"]
        self.assertTrue(process["cleanupParentReaped"])
        self.assertFalse(process["groupPresentAfterCleanup"])
        current.execution_deadline = time.monotonic()
        with mock.patch.object(subject.base.capture, "capture") as launch:
            with self.assertRaisesRegex(ValueError, "HOSTED_COMMAND_BUDGET"):
                current.command("metadata-control", ["/bin/true"], 2)
            launch.assert_not_called()
        self.assertEqual(current.state["nativeChildInvocations"], 0)


    def test_in_process_original_native_finish_reaps_before_first_signal_returns(self):
        current = subject.HostedCaller.__new__(subject.HostedCaller)
        current.private = self.root / "controller"
        current.private.mkdir()
        current.paths = {"root": self.root / "native"}
        current.hard_deadline = time.monotonic() + 1500
        current.execution_deadline = current.hard_deadline - 100
        current.latch = subject.base.InterruptionLatch()
        current.state = {"nativeChildInvocations": 0, "nativeOwnerInvocations": 0,
                         "inProcessInvocations": [], "processes": []}
        current.plan, current.context = {}, {}
        owner = subject.base.NativeCallerObservation.__new__(subject.base.NativeCallerObservation)
        owner.plan = original_controls.NativeCallerOwnerTests.plan()
        owner.repo = self.root / "repo"
        owner.root = current.paths["root"]
        owner.folder = owner.root / "runtime"
        owner.folder.mkdir(parents=True)
        owner.xctest = None
        owner.test_log = (owner.folder / "xctest.log").open("xb")
        owner.before = owner.products_before = owner.packet = None
        owner.caller_entered = False
        owner.started = time.monotonic()
        owner.hard_deadline = owner.started + subject.base.WHOLE_SECONDS
        owner.deadline = owner.hard_deadline - subject.base.FINAL_RESERVE_SECONDS
        owner.latch = subject.base.InterruptionLatch()
        owner.stop, owner.watcher = threading.Event(), None
        owner.minimum_free = owner.capacity_reason = None
        owner.summary = {"status": "FAIL", "runtimeObservationAuthenticated": False,
                         "actualRuntimeDispatches": None, "processes": [],
                         "formalAcceptanceGranted": False, "rawNativeTypesObserved": False}
        subject.base.start_registered_xctest(owner, ["/bin/sleep", "30"], os.environ.copy())
        child = owner.xctest
        real_wait = child.wait
        real_group = subject.base.capture.group_observation
        injected, rescue = [], None
        def wait(*args, **kwargs):
            if kwargs.get("timeout") == 90:
                raise RuntimeError("INJECTED_FINISH_WAIT_EXCEPTION")
            return real_wait(*args, **kwargs)
        def group(pgid):
            if pgid == child.pid and not injected:
                injected.append({"pid": child.pid, "signal": int(signal.SIGTERM),
                                 "stage": "original-finish-group-observation"})
                os.kill(os.getpid(), signal.SIGTERM)
            return real_group(pgid)
        old_handlers = {s: signal.getsignal(s) for s in
                        (signal.SIGALRM, signal.SIGTERM, signal.SIGINT)}
        old_timer = signal.getitimer(signal.ITIMER_REAL)
        def controller_signal(signum, frame):
            current.latch.interrupt("CONTROLLER_SIGNAL_" + str(signum))
        for signum in old_handlers:
            signal.signal(signum, controller_signal)
        signal.setitimer(signal.ITIMER_REAL, 1500)
        try:
            with mock.patch.object(subject.signed, "SignedNativeCallerObservation", return_value=owner), \
                 mock.patch.object(current, "capacity", return_value=8*1024**3), \
                 mock.patch.object(owner, "check_capacity", side_effect=ValueError("INJECTED_BODY_FAILURE")), \
                 mock.patch.object(child, "wait", side_effect=wait), \
                 mock.patch.object(subject.base.capture, "group_observation", side_effect=group):
                returned = current.run_native_original({})
            self.assertEqual(len(injected), 1)
            self.assertEqual(returned["firstInterruption"],
                             "NATIVE_OWNER_SIGNAL_" + str(signal.SIGTERM))
            self.assertEqual(current.latch.reason,
                             "INNER_native-owner:" + returned["firstInterruption"])
            self.assertIsNotNone(child.poll())
            self.assertFalse(real_group(child.pid)["present"])
            self.assertEqual(current.state["nativeOwnerInvocations"], 1)
            self.assertEqual(current.state["nativeChildInvocations"], 0)
            self.assertEqual(current.state["inProcessInvocations"][0]["execution"],
                             "IN_PROCESS_ORIGINAL_OWNER")
            self.assertEqual(signal.getitimer(signal.ITIMER_REAL), (0.0, 0.0))
            with mock.patch.object(subject.base.capture, "capture") as launch:
                with self.assertRaisesRegex(ValueError, "HOSTED_COMMAND_BUDGET"):
                    current.command("must-not-launch-after-inner-signal", ["/bin/true"], 1)
                launch.assert_not_called()
            finished = json.loads((owner.folder / "xctest-process.finished.original.json").read_bytes())
            self.assertTrue(finished["parentReaped"])
            self.assertFalse(finished["groupAfterCleanup"]["present"])
        finally:
            signal.setitimer(signal.ITIMER_REAL, 0)
            for signum, handler in old_handlers.items():
                signal.signal(signum, handler)
            signal.setitimer(signal.ITIMER_REAL, *old_timer)
            if child.poll() is None or real_group(child.pid)["present"] is not False:
                rescue = subject.base.capture.kill_group(child)
                real_wait(timeout=5)
            subject.record(self.root / "in-process-signal-control.original.json",
                {"injection": injected, "pid": child.pid, "returnCode": child.returncode,
                 "groupAfterCleanup": real_group(child.pid), "testRescue": rescue,
                 "controllerFirstInterruption": current.latch.reason,
                 "nativeExecuted": False})
            destination = os.environ.get("CHERRI_HOSTED_CONTROL_ORIGINALS")
            if destination:
                shutil.copytree(self.root, Path(destination) / "native-first-cleanup-signal")
            self.assertIsNone(rescue, "original owner must finish without test rescue")

    def test_in_process_owner_restores_same_absolute_deadline(self):
        current = subject.HostedCaller.__new__(subject.HostedCaller)
        current.private = self.root / "controller"
        current.private.mkdir()
        current.hard_deadline = time.monotonic() + 200
        current.execution_deadline = current.hard_deadline - 20
        current.latch = subject.base.InterruptionLatch()
        current.state = {"inProcessInvocations": []}
        result_path = self.root / "result.original.json"
        original_deadline = current.hard_deadline
        old_timer = signal.getitimer(signal.ITIMER_REAL)
        try:
            def completed():
                signal.setitimer(signal.ITIMER_REAL, 0)
                subject.record(result_path, {"firstInterruption": None})
                return 0
            with mock.patch.object(current, "capacity", return_value=8*1024**3):
                self.assertEqual(current.invoke_original_owner("standalone-build-owner",
                    completed, 20, result_path), 0)
            remaining, interval = signal.getitimer(signal.ITIMER_REAL)
            self.assertEqual(interval, 0)
            self.assertLessEqual(time.monotonic()+remaining, original_deadline+0.02)
            self.assertGreater(remaining, 0)
            self.assertEqual(current.hard_deadline, original_deadline)
        finally:
            signal.setitimer(signal.ITIMER_REAL, *old_timer)


if __name__ == "__main__":
    unittest.main()
