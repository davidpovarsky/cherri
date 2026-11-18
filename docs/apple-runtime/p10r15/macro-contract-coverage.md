# Macro contract coverage

All eleven existing Asana children remain OPEN.

This generated ledger records actual synthetic CLI compilation, native graph checks and exact lossless native-action roundtrip. It does not establish signed runtime, physical-device or installed-consumer acceptance.

- Compiler source: ab9a264ba370890397d74042e00c11a3b4c84ffd
- Source fingerprint: 3c0f17d19dae114719cf22deed78044768ea785044b20246b258422b928b862f
- Macro fingerprint: f63d8b0a323c7045af4abf20723684506c9f63b9d31776a408c39766e7c2cf44
- Canonical compiler macros: 13
- Structural execution: PASS
- Compile checks: 12/12
- Causal mutation checks: 5

| Requirement | Asana child | Implementation observation | Executed cases | Status |
|---|---|---|---:|---|
| Typed require | 1220544599477512 | PARTIAL_IMPLEMENTATION | 4 | OPEN |
| debugLog labels | 1220544718902445 | IMPLEMENTATION_GAP | 0 | OPEN |
| Local debugExport | 1220544665232213 | IMPLEMENTATION_GAP | 0 | OPEN |
| Shared typed block facility | 1220544718901241 | PARTIAL_IMPLEMENTATION | 4 | OPEN |
| Measure typed value | 1220544665232149 | PARTIAL_IMPLEMENTATION | 3 | OPEN |
| Typed collection | 1220544651247337 | EXISTING_V2_CONSTRUCT | 4 | OPEN |
| Persistent TTL cache | 1220544665232085 | IMPLEMENTATION_GAP_AND_NATIVE_PREREQUISITE | 0 | OPEN |
| Bounded retry | 1220544651247273 | IMPLEMENTATION_GAP_AND_NATIVE_PREREQUISITE | 0 | OPEN |
| Bounded pagination | 1220544665232021 | IMPLEMENTATION_GAP | 0 | OPEN |
| Debug and Release | 1220544607463126 | PARTIAL_IMPLEMENTATION | 6 | OPEN |
| Macro QA and consumers | 1220544718902349 | PARTIAL_EVIDENCE | 12 | OPEN |

## Actual controlled CLI observations

| Case | Dialect/profile | Native actions | Exact lossless action roundtrip | Result |
|---|---|---:|---|---|
| require_v2_debug | v2/debug | 21 | True | PASS |
| require_v2_release | v2/release | 20 | True | PASS |
| require_legacy_debug | legacy/debug | 21 | True | PASS |
| require_legacy_release | legacy/release | 20 | True | PASS |
| measure_v2_debug | v2/debug | 7 | True | PASS |
| measure_v2_release | v2/release | 3 | True | PASS |
| collect_v2_0 | v2/debug | 6 | True | PASS |
| collect_v2_1 | v2/debug | 6 | True | PASS |
| collect_v2_100 | v2/debug | 6 | True | PASS |
| collect_v2_dictionary | v2/debug | 6 | True | PASS |
| measure_legacy_unavailable | legacy/debug | - | expected unavailable/invalid diagnostic | PASS |
| measure_v2_empty_rejected | v2/debug | - | expected unavailable/invalid diagnostic | PASS |

## Limits and next prerequisites

- Typed require: Full false/zero/null/empty/native-object matrix and native failure/cancel proof are not established by this ledger.
- debugLog labels: Per-entry label metadata, formatting, branch/repeat association and six-mode acceptance are missing.
- Local debugExport: QA debugExportTrace is a separate trace helper and does not fulfill this contract. External prerequisites: Apple file-provider/permission and native output evidence.
- Shared typed block facility: The implemented V2 measure value block is a bounded subset; general void/early-error blocks and a safe Legacy route remain open.
- Measure typed value: Legacy block syntax is deliberately rejected; structural UUID/timer checks are not native elapsed-time proof. External prerequisites: Apple timing accuracy and error/cancel observation.
- Typed collection: No standalone collect alias is introduced. Compiled Repeat Results does not prove runtime flattening behavior. External prerequisites: Native empty/mixed/nested collection shape and performance observation.
- Persistent TTL cache: Existing storage definitions are prerequisites, not a cache implementation; current storeValue exposes text. External prerequisites: Stored Content cross-run/restart/global-local readback; Physical iPad persistence/background proof.
- Bounded retry: No evidence establishes catchable transport failures; Alert plus Stop Shortcut is not exception handling. External prerequisites: Recoverable native error/status contract and signed synthetic endpoint probes.
- Bounded pagination: A bounded GET/body-cursor subset can use existing loops; generic response-status or retry behavior must not be invented. External prerequisites: Signed synthetic multi-page HTTP/schema/error observation.
- Debug and Release: The controlled compiler comparisons cover declared fixtures; they do not establish every diagnostic or runtime path. External prerequisites: Native result equivalence and debug-mode acceptance.
- Macro QA and consumers: This ledger builds and tests a compiler; it does not install a Skill, update a provider or execute on Apple. External prerequisites: Final pinned consumer tuple and signed/physical acceptance evidence.

Standalone collect is deliberately not added. Existing for/yield evidence belongs to the typed collection requirement; it is not counted as a new canonical macro. Cache and retry have both missing source implementation and native prerequisites. Label, export and pagination still require ordinary implementation work.

Reproduce with python3 tools/p10r15/generate_macro_contract_coverage.py --refresh, then --verify. Every refresh builds the source and runs the actual CLI. Verification rejects source/producer drift and edited or incomplete evidence. The source SHA is the compiled revision before committing evidence, avoiding a self-referential commit pin.
