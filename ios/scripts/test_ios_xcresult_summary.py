#!/usr/bin/env python3
"""Offline controls for the new diagnostic protocol; no native calls or keys."""
import copy
import importlib.util
import io
import json
import os
from pathlib import Path
import shutil
import stat
import subprocess
import sys
import tempfile
import time
import unittest
from unittest import mock
import zipfile

HERE = Path(__file__).resolve().parent
REPO = HERE.parents[1]
CANARY = "PRIVATE_XCRESULT_CANARY_path_nonce_message"


def load(name):
    spec = importlib.util.spec_from_file_location("_test_" + name, HERE / (name + ".py"))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


subject = load("collect_ios_xcresult_summary")
consumer = load("verify_ios_xcresult_summary")


def git(repo, *args):
    run = subprocess.run(["/usr/bin/git", "-C", str(repo), *args],
                         stdin=subprocess.DEVNULL, stdout=subprocess.PIPE,
                         stderr=subprocess.PIPE, timeout=10)
    if run.returncode:
        raise AssertionError("SYNTHETIC_GIT_SETUP_FAILED")
    return run.stdout.decode().strip()


def fixture(parent):
    repo = parent / "repo"
    repo.mkdir()
    for name in subject.SOURCE_FILES:
        target = repo / name
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(REPO / name, target)
    git(repo, "init", "-q")
    git(repo, "config", "user.name", "Synthetic protocol control")
    git(repo, "config", "user.email", "synthetic@example.invalid")
    git(repo, "add", ".")
    git(repo, "commit", "-qm", "Synthetic source fixture")
    context = {"repository": subject.REPOSITORY, "workflowPath": subject.WORKFLOW,
               "workflowName": subject.WORKFLOW_NAME, "job": "ios-build",
               "event": "workflow_dispatch", "sourceSha": git(repo, "rev-parse", "HEAD"),
               "runId": 12345, "attempt": 1}
    root = parent / subject.root_name(context)
    sources = subject.source_binding(repo, context["sourceSha"], time.monotonic() + 15)
    reservation = subject.prepare(repo, context, root, deadline=time.monotonic() + 15)
    return repo, context, root, sources, reservation


def complete_native(repo, root):
    (root / "tests.xcresult").mkdir(mode=0o700)
    (repo / "ios/xcodetest.exitcode").write_bytes(b"65\n")


def fake_xcrun(parent, body):
    directory = parent / "bin"
    directory.mkdir(exist_ok=True)
    path = directory / "xcrun"
    path.write_text("#!" + sys.executable + "\nimport os,sys,time,pathlib\n"
                    "assert sys.argv[1:6] == ['xcresulttool','get','test-results','summary','--path']\n"
                    "assert pathlib.Path(sys.argv[6]).is_dir()\n" + body + "\n")
    path.chmod(0o700)
    return mock.patch.dict(os.environ, {"PATH": str(directory) + os.pathsep + os.environ.get("PATH", "")})


def outer_zip(receipt, cipher=None):
    stream = io.BytesIO()
    with zipfile.ZipFile(stream, "w", compression=zipfile.ZIP_DEFLATED) as archive:
        archive.writestr("receipt.json", subject.canonical(receipt))
        if cipher is not None:
            archive.writestr("ios-xcresult-summary.zip.p7m", cipher)
    return stream.getvalue()


def synthetic_encrypt(private, public, archive, archive_binding, recipient, deadline):
    """A named crypto boundary stub; the suite's positive setup encrypts for real."""
    encoded = b"\x30SYNTHETIC_CONTROL_ONLY"
    (public / "project-prerequisite.zip.p7m").write_bytes(encoded)
    return subject.digest_record(encoded)


