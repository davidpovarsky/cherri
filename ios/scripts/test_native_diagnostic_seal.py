"""Generic native archive dependency closure; no Apple execution or signing."""
import hashlib
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest
import zipfile


class GenericDiagnosticSealTests(unittest.TestCase):
    def test_generic_archive_import_without_rp39_profile(self):
        scripts = Path(__file__).resolve().parent
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            isolated = root / "ios" / "scripts"
            isolated.mkdir(parents=True)
            for name in ("collect_ios_xcresult_summary.py", "seal_rp39_diagnostic.py"):
                shutil.copyfile(scripts / name, isolated / name)
            self.assertFalse((isolated / "record_workspace_presentation_source.py").exists())
            module, import_error = None, None
            try:
                spec = importlib.util.spec_from_file_location("_generic_seal_control",
                    isolated / "seal_rp39_diagnostic.py")
                module = importlib.util.module_from_spec(spec)
                spec.loader.exec_module(module)
            except Exception as error:
                import_error = type(error).__name__ + ": " + str(error)
            self.assertIsNone(import_error, "generic archive transport must not import RP39-only profile")
            payload = root / "process.original.json"
            raw = b'{"nativeExecutionObserved":false,"original":"retained"}\n'
            payload.write_bytes(raw)
            original = module.file_record(payload)
            archive = root / "originals.zip"
            identity = {"runId": "synthetic-generic-control", "formalAcceptanceGranted": False}
            recorded, manifest = module.make_archive(
                archive, {"process.original.json": {"path": payload, "original": original}},
                identity, manifest_schema="cherri-native-original-diagnostics/1")
            self.assertEqual(recorded["sha256"], hashlib.sha256(archive.read_bytes()).hexdigest())
            self.assertEqual(manifest["schema"], "cherri-native-original-diagnostics/1")
            self.assertEqual(manifest["identity"], identity)
            self.assertEqual(manifest["members"], [{"name": "process.original.json", **original}])
            with zipfile.ZipFile(archive) as result:
                self.assertEqual(set(result.namelist()), {"process.original.json", "manifest.original.json"})
                self.assertEqual(result.read("process.original.json"), raw)
                self.assertEqual(json.loads(result.read("manifest.original.json")), manifest)
            self.assertEqual(module.file_record(payload), original)
            # RP39-specific entry still requires its real profile, fail-closed.
            with self.assertRaises(FileNotFoundError):
                module.source_receipts(root, {})


if __name__ == "__main__":
    unittest.main()
