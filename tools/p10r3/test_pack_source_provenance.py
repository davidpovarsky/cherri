"""Actual Pack reconstruction, source binding, and prospective generation controls."""
import json
from pathlib import Path
import tempfile
import unittest
from unittest import mock
import verify_or_generate_pack as pack

class PackSourceProvenanceTests(unittest.TestCase):
    def test_pinned_modules_and_corrupted_module(self):
        self.assertEqual([], pack.verify_generated_pack())
        errors = pack.verify_generated_pack(mutated_modules={"basic.cherri": "// corrupted module\\n"})
        self.assertTrue(any("basic.cherri" in error and "drift" in error for error in errors), errors)

    def test_manifest_source_is_required_not_inferred(self):
        original = pack.git_show_text
        def missing_source(commit, path):
            text = original(commit, path)
            if commit == pack.PACK_COMMIT and path.endswith("/manifest.json"):
                value = json.loads(text)
                value.pop("definition_source_sha", None)
                return json.dumps(value)
            return text
        with mock.patch.object(pack, "git_show_text", side_effect=missing_source):
            errors = pack.verify_generated_pack()
        self.assertTrue(any("definition_source_sha" in error for error in errors), errors)

    def test_wrong_source_is_not_relabelled_as_same_pack(self):
        original = pack.git_show_text
        current = pack.subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=pack.REPO_ROOT, text=True).strip()
        def wrong_source(commit, path):
            text = original(commit, path)
            if commit == pack.PACK_COMMIT and path.endswith("/manifest.json"):
                value = json.loads(text)
                value["definition_source_sha"] = current
                return json.dumps(value)
            return text
        with mock.patch.object(pack, "git_show_text", side_effect=wrong_source):
            errors = pack.verify_generated_pack()
        self.assertTrue(any("basic.cherri" in error and "drift" in error for error in errors), errors)

    def test_current_generator_keeps_enhanced_return_type(self):
        with tempfile.TemporaryDirectory() as temp:
            output = Path(temp)
            manifest = pack.generate_pack(output)
            current = pack.parse_declarations((output / "modules/basic.cherri").read_text(encoding="utf-8"))
            pinned = pack.parse_declarations(pack.git_show_text(pack.PACK_COMMIT, "extensions/cherri-apple-actions-pack/modules/basic.cherri"))
            self.assertIn("): variable", current[("action", "getType")])
            self.assertIn("): text", pinned[("action", "getType")])
            self.assertNotEqual(current[("action", "getType")], pinned[("action", "getType")])
            self.assertEqual("CURRENT_WORKTREE", manifest["definition_source_scope"])
            self.assertIsNone(manifest["definition_source_sha"])

if __name__ == "__main__":
    unittest.main()
