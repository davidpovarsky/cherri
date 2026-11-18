#!/usr/bin/env python3
"""Optional local OCR; raw observations remain private and never prove identity."""

import argparse
import copy
import csv
import hashlib
import io
import json
import math
import os
from pathlib import Path
import re
import subprocess
import sys
import urllib.request

ROOT = Path(__file__).resolve().parent
MAX_SEGMENTS = 1000
LOCK_PATH = ROOT / "ocr-tesseract-lock.json"


class OCRError(ValueError):
    """Only fixed, non-sensitive error codes may leave the recognizer."""


def sha(data):
    return hashlib.sha256(data).hexdigest()


def load_lock():
    return json.loads(LOCK_PATH.read_text(encoding="utf-8"))


def verified_models(directory, language, lock=None):
    lock = lock or load_lock()
    code = {"en": "eng", "he": "heb"}.get(language.split("-")[0].lower())
    if code is None:
        raise OCRError("TESSERACT_LANGUAGE_UNSUPPORTED")
    # One exact language model per request; no implicit English fallback.
    item = lock["models"][code]
    path = Path(directory) / (code + ".traineddata")
    try:
        data = path.read_bytes()
    except OSError as exc:
        raise OCRError("MODEL_MISSING") from exc
    if len(data) != item["bytes"] or sha(data) != item["sha256"]:
        raise OCRError("MODEL_CHECKSUM_MISMATCH")
    return code, item["sha256"]


def fetch_models(directory):
    """Explicit setup operation; fixed commit URLs and verified atomic writes."""
    lock = load_lock()
    directory = Path(directory)
    directory.mkdir(parents=True, exist_ok=True)
    for code, item in lock["models"].items():
        expected = f"https://raw.githubusercontent.com/tesseract-ocr/tessdata_fast/{lock['revision']}/{code}.traineddata"
        if item["url"] != expected or not re.fullmatch(r"[0-9a-f]{40}", lock["revision"]):
            raise OCRError("MODEL_LOCK_INVALID")
        with urllib.request.urlopen(expected, timeout=45) as response:
            data = response.read(item["bytes"] + 1)
        if len(data) != item["bytes"] or sha(data) != item["sha256"]:
            raise OCRError("MODEL_DOWNLOAD_CHECKSUM_MISMATCH")
        temporary = directory / (code + ".verified-download")
        temporary.write_bytes(data)
        temporary.replace(directory / (code + ".traineddata"))


def finite_number(value):
    return isinstance(value, (int, float)) and not isinstance(value, bool) and math.isfinite(value)


def parse_tsv(text, language):
    rows = list(csv.DictReader(io.StringIO(text), delimiter="\t", quoting=csv.QUOTE_NONE))
    pages = [r for r in rows if r.get("level") == "1"]
    if len(pages) != 1:
        raise OCRError("TESSERACT_PAGE_INVALID")
    width, height = int(pages[0]["width"]), int(pages[0]["height"])
    lines = {}
    for row in rows:
        if row.get("level") != "5" or not row.get("text", "").strip():
            continue
        confidence = float(row["conf"])
        if not math.isfinite(confidence) or not 0 <= confidence <= 100:
            raise OCRError("TESSERACT_CONFIDENCE_INVALID")
        key = tuple(row[k] for k in ("page_num", "block_num", "par_num", "line_num"))
        lines.setdefault(key, []).append(row)
    if len(lines) > MAX_SEGMENTS:
        raise OCRError("SEGMENT_LIMIT")
    segments = []
    for words in lines.values():
        left = min(int(w["left"]) for w in words)
        top = min(int(w["top"]) for w in words)
        right = max(int(w["left"]) + int(w["width"]) for w in words)
        bottom = max(int(w["top"]) + int(w["height"]) for w in words)
        segments.append({"text": " ".join(w["text"] for w in words),
                         "confidence": min(float(w["conf"]) for w in words) / 100,
                         "confidenceMethod": "minimumWordConfidence",
                         "source": "LOCAL_TESSERACT", "language": language,
                         "frame": {"x": left, "y": top, "width": right - left, "height": bottom - top}})
    return width, height, segments


def tesseract_observe(image, language, tessdata, executable="tesseract", runner=subprocess.run):
    lock = load_lock()
    code, model_hash = verified_models(tessdata, language, lock)
    version = runner([executable, "--version"], capture_output=True, text=True, timeout=10)
    first = version.stdout.splitlines()[0] if version.stdout else ""
    if version.returncode or first != "tesseract " + lock["tesseractVersion"]:
        raise OCRError("TESSERACT_VERSION_MISMATCH")
    result = runner([executable, str(image), "stdout", "--tessdata-dir", str(tessdata),
                     "-l", code, "--oem", "1", "--psm", "6", "-c", "tessedit_create_tsv=1"],
                    capture_output=True, text=True, encoding="utf-8", timeout=90,
                    env=dict(os.environ, OMP_THREAD_LIMIT="1"))
    if result.returncode:
        raise OCRError("TESSERACT_REQUEST_FAILED")
    width, height, segments = parse_tsv(result.stdout, language)
    return {"status": "OBSERVED", "engine": "LOCAL_TESSERACT", "engineVersion": lock["tesseractVersion"],
            "modelRevision": lock["revision"], "modelSha256": model_hash,
            "chosenLanguage": language, "imageWidth": width, "imageHeight": height,
            "segmentationMode": 6, "engineMode": 1, "segments": segments}


