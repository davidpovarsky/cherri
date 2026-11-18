"""Exercise actual oracle/selector code and the two-dispatch policy; no native storage is simulated."""
import copy
from datetime import datetime, timedelta, timezone
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
import urllib.parse

import storage_cross_run as cross
import test_runtime_callback_contract as callback
import test_workflow_input_contract as workflow
from prepare_public_artifacts import PublicationBlocked, stage

ROOT = Path(__file__).resolve().parents[2]
UDID = "00000000-0000-4000-8000-000000000001"


class StorageCrossRunContractTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        callback.CallbackContractTests.setUpClass()
        workflow.WorkflowInputContractTests.setUpClass()
        cls.oracle = callback.CallbackContractTests()
        cls.workflow = workflow.WorkflowInputContractTests()

    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name) / "raw"
        (self.root / "runtime").mkdir(parents=True)
        self.receiver = Path(self.temp.name) / "receiver"
        (self.receiver / "Documents").mkdir(parents=True)
        self.manifest = self.oracle.manifest(cross.FIXTURE)
        self.manifest["fixtures"][0]["fileName"] = cross.FIXTURE + ".cherri"
        cross.write_json(self.root / "compiler-manifest.json", self.manifest)
        self.plan = cross.prepare(self.root, cross.FIXTURE + "-unit-" + "1" * 32, "a" * 40, UDID)
        self.writer = self.event("WRITE", -5)
        cross.write_json(self.root / "runtime/callback-events.json", [self.writer])
        self.execute_oracle("WRITE")

    def event(self, phase, seconds=0):
        ev = self.oracle.event()
        ev.update(fixtureId=cross.FIXTURE, expectedShortcutName=cross.FIXTURE,
                  runId=self.plan["writerNonce" if phase == "WRITE" else "readerNonce"],
                  resultSummary=cross.marker(phase),
                  timestampISO8601=(datetime.now(timezone.utc) + timedelta(seconds=seconds)).isoformat(
                      timespec="milliseconds").replace("+00:00", "Z"))
        ev["tracePayload"] = ev["tracePayload"].replace("qa-unit", cross.FIXTURE)
        return ev

    def execute_oracle(self, phase):
        name = "writer" if phase == "WRITE" else "reader"
        trace = self.root / "runtime" / ("storage-writer-trace.jsonl" if phase == "WRITE" else "runtime-trace.jsonl")
        completed = subprocess.run([sys.executable, "-c", self.oracle.program,
            str(self.root / "runtime/callback-events.json"), self.plan[name + "Nonce"], cross.FIXTURE,
            cross.marker(phase), str(trace), str(self.root / "compiler-manifest.json"), "a" * 40],
            capture_output=True, text=True, timeout=10)
        self.assertEqual(completed.returncode, 0, completed.stderr)
        oracle = json.loads(completed.stdout)
        cross.write_json(self.root / "runtime" / ("storage-" + name + "-oracle.json"), oracle)
        return oracle

    def successful_dispatch(self):
        calls = []
        def runner(argv, **kwargs):
            calls.append((argv, kwargs))
            if argv[2] == "launch":
                reader = self.event("READ")
                cross.write_json(self.receiver / "Documents/callback-events.json", [self.writer, reader])
            return subprocess.CompletedProcess(argv, 0)
        self.assertTrue(cross.dispatch_reader(self.root, UDID, self.receiver, runner=runner))
        self.assertEqual(self.execute_oracle("READ")["traceGate"], "PASS")
        return calls

    def assertions(self):
        return {
            "targetFixtureId": cross.FIXTURE, "expectedRunId": self.plan["readerNonce"],
            "expectedSemanticOutput": cross.marker("READ"), "runtimeContractVerified": True,
            "compilerCommitSha": "a" * 40, "compiledSourceSha256": "b" * 64,
            "unsignedPlistSha256": "c" * 64,
        }

    def test_two_external_invocations_bind_same_key_fresh_nonces_and_exact_oracle(self):
        calls = self.successful_dispatch()
        self.assertEqual([c[0] for c in calls], [
            ["xcrun", "simctl", "terminate", UDID, cross.BUNDLE],
            ["xcrun", "simctl", "launch", UDID, cross.BUNDLE]])
        url = calls[1][1]["env"]["SIMCTL_CHILD_CHERRI_TRIGGER_SHORTCUT_URL"]
        parsed = urllib.parse.parse_qs(urllib.parse.urlsplit(url).query)
        self.assertEqual(parsed["name"], [cross.FIXTURE])
        self.assertEqual(json.loads(parsed["text"][0]), {"phase": "READ", "runToken": self.plan["runToken"]})
        callback_query = urllib.parse.parse_qs(urllib.parse.urlsplit(parsed["x-success"][0]).query)
        self.assertEqual(callback_query["runId"], [self.plan["readerNonce"]])
        self.assertNotEqual(self.plan["readerNonce"], self.plan["writerNonce"])
        report = cross.build_report(self.root)
        self.assertTrue(report["crossRunTextVerified"])
        self.assertFalse(report["cacheImplemented"])
        self.assertFalse(report["typedArchiveProven"])
        self.assertEqual(report["cleanup"], "READ_BRANCH_REQUESTED_DELETE_ONLY")
        cross.write_json(self.root / "runtime/output-assertions.json", self.assertions())
        cross.validate_public_diagnostic(report, self.root)

    def test_writer_faults_block_reader_before_any_native_command(self):
        faults = [
            ("status", "ERROR"), ("status", "CANCELLED"), ("runId", "stale"),
            ("resultSummary", cross.marker("READ")), ("resultSummary", False),
            ("receiverSelfTest", True), ("eventSource", "receiver-self-test"),
            ("tracePayload", ""), ("fixtureId", "qa-unit"), ("timestampISO8601", ""),
        ]
        for key, value in faults:
            with self.subTest(key=key, value=value):
                altered = {**self.writer, key: value}
                cross.write_json(self.root / "runtime/callback-events.json", [altered])
                calls = []
                with self.assertRaises(ValueError):
                    cross.dispatch_reader(self.root, UDID, self.receiver, runner=lambda *a, **kw: calls.append(a))
                self.assertFalse(calls)
                self.assertFalse((self.root / "runtime/storage-reader-dispatch.json").exists())
        cross.write_json(self.root / "runtime/callback-events.json", [self.writer, self.writer])
        with self.assertRaises(ValueError):
            cross.writer_ready(self.root, self.plan)

    def test_writer_only_pass_cannot_mint_cross_run_success(self):
        self.assertTrue(cross.writer_ready(self.root, self.plan)["verified"])
        report = cross.build_report(self.root)
        self.assertFalse(report["crossRunTextVerified"])
        self.assertEqual(report["reader"]["matchingCallbacks"], 0)
        self.assertEqual(report["cleanup"], "NOT_OBSERVED")

    def test_native_dispatch_reentry_and_changed_simulator_rejected(self):
        with self.assertRaises(ValueError):
            cross.dispatch_reader(self.root, "00000000-0000-4000-8000-000000000002", self.receiver)
        calls = self.successful_dispatch()
        with self.assertRaises(FileExistsError):
            cross.dispatch_reader(self.root, UDID, self.receiver,
                runner=lambda *a, **kw: calls.append((a, kw)))
        self.assertEqual(len(calls), 2)

    def test_failed_launch_and_bounded_timeout_remain_failure(self):
        def failed(argv, **kwargs):
            return subprocess.CompletedProcess(argv, 1)
        self.assertFalse(cross.dispatch_reader(self.root, UDID, self.receiver, runner=failed))
        self.assertFalse(cross.build_report(self.root)["crossRunTextVerified"])
        self.assertEqual(cross.build_report(self.root)["dispatchResult"], "RECEIVER_LAUNCH_FAILED")
        (self.root / "runtime/storage-reader-dispatch.json").unlink()
        times = iter((0, 46))
        def launched(argv, **kwargs):
            return subprocess.CompletedProcess(argv, 0)
        self.assertFalse(cross.dispatch_reader(self.root, UDID, self.receiver, runner=launched,
                         monotonic=lambda: next(times), sleeper=lambda _: self.fail("timeout must be finite")))
        self.assertEqual(cross.build_report(self.root)["dispatchResult"], "CALLBACK_TIMEOUT")

    def test_plan_and_release_source_forgery_are_rejected(self):
        for key, value in (
            ("readerNonce", self.plan["writerNonce"]), ("runToken", "../foreign-key"),
            ("compilerCommitSha", "d" * 40), ("fixtureId", "qa-measure-release-v2"),
        ):
            altered = {**self.plan, key: value}
            with self.subTest(key=key), self.assertRaises(ValueError):
                cross.validate_plan(altered, self.root)
        self.manifest["fixtures"][0]["buildProfile"] = "release"
        cross.write_json(self.root / "compiler-manifest.json", self.manifest)
        with self.assertRaises(ValueError):
            cross.load_plan(self.root)

    def test_order_and_replay_fail_even_with_prior_oracle_pass(self):
        self.successful_dispatch()
        events = cross.read_json(self.root / "runtime/callback-events.json")
        events[1]["timestampISO8601"] = events[0]["timestampISO8601"]
        cross.write_json(self.root / "runtime/callback-events.json", events)
        self.assertFalse(cross.build_report(self.root)["crossRunTextVerified"])
        cross.write_json(self.root / "runtime/callback-events.json", events + [events[1]])
        self.assertFalse(cross.build_report(self.root)["crossRunTextVerified"])

    def test_publication_binds_bounded_report_and_quarantines_raw_plan(self):
        self.successful_dispatch()
        cross.write_json(self.root / "runtime/output-assertions.json", self.assertions())
        cross.write_json(self.root / "summary.json", {"privacyGatePassed": True})
        report = cross.build_report(self.root)
        path = self.root / "runtime" / cross.DIAGNOSTIC_FILE
        cross.write_json(path, report)
        publication = stage(self.root, Path(self.temp.name) / "public",
                            {cross.FIXTURE + ".cherri": "b" * 64})
        published = {r["path"] for r in publication["published"]}
        self.assertIn("runtime/" + cross.DIAGNOSTIC_FILE, published)
        self.assertIn("runtime/storage-writer-trace.jsonl", published)
        self.assertNotIn("runtime/" + cross.PLAN_FILE, published)
        text = path.read_text(encoding="utf-8")
        self.assertNotIn(self.plan["runToken"], text)
        self.assertNotIn(UDID, text)
        self.assertNotIn("shortcuts://", text)
        for key, value in (("crossRunTextVerified", False), ("runScopedKeySha256", "d" * 64),
                           ("sourceSha256", "d" * 64)):
            with self.subTest(key=key):
                cross.write_json(path, {**report, key: value})
                with self.assertRaises(PublicationBlocked):
                    stage(self.root, Path(self.temp.name) / ("public-" + key),
                          {cross.FIXTURE + ".cherri": "b" * 64})

    def test_shell_helper_failures_stop_before_native_launch(self):
        script = (ROOT / "scripts/ios27_shortcuts_simulator_qa.sh").read_text(encoding="utf-8")
        start = script.index('    if [ "$STORAGE_CROSS_RUN_ENABLED" = "true" ]; then\n      python3')
        setup = script[start:script.index('    RUN_URL="$(python3 - ', start)]
        nonce_line = next(line for line in script.splitlines()
                          if 'EXEC_RUN_NONCE="$(python3 ' in line and "reader-nonce" in line)
        for failed_call, fragment in ((1, setup), (2, setup), (1, nonce_line)):
            with self.subTest(failed_call=failed_call, reader="reader-nonce" in fragment):
                counter = Path(self.temp.name) / "helper-count"
                counter.write_text("", encoding="utf-8")
                stub = (
                    'set -uo pipefail\n'
                    'python3() { printf x >> "$COUNTER"; '
                    'n=$(wc -c < "$COUNTER"); '
                    'if [ "$n" -eq "$FAIL_CALL" ]; then return 7; fi; }\n')
                result = subprocess.run([self.workflow.bash, "-s"],
                    input=stub + fragment + '\necho UNEXPECTED_NATIVE_CONTINUATION\n',
                    env={**os.environ, "COUNTER": counter.as_posix(),
                         "FAIL_CALL": str(failed_call), "STORAGE_CROSS_RUN_ENABLED": "true",
                         "ROOT_DIR": "/reviewed", "ARTIFACT_DIR": "/synthetic",
                         "EXEC_RUN_NONCE": "synthetic", "SOURCE_SHA": "a" * 40, "UDID": UDID},
                    capture_output=True, text=True, timeout=10)
                self.assertEqual(result.returncode, 2, result.stderr)
                self.assertNotIn("UNEXPECTED_NATIVE_CONTINUATION", result.stdout)
                self.assertEqual(counter.read_text(encoding="utf-8"), "x" * failed_call)

    def test_new_selectors_are_explicit_and_require_exact_normal_trace(self):
        manifest = json.loads((ROOT / "docs/apple-runtime/p10r15/native-continuation-probe-manifest.json").read_bytes())
        script = (ROOT / "scripts/ios27_shortcuts_simulator_qa.sh").read_text(encoding="utf-8")
        selector = script.split("# Select primary target fixture from SAMPLE_ID\n", 1)[1].split("LEGACY_COUNTERPART_SOURCE=", 1)[0]
        selector += '\ntest "$LEGACY_COUNTERPART_ID" = "qa-functional-legacy"\nprintf "%s\\n" "$TARGET_FIXTURE_ID" "$TARGET_SOURCE_FILE" "$EXPECTED_SHORTCUT_NAME" "$EXPECTED_SEMANTIC_OUTPUT" "$STORAGE_CROSS_RUN_ENABLED"\n'
        yaml = (ROOT / ".github/workflows/cherri-ios27-shortcuts-simulator-qa.yml").read_text(encoding="utf-8")
        options = yaml.split("      sample_id:", 1)[1].split("      capture_full_editor:", 1)[0]
        for case in manifest["cases"]:
            with self.subTest(fixture=case["id"]):
                self.assertIn("          - " + case["sampleId"] + "\n", options)
                result = subprocess.run([self.workflow.bash, "-s"], input="set -eu\n" + selector,
                    env={**os.environ, "SAMPLE_ID": case["sampleId"], "ROOT_DIR": "/reviewed",
                         "TARGET_SOURCE_RELATIVE": "../../private-input.cherri"},
                    capture_output=True, text=True, timeout=10)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(result.stdout.splitlines(), [case["id"], "/reviewed/" + case["path"],
                    case["id"], case["expectedSummary"], "true" if case["id"] == cross.FIXTURE else "false"])
                self.assertEqual(hashlib.sha256((ROOT / case["path"]).read_bytes()).hexdigest(), case["sourceSha256"])
                event = self.oracle.event()
                event.update(fixtureId=case["id"], expectedShortcutName=case["id"],
                             resultSummary=case["expectedSummary"])
                event["tracePayload"] = event["tracePayload"].replace("qa-unit", case["id"])
                result = self.oracle.verify([event], case["id"], case["expectedSummary"])
                self.assertEqual((result["runGate"], result["traceGate"]), ("PASS", "PASS"))
                self.assertEqual(self.oracle.verify([{**event, "resultSummary": case["expectedSummary"] + "_WRONG"}],
                    case["id"], case["expectedSummary"])["runGate"], "FAIL_RUNTIME")
                self.assertEqual(self.oracle.verify([{**event, "tracePayload": ""}],
                    case["id"], case["expectedSummary"])["traceGate"], "FAIL_RUNTIME")
        for sample in ("qa-storage-cross-run-private", "qa-http-json-private", "qa-storage-text-cross-run-release"):
            self.assertEqual(self.workflow.validate(["full", "ipad-default", sample, "false", "20", "true"]).returncode, 2)
        self.assertIn('if [ "$HTTP_PROBE_ENABLED" = "true" ] || [ "$STORAGE_CROSS_RUN_ENABLED" = "true" ]; then', script)
        self.assertIn('evaluate_runtime_callback() {', script)


if __name__ == "__main__":
    unittest.main()
