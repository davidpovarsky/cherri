# Optional local OCR for the AX Editor Inspector

The existing AX/native-plist comparison remains authoritative for its stated evidence tier. OCR is an optional displayed-text supplement. It never supplies native action identity, hidden values, UUID equivalence, object types, or a missing AX match. The original action and parameter statuses are preserved exactly, including failures and unknowns.

## Local recognition and provenance

`editor_ocr.py` prefers Apple's local Vision request on macOS, pins request revision 3 and `recognitionLevel = accurate`, disables language correction and automatic language guessing, and queries `supportedRecognitionLanguages()` for the selected request. The chosen supported locale is recorded. Vision coordinates are converted from normalized lower-left image coordinates to pixel coordinates with a top-left origin.

Unsupported language/platform can fall back to Tesseract only when its exact version is 5.5.0 and the requested English or Hebrew model matches `ocr-tesseract-lock.json`. Models are pinned to the upstream tessdata_fast commit 87416418657359cb625c412a48b6e1d6d41c29bd, immutable download URL, byte length and SHA256. Explicit setup downloads verified model bytes atomically. Ordinary recognition makes no network request. General Vision errors do not silently become successful fallback observations.

Capture-time screenshot SHA256, page number, viewport dimensions, fixture, run ID and compiler SHA bind each observation to its source. Historical screenshot manifests without an image hash cannot be used for OCR evidence. OCR pixel boxes are scaled into AX viewport coordinates only when the aspect ratio matches. A segment must fit exactly one fully visible AX card; overlap, missing card, another page, or uncertain geometry remains `NEEDS_REVIEW`. Text already exposed through AX is not counted again.

Raw OCR JSON carries `publicationPolicy: LOCAL_ONLY_RAW_OCR`. Unknown paths are quarantined by the existing publisher; renaming that raw document to an otherwise allowed path is rejected. Public supplements retain only text already present in the expected plist's visible scalar values or sanitized AX text. Other recognized text becomes `[REDACTED_OCR_TEXT]`. Raster remains quarantined regardless of OCR outcome; this is not a pixel-redaction claim.

## Reproducible usage

The raw screenshot and raw OCR file stay in the authorized local run directory. Use the actual run/fixture/compiler identifiers from that run, not the example placeholders.

```sh
python3 tools/shortcuts-simulator-qa/editor_ocr.py --fetch-models /local/verified-tessdata
python3 tools/shortcuts-simulator-qa/editor_ocr.py \
  --image /local/run/editor/page-0001.png --language he-IL --engine auto \
  --tessdata-dir /local/verified-tessdata \
  --screenshots-manifest /local/run/editor/screenshots-manifest.json --page 1 \
  --fixture qa-rtl-hebrew-v2 --run-id ACTUAL_RUN_ID --source-sha ACTUAL_COMPILER_SHA \
  --output /local/run/editor/ocr-private-page-1.json
python3 tools/shortcuts-simulator-qa/editor_inspector.py \
  --plist /local/run/compiled.plist --tree /local/run/editor/editor-tree.json \
  --coverage /local/run/editor/editor-action-coverage.json \
  --fixture qa-rtl-hebrew-v2 --run-id ACTUAL_RUN_ID --source-sha ACTUAL_COMPILER_SHA \
  --ocr-json /local/run/editor/ocr-private-page-1.json \
  --screenshots-manifest /local/run/editor/screenshots-manifest.json \
  --out-dir /local/run/editor
```

The optional Tesseract verifier needs Pillow and the named Linux test fonts. Those are test-fixture dependencies, not new dependencies of the default AX inspector. `verify_local_ocr.py --tessdata-dir ... --report ...` renders controlled text, invokes the real CLI, compares exact recognized text, records actual confidences/geometry, and rejects a corrupted image hash before recognition. Vision's `--self-test` renders controlled text through CoreText and executes the actual Vision request without writing or uploading an image.

## Actual verification and limitations

The initial local runtime attempt failed because a custom tessdata directory did not contain the optional `tsv` configuration file. The retained failed receipt is followed by a repair using the installed Tesseract binary's explicit `tessedit_create_tsv` parameter. Assertions were unchanged.

After that repair, both generated English and Hebrew images matched exactly: 2/2 observations, precision 1.0 and recall 1.0 **only for this two-image generated-text set**. Observed conservative word confidences were approximately 0.927 and 0.911. Both stale-image controls were rejected. Thirty inspector/OCR unit tests passed, including run/image/page mutations, wrong values, absent native identity, ambiguous boxes, 1/7/21-page overlap, repeated cards, Hebrew diacritics/emoji preservation, invalid/nonfinite geometry, model checksum and engine version corruption, and raw OCR publication rejection.

These are not human-reviewed Apple screenshot ground truth, broad OCR accuracy measurements, or signed Editor Inspector acceptance. Actual Apple Vision execution is recorded separately after CI. Existing AX matching and native execution gates remain open where their own evidence is missing.

## Primary references

- Apple Vision text recognition and device-local processing: https://developer.apple.com/documentation/vision/recognizing-text-in-images
- Request settings and runtime language query: https://developer.apple.com/documentation/vision/vnrecognizetextrequest
- Vision normalized coordinate system: https://developer.apple.com/documentation/vision
- Pinned upstream model source: https://github.com/tesseract-ocr/tessdata_fast/tree/87416418657359cb625c412a48b6e1d6d41c29bd

No paid image model or private Apple API is used.
