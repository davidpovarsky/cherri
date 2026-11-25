import copy
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
SPEC = importlib.util.spec_from_file_location("array_probe", ROOT / "observe_function_dictionary_array.py")
PROBE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(PROBE)


class FunctionDictionaryArrayProbeControls(unittest.TestCase):
    def inputs(self):
        return json.loads((ROOT / "fixtures/function-dictionary-array-probe.json").read_text())

    def test_exact_forms_keep_empty_absence_and_wrapped_plain_separate(self):
        cases = PROBE.validate_fixed_inputs(self.inputs())
        self.assertEqual(7, len(cases))
        by_id = {item["id"]: item["arrayValue"] for item in cases}
        self.assertNotIn("Value", by_id["empty_wrapped_omitted"])
        self.assertEqual([], by_id["empty_wrapped_explicit"]["Value"])
        self.assertEqual([], by_id["empty_plain"])
        for name in ("text", "attachment"):
            self.assertEqual(by_id[name + "_plain"], by_id[name + "_wrapped"]["Value"])
            self.assertEqual(1, len(by_id[name + "_plain"]))
        self.assertEqual("", by_id["text_plain"][0]["WFValue"]["Value"]["string"])
        self.assertEqual("ActionOutput", by_id["attachment_plain"][0]["WFValue"]["Value"]["Type"])

    def test_inventory_rejects_omission_reorder_duplicate_extra_and_authority(self):
        mutations = {
            "schema": lambda d: d.update(schema="other"),
            "missing": lambda d: d["cases"].pop(),
            "reorder": lambda d: d["cases"].reverse(),
            "duplicate": lambda d: d["cases"].append(copy.deepcopy(d["cases"][0])),
            "extra_route": lambda d: d.update(command=["anything"]),
            "authority": lambda d: d.update(nativeAcceptanceGranted=True),
            "case_route": lambda d: d["cases"][0].update(command="anything"),
        }
        for name, mutate in mutations.items():
            with self.subTest(name=name):
                value = self.inputs()
                mutate(value)
                with self.assertRaisesRegex(ValueError, "FIXED_CASE_INVENTORY"):
                    PROBE.validate_fixed_inputs(value)

    def test_command_is_fixed_no_shell_and_unknown_case_rejected(self):
        command = PROBE.probe_command("combined.js", "case-output", "inputs.json", "empty_plain")
        self.assertEqual(["/usr/bin/osascript", "-l", "JavaScript", "combined.js", "case-output", "inputs.json", "empty_plain"], command)
        with self.assertRaisesRegex(ValueError, "FIXED_CASE_IDENTITY"):
            PROBE.probe_command("combined.js", "case-output", "inputs.json", "arbitrary")

    def test_original_writer_rejects_collision_and_preserves_first_bytes(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "original.json"
            PROBE.write_new(path, {"first": True})
            original = path.read_bytes()
            with self.assertRaises(FileExistsError):
                PROBE.write_new(path, {"second": True})
            self.assertEqual(original, path.read_bytes())

    def test_helper_closure_includes_transitive_local_imports(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            helpers = root / "tools/shortcuts-simulator-qa"
            helpers.mkdir(parents=True)
            (helpers / "project_prerequisite_observation.py").write_text("import sibling\n")
            (helpers / "sibling.py").write_text("from deepest import value\n")
            (helpers / "deepest.py").write_text("value = 1\n")
            self.assertEqual(["tools/shortcuts-simulator-qa/" + name for name in
                              ["deepest.py", "project_prerequisite_observation.py", "sibling.py"]], PROBE.helper_closure(root))

    def test_helper_closure_rejects_symlink(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            helpers = root / "tools/shortcuts-simulator-qa"
            helpers.mkdir(parents=True)
            (helpers / "project_prerequisite_observation.py").write_text("import redirected\n")
            target = root / "target.py"
            target.write_text("value = 1\n")
            (helpers / "redirected.py").symlink_to(target)
            with self.assertRaisesRegex(ValueError, "HELPER_SOURCE_NOT_REGULAR"):
                PROBE.helper_closure(root)


if __name__ == "__main__":
    unittest.main()
