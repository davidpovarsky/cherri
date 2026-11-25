"""Offline collector-to-workflow artifact transport controls, never native proof."""
import json
import os
from pathlib import Path
import re
import tempfile
import unittest
from unittest import mock

import collect_ios_test_evidence as collector
import joined_editor_evidence as joined
import preview_native_evidence as preview
import test_joined_editor_evidence as joined_controls
import test_preview_native_evidence as preview_controls

ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = ROOT / ".github/workflows/ios-build.yml"
PREFIX = "ios/.build/test-evidence/"
ORIGINAL_FILES = {
    "xcodetest.log", "ios-test-evidence.json", "ios-test-diagnostics.json",
    "documentation-index.json", "documentation-compatibility-manifest.json", "documentation-receipt.json",
}
NEW_FILES = {
    "joined-editor-contract.json", "joined-editor-proof.json",
    "preview-document-contract.json", "preview-native-proof.json",
}


def declared_uploads(text):
    blocks = re.findall(r"(?ms)^      - name: Upload iOS test evidence\n(.*?)(?=^      - name:|\Z)", text)
    if len(blocks) != 1:
        raise ValueError("UPLOAD_STEP_IDENTITY")
    block = blocks[0]
    if "uses: actions/upload-artifact@v4" not in block or "name: ios-build-evidence" not in block:
        raise ValueError("UPLOAD_ARTIFACT_IDENTITY")
    paths = re.findall(r"(?m)^            (\S+)\s*$", block)
    if not paths or len(paths) != len(set(paths)):
        raise ValueError("UPLOAD_PATH_INVENTORY")
    names = set()
    for path in paths:
        if not path.startswith(PREFIX) or Path(path).name not in ORIGINAL_FILES | NEW_FILES:
            raise ValueError("UPLOAD_PATH_SCOPE")
        names.add(path[len(PREFIX):])
    return names


class IOSEvidenceUploadContractTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.temporary = tempfile.TemporaryDirectory(prefix="ios-upload-synthetic-control-")
        cls.addClassCleanup(cls.temporary.cleanup)
        root = Path(cls.temporary.name).resolve(strict=True)
        log, exitcode, output = root / "raw.log", root / "exitcode", root / "output"
        log.write_text(joined_controls.model_log() + preview_controls.model_log())
        exitcode.write_text("0")
        with mock.patch.object(joined, "source_pins", return_value=joined_controls.model_pins()), \
                mock.patch.object(preview, "source_pins", return_value=preview_controls.synthetic_pins()), \
                mock.patch.dict(os.environ, {"GITHUB_RUN_ATTEMPT": "1"}):
            cls.diagnostics, cls.records = collector.collect(log, output, "a" * 40, "1234567", exitcode)
        cls.output = output
        cls.workflow = WORKFLOW.read_text()

    def required_artifacts(self):
        self.assertEqual(len(self.records), 6)
        self.assertEqual(self.diagnostics["joinedEditorEvidence"]["status"], "VERIFIED_JOINED_NATIVE")
        self.assertEqual(self.diagnostics["previewNativeEvidence"]["status"], "VERIFIED_NATIVE_PREVIEW")
        result = set()
        for record in self.records:
            for artifact in record["artifacts"]:
                name = Path(artifact["path"]).name
                payload = (self.output / name).read_bytes()
                self.assertEqual(joined.digest(payload), artifact["sha256"])
                result.add(name)
        self.assertTrue(NEW_FILES.issubset(result))
        return result

    def test_actual_collector_records_have_every_original_in_explicit_upload(self):
        declared = declared_uploads(self.workflow)
        required = self.required_artifacts()
        self.assertEqual(required - declared, set())
        self.assertEqual(declared, ORIGINAL_FILES | NEW_FILES)

    def test_each_missing_new_original_is_detected_from_actual_collector_records(self):
        required = self.required_artifacts()
        complete = declared_uploads(self.workflow) | required
        for missing in sorted(NEW_FILES):
            with self.subTest(missing=missing):
                declared = complete - {missing}
                self.assertEqual(required - declared, {missing})

    def test_existing_six_paths_and_raw_quarantine_are_preserved(self):
        declared = declared_uploads(self.workflow)
        self.assertTrue(ORIGINAL_FILES.issubset(declared))
        original = PREFIX + "xcodetest.log"
        self.assertEqual(self.workflow.count("            " + original + "\n"), 1)
        for replacement in (PREFIX + "*", PREFIX + "tests.xcresult", "../private/raw.log"):
            with self.subTest(path=replacement), self.assertRaises(ValueError):
                declared_uploads(self.workflow.replace(original, replacement, 1))


if __name__ == "__main__":
    unittest.main()
