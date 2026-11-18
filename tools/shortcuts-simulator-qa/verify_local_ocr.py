#!/usr/bin/env python3
"""Actual local OCR against generated English/Hebrew text; never native UI proof."""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import time

from PIL import Image, ImageDraw, ImageFont
from editor_ocr import load_lock


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--tessdata-dir", required=True)
    parser.add_argument("--report", required=True)
    args = parser.parse_args()
    root = Path(__file__).resolve().parent
    cases = [("en-US", "CHERRI QA 12345", "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", "ltr"),
             ("he-IL", "שלום עולם", "/usr/share/fonts/truetype/noto/NotoSansHebrew-Regular.ttf", "rtl")]
    report = {"schemaVersion": "1.0", "proofTier": "GENERATED_TEXT_OCR_ONLY",
              "humanReviewedAppleGroundTruth": False, "nativeEditorProof": False,
              "modelLock": load_lock(), "cases": []}
    with tempfile.TemporaryDirectory(prefix="cherri-ocr-ground-truth-") as tmp:
        temp = Path(tmp)
        for language, expected, font_path, direction in cases:
            image = Image.new("RGB", (1000, 240), "white")
            draw = ImageDraw.Draw(image)
            font = ImageFont.truetype(font_path, 64)
            draw.text((50, 60), expected, fill="black", font=font, direction=direction)
            image_path = temp / "page-0001.png"
            image.save(image_path)
            digest = hashlib.sha256(image_path.read_bytes()).hexdigest()
            capture = temp / "screenshots-manifest.json"
            capture.write_text(json.dumps([{"pageIndex": 1, "fileName": image_path.name,
                                            "imageSha256": digest, "viewportWidth": 1000, "viewportHeight": 240}]))
            output = temp / "ocr-private-page-1.json"
            command = [sys.executable, str(root / "editor_ocr.py"), "--engine", "tesseract",
                       "--tessdata-dir", args.tessdata_dir, "--image", str(image_path),
                       "--language", language, "--screenshots-manifest", str(capture), "--page", "1",
                       "--fixture", "qa-ocr-generated", "--run-id", "generated-ground-truth",
                       "--source-sha", "0" * 40, "--output", str(output)]
            start = time.monotonic()
            process = subprocess.run(command, capture_output=True, text=True, timeout=120)
            data = json.loads(output.read_text(encoding="utf-8")) if output.exists() else {}
            segments = data.get("segments", [])
            matched = process.returncode == 0 and len(segments) == 1 and segments[0]["text"] == expected
            report["cases"].append({"language": language, "status": "PASS" if matched else "FAIL",
                                    "exactTextMatched": matched, "exitCode": process.returncode,
                                    "diagnostic": json.loads(process.stdout),
                                    "observedSegmentCount": len(segments),
                                    "confidence": [s["confidence"] for s in segments],
                                    "frames": [s["frame"] for s in segments],
                                    "wallSeconds": round(time.monotonic() - start, 3),
                                    "imageSha256": digest, "expectedTextSha256": hashlib.sha256(expected.encode()).hexdigest(),
                                    "fontSha256": hashlib.sha256(Path(font_path).read_bytes()).hexdigest()})
            # Corrupted capture provenance must fail before recognition and emit no raw text.
            capture.write_text(json.dumps([{"pageIndex": 1, "fileName": image_path.name, "imageSha256": "f" * 64}]))
            control = subprocess.run(command, capture_output=True, text=True, timeout=30)
            report["cases"][-1]["staleImageRejected"] = control.returncode == 2 and "OCR_IMAGE_BINDING_MISMATCH" in control.stdout and expected not in control.stdout
    report["status"] = "PASS" if all(c["status"] == "PASS" and c["staleImageRejected"] for c in report["cases"]) else "FAIL"
    report["exactTextPrecision"] = sum(c["exactTextMatched"] for c in report["cases"]) / max(1, sum(c["observedSegmentCount"] for c in report["cases"]))
    report["exactTextRecall"] = sum(c["exactTextMatched"] for c in report["cases"]) / len(report["cases"])
    Path(args.report).write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
    print(json.dumps(report))
    return 0 if report["status"] == "PASS" else 1


if __name__ == "__main__":
    raise SystemExit(main())
