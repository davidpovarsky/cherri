"""Controls for new FN04 owner bindings and cancellation ownership.

The synthetic XCTest/state-machine controls do not execute an Apple product.
Authentic packet originals are separately replayed by the qualification owner.
"""
import copy
import hashlib
import json
import os
from pathlib import Path
import signal
import subprocess
import tempfile
import time
import types
import unittest
from unittest import mock

import native_caller_input_owner as subject


class NativeCallerOwnerTests(unittest.TestCase):
    def setUp(self):
        self.temporary = None
        retained = os.environ.get("CHERRI_CALLER_OWNER_TEST_ROOT")
        if retained:
            self.base = Path(retained) / self._testMethodName
            self.base.mkdir(mode=0o700)
        else:
            self.temporary = tempfile.TemporaryDirectory()
            self.base = Path(self.temporary.name)

    def tearDown(self):
        if self.temporary is not None:
            self.temporary.cleanup()

    @staticmethod
    def plan():
        controller, compiler, built = "c" * 40, "a" * 40, "b" * 40
        return {"schema": subject.PLAN_SCHEMA, "repo": "/Users/m1/builds/test-source",
            "root": "/Users/m1/builds/test-native-output",
            "controllerCommitSha": controller, "compilerCommitSha": compiler,
            "builtHarnessCommitSha": built, "buildRoot": "/Users/m1/builds/test-build",
            "buildResultSHA256": "1" * 64, "bundleRoot": "/Users/m1/builds/test-packets",
            "bundleIntakeSHA256": "2" * 64, "pythonExecutableSHA256": "3" * 64,
            "runId": "owned-control-run", "runNonce": "4" * 32, "caseId": "nested_false",
            "estimatedIncrementalBytes": 128 * 1024**2,
            "deviceRecord": {"schema": subject.selection.SCHEMA,
                "sourceCommit": controller, "runId": "owned-control-run",
                "udid": "D631D98E-2271-4948-B465-8EDBA9BD4D68",
                "deviceName": "Cherri-P10R15-D03",
                "deviceTypeIdentifier": "com.apple.CoreSimulator.SimDeviceType.iPhone-16-Pro",
                "runtimeId": "com.apple.CoreSimulator.SimRuntime.iOS-27-0",
                "runtimeName": "iOS 27.0", "runtimeVersion": "27.0"}}

    @staticmethod
    def build_model():
        plan = NativeCallerOwnerTests.plan()
        project = {subject.PROJECT + "control-%02d.swift" % i:
            {"mode": "100644", "gitBlob": ("%040x" % (i + 1)),
             "bytes": i + 1, "sha256": "%064x" % (i + 1)} for i in range(15)}
        source = {"commit": plan["builtHarnessCommitSha"], "tree": "d" * 40,
            "completeTrackedSource": True, "criticalSource": {"sourceClean": True},
            "files": copy.deepcopy(project)}
        process = lambda label: {"label": label,
            "observation": {"result": "PASS", "exitCode": 0},
            "process": {"cleanupParentReaped": True, "groupPresentAfterCleanup": False,
                "stdout": {"eof": True, "overflow": False},
                "stderr": {"eof": True, "overflow": False}}, "firstInterruption": None}
        value = {"schema": "cherri-native-caller-standalone-build/1", "status": "BUILD_VALIDATED",
            "builtHarnessCommitSha": plan["builtHarnessCommitSha"],
            "compiledInputEquivalentSource": plan["compilerCommitSha"],
            "builtHarnessTree": "d" * 40, "sourceUnchanged": True,
            "generatedProjectRetained": True, "nativeInvocation": False,
            "testsExecuted": False, "compilerRebuilt": False, "firstInterruption": None,
            "error": None, "sourceAfterError": None, "wholeOwnerSeconds": 780,
            "buildSeconds": 600, "projectInputCount": 15,
            "processes": [process(label) for label in subject.BUILD_LABELS]}
        return plan, value, source, project

    def test_closed_plan_rejects_identity_nonce_clock_and_namespace_gaps(self):
        valid = self.plan()
        self.assertEqual(subject.validate_plan(copy.deepcopy(valid)), valid)
        mutations = [
            ("unknown field", lambda p: p.update(extra=True)),
            ("missing Python pin", lambda p: p.pop("pythonExecutableSHA256")),
            ("malformed Python pin", lambda p: p.update(pythonExecutableSHA256="3" * 63)),
            ("malformed compiler", lambda p: p.update(compilerCommitSha="x" * 40)),
            ("zero native nonce", lambda p: p.update(runNonce="0" * 32)),
            ("wrong case", lambda p: p.update(caseId="nested_maybe")),
            ("case wrong type", lambda p: p.update(caseId=[])),
            ("same namespaces", lambda p: p.update(root=p["repo"])),
            ("nested output", lambda p: p.update(root=p["repo"] + "/out")),
            ("path escape", lambda p: p.update(root="/Users/m1/builds/../out")),
            ("device source", lambda p: p["deviceRecord"].update(sourceCommit="f" * 40)),
            ("device run", lambda p: p["deviceRecord"].update(runId="other")),
            ("zero estimate", lambda p: p.update(estimatedIncrementalBytes=0)),
            ("boolean estimate", lambda p: p.update(estimatedIncrementalBytes=True)),
        ]
        for name, mutate in mutations:
            with self.subTest(case=name):
                candidate = copy.deepcopy(valid)
                mutate(candidate)
                with self.assertRaises(ValueError):
                    subject.validate_plan(candidate)

    def test_build_binding_requires_original_source_bounds_and_complete_processes(self):
        plan, value, source, project = self.build_model()
        self.assertEqual(subject.validate_build_metadata(value, source, copy.deepcopy(source),
                                                         plan, project), project)
        mutations = [
            ("wrong compiler", lambda v: v.update(compiledInputEquivalentSource="e" * 40)),
            ("wrong harness", lambda v: v.update(builtHarnessCommitSha="e" * 40)),
            ("failed original", lambda v: v.update(status="FAIL")),
            ("source not closed", lambda v: v.update(sourceUnchanged=False)),
            ("closing source error", lambda v: v.update(sourceAfterError={"type": "TimeoutError"})),
            ("generated project lost", lambda v: v.update(generatedProjectRetained=False)),
            ("changed build bound", lambda v: v.update(buildSeconds=601)),
            ("missing process", lambda v: v["processes"].pop()),
            ("wrong process order", lambda v: v["processes"].reverse()),
            ("failed build child", lambda v: v["processes"][6]["observation"].update(exitCode=65)),
            ("unreaped child", lambda v: v["processes"][6]["process"].update(cleanupParentReaped=False)),
            ("unknown group", lambda v: v["processes"][6]["process"].update(groupPresentAfterCleanup=None)),
            ("incomplete original", lambda v: v["processes"][6]["process"]["stdout"].update(eof=False)),
            ("overflowed original", lambda v: v["processes"][6]["process"]["stderr"].update(overflow=True)),
        ]
        for name, mutate in mutations:
            with self.subTest(case=name):
                candidate = copy.deepcopy(value)
                mutate(candidate)
                with self.assertRaises(ValueError):
                    subject.validate_build_metadata(candidate, source, copy.deepcopy(source),
                                                    plan, project)

    def test_build_binding_rejects_changed_source_and_current_compiled_bytes(self):
        plan, value, source, project = self.build_model()
        changed = copy.deepcopy(source)
        changed["files"][next(iter(project))]["sha256"] = "f" * 64
        with self.assertRaisesRegex(ValueError, "BUILD_COMPLETE_CLEAN_SOURCE"):
            subject.validate_build_metadata(value, source, changed, plan, project)
        for mutation in ("missing", "byte", "mode"):
            with self.subTest(case=mutation):
                current = copy.deepcopy(project)
                key = next(iter(current))
                if mutation == "missing":
                    del current[key]
                elif mutation == "byte":
                    current[key]["sha256"] = "f" * 64
                else:
                    current[key]["mode"] = "100755"
                with self.assertRaisesRegex(ValueError, "ALL_COMPILED_INPUTS_EQUIVALENT"):
                    subject.validate_build_metadata(value, source, copy.deepcopy(source),
                                                    plan, current)
        incomplete = copy.deepcopy(source)
        incomplete["completeTrackedSource"] = False
        with self.assertRaisesRegex(ValueError, "BUILD_COMPLETE_CLEAN_SOURCE"):
            subject.validate_build_metadata(value, incomplete, copy.deepcopy(incomplete),
                                            plan, project)

    def owned_registration(self, interrupt):
        log = self.base / "synthetic-child.stdout.original"
        owner = types.SimpleNamespace(folder=self.base, xctest=None, test_log=log.open("xb"))
        real_popen = subprocess.Popen
        created = []
        old_handler = signal.getsignal(signal.SIGTERM)
        def cancelled(signum, frame):
            raise TimeoutError("INJECTED_CANCEL_AFTER_ACTUAL_POPEN")
        def popen(*args, **kwargs):
            child = real_popen(*args, **kwargs)
            created.append(child)
            self.assertEqual(os.getpgid(child.pid), child.pid)
            if interrupt:
                os.kill(os.getpid(), signal.SIGTERM)
            return child
        signal.signal(signal.SIGTERM, cancelled)
        try:
            with mock.patch.object(subject.subprocess, "Popen", side_effect=popen):
                if interrupt:
                    with self.assertRaisesRegex(TimeoutError, "INJECTED_CANCEL"):
                        subject.start_registered_xctest(owner, ["/bin/sleep", "30"], os.environ.copy())
                else:
                    with mock.patch.object(subject, "record", side_effect=OSError("INJECTED_JOURNAL_FAILURE")):
                        with self.assertRaisesRegex(OSError, "INJECTED_JOURNAL_FAILURE"):
                            subject.start_registered_xctest(owner, ["/bin/sleep", "30"], os.environ.copy())
            self.assertEqual(len(created), 1)
            self.assertIs(owner.xctest, created[0])
        finally:
            signal.signal(signal.SIGTERM, old_handler)
            for child in created:
                cleanup = subject.capture.kill_group(child)
                child.wait(timeout=5)
                group = subject.capture.group_observation(child.pid)
                self.assertFalse(group["present"])
                subject.record(self.base / "synthetic-child.finished.original.json",
                    {"argv": ["/bin/sleep", "30"], "pid": child.pid,
                     "exitCode": child.returncode, "parentReaped": child.returncode is not None,
                     "cleanup": cleanup, "groupAfterCleanup": group,
                     "injection": "Popen-return cancellation" if interrupt else "journal write failure",
                     "nativeInvocation": False})
            owner.test_log.close()

    def test_actual_child_registered_before_deferred_cancellation(self):
        self.owned_registration(True)

    def test_actual_child_registered_when_started_journal_fails(self):
        self.owned_registration(False)


    def test_first_signal_during_actual_finish_cleanup_is_deferred(self):
        owner = subject.NativeCallerObservation.__new__(subject.NativeCallerObservation)
        owner.plan = self.plan()
        owner.repo = self.base / "repo"
        owner.root = self.base / "output"
        owner.root.mkdir()
        owner.folder = owner.root / "runtime"
        owner.folder.mkdir()
        owner.xctest = None
        owner.test_log = (owner.folder / "xctest.log").open("xb")
        owner.before = owner.products_before = owner.packet = None
        owner.caller_entered = False
        owner.started = time.monotonic()
        owner.hard_deadline = owner.started + subject.WHOLE_SECONDS
        owner.deadline = owner.hard_deadline - subject.FINAL_RESERVE_SECONDS
        owner.latch = subject.InterruptionLatch()
        owner.stop = subject.threading.Event()
        owner.watcher = None
        owner.minimum_free = owner.capacity_reason = None
        owner.summary = {"status": "FAIL", "runtimeObservationAuthenticated": False,
                         "actualRuntimeDispatches": None, "processes": [],
                         "formalAcceptanceGranted": False, "rawNativeTypesObserved": False}
        real_group = subject.capture.group_observation
        injected = []
        observations = []
        subject.start_registered_xctest(owner, ["/bin/sleep", "30"], os.environ.copy())
        child = owner.xctest
        real_wait = child.wait
        def wait(*args, **kwargs):
            if kwargs.get("timeout") == 90:
                # Isolate the inherited post-wait cleanup branch without
                # spending 90 seconds or altering its production deadline.
                raise RuntimeError("INJECTED_FINISH_WAIT_EXCEPTION")
            return real_wait(*args, **kwargs)
        def group(pgid):
            if pgid == child.pid and not injected:
                injected.append({"stage": "first-finish-group-observation",
                                 "signal": int(signal.SIGTERM), "pid": child.pid})
                os.kill(os.getpid(), signal.SIGTERM)
            value = real_group(pgid)
            observations.append(value)
            return value
        rescue = None
        completed = None
        try:
            with mock.patch.object(owner, "check_capacity",
                    side_effect=ValueError("INJECTED_BODY_FAILURE")), \
                 mock.patch.object(child, "wait", side_effect=wait), \
                 mock.patch.object(subject.capture, "group_observation", side_effect=group):
                completed = owner.run()
            self.assertEqual(len(injected), 1)
            self.assertEqual(completed["firstInterruption"],
                             "NATIVE_OWNER_SIGNAL_" + str(signal.SIGTERM))
            self.assertEqual(completed["status"], "FAIL")
            self.assertFalse(completed["runtimeObservationAuthenticated"])
            self.assertEqual(completed["actualRuntimeDispatches"], 0)
            self.assertIsNotNone(child.poll(), "owned finish must reap before delivering cancellation")
            self.assertFalse(real_group(child.pid)["present"])
            finished = json.loads((owner.folder / "xctest-process.finished.original.json").read_bytes())
            self.assertTrue(finished["parentReaped"])
            self.assertFalse(finished["groupAfterCleanup"]["present"])
            self.assertEqual(finished["exitCode"], -signal.SIGKILL)
            self.assertIn("INJECTED_FINISH_WAIT_EXCEPTION", finished["pendingError"])
            self.assertGreaterEqual(len(observations), 2)
        finally:
            before_rescue = real_group(child.pid)
            if child.poll() is None or before_rescue["present"] is not False:
                rescue = subject.capture.kill_group(child)
                real_wait(timeout=5)
            after = real_group(child.pid)
            owner.test_log.close()
            subject.record(self.base / "finish-signal-control.original.json",
                {"injection": injected, "groupObservationsInsideFinish": observations,
                 "bodyFailureInjected": "INJECTED_BODY_FAILURE",
                 "waitExceptionInjected": "INJECTED_FINISH_WAIT_EXCEPTION",
                 "originalFinishWaitSeconds": 90,
                 "nativeInvocation": False, "result": completed,
                 "groupBeforeTestRescue": before_rescue, "testRescue": rescue,
                 "pid": child.pid, "parentReaped": child.returncode is not None,
                 "exitCode": child.returncode, "groupAfterTestCleanup": after})
            self.assertIsNotNone(child.returncode)
            self.assertFalse(after["present"])

    def state_machine(self, *, caller_failure=False, source_change=False,
                      products_change=False, xctest_failure=False, signing_failure=False):
        plan = self.plan()
        owner = subject.NativeCallerObservation.__new__(subject.NativeCallerObservation)
        owner.plan = plan
        owner.repo = self.base / "repo"
        owner.root = self.base / "output"
        owner.root.mkdir()
        owner.folder = owner.root / "runtime"
        owner.folder.mkdir()
        owner.process_folder = owner.folder / "processes"
        owner.process_folder.mkdir()
        owner.device = plan["deviceRecord"]["udid"]
        owner.device_data_path = None
        owner.xctest = owner.test_log = owner.documents = owner.build = None
        owner.before = owner.packet = owner.canonical_fixture = owner.checked = None
        owner.products_before = None
        owner.unsigned = owner.signed = owner.signed_pin = None
        owner.caller_entered = False
        owner.admission_verified = False
        owner.started = time.monotonic()
        owner.hard_deadline = owner.started + subject.WHOLE_SECONDS
        owner.deadline = owner.hard_deadline - subject.FINAL_RESERVE_SECONDS
        owner.latch = subject.InterruptionLatch()
        owner.stop = subject.threading.Event()
        owner.watcher = None
        owner.minimum_free = None
        owner.capacity_reason = None
        owner.summary = {"status": "FAIL", "runtimeObservationAuthenticated": False,
                         "actualRuntimeDispatches": None, "processes": [],
                         "formalAcceptanceGranted": False, "rawNativeTypesObserved": False}
        source = {"completeTrackedSource": True, "control": "original"}
        after = {"completeTrackedSource": True, "control": "changed"} if source_change else copy.deepcopy(source)
        products = {"control": "original"}
        after_products = {"control": "changed"} if products_change else copy.deepcopy(products)
        build = {"xctestrun": "/unused-test-model.xctestrun"}
        checked = {"name": "CherriFN04Direct_" + "5" * 32,
                   "unsignedSha256": subject.sha(b"unsigned"), "unsigned": b"unsigned"}
        packet, canonical, evidence = {"packet": "model"}, {"canonical": "model"}, {"binding": "model"}
        def sign():
            owner.packet, owner.canonical_fixture, owner.checked = packet, canonical, checked
            if signing_failure:
                raise ValueError("INJECTED_SIGNING_FAILURE")
            owner.unsigned, owner.signed = owner.folder / "unsigned", owner.folder / "signed"
            owner.unsigned.write_bytes(b"unsigned")
            owner.signed.write_bytes(b"AEA1signed")
            owner.signed_pin = {"bytes": 10, "sha256": subject.sha(b"AEA1signed")}
        def start():
            owner.xctest = types.SimpleNamespace(returncode=0)
            (owner.folder / "xctest.log").write_bytes(b"synthetic state-machine control only\n")
        result = {"actualRuntimeDispatches": 1}
        with mock.patch.object(owner, "check_capacity", return_value=8 * 1024**3), \
             mock.patch.object(owner, "start_capacity_watch"), \
             mock.patch.object(owner, "sign_original", side_effect=sign), \
             mock.patch.object(owner, "select_device"), \
             mock.patch.object(owner, "start_observer", side_effect=start), \
             mock.patch.object(subject.source_binding, "source_inventory", side_effect=[source, after]), \
             mock.patch.object(subject, "bind_build", return_value=build), \
             mock.patch.object(subject.native, "test_products_inventory", side_effect=[products, after_products]), \
             mock.patch.object(subject, "bind_packet_bundle", return_value=(packet, canonical, checked, evidence)), \
             mock.patch.object(subject.native.NativeObservation, "finish"), \
             mock.patch.object(subject.native, "validate_xctest",
                 side_effect=ValueError("INJECTED_XCTEST_FAILURE") if xctest_failure else None), \
             mock.patch.object(subject.caller, "observe_caller_input",
                 side_effect=ValueError("INJECTED_CALLBACK_FAILURE") if caller_failure else None,
                 return_value=result):
            return owner.run(), owner

    def test_completed_observation_requires_all_closing_proofs(self):
        value, owner = self.state_machine()
        self.assertEqual(value["status"], "OBSERVATION_VERIFIED")
        self.assertTrue(value["runtimeObservationAuthenticated"])
        self.assertEqual(value["actualRuntimeDispatches"], 1)
        self.assertFalse(value["formalAcceptanceGranted"])
        self.assertFalse(value["rawNativeTypesObserved"])
        self.assertTrue((owner.folder / "result.original.json").is_file())

    def test_source_change_rejects_already_acquired_runtime(self):
        value, _ = self.state_machine(source_change=True)
        self.assertEqual(value["status"], "FAIL")
        self.assertFalse(value["runtimeObservationAuthenticated"])
        self.assertEqual(value["actualRuntimeDispatches"], 1)
        self.assertIn("CONTROLLER_SOURCE_CHANGED", str(value["finalizationErrors"]))

    def test_product_change_rejects_already_acquired_runtime(self):
        value, _ = self.state_machine(products_change=True)
        self.assertEqual(value["status"], "FAIL")
        self.assertFalse(value["runtimeObservationAuthenticated"])
        self.assertIn("BUILT_PRODUCTS_CHANGED", str(value["finalizationErrors"]))

    def test_failed_xctest_rejects_already_acquired_runtime(self):
        value, _ = self.state_machine(xctest_failure=True)
        self.assertEqual(value["status"], "FAIL")
        self.assertFalse(value["runtimeObservationAuthenticated"])
        self.assertIn("INJECTED_XCTEST_FAILURE", str(value["finalizationErrors"]))

    def test_failed_caller_preserves_unknown_dispatch_count(self):
        value, _ = self.state_machine(caller_failure=True)
        self.assertEqual(value["status"], "FAIL")
        self.assertTrue(value["callerObservationEntered"])
        self.assertIsNone(value["actualRuntimeDispatches"])
        self.assertFalse(value["runtimeObservationAuthenticated"])
        self.assertEqual(value["error"]["message"], "INJECTED_CALLBACK_FAILURE")

    def test_sign_failure_preserves_original_error_without_false_missing_path(self):
        value, _ = self.state_machine(signing_failure=True)
        self.assertEqual(value["status"], "FAIL")
        self.assertFalse(value["callerObservationEntered"])
        self.assertEqual(value["actualRuntimeDispatches"], 0)
        self.assertEqual(value["error"]["message"], "INJECTED_SIGNING_FAILURE")
        self.assertNotIn("AttributeError", str(value))


if __name__ == "__main__":
    unittest.main()
