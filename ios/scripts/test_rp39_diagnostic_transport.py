#!/usr/bin/env python3
"""Synthetic transport controls: real Git/process/crypto, no Apple or native run."""
import contextlib
import importlib.util
import io
import json
import os
from pathlib import Path
import shutil
import signal
import subprocess
import sys
import tempfile
import time
import unittest
from unittest import mock
import zipfile


HERE = Path(__file__).resolve().parent
REPO = HERE.parents[1]
CONTROL_ORIGINALS = {}


def load(name):
    spec = importlib.util.spec_from_file_location("_test_rp39_" + name, HERE / (name + ".py"))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


base = load("collect_ios_xcresult_summary")
profile = load("record_workspace_presentation_source")
seal = load("seal_rp39_diagnostic")


def git(repo, *args):
    value = subprocess.run(["/usr/bin/git", "-C", str(repo), *args],
                           capture_output=True, timeout=10, check=True)
    return value.stdout.decode().strip()


class RP39DiagnosticTransportTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="rp39-synthetic-transport-")
        self.addCleanup(self.temp.cleanup)
        self.path = Path(self.temp.name).resolve()
        self.repo = self.path / "repo"
        self.repo.mkdir()
        for name in profile.RP39_SOURCE_FILES:
            target = self.repo / name
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(REPO / name, target)
        git(self.repo, "init", "-q")
        git(self.repo, "config", "user.name", "Synthetic transport control")
        git(self.repo, "config", "user.email", "synthetic@example.invalid")
        git(self.repo, "add", ".")
        git(self.repo, "commit", "-qm", "Synthetic source fixture")
        self.source = git(self.repo, "rev-parse", "HEAD")
        self.context = {"repository": base.REPOSITORY, "workflowPath": base.WORKFLOW,
                        "workflowName": base.WORKFLOW_NAME, "job": "ios-build",
                        "event": "workflow_dispatch", "sourceSha": self.source,
                        "runId": 12345, "attempt": 1}
        self.env = {"GITHUB_REPOSITORY": base.REPOSITORY, "GITHUB_WORKFLOW": base.WORKFLOW_NAME,
                    "GITHUB_EVENT_NAME": "workflow_dispatch", "GITHUB_SHA": self.source,
                    "GITHUB_WORKFLOW_SHA": self.source, "GITHUB_RUN_ID": "12345",
                    "GITHUB_RUN_ATTEMPT": "1"}
        base._owned_git, base._first_interruption = None, None
        self.addCleanup(setattr, base, "_owned_git", None)
        self.addCleanup(setattr, base, "_first_interruption", None)

    def originals(self, directory):
        CONTROL_ORIGINALS[self._testMethodName] = {
            str(path.relative_to(directory)): path.read_text()
            for path in sorted(directory.rglob("*")) if path.is_file()}

    def profile_records(self):
        with mock.patch.dict(os.environ, self.env), contextlib.redirect_stdout(io.StringIO()):
            context = profile.context_from_environment(os.environ)
            profile.record(self.repo, "before", context, rp39=True)
            profile.record(self.repo, "after", context, rp39=True)

    def selected_developer_directory(self, synthetic):
        if sys.platform != "darwin":
            return synthetic
        result = subprocess.run(["/usr/bin/xcode-select", "--print-path"],
                                capture_output=True, timeout=10, check=True)
        selected = result.stdout.decode("utf-8").strip()
        self.assertTrue(Path(selected).is_absolute() and Path(selected).is_dir())
        return selected

    def test_selected_developer_directory_reaches_original_git(self):
        actual_run = subprocess.run
        expected = self.selected_developer_directory(str(self.path / "SyntheticXcode/Contents/Developer"))
        def checked(*args, **kwargs):
            self.assertEqual(kwargs["env"].get("DEVELOPER_DIR"), expected)
            self.assertNotIn("PRIVATE_ENVIRONMENT_CANARY", kwargs["env"])
            return actual_run(*args, **kwargs)
        with mock.patch.dict(os.environ, {"DEVELOPER_DIR": expected, "PRIVATE_ENVIRONMENT_CANARY": "hidden"}), \
                mock.patch.object(base.subprocess, "run", side_effect=checked):
            raw = base.local_git(self.repo, ["rev-parse", "--verify", "HEAD"], time.monotonic() + 15)
        self.assertEqual(raw, (self.source + "\n").encode())

    def test_owned_git_retains_actual_child_streams_and_cleanup(self):
        output = self.path / "owned"
        expected = self.selected_developer_directory("/synthetic/SelectedXcode")
        with base.envelope(15):
            base._owned_git = base.OwnedGitReads(self.repo, output)
            with mock.patch.dict(os.environ, {"DEVELOPER_DIR": expected}):
                raw = base.local_git(self.repo, ["rev-parse", "--verify", "HEAD"], time.monotonic() + 15)
        self.originals(output)
        self.assertEqual(raw, (self.source + "\n").encode())
        row = base._owned_git.records[0]
        self.assertIn("DEVELOPER_DIR=" + expected, row["argv"])
        self.assertEqual(row["observed"]["result"], "PASS")
        self.assertTrue(row["capture"]["cleanupParentReaped"])
        self.assertEqual(row["capture"]["groupAfterCleanup"]["state"], "ABSENT")
        self.assertEqual((output / "command-01/stdout.original").read_bytes(), raw)
        self.assertTrue((output / "command-01/started.original.json").is_file())
        self.assertTrue((output / "command-01/finished.original.json").is_file())

    def test_missing_darwin_selection_binds_observed_directory_before_real_git(self):
        actual_capture = getattr(base.OwnedGitReads, "capture_command", None)
        actual_platform = sys.platform
        output = self.path / "selected-directory"
        requested = []
        synthetic_directory = self.path / "SyntheticSelectedXcode/Contents/Developer"
        synthetic_directory.mkdir(parents=True)
        expected_selection = self.selected_developer_directory(str(synthetic_directory))
        def capture(instance, operation, argv, deadline):
            requested.append((operation, argv))
            if operation == "selected_developer_directory":
                self.assertEqual(argv, ["/usr/bin/xcode-select", "--print-path"])
                if actual_platform != "darwin":
                    # Linux injects only the unavailable platform selector.
                    # Both following Git reads and all cleanup remain real.
                    argv = [sys.executable, "-c", "import sys;sys.stdout.write(sys.argv[1]+'\\n')",
                            str(synthetic_directory)]
            return actual_capture(instance, operation, argv, deadline)
        env = dict(os.environ, PRIVATE_ENVIRONMENT_CANARY="must-not-reach-git")
        env.pop("DEVELOPER_DIR", None)
        with mock.patch.dict(os.environ, env, clear=True), \
                mock.patch.object(base.sys, "platform", "darwin"), \
                mock.patch.object(base.OwnedGitReads, "capture_command", capture, create=True), base.envelope(15):
            base._owned_git = base.OwnedGitReads(self.repo, output)
            deadline = time.monotonic() + 15
            for _ in range(2):
                raw = base.local_git(self.repo, ["rev-parse", "--verify", "HEAD"], deadline)
                self.assertEqual(raw, (self.source + "\n").encode())
        self.originals(output)
        rows = base._owned_git.records
        git_rows = [r for r in rows if "/usr/bin/git" in r["argv"]]
        self.assertEqual(len(git_rows), 2)
        for row in git_rows:
            self.assertIn("DEVELOPER_DIR=" + expected_selection, row["argv"])
            self.assertFalse(any(x.startswith("PRIVATE_ENVIRONMENT_CANARY=") for x in row["argv"]))
        self.assertEqual([r["operation"] for r in rows], ["selected_developer_directory", "git", "git"])
        self.assertEqual([op for op, _ in requested], ["selected_developer_directory", "git", "git"])
        selection = (output / "command-01/stdout.original").read_text().removesuffix("\n")
        self.assertTrue(Path(selection).is_absolute() and Path(selection).is_dir())
        for row in rows:
            self.assertEqual(row["observed"]["result"], "PASS")
            self.assertTrue(row["capture"]["cleanupParentReaped"])
            self.assertEqual(row["capture"]["groupAfterCleanup"]["state"], "ABSENT")
        self.assertEqual(selection, expected_selection)

    def test_failed_selected_directory_cannot_start_git(self):
        actual_capture = base.OwnedGitReads.capture_command
        output = self.path / "failed-selection"
        def capture(instance, operation, argv, deadline):
            self.assertEqual(operation, "selected_developer_directory")
            self.assertEqual(argv, ["/usr/bin/xcode-select", "--print-path"])
            return actual_capture(instance, operation, [sys.executable, "-c", "raise SystemExit(17)"], deadline)
        env = dict(os.environ)
        env.pop("DEVELOPER_DIR", None)
        with mock.patch.dict(os.environ, env, clear=True), \
                mock.patch.object(base.sys, "platform", "darwin"), \
                mock.patch.object(base.OwnedGitReads, "capture_command", capture), base.envelope(15):
            base._owned_git = base.OwnedGitReads(self.repo, output)
            with self.assertRaisesRegex(base.Rejected, "SOURCE_CHANGED"):
                base.local_git(self.repo, ["rev-parse", "--verify", "HEAD"], time.monotonic() + 15)
        self.originals(output)
        self.assertEqual(len(base._owned_git.records), 1)
        row = base._owned_git.records[0]
        self.assertEqual(row["operation"], "selected_developer_directory")
        self.assertEqual(row["observed"]["result"], "EXIT_NONZERO")
        self.assertEqual(row["capture"]["returnCodeObserved"], 17)
        self.assertTrue(row["capture"]["cleanupParentReaped"])
        self.assertEqual(row["capture"]["groupAfterCleanup"]["state"], "ABSENT")

    def test_invalid_selected_directory_cannot_start_git(self):
        actual_capture = base.OwnedGitReads.capture_command
        env = dict(os.environ)
        env.pop("DEVELOPER_DIR", None)
        for index, invalid in enumerate((b"", b"relative/path\n", b"/missing/SelectedXcode\n", b"/bad\x00path\n", b"\xff\n")):
            with self.subTest(output=index):
                output = self.path / ("invalid-selection-%d" % index)
                def capture(instance, operation, argv, deadline):
                    self.assertEqual(operation, "selected_developer_directory")
                    self.assertEqual(argv, ["/usr/bin/xcode-select", "--print-path"])
                    return actual_capture(instance, operation,
                        [sys.executable, "-c", "import sys;sys.stdout.buffer.write(bytes.fromhex(sys.argv[1]))", invalid.hex()], deadline)
                with mock.patch.dict(os.environ, env, clear=True), \
                        mock.patch.object(base.sys, "platform", "darwin"), \
                        mock.patch.object(base.OwnedGitReads, "capture_command", capture), base.envelope(15):
                    base._owned_git = base.OwnedGitReads(self.repo, output)
                    with self.assertRaisesRegex(base.Rejected, "SOURCE_CHANGED"):
                        base.local_git(self.repo, ["rev-parse", "--verify", "HEAD"], time.monotonic() + 15)
                self.assertEqual(len(base._owned_git.records), 1)
                self.assertEqual(base._owned_git.records[0]["operation"], "selected_developer_directory")
                self.assertEqual(base._owned_git.records[0]["observed"]["result"], "PASS")
                self.assertTrue(base._owned_git.records[0]["capture"]["cleanupParentReaped"])
                self.assertEqual(base._owned_git.records[0]["capture"]["groupAfterCleanup"]["state"], "ABSENT")
                # Malformed UTF-8 is intentional input; retain exact bytes.
                CONTROL_ORIGINALS[self._testMethodName + "/" + output.name] = {
                    str(path.relative_to(output)): {"encoding": "hex", "bytes": path.read_bytes().hex()}
                    for path in sorted(output.rglob("*")) if path.is_file()}

    def test_changed_bootstrap_is_rejected_before_any_child(self):
        path = self.repo / base.HELPER_FILES[-1]
        path.write_bytes(path.read_bytes() + b"\n# synthetic mutation\n")
        with mock.patch.object(subprocess, "Popen") as launch:
            with self.assertRaisesRegex(ValueError, "SOURCE_CHANGED"):
                base.OwnedGitReads(self.repo, self.path / "not-created")
        launch.assert_not_called()
        self.assertFalse((self.path / "not-created").exists())

    def test_second_real_signal_cannot_interrupt_owned_cleanup(self):
        output = self.path / "cancelled"
        owned = base.OwnedGitReads(self.repo, output)
        original_kill = owned.helper.kill_group
        seen = {"pid": None, "signals": []}
        base.write_json(output / "intent.original.json", {"argv": ["/bin/sleep", "5"],
                        "scope": "INJECTED_TWO_SIGNAL_CLEANUP_CONTROL"})
        def journal(phase, detail):
            base.write_json(output / (phase + ".original.json"), {"capture": detail})
            if phase == "started":
                seen["pid"] = detail["pid"]
                seen["signals"].append("first")
                os.kill(os.getpid(), signal.SIGALRM)
        def kill(process):
            seen["signals"].append("second")
            os.kill(os.getpid(), signal.SIGALRM)
            return original_kill(process)
        try:
            with mock.patch.object(owned.helper, "kill_group", side_effect=kill):
                with self.assertRaises(base.EnvelopeExpired):
                    with base.envelope(15):
                        with (output / "stdout.original").open("xb") as stdout, \
                                (output / "stderr.original").open("xb") as stderr:
                            owned.helper.capture(["/bin/sleep", "5"], 13, output=stdout,
                                error_output=stderr, hard_deadline=time.monotonic() + 15,
                                stream_limit=32768, journal=journal)
            self.originals(output)
            self.assertEqual(seen["signals"], ["first", "second"])
            finished = json.loads((output / "finished.original.json").read_text())
            self.assertTrue(finished["capture"]["cleanupParentReaped"])
            self.assertEqual(finished["capture"]["groupAfterCleanup"]["state"], "ABSENT")
            self.assertEqual(base._first_interruption, signal.SIGALRM)
        finally:
            # A counterfactual unlatched implementation must not leak the child.
            pid = seen["pid"]
            if pid is not None:
                try:
                    os.killpg(pid, signal.SIGKILL)
                except ProcessLookupError:
                    pass
                try:
                    os.waitpid(pid, 0)
                except ChildProcessError:
                    pass
            self.originals(output)

    def test_fixed_rp39_mode_source_profile_and_original_modes(self):
        profile.validate_mode("false", "false", "false", "false", "true")
        for index in range(4):
            flags = ["false"] * 4
            flags[index] = "true"
            with self.subTest(conflict=index), self.assertRaisesRegex(ValueError, "COMPONENT_MODE_CONFLICT"):
                profile.validate_mode(*flags, rp39="true")
        with self.assertRaisesRegex(ValueError, "COMPONENT_MODE_VALUE"):
            profile.validate_mode("false", "false", "false", "false", "TRUE")
        workflow = (REPO / base.WORKFLOW).read_text()
        selection = workflow.split("          STARTUP_SELECTION=()\n", 1)[1].split(
            "\n          defaults write com.apple.iphonesimulator", 1)[0]
        for rp39, joined, expected in (("true", "false", profile.RP39_METHODS),
                                       ("false", "true", profile.JOINED_METHODS),
                                       ("false", "false", ())):
            env = dict(os.environ, RP39_DIAGNOSTIC_ONLY=rp39, JOINED_EDITOR_COMPONENT_ONLY=joined,
                       STARTUP_OBSERVATION_ONLY="false", WORKSPACE_PRESENTATION_ONLY="false")
            result = subprocess.run(["/bin/bash", "-c", "STARTUP_SELECTION=()\n" + selection +
                "\nprintf '%s\\0' \"${STARTUP_SELECTION[@]}\"\n"], env=env, capture_output=True, timeout=5, check=True)
            self.assertEqual([v.decode() for v in result.stdout.split(b"\0") if v],
                             ["-only-testing:" + v for v in expected])
        self.profile_records()
        with mock.patch.dict(os.environ, self.env):
            seal.source_receipts(self.repo, self.context)
            changed = self.repo / "ios/CherriApp/Diagnostics/EditorDeliveryObservation.swift"
            changed.write_bytes(changed.read_bytes() + b"\n")
            with self.assertRaisesRegex(ValueError, "SOURCE_CHANGED"):
                seal.source_receipts(self.repo, self.context)
        self.assertEqual(workflow.count("timeout-minutes: 75"), 1)
        self.assertEqual(base.BUDGETS["preflightMs"], 15000)
        self.assertEqual(base.BUDGETS["postTestMs"], 60000)

    def test_real_encryption_retains_complete_synthetic_bundle_bytes(self):
        root = self.path / base.root_name(self.context)
        reservation = base.prepare(self.repo, self.context, root, deadline=time.monotonic() + 15)
        bundle = root / "tests.xcresult"
        (bundle / "Data/empty").mkdir(parents=True)
        (bundle / "Info.plist").write_bytes(b"SYNTHETIC_INFO_ONLY\n")
        canary = b"PRIVATE_RP39_SYNTHETIC_CARET_ATTACHMENT\n"
        (bundle / "Data/attachment").write_bytes(canary)
        (self.repo / "ios/xcodetest.log").write_bytes(canary)
        (self.repo / "ios/xcodetest.exitcode").write_bytes(b"65\n")
        fake_bin = self.path / "bin"
        fake_bin.mkdir()
        fake = fake_bin / "xcrun"
        fake.write_text("#!" + sys.executable + "\nimport sys\nassert sys.argv[1:5] == ['xcresulttool','get','test-results','summary']\nprint('{\"synthetic\":true}')\n")
        fake.chmod(0o700)
        with mock.patch.dict(os.environ, {"PATH": str(fake_bin) + os.pathsep + os.environ["PATH"]}):
            status, _ = base.collect(self.repo, self.context, root, reservation)
        self.assertEqual(status, 0)
        self.profile_records()
        output = self.path / "sealed"
        with mock.patch.dict(os.environ, self.env), seal.base.envelope(300):
            receipt = seal.seal(self.repo, self.context, root, output, started=time.monotonic())
        CONTROL_ORIGINALS[self._testMethodName] = {
            "receipt.original.json": (output / "public/receipt.json").read_text(),
            "encryption.original.json": (output / "private/encryption.original.json").read_text()}
        self.assertTrue(receipt["finishedXCResultPresent"])
        self.assertFalse(receipt["formalAcceptanceGranted"])
        self.assertEqual(receipt["nativeExitCode"], 65)
        self.assertNotIn(canary.strip(), (output / "public/receipt.json").read_bytes())
        self.assertEqual({p.name for p in (output / "public").iterdir()}, seal.PUBLIC)
        self.assertTrue(receipt["encryptionProcess"]["capture"]["cleanupParentReaped"])
        self.assertEqual(receipt["encryptionProcess"]["capture"]["groupAfterCleanup"]["state"], "ABSENT")
        with zipfile.ZipFile(output / "private/rp39-originals.zip") as archive:
            self.assertEqual(archive.read("tests.xcresult/Data/attachment"), canary)
            self.assertEqual(archive.read("xcodetest.log"), canary)
            manifest = json.loads(archive.read("manifest.original.json"))
            self.assertIn("tests.xcresult/Data/empty", manifest["identity"]["directories"])
        self.assertEqual((bundle / "Data/attachment").read_bytes(), canary)

    def test_inventory_rejects_aliases_and_bounded_overflow(self):
        self.profile_records()
        with mock.patch.dict(os.environ, self.env):
            receipts = seal.source_receipts(self.repo, self.context)
        root = self.path / "result-root"
        bundle = root / "tests.xcresult"
        bundle.mkdir(parents=True)
        target = self.path / "outside"
        target.write_bytes(b"SYNTHETIC_PRIVATE_CANARY")
        (bundle / "alias").symlink_to(target)
        with self.assertRaises(ValueError):
            seal.member_inventory(self.repo, root, receipts)
        (bundle / "alias").unlink()
        (bundle / "large").write_bytes(b"X" * 100)
        with mock.patch.object(seal, "LOGICAL_LIMIT", 1), self.assertRaises(ValueError):
            seal.member_inventory(self.repo, root, receipts)

    def test_archive_rejects_changed_original_and_live_guard(self):
        source = self.path / "original"
        source.write_bytes(b"original")
        members = {"one.original": {"path": source, "original": seal.file_record(source)}}
        source.write_bytes(b"replaced")
        with self.assertRaises(ValueError):
            seal.make_archive(self.path / "rejected.zip", members, {"synthetic": True})
        source.write_bytes(b"original")
        members["one.original"]["original"] = seal.file_record(source)
        def denied():
            raise ValueError("SYNTHETIC_LIVE_CAPACITY_FLOOR")
        with self.assertRaisesRegex(ValueError, "SYNTHETIC_LIVE_CAPACITY_FLOOR"):
            seal.make_archive(self.path / "capacity.zip", members, {"synthetic": True}, guard=denied)


    def test_shared_archive_schema_and_create_only_encryption(self):
        source = self.path / "one.original"
        source.write_bytes(b"SYNTHETIC_PRIVATE_ORIGINAL\n")
        members = {"one.original": {"path": source, "original": seal.file_record(source)}}
        identity = {"synthetic": True, "formalAcceptanceGranted": False}
        generic = self.path / "generic.zip"
        generic_record, generic_manifest = seal.make_archive(generic, members, identity,
            manifest_schema="cherri-native-original-diagnostics/1")
        _, default_manifest = seal.make_archive(self.path / "default.zip", members, identity)
        self.assertEqual(generic_manifest["schema"], "cherri-native-original-diagnostics/1")
        self.assertEqual(default_manifest["schema"], seal.SCHEMA)
        self.assertEqual(generic_manifest["members"], default_manifest["members"])
        with self.assertRaisesRegex(ValueError, "SCHEMA_REJECTED"):
            seal.make_archive(self.path / "bad.zip", members, identity, manifest_schema="unreviewed")
        private, stage = self.path / "crypto", self.path / "public-stage"
        private.mkdir(); stage.mkdir()
        helper = seal.base.load_primitives(self.repo, None)
        record, process = seal.encrypt_archive(private, stage, generic, generic_record,
            self.repo / base.RECIPIENT, helper, time.monotonic() + 60)
        CONTROL_ORIGINALS[self._testMethodName] = {
            "encryption.original.json": (private / "encryption.original.json").read_text()}
        self.assertEqual(process["observed"]["result"], "PASS")
        self.assertTrue(process["capture"]["cleanupParentReaped"])
        self.assertEqual(process["capture"]["groupAfterCleanup"]["state"], "ABSENT")
        cipher = stage / "rp39-originals.zip.p7m"
        self.assertEqual(seal.file_record(cipher), record)
        with mock.patch.object(helper, "capture") as launch:
            with self.assertRaisesRegex(ValueError, "ENCRYPTION_REJECTED"):
                seal.encrypt_archive(private, stage, generic, generic_record,
                    self.repo / base.RECIPIENT, helper, time.monotonic() + 60)
            launch.assert_not_called()
        self.assertEqual(seal.file_record(cipher), record)
        second_stage = self.path / "second-stage"
        second_stage.mkdir()
        generic.write_bytes(generic.read_bytes() + b"mutation")
        with mock.patch.object(helper, "capture") as launch:
            with self.assertRaisesRegex(ValueError, "ENCRYPTION_REJECTED"):
                seal.encrypt_archive(private, second_stage, generic, generic_record,
                    self.repo / base.RECIPIENT, helper, time.monotonic() + 60)
            launch.assert_not_called()
        self.assertEqual(list(second_stage.iterdir()), [])


if __name__ == "__main__":
    unittest.main(verbosity=2)
