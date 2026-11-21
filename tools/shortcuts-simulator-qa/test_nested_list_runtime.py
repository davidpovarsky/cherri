"""Actual compiler and harness controls; all callback/signature bytes here are synthetic.

These tests verify the observation boundary. They are never Apple runtime
evidence and cannot grant nested-List semantic acceptance.
"""
import copy
import json
import os
from pathlib import Path
import plistlib
import shutil
import subprocess
import sys
import tempfile
import unittest

import nested_list_probe as probe
import nested_list_runtime as runtime
import prepare_public_artifacts as publication
import test_nested_list_probe as observer_controls
import test_runtime_callback_contract as harness_controls

ROOT = Path(__file__).resolve().parents[2]
COMMIT = "a" * 40
NONCE = "synthetic-control-fresh-nonce"


class NestedRuntimeControls(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.compiler_dir = tempfile.TemporaryDirectory(prefix="nested-runtime-actual-compiler-")
        cls.addClassCleanup(cls.compiler_dir.cleanup)
        build = Path(cls.compiler_dir.name)
        binary = build / ("cherri.exe" if os.name == "nt" else "cherri")
        result = subprocess.run(["go", "build", "-o", str(binary), "."], cwd=ROOT,
                                capture_output=True, timeout=180)
        if result.returncode:
            raise AssertionError(result.stderr.decode("utf-8", errors="replace"))
        cls.compiled = {}
        for fixture in probe.FIXTURES:
            source = ROOT / "tools/shortcuts-simulator-qa/fixtures" / (fixture + ".cherri")
            target = build / (fixture + ".shortcut")
            result = subprocess.run([str(binary), str(source), "--skip-sign", "--build-profile=debug",
                                     "--output=" + str(target)], cwd=build, capture_output=True, timeout=60)
            if result.returncode:
                raise AssertionError(result.stdout.decode("utf-8", errors="replace")
                                     + result.stderr.decode("utf-8", errors="replace"))
            cls.compiled[fixture] = (source.read_bytes(), target.read_bytes())
        harness_controls.CallbackContractTests.setUpClass()
        cls.oracle_program = harness_controls.CallbackContractTests.program
        cls.assertion_program = harness_controls.CallbackContractTests.assertion_program

    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="nested-runtime-control-")
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name) / "synthetic-run"
        (self.root / "builds/signed").mkdir(parents=True)
        (self.root / "runtime").mkdir()
        self.initialize(probe.CONTROL)

    def write(self, path, value):
        (self.root / path).write_bytes(runtime.canonical(value))

    def initialize(self, fixture):
        self.fixture = fixture
        source, unsigned = self.compiled[fixture]
        (self.root / ("builds/" + fixture + ".cherri")).write_bytes(source)
        (self.root / ("builds/" + fixture + ".shortcut")).write_bytes(unsigned)
        signed = b"AEA1SYNTHETIC_CONTROL_ONLY_NOT_A_SIGNATURE"
        (self.root / ("builds/signed/" + fixture + ".shortcut")).write_bytes(signed)
        self.row = {"fixtureId": fixture, "fileName": fixture + ".cherri", "dialect": "v2",
                    "buildProfile": "debug", "runtimeTraceExpectation": "REQUIRED_NDJSON",
                    "sourceSha256": probe.SOURCE_HASHES[fixture], "unsignedPlistSha256": probe.sha(unsigned),
                    "nativeActionCount": probe.COUNTS[fixture],
                    "nestedListProbe": runtime.prepared(ROOT, fixture, source, unsigned)}
        self.manifest = {"syntheticOnly": True, "compilerCommitSha": COMMIT, "fixtures": [self.row]}
        self.write("compiler-manifest.json", self.manifest)
        self.signing = {
            "compilerCommitSha": COMMIT, "signedArtifactValidAEA1": True,
            "signedArtifactSha256": probe.sha(signed), "signedByteSize": len(signed),
            "targetProvenance": {
                "compilerCommitSha": COMMIT, "sourceSha256": probe.SOURCE_HASHES[fixture],
                "unsignedPlistSha256": probe.sha(unsigned), "unsignedByteSize": len(unsigned),
                "nativeActionCount": probe.COUNTS[fixture], "signedArtifactSha256": probe.sha(signed),
                "signedByteSize": len(signed), "headerMagic": "AEA1"}}
        self.write("runtime/signing-provenance.json", self.signing)
        before = {"schemaVersion": "1.4", "kind": "INLINE_DEBUG_ACTION", "stage": "before",
                  "label": fixture[3:-3], "stepSequence": 3, "actionIdentifier": "is.workflow.actions.setvariable",
                  "timestampPrecision": "apple-currentdate", "valueType": "UNKNOWN_AT_TRACE_EVENT",
                  "timestamp": "SYNTHETIC_CONTROL_TIME_NOT_APPLE"}
        after = {**before, "stage": "after", "stepSequence": 49 if fixture == probe.CONTROL else 71,
                 "actionIdentifier": "is.workflow.actions.comment"}
        self.records = [json.loads(probe._collector(fixture, "collector_begin")), before, after,
                        json.loads(probe._collector(fixture, "collector_end"))]
        self.fields = observer_controls.fields(fixture)
        self.event = {"receiverSelfTest": False, "eventSource": "shortcuts-x-callback", "runId": NONCE,
                      "fixtureId": fixture, "expectedShortcutName": fixture, "status": "SUCCESS",
                      "resultSummary": observer_controls.summary(fixture, self.fields),
                      "tracePayload": "\n".join(json.dumps(record) for record in self.records)}
        self.write("runtime/callback-events.json", [self.event])
        trace = self.root / "runtime/runtime-trace.jsonl"
        if trace.exists():
            trace.unlink()

    def observe(self, expected=""):
        return runtime.evaluate(ROOT, self.root, self.fixture, COMMIT, NONCE, expected)

    def actual_oracle(self, expected=""):
        args = [str(self.root / "runtime/callback-events.json"), NONCE, self.fixture, expected,
                str(self.root / "runtime/runtime-trace.jsonl"), str(self.root / "compiler-manifest.json"),
                COMMIT, str(ROOT)]
        result = subprocess.run([sys.executable, "-B", "-c", self.oracle_program, *args],
                                capture_output=True, text=True, encoding="utf-8", timeout=30)
        self.assertEqual(result.returncode, 0, result.stderr)
        return json.loads(result.stdout)

    def actual_assertions(self, oracle, run=None, trace=None):
        args = [str(self.root), run or oracle["runGate"], "PASS", trace or oracle["traceGate"],
                oracle.get("reason", ""), self.fixture, "", NONCE, json.dumps(oracle),
                str(self.root / "compiler-manifest.json"), COMMIT, str(ROOT)]
        result = subprocess.run([sys.executable, "-B", "-c", self.assertion_program, *args],
                                capture_output=True, text=True, encoding="utf-8", timeout=30)
        self.assertEqual(result.returncode, 0, result.stderr)
        return json.loads((self.root / "runtime/output-assertions.json").read_bytes())

    def test_actual_compiler_both_complete_graphs_and_harness_acquisition(self):
        for fixture in probe.FIXTURES:
            with self.subTest(fixture=fixture):
                self.initialize(fixture)
                oracle = self.actual_oracle()
                self.assertTrue(oracle["runtimeObservationVerified"], oracle)
                self.assertEqual(len(oracle["traceRecords"]), 4)
                self.assertEqual(self.row["nestedListProbe"]["graph"]["nativeActionCount"], probe.COUNTS[fixture])
                assertions = self.actual_assertions(oracle)
                self.assertTrue(assertions["runtimeContractVerified"])
                self.assertTrue(assertions["diagnosticOnly"])
                for key in ("semanticContractVerified", "nativeListTypeProved", "formalAcceptanceGranted"):
                    self.assertIs(assertions[key], False)
                observation = json.loads((self.root / "runtime/nested-list-observation.json").read_bytes())
                runtime.validate_public_diagnostic(observation, self.root)

    def test_contrary_fields_survive_real_callback_and_assertion_programs(self):
        for fixture, key, value in ((probe.CONTROL, "FLAT_COUNT", "999"),
                                    (probe.CANDIDATE, "FIRST_FIRST64", observer_controls.b64("CONTRARY"))):
            with self.subTest(fixture=fixture):
                self.initialize(fixture)
                self.fields[key] = value
                self.event["resultSummary"] = observer_controls.summary(fixture, self.fields)
                self.write("runtime/callback-events.json", [self.event])
                oracle = self.actual_oracle()
                self.assertTrue(oracle["runtimeObservationVerified"])
                self.assertFalse(oracle["observation"]["goalChecks"][key])
                self.assertEqual(oracle["observation"]["actualFields"][key], value)
                assertions = self.actual_assertions(oracle)
                self.assertTrue(assertions["runtimeObservationVerified"])
                self.assertFalse(assertions["semanticContractVerified"])
                self.assertEqual(assertions["expectedSemanticOutput"], "")

    def test_callback_correlation_and_status_rejections(self):
        changes = (("runId", "stale"), ("fixtureId", "other"), ("expectedShortcutName", "other"),
                   ("eventSource", "receiver-self-test"), ("receiverSelfTest", True),
                   ("receiverSelfTest", 0), ("status", "ERROR"), ("status", "CANCELLED"),
                   ("resultSummary", "SUCCESS"))
        for key, value in changes:
            with self.subTest(key=key, value=value):
                self.write("runtime/callback-events.json", [{**self.event, key: value}])
                observed = self.observe()
                self.assertFalse(observed["runtimeObservationVerified"])
                self.assertEqual(observed["runGate"], "FAIL_RUNTIME")
        self.write("runtime/callback-events.json", [self.event, self.event])
        self.assertIn("UNIQUE_NONCE", self.observe()["reason"])

    def test_diagnostic_cannot_use_callback_as_expected_output(self):
        self.assertIn("DIAGNOSTIC_MODE", self.observe(self.event["resultSummary"])["reason"])

    def test_all_static_trace_fields_and_event_inventory_fail_closed(self):
        for index, record in enumerate(self.records):
            for key in record:
                if key == "timestamp":
                    continue
                for remove in (False, True):
                    with self.subTest(event=index, key=key, remove=remove):
                        records = copy.deepcopy(self.records)
                        if remove:
                            records[index].pop(key)
                        else:
                            records[index][key] = "WRONG"
                        event = {**self.event, "tracePayload": "\n".join(map(json.dumps, records))}
                        self.write("runtime/callback-events.json", [event])
                        result = self.observe()
                        self.assertEqual(result["runGate"], "PASS", result)
                        self.assertEqual(result["traceGate"], "FAIL_RUNTIME", result)
        for records in (self.records[:-1], self.records + [self.records[-1]], list(reversed(self.records))):
            self.write("runtime/callback-events.json", [{**self.event, "tracePayload": "\n".join(map(json.dumps, records))}])
            self.assertFalse(self.observe()["runtimeObservationVerified"])
        for bad in ("", "\ufffc", "x" * 257, None):
            records = copy.deepcopy(self.records); records[1]["timestamp"] = bad
            self.write("runtime/callback-events.json", [{**self.event, "tracePayload": "\n".join(map(json.dumps, records))}])
            self.assertFalse(self.observe()["runtimeObservationVerified"])

    def test_prepared_and_signing_provenance_types_are_exact(self):
        for key, value in (("nativeActionCount", 56.0), ("sourceSha256", "0" * 64),
                           ("unsignedPlistSha256", "0" * 64), ("nestedListProbe", None),
                           ("buildProfile", "release"), ("runtimeTraceExpectation", "ABSENT_RELEASE")):
            with self.subTest(key=key):
                changed = copy.deepcopy(self.manifest); changed["fixtures"][0][key] = value
                self.write("compiler-manifest.json", changed)
                self.assertFalse(self.observe()["runtimeObservationVerified"])
        self.write("compiler-manifest.json", self.manifest)
        for key in self.signing["targetProvenance"]:
            with self.subTest(signing=key):
                changed = copy.deepcopy(self.signing); changed["targetProvenance"][key] = "WRONG"
                self.write("runtime/signing-provenance.json", changed)
                self.assertFalse(self.observe()["runtimeObservationVerified"])
        self.write("runtime/signing-provenance.json", {**self.signing, "targetProvenance": None})
        self.assertFalse(self.observe()["runtimeObservationVerified"])

    def test_mutated_unsigned_graph_cannot_self_reconcile_hashes(self):
        changed = plistlib.loads(self.compiled[self.fixture][1])
        changed["WFWorkflowActions"][-1]["WFWorkflowActionParameters"]["WFOutput"] = "SUCCESS"
        unsigned = plistlib.dumps(changed)
        (self.root / ("builds/" + self.fixture + ".shortcut")).write_bytes(unsigned)
        manifest = copy.deepcopy(self.manifest)
        manifest["fixtures"][0]["unsignedPlistSha256"] = probe.sha(unsigned)
        manifest["fixtures"][0]["nestedListProbe"]["unsignedPlistSha256"] = probe.sha(unsigned)
        self.write("compiler-manifest.json", manifest)
        signing = copy.deepcopy(self.signing)
        signing["targetProvenance"].update(unsignedPlistSha256=probe.sha(unsigned), unsignedByteSize=len(unsigned))
        self.write("runtime/signing-provenance.json", signing)
        self.assertFalse(self.observe()["runtimeObservationVerified"])

    def test_original_signed_source_and_callback_bytes_are_bound(self):
        for path in ("builds/" + self.fixture + ".cherri", "builds/signed/" + self.fixture + ".shortcut"):
            old = (self.root / path).read_bytes()
            (self.root / path).write_bytes(old + b"changed")
            self.assertFalse(self.observe()["runtimeObservationVerified"])
            (self.root / path).write_bytes(old)
        raw = (self.root / "runtime/callback-events.json").read_bytes()
        result = self.observe()
        self.assertEqual(result["inputBindings"]["callbackEvents"]["sha256"], probe.sha(raw))
        (self.root / "runtime/callback-events.json").write_bytes(b'[{"runId":"x","runId":"y"}]')
        self.assertFalse(self.observe()["runtimeObservationVerified"])

    def test_missing_callback_never_invents_trace(self):
        (self.root / "runtime/callback-events.json").unlink()
        oracle = self.actual_oracle()
        assertions = self.actual_assertions(oracle)
        self.assertFalse(assertions["runtimeObservationVerified"])
        self.assertFalse((self.root / "runtime/runtime-trace.jsonl").exists())

    def test_forged_or_stale_oracle_cannot_rewrite_actual_values(self):
        oracle = self.actual_oracle()
        self.fields["FLAT_COUNT"] = "999"
        self.event["resultSummary"] = observer_controls.summary(self.fixture, self.fields)
        self.write("runtime/callback-events.json", [self.event])
        assertions = self.actual_assertions(oracle)
        self.assertFalse(assertions["callbackOracleAgreement"])
        self.assertFalse(assertions["runtimeContractVerified"])
        observation = json.loads((self.root / "runtime/nested-list-observation.json").read_bytes())
        self.assertEqual(observation["observation"]["actualFields"]["FLAT_COUNT"], "999")

    def test_failing_shell_gate_cannot_be_reported_as_verified(self):
        oracle = self.actual_oracle()
        assertions = self.actual_assertions(oracle, run="FAIL_RUNTIME")
        self.assertFalse(assertions["runtimeObservationVerified"])

    def test_publication_recomputes_originals_and_rejects_promotion(self):
        oracle = self.actual_oracle()
        self.actual_assertions(oracle)
        self.write("summary.json", {"privacyGatePassed": True})
        trusted = publication.trusted_fixture_sources(ROOT / "tools/shortcuts-simulator-qa/fixtures")
        published = publication.stage(self.root, Path(self.temporary.name) / "public-control", trusted)
        self.assertIn("runtime/nested-list-observation.json", [item["path"] for item in published["published"]])
        self.assertTrue(any(item["path"].endswith(".shortcut") for item in published["quarantined"]))
        path = self.root / "runtime/nested-list-observation.json"
        original = json.loads(path.read_bytes())
        for key in ("formalAcceptanceGranted", "nativeListTypeProved", "semanticContractVerified"):
            changed = copy.deepcopy(original); changed[key] = True
            self.write("runtime/nested-list-observation.json", changed)
            with self.subTest(key=key), self.assertRaises(publication.PublicationBlocked):
                publication.stage(self.root, Path(self.temporary.name) / ("rejected-" + key), trusted)
        self.write("runtime/nested-list-observation.json", original)
        self.fields["FLAT_COUNT"] = "999"
        self.event["resultSummary"] = observer_controls.summary(self.fixture, self.fields)
        self.write("runtime/callback-events.json", [self.event])
        with self.assertRaises(publication.PublicationBlocked):
            publication.stage(self.root, Path(self.temporary.name) / "stale-report", trusted)


if __name__ == "__main__":
    unittest.main()
