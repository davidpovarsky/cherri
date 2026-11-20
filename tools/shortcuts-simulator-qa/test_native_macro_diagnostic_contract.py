"""Strict dispatch and callback contracts for the next causal native split."""
import ast
import base64
import hashlib
import json
import os
import re
from pathlib import Path
import subprocess
import unittest

from test_workflow_input_contract import bash_path
import test_runtime_callback_contract as callback_contract
from fixture_compile_contract import EXPECTED_REJECTIONS
import file_materialization_diagnostic as file_probe
import file_carrier_controls as carrier_controls
import n03_text_equality_diagnostic as n03_probe
import tempfile


CASES = {
    **carrier_controls.EXPECTED,
    file_probe.FIXTURE_ID: file_probe.EXPECTED_OUTPUT,
    n03_probe.FIXTURE_ID: n03_probe.EXPECTED_OUTPUT,
    "qa-export-unicode-diagnostic-v2": "CHERRI_QA_EXPORT_UNICODE_DIAG:TYPE=Y:NAME=Y:COUNT=Y:STATUS=Y:PAYLOAD=Y:RESTORED_EQ=Y:RESTORED_NE=N:LITERAL_EQ=Y:LITERAL_NE=N:UNCHANGED=Y:ORIGINAL64=cXVvdGUgIiBiYWNrc2xhc2ggXCBuZXdsaW5lCkhlYnJldyDXqda414HXnNeV1rnXnSBjaGVycnkg8J+NkiB0b2tlbiBTWU5USEVUSUNfU0VDUkVU:PAYLOAD_TEXT64=Y1hWdmRHVWdJaUJpWVdOcmMyeGhjMmdnWENCdVpYZHNhVzVsQ2tobFluSmxkeURYcWRhNDE0SFhuTmVWMXJuWG5TQmphR1Z5Y25rZzhKK05raUIwYjJ0bGJpQlRXVTVVU0VWVVNVTmZVMFZEVWtWVQ==:DECODED_FILE64=cXVvdGUgIiBiYWNrc2xhc2ggXCBuZXdsaW5lCkhlYnJldyDXqda414HXnNeV1rnXnSBjaGVycnkg8J+NkiB0b2tlbiBTWU5USEVUSUNfU0VDUkVU:RESTORED64=cXVvdGUgIiBiYWNrc2xhc2ggXCBuZXdsaW5lCkhlYnJldyDXqda414HXnNeV1rnXnSBjaGVycnkg8J+NkiB0b2tlbiBTWU5USEVUSUNfU0VDUkVU:LITERAL64=cXVvdGUgIiBiYWNrc2xhc2ggXCBuZXdsaW5lCkhlYnJldyDXqda414HXnNeV1rnXnSBjaGVycnkg8J+NkiB0b2tlbiBTWU5USEVUSUNfU0VDUkVU:AFTER64=cXVvdGUgIiBiYWNrc2xhc2ggXCBuZXdsaW5lCkhlYnJldyDXqda414HXnNeV1rnXnSBjaGVycnkg8J+NkiB0b2tlbiBTWU5USEVUSUNfU0VDUkVU:NAME64=Y2hlcnJpLWRlYnVnLmpzb24=:STATUS64=VEVYVF9QQVlMT0FEX0VOQ09ERUQ=",
    "qa-number-coercion-control-v2": "CHERRI_QA_NUMBER_COERCION_RESULT:DEQ=Y:DNE=N:REQ=Y:RNE=N:BEQ=Y:BNE=N:ONE_NE_ZERO=Y:ZERO_NE_ONE=Y:ONE_EQ_ONE=Y",
    "qa-nil-clear-v2": "CHERRI_QA_NIL_CLEAR:binding=0:assignment=0:control=0:present=1",
    "qa-require-zero-diag-v2": "CHERRI_QA_REQUIRE_ZERO_DIAG:L=ok:D=ok:R=ok:B=ok",
    "qa-label-file-controls-v2": "CHERRI_QA_LABEL_FILE_CONTROLS:B=ok:V=ok:VT=ok:L=ok:T=ok:R=ok:S=ok",
    "qa-export-file-control-v2": "CHERRI_QA_EXPORT_FILE_CONTROL:NAME=ok:TEXT=ok",
    "qa-export-build-only-v2": "CHERRI_QA_EXPORT_BUILD_ONLY:NAME=ok",
    "qa-export-parse-only-v2": "CHERRI_QA_EXPORT_PARSE_ONLY:SCHEMA=ok:COUNT=ok",
    "qa-collect-empty-split-v2": "CHERRI_QA_COLLECT_EMPTY_SPLIT:direct=0:repeat=0:cleared=0:emptyText=1",
    "qa-export-file-bytes-v2": "CHERRI_QA_EXPORT_FILE_BYTES:NAME=ok:TEXT=ok",
    "qa-file-name-binding-v2": "CHERRI_QA_FILE_NAME_BINDING:L=ok:V=ok:I=ok:LT=ok:VT=ok:IT=ok",
    "qa-file-name-extension-v2": "CHERRI_QA_FILE_NAME_EXTENSION:NAME=ok:EXTENSION=ok:TYPE=ok:BYTES=ok:TEXT=ok",
    "qa-f04-yield-list-v2": "CHERRI_QA_F04_LIST_RESULT:pairs=2:scalars=2:shape=LEFT,TAIL|RIGHT,TAIL",
    "qa-f04-yield-empty-list-v2": "CHERRI_QA_F04_EMPTY_LIST_RESULT:outer=2:scalars=2:shape=0,0",
    "qa-f01-ifresult-let-v2": "CHERRI_QA_F01_IFRESULT:P15_F01_IFRESULT_OK",
    "qa-f01-ifresult-var-v2": "CHERRI_QA_F01_IFRESULT:P15_F01_IFRESULT_OK",
}