class TestIOSXCResultSummary(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.owned = tempfile.TemporaryDirectory(prefix="ios-xcresult-controls-")
        cls.parent = Path(cls.owned.name).resolve()
        cls.repo, cls.context, cls.root, cls.sources, cls.reservation = fixture(cls.parent)
        complete_native(cls.repo, cls.root)
        with fake_xcrun(cls.parent, "sys.stdout.write(" + repr('{"private":"' + CANARY + '"}\n') + ")"):
            cls.code, cls.receipt = subject.collect(
                cls.repo, cls.context, cls.root, cls.reservation)
        if cls.code != 0:
            raise AssertionError("SYNTHETIC_SEAL_SETUP_FAILED")
        cls.cipher = (cls.root / "public/ios-xcresult-summary.zip.p7m").read_bytes()
        cls.plain = (cls.root / "private/ios-xcresult-summary.zip").read_bytes()
        cls.outer = outer_zip(cls.receipt, cls.cipher)
        cls.artifact = cls.parent / "artifact.original.zip"
        cls.artifact.write_bytes(cls.outer)
        cls.plaintext = cls.parent / "synthetic-known-plaintext.zip"
        cls.plaintext.write_bytes(cls.plain)
        # Independent synthetic API/source witness, not values copied from receipt.
        cls.expected = {
            "schema": consumer.EXPECTED_SCHEMA, "context": cls.context, "workflowId": 777,
            "artifact": {"id": 888, "name": subject.ARTIFACT_NAME,
                         **subject.digest_record(cls.outer), "runId": cls.context["runId"],
                         "attempt": cls.context["attempt"], "sourceSha": cls.context["sourceSha"]},
            "sources": cls.sources,
        }

    @classmethod
    def tearDownClass(cls):
        cls.owned.cleanup()

    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="ios-xcresult-case-")
        self.addCleanup(self.temp.cleanup)
        self.path = Path(self.temp.name).resolve()

    def new_fixture(self):
        return fixture(self.path)

    def candidate(self, receipt=None, plain=None, cipher=None):
        receipt = copy.deepcopy(self.receipt if receipt is None else receipt)
        cipher = self.cipher if cipher is None else cipher
        if plain is not None:
            receipt["plaintextArchive"] = subject.digest_record(plain)
        raw = outer_zip(receipt, cipher)
        artifact = self.path / "candidate.zip"
        artifact.write_bytes(raw)
        expected = copy.deepcopy(self.expected)
        expected["artifact"].update(subject.digest_record(raw))
        plaintext = self.path / "candidate-private.zip"
        plaintext.write_bytes(self.plain if plain is None else plain)
        return artifact, plaintext, expected

    def test_real_capture_seal_and_independent_private_intake(self):
        self.assertEqual(self.code, 0)
        self.assertEqual(self.receipt["disposition"], "SEALED_COMPLETE")
        result = consumer.private_intake(self.artifact, self.plaintext, self.expected, subject)
        self.assertTrue(result["completeSummaryJsonObject"])
        self.assertEqual(result["nativeExitCode"], 65)
        self.assertEqual(result["memberCount"], 6)
        self.assertEqual(result["issueSemantics"], "UNOBSERVED")
        self.assertFalse(result["formalAcceptanceGranted"])
        self.assertNotIn(CANARY.encode(), subject.canonical(self.receipt))
        self.assertNotIn(CANARY.encode(), self.cipher)
        self.assertIn(CANARY.encode(), self.plain)
        self.assertEqual(set(p.name for p in (self.root / "public").iterdir()), subject.PUBLIC_FILES)

    def test_authenticated_expectations_precede_private_read(self):
        cases = []
        for field, value in [("runId", 54321), ("attempt", 2), ("event", "push"),
                             ("workflowPath", ".github/workflows/other.yml")]:
            value_expected = copy.deepcopy(self.expected)
            value_expected["context"][field] = value
            if field in {"runId", "attempt"}:
                value_expected["artifact"][field] = value
            cases.append(value_expected)
        wrong_digest = copy.deepcopy(self.expected)
        wrong_digest["artifact"]["sha256"] = "0" * 64
        cases.append(wrong_digest)
        wrong_source = copy.deepcopy(self.expected)
        wrong_source["sources"]["files"][subject.SOURCE_FILES[0]]["sha256"] = "0" * 64
        cases.append(wrong_source)
        for index, expected in enumerate(cases):
            with self.subTest(case=index):
                reads = []
                original = subject.read_regular
                def observed(path, limit):
                    reads.append(Path(path))
                    return original(path, limit)
                with mock.patch.object(subject, "read_regular", side_effect=observed):
                    with self.assertRaises((ValueError, TypeError)):
                        consumer.private_intake(self.artifact, self.plaintext, expected, subject)
                self.assertNotIn(self.plaintext, reads)

    def test_consumer_import_requires_independent_source_hash(self):
        loaded = consumer.load_protocol(self.expected)
        self.assertEqual(loaded.SCHEMA, subject.SCHEMA)
        for name in (consumer.PROTOCOL_SOURCE, consumer.VERIFIER_SOURCE):
            expected = copy.deepcopy(self.expected)
            expected["sources"]["files"][name]["sha256"] = "0" * 64
            with self.assertRaises(ValueError):
                consumer.load_protocol(expected)

    def test_receipt_cannot_substitute_identity_or_old_protocol(self):
        mutations = [
            lambda r: r.update(schema="cherri-project-prerequisite-observation/3"),
            lambda r: r["context"].update(attempt=2),
            lambda r: r["context"].update(sourceSha="0" * 40),
            lambda r: r.update(recipientSha256="0" * 64),
            lambda r: r["sources"]["files"][subject.HELPER_FILES[0]].update(sha256="0" * 64),
            lambda r: r.update(formalAcceptanceGranted=True),
        ]
        for index, change in enumerate(mutations):
            with self.subTest(case=index):
                receipt = copy.deepcopy(self.receipt)
                change(receipt)
                artifact, plaintext, expected = self.candidate(receipt)
                with self.assertRaises((ValueError, TypeError)):
                    consumer.private_intake(artifact, plaintext, expected, subject)

    def test_preexisting_exit_or_reservation_is_rejected(self):
        repo, context, root, sources, reservation = self.new_fixture()
        with self.assertRaises(ValueError):
            subject.prepare(repo, context, root, deadline=time.monotonic() + 15)
        other_context = {**context, "attempt": 2}
        (repo / "ios/xcodetest.exitcode").write_bytes(b"65\n")
        other = self.path / subject.root_name(other_context)
        with self.assertRaisesRegex(ValueError, "NATIVE_EXIT_INVALID"):
            subject.prepare(repo, other_context, other, deadline=time.monotonic() + 15)
        self.assertFalse(other.exists())

    def test_marker_hash_parent_mode_inode_and_symlink(self):
        repo, context, root, sources, expected = self.new_fixture()
        marker = root / "private/reservation.json"
        original = marker.read_bytes()
        marker.write_bytes(original + b" ")
        with self.assertRaises(ValueError):
            subject.reservation(repo, context, root, expected)
        marker.write_bytes(original)
        root.chmod(0o755)
        with self.assertRaises(ValueError):
            subject.reservation(repo, context, root, expected)
        root.chmod(0o700)
        retained = self.path / "original-root"
        root.rename(retained)
        root.mkdir(mode=0o700)
        shutil.copytree(retained / "private", root / "private")
        with self.assertRaises(ValueError):
            subject.reservation(repo, context, root, expected)
        shutil.rmtree(root)
        root.symlink_to(retained, target_is_directory=True)
        with self.assertRaises(ValueError):
            subject.reservation(repo, context, root, expected)

    def test_source_and_transitive_import_reject_changed_bytes(self):
        repo, context, root, sources, expected = self.new_fixture()
        paths = [subject.HELPER_FILES[0], subject.RECIPIENT,
                 "ios/CherriUITests/CherriEditorUITests.swift"]
        for name in paths:
            target = repo / name
            original = target.read_bytes()
            target.write_bytes(original + b"\nPRIVATE_SOURCE_MUTATION\n")
            with self.assertRaises(ValueError):
                subject.source_binding(repo, context["sourceSha"], time.monotonic() + 15)
            if name in subject.HELPER_FILES:
                with self.assertRaises(ValueError):
                    subject.load_primitives(repo, sources)
            target.write_bytes(original)

    def test_mutation_during_capture_is_receipt_only(self):
        repo, context, root, sources, expected = self.new_fixture()
        complete_native(repo, root)
        target = repo / "ios/CherriUITests/CherriEditorUITests.swift"
        body = "pathlib.Path(" + repr(str(target)) + ").write_bytes(b'PRIVATE_CHANGED')\n"
        body += "sys.stdout.write('{}\\n')"
        with fake_xcrun(self.path, body):
            code, receipt = subject.collect(repo, context, root, expected)
        self.assertEqual(code, 70)
        self.assertEqual(receipt["disposition"], "REJECTED")
        self.assertIsNone(receipt["ciphertext"])
        self.assertIsNone(receipt["plaintextArchive"])
        self.assertEqual(set(p.name for p in (root / "public").iterdir()), {"receipt.json"})
        self.assertNotIn(b"PRIVATE_CHANGED", subject.canonical(receipt))

    def test_actual_helper_exact_cap_and_detection_byte(self):
        helper = subject.load_primitives(self.repo, self.sources)
        for extra in (0, 1):
            out, err = io.BytesIO(), io.BytesIO()
            observed, detail = helper.capture(
                [sys.executable, "-c", "import os;os.write(1,b'x'*" +
                 str(subject.STREAM_LIMIT + extra) + ")"],
                3, output=out, error_output=err, hard_deadline=time.monotonic() + 3,
                stream_limit=subject.STREAM_LIMIT)
            self.assertEqual(len(out.getvalue()), subject.STREAM_LIMIT + extra)
            self.assertEqual(observed["result"], "PASS" if extra == 0 else "OUTPUT_OVERFLOW")
            self.assertEqual(detail["stdout"]["overflow"], bool(extra))

    def test_collector_overflow_never_seals_or_clips_original(self):
        repo, context, root, sources, expected = self.new_fixture()
        complete_native(repo, root)
        with fake_xcrun(self.path, "os.write(1,b'x'*" + str(subject.STREAM_LIMIT + 1) + ")"):
            code, receipt = subject.collect(repo, context, root, expected)
        self.assertEqual(code, 70)
        self.assertEqual(receipt["rejectionCode"], "CAPTURE_OVERFLOW")
        self.assertEqual((root / "private/xcresult-summary.stdout").stat().st_size,
                         subject.STREAM_LIMIT + 1)
        self.assertEqual(set(p.name for p in (root / "public").iterdir()), {"receipt.json"})

    def test_actual_timeout_nonzero_and_held_pipe_never_pass(self):
        helper = subject.load_primitives(self.repo, self.sources)
        cases = [
            ("import time;time.sleep(5)", 0.15),
            ("raise SystemExit(65)", 2),
            ("import os,time\nif os.fork()==0: time.sleep(5)\n", 0.2),
        ]
        for code, allowance in cases:
            observed, details = helper.capture(
                [sys.executable, "-c", code], allowance,
                output=io.BytesIO(), error_output=io.BytesIO(),
                hard_deadline=time.monotonic() + allowance, stream_limit=1024)
            self.assertNotEqual(observed["result"], "PASS")

    def test_encryption_failure_is_fixed_and_has_no_cipher(self):
        repo, context, root, sources, expected = self.new_fixture()
        complete_native(repo, root)
        real_load = subject.load_primitives
        def failing_load(*args):
            helper = real_load(*args)
            helper.encrypt_archive = mock.Mock(side_effect=RuntimeError(CANARY))
            return helper
        with fake_xcrun(self.path, "sys.stdout.write('{}\\n')"), \
                mock.patch.object(subject, "load_primitives", side_effect=failing_load):
            code, receipt = subject.collect(repo, context, root, expected)
        self.assertEqual(code, 70)
        self.assertEqual(receipt["rejectionCode"], "ENCRYPTION_REJECTED")
        self.assertEqual(set(p.name for p in (root / "public").iterdir()), {"receipt.json"})
        self.assertNotIn(CANARY.encode(), subject.canonical(receipt))


    def test_missing_invalid_exit_and_result_child_never_launch(self):
        cases = [
            ("missing-exit", None, "directory"),
            ("empty-exit", b"", "directory"),
            ("missing-newline", b"65", "directory"),
            ("out-of-range", b"256\n", "directory"),
            ("multiple-exits", b"65\n0\n", "directory"),
            ("missing-result", b"65\n", "missing"),
            ("symlink-result", b"65\n", "symlink"),
        ]
        for index, (name, exit_bytes, result_kind) in enumerate(cases):
            with self.subTest(case=name):
                parent = self.path / str(index)
                parent.mkdir()
                repo, context, root, sources, expected = fixture(parent)
                if result_kind == "directory":
                    (root / "tests.xcresult").mkdir(mode=0o700)
                elif result_kind == "symlink":
                    target = parent / "other-result"
                    target.mkdir()
                    (root / "tests.xcresult").symlink_to(target, target_is_directory=True)
                if exit_bytes is not None:
                    (repo / "ios/xcodetest.exitcode").write_bytes(exit_bytes)
                helper = subject.load_primitives(repo, sources)
                with mock.patch.object(subject, "load_primitives", return_value=helper), \
                        mock.patch.object(helper, "capture") as capture:
                    code, receipt = subject.collect(repo, context, root, expected)
                capture.assert_not_called()
                self.assertEqual(code, 70)
                self.assertEqual(receipt["disposition"], "REJECTED")
                self.assertIsNone(receipt["ciphertext"])
                self.assertIsNone(receipt["plaintextArchive"])
                self.assertEqual(set(p.name for p in (root / "public").iterdir()), {"receipt.json"})
                if result_kind == "directory":
                    self.assertEqual(receipt["rejectionCode"], "NATIVE_EXIT_INVALID")

    def test_exact_cap_collector_and_incomplete_private_classification(self):
        prefix, suffix = b'{"x":"', b'"}\n'
        exact_body = ("sys.stdout.buffer.write(" + repr(prefix) + " + b'x' * " +
                      str(subject.STREAM_LIMIT - len(prefix) - len(suffix)) + " + " +
                      repr(suffix) + ")")
        cases = [
            ("exact-cap", exact_body, "SEALED_COMPLETE", "JSON_OBJECT"),
            ("array", "sys.stdout.write('[]\\n')", "SEALED_INCOMPLETE", "INVALID_JSON"),
            ("malformed", "sys.stdout.write('{bad\\n')", "SEALED_INCOMPLETE", "INVALID_JSON"),
            ("nonzero", "sys.stdout.write('{}\\n')\nraise SystemExit(65)",
             "SEALED_INCOMPLETE", "NOT_CHECKED"),
        ]
        for index, (name, body, disposition, json_status) in enumerate(cases):
            with self.subTest(case=name):
                parent = self.path / str(index)
                parent.mkdir()
                repo, context, root, sources, expected_reservation = fixture(parent)
                complete_native(repo, root)
                helper = subject.load_primitives(repo, sources)
                with fake_xcrun(parent, body), \
                        mock.patch.object(subject, "load_primitives", return_value=helper), \
                        mock.patch.object(helper, "encrypt_archive", side_effect=synthetic_encrypt):
                    code, receipt = subject.collect(repo, context, root, expected_reservation)
                self.assertEqual(code, 0 if disposition == "SEALED_COMPLETE" else 70)
                self.assertEqual(receipt["disposition"], disposition)
                self.assertEqual(receipt["jsonStatus"], json_status)
                self.assertEqual(receipt["nativeExitCode"], 65)
                if name == "exact-cap":
                    self.assertEqual(receipt["streams"]["stdout"]["bytes"], subject.STREAM_LIMIT)
                cipher = (root / "public/ios-xcresult-summary.zip.p7m").read_bytes()
                raw = outer_zip(receipt, cipher)
                artifact = parent / "artifact.zip"
                artifact.write_bytes(raw)
                expected = {
                    "schema": consumer.EXPECTED_SCHEMA, "context": context, "workflowId": 777,
                    "artifact": {"id": 888, "name": subject.ARTIFACT_NAME,
                                 **subject.digest_record(raw), "runId": context["runId"],
                                 "attempt": context["attempt"], "sourceSha": context["sourceSha"]},
                    "sources": sources,
                }
                result = consumer.private_intake(
                    artifact, root / "private/ios-xcresult-summary.zip", expected, subject)
                self.assertEqual(result["completeSummaryJsonObject"], disposition == "SEALED_COMPLETE")
                self.assertFalse(result["formalAcceptanceGranted"])
                if disposition == "SEALED_INCOMPLETE":
                    changed = copy.deepcopy(receipt)
                    changed["disposition"] = "SEALED_COMPLETE"
                    with self.assertRaises(ValueError):
                        subject.validate_receipt(changed)

    def test_ciphertext_oversize_stays_private_without_public_partial(self):
        repo, context, root, sources, expected = self.new_fixture()
        complete_native(repo, root)
        helper = subject.load_primitives(repo, sources)
        def oversized(private, public, archive, archive_binding, recipient, deadline):
            encoded = b"\x30" + b"x" * subject.CIPHER_LIMIT
            (public / "project-prerequisite.zip.p7m").write_bytes(encoded)
            return subject.digest_record(encoded)
        with fake_xcrun(self.path, "sys.stdout.write('{}\\n')"), \
                mock.patch.object(subject, "load_primitives", return_value=helper), \
                mock.patch.object(helper, "encrypt_archive", side_effect=oversized):
            code, receipt = subject.collect(repo, context, root, expected)
        self.assertEqual(code, 70)
        self.assertEqual(receipt["disposition"], "REJECTED")
        self.assertIsNone(receipt["ciphertext"])
        self.assertIsNone(receipt["plaintextArchive"])
        self.assertEqual((root / "private/encryption-public/project-prerequisite.zip.p7m").stat().st_size,
                         subject.CIPHER_LIMIT + 1)
        self.assertEqual(set(p.name for p in (root / "public").iterdir()), {"receipt.json"})

    def test_unsealed_rejection_never_reads_plaintext(self):
        rejection = subject.public_receipt(
            self.context, self.sources, self.reservation,
            elapsed=10, native=65, rejection="CAPTURE_OVERFLOW")
        raw = outer_zip(rejection)
        artifact = self.path / "rejection.zip"
        artifact.write_bytes(raw)
        expected = copy.deepcopy(self.expected)
        expected["artifact"].update(subject.digest_record(raw))
        self.assertEqual(consumer.public_intake(artifact, expected, subject)["disposition"], "REJECTED")
        with self.assertRaises(ValueError):
            consumer.private_intake(artifact, self.path / "PRIVATE_MUST_NOT_READ", expected, subject)

    def test_plain_cipher_and_member_hash_rejection(self):
        for mode in ("cipher", "plain", "member"):
            with self.subTest(mode=mode):
                receipt = copy.deepcopy(self.receipt)
                if mode == "cipher":
                    receipt["ciphertext"]["sha256"] = "0" * 64
                elif mode == "plain":
                    receipt["plaintextArchive"]["sha256"] = "0" * 64
                else:
                    receipt["members"][0]["sha256"] = "0" * 64
                artifact, plaintext, expected = self.candidate(receipt)
                with self.assertRaises(ValueError):
                    consumer.private_intake(artifact, plaintext, expected, subject)

    def test_exact_zip_inventory_and_member_types(self):
        with zipfile.ZipFile(io.BytesIO(self.plain)) as archive:
            original = [(info, archive.read(info)) for info in archive.infolist()]
        for mutation in ("missing", "extra", "duplicate", "traversal", "symlink"):
            stream = io.BytesIO()
            with zipfile.ZipFile(stream, "w") as archive:
                for index, (info, raw) in enumerate(original):
                    if mutation == "missing" and index == 0:
                        continue
                    info = copy.copy(info)
                    if index == 0 and mutation == "traversal":
                        info.filename = "../identity.json"
                    if index == 0 and mutation == "symlink":
                        info.external_attr = (stat.S_IFLNK | 0o777) << 16
                    archive.writestr(info, raw)
                if mutation == "extra":
                    archive.writestr("extra", b"PRIVATE")
                if mutation == "duplicate":
                    archive.writestr(original[0][0], original[0][1])
            artifact, plaintext, expected = self.candidate(plain=stream.getvalue())
            with self.subTest(mutation=mutation), self.assertRaises((ValueError, zipfile.BadZipFile)):
                consumer.private_intake(artifact, plaintext, expected, subject)

        # Isolate the size boundary from hashes and the private compression policy.
        for compression in (zipfile.ZIP_STORED, zipfile.ZIP_DEFLATED):
            for extra in (0, 1):
                with self.subTest(compression=compression, extra=extra):
                    payload = b"x" * (subject.METADATA_LIMIT + extra)
                    stream = io.BytesIO()
                    with zipfile.ZipFile(stream, "w", compression=compression) as archive:
                        archive.writestr("bounded.json", payload)
                    limits = {"bounded.json": subject.METADATA_LIMIT}
                    if extra:
                        with self.assertRaises(ValueError):
                            consumer.zip_members(stream.getvalue(), limits)
                    else:
                        self.assertEqual(consumer.zip_members(stream.getvalue(), limits),
                                         {"bounded.json": payload})

    def test_json_syntax_and_false_completeness(self):
        for raw in (b'{"a":1,"a":2}', b'{"a":NaN}', b'{"a":1e999}', b'{"a":'):
            with self.assertRaises(ValueError):
                subject.json_value(raw)
        self.assertIsInstance(subject.json_value(b"[]"), list)
        receipt = copy.deepcopy(self.receipt)
        receipt["captureResult"] = "TIMEOUT"
        with self.assertRaises(ValueError):
            subject.validate_receipt(receipt)
        receipt = copy.deepcopy(self.receipt)
        receipt["streams"]["stdout"]["complete"] = False
        with self.assertRaises(ValueError):
            subject.validate_receipt(receipt)

    def test_cli_argument_and_runtime_errors_do_not_echo_private_values(self):
        commands = [
            [sys.executable, "-I", "-B", str(HERE / "collect_ios_xcresult_summary.py"),
             "collect", "--repo", CANARY, "--private-unknown", CANARY],
            [sys.executable, "-I", "-B", str(HERE / "verify_ios_xcresult_summary.py"),
             "--expected", CANARY, "--artifact", CANARY, "--private-unknown", CANARY],
            [sys.executable, "-I", "-B", str(HERE / "verify_ios_xcresult_summary.py"),
             "--expected", str(self.path / CANARY), "--artifact", CANARY],
        ]
        for command in commands:
            result = subprocess.run(command, capture_output=True, timeout=5)
            self.assertEqual(result.returncode, 70)
            self.assertNotIn(CANARY.encode(), result.stdout + result.stderr)
            self.assertNotIn(b"Traceback", result.stderr)

    def test_original_native_exit_sequence_and_bounds_remain(self):
        workflow = (REPO / subject.WORKFLOW).read_text()
        sequence = """          status=$?
          set -e

          printf '%s\\n' "$status" > xcodetest.exitcode
          echo "xcodebuild test exit: $status (bounded diagnostics are collected in the next step)"
          exit "$status"
"""
        self.assertIn(sequence, workflow)
        self.assertIn("timeout-minutes: 75", workflow)
        self.assertIn('-resultBundlePath "$XCRESULT_ROOT/tests.xcresult"', workflow)
        self.assertIn('"-only-testing:CherriUITests/CherriEditorUITests/testBRG27EditorRealWorldWorkflowUI"', workflow)
        self.assertIn('"-only-testing:CherriUITests/CherriEditorUITests/testD03CurrentDocumentationSearchUI"', workflow)
        self.assertIn('"-only-testing:CherriUITests/CherriEditorUITests/testE01CompletionDisplayAndInsertionUI"', workflow)
        self.assertEqual(subject.BUDGETS, {"preflightMs": 15000, "summaryMs": 20000,
                                         "encryptionMs": 20000, "postTestMs": 60000})


if __name__ == "__main__":
    unittest.main()
