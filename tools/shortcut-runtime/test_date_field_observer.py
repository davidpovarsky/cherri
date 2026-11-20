"""Actual source-gate controls for the Date observer; no native evidence."""
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch

import observe_literal_object_character as observer


class DateObserverSourceTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        real = Path(__file__).resolve().parents[2]
        self.original = {}
        for relative in observer.DATE_INPUT_PINS:
            data = (real / relative).read_bytes()
            target = self.root / relative
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes(data)
            self.original[relative] = data

    def git(self, verb, value):
        self.assertEqual(verb, "show")
        sha, path = value.split(":", 1)
        self.assertEqual(sha, "a" * 40)
        return self.original[path]

    def test_exact_two_input_pins(self):
        got = observer.checked_input_files(self.root, "a" * 40, "date-field-parameters", self.git)
        self.assertEqual(got, self.original)
        self.assertEqual(len(got), 2)

    def test_changed_input_and_unchanged_expected_pin_are_rejected(self):
        for relative, data in self.original.items():
            target = self.root / relative
            target.write_bytes(data + b"\n")
            with self.subTest(path=relative), self.assertRaisesRegex(SystemExit, "Pinned authored"):
                observer.checked_input_files(self.root, "a" * 40, "date-field-parameters", self.git)
            target.write_bytes(data)

    def test_expected_pin_but_wrong_git_object_is_rejected(self):
        with self.assertRaisesRegex(SystemExit, "Pinned authored"):
            observer.checked_input_files(self.root, "a" * 40, "date-field-parameters", lambda *_: b"different")

    def test_non_macos_cannot_emit_date_evidence(self):
        output = self.root / "never-created"
        args = ["observer", "--prelude", "not-read", "--output", str(output),
                "--source-sha", "a" * 40, "--run-id", "mock-control-only",
                "--observation-kind", "date-field-parameters"]
        with patch.object(sys, "argv", args), patch.object(observer.platform, "system", return_value="Linux"), \
                patch.object(observer.subprocess, "run") as run:
            with self.assertRaisesRegex(SystemExit, "Actual macOS is required"):
                observer.main()
            run.assert_not_called()
        self.assertFalse(output.exists())


if __name__ == "__main__":
    unittest.main()
