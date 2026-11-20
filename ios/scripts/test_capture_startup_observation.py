import base64
import copy
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest
from unittest import mock

import capture_startup_observation as capture
import collect_ios_test_evidence as collector

TEST = "CherriEditorUITests/testD03CurrentDocumentationSearchUI"
NONCE = "a" * 32
UDID = "A1B2C3D4-1234-5678-ABCD-1234567890AB"
ROOT = Path(__file__).resolve().parents[2]


def encode(row):
    return json.dumps(row, sort_keys=True)


def session(phase="before-launch", nonce=NONCE, test=TEST, uptime=1000):
    value = {"schemaVersion": "1", "nonce": nonce, "testId": test, "phase": phase, "uptimeMs": uptime}
    if phase == "readiness-start":
        value["readinessStartedAtMs"] = uptime - 1
    return capture.SESSION + encode(value)


def description(text):
    raw = text.encode()
    return {"base64": base64.b64encode(raw[:4096]).decode(), "originalByteCount": len(raw), "truncated": len(raw) > 4096}


def issue():
    return {"schemaVersion": "1", "nonce": NONCE, "testId": TEST, "uptimeMs": 4000,
            "issueIndex": 1, "issueType": "system", "sourceFile": "CherriEditorUITests.swift",
            "sourceLine": 122, "errorPresent": True, "errorDomain": description("XCTestErrorDomain"),
            "errorCode": 1001, "compact": description("Failed to get matching snapshots: PRIVATE_CANARY"),
            "detailed": description("PRIVATE_USER_SOURCE"), "errorDescription": description("PRIVATE_ERROR_DETAILS")}


def events():
    common = {"schemaVersion": "1", "nonce": NONCE, "testId": TEST, "documentId": None,
              "mainThread": False, "processId": 1234}
    return [
        dict(common, sequence=0, phase="observer-opened", observedUptimeMs=1100, writtenUptimeMs=1100),
        dict(common, sequence=1, phase="app-initialized", observedUptimeMs=1100, writtenUptimeMs=1101, mainThread=True),
        dict(common, sequence=2, phase="heartbeat", observedUptimeMs=3100, writtenUptimeMs=3101),
        dict(common, sequence=3, phase="document-initialized", observedUptimeMs=4000, writtenUptimeMs=4001,
             documentId="b"*32, mainThread=True),
        dict(common, sequence=4, phase="heartbeat", observedUptimeMs=5100, writtenUptimeMs=5101),
    ]


def event_bytes(rows=None):
    return ("\n".join(encode(row) for row in (events() if rows is None else rows)) + "\n").encode()


def log_lines(*middle, conclusion="failed"):
    method = TEST.split("/")[1]
    return "\n".join([
        f"Test Case '-[CherriUITests.CherriEditorUITests {method}]' started.",
        session(), session("after-launch", uptime=2000),
        session("readiness-start", uptime=2002), *middle,
        f"Test Case '-[CherriUITests.CherriEditorUITests {method}]' {conclusion} (5.0 seconds).",
    ]) + "\n"