def vision_observe(image, language, runner=subprocess.run):
    if sys.platform != "darwin":
        raise OCRError("VISION_PLATFORM_UNAVAILABLE")
    result = runner(["swift", str(ROOT / "recognize_editor_text.swift"), str(image), language],
                    capture_output=True, text=True, encoding="utf-8", timeout=120)
    try:
        data = json.loads(result.stdout)
    except (ValueError, TypeError) as exc:
        raise OCRError("VISION_RESPONSE_INVALID") from exc
    if result.returncode:
        if result.returncode == 3 and data.get("status") == "LANGUAGE_UNSUPPORTED":
            raise OCRError("VISION_LANGUAGE_UNSUPPORTED")
        raise OCRError("VISION_REQUEST_FAILED")
    if data.get("recognitionLevel") != "accurate" or data.get("revision") != 3:
        raise OCRError("VISION_CONFIGURATION_MISMATCH")
    return data


def observe(image, language, engine, tessdata, vision=vision_observe, tesseract=tesseract_observe):
    if engine in ("auto", "vision"):
        try:
            return vision(image, language)
        except OCRError as exc:
            if engine == "vision" or str(exc) not in {"VISION_PLATFORM_UNAVAILABLE", "VISION_LANGUAGE_UNSUPPORTED"}:
                raise
    if tessdata is None:
        raise OCRError("VERIFIED_TESSDATA_REQUIRED")
    return tesseract(image, language, tessdata)


def validate_segments(data):
    if not isinstance(data, dict):
        raise OCRError("OCR_DOCUMENT_INVALID")
    width, height = data.get("imageWidth"), data.get("imageHeight")
    if not all(finite_number(v) and 0 < v <= 16384 for v in (width, height)):
        raise OCRError("IMAGE_GEOMETRY_INVALID")
    segments = data.get("segments")
    if not isinstance(segments, list) or len(segments) > MAX_SEGMENTS:
        raise OCRError("SEGMENT_LIMIT")
    for segment in segments:
        if not isinstance(segment, dict):
            raise OCRError("SEGMENT_INVALID")
        text = segment.get("text")
        confidence = segment.get("confidence")
        box = segment.get("frame", {})
        x, y, w, h = (box.get(k) for k in ("x", "y", "width", "height"))
        if not isinstance(text, str) or not text.strip() or len(text) > 2048:
            raise OCRError("TEXT_OBSERVATION_INVALID")
        if not finite_number(confidence) or not 0 <= confidence <= 1:
            raise OCRError("CONFIDENCE_INVALID")
        if not all(finite_number(v) for v in (x, y, w, h)) or min(x, y) < 0 or min(w, h) <= 0 or x + w > width + 1 or y + h > height + 1:
            raise OCRError("SEGMENT_GEOMETRY_INVALID")
        if segment.get("source") not in {"APPLE_VISION", "LOCAL_TESSERACT"} or not re.fullmatch(r"[a-zA-Z]{2,3}(?:-[A-Za-z0-9]{2,8})*", segment.get("language", "")):
            raise OCRError("SEGMENT_PROVENANCE_INVALID")


