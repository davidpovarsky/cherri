"""Run the actual validation prefix and dispatch shell body with inert fixtures."""
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import textwrap
import unittest


def bash_path():
    if os.name == "nt":
        git_bash = Path(os.environ.get("ProgramFiles", "C:/Program Files")) / "Git/bin/bash.exe"
        if git_bash.is_file():
            return str(git_bash)
    return shutil.which("bash")


class WorkflowInputContractTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.bash = bash_path()
        if not cls.bash:
            raise unittest.SkipTest("bash unavailable; CLI input validation was not executed")
        cls.root = Path(__file__).resolve().parents[2]
        script = (cls.root / "scripts/ios27_shortcuts_simulator_qa.sh").read_text(encoding="utf-8")
        cls.validation = script[:script.index('ROOT_DIR="')]
        workflow = (cls.root / ".github/workflows/cherri-ios27-shortcuts-simulator-qa.yml").read_text(encoding="utf-8")
        step = workflow.split("      - name: Run iPadOS 27 Simulator Shortcuts QA Orchestrator\n", 1)[1]
        cls.step = step.split("      - name: ", 1)[0]
        cls.dispatch_body = textwrap.dedent(cls.step.split("        run: |\n", 1)[1])

    def validate(self, args):
        return subprocess.run([self.bash, "-c", self.validation, "qa-validation", *args],
                              capture_output=True, text=True, timeout=10)

    def test_all_page_bounds_and_false_boolean(self):
        for pages in map(str, range(1, 21)):
            result = self.validate(["full", "ipad-default", "qa-functional-legacy", "false", pages, "true"])
            self.assertEqual(result.returncode, 0, result.stderr)

    def test_only_explicit_macro_fixture_choices_are_accepted(self):
        for fixture in ("qa-require", "qa-measure", "qa-measure-release"):
            self.assertEqual(self.validate(["full", "ipad-default", fixture, "true", "20", "true"]).returncode, 0)
        for fixture in ("qa-unit-release", "qa-functional-release", "qa-measure-release;exit 0"):
            self.assertEqual(self.validate(["full", "ipad-default", fixture, "true", "20", "true"]).returncode, 2)

    def test_self_contained_short_circuit_is_a_separate_explicit_target(self):
        self.assertEqual(self.validate(["full", "ipad-default", "qa-short-circuit-local", "true", "20", "true"]).returncode, 0)
        for fixture in ("qa-short-circuit-local-release", "qa-short-circuit-local;exit 0"):
            self.assertEqual(self.validate(["full", "ipad-default", fixture, "true", "20", "true"]).returncode, 2)
        script = (self.root / "scripts/ios27_shortcuts_simulator_qa.sh").read_text(encoding="utf-8")
        selector = script.split("# Select primary target fixture from SAMPLE_ID\n", 1)[1].split('TARGET_SOURCE_FILE=', 1)[0]
        selector += '\nprintf "%s\\n" "$TARGET_FIXTURE_ID" "$EXPECTED_SEMANTIC_OUTPUT"\n'
        result = subprocess.run([self.bash, "-c", selector], env={**os.environ, "SAMPLE_ID": "qa-short-circuit-local"},
                                capture_output=True, text=True, timeout=10)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout.splitlines(), ["qa-short-circuit-local-v2", "CHERRI_QA_F01_LOCAL_RESULT:P15_F01_LOCAL_OK:calls=1"])

    def test_f01_diagnostics_bind_four_exact_independent_targets(self):
        cases = {
            "qa-f01-diag-counter": "CHERRI_QA_F01_DIAG_COUNTER:calls=1",
            "qa-f01-diag-predicate": "CHERRI_QA_F01_DIAG_PREDICATE:P15_F01_PREDICATE_OK",
            "qa-f01-diag-control": "CHERRI_QA_F01_DIAG_CONTROL:P15_F01_CONTROL_OK:calls=1",
            "qa-f01-diag-logic": "CHERRI_QA_F01_DIAG_LOGIC:P15_F01_LOGIC_OK"
        }
        script = (self.root / "scripts/ios27_shortcuts_simulator_qa.sh").read_text(encoding="utf-8")
        selector = script.split("# Select primary target fixture from SAMPLE_ID\n", 1)[1].split("LEGACY_COUNTERPART_SOURCE=", 1)[0]
        selector += '\nprintf "%s\\n" "$TARGET_FIXTURE_ID" "$TARGET_SOURCE_FILE" "$EXPECTED_SHORTCUT_NAME" "$EXPECTED_SEMANTIC_OUTPUT" "$LEGACY_COUNTERPART_ID"\n'
        workflow = (self.root / ".github/workflows/cherri-ios27-shortcuts-simulator-qa.yml").read_text(encoding="utf-8")
        for sample, expected in cases.items():
            with self.subTest(sample=sample):
                self.assertEqual(self.validate(["runtime", "ipad-default", sample, "false", "1", "true"]).returncode, 0)
                self.assertIn("          - " + sample + "\n", workflow)
                result = subprocess.run([self.bash, "-c", selector],
                    env={**os.environ, "SAMPLE_ID": sample, "ROOT_DIR": "/reviewed",
                         "TARGET_SOURCE_RELATIVE": "../../private.cherri"},
                    capture_output=True, text=True, timeout=10)
                self.assertEqual(result.returncode, 0, result.stderr)
                fid = sample + "-v2"
                self.assertEqual(result.stdout.splitlines(), [fid,
                    "/reviewed/tools/shortcuts-simulator-qa/fixtures/" + fid + ".cherri", fid, expected, "qa-functional-legacy"])
        for sample in ("qa-f01-diag-private", "qa-f01-diag-counter-release", "qa-f01-diag-logic;exit 0"):
            self.assertEqual(self.validate(["runtime", "ipad-default", sample, "false", "1", "true"]).returncode, 2)

    def test_label_export_choices_bind_exact_reviewed_nested_source(self):
        cases = {
            "qa-label-file-carrier": ("p15-label-probes", "CHERRI_QA_LABEL_PROBE_RESULT:P15_LABEL_FILE_CARRIER_OK"),
            "qa-label-raw-empty": ("p15-label-probes", "CHERRI_QA_LABEL_PROBE_RESULT:P15_LABEL_RAW_EMPTY_OK"),
            "qa-label-raw-grouping": ("p15-label-probes", "CHERRI_QA_LABEL_PROBE_RESULT:P15_LABEL_RAW_GROUPING_OK"),
            "qa-export-unicode": ("p15-export-probes", "CHERRI_QA_EXPORT_PROBE_RESULT:P15_EXPORT_UNICODE_OK"),
            "qa-export-empty": ("p15-export-probes", "CHERRI_QA_EXPORT_PROBE_RESULT:P15_EXPORT_EMPTY_OK"),
            "qa-export-thousand": ("p15-export-probes", "CHERRI_QA_EXPORT_PROBE_RESULT:P15_EXPORT_THOUSAND_OK"),
            "qa-export-redacted": ("p15-export-probes", "CHERRI_QA_EXPORT_PROBE_RESULT:P15_EXPORT_REDACTED_OK"),
            "qa-export-unsupported-file": ("p15-export-probes", "CHERRI_QA_EXPORT_PROBE_RESULT:P15_EXPORT_UNSUPPORTED_FILE_OK"),
            "qa-export-text-file": ("p15-export-probes", "CHERRI_QA_EXPORT_PROBE_RESULT:P15_EXPORT_TEXT_FILE_OK"),
        }
        script = (self.root / "scripts/ios27_shortcuts_simulator_qa.sh").read_text(encoding="utf-8")
        selector = script.split("# Select primary target fixture from SAMPLE_ID\n", 1)[1].split('LEGACY_COUNTERPART_SOURCE=', 1)[0]
        selector += '\nprintf "%s\\n" "$TARGET_FIXTURE_ID" "$TARGET_SOURCE_FILE" "$EXPECTED_SHORTCUT_NAME" "$EXPECTED_SEMANTIC_OUTPUT"\n'
        workflow = (self.root / ".github/workflows/cherri-ios27-shortcuts-simulator-qa.yml").read_text(encoding="utf-8")
        options = workflow.split("      sample_id:", 1)[1].split("      capture_full_editor:", 1)[0]
        for sample, (folder, marker) in cases.items():
            with self.subTest(sample=sample):
                self.assertIn("          - " + sample + "\n", options)
                self.assertEqual(self.validate(["full", "ipad-default", sample, "true", "20", "true"]).returncode, 0)
                result = subprocess.run([self.bash, "-c", selector],
                    env={**os.environ, "SAMPLE_ID": sample, "ROOT_DIR": "/reviewed",
                         "TARGET_SOURCE_RELATIVE": "../../private-input.cherri"},
                    capture_output=True, text=True, timeout=10)
                self.assertEqual(result.returncode, 0, result.stderr)
                fid = sample + "-v2"
                self.assertEqual(result.stdout.splitlines(), [fid,
                    "/reviewed/tools/shortcuts-simulator-qa/fixtures/" + folder + "/" + fid + ".cherri", fid, marker])
        # The optional relative source path must not affect existing root fixtures.
        result = subprocess.run([self.bash, "-c", selector],
            env={**os.environ, "SAMPLE_ID": "qa-functional", "ROOT_DIR": "/reviewed",
                 "TARGET_SOURCE_RELATIVE": "../../private-input.cherri"},
            capture_output=True, text=True, timeout=10)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout.splitlines()[1], "/reviewed/tools/shortcuts-simulator-qa/fixtures/qa-functional-v2.cherri")
        for sample in ("qa-label-file-carrier-release", "qa-label-dictionary-carrier", "qa-export-unicode-legacy",
                       "qa-export-private", "qa-export-unicode;exit 0", "../p15-export-probes/qa-export-empty"):
            self.assertEqual(self.validate(["full", "ipad-default", sample, "true", "20", "true"]).returncode, 2)

    def test_out_of_range_noncanonical_and_overflow_are_rejected(self):
        for pages in ("0", "21", "01", "-1", "1.0", " 1", "1 ", "9" * 200, "1\n2", "$(exit 0)"):
            result = self.validate(["full", "ipad-default", "qa-bool-list", "true", pages, "true"])
            self.assertEqual(result.returncode, 2, pages)

    def test_unsupported_modes_and_boolean_values_rejected_before_side_effects(self):
        base = ["full", "ipad-default", "qa-functional", "true", "20", "true"]
        for index, value in ((0, "unbounded"), (1, "physical"), (2, "private-input"),
                             (3, "yes"), (5, "1")):
            args = base.copy()
            args[index] = value
            self.assertEqual(self.validate(args).returncode, 2, (index, value))

    def test_dispatch_values_remain_literal_arguments_and_preserve_false(self):
        self.assertNotIn("${{", self.dispatch_body)
        # toJSON produces the non-empty string "false", which survives Actions'
        # fallback expression. The previous bare boolean false was replaced by true.
        self.assertIn("toJSON(inputs.capture_full_editor)", self.step)
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "scripts").mkdir()
            stub = root / "scripts/ios27_shortcuts_simulator_qa.sh"
            stub.write_text('#!/usr/bin/env bash\nprintf "%s\\n" "$@"\n', encoding="utf-8")
            values = ["full", "ipad-default", "qa-functional", "false", '$(touch dispatch-injected)', "true"]
            names = ["QA_PROBE_LEVEL", "QA_DEVICE_PROFILE", "QA_SAMPLE_ID", "QA_CAPTURE_FULL_EDITOR",
                     "QA_MAX_EDITOR_PAGES", "QA_REQUIRE_RUNTIME"]
            result = subprocess.run([self.bash, "-c", self.dispatch_body], cwd=root,
                                    env={**os.environ, **dict(zip(names, values))},
                                    capture_output=True, text=True, timeout=10)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(result.stdout.splitlines(), values)
            self.assertFalse((root / "dispatch-injected").exists())


if __name__ == "__main__":
    unittest.main()
