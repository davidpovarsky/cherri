# Macro contract coverage

All eleven existing Asana children remain OPEN.

This generated ledger records actual synthetic CLI compilation, native graph checks and exact lossless native-action roundtrip. It does not establish signed runtime, physical-device or installed-consumer acceptance.

- Compiler source: 0e577e2476b7097479f5d2f50d8e3ee2ce7d2571
- Source fingerprint: 546019943f2ea38f08579148eb94d58f96161651fad09f56edc80d347264a596
- Macro fingerprint: 1fe7f7993e2473aec591341712f42c521f913974b8e84091a5c2950232230dec
- Canonical compiler macros: 15
- Structural execution: PASS
- Compile checks: 19/19
- Causal mutation checks: 8

| Requirement | Asana child | Implementation observation | Executed cases | Status |
|---|---|---|---:|---|
| Typed require | 1220544599477512 | PARTIAL_IMPLEMENTATION | 4 | OPEN |
| debugLog labels | 1220544718902445 | PARTIAL_IMPLEMENTATION | 0 | OPEN |
| Local debugExport | 1220544665232213 | PARTIAL_IMPLEMENTATION | 0 | OPEN |
| Shared typed block facility | 1220544718901241 | PARTIAL_IMPLEMENTATION | 7 | OPEN |
| Measure typed value | 1220544665232149 | PARTIAL_IMPLEMENTATION | 4 | OPEN |
| Typed collection | 1220544651247337 | EXISTING_V2_CONSTRUCT | 4 | OPEN |
| Persistent TTL cache | 1220544665232085 | IMPLEMENTATION_GAP_AND_NATIVE_PREREQUISITE | 0 | OPEN |
| Bounded retry | 1220544651247273 | IMPLEMENTATION_GAP_AND_NATIVE_PREREQUISITE | 0 | OPEN |
| Bounded pagination | 1220544665232021 | BOUNDED_GET_IMPLEMENTATION | 4 | OPEN |
| Debug and Release | 1220544607463126 | PARTIAL_IMPLEMENTATION | 8 | OPEN |
| Macro QA and consumers | 1220544718902349 | PARTIAL_EVIDENCE | 19 | OPEN |

## Actual controlled CLI observations

| Case | Dialect/profile | Native actions | Exact lossless action roundtrip | Result |
|---|---|---:|---|---|
| require_v2_debug | v2/debug | 21 | True | PASS |
| require_v2_release | v2/release | 20 | True | PASS |
| require_legacy_debug | legacy/debug | 21 | True | PASS |
| require_legacy_release | legacy/release | 20 | True | PASS |
| measure_v2_debug | v2/debug | 7 | True | PASS |
| measure_v2_release | v2/release | 3 | True | PASS |
| measure_legacy_debug | legacy/debug | 7 | True | PASS |
| measure_legacy_release | legacy/release | 3 | True | PASS |
| collect_v2_0 | v2/debug | 8 | True | PASS |
| collect_v2_1 | v2/debug | 7 | True | PASS |
| collect_v2_100 | v2/debug | 106 | True | PASS |
| collect_v2_dictionary | v2/debug | 8 | True | PASS |
| measure_legacy_empty_rejected | legacy/debug | - | expected unavailable/invalid diagnostic | PASS |
| measure_legacy_control_value_rejected | legacy/debug | - | expected unavailable/invalid diagnostic | PASS |
| measure_v2_empty_rejected | v2/debug | - | expected unavailable/invalid diagnostic | PASS |
| paginate_v2 | v2/debug | 182 | True | PASS |
| paginate-zero_v2 | v2/debug | 3 | True | PASS |
| paginate_legacy | legacy/debug | 182 | True | PASS |
| paginate-zero_legacy | legacy/debug | 3 | True | PASS |

## Limits and next prerequisites

- Typed require: Full false/zero/null/empty/native-object matrix and native failure/cancel proof are not established by this ledger.
- debugLog labels: Per-entry metadata and bounded source contracts are implemented; signed File-carrier byte identity still requires repair and acceptance. External prerequisites: Native file/object identity and six-mode label acceptance.
- Local debugExport: Local File export source supports default full redaction and explicit Text-only base64 payloads; structured types, dates/labels, selective redaction and native acceptance remain open. QA debugExportTrace is separate. External prerequisites: Apple returned-file/UTF-8/cardinality runtime evidence.
- Shared typed block facility: V2 and Legacy share the once-body/typed-result measure policy. General parameterized/void/early-error blocks and Legacy control-result declarations remain open.
- Measure typed value: Legacy uses a final yield or value-producing call through existing structured tokens. Structural UUID/timer checks are not native elapsed-time, abort or cancellation proof. External prerequisites: Apple timing accuracy and error/cancel observation.
- Typed collection: No standalone collect alias is introduced. Compiled Repeat Results does not prove runtime flattening behavior. External prerequisites: Native empty/mixed/nested collection shape and performance observation.
- Persistent TTL cache: Existing storage definitions are prerequisites, not a cache implementation; current storeValue exposes text. External prerequisites: Stored Content cross-run/restart/global-local readback; Physical iPad persistence/background proof.
- Bounded retry: No evidence establishes catchable transport failures; Alert plus Stop Shortcut is not exception handling. External prerequisites: Recoverable native error/status contract and signed synthetic endpoint probes.
- Bounded pagination: The shared bounded GET/body-cursor implementation has page/item limits and cycle guards. Response headers/status, transport recovery, redirect limits and generic retry remain outside its proven contract. External prerequisites: Signed synthetic multi-page HTTP/schema/error observation.
- Debug and Release: The controlled compiler comparisons cover declared fixtures; they do not establish every diagnostic or runtime path. External prerequisites: Native result equivalence and debug-mode acceptance.
- Macro QA and consumers: This ledger builds and tests a compiler; it does not install a Skill, update a provider or execute on Apple. External prerequisites: Final pinned consumer tuple and signed/physical acceptance evidence.

Standalone collect is deliberately not added. Existing for/yield evidence belongs to the typed collection requirement; it is not counted as a new canonical macro. Numeric input construction precedes the constant-size native loop and is counted separately. Cache and retry have both missing source implementation and native prerequisites. Label and bounded GET pagination have implementations with open native acceptance. Local export has a bounded Text-only/default-redacted source subset; its native acceptance and remaining typed serialization contract stay open.

Reproduce with python3 tools/p10r15/generate_macro_contract_coverage.py --refresh, then --verify. Every refresh builds the source and runs the actual CLI. Verification rejects source/producer drift and edited or incomplete evidence. The source SHA is the compiled revision before committing evidence, avoiding a self-referential commit pin.
