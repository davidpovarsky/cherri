"""Offline provenance controls using one retained real compiler artifact.

All callback, signing and run identities authored below are synthetic test data.
No test launches Go, Shortcuts, a simulator, a signer or a workflow.
"""
import base64
import copy
import json
from pathlib import Path
import plistlib
import subprocess
import sys
import tempfile
import unittest

import tagged_carrier_graph as graph
import tagged_carrier_observation as parser
import tagged_carrier_runtime as runtime
import prepare_public_artifacts as publication
import test_runtime_callback_contract as harness_controls

ROOT = Path(__file__).resolve().parents[2]
QA = ROOT / "tools/shortcuts-simulator-qa"
FIXTURE = parser.FIXTURE
COMMIT = "a" * 40
RUN = "123456"
NONCE = FIXTURE + "-" + RUN + "-synthetic-control-nonce"
ORIGINAL = QA / "fixtures/source-graphs/qa-tagged-value-carrier-v2.compiler-cb23a754.plist"


def b64(value):
    return base64.b64encode(value.encode("utf-8")).decode("ascii")


class SyntheticTaggedRun:
    def __init__(self, root):
        self.root = Path(root)
        (self.root / "builds/signed").mkdir(parents=True)
        (self.root / "runtime").mkdir()
        self.source = (QA / ("fixtures/" + FIXTURE + ".cherri")).read_bytes()
        self.unsigned = ORIGINAL.read_bytes()
        self.signed = b"AEA1SYNTHETIC_CONTROL_ONLY_NOT_AN_APPLE_SIGNATURE"
        (self.root / ("builds/" + FIXTURE + ".cherri")).write_bytes(self.source)
        (self.root / ("builds/" + FIXTURE + ".shortcut")).write_bytes(self.unsigned)
        (self.root / ("builds/signed/" + FIXTURE + ".shortcut")).write_bytes(self.signed)
        self.row = {"fixtureId": FIXTURE, "fileName": FIXTURE + ".cherri", "dialect": "v2",
                    "buildProfile": "debug", "runtimeTraceExpectation": "REQUIRED_NDJSON",
                    "sourceSha256": graph.SOURCE_SHA256, "unsignedPlistSha256": runtime.sha(self.unsigned),
                    "nativeActionCount": 225,
                    "taggedCarrierProbe": runtime.prepared(ROOT, FIXTURE, self.source, self.unsigned)}
        self.manifest = {"schemaVersion": "1.5", "syntheticOnly": True,
                         "compilerCommitSha": COMMIT, "fixtures": [self.row]}
        self.signing = {
            "compilerCommitSha": COMMIT, "signedArtifactValidAEA1": True,
            "signedArtifactSha256": runtime.sha(self.signed), "signedByteSize": len(self.signed),
            "targetProvenance": {
                "compilerCommitSha": COMMIT, "sourceSha256": graph.SOURCE_SHA256,
                "unsignedPlistSha256": runtime.sha(self.unsigned), "unsignedByteSize": len(self.unsigned),
                "nativeActionCount": 225, "signedArtifactSha256": runtime.sha(self.signed),
                "signedByteSize": len(self.signed), "headerMagic": "AEA1"}}
        def collector(stage):
            return {"schemaVersion": "1.4", "kind": "COLLECTOR_EVENT", "stage": stage,
                    "runId": FIXTURE, "traceTransport": "ndjson-stream",
                    "collectorSentinel": "===CHERRI_QA_TRACE_NDJSON_V14==="}
        before = {"schemaVersion": "1.4", "kind": "INLINE_DEBUG_ACTION", "stage": "before",
                  "label": "tagged-value-carrier", "stepSequence": 3,
                  "actionIdentifier": "is.workflow.actions.setvariable",
                  "timestampPrecision": "apple-currentdate", "valueType": "UNKNOWN_AT_TRACE_EVENT",
                  "timestamp": "SYNTHETIC_CONTROL_TIME_NOT_APPLE"}
        after = {**before, "stage": "after", "stepSequence": 218,
                 "actionIdentifier": "is.workflow.actions.comment"}
        self.records = [collector("collector_begin"), before, after, collector("collector_end")]
        self.fields = {**parser.GOALS, "ORIGINAL_LIST_TYPE64": b64("List"),
                       "DECODED_LIST_TYPE64": b64("List"), "TEXT_TYPE64": b64("Text"),
                       "ORIGINAL_MAP_TYPE64": b64("Dictionary"), "DECODED_MAP_TYPE64": b64("Dictionary")}
        self.event = {"receiverSelfTest": False, "eventSource": "shortcuts-x-callback",
                      "runId": NONCE, "fixtureId": FIXTURE, "expectedShortcutName": FIXTURE,
                      "status": "SUCCESS", "resultSummary": self.summary(),
                      "tracePayload": "\n".join(json.dumps(record) for record in self.records)}
        self.write("compiler-manifest.json", self.manifest)
        self.write("runtime/signing-provenance.json", self.signing)
        self.write("runtime/callback-events.json", [self.event])

    def write(self, path, value):
        (self.root / path).write_bytes(runtime.canonical(value))

    def summary(self):
        return parser.PREFIX + ":".join(key + "=" + self.fields[key] for key in parser.FIELDS)

    def observe(self, expected=""):
        return runtime.evaluate(ROOT, self.root, FIXTURE, COMMIT, NONCE, expected)

    def oracle(self, program):
        args = [str(self.root / "runtime/callback-events.json"), NONCE, FIXTURE, "",
                str(self.root / "runtime/runtime-trace.jsonl"), str(self.root / "compiler-manifest.json"),
                COMMIT, str(ROOT)]
        p = subprocess.run([sys.executable, "-B", "-c", program, *args],
                           capture_output=True, text=True, encoding="utf-8", timeout=15)
        if p.returncode:
            raise AssertionError(p.stderr)
        return json.loads(p.stdout)

    def assertions(self, program, oracle):
        args = [str(self.root), oracle["runGate"], "PASS", oracle["traceGate"],
                oracle.get("reason", ""), FIXTURE, "", NONCE, json.dumps(oracle),
                str(self.root / "compiler-manifest.json"), COMMIT, str(ROOT)]
        p = subprocess.run([sys.executable, "-B", "-c", program, *args],
                           capture_output=True, text=True, encoding="utf-8", timeout=15)
        if p.returncode:
            raise AssertionError(p.stderr)
        return runtime.strict_json((self.root / "runtime/output-assertions.json").read_bytes())


