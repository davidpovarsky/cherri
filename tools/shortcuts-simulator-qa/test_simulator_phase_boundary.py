import hashlib
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
import zipfile
from unittest import mock

import simulator_phase_boundary as boundary

UDID = "11111111-2222-3333-4444-555555555555"
SOURCE = "a" * 40
IDENTITY = {"compilerCommitSha": SOURCE, "runId": "12345", "runAttempt": "1",
            "fixtureId": "qa-nested-list-control-v2", "phase": "pre-import"}

PRE_IMPORT_START = "    # Stage 3.2: Pre-XCTest outside-runner signed .shortcut import so simctl openurl file:// never contends with active AX/XCTest\n"
PRE_IMPORT_END = '    HANDSHAKE_PID=""\n'
PRE_IMPORT_GUARD = '''      if ! ensure_phase_session_ready pre-import; then
        echo "Pre-import selected-device session boundary was not verified."
        exit 70
      fi
'''


def pre_import_stage(script):
    # Execute the actual mutually exclusive branches, without running the
    # surrounding orchestrator or substituting a second routing model.
    if script.count(PRE_IMPORT_START) != 1:
        raise AssertionError("one original pre-import stage is required")
    remaining = script.split(PRE_IMPORT_START, 1)[1]
    if PRE_IMPORT_END not in remaining:
        raise AssertionError("pre-import stage must end before the handshake")
    return remaining.split(PRE_IMPORT_END, 1)[0]


