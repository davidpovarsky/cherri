import json
from pathlib import Path
import struct
import tempfile
import unittest
import zlib

from prepare_public_artifacts import PublicationBlocked, sha, stage, trusted_fixture_sources


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

    def test_long_ax_tree_compact_lossless_and_pretty_exceeds_privacy_limit(self):
        # A long, entirely synthetic 20-page capture must remain complete.
        # Compact text is not redaction and never relaxes the 10 MiB gate.
        nodes = [
            {
                "nodeIndex": i, "parentIndex": i - 1 if i else None,
                "siblingOrder": i % 6, "sourceScreenIndex": i // 800,
                "elementTypeRaw": 42, "elementTypeName": "StaticText",
                "label": "ANCHOR_" + "x" * 256,
                "identifier": "[REDACTED_NON_ALLOWLISTED_LEN_400]",
                "value": "[REDACTED_NON_ALLOWLISTED_LEN_300]",
                "frame": {"x": 100.5, "y": 221.7, "width": 640.0, "height": 37.0},
                "enabled": True, "visible": True, "geometryValid": True,
                "identifierDigest": "a" * 64, "nativeUUIDDigests": [],
                "nativeActionIdentifier": None, "actionContainerHint": False,
            }
            for i in range(12000)
        ]
        pretty = json.dumps(nodes, ensure_ascii=False, indent=2, sort_keys=True)
        compact = json.dumps(nodes, ensure_ascii=False, separators=(",", ":"), sort_keys=True)
        self.assertGreater(len(pretty.encode("utf-8")), 10 * 1024 * 1024)
        self.assertLess(len(compact.encode("utf-8")), 10 * 1024 * 1024)
        path = self.root / "editor/editor-tree.json"
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(pretty, encoding="utf-8")
        with self.assertRaisesRegex(PublicationBlocked, "TEXT_SIZE_LIMIT"):
            self.publish()
        self.assertFalse(self.output.exists())
        # Staging the complete compact bytes, not a prefix/subset, must succeed.
        path.write_text(compact, encoding="utf-8")
        published = self.publish()
        self.assertTrue(published["publicationGatePassed"])
        staged = json.loads((self.output / "editor/editor-tree.json").read_text(encoding="utf-8"))
        self.assertEqual(staged, nodes)
        self.assertEqual(staged[-1]["nodeIndex"], 11999)
        self.assertEqual(staged[-1]["parentIndex"], 11998)

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


    def test_nested_source_trust_is_explicit_relative_and_content_bound(self):
        fixtures = Path(self.temp.name) / "fixtures"
        fixtures.mkdir()
        approved = [
            "p15-label-probes/qa-label-file-carrier-v2.cherri",
            "p15-label-probes/qa-label-raw-empty-v2.cherri",
            "p15-label-probes/qa-label-raw-grouping-v2.cherri",
            "p15-export-probes/qa-export-unicode-v2.cherri",
            "p15-export-probes/qa-export-empty-v2.cherri",
            "p15-export-probes/qa-export-thousand-v2.cherri",
            "p15-export-probes/qa-export-redacted-v2.cherri",
            "p15-export-probes/qa-export-unsupported-file-v2.cherri",
            "p15-export-probes/qa-export-text-file-v2.cherri",
            "p15-paginate-probes/qa-paginate-ordered-v2.cherri",
            "p15-paginate-probes/qa-paginate-cycle-v2.cherri",
            "p15-paginate-probes/qa-paginate-page-cap-v2.cherri",
            "p15-paginate-probes/qa-paginate-item-cap-v2.cherri",
            "p15-paginate-probes/qa-paginate-empty-v2.cherri",
            "p15-binding-defaults/CherriBindingCapturePOC.cherri",
            "p15-binding-defaults/CherriTypedDefaultsPOC.cherri",
        ]
        unknown = ["private/qa-export-unicode-v2.cherri", "p15-export-probes/qa-private.cherri",
                   "p15-label-probes/qa-label-dictionary-carrier-v2.cherri",
                   "p15-export-probes/qa-export-empty-legacy.cherri",
                   "p15-paginate-probes/qa-paginate-private.cherri",
                   "private/qa-paginate-ordered-v2.cherri"]
        for index, name in enumerate(approved + unknown):
            p = fixtures / name
            p.parent.mkdir(parents=True, exist_ok=True)
            p.write_text("synthetic-" + str(index), encoding="utf-8")
        self.trusted = trusted_fixture_sources(fixtures)
        self.assertEqual(set(self.trusted), set(approved))
        self.assertNotIn("qa-export-unicode-v2.cherri", self.trusted)
        entries = [{"fixtureId": Path(name).stem, "fileName": name,
                    "sourceSha256": sha((fixtures / name).read_bytes())} for name in approved]
        self.write("compiler-manifest.json", {"syntheticOnly": True, "fixtures": entries})
        # Reviewed source hash does not authorize publishing its source/plist.
        raw = self.root / "builds/p15-export-probes"
        raw.mkdir(parents=True)
        (raw / "qa-export-unicode-v2.cherri").write_bytes((fixtures / approved[3]).read_bytes())
        (raw / "qa-export-unicode-v2.shortcut").write_bytes(b"<plist>synthetic</plist>")
        result = self.publish()
        self.assertEqual(len(result["quarantined"]), 2)
        self.assertTrue((self.output / "compiler-manifest.json").is_file())
        self.assertFalse((self.output / "builds").exists())

    def test_nested_path_alias_hash_and_identity_mutations_are_rejected(self):
        name = "p15-export-probes/qa-export-unicode-v2.cherri"
        self.trusted = {name: sha(b"reviewed synthetic")}
        valid = {"fixtureId": "qa-export-unicode-v2", "fileName": name, "sourceSha256": self.trusted[name]}
        cases = [
            ({**valid, "fileName": "qa-export-unicode-v2.cherri"}, "FIXTURE_SOURCE_HASH_MISMATCH"),
            ({**valid, "fileName": "private/qa-export-unicode-v2.cherri"}, "FIXTURE_SOURCE_HASH_MISMATCH"),
            ({**valid, "fileName": "p15-export-probes/../qa-export-unicode-v2.cherri"}, "FIXTURE_SOURCE_HASH_MISMATCH"),
            ({**valid, "sourceSha256": sha(b"changed source")}, "FIXTURE_SOURCE_HASH_MISMATCH"),
            ({**valid, "fixtureId": "qa-label-file-carrier-v2"}, "FIXTURE_SOURCE_ID_MISMATCH"),
        ]
        for entry, diagnostic in cases:
            with self.subTest(entry=entry):
                self.write("compiler-manifest.json", {"syntheticOnly": True, "fixtures": [entry]})
                with self.assertRaisesRegex(PublicationBlocked, diagnostic):
                    self.publish()
                self.assertFalse(self.output.exists())

    def test_nested_source_symlink_and_directory_symlink_are_not_trusted(self):
        fixtures = Path(self.temp.name) / "fixtures"
        reviewed_dir = fixtures / "p15-export-probes"
        reviewed_dir.mkdir(parents=True)
        target = Path(self.temp.name) / "synthetic.cherri"
        target.write_text("same-hash synthetic", encoding="utf-8")
        source = reviewed_dir / "qa-export-unicode-v2.cherri"
        try:
            source.symlink_to(target)
        except (OSError, NotImplementedError):
            self.skipTest("Host cannot create symlinks; this negative must run on Linux/macOS CI")
        with self.assertRaisesRegex(PublicationBlocked, "FIXTURE_SOURCE_SYMLINK"):
            trusted_fixture_sources(fixtures)
        source.unlink()
        reviewed_dir.rmdir()
        outside = Path(self.temp.name) / "outside"
        outside.mkdir()
        (outside / source.name).write_bytes(target.read_bytes())
        reviewed_dir.symlink_to(outside, target_is_directory=True)
        with self.assertRaisesRegex(PublicationBlocked, "FIXTURE_SOURCE_SYMLINK"):
            trusted_fixture_sources(fixtures)


    def test_unicode_escaped_json_secrets_are_blocked_without_echo(self):
        cases = [
            ("environment.json", {"note": "CHERRI_PRIVATE_CANARY_pixel_backup"}),
            ("runtime/output-assertions.json", {"nested": [{"password": "synthetic-test-value"}]}),
            ("runtime/runtime-trace.jsonl", {"value": "cherriqa://done?token=synthetic-test-value"}),
        ]
        for name, value in cases:
            with self.subTest(name=name), tempfile.TemporaryDirectory() as directory:
                root = Path(directory) / "raw"
                root.mkdir()
                for base in ("compiler-manifest.json", "summary.json"):
                    (root / base).write_bytes((self.root / base).read_bytes())
                path = root / name
                path.parent.mkdir(parents=True, exist_ok=True)
                raw = json.dumps(value)
                # Legal JSON escapes hide letters from raw-byte token matching.
                escaped = "".join("\\u%04x" % ord(ch) if ch.isalpha() else ch for ch in raw)
                path.write_text(escaped + "\n", encoding="utf-8")
                self.assertEqual(json.loads(escaped), value)
                output = Path(directory) / "public"
                with self.assertRaisesRegex(PublicationBlocked, "SECRET_OR_PRIVATE_CANARY") as caught:
                    stage(root, output, self.trusted)
                self.assertNotIn("synthetic-test-value", str(caught.exception))
                self.assertNotIn("pixel_backup", str(caught.exception))
                self.assertFalse(output.exists())
                self.assertEqual(path.read_text(encoding="utf-8"), escaped + "\n")

    def test_nested_raw_ocr_markers_are_blocked_in_json_and_jsonl(self):
        for name, value in [
            ("environment.json", {"nested": [{"publicationPolicy": "LOCAL_ONLY_RAW_OCR", "text": "local"}]}),
            ("runtime/runtime-trace.jsonl", [{"publicationPolicy": "LOCAL_ONLY_RAW_OCR", "text": "local"}]),
        ]:
            with self.subTest(name=name), tempfile.TemporaryDirectory() as directory:
                root = Path(directory) / "raw"
                root.mkdir()
                for base in ("compiler-manifest.json", "summary.json"):
                    (root / base).write_bytes((self.root / base).read_bytes())
                path = root / name
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text(json.dumps(value) + "\n", encoding="utf-8")
                output = Path(directory) / "public"
                with self.assertRaisesRegex(PublicationBlocked, "RAW_OCR_NOT_PUBLIC"):
                    stage(root, output, self.trusted)
                self.assertFalse(output.exists())


    def test_duplicate_decoded_keys_cannot_discard_private_values(self):
        (self.root / "environment.json").write_text(
            '{"\\u0070assword":"\\u0073ynthetic-test-value","password":""}',
            encoding="utf-8")
        with self.assertRaisesRegex(PublicationBlocked, "INVALID_TEXT_OR_STRUCTURED_DATA"):
            self.publish()
        self.assertFalse(self.output.exists())

    def test_output_never_overwrites_existing_evidence(self):
        self.output.mkdir()
        with self.assertRaisesRegex(PublicationBlocked, "OUTPUT_MUST_BE_NEW"):
            self.publish()


if __name__ == "__main__":
    unittest.main()
