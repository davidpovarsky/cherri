import io
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch
import zipfile
import collect_editor_contract_evidence as capture

class PublicEditorCaptureTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.base = Path(self.temp.name)
        self.root = self.base / "public"
        self.root.mkdir()
        self.output = self.base / "capture"

    def file(self, name, data=b"public probe bytes"):
        path = self.root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(data)
        return path

    def collect(self):
        return capture.collect(self.root, self.output, "a" * 40, "1234")

    def test_exact_public_bytes_and_no_acceptance(self):
        name = "editor-contract-e05-12345/swift/receipt.json"
        self.file(name)
        report = self.collect()
        self.assertEqual(report["status"], "CAPTURED")
        self.assertFalse(report["ui_executed"])
        self.assertEqual(report["formal_requirements_proved"], [])
        with zipfile.ZipFile(self.output / "public-editor-evidence.zip") as archive:
            self.assertEqual(archive.namelist(), [name])
            self.assertEqual(archive.read(name), b"public probe bytes")

    def test_exact_windows_swift_sidecars_retained(self):
        names = []
        for ident, stem in (("e05", "editor-publication-probe"), ("e06", "position-probe")):
            for suffix in (".exp", ".lib"):
                name = f"canonical-swift-paths/editor-contract-{ident}-12345/swift/{stem}{suffix}"
                self.file(name, name.encode())
                names.append(name)
        report = self.collect()
        self.assertEqual([item["path"] for item in report["files"]], sorted(names))
        with zipfile.ZipFile(self.output / "public-editor-evidence.zip") as archive:
            for name in names:
                self.assertEqual(archive.read(name), name.encode())

    def test_other_sidecar_names_and_extensions_rejected(self):
        for ident, name in (("e05", "position-probe.lib"), ("e06", "editor-publication-probe.exp"),
                            ("e05", "private.lib"), ("e06", "position-probe.pdb")):
            with self.subTest(ident=ident, name=name):
                self.assertFalse(capture.allowed(Path(f"editor-contract-{ident}-12345/swift/{name}")))

    def test_foreign_path_rejected_before_publication(self):
        self.file("private-corpus.json")
        with self.assertRaisesRegex(ValueError, "foreign"):
            self.collect()
        self.assertFalse(self.output.exists())

    def test_fixed_size_budget_rejects_before_publication(self):
        self.file("editor-contract-e05-12345/swift/receipt.json", b"12345")
        with patch.object(capture, "MAX_FILE_BYTES", 4):
            with self.assertRaisesRegex(ValueError, "fixed capture budget"):
                self.collect()
        self.assertFalse(self.output.exists())

    def test_growing_file_uses_bounded_read_and_is_rejected(self):
        target = self.file("editor-contract-e05-12345/swift/receipt.json", b"1234")
        calls = []
        class GrowingFile(io.BytesIO):
            def read(self, size=-1):
                calls.append(size)
                return super().read(size)
        original_open = Path.open
        def growing_open(path, *args, **kwargs):
            if path == target:
                return GrowingFile(b"123456789")
            return original_open(path, *args, **kwargs)
        with patch.object(capture, "MAX_FILE_BYTES", 4), patch.object(Path, "open", growing_open):
            with self.assertRaisesRegex(ValueError, "changed during capture"):
                self.collect()
        self.assertEqual(calls, [5])
        self.assertFalse(self.output.exists())

    def test_linked_directory_rejected_without_traversal(self):
        target = self.base / "other"
        target.mkdir()
        alias = self.root / "editor-contract-e05-12345"
        if os.name == "nt":
            subprocess.run(["cmd", "/d", "/c", "mklink", "/J", str(alias), str(target)], check=True, capture_output=True)
        else:
            alias.symlink_to(target, target_is_directory=True)
        self.addCleanup(lambda: alias.rmdir() if os.name == "nt" else alias.unlink())
        with self.assertRaisesRegex(ValueError, "linked evidence directory"):
            self.collect()
        self.assertFalse(self.output.exists())

    def test_absent_execution_remains_not_run(self):
        self.root.rmdir()
        report = self.collect()
        self.assertEqual(report["status"], "NOT_RUN")
        self.assertEqual(report["files"], [])
        self.assertEqual(report["formal_requirements_proved"], [])

if __name__ == "__main__":
    unittest.main()
