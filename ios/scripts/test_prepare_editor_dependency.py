import copy
import hashlib
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import prepare_editor_dependency as subject


class EditorDependencyPatchTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.revision = "a" * 40
        self.manifest = {"schemaVersion": 1, "patchId": "synthetic-test", "revision": self.revision, "files": []}
        for index in range(2):
            source = f"Synthetic source {index}\nold marker\n".encode()
            result = source.replace(b"old marker", b"new marker")
            relative = f"Sources/CodeEditorView/File{index}.swift"
            path = self.root / relative
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(source)
            self.manifest["files"].append({
                "path": relative, "originalGitBlob": str(index) * 40,
                "originalSha256": hashlib.sha256(source).hexdigest(),
                "patchedSha256": hashlib.sha256(result).hexdigest(),
                "replacements": [{"old": "old marker", "new": "new marker"}],
            })
        self.git = patch.object(subject, "git_value", side_effect=self.git_value)
        self.git.start()
        self.addCleanup(self.git.stop)

    def git_value(self, checkout, *args):
        if args == ("rev-parse", "HEAD"):
            return self.revision
        relative = args[1].split(":", 1)[1]
        return next(entry["originalGitBlob"] for entry in self.manifest["files"] if entry["path"] == relative)

    def contents(self):
        return [(self.root / entry["path"]).read_bytes() for entry in self.manifest["files"]]

    def test_applies_exact_bytes_and_is_idempotent(self):
        receipt = subject.apply_patch(self.root, self.manifest)
        self.assertEqual([item["status"] for item in receipt["files"]], ["APPLIED", "APPLIED"])
        expected = self.contents()
        repeated = subject.apply_patch(self.root, self.manifest)
        self.assertEqual([item["status"] for item in repeated["files"]], ["ALREADY_APPLIED"] * 2)
        self.assertEqual(self.contents(), expected)
        self.assertEqual(subject.apply_patch(self.root, self.manifest, check_only=True)["status"], "PASS")

    def test_unknown_revision_fails_before_mutation(self):
        original = self.contents()
        with patch.object(subject, "git_value", return_value="b" * 40):
            with self.assertRaisesRegex(subject.PatchError, "revision mismatch"):
                subject.apply_patch(self.root, self.manifest)
        self.assertEqual(self.contents(), original)

    def test_unknown_second_file_refuses_all_changes(self):
        path = self.root / self.manifest["files"][1]["path"]
        path.write_bytes(b"unexpected local modification")
        original = self.contents()
        with self.assertRaisesRegex(subject.PatchError, "Unrecognized"):
            subject.apply_patch(self.root, self.manifest)
        self.assertEqual(self.contents(), original)

    def test_tampered_git_blob_is_rejected(self):
        with patch.object(subject, "git_value", side_effect=[self.revision, "f" * 40]):
            with self.assertRaisesRegex(subject.PatchError, "Git blob"):
                subject.apply_patch(self.root, self.manifest)

    def test_changed_patch_output_is_rejected(self):
        altered = copy.deepcopy(self.manifest)
        altered["files"][1]["replacements"][0]["new"] = "different output"
        original = self.contents()
        with self.assertRaisesRegex(subject.PatchError, "Patched source hash"):
            subject.apply_patch(self.root, altered)
        self.assertEqual(self.contents(), original)

    def test_changed_anchor_is_rejected(self):
        altered = copy.deepcopy(self.manifest)
        altered["files"][1]["replacements"][0]["old"] = "missing anchor"
        with self.assertRaisesRegex(subject.PatchError, "anchor"):
            subject.apply_patch(self.root, altered)

    def test_partial_prior_application_resumes_only_exact_bytes(self):
        path = self.root / self.manifest["files"][0]["path"]
        path.write_bytes(path.read_bytes().replace(b"old marker", b"new marker"))
        receipt = subject.apply_patch(self.root, self.manifest)
        self.assertEqual([item["status"] for item in receipt["files"]], ["ALREADY_APPLIED", "APPLIED"])

    def test_check_only_cannot_claim_unpatched_success(self):
        original = self.contents()
        with self.assertRaisesRegex(subject.PatchError, "not been applied"):
            subject.apply_patch(self.root, self.manifest, check_only=True)
        self.assertEqual(self.contents(), original)

    def test_path_traversal_is_rejected(self):
        altered = copy.deepcopy(self.manifest)
        altered["files"][0]["path"] = "Sources/CodeEditorView/../../outside.swift"
        with self.assertRaisesRegex(subject.PatchError, "Unsafe"):
            subject.apply_patch(self.root, altered)


if __name__ == "__main__":
    unittest.main()
