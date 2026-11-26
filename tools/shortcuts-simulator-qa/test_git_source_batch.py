#!/usr/bin/env python3
"""Real Git/pipe controls for selected source verification; no Apple authority."""
import base64
import contextlib
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import tempfile
import time
import types
import unittest
from unittest import mock

import git_source_batch as batch
import qa_terminal_receipts as production

REAL_POPEN = subprocess.Popen
REAL_RUN = subprocess.run

ORIGINAL_SOURCE_FILES = ('tools/shortcuts-simulator-qa/XcodeProject/Shared/PrimaryNoInputLaunch.swift', 'tools/shortcuts-simulator-qa/verify_primary_no_input.py', 'tools/shortcuts-simulator-qa/verify_primary_no_input.swift', 'tools/shortcuts-simulator-qa/owned_simulator_selection.py', 'tools/shortcuts-simulator-qa/test_owned_simulator_selection.py', 'tools/shortcuts-simulator-qa/optional_forwarding_diagnostic.py', 'tools/shortcuts-simulator-qa/verify_optional_receiver.py', 'tools/shortcuts-simulator-qa/tagged_carrier_runtime.py', 'tools/shortcuts-simulator-qa/test_tagged_carrier_runtime.py', 'tools/shortcuts-simulator-qa/tagged_carrier_public_originals.py', 'tools/shortcuts-simulator-qa/test_tagged_carrier_public_originals.py', 'tools/shortcuts-simulator-qa/tagged_carrier_graph.py', 'tools/shortcuts-simulator-qa/test_tagged_carrier_graph.py', 'tools/shortcuts-simulator-qa/tagged_carrier_observation.py', 'tools/shortcuts-simulator-qa/test_tagged_carrier_observation.py', 'tools/shortcuts-simulator-qa/native_comparison_graph.py', 'tools/shortcuts-simulator-qa/nested_list_public_originals.py', 'tools/shortcuts-simulator-qa/runtime_poc_publication.py', 'tools/shortcuts-simulator-qa/runtime_poc_device.py', 'tools/shortcuts-simulator-qa/test_runtime_callback_contract.py', 'tools/shortcuts-simulator-qa/fixtures/qa-tagged-value-carrier-v2.cherri', 'tools/shortcuts-simulator-qa/fixtures/qa-tagged-value-carrier-v2.expected.json', 'tools/shortcuts-simulator-qa/fixtures/qa-tagged-value-carrier-v2.enrollment.json', 'tools/shortcuts-simulator-qa/XcodeProject/UITests/CherriSimulatorQAUITests.swift', 'tools/shortcuts-simulator-qa/simulator_listapps_observation.py', 'tools/shortcuts-simulator-qa/test_simulator_listapps_observation.py', 'tools/shortcuts-simulator-qa/project_prerequisite_observation.py', 'tools/shortcuts-simulator-qa/test_process_capture_journal.py', 'tools/shortcuts-simulator-qa/simulator_phase_boundary.py', 'tools/shortcuts-simulator-qa/test_simulator_phase_boundary.py', 'tools/shortcuts-simulator-qa/nested_list_runtime.py', 'tools/shortcuts-simulator-qa/nested_list_probe.py', 'tools/shortcuts-simulator-qa/fixtures/qa-nested-list-control-v2.cherri', 'tools/shortcuts-simulator-qa/fixtures/qa-nested-list-control-v2.expected.json', 'tools/shortcuts-simulator-qa/fixtures/qa-nested-list-candidate-v2.cherri', 'tools/shortcuts-simulator-qa/fixtures/qa-nested-list-candidate-v2.expected.json', 'tools/shortcuts-simulator-qa/XcodeProject/ReceiverApp/ReceiverApp.swift', 'tools/shortcuts-simulator-qa/xcode_destination_diagnostic.py', 'tools/shortcuts-simulator-qa/project_prerequisite.py', 'tools/shortcuts-simulator-qa/qa_terminal_receipts.py', 'tools/shortcuts-simulator-qa/prepare_public_artifacts.py', 'tools/shortcuts-simulator-qa/simulator_session_recovery.py', 'tools/shortcuts-simulator-qa/xcode_destination_readiness.py', 'scripts/ios27_shortcuts_simulator_qa.sh', '.github/workflows/cherri-ios27-shortcuts-simulator-qa.yml')


class TerminalSourceBatchTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="cherri-source-batch-")
        self.base = Path(self.temporary.name).resolve(strict=True)
        self.root = self.base / "repo"
        self.root.mkdir()
        for relative in production.SOURCE_FILES:
            path = self.root / relative
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(("synthetic exact source: " + relative + "\n").encode())
        owner = self.root / "tools/shortcuts-simulator-qa/qa_terminal_receipts.py"
        owner.write_bytes(Path(production.__file__).read_bytes())
        self.git("init", "-q")
        self.git("add", ".")
        self.commit("original fixture")
        self.source = self.git("rev-parse", "HEAD").decode().strip()
        spec = importlib.util.spec_from_file_location("actual_terminal_source_fixture", owner)
        self.module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(self.module)
        self.originals = self.base / "originals"
        self.originals.mkdir(mode=0o700)
        self.calls, self.processes = [], []

    def tearDown(self):
        # Only the actual children created by this test are eligible. An
        # exhausted production deadline remains a failed original observation.
        for process in self.processes:
            if process.poll() is None:
                try:
                    os.killpg(process.pid, signal.SIGKILL)
                except ProcessLookupError:
                    pass
            process.wait(timeout=2)
        self.temporary.cleanup()

    def git(self, *args):
        return REAL_RUN(["git", "--no-replace-objects", "-C", str(self.root), *args],
            check=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=5).stdout

    def commit(self, message):
        self.git("-c", "user.name=Synthetic source verifier", "-c",
                 "user.email=synthetic@invalid.example", "commit", "-qm", message)

    def record(self, argv, *args, **kwargs):
        self.calls.append(list(argv))
        process = REAL_POPEN(argv, *args, **kwargs)
        self.processes.append(process)
        return process

    def verify(self, *, originals=None):
        with mock.patch.object(subprocess, "Popen", side_effect=self.record):
            return self.module.verify_checkout(self.root, self.source, originals=originals)

    def transcript(self):
        requests = ["HEAD"] + [self.source + ":" + p for p in production.SOURCE_FILES] + ["HEAD"]
        raw = REAL_RUN(list(batch.COMMAND), cwd=self.root,
            input=("\n".join(requests) + "\n").encode(), stdout=subprocess.PIPE,
            stderr=subprocess.PIPE, check=True, timeout=5).stdout
        blocks, offset = [], 0
        for _ in requests:
            end = raw.index(b"\n", offset)
            size = int(raw[offset:end].split()[-1])
            next_offset = end + 1 + size + 1
            blocks.append(raw[offset:next_offset])
            offset = next_offset
        self.assertEqual(offset, len(raw))
        return requests, blocks

    @contextlib.contextmanager
    def fake_git(self, blocks, *, trailing=b"", stderr=b"", exit_code=0, delay=0):
        requests = ["HEAD"] + [self.source + ":" + p for p in production.SOURCE_FILES] + ["HEAD"]
        configuration = self.base / "fake-protocol.json"
        configuration.write_text(json.dumps({"requests": requests,
            "blocks": [base64.b64encode(p).decode() for p in blocks],
            "trailing": base64.b64encode(trailing).decode(),
            "stderr": base64.b64encode(stderr).decode(), "exit": exit_code, "delay": delay}))
        script = self.base / "fake-git.py"
        script.write_text('''import base64,json,sys,time
v=json.load(open(sys.argv[1]))
time.sleep(v['delay'])
sys.stderr.buffer.write(base64.b64decode(v['stderr']));sys.stderr.buffer.flush()
for expected,block in zip(v['requests'],v['blocks']):
 actual=sys.stdin.buffer.readline()
 if actual != expected.encode()+b'\\n': raise SystemExit(97)
 sys.stdout.buffer.write(base64.b64decode(block));sys.stdout.buffer.flush()
sys.stdout.buffer.write(base64.b64decode(v['trailing']));sys.stdout.buffer.flush()
raise SystemExit(v['exit'])
''')
        with mock.patch.object(batch, "COMMAND", (sys.executable, "-B", str(script), str(configuration))):
            yield

    def assert_rejected_transcript(self, blocks, error, **kwargs):
        with self.fake_git(blocks, **kwargs):
            with self.assertRaisesRegex(ValueError, error):
                self.verify(originals=self.originals)
        receipt = json.loads((self.originals / "result.original.json").read_bytes())
        self.assertEqual(receipt["status"], "SOURCE_UNVERIFIED")
        self.assertFalse(receipt["formalAcceptanceGranted"])
        self.assertFalse(receipt["nativeExecution"])
        self.assertEqual(receipt["process"]["groupAfterCleanup"]["state"], "ABSENT")

    def test_actual_public_verifier_uses_one_git_process(self):
        self.verify()
        self.assertEqual(len(self.calls), 1,
            "the unchanged physical/source check must use one Git process inside its original 10 seconds")
        self.assertEqual(self.calls[0], ["git", "--no-replace-objects", "cat-file", "--batch"])

    def test_all_original_45_paths_and_new_execution_closure_are_preserved(self):
        self.assertEqual(len(ORIGINAL_SOURCE_FILES), 45)
        self.assertTrue(set(ORIGINAL_SOURCE_FILES).issubset(production.SOURCE_FILES))
        self.assertIn("tools/shortcuts-simulator-qa/git_source_batch.py", production.SOURCE_FILES)
        self.assertIn("tools/shortcuts-simulator-qa/test_git_source_batch.py", production.SOURCE_FILES)

    def test_public_entrypoint_retains_original_ten_second_allowance(self):
        seen = []
        original = batch.verify_raw_sources
        def observe(*args, **kwargs):
            seen.append(kwargs["deadline"] - time.monotonic())
            return original(*args, **kwargs)
        with mock.patch.object(batch, "verify_raw_sources", observe):
            self.verify()
        self.assertEqual(len(seen), 1)
        self.assertGreater(seen[0], 9)
        self.assertLessEqual(seen[0], 10)

    def test_linked_worktree_uses_real_common_git_objects(self):
        linked = self.base / "linked"
        self.git("worktree", "add", "--detach", str(linked), self.source)
        self.root = linked
        owner = linked / "tools/shortcuts-simulator-qa/qa_terminal_receipts.py"
        spec = importlib.util.spec_from_file_location("linked_terminal_fixture", owner)
        self.module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(self.module)
        self.verify()

    def test_graft_change_during_same_batch_is_rejected(self):
        original = batch.Batch.object
        def change(instance, request, *args):
            result = original(instance, request, *args)
            if request == self.source + ":" + production.SOURCE_FILES[-1]:
                (self.root / ".git/info/grafts").write_bytes(b"synthetic view change\n")
            return result
        with mock.patch.object(batch.Batch, "object", change):
            with self.assertRaisesRegex(ValueError, "SOURCE_GIT_REDIRECT"):
                self.verify()

    def test_launch_failure_retains_no_fabricated_pid(self):
        with mock.patch.object(subprocess, "Popen", side_effect=OSError("synthetic launch failure")):
            with self.assertRaises(OSError):
                self.module.verify_checkout(self.root, self.source, originals=self.originals)
        value = json.loads((self.originals / "result.original.json").read_bytes())
        self.assertEqual(value["status"], "SOURCE_UNVERIFIED")
        self.assertIsNone(value["process"]["pid"])
        self.assertIsNone(value["process"]["parentReaped"])
        self.assertEqual((self.originals / "stdout.original").read_bytes(), b"")

    def test_healthy_raw_transcript_and_closing_head_are_retained(self):
        value = self.verify(originals=self.originals)
        self.assertEqual(value["status"], "SOURCE_VERIFIED")
        self.assertEqual(value["headBefore"], self.source)
        self.assertEqual(value["headAfter"], self.source)
        self.assertFalse(value["completeTrackedSource"])
        self.assertEqual(len(value["sourceFiles"]), len(production.SOURCE_FILES))
        requests, blocks = self.transcript()
        self.assertEqual((self.originals / "stdin.original").read_bytes(), ("\n".join(requests) + "\n").encode())
        self.assertEqual((self.originals / "stdout.original").read_bytes(), b"".join(blocks))
        self.assertEqual((self.originals / "stderr.original").read_bytes(), b"")
        self.assertTrue(value["process"]["parentReaped"])
        self.assertEqual(value["process"]["groupAfterCleanup"]["state"], "ABSENT")

    def test_every_physical_source_remains_required(self):
        for relative in production.SOURCE_FILES:
            with self.subTest(path=relative):
                path = self.root / relative
                original = path.read_bytes()
                try:
                    path.write_bytes(original + b"source mutation\n")
                    with self.assertRaisesRegex(ValueError, "ACQUISITION_SOURCE_BYTES"):
                        self.verify()
                finally:
                    path.write_bytes(original)

    def test_wrong_head_is_rejected(self):
        (self.root / "other.txt").write_bytes(b"other commit\n")
        self.git("add", ".")
        self.commit("different current HEAD")
        with self.assertRaisesRegex(ValueError, "CHECKOUT_SOURCE_MISMATCH"):
            self.verify()

    def test_actual_head_change_during_same_batch_is_rejected(self):
        old = self.source
        (self.root / "other.txt").write_bytes(b"other commit\n")
        self.git("add", ".")
        self.commit("future HEAD")
        other = self.git("rev-parse", "HEAD").decode().strip()
        self.git("reset", "--hard", old)
        original = batch.Batch.object
        def change(instance, request, *args):
            result = original(instance, request, *args)
            if request == self.source + ":" + production.SOURCE_FILES[-1]:
                self.git("update-ref", "HEAD", other, old)
            return result
        with mock.patch.object(batch.Batch, "object", change):
            with self.assertRaisesRegex(ValueError, "CHECKOUT_SOURCE_MISMATCH"):
                self.verify()

    def test_physical_change_during_same_batch_is_rejected(self):
        original = batch.Batch.object
        target = production.SOURCE_FILES[0]
        def change(instance, request, *args):
            result = original(instance, request, *args)
            if request == self.source + ":" + target:
                (self.root / target).write_bytes(b"changed after Git response\n")
            return result
        with mock.patch.object(batch.Batch, "object", change):
            with self.assertRaisesRegex(ValueError, "SOURCE_PHYSICAL_CHANGED"):
                self.verify()

    def test_executable_change_during_same_batch_is_rejected(self):
        original = batch.Batch.object
        target = production.SOURCE_FILES[0]
        def change(instance, request, *args):
            result = original(instance, request, *args)
            if request == self.source + ":" + target:
                (self.root / target).chmod(0o755)
            return result
        with mock.patch.object(batch.Batch, "object", change):
            with self.assertRaisesRegex(ValueError, "SOURCE_PHYSICAL_CHANGED"):
                self.verify()

    def test_missing_and_symlink_physical_paths_fail_closed(self):
        target = self.root / production.SOURCE_FILES[0]
        original = target.read_bytes()
        target.unlink()
        with self.assertRaisesRegex(ValueError, "ORIGINAL_UNAVAILABLE"):
            self.verify()
        outside = self.base / "outside.txt"
        outside.write_bytes(original)
        target.symlink_to(outside)
        with self.assertRaisesRegex(ValueError, "ORIGINAL_SYMLINK"):
            self.verify()

    def test_file_size_limit_remains_four_mib(self):
        target = self.root / production.SOURCE_FILES[0]
        target.write_bytes(b"x" * (batch.FILE_LIMIT + 1))
        with self.assertRaisesRegex(ValueError, "SOURCE_FILE_SIZE"):
            self.verify()

    def test_export_attributes_do_not_transform_raw_source(self):
        target = self.root / production.SOURCE_FILES[0]
        target.write_bytes(b"literal $Format:%H$ remains literal\n")
        (self.root / ".gitattributes").write_bytes(b"* export-subst export-ignore\n")
        self.git("add", ".")
        self.commit("archive filters")
        self.source = self.git("rev-parse", "HEAD").decode().strip()
        self.verify()
        target.write_bytes(("literal " + self.source + " remains literal\n").encode())
        with self.assertRaisesRegex(ValueError, "ACQUISITION_SOURCE_BYTES"):
            self.verify()

    def test_replacement_objects_are_disabled(self):
        old = self.source
        target = self.root / production.SOURCE_FILES[0]
        target.write_bytes(b"replacement object source\n")
        self.git("add", ".")
        self.commit("replacement")
        other = self.git("rev-parse", "HEAD").decode().strip()
        self.git("reset", "--hard", old)
        self.git("replace", old, other)
        self.verify()
        target.write_bytes(b"replacement object source\n")
        with self.assertRaisesRegex(ValueError, "ACQUISITION_SOURCE_BYTES"):
            self.verify()

    def test_git_environment_redirect_is_rejected_before_spawn(self):
        for name in batch.REDIRECT_ENV:
            with self.subTest(name=name), mock.patch.dict(os.environ, {name: "synthetic-redirect"}):
                with self.assertRaisesRegex(ValueError, "SOURCE_GIT_REDIRECT"):
                    self.verify()
        self.assertEqual(self.calls, [])

    def test_graft_or_alternate_view_is_rejected_before_spawn(self):
        for relative in ("info/grafts", "objects/info/alternates"):
            with self.subTest(path=relative):
                path = self.root / ".git" / relative
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_bytes(b"synthetic redirect\n")
                try:
                    with self.assertRaisesRegex(ValueError, "SOURCE_GIT_REDIRECT"):
                        self.verify()
                finally:
                    path.unlink()
        self.assertEqual(self.calls, [])

    def test_binary_blob_is_compared_as_exact_bytes(self):
        (self.root / production.SOURCE_FILES[0]).write_bytes(bytes(range(256)) + b"\x00\xff\n")
        self.git("add", ".")
        self.commit("binary source")
        self.source = self.git("rev-parse", "HEAD").decode().strip()
        self.verify()

    def test_missing_batch_object_is_rejected(self):
        _, blocks = self.transcript()
        blocks[1] = (self.source + ":" + production.SOURCE_FILES[0] + " missing\n").encode()
        self.assert_rejected_transcript(blocks, "SOURCE_BATCH_HEADER")

    def test_truncated_batch_object_is_rejected(self):
        _, blocks = self.transcript()
        self.assert_rejected_transcript(blocks[:-1] + [blocks[-1][:-1]], "SOURCE_BATCH_TRUNCATED")

    def test_trailing_stdout_is_rejected(self):
        _, blocks = self.transcript()
        self.assert_rejected_transcript(blocks, "SOURCE_BATCH_TRAILING", trailing=b"unrequested bytes\n")

    def test_wrong_object_type_is_rejected(self):
        _, blocks = self.transcript()
        blocks[1] = blocks[1].replace(b" blob ", b" tree ", 1)
        self.assert_rejected_transcript(blocks, "SOURCE_BATCH_HEADER")

    def test_raw_object_digest_is_recomputed(self):
        _, blocks = self.transcript()
        blocks[1] = b"0" * 40 + blocks[1][40:]
        self.assert_rejected_transcript(blocks, "SOURCE_BATCH_OBJECT")

    def test_missing_object_terminator_is_rejected(self):
        _, blocks = self.transcript()
        blocks[1] = blocks[1][:-1] + b"!"
        self.assert_rejected_transcript(blocks, "SOURCE_BATCH_TERMINATOR")

    def test_oversized_header_is_rejected_before_reading_body(self):
        _, blocks = self.transcript()
        blocks[1] = blocks[1][:40] + b" blob 999999999\n"
        self.assert_rejected_transcript(blocks, "SOURCE_BATCH_SIZE")

    def test_stderr_overflow_is_retained_and_rejected(self):
        _, blocks = self.transcript()
        self.assert_rejected_transcript(blocks, "SOURCE_BATCH_OUTPUT_OVERFLOW", stderr=b"x" * (batch.STDERR_LIMIT + 4096))
        self.assertEqual((self.originals / "stderr.original").stat().st_size, batch.STDERR_LIMIT + 1)

    def test_stdout_overflow_is_retained_and_rejected(self):
        _, blocks = self.transcript()
        original = batch.Batch.__init__
        def bounded(instance, root, deadline, stdout_limit):
            original(instance, root, deadline, 32)
        with mock.patch.object(batch.Batch, "__init__", bounded):
            self.assert_rejected_transcript(blocks, "SOURCE_BATCH_OUTPUT_OVERFLOW")
        self.assertEqual((self.originals / "stdout.original").stat().st_size, 33)

    def test_process_nonzero_is_not_source_success(self):
        _, blocks = self.transcript()
        self.assert_rejected_transcript(blocks, "SOURCE_BATCH_EXIT", exit_code=23)
        value = json.loads((self.originals / "result.original.json").read_bytes())
        self.assertEqual(value["process"]["exitCode"], 23)

    def test_closing_head_digest_must_match_start(self):
        _, blocks = self.transcript()
        body = b"tree " + b"0" * 40 + b"\n\nchanged commit\n"
        object_id = hashlib.sha1(b"commit " + str(len(body)).encode() + b"\0" + body).hexdigest()
        blocks[-1] = object_id.encode() + b" commit " + str(len(body)).encode() + b"\n" + body + b"\n"
        self.assert_rejected_transcript(blocks, "CHECKOUT_SOURCE_MISMATCH")

    def test_actual_deadline_includes_all_object_requests(self):
        original = batch.Batch.object
        def delayed(instance, *args):
            time.sleep(.06)
            return original(instance, *args)
        start = time.monotonic()
        with mock.patch.object(batch.Batch, "object", delayed), mock.patch.object(subprocess, "Popen", side_effect=self.record):
            with self.assertRaisesRegex(ValueError, "SOURCE_GUARD_DEADLINE"):
                batch.verify_raw_sources(self.root, self.source, production.SOURCE_FILES,
                    deadline=start + .15, originals=self.originals)
        self.assertLess(time.monotonic() - start, 2)
        value = json.loads((self.originals / "result.original.json").read_bytes())
        self.assertEqual(value["status"], "SOURCE_UNVERIFIED")

    def test_cancellation_during_popen_registration_retains_owned_pid(self):
        def interrupt(argv, *args, **kwargs):
            process = self.record(argv, *args, **kwargs)
            os.kill(os.getpid(), signal.SIGALRM)
            return process
        with mock.patch.object(subprocess, "Popen", side_effect=interrupt):
            with self.assertRaises(batch.SourceInterrupted):
                self.module.verify_checkout(self.root, self.source, originals=self.originals)
        value = json.loads((self.originals / "result.original.json").read_bytes())
        self.assertEqual(value["status"], "SOURCE_UNVERIFIED")
        self.assertEqual(value["process"]["pid"], self.processes[0].pid)
        self.assertTrue(value["process"]["parentReaped"])
        self.assertEqual(value["process"]["groupAfterCleanup"]["state"], "ABSENT")

    def test_unknown_group_cannot_authenticate_zero_exit(self):
        import project_prerequisite_observation as capture
        unknown = {"state": "PERMISSION_DENIED", "present": None, "errno": 1}
        with mock.patch.object(capture, "group_observation", return_value=unknown):
            with self.assertRaisesRegex(ValueError, "SOURCE_BATCH_PROCESS_CLEANUP"):
                self.verify(originals=self.originals)
        value = json.loads((self.originals / "result.original.json").read_bytes())
        self.assertEqual(value["process"]["exitCode"], 0)
        self.assertEqual(value["status"], "SOURCE_UNVERIFIED")

    def test_original_receipts_are_create_only(self):
        original = b"earlier immutable original\n"
        (self.originals / "stdin.original").write_bytes(original)
        with self.assertRaises(FileExistsError):
            self.verify(originals=self.originals)
        self.assertEqual((self.originals / "stdin.original").read_bytes(), original)

    def test_bootstrap_without_builds_retains_source_before_context_failure(self):
        import prepare_public_artifacts as public
        for mode in ("checkpoint", "output-context", "collect", "receiver-path", "finalize"):
            with self.subTest(mode=mode):
                artifacts = self.base / ("bootstrap-" + mode)
                artifacts.mkdir()
                args = types.SimpleNamespace(repo=str(self.root), root=str(artifacts),
                    source_sha=self.source, mode=mode, fixture_id=None, run_id=artifacts.name)
                with mock.patch.object(public, "trusted_fixture_sources", return_value={}), \
                     mock.patch.object(public, "verify_synthetic_sources",
                         side_effect=ValueError("SYNTHETIC_SOURCE_INVALID")):
                    with self.assertRaisesRegex(ValueError, "SYNTHETIC_SOURCE_INVALID"):
                        self.module._cli_context(args)
                receipts = list((artifacts / "builds").glob("qa-source-*/result.original.json"))
                self.assertEqual(len(receipts), 1)
                value = json.loads(receipts[0].read_bytes())
                self.assertEqual(value["status"], "SOURCE_VERIFIED")
                self.assertFalse(value["formalAcceptanceGranted"])
                self.assertTrue(value["process"]["parentReaped"])
                self.assertEqual(value["process"]["groupAfterCleanup"]["state"], "ABSENT")
                self.assertEqual((artifacts / "builds").stat().st_mode & 0o777, 0o700)
                self.assertFalse((artifacts / "acquisition-status.json").exists())

    def test_bootstrap_builds_symlink_is_rejected_before_spawn(self):
        artifacts = self.base / "bootstrap-link"
        artifacts.mkdir()
        outside = self.base / "foreign-private"
        outside.mkdir()
        (artifacts / "builds").symlink_to(outside, target_is_directory=True)
        args = types.SimpleNamespace(repo=str(self.root), root=str(artifacts),
            source_sha=self.source, mode="finalize")
        with mock.patch.object(subprocess, "Popen", side_effect=self.record):
            with self.assertRaisesRegex(ValueError, "ORIGINAL_SYMLINK"):
                self.module._cli_context(args)
        self.assertEqual(self.calls, [])
        self.assertEqual(list(outside.iterdir()), [])

    def test_bootstrap_builds_file_is_rejected_without_replacement(self):
        artifacts = self.base / "bootstrap-file"
        artifacts.mkdir()
        original = b"existing original, not a directory\n"
        (artifacts / "builds").write_bytes(original)
        args = types.SimpleNamespace(repo=str(self.root), root=str(artifacts),
            source_sha=self.source, mode="finalize")
        with mock.patch.object(subprocess, "Popen", side_effect=self.record):
            with self.assertRaises((ValueError, OSError)):
                self.module._cli_context(args)
        self.assertEqual(self.calls, [])
        self.assertEqual((artifacts / "builds").read_bytes(), original)

    def test_metadata_and_cleanup_errors_preserve_first_outcome_without_pid(self):
        with mock.patch.dict(os.environ, {"GIT_DIR": "synthetic-redirect"}), \
             mock.patch.object(batch.Batch, "finish", side_effect=RuntimeError("cleanup observation failed")):
            with self.assertRaisesRegex(ValueError, "SOURCE_GIT_REDIRECT"):
                self.verify(originals=self.originals)
        value = json.loads((self.originals / "result.original.json").read_bytes())
        self.assertEqual(value["error"], "SOURCE_GIT_REDIRECT")
        self.assertEqual(value["cleanupError"], "RuntimeError")
        self.assertEqual(value["status"], "SOURCE_UNVERIFIED")
        self.assertIsNone(value["process"]["pid"])
        self.assertIsNone(value["process"]["parentReaped"])
        self.assertEqual(self.calls, [])
        for name in ("stdin", "stdout", "stderr"):
            self.assertEqual((self.originals / (name + ".original")).read_bytes(), b"")

    def test_launch_and_cleanup_errors_preserve_first_outcome_without_pid(self):
        with mock.patch.object(subprocess, "Popen", side_effect=OSError("synthetic launch failure")), \
             mock.patch.object(batch.Batch, "finish", side_effect=RuntimeError("cleanup observation failed")):
            with self.assertRaisesRegex(OSError, "synthetic launch failure"):
                self.module.verify_checkout(self.root, self.source, originals=self.originals)
        value = json.loads((self.originals / "result.original.json").read_bytes())
        self.assertEqual(value["error"], "OSError")
        self.assertEqual(value["cleanupError"], "RuntimeError")
        self.assertEqual(value["status"], "SOURCE_UNVERIFIED")
        self.assertIsNone(value["process"]["pid"])
        self.assertIsNone(value["process"]["parentReaped"])

    def test_cleanup_error_after_zero_exit_retains_transcript_without_authentication(self):
        original = batch.Batch.finish
        def finish(instance):
            original(instance)
            raise RuntimeError("synthetic final cleanup observation failure")
        with mock.patch.object(batch.Batch, "finish", finish):
            with self.assertRaisesRegex(RuntimeError, "synthetic final cleanup"):
                self.verify(originals=self.originals)
        value = json.loads((self.originals / "result.original.json").read_bytes())
        self.assertEqual(value["error"], "RuntimeError")
        self.assertEqual(value["cleanupError"], "RuntimeError")
        self.assertEqual(value["status"], "SOURCE_UNVERIFIED")
        self.assertEqual(value["process"]["exitCode"], 0)
        self.assertTrue(value["process"]["parentReaped"])
        self.assertEqual(value["process"]["groupAfterCleanup"]["state"], "ABSENT")
        self.assertGreater((self.originals / "stdout.original").stat().st_size, 0)

    def test_unstarted_cleanup_does_not_import_process_observer(self):
        with mock.patch.dict(sys.modules, {"project_prerequisite_observation": None}), \
             mock.patch.object(subprocess, "Popen", side_effect=OSError("synthetic launch failure")):
            with self.assertRaisesRegex(OSError, "synthetic launch failure"):
                self.module.verify_checkout(self.root, self.source, originals=self.originals)
        value = json.loads((self.originals / "result.original.json").read_bytes())
        self.assertEqual(value["error"], "OSError")
        self.assertIsNone(value.get("cleanupError"))
        self.assertIsNone(value["process"]["pid"])


if __name__ == "__main__":
    unittest.main()
