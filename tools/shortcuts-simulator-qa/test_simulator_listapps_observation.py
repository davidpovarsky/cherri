import hashlib
import json
from pathlib import Path
import tempfile
import subprocess
import unittest
import zipfile
from unittest import mock

import simulator_listapps_observation as app

UDID = "11111111-2222-3333-4444-555555555555"
SOURCE = "a" * 40
IDENTITY = {"compilerCommitSha": SOURCE, "runId": "12345", "runAttempt": "1",
            "fixtureId": "qa-short-circuit-v2"}
ORIGINAL = b'{ "com.apple.shortcuts" = {}; "PRIVATE_LISTAPPS_CANARY" = "unchanged"; }\n'


class Runner:
    def __init__(self, failure=None, late=False, changed_after=False, repo=None):
        self.failure, self.late, self.changed_after = failure, late, changed_after
        self.commands = []
        self.repo = repo
        self.list_seen = False

    def __call__(self, argv, allowance, *, output, error_output, hard_deadline,
                 stream_limit, journal):
        self.commands.append(list(argv))
        if argv != ["xcrun", "simctl", "listapps", UDID]:
            raise AssertionError("Only the native command is a synthetic double; Git is real")
        raw = ORIGINAL
        self.list_seen = True
        if self.changed_after:
            subprocess.run(["git", "-C", str(self.repo), "-c", "user.name=Synthetic control",
                            "-c", "user.email=synthetic@invalid.example", "commit",
                            "--allow-empty", "-qm", "Change actual fixture HEAD"],
                           check=True, capture_output=True, timeout=5)
        detail = {"fixtureProcessControl": True, "pid": 123,
                  "startedAt": "2026-11-25T00:00:00+00:00"}
        journal("started", detail)
        output.write(raw)
        result, code = self.failure or "PASS", 0 if not self.failure else 37
        if self.failure:
            error_output.write(b"original private native failure\n")
        detail = {**detail, "finishedAt": "2026-11-25T00:00:00.001+00:00",
                  "returnCodeObserved": code, "cleanupParentReaped": True,
                  "groupPresentAfterCleanup": False}
        journal("finished", detail)
        return {"result": result, "exitCode": code,
                "elapsedMs": round(allowance * 1000) + 1 if self.late and argv[0] != "git" else 1}, detail


class ListAppsTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name).resolve(strict=True)
        self.repo = self.root / "repo"
        self.repo.mkdir()
        for name in app.observation.QA_SOURCE_FILES:
            path = self.repo / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes((Path(__file__).resolve().parents[2] / name).read_bytes())
        recipient = self.repo / app.RECIPIENT
        recipient.parent.mkdir(parents=True, exist_ok=True)
        recipient.write_bytes((Path(__file__).resolve().parents[2] / app.RECIPIENT).read_bytes())
        for args in (["init", "-q"], ["add", "."],
                     ["-c", "user.name=Synthetic control", "-c", "user.email=synthetic@invalid.example",
                      "commit", "-qm", "Pin real source fixture"]):
            subprocess.run(["git", "--no-replace-objects", "-C", str(self.repo), *args],
                           check=True, capture_output=True, timeout=5)
        self.source = subprocess.check_output(
            ["git", "--no-replace-objects", "-C", str(self.repo), "rev-parse", "HEAD"],
            timeout=5, text=True).strip()
        self.identity = {**IDENTITY, "compilerCommitSha": self.source}

    def probe(self, runner=None, **kwargs):
        runner = runner or Runner(repo=self.repo)
        if isinstance(runner, Runner):
            runner.repo = self.repo
        return app.ListAppsObservation(self.repo, self.root / "packet", UDID, self.identity,
                                       runner=runner, **kwargs)

    def test_exact_stdout_only_released_after_source_checks_and_real_sealing(self):
        runner = Runner(repo=self.repo)
        probe = self.probe(runner)
        self.assertTrue(probe.observe())
        with self.assertRaisesRegex(ValueError, "NOT_SEALED"):
            probe.release_stdout(self.root / "caller.plist")
        summary = probe.seal()
        probe.release_stdout(self.root / "caller.plist")
        self.assertEqual((self.root / "caller.plist").read_bytes(), ORIGINAL)
        self.assertEqual(runner.commands[0], ["xcrun", "simctl", "listapps", UDID])
        self.assertEqual([row["stage"] for row in probe.commands],
                         ["source-before", "listapps", "source-after"])
        self.assertEqual(summary["status"], "READY")
        self.assertFalse(summary["deviceMutated"])
        self.assertFalse(summary["shortcutExecuted"])
        self.assertFalse(summary["formalAcceptanceGranted"])
        self.assertFalse(any(any(word in row for word in ("boot", "erase", "create", "shutdown", "openurl"))
                             for row in runner.commands))

    def test_private_output_canary_and_exact_process_journals_stay_in_encrypted_packet(self):
        probe = self.probe()
        self.assertTrue(probe.observe())
        summary = probe.seal()
        self.assertEqual(sorted(p.name for p in probe.public.iterdir()),
                         ["session-originals.zip.p7m", "summary.json"])
        for p in probe.public.iterdir():
            self.assertNotIn(b"PRIVATE_LISTAPPS_CANARY", p.read_bytes())
            self.assertNotIn(UDID.encode(), p.read_bytes())
        raw = (probe.private / "session-originals.zip").read_bytes()
        self.assertEqual(summary["plaintextArchive"]["sha256"], hashlib.sha256(raw).hexdigest())
        with zipfile.ZipFile(probe.private / "session-originals.zip") as archive:
            self.assertEqual(archive.read("command-02.stdout"), ORIGINAL)
            started = json.loads(archive.read("command-02.started.original.json"))
            finished = json.loads(archive.read("command-02.finished.original.json"))
            process = json.loads(archive.read("command-02.process.original.json"))
            self.assertEqual(started["capture"]["pid"], 123)
            self.assertEqual(finished["capture"]["pid"], 123)
            self.assertEqual(process["stdout"]["sha256"], hashlib.sha256(ORIGINAL).hexdigest())
            self.assertEqual(process["argv"], ["xcrun", "simctl", "listapps", UDID])

    def test_nonzero_output_with_shortcuts_text_cannot_be_released(self):
        runner = Runner(failure="EXIT_NONZERO")
        probe = self.probe(runner)
        self.assertFalse(probe.observe())
        self.assertEqual(len(runner.commands), 1)
        summary = probe.seal()
        self.assertEqual(summary["status"], "BLOCKED")
        self.assertEqual(probe.commands[-1]["exitCode"], 37)
        self.assertEqual((probe.private / "command-02.stderr").read_bytes(),
                         b"original private native failure\n")
        with self.assertRaisesRegex(ValueError, "NOT_READY"):
            probe.release_stdout(self.root / "caller.plist")
        self.assertFalse((self.root / "caller.plist").exists())

    def test_every_incomplete_process_verdict_stays_blocked(self):
        for index, result in enumerate(sorted(app.RESULTS - {"PASS", "CAPTURE_INTERRUPTED"})):
            with self.subTest(result=result):
                probe = app.ListAppsObservation(self.repo, self.root / ("packet-" + str(index)),
                                               UDID, self.identity, runner=Runner(failure=result, repo=self.repo))
                self.assertFalse(probe.observe())
                self.assertIsNone(probe.stdout)
                self.assertEqual(probe.commands[-1]["result"], result)

    def test_source_change_after_listapps_blocks_exact_output(self):
        probe = self.probe(Runner(changed_after=True))
        self.assertFalse(probe.observe())
        self.assertIsNone(probe.stdout)
        self.assertEqual(probe.source_before, self.source)
        self.assertIsNone(probe.source_after)

    def test_wrong_initial_source_stops_before_any_simulator_command(self):
        runner = Runner(repo=self.repo)
        probe = app.ListAppsObservation(self.repo, self.root / "packet", UDID,
                                       {**self.identity, "compilerCommitSha": "b" * 40}, runner=runner)
        self.assertFalse(probe.observe())
        self.assertEqual(runner.commands, [])

    def test_late_zero_exit_cannot_claim_ready(self):
        probe = self.probe(Runner(late=True))
        self.assertFalse(probe.observe())
        self.assertEqual(probe.commands[-1]["result"], "DEADLINE_EXHAUSTED")
        self.assertIsNone(probe.stdout)

    def test_interruption_retains_actual_journal_before_rejecting_capture(self):
        runner = Runner(repo=self.repo)
        def interrupted(argv, allowance, **kwargs):
            runner(argv, allowance, **kwargs)
            raise TimeoutError("synthetic external interruption after process cleanup")
        probe = self.probe(interrupted)
        self.assertFalse(probe.observe())
        self.assertEqual(probe.commands[-1]["result"], "CAPTURE_INTERRUPTED")
        self.assertEqual(probe.commands[-1]["capture"]["pid"], 123)
        summary = probe.seal()
        with zipfile.ZipFile(probe.private / "session-originals.zip") as archive:
            self.assertIn("command-02.started.original.json", archive.namelist())
            self.assertIn("command-02.finished.original.json", archive.namelist())
            receipt = json.loads(archive.read("command-02.process.original.json"))
            self.assertEqual(receipt["result"], "CAPTURE_INTERRUPTED")
        self.assertEqual(summary["status"], "BLOCKED")

    def test_existing_caller_output_is_preserved_not_overwritten(self):
        probe = self.probe()
        self.assertTrue(probe.observe())
        probe.seal()
        target = self.root / "caller.plist"
        target.write_bytes(b"prior original")
        with self.assertRaisesRegex(ValueError, "OUTPUT_EXISTS"):
            probe.release_stdout(target)
        self.assertEqual(target.read_bytes(), b"prior original")

    def test_recipient_symlink_is_not_published(self):
        probe = self.probe()
        self.assertTrue(probe.observe())
        recipient = self.repo / app.RECIPIENT
        real = recipient.with_suffix(".original")
        recipient.rename(real)
        recipient.symlink_to(real)
        with self.assertRaises(ValueError):
            probe.seal()
        self.assertFalse((probe.public / "summary.json").exists())

    def test_phase_budget_prevents_new_child_after_deadline(self):
        ticks = iter([0])
        runner = Runner(repo=self.repo)
        probe = self.probe(runner, now=lambda: next(ticks, 91))
        self.assertFalse(probe.observe())
        self.assertEqual(runner.commands, [])
        self.assertEqual(probe.commands[0]["result"], "DEADLINE_EXHAUSTED")



