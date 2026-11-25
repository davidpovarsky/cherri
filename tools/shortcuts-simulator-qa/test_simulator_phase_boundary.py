import hashlib
import json
from pathlib import Path
import tempfile
import unittest
import zipfile
from unittest import mock

import simulator_phase_boundary as boundary

UDID = "11111111-2222-3333-4444-555555555555"
SOURCE = "a" * 40
IDENTITY = {"compilerCommitSha": SOURCE, "runId": "12345", "runAttempt": "1",
            "fixtureId": "qa-nested-list-control-v2", "phase": "pre-import"}


def devices(state="Booted", *, udid=UDID, runtime="com.apple.CoreSimulator.SimRuntime.iOS-27-0", available=True):
    return json.dumps({"devices": {runtime: [{"udid": udid, "name": "Unpublished device",
                                              "state": state, "isAvailable": available}]}}).encode()


class Runner:
    def __init__(self, states=None, failure=None, late=None, source=SOURCE):
        self.states = iter(states or ["Booted"] * 4)
        self.failure, self.late, self.source = failure, late, source
        self.commands = []

    def __call__(self, argv, allowance, *, output, error_output, hard_deadline, stream_limit):
        self.commands.append(list(argv))
        result, code, elapsed = "PASS", 0, 1
        if argv[0] == "git":
            raw = (self.source + "\n").encode()
        elif argv[1:4] == ["simctl", "list", "devices"]:
            raw = next(self.states)
            if isinstance(raw, str):
                raw = devices(raw)
        elif argv[1:3] == ["simctl", "getenv"]:
            raw = b"/private/owned-test-home\n"
        else:
            raw = b""
        key = argv[2] if argv[0] == "xcrun" else "git"
        if self.failure == key:
            result, code = "EXIT_NONZERO", 149
            error_output.write(b"original native failure\n")
        if self.late == key:
            elapsed = round(allowance * 1000) + 1
        output.write(raw)
        return {"result": result, "exitCode": code, "elapsedMs": elapsed}, {
            "fixtureProcessControl": True, "stdout": {"sha256": hashlib.sha256(raw).hexdigest()}}


def prepared(root, udid, allowance, **kwargs):
    return {"result": "PASS", "exitCode": 0, "elapsedMs": 1}, {
        "testControlOnly": True, "allowance": allowance, "selectedDevice": udid}


class PhaseTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.repo = self.root / "repo"
        self.repo.mkdir()

    def phase(self, runner=None, **kwargs):
        return boundary.Boundary(self.repo, self.root / "packet", UDID, IDENTITY,
                                 runner=runner or Runner(), prepare=kwargs.pop("prepare", prepared), **kwargs)

    def test_ready_requires_fresh_state_before_after_bootstatus_and_liveness(self):
        runner = Runner()
        phase = self.phase(runner)
        self.assertTrue(phase.run_session())
        self.assertEqual([row["state"] for row in phase.states], ["Booted"] * 4)
        self.assertFalse(phase.boot_issued)
        self.assertEqual(sum(command[1:3] == ["simctl", "bootstatus"] for command in runner.commands), 1)
        self.assertEqual(sum(command[1:3] == ["simctl", "getenv"] for command in runner.commands), 1)
        self.assertFalse(any("erase" in command or "create" in command or "shutdown" in command for command in runner.commands))

    def test_stopped_selected_device_boots_once_without_erase_or_recreation(self):
        runner = Runner(["Shutdown", "Shutdown", "Booted", "Booted"])
        phase = self.phase(runner)
        self.assertTrue(phase.run_session())
        self.assertTrue(phase.boot_issued)
        boots = [command for command in runner.commands if command[1:3] == ["simctl", "boot"]]
        self.assertEqual(boots, [["xcrun", "simctl", "boot", UDID]])
        self.assertFalse(any("shutdown" in command or "erase" in command or "create" in command for command in runner.commands))

    def test_shutdown_race_after_ready_getenv_is_not_promoted(self):
        phase = self.phase(Runner(["Booted", "Booted", "Booted", "Shutdown"]))
        self.assertFalse(phase.run_session())
        self.assertEqual(phase.status, "BLOCKED")

    def test_boot_failure_retains_exact_exit_and_stderr_and_stops(self):
        runner = Runner(["Shutdown", "Shutdown"], failure="boot")
        phase = self.phase(runner)
        self.assertFalse(phase.run_session())
        self.assertEqual(phase.commands[-1]["exitCode"], 149)
        self.assertEqual((phase.private / phase.commands[-1]["stderr"]["path"]).read_bytes(),
                         b"original native failure\n")
        self.assertFalse(any(command[1:3] == ["simctl", "bootstatus"] for command in runner.commands))

    def test_source_mismatch_stops_before_any_simulator_command(self):
        runner = Runner(source="b" * 40)
        phase = self.phase(runner)
        self.assertFalse(phase.run_session())
        self.assertEqual(len(runner.commands), 1)

    def test_late_zero_exit_remains_failure(self):
        phase = self.phase(Runner(late="bootstatus"))
        self.assertFalse(phase.run_session())
        self.assertEqual(phase.commands[-1]["result"], "DEADLINE_EXHAUSTED")

    def test_still_stopping_is_blocked_without_destructive_shutdown(self):
        runner = Runner(["Shutting Down", "Shutting Down"])
        phase = self.phase(runner)
        self.assertFalse(phase.run_session())
        self.assertFalse(any("boot" in command or "shutdown" in command for command in runner.commands))

    def test_failed_active_xcode_resolution_cannot_boot_or_claim_ready(self):
        def failed(*args, **kwargs):
            return {"result": "EXIT_NONZERO", "exitCode": 1, "elapsedMs": 1}, {"status": "BLOCKED"}
        runner = Runner()
        phase = self.phase(runner, prepare=failed)
        self.assertFalse(phase.run_session())
        self.assertEqual(len(phase.states), 1)
        self.assertFalse(phase.boot_issued)

    def assert_failed_prepare_packet(self, failing_stage):
        tool = self.root / "Selected Xcode.app/Contents/Developer/usr/bin/simctl"
        tool.parent.mkdir(parents=True)
        tool.write_bytes(b"SYNTHETIC_ACTIVE_TOOL")
        tool.chmod(0o700)
        app = tool.parent.parent.parent / "Applications/Simulator.app"
        app.mkdir(parents=True)
        observed_argv = []
        expected_exit = 61 if failing_stage == "resolve" else 73

        def native_control(argv, allowance, *, output):
            observed_argv.append(list(argv))
            stage = "resolve" if argv == ["xcrun", "--find", "simctl"] else "launch"
            failed = stage == failing_stage
            raw = (b"original selected-tool failure\n" if failed else
                   (str(tool) + "\n").encode())
            output.write(raw)
            return {"result": "EXIT_NONZERO" if failed else "PASS",
                    "exitCode": expected_exit if failed else 0, "elapsedMs": 1}

        def actual_prepare(root, udid, allowance, **kwargs):
            return boundary.launch.prepare(root, udid, allowance,
                                           runner=native_control, **kwargs)

        runner = Runner()
        phase = self.phase(runner, prepare=actual_prepare)
        self.assertFalse(phase.run_session())
        original = (phase.private / "prepare.original.json").read_bytes()
        receipt = json.loads(original)
        self.assertEqual(receipt, phase.prepare_receipt)
        self.assertEqual(receipt["commands"][-1]["stage"], failing_stage)
        self.assertEqual(receipt["commands"][-1]["result"], "EXIT_NONZERO")
        self.assertEqual(receipt["commands"][-1]["exitCode"], expected_exit)
        self.assertEqual(observed_argv[0], ["xcrun", "--find", "simctl"])
        if failing_stage == "launch":
            self.assertEqual(observed_argv[1],
                             ["open", "-g", "-a", str(app), "--args",
                              "-CurrentDeviceUDID", UDID])
        self.assertFalse(phase.boot_issued)
        self.assertFalse(any(command[1:3] in (["simctl", "bootstatus"],
                                             ["simctl", "getenv"])
                             for command in runner.commands))
        recipient = self.repo / boundary.RECIPIENT
        recipient.parent.mkdir(parents=True)
        recipient.write_bytes((Path(__file__).resolve().parents[2] /
                               boundary.RECIPIENT).read_bytes())
        summary = phase.seal()
        self.assertEqual(summary["status"], "BLOCKED")
        self.assertFalse(summary["formalAcceptanceGranted"])
        self.assertFalse(summary["shortcutExecuted"])
        member = next(row for row in summary["members"]
                      if row["name"] == "prepare.original.json")
        self.assertEqual(member["bytes"], len(original))
        self.assertEqual(member["sha256"], hashlib.sha256(original).hexdigest())
        with zipfile.ZipFile(phase.private / "session-originals.zip") as archive:
            self.assertEqual(archive.read("prepare.original.json"), original)
            for command in receipt["commands"]:
                stream = command["combinedOutput"]
                raw = archive.read(stream["path"])
                self.assertEqual(len(raw), stream["bytes"])
                self.assertEqual(hashlib.sha256(raw).hexdigest(), stream["sha256"])
        cipher = (phase.public / "session-originals.zip.p7m").read_bytes()
        self.assertEqual(summary["ciphertext"]["sha256"], hashlib.sha256(cipher).hexdigest())

    def test_failed_resolver_original_is_retained_in_sealed_packet(self):
        self.assert_failed_prepare_packet("resolve")

    def test_failed_open_original_is_retained_in_sealed_packet(self):
        self.assert_failed_prepare_packet("launch")

    def test_identity_types_and_phase_are_not_coerced(self):
        for key, value in (("runAttempt", True), ("runId", 123), ("phase", "any-command"),
                           ("compilerCommitSha", "main")):
            identity = {**IDENTITY, key: value}
            with self.assertRaises(ValueError):
                boundary.Boundary(self.repo, self.root / key, UDID, identity)

    def test_existing_output_prevents_a_second_session_action(self):
        phase = self.phase()
        with self.assertRaises(ValueError):
            self.phase()
        self.assertEqual(phase.commands, [])

    def test_original_process_receipts_record_exact_argv_and_streams(self):
        phase = self.phase()
        self.assertTrue(phase.run_session())
        records = sorted(phase.private.glob("command-*.process.json"))
        self.assertEqual(len(records), len(phase.commands))
        for path in records:
            row = json.loads(path.read_text())
            self.assertGreater(row["allowanceMs"], 0)
            self.assertLessEqual(row["allowanceMs"], boundary.STAGE_SECONDS[row["stage"]] * 1000)
            for stream in ("stdout", "stderr"):
                raw = (phase.private / row[stream]["path"]).read_bytes()
                self.assertEqual(row[stream]["bytes"], len(raw))
                self.assertEqual(row[stream]["sha256"], hashlib.sha256(raw).hexdigest())
        self.assertTrue((phase.private / "intent.original.json").is_file())

    def test_source_change_after_session_is_rejected(self):
        runner = Runner()
        original = runner.__call__
        def changed(argv, allowance, **kwargs):
            if argv[0] == "git" and runner.commands:
                runner.source = "b" * 40
            return original(argv, allowance, **kwargs)
        phase = self.phase(changed)
        self.assertFalse(phase.run_session())
        self.assertEqual(phase.status, "BLOCKED")
        self.assertIsNone(phase.source_after)

    def test_orchestrator_uses_both_boundaries_and_keeps_original_runtime_oracle(self):
        repo = Path(__file__).resolve().parents[2]
        script = (repo / "scripts/ios27_shortcuts_simulator_qa.sh").read_text()
        self.assertEqual(script.count("if ! ensure_phase_session_ready pre-import; then"), 1)
        self.assertEqual(script.count("if ! ensure_phase_session_ready post-test-runtime; then"), 1)
        self.assertIn("Pre-import selected-device session boundary was not verified.", script)
        self.assertIn("Post-XCTest selected-device session boundary was not verified.", script)
        self.assertTrue("from nested_list_runtime import apply_assertions" in script)
        self.assertTrue("exact runtime/trace contract was not verified" in script)
        self.assertNotIn("simctl erase", Path(boundary.__file__).read_text())
        self.assertNotIn("simctl create", Path(boundary.__file__).read_text())

    def test_deadline_exhaustion_starts_no_child(self):
        samples = iter([0, 241, 241])
        runner = Runner()
        phase = self.phase(runner, now=lambda: next(samples))
        self.assertFalse(phase.run_session())
        self.assertEqual(runner.commands, [])

    def test_sealed_packet_uses_existing_recipient_and_public_has_no_raw_device(self):
        phase = self.phase()
        self.assertTrue(phase.run_session())
        existing_repo = Path(__file__).resolve().parents[2]
        recipient = self.repo / boundary.RECIPIENT
        recipient.parent.mkdir(parents=True)
        recipient.write_bytes((existing_repo / boundary.RECIPIENT).read_bytes())
        summary = phase.seal()
        self.assertEqual({p.name for p in phase.public.iterdir()},
                         {"summary.json", "session-originals.zip.p7m"})
        self.assertNotIn(UDID.encode(), (phase.public / "summary.json").read_bytes())
        self.assertFalse(summary["formalAcceptanceGranted"])
        self.assertFalse(summary["shortcutExecuted"])
        cipher = (phase.public / "session-originals.zip.p7m").read_bytes()
        self.assertEqual(summary["ciphertext"]["sha256"], hashlib.sha256(cipher).hexdigest())
        self.assertEqual(summary["plaintextArchive"]["sha256"],
                         hashlib.sha256((phase.private / "session-originals.zip").read_bytes()).hexdigest())