class StartupObservationControls(unittest.TestCase):
    def test_observed_events_preserve_identity_and_partial_health(self):
        rows, health = capture.validate_app_events(event_bytes(), TEST, NONCE)
        self.assertEqual(rows, events())
        self.assertEqual(health["heartbeatCount"], 2)
        self.assertEqual(health["heartbeatSpanMs"], 2000)
        self.assertEqual(health["largestObservedHeartbeatGapMs"], 2000)
        self.assertEqual(health["terminatedBy"], "NO_TERMINAL_EVENT")
        self.assertFalse(health["absenceProvesMissingLifecycle"])

    def test_one_event_does_not_claim_observer_health_or_editor(self):
        rows, health = capture.validate_app_events(event_bytes(events()[:1]), TEST, NONCE)
        self.assertEqual(len(rows), 1)
        self.assertEqual(health["status"], "NO_HEARTBEAT_OBSERVED")
        self.assertIsNone(health["heartbeatSpanMs"])

    def test_actual_event_corruptions_are_rejected(self):
        mutations = {
            "private_field": lambda rows: rows[1].update(sourceText="PRIVATE"),
            "borrowed_nonce": lambda rows: rows[1].update(nonce="c"*32),
            "borrowed_test": lambda rows: rows[1].update(testId="CherriEditorUITests/testForeign"),
            "duplicate_sequence": lambda rows: rows[1].update(sequence=0),
            "bool_sequence": lambda rows: rows[1].update(sequence=True),
            "unknown_phase": lambda rows: rows[1].update(phase="EDITOR_READY"),
            "nontext_phase": lambda rows: rows[1].update(phase=[]),
            "heartbeat_document": lambda rows: rows[2].update(documentId="c"*32),
            "heartbeat_on_main": lambda rows: rows[2].update(mainThread=True),
            "backward_heartbeat": lambda rows: rows[4].update(observedUptimeMs=3000),
            "private_document": lambda rows: rows[1].update(documentId="PRIVATE"),
            "newline_document": lambda rows: rows[1].update(documentId="b"*32+"\n"),
            "bool_clock": lambda rows: rows[1].update(observedUptimeMs=True),
            "future_observation": lambda rows: rows[1].update(observedUptimeMs=1200),
            "backward_write": lambda rows: rows[2].update(writtenUptimeMs=1100, observedUptimeMs=1099),
            "borrowed_process": lambda rows: rows[1].update(processId=999),
            "bool_main": lambda rows: rows[1].update(mainThread=1),
            "missing_open": lambda rows: rows[0].update(phase="heartbeat"),
            "after_terminal": lambda rows: rows[2].update(phase="observer-expired"),
        }
        for name, change in mutations.items():
            with self.subTest(name=name):
                rows = events()
                change(rows)
                with self.assertRaises(ValueError):
                    capture.validate_app_events(event_bytes(rows), TEST, NONCE)

    def test_duplicate_json_key_and_incomplete_file_rejected(self):
        raw = event_bytes()
        with self.assertRaises(ValueError):
            capture.validate_app_events(raw[:-1], TEST, NONCE)
        duplicate = raw.replace(b'"schemaVersion": "1"', b'"schemaVersion": "1", "schemaVersion": "1"', 1)
        with self.assertRaises(ValueError):
            capture.validate_app_events(duplicate, TEST, NONCE)
        with self.assertRaises(ValueError):
            capture.validate_app_events(b" "*(capture.MAX_FILE+1), TEST, NONCE)

    def test_private_issue_yields_exact_code_and_digests_without_payload(self):
        original = issue()
        result = capture.issue(capture.ISSUE+encode(original), TEST, NONCE)
        self.assertEqual(result["errorDomain"], "XCTestErrorDomain")
        self.assertEqual(result["errorCode"], 1001)
        self.assertEqual(result["sourceLine"], 122)
        self.assertIn("MATCHING_SNAPSHOT_FAILURE", result["diagnosticSignatures"])
        self.assertEqual(result["descriptions"]["detailed"]["capturedSha256"], capture.digest(b"PRIVATE_USER_SOURCE"))
        self.assertNotIn("PRIVATE_", encode(result))
        original["errorDomain"] = description("PRIVATE_DOMAIN")
        original["sourceFile"] = "PRIVATE_FILENAME"
        result = capture.issue(capture.ISSUE+encode(original), TEST, NONCE)
        self.assertEqual(result["errorDomain"], "UNREVIEWED")
        self.assertEqual(result["sourceFile"], "UNREVIEWED")
        self.assertNotIn("PRIVATE_", encode(result))

    def test_issue_mutations_rejected(self):
        for key, value in [
            ("nonce", "b"*32), ("testId", "foreign"), ("issueIndex", True),
            ("errorCode", False), ("errorPresent", False), ("uptimeMs", "1"),
            ("compact", {"base64": "!!!!", "originalByteCount": 1, "truncated": False}),
        ]:
            with self.subTest(key=key):
                row = issue(); row[key] = value
                with self.assertRaises(ValueError):
                    capture.issue(capture.ISSUE+encode(row), TEST, NONCE)
        row = issue(); row["compact"]["originalByteCount"] += 1
        with self.assertRaises(ValueError):
            capture.issue(capture.ISSUE+encode(row), TEST, NONCE)

    def test_test_boundary_and_nonce_binding(self):
        text = log_lines(capture.ISSUE+encode(issue()))
        result, private = capture.test_observations(text)
        self.assertEqual(len(result[TEST]["issues"]), 1)
        self.assertEqual(len(private), 1)
        self.assertTrue(result[TEST]["uniqueTestInvocation"])
        result, private = capture.test_observations(session()+"\n"+capture.ISSUE+encode(issue()))
        self.assertEqual(result[TEST]["sessions"], [])
        self.assertEqual(private, [])
        repeated = text + text
        result, _ = capture.test_observations(repeated)
        self.assertFalse(result[TEST]["uniqueTestInvocation"])
        self.assertGreater(result[TEST]["rejectedMetadataCount"], 0)

    def test_bad_session_order_and_stale_nonce_rejected(self):
        for line in [session("before-launch", nonce="../PRIVATE"),
                     session("after-launch", nonce="b"*32, uptime=2001),
                     session("before-launch", uptime=1999),
                     session("readiness-start", uptime=500)]:
            result, _ = capture.test_observations(log_lines(line))
            self.assertGreater(result[TEST]["rejectedMetadataCount"], 0)

    def test_nonce_selected_file_does_not_scan_siblings(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            cache = root/"Library"/"Caches"/"CherriStartupObservation"
            cache.mkdir(parents=True)
            selected = cache/(NONCE+".jsonl"); selected.write_bytes(event_bytes())
            unrelated = cache/("c"*32+".jsonl"); unrelated.write_bytes(b"PRIVATE_UNRELATED")
            self.assertEqual(capture.contained_file(root, NONCE), selected)
            self.assertEqual(unrelated.read_bytes(), b"PRIVATE_UNRELATED")
            with self.assertRaises(ValueError):
                capture.contained_file(root, "../PRIVATE")
            selected.write_bytes(b"x"*(capture.MAX_FILE+1))
            with self.assertRaisesRegex(ValueError, "OVERSIZE"):
                capture.contained_file(root, NONCE)

    def test_symlink_event_file_rejected(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            cache = root/"Library"/"Caches"/"CherriStartupObservation"
            cache.mkdir(parents=True)
            target = root/"other"; target.write_bytes(event_bytes())
            selected = cache/(NONCE+".jsonl")
            try:
                selected.symlink_to(target)
            except OSError as error:
                self.skipTest("host does not permit symlink creation: "+type(error).__name__)
            with self.assertRaisesRegex(ValueError, "SYMLINK"):
                capture.contained_file(root, NONCE)


    def test_ordinary_public_log_drops_private_observation_and_session_lines(self):
        raw = log_lines(capture.ISSUE+encode(issue()))
        bounded, receipts, failures = collector.bounded_log(raw)
        self.assertIn("Test Case", bounded)
        self.assertNotIn(capture.SESSION, bounded)
        self.assertNotIn(capture.ISSUE, bounded)
        self.assertNotIn(NONCE, bounded)
        self.assertNotIn("PRIVATE", bounded)
        for field in ("compact", "detailed", "errorDescription", "errorDomain"):
            self.assertNotIn(issue()[field]["base64"], bounded)
        self.assertEqual(receipts, [])
        self.assertEqual(failures, [])

    def test_diagnostic_scope_keeps_actual_pass_without_acceptance(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            name = "testBRG27EditorRealWorldWorkflowUI"
            text = (f"Test Case '-[CherriUITests.CherriEditorUITests {name}]' started.\n"
                    "[CHECKPOINT:BRG27_WORKFLOW_VERIFIED]\n"
                    f"Test Case '-[CherriUITests.CherriEditorUITests {name}]' passed (1.0 seconds).\n")
            log = root/"log"; log.write_text(text, encoding="utf-8")
            code = root/"exit"; code.write_text("0", encoding="utf-8")
            ordinary, records = collector.collect(log, root/"ordinary", "a"*40, "42", code)
            self.assertEqual(len(records), 1)
            observed, records = collector.collect(log, root/"observed", "a"*40, "42", code, diagnostic_only=True)
            self.assertEqual(records, [])
            self.assertEqual(observed["runStatus"], "PASSED")
            self.assertEqual(observed["testResults"], ordinary["testResults"])
            code.write_text("65", encoding="utf-8")
            failed, records = collector.collect(log, root/"failed", "a"*40, "42", code, diagnostic_only=True)
            self.assertEqual(failed["runStatus"], "FAILED")
            self.assertEqual(records, [])


class ActualCaptureControls(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.temporary = tempfile.TemporaryDirectory()
        cls.root = Path(cls.temporary.name)
        cls.repo = cls.root/"source"; cls.repo.mkdir()
        for name in capture.SOURCES:
            destination = cls.repo/name
            destination.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(ROOT/name, destination)
        subprocess.run(["git", "init", "-q"], cwd=cls.repo, check=True, capture_output=True)
        # The control owns this temporary repository; checkout must preserve the
        # exact fixture bytes on Windows as well as POSIX.
        subprocess.run(["git", "config", "core.autocrlf", "false"], cwd=cls.repo, check=True, capture_output=True)
        subprocess.run(["git", "config", "core.eol", "lf"], cwd=cls.repo, check=True, capture_output=True)
        subprocess.run(["git", "add", "."], cwd=cls.repo, check=True, capture_output=True)
        subprocess.run(["git", "-c", "user.name=Cherri Local Control", "-c", "user.email=local-control@example.invalid",
                        "-c", "commit.gpgsign=false", "commit", "-qm", "Owned local observation control"],
                       cwd=cls.repo, check=True, capture_output=True)
        cls.source = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=cls.repo).decode().strip()

    @classmethod
    def tearDownClass(cls):
        cls.temporary.cleanup()

    def test_real_source_blob_binding_and_dirty_rejection(self):
        bindings = capture.source_binding(self.repo, self.source)
        self.assertEqual(set(bindings), set(capture.SOURCES))
        name = capture.SOURCES[0]; path = self.repo/name; original = path.read_bytes()
        try:
            path.write_bytes(original+b"\n")
            with self.assertRaisesRegex(ValueError, "SOURCE_BYTES"):
                capture.source_binding(self.repo, self.source)
        finally:
            path.write_bytes(original)
        with self.assertRaisesRegex(ValueError, "SOURCE_HEAD"):
            capture.source_binding(self.repo, "a"*40)


    def test_real_git_replacement_cannot_rebind_claimed_source(self):
        path = self.repo/capture.SOURCES[0]
        original = path.read_bytes()
        forged = original+b"\n// REPLACEMENT_OBJECT_CONTROL\n"
        def git(*args):
            return subprocess.check_output(["git", *args], cwd=self.repo, stderr=subprocess.PIPE)
        try:
            path.write_bytes(forged)
            git("add", capture.SOURCES[0])
            git("-c", "user.name=Cherri Local Control", "-c", "user.email=local-control@example.invalid",
                "-c", "commit.gpgsign=false", "commit", "-qm", "Local replacement negative control")
            replacement = git("rev-parse", "HEAD").decode().strip()
            git("checkout", "--detach", self.source)
            git("replace", self.source, replacement)
            path.write_bytes(forged)
            self.assertEqual(git("rev-parse", "HEAD").decode().strip(), self.source)
            self.assertEqual(git("show", self.source+":"+capture.SOURCES[0]), forged)
            with self.assertRaisesRegex(ValueError, "SOURCE_BYTES"):
                capture.source_binding(self.repo, self.source)
            # The same actual claimed source remains readable despite an
            # unrelated local replacement; only its real object owns the bytes.
            path.write_bytes(original)
            actual = capture.source_binding(self.repo, self.source)
            self.assertEqual(actual[capture.SOURCES[0]], capture.digest(original))
        finally:
            subprocess.run(["git", "replace", "-d", self.source], cwd=self.repo,
                           stdout=subprocess.PIPE, stderr=subprocess.PIPE)
            subprocess.run(["git", "--no-replace-objects", "reset", "--hard", self.source], cwd=self.repo,
                           check=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)

    def test_full_capture_selects_one_owned_file_and_keeps_private_originals(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            app = root/"app-data"; cache = app/"Library"/"Caches"/"CherriStartupObservation"
            cache.mkdir(parents=True)
            exact = event_bytes(); (cache/(NONCE+".jsonl")).write_bytes(exact)
            (cache/("f"*32+".jsonl")).write_bytes(b"PRIVATE_UNRELATED")
            log = root/"test.log"; log.write_text(log_lines(capture.ISSUE+encode(issue())), encoding="utf-8")
            udid = root/"udid"; udid.write_text(UDID+"\n", encoding="ascii")
            output, private = root/"public", root/"private"
            actual_run = subprocess.run
            query_calls = []
            def query(command, **kwargs):
                if command[0] != "xcrun":
                    return actual_run(command, **kwargs)
                query_calls.append(command)
                self.assertEqual(kwargs["timeout"], 20)
                kwargs["stdout"].write((str(app)+"\n").encode())
                return subprocess.CompletedProcess(command, 0)
            with mock.patch.object(capture.subprocess, "run", side_effect=query):
                report = capture.capture(self.repo, log, udid, output, private, self.source, "123")
            self.assertEqual(query_calls, [["xcrun", "simctl", "get_app_container", UDID, "org.cherrilang.Cherri", "data"]])
            self.assertEqual(report["tests"][TEST]["appStatus"], "OBSERVED")
            self.assertEqual((output/"startup-events-d03.jsonl").read_bytes(), exact)
            self.assertEqual((private/"app-d03.jsonl").read_bytes(), exact)
            self.assertIn(b"PRIVATE_USER_SOURCE", base64.b64decode(issue()["detailed"]["base64"]))
            self.assertEqual(report["acceptanceRecords"], [])
            self.assertFalse(report["formalAcceptanceGranted"])
            public_bytes = b"".join(path.read_bytes() for path in output.iterdir())
            self.assertNotIn(b"PRIVATE", public_bytes)
            self.assertNotIn(str(app).encode(), public_bytes)
            self.assertIn(capture.ISSUE.encode(), (private/"issues-original.jsonl").read_bytes())
            with self.assertRaisesRegex(ValueError, "OUTPUT_EXISTS"):
                capture.capture(self.repo, log, udid, output, private, self.source, "123")

    def test_query_timeout_is_not_observer_or_app_failure(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary); log=root/"test.log"; log.write_text(log_lines(), encoding="utf-8")
            udid=root/"udid"; udid.write_text(UDID, encoding="ascii")
            actual_run = subprocess.run
            def query(command, **kwargs):
                if command[0] != "xcrun":
                    return actual_run(command, **kwargs)
                kwargs["stderr"].write(b"ORIGINAL_QUERY_TIMEOUT")
                raise subprocess.TimeoutExpired(command, 20)
            with mock.patch.object(capture.subprocess, "run", side_effect=query):
                report = capture.capture(self.repo, log, udid, root/"out", root/"private", self.source, "123")
            self.assertTrue(report["containerQuery"]["timedOut"])
            self.assertEqual(report["tests"][TEST]["appError"], "CONTAINER_UNAVAILABLE")
            self.assertEqual(report["tests"][TEST]["appStatus"], "NOT_OBSERVED")
            self.assertEqual((root/"private"/"container-query.stderr").read_bytes(), b"ORIGINAL_QUERY_TIMEOUT")
            self.assertFalse(report["formalAcceptanceGranted"])


if __name__ == "__main__":
    unittest.main()
