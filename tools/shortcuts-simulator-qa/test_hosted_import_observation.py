#!/usr/bin/env python3
"""Causal adapter controls. Synthetic signing/UI/DB observations confer no native authority."""
import argparse
import copy
import datetime
import fcntl
import json
import os
from pathlib import Path
import plistlib
import signal
import sys
import tempfile
import time
import unittest
from unittest import mock
import zipfile

import hosted_import_observation as hosted


class HostedImportObservationTests(unittest.TestCase):
    def setUp(self):
        output_environment = mock.patch.dict(os.environ, {"GITHUB_OUTPUT": ""})
        output_environment.start()
        self.addCleanup(output_environment.stop)
        self.temporary = tempfile.TemporaryDirectory(prefix="hosted-import-control-")
        self.addCleanup(self.temporary.cleanup)
        self.base = Path(self.temporary.name).resolve(strict=True)
        self.repo = self.base / "repo"
        self.artifacts = self.base / "test-run"
        self.root = self.artifacts / "builds/hosted-import"
        self.repo.mkdir()
        self.root.mkdir(parents=True)
        (self.artifacts / "runtime").mkdir()
        (self.artifacts / "builds/signed").mkdir()
        self.fixture = "qa-storage-delete-v2"
        self.commit = "a" * 40
        self.udid = "00000000-0000-0000-0000-000000000001"
        self.nonce = self.fixture + "-test-run-unique"
        self.phase = "import-" + self.nonce
        self.source_bytes = b'// fixed synthetic packet, not a native workflow\n'
        self.unsigned = plistlib.dumps({"WFWorkflowActions": [
            {"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
             "WFWorkflowActionParameters": {"WFCommentActionText": "synthetic"}}]})
        self.signed = b"AEA1-synthetic-signature-double-not-Apple-authenticated"
        source_path = self.repo / "tools/shortcuts-simulator-qa/fixtures" / (self.fixture + ".cherri")
        source_path.parent.mkdir(parents=True)
        source_path.write_bytes(self.source_bytes)
        (self.artifacts / "builds" / (self.fixture + ".cherri")).write_bytes(self.source_bytes)
        (self.artifacts / "builds" / (self.fixture + ".shortcut")).write_bytes(self.unsigned)
        (self.artifacts / "builds/signed" / (self.fixture + ".shortcut")).write_bytes(self.signed)
        self.target = {"compilerCommitSha": self.commit, "sourceSha256": hosted.sha(self.source_bytes),
            "unsignedPlistSha256": hosted.sha(self.unsigned), "unsignedByteSize": len(self.unsigned),
            "nativeActionCount": 1, "signedArtifactSha256": hosted.sha(self.signed),
            "signedByteSize": len(self.signed), "headerMagic": "AEA1"}
        self.signing = {"compilerCommitSha": self.commit, "signedArtifactValidAEA1": True,
            "signedArtifactSha256": hosted.sha(self.signed), "signedByteSize": len(self.signed),
            "targetProvenance": self.target}
        self.manifest = {"syntheticOnly": True, "compilerCommitSha": self.commit, "fixtures": [{
            "fixtureId": self.fixture, "fileName": self.fixture + ".cherri", "dialect": "v2",
            "buildProfile": "debug", "runtimeTraceExpectation": "REQUIRED_NDJSON",
            "sourceSha256": hosted.sha(self.source_bytes), "unsignedPlistSha256": hosted.sha(self.unsigned),
            "nativeActionCount": 1}]}
        self.put("compiler-manifest.json", self.manifest)
        self.put("runtime/signing-provenance.json", self.signing)
        (self.artifacts / "builds/xcodebuild-build.log").write_bytes(b"synthetic build log\n")
        self.context = {"schema": hosted.library.CONTEXT_SCHEMA, "runId": "test-run", "runNonce": self.nonce,
            "deviceUdid": self.udid, "phaseId": self.phase, "expectedName": self.fixture,
            "compilerCommitSha": self.commit, "builtHarnessCommitSha": self.commit,
            "controllerCommitSha": self.commit, "unsignedSHA256": hosted.sha(self.unsigned),
            "signedSHA256": hosted.sha(self.signed)}
        self.value = {"schema": hosted.SCHEMA, "fixtureId": self.fixture, "testId": hosted.TEST_ID,
            "repo": str(self.repo), "artifactRoot": str(self.artifacts), "privateRoot": str(self.root),
            "context": self.context, "workflowRunAttempt": "1", "packet": self.packet(),
            "deviceDataPath": str(self.base / "unexecuted-device"), "buildExitCode": 0,
            "buildLog": hosted.terminal.file_binding(self.artifacts, "builds/xcodebuild-build.log"),
            "deliverySeconds": 45, "phaseSeconds": 900, "formalAcceptanceGranted": False}
        self.addCleanup(self.retain_process_originals)

    def retain_process_originals(self):
        output = os.environ.get("P15_HOSTED_IMPORT_TEST_ORIGINALS")
        if not output or not (self.root / "process-started.original.json").is_file():
            return
        directory = Path(output) / self._testMethodName
        directory.mkdir(parents=True, exist_ok=False)
        for path in sorted(self.root.iterdir()):
            if path.is_file() and not path.is_symlink():
                (directory / path.name).write_bytes(path.read_bytes())

    def put(self, path, value):
        (self.artifacts / path).write_bytes(hosted.canonical(value))

    def packet(self):
        return hosted.packet(self.repo, self.artifacts, self.commit, "test-run", self.fixture)

    def save_prepared(self):
        hosted.write(self.root, "prepared.original.json", self.value)

    def test_packet_joins_exact_existing_manifest_source_unsigned_and_signing(self):
        self.assertEqual(self.packet()["actionCount"], 1)
        self.assertEqual(self.packet()["files"]["signed"]["sha256"], hosted.sha(self.signed))

    def test_aead_magic_alone_and_all_mismatched_signing_bindings_rejected(self):
        for key, value in self.target.items():
            with self.subTest(field=key):
                changed = copy.deepcopy(self.signing)
                changed["targetProvenance"][key] = (value + 1 if type(value) is int else value + "wrong")
                self.put("runtime/signing-provenance.json", changed)
                with self.assertRaises(ValueError):
                    self.packet()
        changed = copy.deepcopy(self.signing)
        del changed["targetProvenance"]
        self.put("runtime/signing-provenance.json", changed)
        with self.assertRaisesRegex(ValueError, "HOSTED_SIGNING_TARGET"):
            self.packet()

    def test_source_packet_changes_duplicate_manifest_and_wrong_profile_rejected(self):
        for change in ("source", "unsigned", "duplicate", "profile", "name"):
            with self.subTest(change=change):
                manifest = copy.deepcopy(self.manifest)
                path = None
                if change in ("source", "unsigned"):
                    path = self.artifacts / "builds" / (self.fixture + (".cherri" if change == "source" else ".shortcut"))
                    original = path.read_bytes()
                    path.write_bytes(original + b" ")
                elif change == "duplicate":
                    manifest["fixtures"] *= 2
                elif change == "profile":
                    manifest["fixtures"][0]["buildProfile"] = "release"
                else:
                    manifest["fixtures"][0]["fileName"] = "other.cherri"
                self.put("compiler-manifest.json", manifest)
                with self.assertRaises(Exception):
                    self.packet()
                if path:
                    path.write_bytes(original)
                self.put("compiler-manifest.json", self.manifest)

    def test_prepared_source_budget_bool_or_foreign_identity_rejected(self):
        self.save_prepared()
        self.assertEqual(hosted.load(self.root)[0], self.value)
        changes = [("buildExitCode", False), ("deliverySeconds", 60), ("phaseSeconds", 901),
                   ("testId", "Other/test02")]
        for key, bad in changes:
            with self.subTest(field=key):
                changed = copy.deepcopy(self.value)
                changed[key] = bad
                (self.root / "prepared.original.json").write_bytes(hosted.canonical(changed))
                with self.assertRaises(ValueError):
                    hosted.load(self.root)
        changed = copy.deepcopy(self.value)
        changed["context"]["builtHarnessCommitSha"] = "b" * 40
        (self.root / "prepared.original.json").write_bytes(hosted.canonical(changed))
        with self.assertRaisesRegex(ValueError, "HOSTED_PREPARED_IDENTITY"):
            hosted.load(self.root)

    def test_no_r2_or_unenrolled_fixture_or_name_relabel(self):
        args = argparse.Namespace(fixture=self.fixture, name=self.fixture, source=self.commit,
            run_id="test-run", attempt="1", nonce=self.nonce, udid=self.udid, root=str(self.root),
            repo=str(self.repo), artifacts=str(self.artifacts))
        self.assertEqual(hosted.configuration(args)[-1], self.phase)
        for name in ("qa-function-envelope-presence-v2-r2", "qa-functional-v2"):
            args.fixture = args.name = name
            with self.assertRaises(ValueError):
                hosted.configuration(args)
        args.fixture, args.name = self.fixture, self.fixture + "-renamed"
        with self.assertRaises(ValueError):
            hosted.configuration(args)

    def test_paths_and_create_only_originals(self):
        hosted.write(self.root, "once.json", {"x": 1})
        with self.assertRaises(FileExistsError):
            hosted.write(self.root, "once.json", {"x": 2})
        (self.root / "alias").symlink_to(self.base)
        for path in ("../escape", "alias/escape", "/escape"):
            with self.subTest(path=path), self.assertRaises(ValueError):
                hosted.write(self.root, path, b"not-written")

    def test_preexisting_library_record_prevents_even_process_creation(self):
        with mock.patch.object(hosted, "source"), mock.patch.object(hosted, "packet_again"), \
             mock.patch.object(hosted.library, "observe", return_value={"synthetic": True}), \
             mock.patch.object(hosted.library, "admit_absence", side_effect=ValueError("PREEXISTING_LIBRARY_RECORD")), \
             mock.patch.object(hosted, "capture") as process:
            with self.assertRaisesRegex(ValueError, "PREEXISTING_LIBRARY_RECORD"):
                hosted.dispatch(self.value, self.root, time.monotonic() + 10)
            process.assert_not_called()
        self.assertFalse((self.root / "process-started.original.json").exists())
        self.assertTrue((self.root / "dispatch-claim.original.json").is_file())

    def test_dispatch_claim_blocks_second_file_open(self):
        hosted.write(self.root, "dispatch-claim.original.json", {"old": True})
        with mock.patch.object(hosted, "source") as source, mock.patch.object(hosted, "capture") as process:
            with self.assertRaises(FileExistsError):
                hosted.dispatch(self.value, self.root, time.monotonic() + 10)
            source.assert_not_called()
            process.assert_not_called()

    def test_process_projection_requires_true_reaped_zero_and_absent_groups(self):
        detail = {"startedAt": "2026-11-26T00:00:01Z", "finishedAt": "2026-11-26T00:00:02Z", "pid": 1,
            "cleanupParentReaped": True, "returnCodeObserved": 0,
            "groupBeforeCleanup": {"state": "ABSENT"}, "groupAfterCleanup": {"state": "ABSENT"}}
        observed = {"result": "PASS", "exitCode": 0, "elapsedMs": 1000}
        self.assertEqual(hosted.dispatch_projection([], detail, observed, "x")["outcome"], "EXITED")
        for key, value in (("cleanupParentReaped", False), ("returnCodeObserved", False),
                           ("groupBeforeCleanup", None), ("groupAfterCleanup", {"state": "PERMISSION_DENIED"})):
            with self.subTest(field=key):
                changed = {**detail, key: value}
                self.assertEqual(hosted.dispatch_projection([], changed, observed, "x")["outcome"], "FAILED")
        self.assertEqual(hosted.dispatch_projection([], detail, {**observed, "exitCode": False}, "x")["outcome"], "FAILED")

    def admission_inputs(self):
        """Authored provenance doubles only; no Apple operation is performed."""
        self.save_prepared()
        prepared_sha = hosted.sha(hosted.read(self.root, "prepared.original.json"))
        stamp = lambda n: "2026-11-26T00:00:%02d.000Z" % n
        signed = self.artifacts / "builds/signed" / (self.fixture + ".shortcut")
        argv = ["xcrun", "simctl", "openurl", self.udid, signed.as_uri()]
        detail = {"startedAt": stamp(1), "finishedAt": stamp(2), "pid": 123,
            "cleanupParentReaped": True, "returnCodeObserved": 0,
            "groupBeforeCleanup": {"state": "ABSENT"}, "groupAfterCleanup": {"state": "ABSENT"},
            "stdout": {"bytes": 0, "eof": True, "overflow": False, "sha256": hosted.sha(b"")},
            "stderr": {"bytes": 0, "eof": True, "overflow": False, "sha256": hosted.sha(b"")}}
        process = {"result": "PASS", "exitCode": 0, "elapsedMs": 1000}
        completed = hosted.dispatch_projection(argv, detail, process, stamp(1))
        completed["completedUTC"] = stamp(2)
        started = {key: completed[key] for key in ("schemaVersion", "command", "timeoutSeconds", "startedUTC", "pid", "phasePublishedUTC")}
        started.update(outcome="LAUNCHED", timedOut=False, exitCode=None)
        originals = {
            "owner-result.original.json": {"status": "DELIVERY_COMPLETED", "preparedSha256": prepared_sha},
            "process-result.original.json": {"context": self.context, "errorType": None, "argv": argv, "details": detail, "process": process},
            "dispatch-started.original.json": started, "dispatch-result.original.json": completed,
            "dispatch-" + self.phase + ".json": completed}
        node = {"kind": "cell", "enabled": True, "label": self.fixture, "identifier": "owned-cell", "children": []}
        timing = {"contract": "dispatch-exit-anchored-add-v1", "dispatchState": "EXITED",
            "dispatchReceipt": "dispatch-" + self.phase + ".json", "sheetContract": "requested-identity-and-add-v1",
            "sheetButtonExists": True, "sheetButtonHittable": True, "sheetIdentityMatched": True, "sheetCanTap": True,
            "sheetSelectionContract": "current-container-bound-add-v1", "sheetSelectionReason": "UNIQUE_CURRENT_NAMED_CONTAINER",
            "sheetQueryCandidateCount": 1, "sheetCurrentContainerCount": 1, "sheetMatchedCandidateCount": 1,
            "sheetSelectedQueryIndex": 0, "sheetSelectedContainerPath": [0], "sheetSelectedTitlePath": [0, 0],
            "sheetSelectedButtonPath": [0, 1], "addObservedUTC": stamp(2), "lastSheetCheckUTC": stamp(2),
            "sheetMatchedAfterCaptureUTC": stamp(2), "libraryContract": "dispatch-exit-anchored-library-v1",
            "libraryCanConfirm": True, "librarySelectionContract": "unique-current-library-cell-v1",
            "librarySelectionReason": "UNIQUE_CURRENT_NAMED_LIBRARY_CELL", "libraryCurrentNamedCellCount": 1,
            "libraryQueryCandidateCount": 1, "librarySelectedQueryIndex": 0, "librarySelectedCellPath": [0],
            "librarySelectedCellLabel": self.fixture, "librarySelectedCellIdentifier": "owned-cell",
            "libraryDeadlineUTC": stamp(32), "librarySelectionObservedUTC": stamp(4), "libraryMatchedAfterCaptureUTC": stamp(4)}
        ui = {"testId": hosted.TEST_ID, "preparedSha256": prepared_sha, "context": self.context, "workflowRunAttempt": "1",
            "id": self.phase, "operation": "import", "outcome": "IMPORTED", "name": self.fixture,
            "identityVerified": True, "libraryIdentityVerified": True, "importTiming": timing,
            "libraryUniqueCurrentCellVerified": True, "requiresAbsentLibraryRecord": True, "replaceTapped": False}
        originals["ui-receipt.original.json"] = ui
        for i in (1, 2):
            originals[self.phase + "-library-observation-%04d.json" % i] = {
                "schema": "cherri-current-library-selection-observation/1", "phase": self.phase,
                "ordinal": i, "expectedName": self.fixture, "foreground": True, "dispatchConfirmed": True,
                "decision": "UNIQUE_CURRENT_NAMED_LIBRARY_CELL", "canConfirm": True,
                "currentNamedCellCount": 1, "queryCandidateCount": 1, "candidates": [{"exists": True,
                    "hittable": True, "queryIndex": 0, "snapshot": node}], "current": {"children": [node]},
                "observedUTC": stamp(i + 2), "deadlineUTC": stamp(32)}
        action_hash = hosted.library.action_input(self.unsigned)[1]
        before = {"schema": hosted.library.SCHEMA, "status": "OBSERVED", "stage": "before", "context": self.context,
            "actualPlatform": "darwin", "queryOnly": True, "sourceSQLWrites": 0, "formalAcceptanceGranted": False,
            "unsignedSHA256": self.context["unsignedSHA256"], "signedSHA256": self.context["signedSHA256"],
            "query": hosted.library.QUERY, "queryName": self.fixture, "records": [], "matchingRecordCount": 0,
            "activeRecordCount": 0, "readStartedUTC": stamp(0), "readCompletedUTC": stamp(1),
            "expectedActionsTypedSHA256": action_hash}
        row = {"rowId": 1, "workflowUUID": "00000000-0000-0000-0000-000000000002", "name": self.fixture,
            "actionCount": 1, "hidden": False, "tombstoned": False, "actionsMatchUnsigned": True,
            "actionsTypedSHA256": action_hash, "createdUTC": stamp(2), "modifiedUTC": stamp(2)}
        after = {**before, "stage": "after", "records": [row], "matchingRecordCount": 1,
            "activeRecordCount": 1, "readStartedUTC": stamp(5), "readCompletedUTC": stamp(6)}
        (self.root / "library-before").mkdir()
        originals["library-before/result.original.json"] = before
        for name, value in originals.items():
            hosted.write(self.root, name, value)
        hosted.write(self.root, "stdout.original", b"")
        hosted.write(self.root, "stderr.original", b"")
        return after

    def test_admission_requires_joined_original_process_ax_and_new_native_record(self):
        after = self.admission_inputs()
        with mock.patch.object(hosted, "source"), mock.patch.object(hosted.library, "observe", return_value=after):
            self.assertTrue(hosted.verify(self.root))
        actual = hosted.library.strict_json(hosted.read(self.root, "admission.original.json"))
        self.assertTrue(actual["importIdentityAuthenticated"])
        self.assertFalse(actual["runtimeExecuted"])
        self.assertFalse(actual["formalAcceptanceGranted"])

    def test_admission_rejects_missing_raw_projection_wrong_ui_and_ambiguous_record(self):
        changes = ("raw_stream", "projection", "ui_owner", "ui_name", "replace", "duplicate_ax", "wrong_ax_cell", "record_count", "record_actions", "old_record")
        for change in changes:
            with self.subTest(change=change):
                case = HostedImportObservationTests("runTest")
                case.setUp()
                try:
                    after = case.admission_inputs()
                    def mutate(name, function):
                        path = case.root / name
                        value = hosted.library.strict_json(path.read_bytes())
                        function(value)
                        path.write_bytes(hosted.canonical(value))
                    if change == "raw_stream":
                        (case.root / "stdout.original").write_bytes(b"unjoined bytes")
                    elif change == "projection":
                        mutate("dispatch-result.original.json", lambda v: v.update(exitCode=False))
                    elif change.startswith("ui_"):
                        mutate("ui-receipt.original.json", lambda v: v.update(**({"testId": "Other/test"} if change == "ui_owner" else {"name": "different"})))
                    elif change == "replace":
                        mutate("ui-receipt.original.json", lambda v: v.update(replaceTapped=True))
                    elif change in ("duplicate_ax", "wrong_ax_cell"):
                        mutate(case.phase + "-library-observation-0002.json", lambda v: v.update(queryCandidateCount=2) if change == "duplicate_ax" else v["current"]["children"][0].update(identifier="other"))
                    elif change == "record_count":
                        after["records"] *= 2
                        after.update(matchingRecordCount=2, activeRecordCount=2)
                    elif change == "record_actions":
                        after["records"][0]["actionsMatchUnsigned"] = False
                    else:
                        mutate("library-before/result.original.json", lambda v: v.update(records=after["records"], matchingRecordCount=1, activeRecordCount=1))
                    with mock.patch.object(hosted, "source"), mock.patch.object(hosted.library, "observe", return_value=after):
                        self.assertFalse(hosted.verify(case.root))
                    result = hosted.library.strict_json(hosted.read(case.root, "admission.original.json"))
                    self.assertFalse(result["importIdentityAuthenticated"])
                    self.assertFalse(result["runtimeExecuted"])
                finally:
                    case.doCleanups()

    def test_actual_owned_posix_child_cancellation_keeps_cleanup_and_skips_new_git(self):
        lock_path, ready = self.base / "child.lock", self.base / "child.ready"
        code = "import fcntl,pathlib,time; f=open(%r,'w'); fcntl.flock(f,fcntl.LOCK_EX); pathlib.Path(%r).write_text('ready'); time.sleep(8)" % (str(lock_path), str(ready))
        actual_capture = hosted.capture
        def observed_capture(argv, allowance, **kwargs):
            self.assertEqual(argv[:3], ["xcrun", "simctl", "openurl"])
            self.assertEqual(allowance, 45)
            callback = kwargs["journal"]
            def journal(stage, detail):
                callback(stage, detail)
                if stage == "started":
                    until = time.monotonic() + 2
                    while not ready.exists() and time.monotonic() < until:
                        time.sleep(.01)
                    self.assertTrue(ready.is_file(), "owned synthetic child did not become ready")
                    os.kill(os.getpid(), signal.SIGTERM)
            kwargs["journal"] = journal
            return actual_capture([sys.executable, "-c", code], allowance, **kwargs)
        with mock.patch.object(hosted, "source") as source, mock.patch.object(hosted, "packet_again"), \
             mock.patch.object(hosted.library, "observe", return_value={"synthetic": True}), \
             mock.patch.object(hosted.library, "admit_absence", return_value={"syntheticAbsent": True}), \
             mock.patch.object(hosted, "capture", side_effect=observed_capture):
            with hosted.interrupts(time.monotonic() + 10), self.assertRaises(TimeoutError):
                hosted.dispatch(self.value, self.root, time.monotonic() + 10)
            self.assertEqual(source.call_count, 1)
        result = hosted.library.strict_json(hosted.read(self.root, "process-result.original.json"))
        self.assertEqual(result["process"]["result"], "OWNER_INTERRUPTED")
        self.assertTrue(result["details"]["cleanupParentReaped"])
        self.assertEqual(result["details"]["groupAfterCleanup"]["state"], "ABSENT")
        self.assertTrue((self.root / "process-started.original.json").is_file())
        self.assertTrue((self.root / "process-finished.original.json").is_file())
        self.assertEqual(hosted.library.strict_json(hosted.read(self.root, "closing-error.original.json"))["status"], "NOT_RUN")
        with lock_path.open("a") as stream:
            fcntl.flock(stream, fcntl.LOCK_EX | fcntl.LOCK_NB)

    def test_seal_preserves_failed_originals_and_publishes_only_ciphertext(self):
        hosted.write(self.root, "owner-result.original.json", {"status": "FAILED", "private": "private-canary-שלום"})
        destination = self.base / "sealed/hosted-import"
        destination.parent.mkdir()
        def encrypt(private, public, archive, binding, recipient, deadline):
            with zipfile.ZipFile(archive) as z:
                self.assertIn(b"private-canary", z.read("owner-result.original.json"))
                self.assertIn("manifest.original.json", z.namelist())
            cipher = b"0synthetic-encryption-double-no-native-authority"
            (public / "project-prerequisite.zip.p7m").write_bytes(cipher)
            return {"bytes": len(cipher), "sha256": hosted.sha(cipher)}
        with mock.patch.object(hosted.observation, "encrypt_archive", side_effect=encrypt):
            summary = hosted.seal(self.root, destination)
        self.assertFalse(summary["formalAcceptanceGranted"])
        self.assertEqual({p.name for p in (destination / "public").iterdir()}, {"summary.json", "session-originals.zip.p7m"})
        self.assertNotIn(b"private-canary", (destination / "public/summary.json").read_bytes())
        with self.assertRaises(ValueError):
            hosted.seal(self.root, destination)

    def test_seal_rejects_symlink_and_actual_overflow_before_encryption(self):
        target = self.base / "not-owned"
        target.write_text("private")
        (self.root / "alias").symlink_to(target)
        destination = self.base / "sealed/hosted-import"
        destination.parent.mkdir()
        with mock.patch.object(hosted.observation, "encrypt_archive") as encrypt:
            with self.assertRaisesRegex(ValueError, "HOSTED_SEAL_MEMBERS"):
                hosted.seal(self.root, destination)
            encrypt.assert_not_called()
        (self.root / "alias").unlink()
        other = self.base / "other/hosted-import"
        other.parent.mkdir()
        hosted.write(self.root, "large.original", b"123456789")
        with mock.patch.object(hosted, "MAX_ORIGINAL_BYTES", 8), mock.patch.object(hosted.observation, "encrypt_archive") as encrypt:
            with self.assertRaisesRegex(ValueError, "HOSTED_SEAL_TOTAL"):
                hosted.seal(self.root, other)
            encrypt.assert_not_called()

    def test_enrollment_closure_and_real_shell_syntax(self):
        # These path/dispatch assertions bind this additive adapter to the
        # actual entrypoint, rather than accepting disconnected unit fixtures.
        repo = Path(__file__).resolve().parents[2]
        required = ("hosted_import_observation.py", "test_hosted_import_observation.py",
            "shortcut_library_identity.py", "XcodeProject/UITests/QAImportedShortcutObserver.swift",
            "XcodeProject/Shared/DifferentialImportTiming.swift", "XcodeProject/Shared/ImportSheetSelection.swift")
        self.assertTrue(all("tools/shortcuts-simulator-qa/" + x in hosted.terminal.SOURCE_FILES for x in required))
        shell = (repo / "scripts/ios27_shortcuts_simulator_qa.sh").read_text()
        self.assertIn('if [ "$HOSTED_IMPORT_IDENTITY" = "true" ]; then', shell)
        self.assertIn('--build-exit-code "$XCODEBUILD_BUILD_RC"', shell)
        self.assertIn('[ "$HOSTED_IMPORT_ADMITTED" = "true" ]', shell)
        self.assertIn('seal_hosted_import; rm -rf', shell)
        import subprocess
        result = subprocess.run(["bash", "-n", str(repo / "scripts/ios27_shortcuts_simulator_qa.sh")],
                                capture_output=True, timeout=5)
        self.assertEqual(result.returncode, 0, result.stderr)


if __name__ == "__main__":
    unittest.main()