class TaggedRuntimeControls(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        harness_controls.CallbackContractTests.setUpClass()
        cls.oracle_program = harness_controls.CallbackContractTests.program
        cls.assertion_program = harness_controls.CallbackContractTests.assertion_program

    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="tagged-runtime-synthetic-")
        self.addCleanup(self.temporary.cleanup)
        self.sample = SyntheticTaggedRun(Path(self.temporary.name).resolve() / RUN)

    def test_retained_real_graph_and_actual_shell_handlers_join_exact_originals(self):
        self.assertEqual(runtime.sha(self.sample.unsigned),
                         "dabf4b0869396585e315a0fbc397945fcdc1efc7defbf64763b274e75b85c2ea")
        observed = self.sample.oracle(self.oracle_program)
        self.assertTrue(observed["runtimeObservationVerified"], observed)
        self.assertTrue(observed["nativeCarrierGoalsMatched"])
        self.assertEqual(len(observed["traceRecords"]), 4)
        self.assertEqual(self.sample.row["taggedCarrierProbe"]["graph"]["actionCount"], 225)
        assertions = self.sample.assertions(self.assertion_program, observed)
        document = runtime.strict_json((self.sample.root / runtime.DIAGNOSTIC).read_bytes())
        runtime.validate_public_diagnostic(document, self.sample.root)
        self.assertTrue(assertions["runtimeContractVerified"])
        self.assertTrue(assertions["diagnosticOnly"])
        for key in ("semanticContractVerified", "formalAcceptanceGranted", "appleExecutionAuthenticated"):
            self.assertIs(assertions[key], False)
            self.assertIs(document[key], False)
        self.assertFalse(document["observation"]["sourceAuthenticated"])
        self.assertFalse(document["observation"]["appleExecutionAuthenticated"])
        for key, path in (("compiledSource", "builds/" + FIXTURE + ".cherri"),
                          ("unsignedShortcut", "builds/" + FIXTURE + ".shortcut"),
                          ("signedShortcut", "builds/signed/" + FIXTURE + ".shortcut"),
                          ("callbackEvents", "runtime/callback-events.json")):
            raw = (self.sample.root / path).read_bytes()
            self.assertEqual(document["inputBindings"][key], {"path": path, "bytes": len(raw), "sha256": runtime.sha(raw)})

    def test_contradictory_type_and_value_observations_survive_actual_handlers(self):
        self.sample.fields.update(DECODED_MAP_TYPE64=b64("Text"), MAP_KEYS="-1",
                               MAP_ALPHA64="UNOBSERVED", MAP_BETA64="UNOBSERVED",
                               FIRST_FIRST64=b64("CONTRARY"))
        self.sample.event["resultSummary"] = self.sample.summary()
        self.sample.write("runtime/callback-events.json", [self.sample.event])
        oracle = self.sample.oracle(self.oracle_program)
        self.assertTrue(oracle["runtimeObservationVerified"])
        self.assertFalse(oracle["nativeCarrierGoalsMatched"])
        self.assertFalse(oracle["observation"]["typeRelations"]["mapMatchesOriginal"])
        assertions = self.sample.assertions(self.assertion_program, oracle)
        self.assertTrue(assertions["runtimeObservationVerified"])
        self.assertFalse(assertions["nativeCarrierGoalsMatched"])
        original = runtime.strict_json((self.sample.root / runtime.DIAGNOSTIC).read_bytes())
        self.assertEqual(original["observation"]["actualFields"], self.sample.fields)
        runtime.validate_public_diagnostic(original, self.sample.root)

    def test_absent_decoded_cell_keeps_all_type_relations_false(self):
        self.sample.fields.update(FIRST_PRESENT="N", FIRST_COUNT="-1", FIRST_FIRST64="UNOBSERVED",
                               FIRST_LAST64="UNOBSERVED", DECODED_LIST_TYPE64="UNOBSERVED")
        self.sample.event["resultSummary"] = self.sample.summary()
        self.sample.write("runtime/callback-events.json", [self.sample.event])
        result = self.sample.observe()
        self.assertTrue(result["runtimeObservationVerified"])
        self.assertFalse(result["observation"]["typeWitnessesComplete"])
        self.assertFalse(any(result["observation"]["typeRelations"].values()))
        self.assertFalse(result["nativeCarrierGoalsMatched"])

    def test_callback_nonce_origin_identity_status_and_duplicate_fail_closed(self):
        for key, value in (("runId", "stale"), ("fixtureId", "qa-nested-list-candidate-v2"),
                           ("expectedShortcutName", "other"), ("eventSource", "receiver-self-test"),
                           ("receiverSelfTest", True), ("receiverSelfTest", 0),
                           ("status", "ERROR"), ("status", "CANCELLED"), ("resultSummary", "SUCCESS")):
            with self.subTest(field=key, value=value):
                self.sample.write("runtime/callback-events.json", [{**self.sample.event, key: value}])
                result = self.sample.observe()
                self.assertFalse(result["runtimeObservationVerified"])
                self.assertEqual(result["runGate"], "FAIL_RUNTIME")
        self.sample.write("runtime/callback-events.json", [self.sample.event, self.sample.event])
        self.assertIn("UNIQUE_NONCE", self.sample.observe()["reason"])

    def test_callback_cannot_supply_its_own_expected_output(self):
        self.assertIn("DIAGNOSTIC_MODE", self.sample.observe(self.sample.event["resultSummary"])["reason"])

    def test_every_static_trace_field_and_order_is_bound(self):
        for index, record in enumerate(self.sample.records):
            for key in record:
                if key == "timestamp":
                    continue
                with self.subTest(event=index, field=key):
                    records = copy.deepcopy(self.sample.records)
                    records[index][key] = "WRONG"
                    self.sample.write("runtime/callback-events.json",
                                   [{**self.sample.event, "tracePayload": "\n".join(map(json.dumps, records))}])
                    result = self.sample.observe()
                    self.assertEqual(result["runGate"], "PASS")
                    self.assertEqual(result["traceGate"], "FAIL_RUNTIME")
                    self.assertFalse(result["runtimeObservationVerified"])
        for records in (self.sample.records[:-1], self.sample.records + [self.sample.records[-1]],
                        list(reversed(self.sample.records))):
            with self.subTest(inventory=len(records), first=records[0]["stage"]):
                self.sample.write("runtime/callback-events.json",
                               [{**self.sample.event, "tracePayload": "\n".join(map(json.dumps, records))}])
                self.assertFalse(self.sample.observe()["runtimeObservationVerified"])
        for stamp in ("", "\ufffc", "\x00", "x" * 257, None):
            with self.subTest(timestamp=repr(stamp)[:20]):
                records = copy.deepcopy(self.sample.records); records[1]["timestamp"] = stamp
                self.sample.write("runtime/callback-events.json",
                               [{**self.sample.event, "tracePayload": "\n".join(map(json.dumps, records))}])
                self.assertFalse(self.sample.observe()["runtimeObservationVerified"])

    def test_missing_trace_never_gets_fabricated_by_actual_assertion_handler(self):
        event = {**self.sample.event, "tracePayload": ""}
        self.sample.write("runtime/callback-events.json", [event])
        oracle = self.sample.oracle(self.oracle_program)
        assertions = self.sample.assertions(self.assertion_program, oracle)
        self.assertFalse(assertions["runtimeObservationVerified"])
        self.assertFalse((self.sample.root / "runtime/runtime-trace.jsonl").exists())
        self.assertEqual(runtime.strict_json((self.sample.root / runtime.DIAGNOSTIC).read_bytes())["traceRecords"], [])

    def test_manifest_and_signing_provenance_fields_are_exact_and_typed(self):
        for key, value in (("nativeActionCount", 225.0), ("sourceSha256", "0" * 64),
                           ("unsignedPlistSha256", "0" * 64), ("taggedCarrierProbe", None),
                           ("dialect", "legacy"), ("buildProfile", "release"),
                           ("runtimeTraceExpectation", "ABSENT_RELEASE")):
            with self.subTest(manifest=key):
                changed = copy.deepcopy(self.sample.manifest); changed["fixtures"][0][key] = value
                self.sample.write("compiler-manifest.json", changed)
                self.assertFalse(self.sample.observe()["runtimeObservationVerified"])
        self.sample.write("compiler-manifest.json", self.sample.manifest)
        for key in self.sample.signing["targetProvenance"]:
            with self.subTest(signing=key):
                changed = copy.deepcopy(self.sample.signing); changed["targetProvenance"][key] = "WRONG"
                self.sample.write("runtime/signing-provenance.json", changed)
                self.assertFalse(self.sample.observe()["runtimeObservationVerified"])
        self.sample.write("runtime/signing-provenance.json", self.sample.signing)
        for key, value in (("compilerCommitSha", "b" * 40), ("signedArtifactValidAEA1", 1),
                           ("signedArtifactSha256", "b" * 64), ("signedByteSize", float(len(self.sample.signed)))):
            with self.subTest(signing_summary=key):
                self.sample.write("runtime/signing-provenance.json", {**self.sample.signing, key: value})
                self.assertFalse(self.sample.observe()["runtimeObservationVerified"])

    def test_full_graph_cannot_be_replaced_by_reconciled_hashes(self):
        changed = plistlib.loads(self.sample.unsigned)
        changed["WFWorkflowActions"][-1]["WFWorkflowActionParameters"]["WFOutput"] = "SUCCESS"
        raw = plistlib.dumps(changed)
        (self.sample.root / ("builds/" + FIXTURE + ".shortcut")).write_bytes(raw)
        altered = copy.deepcopy(self.sample.manifest)
        altered["fixtures"][0]["unsignedPlistSha256"] = runtime.sha(raw)
        altered["fixtures"][0]["taggedCarrierProbe"]["unsignedPlistSha256"] = runtime.sha(raw)
        self.sample.write("compiler-manifest.json", altered)
        signing = copy.deepcopy(self.sample.signing)
        signing["targetProvenance"].update(unsignedPlistSha256=runtime.sha(raw), unsignedByteSize=len(raw))
        self.sample.write("runtime/signing-provenance.json", signing)
        result = self.sample.observe()
        self.assertFalse(result["runtimeObservationVerified"])
        self.assertIn("GRAPH_", result["reason"])

    def test_source_and_signed_bytes_missing_originals_and_symlinks_fail_closed(self):
        for path in ("builds/" + FIXTURE + ".cherri", "builds/signed/" + FIXTURE + ".shortcut"):
            with self.subTest(path=path):
                p = self.sample.root / path; original = p.read_bytes()
                p.write_bytes(original + b"changed")
                self.assertFalse(self.sample.observe()["runtimeObservationVerified"])
                p.unlink()
                self.assertFalse(self.sample.observe()["runtimeObservationVerified"])
                outside = Path(self.temporary.name).resolve() / "outside"; outside.write_bytes(original)
                p.symlink_to(outside)
                self.assertFalse(self.sample.observe()["runtimeObservationVerified"])
                p.unlink(); p.write_bytes(original)

    def test_malformed_xml_has_a_bounded_rejection_from_actual_runtime_handler(self):
        target = self.sample.root / ("builds/" + FIXTURE + ".shortcut")
        target.write_bytes(self.sample.unsigned + b"trailing junk")
        result = self.sample.oracle(self.oracle_program)
        self.assertEqual(result["runGate"], "FAIL_RUNTIME")
        self.assertEqual(result["reason"], "TAGGED_RUNTIME_UNSIGNED_PLIST")
        self.assertFalse(result["runtimeObservationVerified"])
        self.assertFalse((self.sample.root / "runtime/runtime-trace.jsonl").exists())

    def test_duplicate_json_nonfinite_and_inflated_callback_are_rejected(self):
        for raw in (b'[{"runId":"x","runId":"y"}]', b'[{"value":NaN}]',
                    b" " * (runtime.MAX_CALLBACK + 1)):
            with self.subTest(length=len(raw)):
                (self.sample.root / "runtime/callback-events.json").write_bytes(raw)
                self.assertFalse(self.sample.observe()["runtimeObservationVerified"])

    def test_oracle_and_public_reports_are_independently_recomputed(self):
        oracle = self.sample.oracle(self.oracle_program)
        tampered = copy.deepcopy(oracle); tampered["observation"]["actualFields"]["FIRST_COUNT"] = "999"
        assertions = self.sample.assertions(self.assertion_program, tampered)
        self.assertFalse(assertions["runtimeObservationVerified"])
        self.assertFalse(assertions["nativeCarrierGoalsMatched"])
        self.assertFalse(assertions["callbackOracleAgreement"])
        self.sample.assertions(self.assertion_program, oracle)
        value = runtime.strict_json((self.sample.root / runtime.DIAGNOSTIC).read_bytes())
        for field in ("formalAcceptanceGranted", "appleExecutionAuthenticated", "semanticContractVerified"):
            with self.subTest(field=field):
                changed = {**value, field: True}
                with self.assertRaises(ValueError):
                    runtime.validate_public_diagnostic(changed, self.sample.root)

    def test_original_engineering_contract_and_old_nested_targets_are_not_relabelled(self):
        expected = QA / ("fixtures/" + FIXTURE + ".expected.json")
        self.assertEqual(runtime.sha(expected.read_bytes()), graph.CONTRACT_SHA256)
        self.assertEqual(json.loads(expected.read_bytes())["enrollment"], "UNENROLLED_ENGINEERING_FIXTURE")
        for fixture in ("qa-nested-list-control-v2", "qa-nested-list-candidate-v2", "qa-unit"):
            self.assertIsNone(runtime.prepared(ROOT, fixture, b"unrelated", b"unrelated"))
        original = (ROOT / "scripts/ios27_shortcuts_simulator_qa.sh").read_text()
        self.assertIn('from nested_list_runtime import emit', original)
        self.assertIn('from nested_list_runtime import apply_assertions', original)

    def test_enrollment_bytes_cannot_silently_change_scope(self):
        root = Path(self.temporary.name).resolve() / "repo"
        dest = root / runtime.ENROLLMENT
        dest.parent.mkdir(parents=True)
        dest.write_bytes((ROOT / runtime.ENROLLMENT).read_bytes() + b" ")
        with self.assertRaisesRegex(ValueError, "ENROLLMENT_SOURCE"):
            runtime.enrollment(root)


if __name__ == "__main__":
    unittest.main()
