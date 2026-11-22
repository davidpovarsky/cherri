import copy
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

import prepare_public_artifacts as publication
import simulator_session_recovery as recovery
import xcode_destination_readiness as destination
import xcode_destination_diagnostic as diagnostic

UDID = "11111111-2222-3333-4444-555555555555"
OTHER = "AAAAAAAA-BBBB-CCCC-DDDD-EEEEEEEEEEEE"
SOURCE, RUN, FIXTURE = "a" * 40, "123456", "qa-destination-control-v2"
IDENTITY = {"compilerCommitSha": SOURCE, "runId": RUN, "fixtureId": FIXTURE}


def listing(device=UDID, classic=False, extra=""):
    header = (f'Available destinations for the "{destination.SCHEME}" scheme:' if classic else
              f'Destinations compatible with the "{destination.SCHEME}" scheme:')
    return (header + "\n\t{ platform:iOS Simulator, arch:arm64, id:" + device +
            ", OS:27.1, name:PUBLIC_OWNED_SIMULATOR }\n" + extra).encode()


class DestinationTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.repo, self.scratch, self.artifacts = [self.root / x for x in ("repo", "scratch", RUN)]
        for path in (self.repo / destination.PROJECT, self.scratch,
                     self.artifacts / "builds", self.artifacts / "runtime"):
            path.mkdir(parents=True)
        self.tool = self.root / "Selected Xcode.app/Contents/Developer/usr/bin/simctl"
        self.tool.parent.mkdir(parents=True)
        self.tool.write_bytes(b"SYNTHETIC_ACTIVE_TOOL")
        self.tool.chmod(0o700)
        self.simulator = self.root / "Selected Xcode.app/Contents/Developer/Applications/Simulator.app"
        self.simulator.mkdir(parents=True)
        self.clock = 10.0
        self.calls = []
        self.outputs = [listing()]
        self.test_code = 0

    def runner(self, args, allowance, *, output=None):
        self.calls.append((args, allowance))
        self.clock += 1
        code = 0
        if "-showdestinations" in args:
            output.write(self.outputs.pop(0))
        elif args == ["xcrun", "--find", "simctl"]:
            output.write((str(self.tool) + "\n").encode())
        elif "test-without-building" in args:
            code = self.test_code
            output.write(b"PUBLIC_SYNTHETIC_XCODE_TEST_OUTPUT\n")
        return {"result": "PASS" if code == 0 else "EXIT_NONZERO", "exitCode": code, "elapsedMs": 1000}

    def phase(self, runner=None, identity=None):
        return destination.DestinationTestPhase(self.repo, self.scratch, self.artifacts,
            UDID, identity or IDENTITY, runner=runner or self.runner, now=lambda: self.clock)

    def independent_case(self, name):
        # Each synthetic unit-control invocation has its own artifact root.
        # Production must never overwrite a prior invocation's originals.
        self.artifacts = self.root / ("case-" + name)
        (self.artifacts / "builds").mkdir(parents=True)
        (self.artifacts / "runtime").mkdir()

    def test_exact_available_target_and_both_observed_header_forms(self):
        for classic in (False, True):
            value = destination.destination_observation(listing(classic=classic), UDID)
            self.assertEqual(value["status"], "AVAILABLE")
            self.assertEqual(value["availableMatches"], 1)
            self.assertEqual(value["outputSha256"], hashlib.sha256(listing(classic=classic)).hexdigest())
            self.assertNotIn("PUBLIC_OWNED_SIMULATOR", json.dumps(value))
            self.assertNotIn(UDID, json.dumps(value))

    def test_placeholder_other_platform_name_and_id_substring_never_authorize(self):
        controls = [
            listing(OTHER), listing("dvtdevice-DVTiOSDeviceSimulatorPlaceholder-iphonesimulator:placeholder"),
            listing(OTHER).replace(b"PUBLIC_OWNED_SIMULATOR", UDID.encode()),
            listing(UDID + "0"), listing().replace(b"platform:iOS Simulator", b"platform:iOS"),
            listing().replace(b"OS:27.1", b"error:runtime unavailable"),
            listing().replace(destination.SCHEME.encode(), b"WrongScheme"),
            listing() + listing(), listing() + listing().split(b"\n", 1)[1],
            listing().replace(b"id:" + UDID.encode(), b"id:" + UDID.encode() + b", id:" + UDID.encode()),
            listing().replace(b" }", b" "), b"\xff", b"\x00" + listing(), b"X" * (destination.MAX_OUTPUT + 1),
        ]
        for data in controls:
            with self.subTest(data_sha=hashlib.sha256(data).hexdigest()):
                self.assertNotEqual(destination.destination_observation(data, UDID)["status"], "AVAILABLE")

    def test_incompatible_and_conflicting_target_rows_remain_blocked(self):
        bad = (f'Destinations incompatible with the "{destination.SCHEME}" scheme:\n'
               f'{{ platform:iOS Simulator, id:{UDID}, error:Unavailable }}\n').encode()
        self.assertEqual(destination.destination_observation(listing(OTHER) + bad, UDID)["status"], "INELIGIBLE")
        self.assertEqual(destination.destination_observation(listing() + bad, UDID)["status"], "AMBIGUOUS")

    def test_immediate_ready_keeps_exact_original_test_and_shared_allowance(self):
        code, receipt = self.phase().run()
        destination.validate_receipt(receipt, SOURCE, RUN, FIXTURE)
        self.assertEqual(code, 0)
        self.assertFalse(receipt["refreshAttempted"])
        self.assertEqual([x["stage"] for x in receipt["commands"]], ["destinations", "test"])
        args, allowance = self.calls[-1]
        self.assertEqual(args, ["xcodebuild", "test-without-building", "-project", str(self.repo / destination.PROJECT),
            "-scheme", destination.SCHEME, "-derivedDataPath", str(self.scratch / "DerivedData"),
            "-destination", "platform=iOS Simulator,id=" + UDID,
            "-only-testing:" + destination.TEST, "-resultBundlePath", str(self.scratch / "CherriSimulatorQA.xcresult"),
            "CODE_SIGNING_ALLOWED=NO"])
        self.assertEqual(allowance, 899)
        self.assertEqual(list(self.scratch.iterdir()), [])
        self.assertFalse(receipt["formalAcceptanceGranted"])

    def test_missing_target_one_owned_non_reboot_refresh_then_fresh_query(self):
        self.outputs = [listing(OTHER), listing()]
        code, receipt = self.phase().run()
        self.assertEqual(code, 0)
        self.assertTrue(receipt["refreshAttempted"])
        self.assertEqual([x["stage"] for x in receipt["commands"]], ["destinations", "prepare", "bootstatus", "destinations", "test"])
        self.assertEqual(self.calls[1][0], ["xcrun", "--find", "simctl"])
        self.assertEqual(self.calls[2][0], ["open", "-g", "-a", str(self.simulator), "--args", "-CurrentDeviceUDID", UDID])
        self.assertEqual(self.calls[3][0], ["xcrun", "simctl", "bootstatus", UDID, "-b"])
        self.assertEqual(self.calls[1][1], 5)
        self.assertEqual(self.calls[2][1], 14)
        self.assertEqual(self.calls[-1][1], 895)
        self.assertNotIn("shutdown", json.dumps(self.calls))
        self.assertNotIn("openurl", json.dumps(self.calls))
        destination.validate_receipt(receipt)

    def test_still_missing_never_tests_or_retries_import(self):
        self.outputs = [listing(OTHER), listing(OTHER)]
        code, receipt = self.phase().run()
        self.assertEqual(code, 70)
        self.assertEqual(receipt["status"], "BLOCKED_DESTINATION")
        self.assertFalse(receipt["testStarted"])
        self.assertEqual(len(self.calls), 5)
        self.assertFalse((self.artifacts / "builds/xcodebuild-test.log").exists())

    def test_headless_active_tool_requires_owned_bootstatus_and_fresh_available_destination(self):
        self.simulator.rmdir()
        self.tool.chmod(0o700)
        self.outputs = [listing(OTHER), listing()]
        code, receipt = self.phase().run()
        self.assertEqual(code, 0)
        self.assertEqual([x["stage"] for x in receipt["commands"]],
            ["destinations", "prepare", "bootstatus", "destinations", "test"])
        self.assertFalse(any(args[0] == "open" for args, _ in self.calls))
        self.assertEqual(self.calls[2][0], ["xcrun", "simctl", "bootstatus", UDID, "-b"])
        self.assertEqual(sum("-showdestinations" in args for args, _ in self.calls), 2)
        self.assertEqual(sum("test-without-building" in args for args, _ in self.calls), 1)
        observed = self.diagnostic()["launch"]
        self.assertEqual(observed["selectionStatus"], "HEADLESS_TOOL_RESOLVED")
        self.assertEqual(observed["preparationMode"], "HEADLESS_BOOTSTATUS")
        self.assertFalse(observed["simulatorAppExists"])
        self.assertTrue(observed["toolExecutable"])
        self.assertTrue(observed["toolBoundToActiveDeveloper"])
        self.assertEqual([x["stage"] for x in observed["commands"]], ["resolve"])
        self.assertEqual(receipt["phaseBudgetMs"], 900000)
        self.assertEqual(receipt["gateBudgetMs"], 90000)
        self.assertFalse(receipt["formalAcceptanceGranted"])
        destination.validate_receipt(receipt, artifact_root=self.artifacts)

    def test_headless_missing_ineligible_ambiguous_or_late_fresh_query_never_tests(self):
        self.simulator.rmdir()
        bad = (f'Destinations incompatible with the "{destination.SCHEME}" scheme:\n'
               f'{{ platform:iOS Simulator, id:{UDID}, error:Unavailable }}\n').encode()
        cases = [("missing", listing(OTHER)), ("ineligible", listing(OTHER) + bad),
                 ("ambiguous", listing() + bad), ("late", listing())]
        for name, final in cases:
            with self.subTest(name=name):
                self.independent_case("headless-query-" + name)
                self.outputs, self.calls = [listing(OTHER), final], []
                queries = []
                def runner(args, allowance, *, output=None):
                    result = self.runner(args, allowance, output=output)
                    if "-showdestinations" in args:
                        queries.append(True)
                        if name == "late" and len(queries) == 2:
                            self.clock += 91; result["elapsedMs"] += 91000
                    return result
                code, receipt = self.phase(runner).run()
                self.assertEqual(code, 70)
                self.assertFalse(receipt["testStarted"])
                self.assertEqual(len(queries), 2)
                self.assertEqual(sum("bootstatus" in args for args, _ in self.calls), 1)
                self.assertFalse(any(args[0] == "open" or "test-without-building" in args for args, _ in self.calls))
                destination.validate_receipt(receipt, artifact_root=self.artifacts)

    def test_headless_failed_timed_out_or_late_bootstatus_cannot_requery(self):
        self.simulator.rmdir()
        for name, result, code, elapsed in (("failed", "EXIT_NONZERO", 72, 1),
                ("timeout", "TIMEOUT", None, 60), ("late", "PASS", 0, 61)):
            with self.subTest(name=name):
                self.independent_case("headless-boot-" + name)
                self.outputs, self.calls = [listing(OTHER)], []
                def runner(args, allowance, *, output=None):
                    if "bootstatus" in args:
                        self.calls.append((args, allowance)); self.clock += elapsed
                        return {"result": result, "exitCode": code, "elapsedMs": elapsed * 1000}
                    return self.runner(args, allowance, output=output)
                actual, receipt = self.phase(runner).run()
                self.assertEqual(actual, 70)
                self.assertFalse(receipt["testStarted"])
                self.assertEqual(len(self.calls), 3)
                self.assertEqual(len(receipt["queries"]), 1)
                self.assertFalse(any(args[0] == "open" for args, _ in self.calls))
                destination.validate_receipt(receipt, artifact_root=self.artifacts)

    def test_headless_requires_regular_executable_tool_in_same_physical_developer_root(self):
        self.simulator.rmdir()
        foreign = self.root / "Foreign Xcode.app/Contents/Developer/usr/bin/simctl"
        foreign.parent.mkdir(parents=True); foreign.write_bytes(b"FOREIGN_TOOL"); foreign.chmod(0o700)
        original_resolve = Path.resolve
        for name in ("missing", "directory", "not-executable", "foreign-resolved-tool"):
            with self.subTest(name=name):
                self.independent_case("headless-tool-" + name)
                self.outputs, self.calls = [listing(OTHER)], []
                self.tool.unlink()
                if name == "directory":
                    self.tool.mkdir()
                elif name != "missing":
                    self.tool.write_bytes(b"SYNTHETIC_ACTIVE_TOOL"); self.tool.chmod(0o700)
                def resolve(path, *args, **kwargs):
                    return foreign if name == "foreign-resolved-tool" and path == self.tool else original_resolve(path, *args, **kwargs)
                physical_tool = original_resolve(self.tool)
                intercepted_access = []
                actual_access = os.access
                def access(path, mode):
                    if name == "not-executable" and Path(path) == physical_tool:
                        intercepted_access.append((Path(path), mode))
                        return False
                    return actual_access(path, mode)
                with patch.object(Path, "resolve", resolve), patch.object(diagnostic.os, "access", access):
                    code, receipt = self.phase().run()
                if name == "not-executable":
                    self.assertEqual(intercepted_access, [(physical_tool, os.X_OK)])
                self.assertEqual((code, len(self.calls)), (70, 2))
                self.assertFalse(receipt["testStarted"])
                self.assertEqual(self.diagnostic()["launch"]["selectionStatus"], "RESOLUTION_REJECTED")
                destination.validate_receipt(receipt, artifact_root=self.artifacts)
                if self.tool.is_dir():self.tool.rmdir()
                if not self.tool.exists():self.tool.write_bytes(b"SYNTHETIC_ACTIVE_TOOL"); self.tool.chmod(0o700)

    def test_headless_cannot_replace_invalid_or_foreign_application_with_absence(self):
        original_resolve, original_symlink = Path.resolve, Path.is_symlink
        foreign = self.root / "Foreign Xcode.app/Contents/Developer/Applications/Simulator.app"
        foreign.mkdir(parents=True)
        for name in ("regular-file", "dangling-link", "foreign-application"):
            with self.subTest(name=name):
                self.independent_case("headless-app-" + name)
                self.outputs, self.calls = [listing(OTHER)], []
                if self.simulator.is_dir():self.simulator.rmdir()
                if self.simulator.is_file():self.simulator.unlink()
                if name == "regular-file":self.simulator.write_bytes(b"NOT_AN_APP_BUNDLE")
                if name == "foreign-application":self.simulator.mkdir()
                def resolve(path, *args, **kwargs):
                    return foreign if name == "foreign-application" and path == self.simulator else original_resolve(path, *args, **kwargs)
                def symlink(path):
                    return True if name == "dangling-link" and path == self.simulator else original_symlink(path)
                with patch.object(Path, "resolve", resolve), patch.object(Path, "is_symlink", symlink):
                    code, receipt = self.phase().run()
                self.assertEqual((code, len(self.calls)), (70, 2))
                self.assertFalse(receipt["testStarted"])
                self.assertEqual(self.diagnostic()["launch"]["selectionStatus"], "RESOLUTION_REJECTED")
                destination.validate_receipt(receipt, artifact_root=self.artifacts)

    def test_headless_receipt_rejects_fabricated_gui_tool_scope_and_version(self):
        self.simulator.rmdir(); self.outputs = [listing(OTHER), listing()]
        _, receipt = self.phase().run(); valid = self.diagnostic()
        mutations = {
            "tool executable false": lambda x: x["launch"].update(toolExecutable=False),
            "tool executable Boolean": lambda x: x["launch"].update(toolExecutable=1),
            "foreign tool": lambda x: x["launch"].update(toolBoundToActiveDeveloper=False),
            "app not absent": lambda x: x["launch"].update(simulatorAppAbsent=False),
            "app exists": lambda x: x["launch"].update(simulatorAppExists=True),
            "app bound": lambda x: x["launch"].update(simulatorAppBoundToActiveDeveloper=True),
            "claims GUI": lambda x: x["launch"].update(preparationMode="GRAPHICAL_LAUNCH"),
            "claims launch": lambda x: x["launch"].update(selectionStatus="BOUND_TO_ACTIVE_XCODE"),
            "old launch schema": lambda x: x["launch"].update(schemaVersion="1.0"),
            "formal promotion": lambda x: x["launch"].update(formalAcceptanceGranted=True),
        }
        for name, mutate in mutations.items():
            with self.subTest(name=name):
                changed = copy.deepcopy(valid); mutate(changed)
                with self.assertRaises(ValueError):diagnostic.validate(changed, self.artifacts, receipt)
        older = copy.deepcopy(receipt); older["schemaVersion"] = "1.1"
        with self.assertRaisesRegex(ValueError, "DESTINATION_COMMAND_ORDER"):
            destination.validate_receipt(older)

    def test_original_version_one_point_one_graphical_receipt_remains_valid(self):
        self.outputs = [listing(OTHER), listing()]
        _, receipt = self.phase().run()
        old = copy.deepcopy(receipt); old["schemaVersion"] = "1.1"
        old["commands"][1]["stage"] = "open"
        launch = self.diagnostic()["launch"]; launch["schemaVersion"] = "1.0"
        for field in ("preparationMode", "toolExecutable", "toolBoundToActiveDeveloper",
                      "simulatorAppAbsent", "simulatorAppBoundToActiveDeveloper"):
            del launch[field]
        original = diagnostic.document(self.artifacts, old, launch)
        diagnostic.validate(original, self.artifacts, old)
        (self.artifacts / "runtime/xcode-destination-command-diagnostic.json").write_bytes(diagnostic.encoded(original))
        destination.validate_receipt(old, artifact_root=self.artifacts)

    def test_ineligible_or_unknown_metadata_does_not_refresh(self):
        for index, output in enumerate((listing().replace(b"OS:27.1", b"error:unsupported"), b"unknown output")):
            self.independent_case(str(index))
            self.outputs, self.calls = [output], []
            code, receipt = self.phase().run()
            self.assertEqual((code, len(self.calls), receipt["refreshAttempted"]), (70, 1, False))

    def test_query_failure_timeout_or_late_zero_exit_never_starts_test(self):
        for outcome, code, delay in (("EXIT_NONZERO", 70, 1), ("TIMEOUT", None, 30), ("PASS", 0, 31)):
            self.independent_case(outcome)
            self.calls = []
            def runner(args, allowance, *, output=None):
                self.calls.append((args, allowance))
                output.write(listing())
                self.clock += delay
                return {"result": outcome, "exitCode": code, "elapsedMs": delay * 1000}
            result, receipt = self.phase(runner).run()
            self.assertEqual(result, 70)
            self.assertFalse(receipt["testStarted"])
            self.assertEqual(len(self.calls), 1)
            self.assertFalse(receipt["refreshAttempted"])

    def test_refresh_timeout_stops_without_requery_or_test(self):
        self.outputs = [listing(OTHER)]
        def runner(args, allowance, *, output=None):
            if "bootstatus" in args:
                self.calls.append((args, allowance))
                self.clock += allowance
                return {"result": "TIMEOUT", "exitCode": None, "elapsedMs": round(allowance * 1000)}
            return self.runner(args, allowance, output=output)
        code, receipt = self.phase(runner).run()
        self.assertEqual((code, len(self.calls)), (70, 4))
        self.assertFalse(receipt["testStarted"])
        self.assertEqual(receipt["commands"][-1]["result"], "TIMEOUT")

    def test_gate_clock_expiry_cannot_be_repaired_by_positive_query(self):
        self.outputs = [listing(OTHER), listing()]
        def runner(args, allowance, *, output=None):
            result = self.runner(args, allowance, output=output)
            if len(self.calls) == 5:
                self.clock += 100
                result["elapsedMs"] += 100000
            return result
        code, receipt = self.phase(runner).run()
        self.assertEqual(code, 70)
        self.assertEqual(receipt["queries"][-1]["status"], "AVAILABLE")
        self.assertFalse(receipt["testStarted"])
        self.assertGreater(receipt["gateElapsedMs"], receipt["gateBudgetMs"])

    def test_actual_xcode_failure_and_timeout_are_preserved_after_ready(self):
        self.test_code = 65
        code, receipt = self.phase().run()
        self.assertEqual(code, 65)
        self.assertEqual(receipt["commands"][-1]["result"], "EXIT_NONZERO")
        self.assertTrue(receipt["testStarted"])
        self.independent_case("test-timeout")
        self.outputs, self.calls = [listing()], []
        def runner(args, allowance, *, output=None):
            if "test-without-building" in args:
                self.calls.append((args, allowance))
                self.clock += allowance
                return {"result": "TIMEOUT", "exitCode": None, "elapsedMs": round(allowance * 1000)}
            return self.runner(args, allowance, output=output)
        code, receipt = self.phase(runner).run()
        self.assertEqual(code, 124)
        self.assertEqual(receipt["elapsedMs"], 900000)

    def test_receipt_rejects_false_metadata_identity_time_or_acceptance(self):
        _, valid = self.phase().run()
        mutations = {
            "wrong source": lambda x: x.update(compilerCommitSha="b" * 40),
            "wrong run": lambda x: x.update(runId="another"),
            "wrong fixture": lambda x: x.update(fixtureId="another"),
            "private field": lambda x: x.update(rawOutput="unreviewed"),
            "formal promotion": lambda x: x.update(formalAcceptanceGranted=True),
            "alternate test": lambda x: x.update(testIdentifier="other"),
            "alternate device": lambda x: x["queries"][0].update(targetDeviceIdSha256="f" * 64),
            "missing source row": lambda x: x["queries"][0].update(availableMatches=0),
            "contradictory ineligible": lambda x: x["queries"][0].update(ineligibleMatches=1),
            "oversized positive": lambda x: x["queries"][0].update(outputBytes=destination.MAX_OUTPUT + 1),
            "query exit failure": lambda x: x["commands"][0].update(exitCode=70),
            "late stage": lambda x: x["commands"][0].update(elapsedMs=31000),
            "understated total": lambda x: x.update(elapsedMs=1),
            "understated gate": lambda x: x.update(gateElapsedMs=0),
            "unbudgeted query": lambda x: x["commands"][-1].update(allowanceMs=900000),
            "invented refresh": lambda x: x.update(refreshAttempted=True),
            "missing query": lambda x: x.update(queries=[]),
            "unknown command": lambda x: x["commands"][0].update(stage="shutdown"),
            "wrong sequence": lambda x: x["commands"].reverse(),
            "Boolean index": lambda x: x["queries"][0].update(commandIndex=False),
        }
        for name, mutate in mutations.items():
            with self.subTest(name=name):
                changed = copy.deepcopy(valid)
                mutate(changed)
                with self.assertRaises(ValueError):
                    destination.validate_receipt(changed, SOURCE, RUN, FIXTURE)

    def test_identity_is_checked_before_any_command(self):
        for identity in ({**IDENTITY, "raw": "unexpected"}, {**IDENTITY, "compilerCommitSha": "bad"}):
            with self.assertRaises(ValueError):
                self.phase(identity=identity)
        self.assertEqual(self.calls, [])

    def test_original_query_bytes_and_oversize_are_retained_once(self):
        _, value = self.phase().run()
        query = value["queries"][0]
        original = self.artifacts / query["rawRelativePath"]
        self.assertEqual(original.read_bytes(), listing())
        destination.validate_receipt(value, SOURCE, RUN, FIXTURE, artifact_root=self.artifacts)
        self.assertEqual(len(list((self.artifacts / "builds").glob("xcode-destination-query-*"))), 1)
        self.independent_case("oversize")
        oversized = b"X" * (destination.MAX_OUTPUT + 1)
        self.outputs = [oversized]
        code, blocked = self.phase().run()
        self.assertEqual(code, 70)
        raw = self.artifacts / blocked["queries"][0]["rawRelativePath"]
        self.assertEqual(raw.read_bytes(), oversized)
        self.assertEqual(blocked["queries"][0]["outputSha256"], hashlib.sha256(oversized).hexdigest())
        self.assertEqual(blocked["queries"][0]["status"], "UNPARSEABLE")
        self.assertEqual(list((self.artifacts / "builds").iterdir()), [raw])
        destination.validate_receipt(blocked, artifact_root=self.artifacts)

    def test_original_query_missing_changed_or_false_parsing_is_rejected(self):
        _, value = self.phase().run()
        raw = self.artifacts / value["queries"][0]["rawRelativePath"]
        original = raw.read_bytes()
        raw.write_bytes(listing(OTHER))
        with self.assertRaisesRegex(ValueError, "DESTINATION_ORIGINAL_(HASH|SIZE)"):
            destination.validate_receipt(value, artifact_root=self.artifacts)
        changed = copy.deepcopy(value)
        changed["queries"][0].update(outputBytes=raw.stat().st_size, outputSha256=hashlib.sha256(raw.read_bytes()).hexdigest())
        with self.assertRaisesRegex(ValueError, "DESTINATION_ORIGINAL_OBSERVATION"):
            destination.validate_receipt(changed, artifact_root=self.artifacts)
        raw.unlink()
        with self.assertRaisesRegex(ValueError, "DESTINATION_ORIGINAL_PATH"):
            destination.validate_receipt(value, artifact_root=self.artifacts)
        if os.name == "posix":
            outside = self.root / "outside-query.log"
            outside.write_bytes(original)
            raw.symlink_to(outside)
            with self.assertRaisesRegex(ValueError, "DESTINATION_ORIGINAL_PATH"):
                destination.validate_receipt(value, artifact_root=self.artifacts)

    def test_attempt_reservation_exists_before_actual_test_command(self):
        def runner(args, allowance, *, output=None):
            if "test-without-building" in args:
                marker = self.artifacts / "runtime/xcode-destination-attempt.json"
                value = json.loads(marker.read_bytes())
                self.assertEqual(value["state"], "ATTEMPT_RESERVED")
                self.assertEqual(value["compilerCommitSha"], SOURCE)
                self.assertEqual(value["runId"], RUN)
                self.assertEqual(value["testIdentifier"], destination.TEST)
            return self.runner(args, allowance, output=output)
        self.assertEqual(self.phase(runner).run()[0], 0)
        with self.assertRaisesRegex(ValueError, "DESTINATION_ATTEMPT_ALREADY_RESERVED"):
            self.phase(runner).run()
        self.assertEqual(sum("test-without-building" in args for args, _ in self.calls), 1)

    def test_durable_reservation_delay_cannot_borrow_gate_deadline(self):
        def delayed_sync(_):
            self.clock += 91
        with patch.object(destination.os, "fsync", side_effect=delayed_sync):
            code, receipt = self.phase().run()
        self.assertEqual(code, 70)
        self.assertFalse(receipt["testStarted"])
        self.assertEqual(len(self.calls), 1)
        self.assertTrue((self.artifacts / "runtime/xcode-destination-attempt.json").is_file())
        self.assertGreater(receipt["gateElapsedMs"], receipt["gateBudgetMs"])

    def test_validator_failure_after_test_cannot_enable_a_second_attempt(self):
        self.outputs = [listing(), listing()]
        original = destination.DestinationTestPhase
        factory = lambda *args, **kwargs: original(*args, **kwargs,
            runner=self.runner, now=lambda: self.clock)
        argv = ["destination", "--repo", str(self.repo), "--scratch", str(self.scratch),
                "--artifacts", str(self.artifacts), "--udid", UDID,
                "--source-sha", SOURCE, "--run-id", RUN, "--fixture-id", FIXTURE]
        with patch.object(sys, "argv", argv), patch.object(destination, "DestinationTestPhase", side_effect=factory), \
             patch.object(destination, "validate_receipt", side_effect=ValueError("POST_TEST_VALIDATION_FAILURE")):
            with self.assertRaises(ValueError):
                destination.main()
            # Even if a later local cleanup removes the raw query, the durable
            # test reservation must independently prevent a duplicate attempt.
            (self.artifacts / "builds/xcode-destination-query-1.log").unlink()
            with self.assertRaises(ValueError):
                destination.main()
        self.assertEqual(sum("test-without-building" in args for args, _ in self.calls), 1)

    def test_receipt_write_failure_after_test_cannot_enable_a_second_attempt(self):
        self.outputs = [listing(), listing()]
        original = destination.DestinationTestPhase
        factory = lambda *args, **kwargs: original(*args, **kwargs,
            runner=self.runner, now=lambda: self.clock)
        argv = ["destination", "--repo", str(self.repo), "--scratch", str(self.scratch),
                "--artifacts", str(self.artifacts), "--udid", UDID,
                "--source-sha", SOURCE, "--run-id", RUN, "--fixture-id", FIXTURE]
        real_open = Path.open
        def failing_write(path, mode="r", *args, **kwargs):
            if path.name == "xcode-destination-readiness.json" and "x" in mode:
                raise OSError("POST_TEST_RECEIPT_WRITE_FAILURE")
            return real_open(path, mode, *args, **kwargs)
        with patch.object(sys, "argv", argv), patch.object(destination, "DestinationTestPhase", side_effect=factory), \
             patch.object(Path, "open", failing_write):
            with self.assertRaises(OSError):
                destination.main()
            (self.artifacts / "builds/xcode-destination-query-1.log").unlink()
            with self.assertRaises((OSError, ValueError)):
                destination.main()
        self.assertEqual(sum("test-without-building" in args for args, _ in self.calls), 1)

    def test_publication_preserves_exact_receipt_and_rejects_rebinding(self):
        _, value = self.phase().run()
        raw = self.artifacts
        (raw / "compiler-manifest.json").write_text(json.dumps({"syntheticOnly": True,
            "compilerCommitSha": SOURCE, "fixtures": [{"fixtureId": FIXTURE,
            "fileName": FIXTURE + ".cherri", "sourceSha256": "1" * 64}]}))
        (raw / "summary.json").write_text('{"privacyGatePassed":true}')
        (raw / "runtime/output-assertions.json").write_text(json.dumps({"targetFixtureId": FIXTURE}))
        path = raw / "runtime/xcode-destination-readiness.json"
        path.write_text(json.dumps(value))
        # Native command output is deliberately outside the publication allowlist.
        (raw / "builds/xcodebuild-test.log").unlink()
        trusted = {FIXTURE + ".cherri": "1" * 64}
        publication.stage(raw, self.root / "public", trusted)
        self.assertEqual(path.read_bytes(), (self.root / "public/runtime/xcode-destination-readiness.json").read_bytes())
        self.assertFalse((self.root / "public/builds/xcode-destination-query-1.log").exists())
        self.assertFalse((self.root / "public/runtime/xcode-destination-attempt.json").exists())
        for key, wrong in (("compilerCommitSha", "b" * 40), ("runId", "wrong"), ("fixtureId", "wrong")):
            with self.subTest(key=key):
                path.write_text(json.dumps({**value, key: wrong}))
                with self.assertRaisesRegex(publication.PublicationBlocked, "INVALID_BOUNDED_DESTINATION_DIAGNOSTIC"):
                    publication.stage(raw, self.root / ("rejected-" + key), trusted)

    def diagnostic(self):
        return json.loads((self.artifacts / "runtime/xcode-destination-command-diagnostic.json").read_bytes())

    def test_failed_explicit_open_retains_exact_original_and_stops_before_boot_query_or_test(self):
        self.outputs = [listing(OTHER)]
        private = (b'CHERRI_PRIVATE_CANARY /Users/private/name@example.invalid\n'
                   b'The application cannot be opened: NSOSStatusErrorDomain Code=-10810\n')
        def runner(args, allowance, *, output=None):
            if args[0] == "open":
                self.calls.append((args, allowance)); self.clock += 1
                output.write(private)
                return {"result": "EXIT_NONZERO", "exitCode": 1, "elapsedMs": 1000}
            return self.runner(args, allowance, output=output)
        code, receipt = self.phase(runner).run()
        self.assertEqual((code, len(self.calls), receipt["testStarted"]), (70, 3, False))
        self.assertEqual(receipt["commands"][-1]["exitCode"], 1)
        self.assertEqual((self.artifacts / "builds/xcode-destination-launch.log").read_bytes(), private)
        value = self.diagnostic()
        observed = value["launch"]["commands"][-1]["combinedOutput"]["safeObservation"]
        self.assertEqual(observed["knownErrors"], ["APPLICATION_LAUNCH_FAILED"])
        self.assertEqual(observed["systemErrorCodes"], [{"domain": "NSOSStatusErrorDomain", "code": -10810}])
        public = json.dumps(value)
        for secret in ("CHERRI_PRIVATE_CANARY", "/Users/private", "name@example.invalid", UDID, str(self.simulator)):
            self.assertNotIn(secret, public)
        destination.validate_receipt(receipt, artifact_root=self.artifacts)
        (self.artifacts / "compiler-manifest.json").write_text(json.dumps({"syntheticOnly": True,
            "compilerCommitSha": SOURCE, "fixtures": [{"fixtureId": FIXTURE,
            "fileName": FIXTURE + ".cherri", "sourceSha256": "1" * 64}]}))
        (self.artifacts / "summary.json").write_text('{"privacyGatePassed":true}')
        (self.artifacts / "runtime/output-assertions.json").write_text(json.dumps({"targetFixtureId": FIXTURE}))
        main = self.artifacts / "runtime/xcode-destination-readiness.json"
        main.write_text(json.dumps(receipt))
        trusted = {FIXTURE + ".cherri": "1" * 64}
        public_root = self.root / "published-open-failure"
        publication.stage(self.artifacts, public_root, trusted)
        side = "runtime/xcode-destination-command-diagnostic.json"
        self.assertEqual((public_root / side).read_bytes(), (self.artifacts / side).read_bytes())
        for name in ("resolve", "launch", "query-1"):
            self.assertFalse((public_root / ("builds/xcode-destination-" + name + ".log")).exists())
        main.write_text(json.dumps({**receipt, "schemaVersion": "1.0"}))
        with self.assertRaisesRegex(publication.PublicationBlocked, "INVALID_BOUNDED_DESTINATION_DIAGNOSTIC"):
            publication.stage(self.artifacts, self.root / "rejected-legacy-with-unbound-side", trusted)

    def test_resolve_failure_retains_original_without_guessing_an_application(self):
        self.outputs = [listing(OTHER)]
        original = b"xcrun: error: unable to find utility simctl\n"
        def runner(args, allowance, *, output=None):
            if args == ["xcrun", "--find", "simctl"]:
                self.calls.append((args, allowance)); self.clock += 1; output.write(original)
                return {"result": "EXIT_NONZERO", "exitCode": 72, "elapsedMs": 1000}
            return self.runner(args, allowance, output=output)
        code, receipt = self.phase(runner).run()
        self.assertEqual((code, len(self.calls)), (70, 2))
        self.assertEqual(receipt["commands"][-1]["exitCode"], 72)
        self.assertFalse((self.artifacts / "builds/xcode-destination-launch.log").exists())
        self.assertEqual((self.artifacts / "builds/xcode-destination-resolve.log").read_bytes(), original)
        self.assertEqual(self.diagnostic()["launch"]["selectionStatus"], "RESOLVE_COMMAND_FAILED")
        destination.validate_receipt(receipt, artifact_root=self.artifacts)

    def test_invalid_resolved_path_never_launches_fallback(self):
        values = (b"simctl\n", b"/usr/bin/simctl\n", b"/bad/path\n/second/path\n", b"\xff")
        self.simulator.rmdir()
        for index, original in enumerate(values):
            with self.subTest(index=index):
                self.independent_case("invalid-active-" + str(index))
                self.outputs, self.calls = [listing(OTHER)], []
                def runner(args, allowance, *, output=None):
                    if args == ["xcrun", "--find", "simctl"]:
                        self.calls.append((args, allowance)); self.clock += 1; output.write(original)
                        return {"result": "PASS", "exitCode": 0, "elapsedMs": 1000}
                    return self.runner(args, allowance, output=output)
                code, receipt = self.phase(runner).run()
                self.assertEqual((code, len(self.calls)), (70, 2))
                self.assertEqual(receipt["commands"][-1]["result"], "LAUNCH_FAILED")
                self.assertEqual(self.diagnostic()["launch"]["selectionStatus"], "RESOLUTION_REJECTED")
                destination.validate_receipt(receipt, artifact_root=self.artifacts)

    def test_resolve_and_launch_share_original_fifteen_second_allowance(self):
        self.outputs = [listing(OTHER)]
        def runner(args, allowance, *, output=None):
            if args == ["xcrun", "--find", "simctl"]:
                self.calls.append((args, allowance)); self.clock += 4
                output.write((str(self.tool) + "\n").encode())
                return {"result": "PASS", "exitCode": 0, "elapsedMs": 4000}
            if args[0] == "open":
                self.calls.append((args, allowance)); self.clock += 12
                output.write(b"LATE_ZERO_EXIT_CONTROL")
                return {"result": "PASS", "exitCode": 0, "elapsedMs": 12000}
            return self.runner(args, allowance, output=output)
        code, receipt = self.phase(runner).run()
        self.assertEqual((code, len(self.calls)), (70, 3))
        self.assertEqual(self.calls[-1][1], 11)
        self.assertEqual(receipt["commands"][-1]["result"], "DEADLINE_EXHAUSTED")
        self.assertFalse(receipt["testStarted"])
        destination.validate_receipt(receipt, artifact_root=self.artifacts)

    def test_late_resolver_zero_exit_cannot_start_application(self):
        self.outputs = [listing(OTHER)]
        def runner(args, allowance, *, output=None):
            result = self.runner(args, allowance, output=output)
            if args == ["xcrun", "--find", "simctl"]:
                self.clock += 5; result["elapsedMs"] += 5000
            return result
        code, receipt = self.phase(runner).run()
        self.assertEqual((code, len(self.calls)), (70, 2))
        self.assertEqual(receipt["commands"][-1]["result"], "DEADLINE_EXHAUSTED")
        destination.validate_receipt(receipt, artifact_root=self.artifacts)

    def test_zero_remaining_launch_budget_records_no_child_process(self):
        result, value = diagnostic.launch(self.artifacts, UDID, 0, runner=self.runner, now=lambda: self.clock)
        self.assertEqual(result["result"], "DEADLINE_EXHAUSTED")
        self.assertEqual(self.calls, [])
        self.assertEqual(value["commands"][0]["combinedOutput"]["bytes"], 0)
        self.assertEqual(value["selectionStatus"], "RESOLVE_COMMAND_FAILED")

    def test_launch_diagnostic_rejects_promotion_rebinding_types_and_fabricated_results(self):
        self.outputs = [listing(OTHER), listing()]
        _, receipt = self.phase().run()
        valid = self.diagnostic()
        mutations = {
            "formal promotion": lambda x: x.update(formalAcceptanceGranted=True),
            "scope": lambda x: x.update(scope="NATIVE_SEMANTICS"),
            "unknown raw field": lambda x: x.update(rawOutput="PRIVATE"),
            "canonical binding": lambda x: x.update(destinationReceiptCanonicalSha256="f" * 64),
            "Boolean count": lambda x: x["queryOutputs"][0]["safeObservation"].update(destinationRows=True),
            "raw path": lambda x: x["queryOutputs"][0].update(path="../unowned.log"),
            "hash": lambda x: x["launch"].update(toolPathSha256="f" * 64),
            "numeric boolean flag": lambda x: x["launch"].update(toolExists=1),
            "launch formal": lambda x: x["launch"].update(formalAcceptanceGranted=True),
            "Boolean exit": lambda x: x["launch"]["commands"][1].update(exitCode=False),
            "false positive": lambda x: x["launch"]["commands"][1].update(exitCode=1),
            "false failure": lambda x: x["launch"]["commands"][1].update(result="EXIT_NONZERO"),
            "unknown result": lambda x: x["launch"]["commands"][1].update(result="READY"),
            "extra budget": lambda x: x["launch"]["commands"][1].update(allowanceMs=15000),
            "late positive": lambda x: x["launch"]["commands"][1].update(elapsedMs=15001),
            "different stage": lambda x: x["launch"]["commands"][1].update(stage="reboot"),
            "invented observation": lambda x: x["launch"]["commands"][1]["combinedOutput"]["safeObservation"].update(knownErrors=["APPLICATION_LAUNCH_FAILED"]),
        }
        for name, mutate in mutations.items():
            with self.subTest(name=name):
                changed = copy.deepcopy(valid); mutate(changed)
                with self.assertRaises(ValueError):
                    diagnostic.validate(changed, self.artifacts, receipt)
        aggregate = copy.deepcopy(receipt)
        aggregate["commands"][1]["exitCode"] = 77
        with self.assertRaisesRegex(ValueError, "SIMULATOR_LAUNCH_AGGREGATE_RESULT"):
            diagnostic.validate(valid, self.artifacts, aggregate)

    def test_changed_or_removed_launch_original_is_rejected(self):
        self.outputs = [listing(OTHER), listing()]
        _, receipt = self.phase().run()
        raw = self.artifacts / "builds/xcode-destination-launch.log"
        raw.write_bytes(b"ALTERED_ORIGINAL")
        with self.assertRaisesRegex(ValueError, "SIMULATOR_LAUNCH_ORIGINAL_CHANGED"):
            destination.validate_receipt(receipt, artifact_root=self.artifacts)
        raw.unlink()
        with self.assertRaisesRegex(ValueError, "DESTINATION_DIAGNOSTIC_ORIGINAL"):
            destination.validate_receipt(receipt, artifact_root=self.artifacts)

    def test_legacy_receipt_remains_valid_but_new_schema_requires_exact_diagnostic(self):
        _, receipt = self.phase().run()
        side = self.artifacts / "runtime/xcode-destination-command-diagnostic.json"
        side.unlink()
        with self.assertRaisesRegex(ValueError, "DESTINATION_DIAGNOSTIC_REQUIRED"):
            destination.validate_receipt(receipt, artifact_root=self.artifacts)
        legacy = {**receipt, "schemaVersion": "1.0"}
        destination.validate_receipt(legacy, artifact_root=self.artifacts)

    def test_safe_diagnostic_only_reports_fixed_errors_numeric_codes_and_destination_properties(self):
        raw = (listing().replace(b"PUBLIC_OWNED_SIMULATOR", b"CHERRI_PRIVATE_CANARY name@example.invalid")
               + b'LSOpenURLsWithRole() failed with error -10810 for /Users/private/SECRET.app\n'
               + b'{ platform:iOS Simulator, id:private-placeholder, OS:27.0.1, name:SECRET }\n')
        value = diagnostic.safe_detail(raw)
        self.assertEqual(value["destinationRows"], 2)
        self.assertEqual(value["placeholderRows"], 1)
        self.assertEqual(value["platforms"], ["iOS Simulator"])
        self.assertEqual(value["osVersions"], ["27.0.1", "27.1"])
        self.assertEqual(value["systemErrorCodes"], [{"domain": "NSOSStatusErrorDomain", "code": -10810}])
        for secret in ("CHERRI_PRIVATE_CANARY", "name@example.invalid", "SECRET", "private", UDID):
            self.assertNotIn(secret, json.dumps(value))
        for raw, expected in ((b"\xff", "INVALID_UTF8"), (b"\x00", "NUL_PRESENT"), (b"x" * (diagnostic.LIMIT + 1), "OVERSIZED")):
            self.assertEqual(diagnostic.safe_detail(raw)["encoding"], expected)



