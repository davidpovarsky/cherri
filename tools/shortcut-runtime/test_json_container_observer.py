"""Source and process-gate controls, never synthesized native evidence."""
import json
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch

import observe_literal_object_character as observer


class JSONContainerObserverTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        real = Path(__file__).resolve().parents[2]
        self.original = {}
        for relative in observer.JSON_INPUT_PINS:
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

    def test_exact_pinned_original_closure(self):
        inputs = observer.checked_input_files(self.root, "a" * 40, "json-container-null", self.git)
        self.assertEqual(inputs, self.original)
        self.assertEqual(len(inputs), 3)
        body = json.loads(inputs[observer.JSON_BODY])
        rows = body["Value"]["WFDictionaryFieldValueItems"]
        empty = next(row for row in rows if row["WFKey"]["Value"]["string"] == "empty_val")
        self.assertEqual(empty["WFItemType"], 0)
        self.assertEqual(empty["WFValue"]["Value"]["string"], "")
        controls = json.loads(inputs[observer.S01_CONTROLS])
        self.assertEqual(len(controls["controls"]), 7)
        self.assertEqual(sum(len(case["classes"]) for case in controls["controls"]), 19)
        self.assertEqual(controls["sourceSha256"], observer.digest(inputs[observer.SCRIPT]))

    def test_each_changed_authored_input_rejected(self):
        for relative, original in self.original.items():
            path = self.root / relative
            path.write_bytes(original + b"\n")
            with self.subTest(path=relative), self.assertRaisesRegex(SystemExit, "Pinned authored"):
                observer.checked_input_files(self.root, "a" * 40, "json-container-null", self.git)
            path.write_bytes(original)

    def test_matching_pin_with_mismatched_git_blob_rejected(self):
        with self.assertRaisesRegex(SystemExit, "Pinned authored"):
            observer.checked_input_files(self.root, "a" * 40, "json-container-null", lambda *_: b"different source")

    def test_original_literal_mode_does_not_consume_new_inputs(self):
        def forbidden_read(*_):
            self.fail("original literal mode unexpectedly read JSON inputs")
        self.assertEqual(observer.checked_input_files(self.root, "a" * 40, "literal-object-character", forbidden_read), {})

    def test_stale_or_file_output_is_rejected_without_overwrite(self):
        directory = self.root / "output"
        observer.prepare_output(directory)
        stale = directory / "observations-original.json"
        stale.write_bytes(b"SYNTHETIC_STALE_CONTROL")
        with self.assertRaisesRegex(SystemExit, "fresh empty"):
            observer.prepare_output(directory)
        self.assertEqual(stale.read_bytes(), b"SYNTHETIC_STALE_CONTROL")
        with self.assertRaisesRegex(SystemExit, "fresh empty"):
            observer.prepare_output(stale)

    def test_non_macos_refuses_before_process_or_output(self):
        output = self.root / "never-created"
        args = ["observer", "--prelude", "not-read", "--output", str(output),
                "--source-sha", "a" * 40, "--run-id", "synthetic-control-only",
                "--observation-kind", "json-container-null"]
        with patch.object(sys, "argv", args), patch.object(observer.platform, "system", return_value="Linux"), \
                patch.object(observer.subprocess, "run") as run:
            with self.assertRaisesRegex(SystemExit, "Actual macOS is required"):
                observer.main()
            run.assert_not_called()
        self.assertFalse(output.exists())


if __name__ == "__main__":
    unittest.main()