class NativeMacroDiagnosticContracts(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.root = Path(__file__).resolve().parents[2]
        cls.script = (cls.root / "scripts/ios27_shortcuts_simulator_qa.sh").read_text(encoding="utf-8")
        cls.bash = bash_path()
        callback_contract.CallbackContractTests.setUpClass()
        cls.oracle = callback_contract.CallbackContractTests()

    def test_exact_reviewed_source_bindings_and_rejected_sample_variants(self):
        if not self.bash:
            self.skipTest("bash unavailable; dispatch validation was not executed")
        selector = self.script.split("# Select primary target fixture from SAMPLE_ID\n", 1)[1].split("LEGACY_COUNTERPART_SOURCE=", 1)[0]
        selector += '\nprintf "%s\\n" "$TARGET_FIXTURE_ID" "$TARGET_SOURCE_FILE" "$EXPECTED_SHORTCUT_NAME" "$EXPECTED_SEMANTIC_OUTPUT" "$HTTP_PROBE_ENABLED"\n'
        validation = self.script[:self.script.index('ROOT_DIR="')]
        workflow = (self.root / ".github/workflows/cherri-ios27-shortcuts-simulator-qa.yml").read_text(encoding="utf-8")
        for fid, expected in CASES.items():
            sample = fid.removesuffix("-v2")
            with self.subTest(fixture=fid):
                self.assertIn("          - " + sample + "\n", workflow)
                result = subprocess.run([self.bash, "-s"], input=selector,
                    env={**os.environ, "ROOT_DIR": "/reviewed", "SAMPLE_ID": sample,
                         "TARGET_SOURCE_RELATIVE": "../../private.cherri", "HTTP_PROBE_ENABLED": "true"},
                    capture_output=True, text=True, timeout=10)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(result.stdout.splitlines(), [fid,
                    "/reviewed/tools/shortcuts-simulator-qa/fixtures/" + fid + ".cherri",
                    fid, expected, "false"])
                for choice, code in ((sample, 0), (sample + "-release", 2), (sample + ";exit 0", 2)):
                    result = subprocess.run([self.bash, "-c", validation, "qa-validation",
                                             "runtime", "ipad-default", choice, "false", "1", "true"],
                                            capture_output=True, text=True, timeout=10)
                    self.assertEqual(result.returncode, code, (choice, result.stderr))
        for sample in ("qa-label-file-private", "qa-export-build-private", "qa-require-zero-private"):
            result = subprocess.run([self.bash, "-c", validation, "qa-validation",
                                     "runtime", "ipad-default", sample, "false", "1", "true"],
                                    capture_output=True, text=True, timeout=10)
            self.assertEqual(result.returncode, 2)

    def test_each_independent_flag_and_actual_trace_remain_required(self):
        for fid, expected in CASES.items():
            event = self.oracle.event()
            event.update(fixtureId=fid, expectedShortcutName=fid, resultSummary=expected)
            event["tracePayload"] = event["tracePayload"].replace("qa-unit", fid)
            with self.subTest(fixture=fid):
                result = self.oracle.verify([event], fid, expected)
                self.assertEqual((result["runGate"], result["traceGate"]), ("PASS", "PASS"))
                self.assertEqual(self.oracle.verify([{**event, "resultSummary": expected + "-WRONG"}], fid, expected)["runGate"], "FAIL_RUNTIME")
                # Reject a wrong value in every independently reported flag,
                # including later flags that a single verdict used to mask.
                for match in re.finditer(r"=(ok|Y|N|[0-9]+)(?=:|$)", expected):
                    old = match.group(1)
                    wrong_value = "bad" if old == "ok" else "N" if old == "Y" else "Y" if old == "N" else "1" if old == "0" else "0"
                    wrong = expected[:match.start(1)] + wrong_value + expected[match.end(1):]
                    self.assertEqual(self.oracle.verify([{**event, "resultSummary": wrong}], fid, expected)["runGate"],
                                     "FAIL_RUNTIME", wrong)
                for key, value in (("status", "ERROR"), ("status", "CANCELLED"), ("runId", "stale"),
                                   ("expectedShortcutName", "qa-private"), ("receiverSelfTest", True)):
                    self.assertEqual(self.oracle.verify([{**event, key: value}], fid, expected)["runGate"], "FAIL_RUNTIME")
                self.assertEqual(self.oracle.verify([{**event, "tracePayload": ""}], fid, expected)["traceGate"], "FAIL_RUNTIME")
                self.assertEqual(self.oracle.verify([event, event], fid, expected)["runGate"], "FAIL_RUNTIME")
                manifest = self.oracle.manifest(fid)
                manifest["fixtures"][0].update(buildProfile="release", runtimeTraceExpectation="ABSENT_RELEASE")
                self.assertEqual(self.oracle.verify([event], fid, expected, manifest)["runGate"], "FAIL_RUNTIME")

    def test_file_carrier_sources_and_every_actual_field_remain_required(self):
        for fid, expected in carrier_controls.EXPECTED.items():
            source = (self.root / "tools/shortcuts-simulator-qa/fixtures" / (fid + ".cherri")).read_bytes()
            receipt = carrier_controls.verify_source(self.root, fid, source)
            self.assertEqual(receipt["appleRuntime"], "NOT_RUN")
            self.assertFalse(receipt["formalAcceptanceGranted"])
            with self.assertRaises(ValueError):
                carrier_controls.verify_source(self.root, fid, source + b"\n// changed\n")
            event = self.oracle.event()
            event.update(fixtureId=fid, expectedShortcutName=fid, resultSummary=expected)
            event["tracePayload"] = event["tracePayload"].replace("qa-unit", fid)
            self.assertEqual(self.oracle.verify([event], fid, expected)["runGate"], "PASS")
            for label, _, _, correct in carrier_controls.FIELDS:
                for altered in ("WRONG", "OVERSIZE", "A" * 513):
                    with self.subTest(fixture=fid, field=label, altered=altered[:10]):
                        wrong = expected.replace(":" + label + "=" + correct, ":" + label + "=" + altered)
                        self.assertNotEqual(wrong, expected)
                        self.assertEqual(self.oracle.verify([{**event, "resultSummary": wrong}], fid, expected)["runGate"], "FAIL_RUNTIME")
        workflow = (self.root / ".github/workflows/cherri-ios27-shortcuts-simulator-qa.yml").read_text(encoding="utf-8")
        self.assertIn("inputs.sample_id == 'qa-file-carrier-controls'", workflow)
        self.assertIn("fail-fast: false", workflow)
        self.assertIn("max-parallel: 1", workflow)
        self.assertIn("timeout-minutes: 45", workflow)

    def test_file_materialization_type_count_and_byte_fields_are_independent(self):
        fid, expected = file_probe.FIXTURE_ID, file_probe.EXPECTED_OUTPUT
        source = (self.root / "tools/shortcuts-simulator-qa/fixtures" / (fid + ".cherri")).read_bytes()
        self.assertEqual(file_probe.verify_source(self.root, fid, source)["sourceSha256"], file_probe.SOURCE_SHA256)
        with self.assertRaises(ValueError):
            file_probe.verify_source(self.root, fid, source + b"\n// mutation\n")
        for name, digest in (("qa-export-empty-v2", "86a22c60dfa32590012f825c954ad3bba1317d0c8d31e872e2869b06774d7943"),
                             ("qa-export-unicode-v2", "073c2d4fbbe461f35b17796e4fef814f65f85a71acd0a5efa9ad0f195287b079")):
            original = self.root / "tools/shortcuts-simulator-qa/fixtures/p15-export-probes" / (name + ".cherri")
            self.assertEqual(hashlib.sha256(original.read_bytes()).hexdigest(), digest)
        event = self.oracle.event()
        event.update(fixtureId=fid, expectedShortcutName=fid, resultSummary=expected)
        event["tracePayload"] = event["tracePayload"].replace("qa-unit", fid)
        self.assertEqual(self.oracle.verify([event], fid, expected)["runGate"], "PASS")
        self.assertEqual(len(file_probe.EXPECTED_FIELDS), 18)
        for label, _, correct in file_probe.EXPECTED_FIELDS:
            for altered in ("WRONG", "OVERSIZE", "A" * 513):
                with self.subTest(field=label, altered=altered[:10]):
                    wrong = expected.replace(":" + label + "=" + correct, ":" + label + "=" + altered)
                    self.assertNotEqual(wrong, expected)
                    self.assertEqual(self.oracle.verify([{**event, "resultSummary": wrong}], fid, expected)["runGate"], "FAIL_RUNTIME")

    def test_n03_public_source_target_and_operand_witnesses_remain_independent(self):
        fid, expected = n03_probe.FIXTURE_ID, n03_probe.EXPECTED_OUTPUT
        source = (self.root / n03_probe.SOURCE_PATH).read_bytes()
        receipt = n03_probe.verify_source(self.root, fid, source)
        self.assertFalse(receipt["formalAcceptanceGranted"])
        self.assertEqual(receipt["appleRuntime"], "NOT_RUN")
        with self.assertRaises(ValueError):
            n03_probe.verify_source(self.root, fid, source.replace(b'text("ABC")', b'text("abc")'))
        with tempfile.TemporaryDirectory() as temporary:
            copied = Path(temporary) / n03_probe.SOURCE_PATH
            copied.parent.mkdir(parents=True)
            copied.write_bytes(source)
            contract = json.loads((self.root / n03_probe.SOURCE_PATH).with_suffix(".expected.json").read_bytes())
            contract["expected_fields_for_exact_equality"]["DYN_CASE_EQ"] = "Y"
            copied.with_suffix(".expected.json").write_text(json.dumps(contract), encoding="utf-8")
            with self.assertRaises(ValueError):
                n03_probe.verify_source(temporary, fid, source)
        event = self.oracle.event()
        event.update(fixtureId=fid, expectedShortcutName=fid, resultSummary=expected)
        event["tracePayload"] = event["tracePayload"].replace("qa-unit", fid)
        self.assertEqual(self.oracle.verify([event], fid, expected)["runGate"], "PASS")
        for field, correct in n03_probe.EXPECTED_FIELDS[:3]:
            for altered in ("WRONG", "AQ==", ""):
                wrong = expected.replace(":" + field + "=" + correct, ":" + field + "=" + altered)
                self.assertNotEqual(wrong, expected)
                result = self.oracle.verify([{**event, "resultSummary": wrong}], fid, expected)
                self.assertEqual(result["runGate"], "FAIL_RUNTIME")

    def test_unicode_byte_witnesses_are_independent_and_required(self):
        fid = "qa-export-unicode-diagnostic-v2"
        contract = json.loads((self.root / "docs/apple-runtime/p10r15/export-unicode-diagnostic-contract.json").read_text(encoding="utf-8"))
        original = self.root / "tools/shortcuts-simulator-qa/fixtures/p15-export-probes/qa-export-unicode-v2.cherri"
        self.assertEqual(hashlib.sha256(original.read_bytes()).hexdigest(),
                         "073c2d4fbbe461f35b17796e4fef814f65f85a71acd0a5efa9ad0f195287b079")
        source = self.root / "tools/shortcuts-simulator-qa/fixtures" / (fid + ".cherri")
        self.assertEqual(hashlib.sha256(source.read_bytes()).hexdigest(), contract["fixtureSha256"])
        expected = CASES[fid]
        self.assertEqual(expected, contract["expectedCallbackSummary"])
        self.assertIs(contract["diagnosticOnly"], True)
        self.assertIs(contract["formalAcceptanceGranted"], False)
        literal = base64.b64decode(contract["expectedOriginalBase64"], validate=True)
        self.assertEqual(len(literal), 84)
        self.assertEqual(literal.decode("utf-8"),
                         'quote " backslash \\ newline\nHebrew שָׁלוֹם cherry 🍒 token SYNTHETIC_SECRET')
        event = self.oracle.event()
        event.update(fixtureId=fid, expectedShortcutName=fid, resultSummary=expected)
        event["tracePayload"] = event["tracePayload"].replace("qa-unit", fid)
        fields = dict(item.split("=", 1) for item in expected.split(":")[1:])
        self.assertEqual(len(contract["byteWitnesses"]), 8)
        for witness in contract["byteWitnesses"]:
            key, correct = witness["name"], witness["expectedBase64"]
            with self.subTest(witness=key):
                self.assertEqual(fields[key], correct)
                self.assertLessEqual(len(correct), 512)
                base64.b64decode(correct, validate=True)
                for altered in ("OVERSIZE", "AQ==", correct[:-1], "A" * 513):
                    wrong = expected.replace(":" + key + "=" + correct, ":" + key + "=" + altered)
                    self.assertNotEqual(wrong, expected)
                    self.assertEqual(self.oracle.verify([{**event, "resultSummary": wrong}], fid, expected)["runGate"],
                                     "FAIL_RUNTIME")
        self.assertEqual(base64.b64decode(fields["PAYLOAD_TEXT64"]).decode("ascii"),
                         contract["expectedOriginalBase64"])
        self.assertEqual(base64.b64decode(fields["NAME64"]), b"cherri-debug.json")
        self.assertEqual(base64.b64decode(fields["STATUS64"]), b"TEXT_PAYLOAD_ENCODED")

    def test_f04_flattening_dropped_empties_and_inner_value_loss_are_rejected(self):
        wrong_results = {
            "qa-f04-yield-list-v2": (
                "CHERRI_QA_F04_LIST_RESULT:pairs=4:scalars=2:shape=COUNT_MISMATCH",
                "CHERRI_QA_F04_LIST_RESULT:pairs=2:scalars=2:shape=LEFT,TAIL|LEFT,TAIL",
                "CHERRI_QA_F04_LIST_RESULT:pairs=2:scalars=2:shape=LEFT,TAIL|RIGHT,LOST",
            ),
            "qa-f04-yield-empty-list-v2": (
                "CHERRI_QA_F04_EMPTY_LIST_RESULT:outer=0:scalars=2:shape=COUNT_MISMATCH",
                "CHERRI_QA_F04_EMPTY_LIST_RESULT:outer=2:scalars=2:shape=0,1",
                "CHERRI_QA_F04_EMPTY_LIST_RESULT:outer=2:scalars=0:shape=0,0",
            ),
        }
        for fid, wrong in wrong_results.items():
            event = self.oracle.event()
            event.update(fixtureId=fid, expectedShortcutName=fid)
            event["tracePayload"] = event["tracePayload"].replace("qa-unit", fid)
            for summary in wrong:
                with self.subTest(fixture=fid, observed=summary):
                    result = self.oracle.verify([{**event, "resultSummary": summary}], fid, CASES[fid])
                    self.assertEqual(result["runGate"], "FAIL_RUNTIME")

    def test_phase_zero_records_explicit_outcomes_for_reviewed_sources(self):
        start = self.script.index("fixture_specs = [")
        stop = self.script.index("\n]\n", start) + 2
        assignment = ast.parse(self.script[start:stop]).body[0]
        specs = ast.literal_eval(assignment.value)
        self.assertEqual(set(EXPECTED_REJECTIONS), {"qa-f04-yield-list-v2", "qa-f04-yield-empty-list-v2"})
        self.assertIn("verify_expected_rejection(fid,", self.script)
        self.assertIn('"fixtureRejections": fixture_rejections', self.script)
        for fid in CASES:
            matches = [row for row in specs if row[0] == fid]
            self.assertEqual(matches, [(fid, fid + ".cherri", "v2")])
            source = (self.root / "tools/shortcuts-simulator-qa/fixtures" / (fid + ".cherri")).read_text(encoding="utf-8")
            self.assertIn('debugClearLogs()', source)
            self.assertIn('debugExportTrace(runId: "' + fid + '")', source)
        # Existing originals remain separate selectable sources, not replacements.
        for fid, source in (
            ("qa-require-v2", "qa-require-v2.cherri"),
            ("qa-label-file-carrier-v2", "p15-label-probes/qa-label-file-carrier-v2.cherri"),
            ("qa-export-unicode-v2", "p15-export-probes/qa-export-unicode-v2.cherri"),
        ):
            self.assertIn((fid, source, "v2"), specs)


if __name__ == "__main__":
    unittest.main()