class WorkingBytesTests(unittest.TestCase):
    def test_actual_git_identity_cannot_hide_changed_helper_capture_or_phase(self):
        import subprocess
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve(strict=True)
            repo = root / "repo"
            repo.mkdir()
            files = app.observation.QA_SOURCE_FILES
            for name in files:
                path = repo / name
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_bytes(("Synthetic tracked source " + name + "\n").encode())
            for argv in (["git", "init", "-q"],
                         ["git", "add", "--", *files],
                         ["git", "-c", "user.name=Journal control", "-c",
                          "user.email=journal-control@example.invalid",
                          "commit", "-qm", "Create synthetic source fixture"]):
                process = subprocess.run(argv, cwd=repo, capture_output=True, timeout=10)
                self.assertEqual(process.returncode, 0, process.stderr)
            commit = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=repo, text=True).strip()
            identity = {**IDENTITY, "compilerCommitSha": commit}
            targets = (
                "tools/shortcuts-simulator-qa/simulator_listapps_observation.py",
                "tools/shortcuts-simulator-qa/project_prerequisite_observation.py",
                "tools/shortcuts-simulator-qa/simulator_phase_boundary.py",
            )
            for index, name in enumerate(targets):
                with self.subTest(changed=name):
                    path = repo / name
                    original = path.read_bytes()
                    path.write_bytes(original + b"UNCOMMITTED_SOURCE_CHANGE\n")
                    try:
                        probe = app.ListAppsObservation(repo, root / ("packet-" + str(index)),
                                                       UDID, identity)
                        with self.assertRaisesRegex(ValueError, "SOURCE_(BYTES|INVENTORY)"):
                            probe.source("source-before")
                        self.assertTrue(all(row["argv"][0] == "git" for row in probe.commands))
                        self.assertEqual(subprocess.check_output(
                            ["git", "rev-parse", "HEAD"], cwd=repo, text=True).strip(), commit)
                    finally:
                        path.write_bytes(original)

if __name__ == "__main__":
    unittest.main()
