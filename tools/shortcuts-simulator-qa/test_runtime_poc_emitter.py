"""Run the real shell heredoc producer on local controls; no Apple execution."""
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

import runtime_poc_formal_evidence as formal
import runtime_poc_coverage as coverage
from test_runtime_poc_formal_evidence import create_control_fixture

REPO = Path(__file__).resolve().parents[2]


class ActualPoCEmitterTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="cherri-actual-emitter-control-")
        self.addCleanup(self.temp.cleanup)
        self.control = create_control_fixture(self.temp.name, REPO)
        self.root = Path(self.control["root"])
        # Exercise a failed coverage observation independently of the old raw17.
        (self.root / coverage.OBSERVED).write_bytes(b"BEFORE_COVERAGE_TEST")
        observed = coverage.observe(self.control["repo"], self.root, self.control["commit"],
                                    self.control["run_id"], self.control["attempt"], allow_deleted=True)
        (self.root / coverage.RESULT).write_bytes(coverage.encoded(observed))
        source = (REPO / "scripts/ios27_runtime_poc.sh").read_text(encoding="utf-8")
        self.assertEqual(source.count("python3 - << PY\n"), 1)
        self.program = source.split("python3 - << PY\n", 1)[1].split("\nPY\n", 1)[0]
        replacements = {
            "${HEAD_SHA}": self.control["commit"], "${GITHUB_RUN_ID:-}": self.control["run_id"],
            "${POC_PASSED}": "false", "${DICT_PASSED}": "false",
            "${MENU_RUNTIME_STATUS}": "FAIL", "${MENU_DIAGNOSTIC}": "SYNTHETIC_LOCAL_CONTROL_NOT_APPLE_EXECUTION",
            "${LEGACY_IF_PASSED}": "false", "${MENU_ALPHA_PASSED}": "false",
            "${MENU_BETA_PASSED}": "false",
        }
        for key, value in replacements.items():
            self.program = self.program.replace(key, value)
        (self.root / "clipboard-result.txt").write_bytes(b"BEFORE_RUNTIME_TEST\n")
        (self.root / "dict-clipboard-result.txt").write_bytes(b"BEFORE_DICT_TEST\n")
        (self.root / "menu-alpha-clipboard-result.txt").write_bytes(b"BEFORE_MENU_ALPHA\n")
        (self.root / "menu-beta-clipboard-result.txt").write_bytes(b"BEFORE_MENU_BETA\n")
        (self.root / "legacy-if-clipboard-result.txt").write_bytes(b"BEFORE_LEGACY_TEST\n")

    def emit(self, program=None):
        result = subprocess.run([sys.executable, "-c", program or self.program],
                                cwd=self.temp.name, capture_output=True, text=True, encoding="utf-8")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        return (self.root / "runtime-diagnostics.json").read_bytes()

    def export(self):
        c = self.control
        return formal.export_records(c["repo"], c["root"], "runtime-diagnostics.json",
                                     c["commit"], c["run_id"], c["attempt"])

    def test_actual_failed_emitter_enters_formal_schema_without_upgrading_result(self):
        raw = self.emit()
        records = json.loads(raw)
        self.assertEqual(len(records), 17)
        self.assertTrue(all(record["result"] == "failed" and record["exit_code"] == 1
                            and record["runtime_pass"] is False for record in records))
        menu = next(record for record in records if record["evidence_id"] == "ev-runtime-menu-structural")
        self.assertIs(menu["assertions"]["menu_start"], True)
        self.assertEqual(menu["assertions"]["cases"], ["Alpha", "Beta"])
        output = self.export()
        self.assertEqual(len(output), 6)
        self.assertEqual({requirement for record in output for requirement in record["requirements"]},
                         {"V01", "V03", "L02", "L04", "N01", "FN01", "FN02", "FN03", "S01", "F01"})
        self.assertTrue(all(record["result"] == "failed" and record["exit_code"] == 1 for record in output))
        self.assertTrue(all(record["runtime_pass"] is False for record in output[:4]))
        self.assertTrue(all(record["assertions"]["exact_output_match"] == "false" for record in output[4:]))
        self.assertTrue(all(isinstance(value, str) for record in output
                            for value in record["assertions"].values()))
        self.assertEqual((self.root / "runtime-diagnostics.json").read_bytes(), raw)

    def test_old_unconditional_menu_success_is_rejected_by_actual_exporter(self):
        target = '"result": "passed" if menu_observed_pass else "failed"'
        self.assertEqual(self.program.count(target), 1)
        self.emit(self.program.replace(target, '"result": "passed"', 1))
        with self.assertRaises(formal.PublicationBlocked):
            self.export()


if __name__ == "__main__":
    unittest.main()
