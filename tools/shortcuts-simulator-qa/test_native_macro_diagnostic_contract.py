"""Strict dispatch and callback contracts for the next causal native split."""
import ast
import os
from pathlib import Path
import subprocess
import unittest

from test_workflow_input_contract import bash_path
import test_runtime_callback_contract as callback_contract


CASES = {
    "qa-require-zero-diag-v2": "CHERRI_QA_REQUIRE_ZERO_DIAG:L=ok:D=ok:R=ok:B=ok",
    "qa-label-file-controls-v2": "CHERRI_QA_LABEL_FILE_CONTROLS:B=ok:V=ok:VT=ok:L=ok:T=ok:R=ok:S=ok",
    "qa-export-file-control-v2": "CHERRI_QA_EXPORT_FILE_CONTROL:NAME=ok:TEXT=ok",
    "qa-export-build-only-v2": "CHERRI_QA_EXPORT_BUILD_ONLY:NAME=ok",
    "qa-export-parse-only-v2": "CHERRI_QA_EXPORT_PARSE_ONLY:SCHEMA=ok:COUNT=ok",
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
                # Reject a wrong value in every independently reported flag,
                # including later flags that a single verdict used to mask.
                start = 0
                while True:
                    index = expected.find("=ok", start)
                    if index < 0:
                        break
                    wrong = expected[:index] + "=bad" + expected[index + 3:]
                    self.assertEqual(self.oracle.verify([{**event, "resultSummary": wrong}], fid, expected)["runGate"],
                                     "FAIL_RUNTIME", wrong)
                    start = index + 3
                for key, value in (("status", "ERROR"), ("status", "CANCELLED"), ("runId", "stale"),
                                   ("expectedShortcutName", "qa-private"), ("receiverSelfTest", True)):
                    self.assertEqual(self.oracle.verify([{**event, key: value}], fid, expected)["runGate"], "FAIL_RUNTIME")
                self.assertEqual(self.oracle.verify([{**event, "tracePayload": ""}], fid, expected)["traceGate"], "FAIL_RUNTIME")
                self.assertEqual(self.oracle.verify([event, event], fid, expected)["runGate"], "FAIL_RUNTIME")
                manifest = self.oracle.manifest(fid)
                manifest["fixtures"][0].update(buildProfile="release", runtimeTraceExpectation="ABSENT_RELEASE")
                self.assertEqual(self.oracle.verify([event], fid, expected, manifest)["runGate"], "FAIL_RUNTIME")

    def test_phase_zero_compiles_all_new_sources_at_the_existing_debug_boundary(self):
        start = self.script.index("fixture_specs = [")
        stop = self.script.index("\n]\n", start) + 2
        assignment = ast.parse(self.script[start:stop]).body[0]
        specs = ast.literal_eval(assignment.value)
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
