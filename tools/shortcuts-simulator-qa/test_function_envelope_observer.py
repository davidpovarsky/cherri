"""Causal controls for the fixed outer function observer.

All signing/build/process/UI fixtures below are synthetic, temporary test data.
This suite never starts simctl, Xcode, a signing service or a native workflow.
"""
import copy
import hashlib
import json
import os
from pathlib import Path
import plistlib
import tempfile
import threading
from types import SimpleNamespace
import unittest
from unittest import mock

import function_envelope_observer as observer


def raw(value):
    return json.dumps(value, sort_keys=True, separators=(",", ":")).encode() + b"\n"


def put(path, data):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(data)


def plan():
    return {
        "schema": "cherri-function-envelope-native-observer-plan/1",
        "repo": "/Users/m1/builds/source",
        "root": "/Users/m1/builds/originals",
        "controllerCommitSha": "a" * 40,
        "builtHarnessCommitSha": observer.BUILD_SOURCE,
        "compilerCommitSha": observer.runtime.COMPILER_SOURCE,
        "receiverBuildRoot": "/Users/m1/builds/build-originals",
        "receiverBuildQualificationSHA256": observer.BUILD_QUALIFICATION,
        "udid": observer.DEVICE, "runId": "owned-function-01",
        "runAttempt": 1, "runNonce": "nonce-first-01",
    }


def import_pair():
    argv = ["/usr/bin/xcrun", "simctl", "openurl", observer.DEVICE, "file:///owned/signed.shortcut"]
    dispatch = {"command": argv, "outcome": "EXITED", "timedOut": False, "exitCode": 0,
                "processReturncode": 0, "parentReaped": True, "timeoutSeconds": 60}
    receipt = {"id": "import-one", "operation": "import", "outcome": "IMPORTED",
               "name": observer.launch.WORKFLOW, "identityVerified": True,
               "libraryIdentityVerified": True,
               "importTiming": {"contract": "dispatch-exit-anchored-add-v1",
                                "dispatchState": "EXITED", "dispatchReceipt": "dispatch-import-one.json"}}
    return argv, dispatch, receipt


def test_log():
    label = observer.TEST_LABEL
    return ("Test Case '-[" + label + "]' started.\n"
            "Test Case '-[" + label + "]' passed (83.959 seconds).\n"
            "Executed 1 test, with 0 failures (0 unexpected) in 83.959 seconds\n").encode()


def products_fixture(root):
    root.mkdir()
    for name in ("CherriSimulatorQAUITests-Runner.app", "CherriSimulatorQAReceiver.app"):
        put(root / "Debug-iphonesimulator" / name / "image", b"synthetic image")
    target = {
        "BlueprintName": "CherriSimulatorQAUITests",
        "TestHostBundleIdentifier": "org.cherrilang.qa.UITests.xctrunner",
        "IsUITestBundle": True,
        "TestHostPath": "__TESTROOT__/Debug-iphonesimulator/CherriSimulatorQAUITests-Runner.app",
        "TestBundlePath": "__TESTHOST__/PlugIns/CherriSimulatorQAUITests.xctest",
        "UITargetAppPath": "__TESTROOT__/Debug-iphonesimulator/CherriSimulatorQAReceiver.app",
    }
    value = {"__xctestrun_metadata__": {"FormatVersion": 2},
             "TestConfigurations": [{"IsEnabled": True, "TestTargets": [target]}]}
    path = root / "fixture.xctestrun"
    path.write_bytes(plistlib.dumps(value))
    return path, value


