# Native macro causal diagnostic fixtures

These five explicit signed-QA selections isolate the observed require, binary File carrier, and debug export failures. They use the existing canonical compiler, signed simulator harness, synthetic anchors, fresh correlated callback, exact result marker, and normal required trace. No compiler macro, native action registry, signing route, publisher exception, or runtime success rule changes.

## Why the split is necessary

- Require run 44253536149 returned a fresh success callback containing P15_REQUIRE_ZERO instead of its required result. The diagnostic compares four independent native numeric paths with zero: mutable literal (L), direct number(0) (D), the unchanged require result (R), and a mutable alias of that result (B). The output contains only independent fixed status strings; it does not coerce a Number to Text and treat that as numeric proof.
- File carrier run 44248707476 returned P15_LABEL_FILE_BYTES_FAIL. Its single verdict can overwrite an earlier baseline or type failure. The new source reports baseline encode (B), Get Variable byte transport (V), transported type (VT), label (L), restored type (T), restored bytes (R), and original-source preservation (S) independently. It preserves the original synthetic bytes and Unicode label. Generic typed File-in-Dictionary support remains unproven.
- Export run 44248722472 returned a correlated native error (code 0, redacted message length 82), without an output result. The File control uses existing conversion, Set Name, Get Name, and Get Text on a literal JSON value. Build-only uses the exact original Unicode payload and export options, then checks its File name. Parse-only adds JSON parsing plus independent schema and entry-count checks. Payload Base64 decode is absent from these splits.

The original require, File carrier, and Unicode export source bytes are unchanged. Their hashes and origin commit are recorded in the companion JSON. A diagnostic success does not rewrite their historical failed result.

## Local evidence

The actual CLI was built from compiler commit 867261a3ad6ceac113690f5be01f3d576d572759, then compiled all five newly authored source files with --skip-sign --build-profile=debug. This is the compiler snapshot used for local checks; the later source commit is separately identified by each content hash. Native execution is **NOT_RUN** for these five diagnostics.

| Fixture | Native actions emitted | Source SHA-256 |
| --- | ---: | --- |
| qa-require-zero-diag-v2 | 60 | e3a01abe691f3a1acb99a4377b97759be9cf802bbaae8eec43e6fccef69ce3a1 |
| qa-label-file-controls-v2 | 70 | 4b6c321e07fd9d4060aca9168a12c9885cbc99c04aa38ef8244fba678c4375d1 |
| qa-export-file-control-v2 | 31 | cb6ae6a52b7c3ba4d73f2a19a83a6a1bfc9ec6c92d19d6ea12af3dce185c0db4 |
| qa-export-build-only-v2 | 38 | d797fa0bd820f9d02e7997d1455dfac7f8e44400bb3d87692e919ed3a9c03f2b |
| qa-export-parse-only-v2 | 48 | d3430d4fcb57c63f845804b524dc135feea843546027f09bf0a33deea8f33bc4 |

The companion JSON records exact unsigned plist hashes, compiler binary hash, and emitted action identifiers. The require graph audit follows the original Number producer through the require Get Variable and the subsequent Set Variable, then checks each independent numeric comparison. Six corruptions of those edges or numeric/status checks were rejected. This establishes the emitted graph, not Apple runtime behavior.

The complete current Python suite passed **103/103** tests. The three added test methods exercise the actual selector program and callback oracle: every =ok flag is independently changed to =bad and rejected; stale/self/duplicate callbacks, errors, cancellation, empty trace, unreviewed selector variants, and a Release-profile exception are rejected. All new selections remain explicit V2/debug sources. Existing originals remain selectable.

## Native execution order and limits

Run qa-require-zero-diag on the current integrated compiler so its exact source SHA and compiler SHA determine the observation. The later numeric materialization repair is not silently attributed to this older local build.

Run qa-label-file-controls to distinguish its independent transport edges. For export, inspect qa-export-file-control and qa-export-build-only before proceeding to qa-export-parse-only. Keep every ordinary runtime/trace gate strict even when a failure is diagnostically useful.

An action count, local plist, or passing callback-oracle test is not native success. A signed result still requires the exact expected marker, fresh source-bound callback and required trace; Inspector mapping and physical-device evidence remain independent gates.
