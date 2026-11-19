# Documentation app evidence

The D03 collector extension joins the positive bundled-index test, the
historical/damaged-index rejection test, and the documentation UI test from one
successful Xcode process. None of the three tests independently produces a D03
integration record.

The accepted unit receipt stays as its exact raw JSON log line. Unknown keys,
duplicate JSON keys, invalid values, extra receipts, incorrect targets, missing
or repeated test boundaries, and a misplaced or repeated UI checkpoint prevent
the record. Rejected arbitrary fields do not enter the published log.

## Build inputs

The existing five positional collector arguments remain unchanged. Full
documentation verification also supplies these explicit build artifacts:

    --documentation-index CherriApp/Resources/Documentation/documentation-index.json
    --documentation-manifest ../skills/cherri-shortcuts/compatibility-manifest.json
    --documentation-receipt .build/documentation-receipt.json

The flags remain optional for unrelated runs. All three inputs are required
before the collector emits D03. It copies their original bytes under
documentation-index.json, documentation-compatibility-manifest.json, and
documentation-receipt.json. The record includes those three digests plus the
bounded Xcode log and diagnostics digest, all with the same run ID.

The receipt pins, four unique source path/hash pairs, bundle SHA256, section
count, and migration count must agree with the actual bundle. The generator
stdout receipt independently binds the source-file count, section count,
byte length, and bundle SHA256. The manifest's compiler source commit may differ
from the app implementation commit.

## Scope

This adapter validates observed data and cannot establish Apple execution by
itself. Formal intake additionally verifies the successful iOS Build run and
checkout, and compares the supplied manifest bytes to the immutable Git object
at that implementation commit. Website and installed Skill evidence remain
separate D03 obligations.

The local Python tests deliberately use synthetic XCTest logs. They establish
collector rejection and byte-binding behavior only; they do not execute Swift,
the app, or Shortcuts. A missing or invalid documentation input yields
documentationEvidence.status=NOT_VERIFIED and no D03 record. Existing unrelated
collector records retain their prior behavior.
