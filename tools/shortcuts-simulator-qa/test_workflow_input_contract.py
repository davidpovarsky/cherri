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
