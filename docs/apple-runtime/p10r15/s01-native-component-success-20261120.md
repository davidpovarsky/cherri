# S01 native component replay — 20 November 2026

## Observed outcome

[Run 44671266567](https://github.com/davidpovarsky/cherri/actions/runs/44671266567) completed successfully on exact compiler source `da0ba0e5e49a1be56f97b725f33ff56e3a2c91af`. The existing full iOS 27 Shortcuts Runtime PoC workflow ran from 03:58:33 to 04:28:34 UTC. All five fixed public sources were compiled, signed through the explicit reviewed public-fixture HubSign route, imported and exercised. The owned iOS 27.2 Simulator was deleted successfully. The signing receipt retains its actual `AEA1_HEADER_PASS` verification tier; this report does not substitute it for a cryptographic verifier.

The unchanged coverage fixture produced exactly **313 UTF-8 bytes**, SHA256 `f2e79540f2c95547ac9c83d59f25b63ce4167153d81731a5e00b746f041037f9`. These are the original expected bytes, including literal U+FFFC, Hebrew, astral and combining-character interpolation and all nine F01 Boolean/effect cases. The source hash `4a13f90af2bdc326e24f9dbf785f76a1ae41043e176b5a3188ab794694a19aa3` and expected-contract hash `8baa3279a770ca20e3daa6036f416360a308d6f40eb1dd0874a38afeb7d0b10c` were independently compared with original K commit `8e66254d56ea3e4ac70f6cda6ef9d31217e0937c`. Neither source nor golden changed. Original K run 44653155762 and its failed 302-byte observation remain retained.

## Retention and independent production replay

Original artifact `14601920849` is 136,750 bytes, ZIP SHA256 `bd01c1decea628db91d4569de32f845c5cd5c61ff8d29ff9cf9dc52b3c393490`. Its API source/run/attempt, nonexpired metadata, size and digest were checked before bounded extraction. The original ZIP, API metadata and job log remain retained.

In a clean checkout of exact `da0ba0e5`, the real `runtime_poc_coverage.verify_result`, `runtime_poc_formal_evidence.export_records` and `runtime_poc_publication.stage` functions were replayed against downloaded bytes. They verified fresh reset/nonce, source/signing/run bindings, owned-device cleanup and exact observations. All 17 raw diagnostic records passed. The six formal rows reproduced byte for byte, and the same 31 published file identities and hashes were reproduced. Replays were separate equality checks; submitted evidence remains the original downloaded files.

## Actual Go intake

The unchanged production command `go run ./tools/language-acceptance --evidence <original-records-wrapper.json> --phase local --out <runtime-intake-report.json>` consumed a wrapper containing the original six rows and the actual successful API run. Original relative artifact paths were restored without changing the referenced bytes; previously retained artifacts were preserved separately.

| Registered runtime row | Requirements observed |
| --- | --- |
| `ios27-runtime-poc:variables` | V01, V03 |
| `ios27-runtime-poc:indexing` | L02, L04 |
| `ios27-runtime-poc:numbers` | N01 |
| `ios27-runtime-poc:functions` | FN01, FN02, FN03 |
| `ios27-runtime-poc:coverage-s01` | S01 |
| `ios27-runtime-poc:coverage-f01` | F01 |

All ten corresponding results contain `executed_levels: [ios-runtime]`. The standalone native-only intake correctly exits **1**, with 174 missing-local-tier errors and **zero other validation errors**. It has `closure_complete: false`: no local producer result was fabricated or borrowed. Its 0 passed / 174 failed due to absent local evidence / 5 skipped counts describe this intentionally native-only input, not a new full-project regression. The intake report hash is `b35404bcaa7e1ed4e949789cd09c891411185dcda64c8c9524975bf69fc69670`.

## Scope still required

This is source-bound component runtime proof. It does not inherit final R4 acceptance, consumer pins, full iOS app/UI success, Optional support, physical-device execution or closure of all 179 requirements. Every immutable contract still requires its own local and external tiers on the appropriate final source. See the paired JSON for exact artifact and execution digests.