class StateTests(unittest.TestCase):
    def test_target_identity_not_name_or_substring(self):
        other = "AAAAAAAA-2222-3333-4444-555555555555"
        with self.assertRaises(ValueError):
            boundary.state_observation(devices(udid=other), UDID)

    def test_wrong_runtime_unknown_state_and_unavailable_are_blocked(self):
        for raw in (devices(runtime="com.apple.CoreSimulator.SimRuntime.iOS-26-0"),
                    devices(state="Unknown"), devices(available=False)):
            with self.assertRaises(ValueError):
                boundary.state_observation(raw, UDID)

    def test_runtime_suffix_cannot_publish_arbitrary_text(self):
        with self.assertRaises(ValueError):
            boundary.state_observation(devices(
                runtime="com.apple.CoreSimulator.SimRuntime.iOS-27-0-CHERRI_PRIVATE_CANARY"), UDID)

    def test_duplicate_target_and_duplicate_json_keys_are_blocked(self):
        value = json.loads(devices())
        rows = next(iter(value["devices"].values()))
        rows.append(dict(rows[0]))
        for raw in (json.dumps(value).encode(), b'{"devices":{},"devices":{}}'):
            with self.assertRaises(ValueError):
                boundary.state_observation(raw, UDID)

    def test_invalid_utf8_and_oversize_do_not_establish_state(self):
        for raw in (b"\xff", b"x" * (boundary.LIMIT + 1)):
            with self.assertRaises((ValueError, UnicodeError)):
                boundary.state_observation(raw, UDID)


if __name__ == "__main__":
    unittest.main()