def build_fixture(base):
    root, repo = base / "build", base / "repo"
    root.mkdir(); repo.mkdir()
    files, controller = {}, {}
    for index in range(13):
        name = observer.PROJECT + "source-%02d.swift" % index
        data = ("synthetic build source %d\n" % index).encode()
        files[name] = {"bytes": len(data), "sha256": observer.sha(data), "mode": "100644"}
        controller[name] = data
        put(repo / name, data)
    before = raw({"commit": observer.BUILD_SOURCE, "tree": "b" * 40, "files": files})
    put(root / "source-before.original.json", before)
    put(root / "qualification-01/source.after.original.json", before)
    processes = []
    for label in ("xcodegen", "xcodebuild", "codesign-verify", "codesign-display"):
        row = {"label": label, "exitCode": 0, "parentReaped": True}
        for side in ("stdout", "stderr"):
            data = (label + " synthetic output").encode() if side == "stdout" else b""
            put(root / (label + "." + side + ".original"), data)
            row[side + "SHA256"] = observer.sha(data)
            row[side + "Bytes"] = len(data)
        processes.append(row)
    owner = {"builtHarnessCommit": observer.BUILD_SOURCE, "compilerRebuilt": False,
             "nativeInvocation": False, "processes": processes}
    put(root / "result.original.json", raw(owner))
    xct = base / "synthetic.xctestrun"
    xct.write_bytes(b"original test configuration")
    image = {"synthetic": True, "bundleIdentifier": observer.launch.RECEIVER, "images": {"image": "fixed"}}
    products = {"receiverApp": str(base / "synthetic.app"),
                "xctestrun": {"path": str(xct), "sha256": observer.sha(xct.read_bytes())}}
    put(root / "build-products.original.json", raw(products))
    put(root / "receiver-image.original.json", raw(image))
    qualification = {
        "schema": "cherri-standalone-receiver-build-qualification/1", "status": "BUILD_VALIDATED",
        "builtHarnessCommit": observer.BUILD_SOURCE, "sourceTrackedBytesUnchanged": True,
        "sourceCleanAfterRetainedProjectRelocation": True, "compilerRebuilt": False, "nativeInvocation": False,
        "originalOwnerResultSHA256": observer.sha(raw(owner)),
        "sourceBeforeSHA256": observer.sha(before), "sourceAfterSHA256": observer.sha(before),
        "buildProductsSHA256": observer.sha(raw(products)), "receiverImageSHA256": observer.sha(raw(image)),
        "xctestrunSHA256": observer.sha(xct.read_bytes()),
    }
    put(root / "qualification-01/result.original.json", raw(qualification))
    p = plan()
    p["receiverBuildRoot"] = str(root)
    p["receiverBuildQualificationSHA256"] = observer.sha(raw(qualification))
    return p, repo, root, controller, image, owner


class FunctionEnvelopeObserverTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.base = Path(self.temporary.name).resolve(strict=True)

    def tearDown(self):
        self.temporary.cleanup()

    def test_exact_closed_plan_and_source_distinction(self):
        value = plan()
        self.assertIs(observer.validate_plan(value), value)
        self.assertNotEqual(value["controllerCommitSha"], value["builtHarnessCommitSha"])
        self.assertNotEqual(value["compilerCommitSha"], value["builtHarnessCommitSha"])

    def test_plan_faults_fail_before_any_process(self):
        changes = [
            ("schema", "other"), ("compilerCommitSha", "c" * 40), ("builtHarnessCommitSha", "c" * 40),
            ("controllerCommitSha", "short"), ("receiverBuildQualificationSHA256", "c" * 64),
            ("udid", "00000000-0000-0000-0000-000000000000"),
            ("runNonce", "unicode-א"), ("runNonce", ""), ("runId", "x" * 65),
            ("runAttempt", True), ("runAttempt", 0), ("runAttempt", 101),
            ("repo", "../source"), ("root", "/Users/m1/../other"),
            ("root", "/Users/m1/builds/source"), ("root", "/Users/m1/builds/source/nested"),
            ("repo", "/Users/m1/builds"), ("unknown", "input=text"),
        ]
        with mock.patch.object(observer.subprocess, "Popen") as process:
            for key, value in changes:
                with self.subTest(key=key, value=value):
                    p = plan(); p[key] = value
                    with self.assertRaises(ValueError):
                        observer.validate_plan(p)
            process.assert_not_called()

    def test_exact_import_process_and_native_identity(self):
        argv, dispatch, receipt = import_pair()
        observer.validate_import(dispatch, receipt, argv, "import-one")

    def test_import_process_faults_never_become_success(self):
        argv, original, receipt = import_pair()
        for key, value in [
            ("command", argv + ["--input=text"]), ("outcome", "LAUNCHED"), ("outcome", "TIMED_OUT"),
            ("timedOut", True), ("exitCode", 1), ("exitCode", False), ("exitCode", None),
            ("processReturncode", 1), ("processReturncode", False), ("parentReaped", False),
            ("timeoutSeconds", 90),
        ]:
            with self.subTest(key=key, value=value):
                changed = {**original, key: value}
                with self.assertRaises(ValueError):
                    observer.validate_import(changed, receipt, argv, "import-one")

    def test_import_identity_and_exit_anchored_timing_faults(self):
        argv, dispatch, original = import_pair()
        changes = [
            ("id", "another"), ("operation", "runtime"), ("outcome", "IMPORTED_ASSUMED"),
            ("name", "renamed target"), ("identityVerified", False), ("libraryIdentityVerified", False),
            ("importTiming", {}), ("importTiming", {"contract": "old"}),
        ]
        for key, value in changes:
            with self.subTest(key=key, value=value):
                receipt = copy.deepcopy(original); receipt[key] = value
                with self.assertRaises(ValueError):
                    observer.validate_import(dispatch, receipt, argv, "import-one")
        for key, value in [("contract", "old"), ("dispatchState", "PENDING"),
                           ("dispatchReceipt", "dispatch-other.json")]:
            with self.subTest(timing=key):
                receipt = copy.deepcopy(original); receipt["importTiming"][key] = value
                with self.assertRaises(ValueError):
                    observer.validate_import(dispatch, receipt, argv, "import-one")

    def test_exact_xctest_target_terminal_and_count(self):
        observer.validate_xctest(test_log(), 0)

    def test_xctest_terminal_only_and_other_false_passes(self):
        data = test_log()
        lines = data.splitlines(keepends=True)
        variants = {
            "terminal_target_only": lines[0] + lines[1].replace(b"CherriSimulatorQAUITests.", b"OtherTests.") + lines[2],
            "start_target_only": lines[0].replace(b"CherriSimulatorQAUITests.", b"OtherTests.") + lines[1] + lines[2],
            "terminal_duplicate": data + lines[1],
            "start_duplicate": lines[0] + data,
            "reversed": lines[1] + lines[0] + lines[2],
            "failed": data.replace(b"' passed (", b"' failed ("),
            "skipped": data.replace(b"' passed (", b"' skipped ("),
            "bad_total": data.replace(b"with 0 failures", b"with 1 failures"),
            "extra_method": data + b"Test Case '-[OtherTests other]' started.\n",
            "invalid_utf8": data + b"\xff",
        }
        for name, value in variants.items():
            with self.subTest(name=name):
                with self.assertRaises((ValueError, UnicodeDecodeError)):
                    observer.validate_xctest(value, 0)
        for code in (1, None, False):
            with self.subTest(code=code):
                with self.assertRaises(ValueError):
                    observer.validate_xctest(data, code)

    def test_fresh_receiver_requires_actual_stop_result(self):
        self.assertTrue(observer.termination_is_stopped({"result": "PASS", "exitCode": 0}, b""))
        absent = b"An error was encountered (domain=NSPOSIXErrorDomain, code=3):\nNo such process\n"
        self.assertTrue(observer.termination_is_stopped({"result": "EXIT_NONZERO", "exitCode": 3}, absent))
        for observed, stderr in [
            ({"result": "PASS", "exitCode": False}, b""),
            ({"result": "EXIT_NONZERO", "exitCode": 3}, b"No such process"),
            ({"result": "EXIT_NONZERO", "exitCode": 1}, absent),
            ({"result": "TIMEOUT", "exitCode": 3}, absent),
            (None, absent), ({"result": "PASS", "exitCode": 0}, "not original bytes"),
        ]:
            with self.subTest(observed=observed):
                self.assertFalse(observer.termination_is_stopped(observed, stderr))

    def test_originals_are_create_only_and_symlink_safe(self):
        target = self.base / "original.json"
        observer.original(target, b"original")
        with self.assertRaises(FileExistsError):
            observer.original(target, b"replacement")
        self.assertEqual(target.read_bytes(), b"original")
        alias = self.base / "alias"; alias.symlink_to(self.base, target_is_directory=True)
        with self.assertRaises(ValueError):
            observer.original(alias / "new.json", b"not allowed")

    def partial_fixture(self):
        root, docs = self.base / "artifacts", self.base / "Documents"
        root.mkdir(); docs.mkdir()
        contract = b'{"synthetic":"contract"}'
        put(root / observer.launch.RELATIVE / "contract.original.json", contract)
        directory = docs / "primary-no-input-launch" / observer.sha(contract)
        for name, data in [("owner.original.json", b"owner original"),
                           ("contract.original.json", contract), ("001-intent.original.json", b"intent original")]:
            put(directory / name, data)
        put(docs / "callback-events.json", b"[]")
        return root, docs, directory

    def test_failed_launch_partial_originals_survive_without_completion(self):
        root, docs, directory = self.partial_fixture()
        destination = self.base / "retained"
        value = observer.retain_failure_receiver(root, docs, destination)
        self.assertEqual(len(value["files"]), 4)
        for p in directory.iterdir():
            self.assertEqual((destination / p.name).read_bytes(), p.read_bytes())
        self.assertFalse(value["successCollector"])
        self.assertFalse(value["nativeExecutionAuthenticated"])
        self.assertFalse(value["formalAcceptanceGranted"])
        self.assertFalse((destination / "002-completion.original.json").exists())

    def test_failed_retention_missing_originals_are_explicit(self):
        root, docs = self.base / "root", self.base / "docs"
        root.mkdir(); docs.mkdir()
        value = observer.retain_failure_receiver(root, docs, self.base / "retained")
        self.assertEqual(value["missing"], ["prepared-contract", "callback-events"])
        self.assertEqual(value["files"], [])

    def test_failed_retention_refuses_symlink_and_unknown_file(self):
        for name in ("symlink", "unknown"):
            with self.subTest(name=name):
                case = self.base / name; case.mkdir()
                root, docs = case / "root", case / "docs"; root.mkdir(); docs.mkdir()
                contract = b"original"
                put(root / observer.launch.RELATIVE / "contract.original.json", contract)
                directory = docs / "primary-no-input-launch" / observer.sha(contract); directory.mkdir(parents=True)
                if name == "symlink":
                    (directory / "owner.original.json").symlink_to(root / observer.launch.RELATIVE / "contract.original.json")
                else:
                    put(directory / "unrelated.txt", b"private")
                with self.assertRaises(ValueError):
                    observer.retain_failure_receiver(root, docs, case / "retained")

    def test_test_products_are_bound_to_both_actual_bundles(self):
        path, _ = products_fixture(self.base / "products")
        before = observer.test_products_inventory(path)
        self.assertEqual(len(before["files"]), 2)
        image = path.parent / "Debug-iphonesimulator/CherriSimulatorQAUITests-Runner.app/image"
        image.write_bytes(b"changed image")
        self.assertNotEqual(observer.test_products_inventory(path), before)

    def test_test_product_path_escape_and_target_mutations(self):
        path, original = products_fixture(self.base / "products")
        changes = [("TestHostPath", "/foreign/Runner.app"), ("BlueprintName", "AnotherTests"),
                   ("TestHostBundleIdentifier", "foreign"), ("IsUITestBundle", False),
                   ("UITargetAppPath", "/foreign/Receiver.app"), ("TestBundlePath", "../escape")]
        for key, value in changes:
            with self.subTest(key=key):
                config = copy.deepcopy(original); config["TestConfigurations"][0]["TestTargets"][0][key] = value
                path.write_bytes(plistlib.dumps(config))
                with self.assertRaises(ValueError):
                    observer.test_products_inventory(path)
        path.write_bytes(plistlib.dumps(original))
        foreign = self.base / "foreign"; foreign.write_bytes(b"foreign")
        (path.parent / "Debug-iphonesimulator/CherriSimulatorQAUITests-Runner.app/link").symlink_to(foreign)
        with self.assertRaisesRegex(ValueError, "SYMLINK_ESCAPE"):
            observer.test_products_inventory(path)

    def test_original_build_receipt_and_controller_source_equivalence(self):
        p, repo, root, controller, image, _ = build_fixture(self.base)
        with mock.patch.object(observer, "git", side_effect=lambda repo, command, spec: controller[spec.split(":", 1)[1]]), \
             mock.patch.object(observer.differential, "receiver_image_inventory", return_value=image):
            value = observer.verify_build(p, repo)
        self.assertEqual(value["builtHarnessCommitSha"], observer.BUILD_SOURCE)
        self.assertEqual(len(value["projectSources"]), 13)

    def test_build_process_stream_and_source_causal_corruptions(self):
        for change in ("stream", "empty_stream", "source", "controller", "image", "xctestrun", "owner"):
            with self.subTest(change=change):
                base = self.base / change; base.mkdir()
                p, repo, root, controller, image, owner = build_fixture(base)
                returned_image = image
                if change == "stream": (root / "xcodebuild.stdout.original").write_bytes(b"changed")
                if change == "empty_stream": (root / "codesign-verify.stderr.original").write_bytes(b"nonempty")
                if change == "source": (repo / next(iter(controller))).write_bytes(b"changed")
                if change == "controller": controller[next(iter(controller))] = b"new source"
                if change == "image": returned_image = {"synthetic": "different"}
                if change == "xctestrun": (base / "synthetic.xctestrun").write_bytes(b"changed")
                if change == "owner": (root / "result.original.json").write_bytes(raw({**owner, "nativeInvocation": True}))
                with mock.patch.object(observer, "git", side_effect=lambda repo, command, spec: controller[spec.split(":", 1)[1]]), \
                     mock.patch.object(observer.differential, "receiver_image_inventory", return_value=returned_image):
                    with self.assertRaises(ValueError):
                        observer.verify_build(p, repo)

    def signing_fixture(self, base):
        root = base / "artifacts"; root.mkdir()
        directory = root / "runtime/signing-originals"; directory.mkdir(parents=True)
        signed = b"AEA1synthetic signed test data"
        put(root / "builds/signed" / (observer.launch.WORKFLOW + ".shortcut"), signed)
        target = {"synthetic": "original target"}
        put(directory / "signing-provenance-target.original.json", raw(target))
        put(root / "runtime/signing-provenance.json", raw({"targetProvenance": target}))
        result = {"status": "SIGNED_ORIGINAL", "compilerCommitSha": observer.runtime.COMPILER_SOURCE,
                  "compilePerformed": False, "processExitCode": 0, "parentReaped": True,
                  "signedSHA256": observer.sha(signed), "sourceUnchanged": True,
                  "signedBytes": len(signed), "targetProvenanceSHA256": observer.sha(raw(target)),
                  "signerCompilerSource": "627c3e7eb4a54a0364b5a5563db89291fa953b97"}
        for name, data in (("stdout.original", b"synthetic signing stdout"), ("stderr.original", b"")):
            put(directory / name, data)
            result[name] = {"bytes": len(data), "sha256": observer.sha(data)}
        put(directory / "result.original.json", raw(result))
        return root, directory, result, signed

    def test_actual_signing_receipt_binds_original_bytes_and_distinct_compiler(self):
        root, directory, result, signed = self.signing_fixture(self.base)
        with mock.patch.object(observer, "SIGNING_RECEIPT", observer.sha(raw(result))), \
             mock.patch.object(observer, "SIGNED_SHA256", observer.sha(signed)):
            binding = observer.verify_signing(root)
        self.assertEqual(binding["compilerCommitSha"], observer.runtime.COMPILER_SOURCE)
        self.assertNotEqual(binding["compilerCommitSha"], binding["signerCompilerSource"])
        self.assertFalse(binding["compilePerformed"])

    def test_signing_process_stream_target_and_summary_corruptions(self):
        for change in ("compiler", "compile", "exit", "bool_exit", "unreaped", "source_changed",
                       "signed", "stream", "target", "summary", "receipt_hash"):
            with self.subTest(change=change):
                base = self.base / change; base.mkdir()
                root, directory, result, signed = self.signing_fixture(base)
                if change == "compiler": result["compilerCommitSha"] = "c" * 40
                if change == "compile": result["compilePerformed"] = True
                if change == "exit": result["processExitCode"] = 1
                if change == "bool_exit": result["processExitCode"] = False
                if change == "unreaped": result["parentReaped"] = False
                if change == "source_changed": result["sourceUnchanged"] = False
                if change == "signed":
                    (root / "builds/signed" / (observer.launch.WORKFLOW + ".shortcut")).write_bytes(b"AEA1changed")
                if change == "stream": (directory / "stderr.original").write_bytes(b"changed")
                if change == "target": (directory / "signing-provenance-target.original.json").write_bytes(b"{}")
                if change == "summary": (root / "runtime/signing-provenance.json").write_bytes(b'{"targetProvenance":{}}')
                put(directory / "result.original.json", raw(result))
                digest = "f" * 64 if change == "receipt_hash" else observer.sha(raw(result))
                with mock.patch.object(observer, "SIGNING_RECEIPT", digest), \
                     mock.patch.object(observer, "SIGNED_SHA256", observer.sha(signed)):
                    with self.assertRaises(ValueError):
                        observer.verify_signing(root)

    def test_full_source_inventory_checks_transitive_bytes(self):
        repo = self.base / "source"; repo.mkdir()
        paths = {
            "tools/shortcuts-simulator-qa/function_envelope_observer.py": b"outer",
            "tools/shortcuts-simulator-qa/nested_list_runtime.py": b"transitive original",
            "tools/shortcuts-simulator-qa/run_differential_batch.py": b"import observer",
        }
        tree = b""
        for name, data in paths.items():
            put(repo / name, data)
            blob = hashlib.sha1(b"blob " + str(len(data)).encode() + b"\0" + data).hexdigest()
            tree += b"100644 blob " + blob.encode() + b"\t" + name.encode() + b"\0"
        critical = {"commit": "a" * 40, "tree": "b" * 40}
        with mock.patch.object(observer.launch, "source_snapshot", return_value=critical), \
             mock.patch.object(observer, "git", return_value=tree):
            actual = observer.source_inventory(repo, "a" * 40)
            self.assertEqual(set(actual["files"]), set(paths))
            put(repo / "tools/shortcuts-simulator-qa/nested_list_runtime.py", b"tampered transitive")
            with self.assertRaisesRegex(ValueError, "SOURCE_BLOB"):
                observer.source_inventory(repo, "a" * 40)

    def test_observer_result_has_its_own_schema(self):
        repo, root, build = self.base/"repo", self.base/"root", self.base/"build"
        repo.mkdir(); root.mkdir(); build.mkdir(); (root/"runtime").mkdir()
        value = plan(); value.update(repo=str(repo), root=str(root), receiverBuildRoot=str(build))
        with mock.patch.object(observer, "validate_plan", return_value=value), \
             mock.patch.object(observer.sys, "platform", "darwin"), \
             mock.patch.object(observer.shutil, "disk_usage", return_value=SimpleNamespace(free=8*1024**3)), \
             mock.patch.object(observer.subprocess, "Popen") as process:
            obj = observer.NativeObservation(value)
        self.assertEqual(obj.summary["schema"], "cherri-function-envelope-native-observer/1")
        self.assertEqual(json.loads((obj.folder/"plan.original.json").read_bytes())["schema"],
                         "cherri-function-envelope-native-observer-plan/1")
        process.assert_not_called()

    def test_cleanup_failure_still_retains_after_source_and_products(self):
        obj = observer.NativeObservation.__new__(observer.NativeObservation)
        obj.plan = plan(); obj.repo = self.base; obj.root = self.base; obj.folder = self.base
        obj.before = None; obj.build = None; obj.product_inventory = None
        obj.capacity_stop = threading.Event(); obj.capacity_thread = None
        obj.minimum_free = 5 * 1024 ** 3
        obj.xctest = SimpleNamespace(returncode=0)
        obj.summary = {"status": "FAIL", "launchOwnerStarted": False, "appleExecutionAuthenticated": False,
                       "formalAcceptanceGranted": False}
        for name in ("check_capacity", "start_capacity_watch", "select_device", "start_observer",
                     "import_original", "one_runtime", "preserve_partial_receiver"):
            setattr(obj, name, mock.Mock())
        obj.finish = mock.Mock(side_effect=RuntimeError("owned cleanup failed"))
        snap = {"synthetic": "source"}; products = {"synthetic": "products"}
        with mock.patch.object(observer, "source_inventory", return_value=snap) as source, \
             mock.patch.object(observer, "verify_build", return_value={"xctestrun": "synthetic"}), \
             mock.patch.object(observer, "test_products_inventory", return_value=products) as product, \
             mock.patch.object(observer.runtime, "provenance", return_value={}), \
             mock.patch.object(observer, "verify_signing", return_value={"synthetic": "signing"}), \
             mock.patch.object(observer, "validate_xctest"), \
             mock.patch.object(observer.signal, "alarm"), mock.patch.object(observer.signal, "signal"):
            result = obj.run()
        self.assertEqual(result["status"], "FAIL")
        self.assertFalse(result["appleExecutionAuthenticated"])
        self.assertEqual(source.call_count, 2)
        self.assertEqual(product.call_count, 2)
        self.assertTrue((self.base/"source.after.original.json").is_file())
        self.assertTrue((self.base/"xctest-products.after.original.json").is_file())

    def test_no_native_steps_if_source_or_signing_admission_fails(self):
        for fail in ("source", "signing"):
            with self.subTest(fail=fail):
                base = self.base / fail; base.mkdir()
                obj = observer.NativeObservation.__new__(observer.NativeObservation)
                obj.plan = plan(); obj.repo = base; obj.root = base; obj.folder = base
                obj.before = None; obj.build = None; obj.product_inventory = None
                obj.capacity_stop = threading.Event(); obj.capacity_thread = None
                obj.minimum_free = 5 * 1024 ** 3
                obj.summary = {"status": "FAIL", "launchOwnerStarted": False, "appleExecutionAuthenticated": False,
                               "formalAcceptanceGranted": False}
                for name in ("check_capacity", "start_capacity_watch", "select_device", "start_observer",
                             "import_original", "one_runtime", "finish", "preserve_partial_receiver"):
                    setattr(obj, name, mock.Mock())
                snap = {"synthetic": "source"}
                with mock.patch.object(observer, "source_inventory",
                        side_effect=ValueError("source failed") if fail == "source" else [snap, snap]), \
                     mock.patch.object(observer, "verify_build", return_value={"xctestrun": "synthetic"}), \
                     mock.patch.object(observer, "test_products_inventory", return_value={"synthetic": "products"}), \
                     mock.patch.object(observer.runtime, "provenance", return_value={}), \
                     mock.patch.object(observer, "verify_signing", side_effect=ValueError("signing failed")), \
                     mock.patch.object(observer.signal, "alarm"):
                    result = obj.run()
                self.assertEqual(result["status"], "FAIL")
                self.assertFalse(result["appleExecutionAuthenticated"])
                self.assertFalse(result["formalAcceptanceGranted"])
                obj.select_device.assert_not_called()
                obj.start_observer.assert_not_called()
                obj.import_original.assert_not_called()
                obj.one_runtime.assert_not_called()
                self.assertTrue((base / "result.original.json").is_file())


if __name__ == "__main__":
    unittest.main()
