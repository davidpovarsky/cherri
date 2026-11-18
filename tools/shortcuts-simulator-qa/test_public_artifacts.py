import json
from pathlib import Path
import struct
import tempfile
import unittest
import zlib

from prepare_public_artifacts import PublicationBlocked, sha, stage


def raster_secret(hebrew=False, contrast=255):
    # Tiny fixed bitmap glyphs avoid a runtime font/OCR dependency. No plaintext
    # canary or metadata is embedded in the PNG: the word exists only as pixels.
    glyphs = {
        "S": [31, 16, 16, 31, 1, 1, 31], "E": [31, 16, 16, 30, 16, 16, 31],
        "C": [31, 16, 16, 16, 16, 16, 31], "R": [30, 17, 17, 30, 20, 18, 17],
        "T": [31, 4, 4, 4, 4, 4, 4],
        "ס": [31, 17, 17, 17, 17, 17, 31], "ו": [12, 4, 4, 4, 4, 4, 4],
        "ד": [31, 1, 1, 1, 1, 1, 1],
    }
    word = "דוס" if hebrew else "SECRET"
    width, height = 6 * len(word), 7
    data = bytearray()
    for y in range(height):
        data.append(0)
        for letter in word:
            for x in range(6):
                on = x < 5 and (glyphs[letter][y] & (1 << (4 - x)))
                data.append(255 - contrast if on else 255)
    def chunk(kind, data):
        return struct.pack(">I", len(data)) + kind + data + struct.pack(">I", zlib.crc32(kind + data))
    return b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", struct.pack(">IIBBBBB", width, height, 8, 0, 0, 0, 0)) + chunk(b"IDAT", zlib.compress(bytes(data))) + chunk(b"IEND", b"")


class PublicArtifactTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name) / "raw"
        self.root.mkdir()
        self.output = Path(self.temp.name) / "public"
        self.trusted = {"qa-test.cherri": sha(b"synthetic source")}
        self.write("compiler-manifest.json", {"syntheticOnly": True, "fixtures": [
            {"fixtureId": "qa-test", "fileName": "qa-test.cherri", "sourceSha256": self.trusted["qa-test.cherri"]}]})
        self.write("summary.json", {"privacyGatePassed": True, "gates": {"PHYSICAL_DEVICE_TESTED": "BLOCKED_DEVICE"}})

    def tearDown(self):
        self.temp.cleanup()

    def write(self, name, value):
        path = self.root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(json.dumps(value), encoding="utf-8")

    def publish(self):
        return stage(self.root, self.output, self.trusted)

    def test_positive_synthetic_json_published(self):
        result = self.publish()
        self.assertTrue(result["publicationGatePassed"])
        self.assertTrue((self.output / "summary.json").exists())
        self.assertFalse(result["pixelRedactionClaimed"])

    def test_raster_secret_english_hebrew_low_contrast_quarantined(self):
        for index, (hebrew, contrast) in enumerate(((False, 255), (True, 255), (True, 8))):
            data = raster_secret(hebrew, contrast)
            self.assertNotIn(b"SECRET", data)
            (self.root / f"canary-{index}.png").write_bytes(data)
        result = self.publish()
        self.assertEqual(len(result["quarantined"]), 3)
        self.assertFalse(list(self.output.rglob("*.png")))
        self.assertEqual(len(list(self.root.glob("*.png"))), 3)

    def test_raster_disguised_as_json_and_xcresult_stay_local(self):
        (self.root / "environment.json").write_bytes(raster_secret())
        bundle = self.root / "tests" / "private.xcresult"
        bundle.mkdir(parents=True)
        (bundle / "Info.json").write_text("{\"private\":true}", encoding="utf-8")
        result = self.publish()
        self.assertEqual(len(result["quarantined"]), 2)
        self.assertFalse((self.output / "environment.json").exists())

    def test_ax_callback_plist_nested_json_canaries_block_or_quarantine(self):
        self.write("editor/editor-tree.json", [{"value": "CHERRI_PRIVATE_CANARY_not_for_logs"}])
        with self.assertRaises(PublicationBlocked) as caught:
            self.publish()
        self.assertNotIn("not_for_logs", str(caught.exception))
        self.assertFalse(self.output.exists())
        (self.root / "editor/editor-tree.json").unlink()
        self.write("runtime/callback-events.json", [{"url": "cherriqa://success?token=synthetic-secret"}])
        with self.assertRaises(PublicationBlocked):
            self.publish()

    def test_nested_auth_secret_is_blocked(self):
        self.write("runtime/output-assertions.json", {"nested": [{"authorization": "Bearer test-secret-12345"}]})
        with self.assertRaises(PublicationBlocked) as caught:
            self.publish()
        self.assertNotIn("test-secret-12345", str(caught.exception))

    def test_png_metadata_and_raw_plist_not_uploaded(self):
        (self.root / "screenshot.png").write_bytes(raster_secret() + b"CHERRI_PRIVATE_CANARY_metadata")
        (self.root / "raw.plist").write_bytes(b"<plist>CHERRI_PRIVATE_CANARY</plist>")
        result = self.publish()
        self.assertEqual(len(result["quarantined"]), 2)

    def test_provenance_not_self_assertion(self):
        self.trusted["qa-test.cherri"] = "different-source-hash"
        with self.assertRaisesRegex(PublicationBlocked, "FIXTURE_SOURCE_HASH_MISMATCH"):
            self.publish()

    def test_private_manifest_and_failed_existing_gate_blocked(self):
        self.write("compiler-manifest.json", {"syntheticOnly": False, "fixtures": []})
        with self.assertRaisesRegex(PublicationBlocked, "SYNTHETIC_PROVENANCE_REQUIRED"):
            self.publish()

    def test_unknown_paths_quarantined(self):
        self.write("unexpected.json", {"unverified": "not public"})
        self.publish()
        self.assertFalse((self.output / "unexpected.json").exists())

    def test_output_never_overwrites_existing_evidence(self):
        self.output.mkdir()
        with self.assertRaisesRegex(PublicationBlocked, "OUTPUT_MUST_BE_NEW"):
            self.publish()


if __name__ == "__main__":
    unittest.main()
