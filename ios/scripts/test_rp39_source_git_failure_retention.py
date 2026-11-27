#!/usr/bin/env python3
"""Real Git/file/crypto controls; synthetic repositories, no native acceptance."""
import contextlib
import importlib.util
import inspect
import io
import json
import os
from pathlib import Path
import platform
import shutil
import subprocess
import sys
import time
import unittest
from unittest import mock
import zipfile


HERE = Path(__file__).resolve().parent
CONTROL_ORIGINALS = {}


def load(name):
    spec = importlib.util.spec_from_file_location("_source_git_control_" + name, HERE / (name + ".py"))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


transport = load("test_rp39_diagnostic_transport")
base, profile, seal = transport.base, transport.profile, transport.seal
executable = load("source_git_identity")


class RP39SourceGitFailureRetentionTests(unittest.TestCase):
    def setUp(self):
        transport.RP39DiagnosticTransportTests.setUp(self)
        base._git_identity = None
        self.addCleanup(setattr, base, "_git_identity", None)

    def retain(self, directory):
        CONTROL_ORIGINALS.setdefault(self._testMethodName, {}).update({
            str(path.relative_to(directory)): {"encoding": "hex", "data": path.read_bytes().hex()}
            for path in sorted(directory.rglob("*")) if path.is_file()})

    def copied_git(self, name="git"):
        path = self.path / name
        source = executable.HOMEBREW_GIT if sys.platform == "darwin" and platform.machine() == "arm64" else Path("/usr/bin/git")
        shutil.copyfile(source.resolve(strict=True), path)
        path.chmod(0o755)
        return path

    def test_verified_real_git_preserves_source_commands_and_deadlines(self):
        deadline = time.monotonic() + 15
        if sys.platform == "darwin" and platform.machine() == "arm64":
            selected = base.select_verified_git(self.repo, deadline)
            selection_scope = "ACTUAL_INSTALLED_HOMEBREW_GIT"
        else:
            # Linux has no Homebrew macOS executable. Only that platform/path
            # selection is substituted; both collectors execute real Git.
            selected = executable.VerifiedGit(self.copied_git(), deadline)
            selection_scope = "LINUX_REAL_GIT_COPY_PLATFORM_SUBSTITUTION"
        base._git_identity = selected
        output = self.path / "verified-git"
        with base.envelope(15):
            base._owned_git = base.OwnedGitReads(self.repo, output)
            source = base.source_binding(self.repo, self.source, deadline)
        self.assertEqual(source["sourceSha"], self.source)
        rows = base._owned_git.records
        self.assertEqual(len(rows), 5)
        for row in rows:
            self.assertEqual(row["operation"], "git")
            self.assertIn(str(selected.path), row["argv"])
            self.assertNotIn("/usr/bin/xcode-select", row["argv"])
            self.assertEqual(row["hardDeadlineMonotonic"], deadline)
            self.assertEqual(row["executionDeadlineMonotonic"], deadline - 2)
            self.assertEqual(row["observed"]["result"], "PASS")
            self.assertTrue(row["capture"]["cleanupParentReaped"])
            self.assertEqual(row["capture"]["groupAfterCleanup"]["state"], "ABSENT")
        self.retain(output)
        actual_run, calls = subprocess.run, []
        def observed(argv, **kwargs):
            calls.append({"argv": argv, "timeout": kwargs["timeout"]})
            return actual_run(argv, **kwargs)
        selected = executable.VerifiedGit(selected.logical, time.monotonic() + 20)
        with mock.patch.object(profile, "select_verified_git", return_value=selected), \
                mock.patch.object(profile.subprocess, "run", side_effect=observed):
            snapshot = profile.source_snapshot(self.repo, profile.context_from_environment(self.env),
                profile.RP39_SOURCE_FILES, verified_homebrew_git=True)
        self.assertEqual(set(snapshot["files"]), set(profile.RP39_SOURCE_FILES))
        self.assertEqual(len(calls), 5)
        self.assertEqual([call["argv"][4:6] for call in calls], [
            ["rev-parse", "HEAD"], ["rev-parse", self.source + "^{tree}"],
            ["ls-tree", "-r"], ["status", "--porcelain"], ["rev-parse", "HEAD"]])
        for call in calls:
            self.assertEqual(call["argv"][:4], [str(selected.path), "--no-replace-objects", "-C", str(self.repo)])
            self.assertGreater(call["timeout"], 0)
            self.assertLessEqual(call["timeout"], 10)
        CONTROL_ORIGINALS[self._testMethodName]["selection.original.json"] = {
            "scope": selection_scope, "path": str(selected.path), "sha256": selected.sha256,
            "bytes": selected.bytes, "profileCommands": calls, "nativeAcceptanceGranted": False}

    def test_executable_alias_bytes_deadline_and_bootstrap_fail_closed(self):
        actual = self.copied_git()
        alias = self.path / "git-alias"
        alias.symlink_to(actual)
        selected = executable.VerifiedGit(alias, time.monotonic() + 15)
        other = self.copied_git("other-git")
        alias.unlink()
        alias.symlink_to(other)
        with self.assertRaisesRegex(ValueError, "SOURCE_GIT_CHANGED"):
            selected.check(time.monotonic() + 15)
        selected = executable.VerifiedGit(actual, time.monotonic() + 15)
        original = actual.read_bytes()
        with actual.open("r+b") as handle:
            handle.seek(len(original) - 1)
            handle.write(bytes((original[-1] ^ 1,)))
        base._git_identity = selected
        with mock.patch.object(subprocess, "Popen") as child:
            with self.assertRaisesRegex(ValueError, "SOURCE_GIT_CHANGED"):
                base.local_git(self.repo, ["rev-parse", "HEAD"], time.monotonic() + 15)
            child.assert_not_called()
        actual.write_bytes(original)
        selected = executable.VerifiedGit(actual, time.monotonic() + 15)
        with self.assertRaisesRegex(ValueError, "SOURCE_GIT_DEADLINE"):
            selected.check(time.monotonic() - 1)
        bootstrap = self.repo / base.GIT_IDENTITY_FILE
        bootstrap.write_bytes(bootstrap.read_bytes() + b"\n# controlled mutation\n")
        with mock.patch.object(subprocess, "Popen") as child:
            with self.assertRaisesRegex(ValueError, "SOURCE_CHANGED"):
                base.select_verified_git(self.repo, time.monotonic() + 15)
            with self.assertRaisesRegex(ValueError, "COMPONENT_SOURCE_CHANGED"):
                profile.select_verified_git(self.repo, time.monotonic() + 20)
            child.assert_not_called()

    def failed_originals(self):
        with mock.patch.dict(os.environ, self.env), contextlib.redirect_stdout(io.StringIO()):
            profile.record(self.repo, "before", profile.context_from_environment(self.env), rp39=True)
        # Corrupt only this disposable repository's Git metadata. Both failing
        # reads below are actual Git processes; the declared source bytes stay exact.
        (self.repo / ".git/HEAD").write_bytes(b"SYNTHETIC_INVALID_HEAD\n")
        root = self.path / base.root_name(self.context)
        output = root.parent / (root.name + "-git-prepare")
        with base.envelope(15):
            base._owned_git = base.OwnedGitReads(self.repo, output)
            with self.assertRaisesRegex(ValueError, "SOURCE_CHANGED"):
                base.local_git(self.repo, ["rev-parse", "--verify", "HEAD"], time.monotonic() + 15)
        row = base._owned_git.records[0]
        self.assertEqual(row["observed"]["result"], "EXIT_NONZERO")
        self.assertTrue(row["capture"]["cleanupParentReaped"])
        self.assertEqual(row["capture"]["groupAfterCleanup"]["state"], "ABSENT")
        with mock.patch.dict(os.environ, self.env), contextlib.redirect_stdout(io.StringIO()):
            with self.assertRaisesRegex(ValueError, "COMPONENT_GIT_READ"):
                profile.record(self.repo, "after", profile.context_from_environment(self.env), rp39=True)
        self.retain(output)
        self.retain(self.repo / "ios/.build/rp39-diagnostic-source")
        return root, output

    def test_real_failed_git_originals_receive_only_failure_encryption(self):
        root, commands = self.failed_originals()
        failure = self.repo / "ios/.build/rp39-diagnostic-source/after-failure.original.json"
        detail = json.loads(failure.read_text())
        self.assertEqual(detail["exception"]["argv"][0], "/usr/bin/git")
        self.assertEqual(detail["exception"]["returnCode"], 128)
        self.assertGreater(detail["exception"]["stderr"]["bytes"], 0)
        self.assertFalse(detail["formalAcceptanceGranted"])
        private_canary = b"SYNTHETIC_PRIVATE_SOURCE_FAILURE_CANARY\n"
        (commands / "control-canary.original").write_bytes(private_canary)
        output = self.path / "failure-seal"
        with mock.patch.dict(os.environ, self.env), seal.base.envelope(300):
            # The same control can exercise the unchanged predecessor seal.
            # Its missing-after failure is retained as the causal RED result.
            options = {"failure_only": True} if "failure_only" in inspect.signature(seal.seal).parameters else {}
            receipt = seal.seal(self.repo, self.context, root, output, started=time.monotonic(), **options)
        self.retain(output)
        self.assertEqual(receipt["schema"], "cherri-rp39-source-failure-diagnostics/1")
        self.assertEqual(receipt["disposition"], "SOURCE_AFTER_FAILED")
        self.assertFalse(receipt["sourceClosureVerified"])
        self.assertFalse(receipt["formalAcceptanceGranted"])
        self.assertFalse(receipt["finishedXCResultPresent"])
        self.assertIsNone(receipt["nativeExitCode"])
        self.assertIsNone(receipt["sourceReceipts"]["after"])
        self.assertTrue(receipt["encryptionProcess"]["capture"]["cleanupParentReaped"])
        self.assertEqual(receipt["encryptionProcess"]["capture"]["groupAfterCleanup"]["state"], "ABSENT")
        public = (output / "public/receipt.json").read_bytes()
        self.assertNotIn(private_canary.strip(), public)
        with zipfile.ZipFile(output / "private/rp39-originals.zip") as archive:
            self.assertEqual(archive.read("sources/after-failure.original.json"), failure.read_bytes())
            self.assertEqual(archive.read("source-reads/prepare/command-01/stderr.original"),
                             (commands / "command-01/stderr.original").read_bytes())
            self.assertEqual(archive.read("source-reads/prepare/control-canary.original"), private_canary)
            self.assertNotIn("sources/after.original.json", archive.namelist())
            self.assertEqual(json.loads(archive.read("manifest.original.json"))["schema"], receipt["schema"])
        with mock.patch.dict(os.environ, self.env), self.assertRaises(FileNotFoundError):
            seal.source_receipts(self.repo, self.context)

    def test_failure_retention_cannot_supply_or_bypass_source_closure(self):
        root, _ = self.failed_originals()
        directory = self.repo / "ios/.build/rp39-diagnostic-source"
        failure = directory / "after-failure.original.json"
        original = failure.read_bytes()
        with mock.patch.dict(os.environ, self.env):
            valid = seal.failed_source_receipts(self.repo, self.context)
            self.assertIsNone(valid["after"])
            failure.unlink()
            with self.assertRaises(FileNotFoundError):
                seal.failed_source_receipts(self.repo, self.context)
            failure.write_bytes(original)
            wrong = json.loads(original)
            wrong["context"]["source"] = "0" * 40
            failure.write_text(json.dumps(wrong))
            with self.assertRaises(ValueError):
                seal.failed_source_receipts(self.repo, self.context)
            failure.write_bytes(original)
            after = directory / "after.original.json"
            after.write_bytes(b"{}\n")
            with self.assertRaises(ValueError):
                seal.failed_source_receipts(self.repo, self.context)
            with self.assertRaises(ValueError):
                seal.source_receipts(self.repo, self.context)
            after.unlink()
            source = self.repo / "ios/CherriApp/App/WorkspaceView.swift"
            source.write_bytes(source.read_bytes() + b"\n# controlled source mutation\n")
            with self.assertRaisesRegex(ValueError, "SOURCE_CHANGED"):
                seal.failed_source_receipts(self.repo, self.context)
            with mock.patch.object(seal, "encrypt_archive") as encryption:
                with self.assertRaisesRegex(ValueError, "SOURCE_CHANGED"):
                    seal.seal(self.repo, self.context, root, self.path / "rejected",
                              started=time.monotonic(), failure_only=True)
                encryption.assert_not_called()


if __name__ == "__main__":
    unittest.main(verbosity=2)
