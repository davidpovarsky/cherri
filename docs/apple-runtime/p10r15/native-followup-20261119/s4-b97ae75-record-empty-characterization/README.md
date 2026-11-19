# S4 explicit Dictionary carrier: actual empty-field characterization

Run 44462966169 completed successfully at 05:57:36 UTC on 2026-11-19, at exact source b97ae752205f1e747b6c98ff7511f3484dd53bc2. The unchanged, 53-action public fixture qa-f04-record-empty-v2 was signed, imported, and executed on the iPadOS 27 Simulator. All four XCTest cases passed. The strict runtime oracle verified one fresh correlated compiled-shortcut callback and the before/after NDJSON trace.

The exact expected and observed result is:

CHERRI_QA_F04_RECORD_EMPTY_RESULT:outer=2:scalars=2:tags=LEFT,RIGHT:keys=2,2:items=0,0:empty=0:dictionary=Y

This establishes the following measured facts for the explicitly authored Dictionary carrier: two outer records, distinct LEFT/RIGHT tags, two keys per record, and zero Count after each items field read. The scalar and empty controls agree. The dictionary flag compares the OUTER record item's native type against an independent Dictionary control.

It does not independently establish the native type or identity of an empty inner List. It does not implement List<List>, change the language's return representation, close F04, replace the original bare-list oracle, or reinterpret the historical flattening failures. The E_TARGET_VALUE_SHAPE guard for unsupported authored List-valued collect loops remains in place. The original F04 source hashes were checked unchanged by the actual CI Phase0 producer.

Source SHA256: 71e107ee20de044274bac55b15439fe86cdb06b6b793d29a7beec49b38d320f4.
Actual CI unsigned plist SHA256: a600ec5d71a857813bc18606cbadae09e0ef91df5bf5e849f70f7c75ba9452d3.
Signed artifact SHA256: 6085c6aaf94110ab9c7a9eeef0549efbcce22cc0f85c7e53cd2280d54ef7f757.

All 20 published artifact hashes verify against the raw ZIP; 20 inputs remain quarantined and no raster was published. Editor capture completed, but the independent editor inspector remains NEEDS_REVIEW. Physical-device proof remains BLOCKED_DEVICE. Inline trace observations do not establish per-action typed coverage.

The original sanitized public ZIP, selected raw receipts, final source-bound CI metadata and bounded audit are included. The full official job log is retained outside Git with its size and digest. Raw bytes are preserved, including original newlines, and all indexed Git blobs are digest-verified before commit.