class RealProcessAndShellTests(unittest.TestCase):
    def test_existing_process_runner_captures_actual_stdout_and_stderr(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "private-process.log"
            with path.open("wb") as stream:
                result = recovery.run_process([sys.executable, "-c", 'import sys; print("PUBLIC_OUT"); print("PUBLIC_ERR",file=sys.stderr)'], 3, output=stream)
            self.assertEqual(result["result"], "PASS")
            self.assertEqual(set(path.read_text().splitlines()), {"PUBLIC_OUT", "PUBLIC_ERR"})

    @unittest.skipUnless(os.name == "posix" and shutil.which("bash"), "actual shell route requires POSIX")
    def test_actual_shell_route_propagates_test_failure_and_refuses_duplicate_invocation(self):
        source = Path(__file__).resolve().parent
        shell = (source.parents[1] / "scripts/ios27_shortcuts_simulator_qa.sh").read_text()
        begin = shell.index('    python3 "$ROOT_DIR/tools/shortcuts-simulator-qa/xcode_destination_readiness.py"')
        end = shell.index('    echo "test02 destination/test phase exit code:', begin)
        snippet = shell[begin:end] + '\nexit "$XCODEBUILD_TEST_RC"\n'
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            repo, scratch, artifacts, bins = [root / x for x in ("repo", "scratch", RUN, "bin")]
            for path in (repo / destination.PROJECT, scratch, artifacts / "builds", artifacts / "runtime", bins):
                path.mkdir(parents=True)
            helper_dir = repo / "tools/shortcuts-simulator-qa"
            for name in ("xcode_destination_readiness.py", "xcode_destination_diagnostic.py", "simulator_session_recovery.py"):
                shutil.copyfile(source / name, helper_dir / name)
            executable = bins / "xcodebuild"
            executable.write_text("#!" + sys.executable + "\nimport sys\nif '-showdestinations' in sys.argv:\n print(" + repr(listing().decode()) + ")\n raise SystemExit(0)\nprint('PUBLIC_XCODE_FAILURE_CONTROL')\nraise SystemExit(7)\n")
            executable.chmod(0o700)
            env = {**os.environ, "PATH": str(bins) + os.pathsep + os.environ["PATH"],
                "ROOT_DIR": str(repo), "QA_SCRATCH_DIR": str(scratch), "ARTIFACT_DIR": str(artifacts),
                "UDID": UDID, "SOURCE_SHA": SOURCE, "RUN_ID": RUN, "TARGET_FIXTURE_ID": FIXTURE}
            result = subprocess.run(["bash", "-c", snippet], env=env, capture_output=True, timeout=10)
            self.assertEqual(result.returncode, 7, result.stderr)
            receipt_path = artifacts / "runtime/xcode-destination-readiness.json"
            original = receipt_path.read_bytes()
            value = destination.validate_receipt(json.loads(original), SOURCE, RUN, FIXTURE)
            self.assertEqual(value["commands"][-1]["exitCode"], 7)
            self.assertTrue(value["testStarted"])
            self.assertFalse(value["formalAcceptanceGranted"])
            second = subprocess.run(["bash", "-c", snippet], env=env, capture_output=True, timeout=10)
            self.assertEqual(second.returncode, 70)
            self.assertEqual(receipt_path.read_bytes(), original)


if __name__ == "__main__":
    unittest.main()
