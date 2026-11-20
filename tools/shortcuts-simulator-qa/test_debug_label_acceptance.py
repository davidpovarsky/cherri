"""Source, dispatch and real callback gates for public label acceptance cases.

Actual compiler graph controls also run inside the orchestrator before signing.
These synthetic controls do not establish Apple runtime or visual acceptance.
"""
import ast
import base64
import json
import os
from pathlib import Path
import re
import subprocess
import unittest

import debug_label_acceptance as labels
import test_runtime_callback_contract as callback_contract
from test_workflow_input_contract import bash_path


class DebugLabelAcceptanceContracts(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.root = Path(__file__).resolve().parents[2]
        cls.shell = (cls.root / "scripts/ios27_shortcuts_simulator_qa.sh").read_text(encoding="utf-8")
        cls.workflow = (cls.root / ".github/workflows/cherri-ios27-shortcuts-simulator-qa.yml").read_text(encoding="utf-8")
        callback_contract.CallbackContractTests.setUpClass()
        cls.oracle = callback_contract.CallbackContractTests()

    def event(self, fid):
        event = self.oracle.event()
        event.update(fixtureId=fid, expectedShortcutName=fid,
                     resultSummary=labels.expected_summary(fid))
        event["tracePayload"] = event["tracePayload"].replace("qa-unit", fid)
        return event

    def test_twelve_exact_public_sources_and_six_mode_pairs(self):
        self.assertEqual(len(labels.SOURCE_HASHES), 12)
        self.assertEqual(len(set(labels.SOURCE_HASHES.values())), 12)
        for mode in labels.MODES:
            v2, legacy = [f"qa-debug-label-{mode}-{dialect}" for dialect in ("v2", "legacy")]
            self.assertEqual(labels.expected_summary(v2), labels.expected_summary(legacy))
            for fid in (v2, legacy):
                with self.subTest(fixture=fid):
                    source = (self.root / "tools/shortcuts-simulator-qa/fixtures" / (fid + ".cherri")).read_bytes()
                    receipt = labels.verify_source(self.root, fid, source)
                    self.assertEqual(receipt["nativeExecution"], "NOT_RUN")
                    self.assertFalse(receipt["formalAcceptanceGranted"])
                    with self.assertRaisesRegex(ValueError, "reviewed source bytes"):
                        labels.verify_source(self.root, fid, source + b"\n// changed\n")
        self.assertIsNone(labels.verify_source(self.root, "qa-private-lookalike", b"private"))
        self.assertIsNone(labels.verify_graph("qa-private-lookalike", []))

    def test_all_fifty_observations_keep_independent_fixed_targets(self):
        self.assertEqual(len(labels.FIELDS), 50)
        self.assertEqual(len({row[0] for row in labels.FIELDS}), 50)
        self.assertEqual(len({row[1] for row in labels.FIELDS}), 50)
        values = {key: value for key, _, value in labels.FIELDS}
        self.assertEqual(values["BUFFER_COUNT"], "7")
        self.assertEqual(values["METADATA_COUNT"], "6")
        self.assertEqual(values["FILE_TYPE64"], base64.b64encode(b"File").decode())
        self.assertEqual(values["MAP_TYPE64"], base64.b64encode(b"Dictionary").decode())
        self.assertEqual(values["FILE_BYTES64"], "AQIDBA==")
        self.assertEqual(values["SOURCE_FILE_BYTES64"], "AQIDBA==")
        self.assertEqual(values["M2_KEYS"], "3")
        self.assertEqual(values["M2_LABEL64"], "")
        self.assertEqual((values["M4_FIRST"], values["M4_LAST"]), ("4", "5"))
        self.assertEqual((values["RAW_CLEAR"], values["METADATA_CLEAR"]), ("0", "0"))
        self.assertEqual(base64.b64decode(values["M1_LABEL64"]).decode(), labels.LABELS[0])

    def test_real_static_dispatch_and_untrusted_selector_rejection(self):
        bash = bash_path()
        if not bash:
            self.skipTest("bash unavailable; no shell execution claimed")
        selector = self.shell.split("# Select primary target fixture from SAMPLE_ID\n", 1)[1].split("LEGACY_COUNTERPART_SOURCE=", 1)[0]
        selector += '\nprintf "%s\\n" "$TARGET_FIXTURE_ID" "$TARGET_SOURCE_FILE" "$EXPECTED_SHORTCUT_NAME" "$EXPECTED_SEMANTIC_OUTPUT" "$HTTP_PROBE_ENABLED"\n'
        validation = self.shell[:self.shell.index('ROOT_DIR="')]
        for fid in labels.SOURCE_HASHES:
            with self.subTest(fixture=fid):
                self.assertIn("          - " + fid + "\n", self.workflow)
                result = subprocess.run([bash, "-s"], input=selector,
                    env={**os.environ, "ROOT_DIR": "/reviewed", "SAMPLE_ID": fid,
                         "TARGET_SOURCE_RELATIVE": "../../private.cherri", "HTTP_PROBE_ENABLED": "true"},
                    capture_output=True, text=True, timeout=10)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(result.stdout.splitlines(), [fid,
                    "/reviewed/tools/shortcuts-simulator-qa/fixtures/" + fid + ".cherri",
                    fid, labels.expected_summary(fid), "false"])
        for sample, code in (("qa-debug-label-plain-v2", 0), ("qa-debug-label-plain-legacy", 0),
                             ("qa-debug-label-private", 2), ("qa-debug-label-plain-v2;exit 0", 2),
                             ("qa-debug-label-six-modes", 2)):
            result = subprocess.run([bash, "-c", validation, "validation", "runtime",
                "ipad-default", sample, "false", "1", "true"], capture_output=True, text=True, timeout=10)
            self.assertEqual(result.returncode, code, result.stderr)

    def test_actual_phase_zero_wires_all_three_causal_gates_before_signing(self):
        marker = 'python3 - "$ROOT_DIR" "$ARTIFACT_DIR" "$SOURCE_SHA"'
        start = self.shell.index("<<'PY'\n", self.shell.index(marker)) + len("<<'PY'\n")
        program = self.shell[start:self.shell.index("\nPY\n", start)]
        tree = ast.parse(program)
        node = next(n for n in tree.body if isinstance(n, ast.Assign)
                    and any(isinstance(t, ast.Name) and t.id == "fixture_specs" for t in n.targets))
        rows = ast.literal_eval(node.value)
        calls = {n.func.id for n in ast.walk(tree) if isinstance(n, ast.Call) and isinstance(n.func, ast.Name)}
        self.assertTrue({"verify_label_source", "verify_label_graph", "verify_label_mutations"} <= calls)
        for fid in labels.SOURCE_HASHES:
            dialect = "legacy" if fid.endswith("-legacy") else "v2"
            self.assertEqual([r for r in rows if r[0] == fid], [(fid, fid + ".cherri", dialect)])
        matrix = re.search(r"inputs.sample_id == 'qa-debug-label-six-modes' && '([^']+)'", self.workflow)
        self.assertIsNotNone(matrix)
        self.assertEqual(set(json.loads(matrix[1])), set(labels.SOURCE_HASHES))
        self.assertIn("max-parallel: 1", self.workflow)
        self.assertIn("fail-fast: false", self.workflow)

    def test_exact_results_for_both_frontends_and_each_mode_require_fresh_trace(self):
        for fid in labels.SOURCE_HASHES:
            with self.subTest(fixture=fid):
                event, expected = self.event(fid), labels.expected_summary(fid)
                actual = self.oracle.verify([event], fid, expected)
                self.assertEqual((actual["runGate"], actual["traceGate"]), ("PASS", "PASS"))
                self.assertEqual(self.oracle.verify([{**event, "tracePayload": ""}], fid, expected)["traceGate"], "FAIL_RUNTIME")
                self.assertEqual(self.oracle.verify([{**event, "runId": "stale"}], fid, expected)["runGate"], "FAIL_RUNTIME")

    def test_changed_value_type_cardinality_name_or_clear_is_never_normalized(self):
        fid = "qa-debug-label-dated-typed-v2"
        expected, event = labels.expected_summary(fid), self.event(fid)
        for key, _, correct in labels.FIELDS:
            with self.subTest(field=key):
                wrong = re.sub(r"(:" + re.escape(key) + r"=)[^:]*", r"\g<1>WRONG", expected)
                self.assertNotEqual(wrong, expected)
                result = self.oracle.verify([{**event, "resultSummary": wrong}], fid, expected)
                self.assertEqual(result["runGate"], "FAIL_RUNTIME")
        self.assertEqual(self.oracle.verify([event, event], fid, expected)["runGate"], "FAIL_RUNTIME")
        for change in ({"status": "ERROR"}, {"status": "CANCELLED"}, {"receiverSelfTest": True},
                       {"fixtureId": "qa-debug-label-typed-legacy"}):
            self.assertEqual(self.oracle.verify([{**event, **change}], fid, expected)["runGate"], "FAIL_RUNTIME")


if __name__ == "__main__":
    unittest.main()