def supplement(observed, comparison, documents, screenshot_manifest):
    """Append OCR-only evidence. Preserve every AX status and its PASS decision."""
    observed, comparison = copy.deepcopy(observed), copy.deepcopy(comparison)
    if not isinstance(screenshot_manifest, list) or any(not isinstance(e, dict) for e in screenshot_manifest):
        raise OCRError("SCREENSHOT_MANIFEST_INVALID")
    manifest = {entry.get("pageIndex"): entry for entry in screenshot_manifest}
    if len(manifest) != len(screenshot_manifest):
        raise OCRError("DUPLICATE_SCREENSHOT_PAGE")
    allowed = {p["value"] for action in comparison["actions"] for p in action["visibleParameters"]}
    allowed.update(text for card in observed["cards"] for text in card["texts"])
    observations, seen_pages = [], set()
    for document in documents:
        validate_segments(document)
        if any(document.get(key) != observed.get(key) for key in ("runId", "fixtureId", "compilerSha")):
            raise OCRError("OCR_RUN_BINDING_MISMATCH")
        page = document.get("pageIndex")
        entry = manifest.get(page)
        if page in seen_pages:
            raise OCRError("DUPLICATE_OCR_PAGE")
        seen_pages.add(page)
        if not entry or not re.fullmatch(r"[0-9a-f]{64}", entry.get("imageSha256", "")) or entry["imageSha256"] != document.get("imageSha256"):
            raise OCRError("OCR_IMAGE_BINDING_MISMATCH")
        vw, vh = entry.get("viewportWidth"), entry.get("viewportHeight")
        if not all(finite_number(v) and v > 0 for v in (vw, vh)):
            raise OCRError("VIEWPORT_INVALID")
        if abs(vw / vh - document["imageWidth"] / document["imageHeight"]) > .005:
            raise OCRError("SCREENSHOT_ASPECT_MISMATCH")
        for segment in document["segments"]:
            box = segment["frame"]
            x, w = box["x"] * vw / document["imageWidth"], box["width"] * vw / document["imageWidth"]
            y, h = box["y"] * vh / document["imageHeight"], box["height"] * vh / document["imageHeight"]
            candidates = []
            for card in observed["cards"]:
                f = card["frame"]
                if card["page"] == page and card["fullyVisible"] and f["x"] <= x and f["y"] <= y and x + w <= f["x"] + f["width"] and y + h <= f["y"] + f["height"]:
                    candidates.append(card)
            card = candidates[0] if len(candidates) == 1 else None
            # AX text wins; do not manufacture duplicate evidence through OCR.
            if card and segment["text"] in card["texts"]:
                continue
            safe_text = segment["text"] if segment["text"] in allowed else "[REDACTED_OCR_TEXT]"
            item = {"text": safe_text, "page": page, "frame": {"x": x, "y": y, "width": w, "height": h},
                    "confidence": segment["confidence"], "language": segment["language"],
                    "source": segment["source"], "matchingCard": card["nodeId"] if card else None,
                    "status": "OCR_ONLY" if card else "NEEDS_REVIEW",
                    "identityProven": False, "hiddenValueProven": False}
            observations.append(item)
            if card:
                card.setdefault("ocrOnlyObservations", []).append(item)
                for action in comparison["actions"]:
                    if any(e.get("nodeId") == card["nodeId"] for e in action["evidence"]):
                        action.setdefault("ocrOnlyObservations", []).append(item)
    for target in (observed, comparison):
        target["ocrUsed"] = bool(documents)
        target["ocrSupplement"] = {"proofTier": "DISPLAYED_TEXT_ONLY", "observations": observations,
                                   "observationCount": len(observations), "promotedAXStatuses": 0,
                                   "rawTextPublication": "ONLY_EXISTING_EXPECTED_OR_AX_TEXT"}
    return observed, comparison


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--fetch-models", type=Path)
    parser.add_argument("--image", type=Path)
    parser.add_argument("--language", default="en-US")
    parser.add_argument("--engine", choices=("auto", "vision", "tesseract"), default="auto")
    parser.add_argument("--tessdata-dir", type=Path)
    parser.add_argument("--screenshots-manifest", type=Path)
    parser.add_argument("--page", type=int)
    for field in ("fixture", "run-id", "source-sha", "output"):
        parser.add_argument("--" + field)
    args = parser.parse_args()
    try:
        if args.fetch_models:
            fetch_models(args.fetch_models)
            print(json.dumps({"status": "MODELS_VERIFIED", "models": ["eng", "heb"]}))
            return 0
        if any(value is None for value in (args.image, args.screenshots_manifest, args.page, args.fixture, args.run_id, args.source_sha, args.output)):
            raise OCRError("REQUIRED_INPUT_MISSING")
        if not re.fullmatch(r"[0-9a-f]{40}", args.source_sha) or not re.fullmatch(r"qa-[a-z0-9-]{1,80}", args.fixture) or not re.fullmatch(r"[A-Za-z0-9_-]{1,160}", args.run_id):
            raise OCRError("BINDING_INVALID")
        data = args.image.read_bytes()
        if not 0 < len(data) <= 20 * 1024 * 1024:
            raise OCRError("IMAGE_SIZE_LIMIT")
        entries = json.loads(args.screenshots_manifest.read_text(encoding="utf-8"))
        if not isinstance(entries, list) or any(not isinstance(e, dict) for e in entries):
            raise OCRError("SCREENSHOT_MANIFEST_INVALID")
        matches = [e for e in entries if e.get("pageIndex") == args.page]
        if len(matches) != 1 or matches[0].get("imageSha256") != sha(data) or matches[0].get("fileName") != args.image.name:
            raise OCRError("OCR_IMAGE_BINDING_MISMATCH")
        result = observe(args.image, args.language, args.engine, args.tessdata_dir)
        validate_segments(result)
        result.update({"schemaVersion": "1.0", "publicationPolicy": "LOCAL_ONLY_RAW_OCR",
                       "fixtureId": args.fixture, "runId": args.run_id, "compilerSha": args.source_sha,
                       "pageIndex": args.page, "imageSha256": sha(data)})
        Path(args.output).write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
        print(json.dumps({"status": "OBSERVED", "engine": result["engine"], "segments": len(result["segments"]), "rawOutput": "LOCAL_ONLY"}))
        return 0
    except (OCRError, OSError, ValueError, TypeError, KeyError, subprocess.SubprocessError) as exc:
        code = str(exc) if isinstance(exc, OCRError) else "LOCAL_OCR_FAILED"
        print(json.dumps({"status": "NEEDS_REVIEW", "reason": code}))
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
