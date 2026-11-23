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


    def test_tuple_mode_preserves_failure_bytes_without_reading_build_products(self):
        originals = {
            "editor-contract-e05-12345/e05-swift-process.json":
                b'{"exit_code":1,"timed_out":false,"source_files":[]}\n',
            "editor-contract-e05-12345/e05-swift-go.jsonl":
                b'{"passed":false,"compiled":false,"summary":null}\n',
            "editor-contract-e05-12345/e05-swift-stderr.log": b"",
            "editor-contract-e05-12345/swift/receipt.json":
                b'{"passed":false,"compile":{"exit_code":1},"execution":null}\n',
            "editor-contract-e05-12345/swift/compile.stdout": b"",
            "editor-contract-e05-12345/swift/compile.stderr":
                b"public synthetic compiler diagnostic\n",
        }
        for name, data in originals.items():
            self.file(name, data)
        products = []
        for prefix, stem, generated in (
            ("editor-contract-e05-12345", "editor-publication-probe",
             "editor-publication-generated.swift"),
            ("canonical-swift-paths/editor-contract-e06-67890", "position-probe",
             "position-generated.swift"),
        ):
            for name in (generated, stem, stem + ".exe", stem + ".exp", stem + ".lib"):
                products.append(self.file(prefix + "/swift/" + name, b"public build product"))
        original_open = Path.open
        def reject_product_read(path, *args, **kwargs):
            if path in products:
                raise AssertionError("excluded public build product was opened")
            return original_open(path, *args, **kwargs)
        with patch.object(Path, "open", reject_product_read):
            report = capture.collect(self.root, self.output, "a" * 40, "1234", tuples_only=True)
        self.assertEqual(report["capture_mode"], "diagnostic_tuples_only")
        self.assertEqual(report["status"], "CAPTURED")
        self.assertFalse(report["ui_executed"])
        self.assertEqual(report["formal_requirements_proved"], [])
        self.assertEqual(report["total_bytes"], sum(map(len, originals.values())))
        self.assertEqual([item["path"] for item in report["files"]], sorted(originals))
        self.assertEqual([item["path"] for item in report["omitted_public_build_products"]],
                         sorted(path.relative_to(self.root).as_posix() for path in products))
        with zipfile.ZipFile(self.output / "public-editor-evidence.zip") as archive:
            self.assertEqual(archive.namelist(), sorted(originals))
            for name, data in originals.items():
                self.assertEqual(archive.read(name), data)
        for name, data in originals.items():
            self.assertEqual((self.root / name).read_bytes(), data)
        for path in products:
            self.assertEqual(path.read_bytes(), b"public build product")

    def test_tuple_mode_rejects_foreign_source_or_executable(self):
        for name in ("editor-contract-e05-12345/swift/foreign.swift",
                     "editor-contract-e05-12345/swift/foreign-probe",
                     "private-corpus.json"):
            with self.subTest(name=name):
                target = self.file(name, b"synthetic rejected input")
                with self.assertRaisesRegex(ValueError, "foreign"):
                    capture.collect(self.root, self.output, "a" * 40, "1234", tuples_only=True)
                self.assertFalse(self.output.exists())
                target.unlink()

    def test_tuple_mode_omitted_products_do_not_bypass_original_caps(self):
        self.file("editor-contract-e05-12345/swift/editor-publication-probe", b"12345")
        self.file("editor-contract-e05-12345/swift/editor-publication-generated.swift", b"12345")
        for limit, value in (("MAX_FILES", 1), ("MAX_FILE_BYTES", 4), ("MAX_TOTAL_BYTES", 9)):
            with self.subTest(limit=limit), patch.object(capture, limit, value):
                with self.assertRaisesRegex(ValueError, "fixed capture budget"):
                    capture.collect(self.root, self.output, "a" * 40, "1234", tuples_only=True)
                self.assertFalse(self.output.exists())

    def test_tuple_mode_build_products_alone_do_not_claim_observation(self):
        self.file("editor-contract-e05-12345/swift/editor-publication-probe")
        report = capture.collect(self.root, self.output, "a" * 40, "1234", tuples_only=True)
        self.assertEqual(report["status"], "NOT_RUN")
        self.assertEqual(report["files"], [])
        self.assertEqual(report["total_bytes"], 0)
        self.assertFalse(report["ui_executed"])
        self.assertEqual(report["formal_requirements_proved"], [])
        with zipfile.ZipFile(self.output / "public-editor-evidence.zip") as archive:
            self.assertEqual(archive.namelist(), [])

if __name__ == "__main__":
    unittest.main()
