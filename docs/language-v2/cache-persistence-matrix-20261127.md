# Cache persistence matrix and Date implementation — 27 November 2026

Original task: [1220544665232085](https://app.asana.com/0/0/1220544665232085). Base: `a7631a04b36e3a4ed0dffbb919d6508252024365`.

This is a bounded implementation and local qualification record. The original cache task remains open. No Apple runtime, physical iPad, cross-run persistence, signing, final 179 acceptance or release result is granted by this change.

## Implemented Date archive

Both frontends now admit a present, source-typed Date result. The shared cache lifecycle still captures Previous before helpers and restores it immediately before the lazy miss body. TTL=0 emits only the body and original-value forwarding; no storage, clock or Date archive helpers are added.

For a positive TTL, Date uses a distinct `cherri.cache.v1.<scope>.date.<key-hash>` namespace and `typeTag: date`. Its value field is a native Number of seconds from the fixed UTC epoch. The existing seven-field envelope, version, TTL, post-body creation time, scope, sensitive-value opt-in and stale/changed-policy checks remain in force.

The new codec uses the existing canonical `backend.QuantityValue` adapter for Adjust Date. A dynamic magnitude is the original bare native reference, matching the independently established upstream quantity contract; it is not a formatted Go string or a fabricated text-token quantity wrapper.

Before writing, the codec checks one actual Date item, one numeric offset, a reconstructed Date and the native Date equality predicate. A miss continues returning its original snapshotted Date reference. On a hit, the numeric archive reconstructs a Date, and type/cardinality plus reverse-offset checks must pass before the hit flag is set. No numeric tolerance is used. Native operator precision and the Date equality predicate still need actual runtime qualification; generated action graphs alone do not establish them. Native parser/Date/storage errors retain native abort behavior. The read path validates a single numeric archive value but has no proven finite or native Date range predicate. Adjust Date therefore runs before the reconstruction guards, and an out-of-range or otherwise invalid numeric archive may abort natively; it is not guaranteed to become a recoverable miss.

V2 also preserves authored reference coercions through the existing canonical semantic adapter, including CurrentDate, action results and mutable references. Legacy retains its conservative source-type admission for transformed references.

Existing scalar Text/Number/Bool archive branches retain their previous serialization and lifecycle. Their original cache, scope, Previous and corruption tests were included in the actual focused regression run.

## Requested support matrix

| Source value | Current implementation | Remaining qualification or implementation |
| --- | --- | --- |
| Text, including empty/Unicode | Existing v1 Base64 UTF-8 envelope, canonical roundtrip guards | Actual cache fresh miss/hit/expiry, missing key, cross-run/restart and physical iPad tests |
| Number, including zero/fractions | Existing native numeric archive | Same native lifecycle and corruption tests |
| Bool, including false | Existing explicit 0/1 scalar contract | Same native lifecycle; no inference from unrelated stored Text proof |
| Date | New epoch-offset archive and native reconstruction guards in both frontends | Before-epoch sign, fractional precision, exact Date semantics, miss/hit and cross-run execution on Apple |
| Dictionary/Record | Rejected by cache admission | A recursive persistence codec must validate supported values and reconstruct their actual source/native shape; the normal argument codec does not establish persistence |
| List, including nested/mixed/empty | Rejected by cache admission | Preserve order/cardinality and the existing native-vs-cell representation; direct native content serialization can collapse empty/nested content |
| Optional/None | Rejected by cache admission | Archive and restore a separate presence bit plus value. The current cache callback returns one reference; treating no content as empty Text or null would lose semantics |
| JsonNull/JsonValue | Rejected by cache admission | Validate and archive the source-owned tagged value algebra, including present null separately from absence; private cell transport is not automatically a persistence format |
| File | Rejected by cache admission | Prove real returned File class and exact bytes/name/required metadata across storage and Base64 reconstruction. Do not substitute Text or a filename for a File |
| Other native objects | Rejected by cache admission | Require an object-specific codec and native evidence; no arbitrary-object promise |

The Dictionary/List, Optional/JsonNull and File boundaries were investigated against `types/tagged_transport.go`, `lower/tagged_values.go`, the canonical parameter/quantity adapters and the current export File materializer. Those existing facilities serve in-process value or argument transport. None is presently a qualified general persistence codec. No private carrier is exposed to a native consumer as ordinary user data.

## Native probe packet

The following synthetic sources are included:

- `tests/language-v2/cache-date-native-v2.cherri`
- `tests/language-v2/cache-date-native-legacy.cherri`

Each source exercises four real cache calls: a miss and a hit for a pre-epoch fractional Date, followed by a miss and a hit for a modern fractional Date. Each miss body increments a counter; the hit's alternate body would return a different Date and increment the counter again. The final output includes the counter and independent formatted original/first/hit timestamps and native type labels. Expected: one body execution per key; first/hit timestamps and types match the original exactly.

The fixture deliberately seeds an invalid `{}` envelope at each test key, then cleans up those same keys. It tests **seeded-invalid miss → hit**, not a fresh missing-key result. The original missing-key proof remains a separate required gate. It does not prove persistence across external invocations, expiry, background execution or physical hardware.

Use the established Apple owner/harness and a unique run identity. Retain failures, original signed inputs, output bytes and callback provenance. A successful XCTest callback without the exact result is not acceptance. Do not replace the existing FN04 input/output investigation with this probe.

## Actual local evidence

Evidence directory:

`/mnt/development/projects/agent-workspaces/cherri-macro-completion-20261127/cache-evidence`

1. `date-red.jsonl` records three new named Date failures on unchanged base production, alongside seven unsupported-type control passes.
2. `date-schema-backend-green.jsonl` records 45 named package passes for the initial implementation. This was before the canonical quantity correction and is not the final Date proof.
3. `date-root-green.jsonl` records the first compiler cohort: 236 named passes and two named failures (a newly added Date mutation subtest and its parent). Its mutation had selected an Adjust Date parameter instead of the intended condition; the failure is retained.
4. Inspection of the pre-existing `quantity_reference_test.go` exposed the first Date proposal's wrong quantity wrapper. `date-quantity-red.jsonl` records the strengthened oracle rejecting that proposal. Production was then changed to the canonical `QuantityValue` adapter.
5. `date-final-green.jsonl` records **298 named passes, 0 failures, 0 skips** in 42.16 seconds: all existing Cache tests, Date compile/graph/corruption tests, schema/backend tests and dynamic quantity regressions.
6. `date-native-source-green.jsonl` records **33 named passes, 0 failures, 0 skips** in 68.59 seconds, qualifying additional read-offset cardinality/type guards and the two native probe source files. This is a distinct Date-only run, not 33 additional unique tests.

7. The final V2 source-reference correction reuses the existing canonical adapter instead of dropping authored coercions. The first root run, date-coercion-green.jsonl, failed because its requested graph-output directory had not been created; the original receipt and failures are retained. With that directory created, date-coercion-green02.jsonl records **246 named passes, 0 failures, 0 skips** in 9.15 seconds across all root TestP15Cache tests, including new CurrentDate/date/text, action-result Date and mutable Date coercion cases.

The read-guard changes were Date-only; the final root cache cohort also requalifies scalar paths after the canonical reference adaptation. Schema/backend and quantity evidence remain distinct, reusable runs. These counts must not be added as though they were disjoint test sets.

## Concurrency, errors and release boundaries

Stored Content remains the existing Apple backend; no duplicate storage service is introduced. There is no new compare-and-set, lock or atomicity claim. Concurrent writers may overwrite the same entry. Native I/O, permission and parse failures are not turned into catchable errors; cancellation still follows the actual native action behavior.

Keep this task open until its original typed matrix, native/persistence cases, final consumer tuple and required physical-device evidence are complete. Passing this component's local checks does not reduce those requirements.