def pre_import_execution(stage, hosted, signed, ready=True, prepared=True, file_state="present"):
    with tempfile.TemporaryDirectory() as temporary:
        root = Path(temporary)
        (root / "artifacts/editor").mkdir(parents=True)
        signed_file = root / "synthetic.shortcut"
        if file_state != "missing":
            signed_file.write_bytes(b"synthetic test input" if file_state == "present" else b"")
        trace = root / "trace.txt"
        trace.write_text("")
        env = dict(os.environ, HOSTED_IMPORT_IDENTITY=str(hosted).lower(),
                   SIGNED_ARTIFACT_VALID_AEA1=str(signed).lower(), SIGNED_FILE=str(signed_file),
                   HOSTED_IMPORT_ROOT=str(root / "hosted-import"), HOSTED_IMPORT_PREPARED="false",
                   ROOT_DIR=str(root), ARTIFACT_DIR=str(root / "artifacts"),
                   QA_SCRATCH_DIR=str(root / "scratch"), TRACE_FILE=str(trace), UDID=UDID,
                   SOURCE_SHA=SOURCE, RUN_ID="12345", GITHUB_RUN_ATTEMPT="1",
                   TARGET_FIXTURE_ID="qa-nested-list-control-v2", EXPECTED_SHORTCUT_NAME="Synthetic",
                   EXEC_RUN_NONCE="synthetic-nonce", XCODEBUILD_BUILD_RC="0",
                   BOUNDARY_RC="0" if ready else "1", PREPARE_RC="0" if prepared else "1")
        harness = r'''set -uo pipefail
event() { printf '%s\n' "$1" >> "$TRACE_FILE"; }
ensure_phase_session_ready() { event "boundary:$1"; return "$BOUNDARY_RC"; }
run_cmd_timeout() {
  if [ "$2:$3:$4" = "xcrun:simctl:launch" ]; then event "warm:$1";
  elif [ "$2:$3:$4" = "xcrun:simctl:io" ]; then event "screenshot:$1";
  else event unexpected-native-command; return 96; fi
}
python3() {
  if [ "$1" = "-B" ] && [ "$2" = "$ROOT_DIR/tools/shortcuts-simulator-qa/hosted_import_observation.py" ] && [ "$3" = "prepare" ]; then
    event hosted-prepare; return "$PREPARE_RC"
  fi
  event unexpected-python-command; return 96
}
simctl_openurl_resilient() {
  if [ "$1" != "$UDID" ] || [ "$2" != "file://$SIGNED_FILE" ]; then
    event wrong-signed-file; return 96
  fi
  event "direct-import:$6"; printf 'synthetic successful transport'; return 0
}
simctl_getenv_ok() { return 0; }
sleep() { :; }
xcrun() { event unexpected-raw-native-command; return 96; }
xcodebuild() { event unexpected-build-command; return 96; }
import_signed_via_localhost_http() { event unexpected-fallback; return 96; }
wait_for_springboard_display() { event unexpected-recovery; return 96; }
'''
        result = subprocess.run(["bash", "-c", harness + stage +
                                 '\nprintf "PREPARED=%s\\n" "$HOSTED_IMPORT_PREPARED"\n'],
                                env=env, capture_output=True, text=True, timeout=5)
        return result, trace.read_text().splitlines()


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
        self.root = Path(self.temporary.name).resolve(strict=True)
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

    def test_temporary_parent_alias_is_resolved_before_sealing(self):
        physical = self.root / "physical"
        physical.mkdir()
        alias = self.root / "temp-alias"
        alias.symlink_to(physical, target_is_directory=True)
        original_factory = tempfile.TemporaryDirectory
        with mock.patch.object(tempfile, "TemporaryDirectory",
                               side_effect=lambda: original_factory(dir=alias)):
            nested = PhaseTests(
                "test_sealed_packet_uses_existing_recipient_and_public_has_no_raw_device")
            try:
                nested.setUp()
                nested.test_sealed_packet_uses_existing_recipient_and_public_has_no_raw_device()
                self.assertEqual(nested.root, nested.root.resolve(strict=True))
            finally:
                nested.doCleanups()

    def test_untrusted_recipient_ancestor_alias_remains_rejected(self):
        alias = self.root / "untrusted-alias"
        alias.symlink_to(self.root, target_is_directory=True)
        recipient = self.repo / boundary.RECIPIENT
        recipient.parent.mkdir(parents=True)
        recipient.write_bytes((Path(__file__).resolve().parents[2] /
                               boundary.RECIPIENT).read_bytes())
        phase = boundary.Boundary(alias / "repo", self.root / "packet", UDID, IDENTITY,
                                  runner=Runner(), prepare=prepared)
        self.assertTrue(phase.run_session())
        with self.assertRaisesRegex(boundary.observation.Rejected, "ANCESTOR_NOT_DIRECTORY"):
            phase.seal()
        self.assertEqual(list(phase.public.iterdir()), [])

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
        self.assert_pre_import_routes(pre_import_stage(script))
        self.assertEqual(script.count("if ! ensure_phase_session_ready post-test-runtime; then"), 1)
        self.assertIn("Pre-import selected-device session boundary was not verified.", script)
        self.assertIn("Post-XCTest selected-device session boundary was not verified.", script)
        self.assertTrue("from nested_list_runtime import apply_assertions" in script)
        self.assertTrue("exact runtime/trace contract was not verified" in script)
        self.assertNotIn("simctl erase", Path(boundary.__file__).read_text())
        self.assertNotIn("simctl create", Path(boundary.__file__).read_text())

    def assert_pre_import_routes(self, stage):
        # Native operations are explicit recording doubles. The original bash
        # decides the branch, boundary order, rejection and number of calls.
        cases = (
            ("selected", True, True, True, True, "present", 0, "true",
             ["boundary:pre-import", "warm:20", "hosted-prepare"]),
            ("selected_prepare_rejected", True, True, True, False, "present", 0, "false",
             ["boundary:pre-import", "warm:20", "hosted-prepare"]),
            ("selected_unsigned_rejected", True, False, True, False, "present", 0, "false",
             ["boundary:pre-import", "warm:20", "hosted-prepare"]),
            ("ordinary_signed", False, True, True, True, "present", 0, "false",
             ["boundary:pre-import", "warm:20", "direct-import:45", "screenshot:15"]),
            ("ordinary_unsigned", False, False, True, True, "present", 0, "false", []),
            ("ordinary_missing_file", False, True, True, True, "missing", 0, "false", []),
            ("ordinary_empty_file", False, True, True, True, "empty", 0, "false", []),
            ("selected_session_rejected", True, True, False, True, "present", 70, None,
             ["boundary:pre-import"]),
            ("ordinary_session_rejected", False, True, False, True, "present", 70, None,
             ["boundary:pre-import"]),
        )
        for name, hosted, signed, ready, prepared, file_state, code, final_prepared, expected in cases:
            result, events = pre_import_execution(stage, hosted, signed, ready, prepared, file_state)
            self.assertEqual(events, expected, name + " event order")
            self.assertEqual(result.returncode, code, name + ": " + result.stderr)
            self.assertEqual(result.stderr, "", name)
            if final_prepared is None:
                self.assertNotIn("PREPARED=", result.stdout, name)
            else:
                self.assertEqual(result.stdout.splitlines()[-1], "PREPARED=" + final_prepared, name)

    def test_pre_import_route_oracle_rejects_missing_moved_and_duplicate_operations(self):
        repo = Path(__file__).resolve().parents[2]
        stage = pre_import_stage((repo / "scripts/ios27_shortcuts_simulator_qa.sh").read_text())
        branch = '    elif [ "$SIGNED_ARTIFACT_VALID_AEA1" = "true" ] && [ -n "${SIGNED_FILE:-}" ] && [ -s "${SIGNED_FILE:-}" ]; then\n'
        self.assertEqual(stage.count(branch), 1)
        selected, ordinary = stage.split(branch)
        self.assertEqual(selected.count(PRE_IMPORT_GUARD), 1)
        self.assertEqual(ordinary.count(PRE_IMPORT_GUARD), 1)
        dispatch = '      PRE_IMP_OUT="$(simctl_openurl_resilient "$UDID" "file://$SIGNED_FILE" 1 "" 0 45)" || PRE_IMP_RC=$?\n'
        self.assertEqual(ordinary.count(dispatch), 1)
        ordinary_late = ordinary.replace(PRE_IMPORT_GUARD, "", 1)
        prefix, suffix = ordinary_late.rsplit("    fi\n", 1)
        mutations = {
            "missing_selected_guard": selected.replace(PRE_IMPORT_GUARD, "", 1) + branch + ordinary,
            "missing_ordinary_guard": selected + branch + ordinary.replace(PRE_IMPORT_GUARD, "", 1),
            "both_guards_in_ordinary_branch": selected.replace(PRE_IMPORT_GUARD, "", 1) + branch + ordinary.replace(PRE_IMPORT_GUARD, PRE_IMPORT_GUARD * 2, 1),
            "wrong_phase": selected.replace("ensure_phase_session_ready pre-import", "ensure_phase_session_ready post-test-runtime", 1) + branch + ordinary,
            "wrong_selected_condition": selected.replace('$HOSTED_IMPORT_IDENTITY" = "true"', '$HOSTED_IMPORT_IDENTITY" = "false"', 1) + branch + ordinary,
            "selected_guard_after_prepare": selected.replace(PRE_IMPORT_GUARD, "", 1) + PRE_IMPORT_GUARD + branch + ordinary,
            "ordinary_guard_after_dispatch": selected + branch + prefix + PRE_IMPORT_GUARD + "    fi\n" + suffix,
            "duplicate_selected_guard": selected.replace(PRE_IMPORT_GUARD, PRE_IMPORT_GUARD * 2, 1) + branch + ordinary,
            "selected_rejection_falls_through": selected.replace("        exit 70\n", "        :\n", 1) + branch + ordinary,
            "ordinary_rejection_falls_through": selected + branch + ordinary.replace("        exit 70\n", "        :\n", 1),
            "selected_adds_ordinary_dispatch": selected + dispatch + branch + ordinary,
            "duplicate_ordinary_dispatch": selected + branch + ordinary.replace(dispatch, dispatch * 2, 1),
            "both_import_branches_execute": selected + "    fi\n" + branch.replace("    elif ", "    if ", 1) + ordinary,
        }
        for name, altered in mutations.items():
            with self.subTest(perturbation=name):
                self.assertNotEqual(altered, stage)
                syntax = subprocess.run(["bash", "-n"], input=altered, capture_output=True,
                                        text=True, timeout=5)
                self.assertEqual(syntax.returncode, 0, syntax.stderr)
                if name == "both_guards_in_ordinary_branch":
                    self.assertEqual(altered.count(PRE_IMPORT_GUARD), 2)
                with self.assertRaises(AssertionError):
                    self.assert_pre_import_routes(altered)

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
