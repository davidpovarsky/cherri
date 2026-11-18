import copy
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

from editor_inspector import inspect
from editor_ocr import OCRError, observe, parse_tsv, supplement, validate_segments, verified_models, sha, tesseract_observe
from prepare_public_artifacts import PublicationBlocked, stage
from test_editor_inspector import specimen


def observation(text="first", page=1):
    return {"schemaVersion": "1.0", "runId": "run-1", "fixtureId": "qa-unit-v2", "compilerSha": "source-sha",
            "pageIndex": page, "imageSha256": "1" * 64, "imageWidth": 800, "imageHeight": 1600,
            "segments": [{"text": text, "confidence": .83, "language": "en-US", "source": "LOCAL_TESSERACT",
                          "frame": {"x": 40, "y": 180, "width": 400, "height": 40}}]}


def manifest(page=1):
    return {"pageIndex": page, "imageSha256": "1" * 64, "viewportWidth": 400, "viewportHeight": 800}


class EditorOCRTests(unittest.TestCase):
    def hidden(self, values=("first", "second"), pages=1):
        source = specimen(values, pages)
        for node in source[1]:
            node.pop("value", None)
        return inspect(*source, "qa-unit-v2", "run-1", "source-sha")

    def test_visible_ocr_supplement_does_not_promote_hidden_parameter(self):
        observed, report = self.hidden()
        before = copy.deepcopy(report)
        observed, report = supplement(observed, report, [observation()], [manifest()])
        self.assertEqual(report["status"], before["status"])
        self.assertNotEqual(report["status"], "PASS")
        self.assertEqual(report["actions"][0]["visibleParameters"], before["actions"][0]["visibleParameters"])
        item = report["actions"][0]["ocrOnlyObservations"][0]
        self.assertEqual(item["status"], "OCR_ONLY")
        self.assertEqual(item["matchingCard"], "p1:n1")
        self.assertEqual(item["frame"], {"x": 20, "y": 90, "width": 200, "height": 20})
        self.assertEqual(item["confidence"], .83)
        self.assertFalse(item["identityProven"])
        self.assertFalse(item["hiddenValueProven"])

    def test_ax_text_takes_precedence(self):
        observed, report = inspect(*specimen(("first", "second")), "qa-unit-v2", "run-1", "source-sha")
        _, after = supplement(observed, report, [observation()], [manifest()])
        self.assertEqual(after["ocrSupplement"]["observationCount"], 0)
        self.assertEqual(after["status"], report["status"])

    def test_confident_wrong_text_remains_redacted_and_unverified(self):
        obs, report = self.hidden()
        data = observation("unlisted personal value")
        data["segments"][0]["confidence"] = 1.0
        _, after = supplement(obs, report, [data], [manifest()])
        self.assertNotIn("unlisted personal value", json.dumps(after))
        self.assertEqual(after["ocrSupplement"]["observations"][0]["text"], "[REDACTED_OCR_TEXT]")
        self.assertNotEqual(after["status"], "PASS")

    def test_ocr_cannot_supply_missing_native_identity(self):
        source = specimen(("first", "second"))
        for node in source[1]:
            node.pop("nativeUUIDDigests", None)
            node.pop("nativeActionIdentifier", None)
            node.pop("value", None)
        obs, report = inspect(*source, "qa-unit-v2", "run-1", "source-sha")
        _, after = supplement(obs, report, [observation()], [manifest()])
        self.assertEqual(after["summary"]["statusCounts"]["AX_UNEXPOSED"], 2)
        self.assertNotEqual(after["status"], "PASS")

    def test_run_image_and_page_mutations_rejected(self):
        for key, value in [("runId", "stale"), ("fixtureId", "qa-other"), ("compilerSha", "other"),
                           ("imageSha256", "2" * 64), ("pageIndex", 2)]:
            with self.subTest(key=key):
                obs, report = self.hidden()
                data = observation()
                data[key] = value
                original = copy.deepcopy((obs, report))
                with self.assertRaises(OCRError):
                    supplement(obs, report, [data], [manifest()])
                self.assertEqual((obs, report), original)

    def test_duplicate_pages_and_missing_capture_hash_rejected(self):
        for documents, captures in [([observation(), observation()], [manifest()]),
                                    ([observation()], [manifest(), manifest()]),
                                    ([observation()], [{**manifest(), "imageSha256": ""}])]:
            with self.assertRaises(OCRError):
                supplement(*self.hidden(), documents, captures)

    def test_invalid_geometry_and_confidence_rejected(self):
        for field, value in [("x", -1), ("x", float("inf")), ("width", 1000), ("height", 0), ("y", True)]:
            data = observation()
            data["segments"][0]["frame"][field] = value
            with self.assertRaises(OCRError):
                validate_segments(data)
        for confidence in [float("nan"), -0.1, 1.01, True]:
            data = observation()
            data["segments"][0]["confidence"] = confidence
            with self.assertRaises(OCRError):
                validate_segments(data)

    def test_ambiguous_geometry_not_assigned(self):
        obs, report = self.hidden()
        obs["cards"][1]["frame"] = copy.deepcopy(obs["cards"][0]["frame"])
        _, after = supplement(obs, report, [observation()], [manifest()])
        item = after["ocrSupplement"]["observations"][0]
        self.assertIsNone(item["matchingCard"])
        self.assertEqual(item["status"], "NEEDS_REVIEW")
        self.assertNotEqual(after["status"], "PASS")

    def test_seven_and_twenty_one_page_overlap_is_not_ocr_identity(self):
        for pages in (1, 7, 21):
            obs, report = self.hidden(("same", "same"), pages)
            docs = [observation("same", p) for p in range(1, pages + 1)]
            _, after = supplement(obs, report, docs, [manifest(p) for p in range(1, pages + 1)])
            self.assertEqual(after["observedCardCount"], 2)
            self.assertEqual(after["ocrSupplement"]["promotedAXStatuses"], 0)
            self.assertNotEqual(after["status"], "PASS")

    def test_hebrew_marks_emoji_and_confidence_retained_exactly(self):
        text = "שָׁלוֹם 😀"
        obs, report = self.hidden((text, "other"))
        data = observation(text)
        data["segments"][0].update(language="he-IL", confidence=.37)
        _, after = supplement(obs, report, [data], [manifest()])
        item = after["ocrSupplement"]["observations"][0]
        self.assertEqual((item["text"], item["language"], item["confidence"]), (text, "he-IL", .37))
        self.assertNotEqual(after["status"], "PASS")

    def test_fallback_only_for_unavailable_language_or_platform(self):
        calls = []
        def fallback(*args):
            calls.append(args)
            return {"engine": "LOCAL_TESSERACT"}
        for code in ("VISION_LANGUAGE_UNSUPPORTED", "VISION_PLATFORM_UNAVAILABLE"):
            def unsupported(*_):
                raise OCRError(code)
            self.assertEqual(observe("image", "he", "auto", "data", unsupported, fallback)["engine"], "LOCAL_TESSERACT")
        self.assertEqual(len(calls), 2)
        def failed(*_):
            raise OCRError("VISION_REQUEST_FAILED")
        with self.assertRaisesRegex(OCRError, "VISION_REQUEST_FAILED"):
            observe("image", "he", "auto", "data", failed, fallback)
        self.assertEqual(len(calls), 2)

    def test_tesseract_tsv_uses_reported_word_confidence_and_bounds(self):
        tsv = "level\tpage_num\tblock_num\tpar_num\tline_num\tword_num\tleft\ttop\twidth\theight\tconf\ttext\n"
        tsv += "1\t1\t0\t0\t0\t0\t0\t0\t800\t1600\t-1\t\n"
        tsv += "5\t1\t1\t1\t1\t1\t40\t180\t90\t40\t88.7\tשלום\n"
        tsv += "5\t1\t1\t1\t1\t2\t140\t180\t80\t40\t79.2\tעולם\n"
        width, height, segments = parse_tsv(tsv, "he-IL")
        self.assertEqual((width, height), (800, 1600))
        self.assertEqual(segments[0]["text"], "שלום עולם")
        self.assertAlmostEqual(segments[0]["confidence"], .792)
        self.assertEqual(segments[0]["frame"]["width"], 180)

    def test_model_bytes_must_match_pinned_checksum(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            data = b"controlled model bytes"
            lock = {"models": {"heb": {"bytes": len(data), "sha256": sha(data)}}}
            (root / "heb.traineddata").write_bytes(data)
            self.assertEqual(verified_models(root, "he-IL", lock)[0], "heb")
            (root / "heb.traineddata").write_bytes(b"corrupt model bytes!!!")
            with self.assertRaisesRegex(OCRError, "CHECKSUM_MISMATCH"):
                verified_models(root, "he-IL", lock)

    def test_engine_version_mismatch_blocks_before_recognition(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            data = b"controlled model bytes"
            lock = {"tesseractVersion": "5.5.0", "models": {"heb": {"bytes": len(data), "sha256": sha(data)}}}
            (root / "heb.traineddata").write_bytes(data)
            calls = []
            def old_version(command, **kwargs):
                calls.append(command)
                return subprocess.CompletedProcess(command, 0, stdout="tesseract 5.4.0\n", stderr="")
            with patch("editor_ocr.load_lock", return_value=lock):
                with self.assertRaisesRegex(OCRError, "TESSERACT_VERSION_MISMATCH"):
                    tesseract_observe("unused-image", "he-IL", root, runner=old_version)
            self.assertEqual(calls, [["tesseract", "--version"]])

    def test_raw_ocr_quarantined_and_renamed_raw_ocr_blocked(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp) / "raw"
            root.mkdir()
            (root / "editor").mkdir()
            trusted = {"qa-unit-v2.cherri": "sourcehash"}
            (root / "compiler-manifest.json").write_text(json.dumps({"syntheticOnly": True, "fixtures": [
                {"fixtureId": "qa-unit-v2", "fileName": "qa-unit-v2.cherri", "sourceSha256": "sourcehash"}]}))
            (root / "summary.json").write_text(json.dumps({"privacyGatePassed": True}))
            raw = {"publicationPolicy": "LOCAL_ONLY_RAW_OCR", "segments": [{"text": "private OCR"}]}
            (root / "editor" / "ocr-private-page-1.json").write_text(json.dumps(raw))
            publication = stage(root, Path(temp) / "public", trusted)
            self.assertEqual(len(publication["quarantined"]), 1)
            (root / "editor" / "editor-actions.json").write_text(json.dumps(raw))
            with self.assertRaisesRegex(PublicationBlocked, "RAW_OCR_NOT_PUBLIC"):
                stage(root, Path(temp) / "public2", trusted)


if __name__ == "__main__":
    unittest.main()
