# Macro contract coverage

All eleven existing Asana children remain OPEN.

This generated ledger records actual synthetic CLI compilation, native graph checks and exact lossless native-action roundtrip. It does not establish signed runtime, physical-device or installed-consumer acceptance.

- Compiler source: 54d6457295ef4c6bf106f743cc2b93ca32735760
- Source fingerprint: 8f9d4c675199ff288ead426c3f7f43d9b415993fae47494d844be8254f3dc578
- Macro fingerprint: ce784c321463b31e50773d31ae44b660b5afad32e7a49e9435ade5d8b593cc7e
- Canonical compiler macros: 16
- Structural execution: PASS
- Compile checks: 21/21
- Causal mutation checks: 56
- Separate label compile checks: 12
- Separate label causal mutation checks: 314

| Requirement | Asana child | Implementation observation | Executed cases | Status |
|---|---|---|---:|---|
| Typed require | 1220544599477512 | PARTIAL_IMPLEMENTATION | 4 | OPEN |
| debugLog labels | 1220544718902445 | PARTIAL_IMPLEMENTATION | 12 | OPEN |
| Local debugExport | 1220544665232213 | PARTIAL_IMPLEMENTATION | 0 | OPEN |
| Shared typed block facility | 1220544718901241 | PARTIAL_IMPLEMENTATION | 9 | OPEN |
| Measure typed value | 1220544665232149 | PARTIAL_IMPLEMENTATION | 8 | OPEN |
| Typed collection | 1220544651247337 | EXISTING_V2_CONSTRUCT | 4 | OPEN |
| Persistent TTL cache | 1220544665232085 | DECLARED_BUT_NOT_EXERCISED | 0 | OPEN |
| Bounded retry | 1220544651247273 | IMPLEMENTATION_GAP_AND_NATIVE_PREREQUISITE | 0 | OPEN |
| Bounded pagination | 1220544665232021 | BOUNDED_GET_IMPLEMENTATION | 4 | OPEN |
| Debug and Release | 1220544607463126 | PARTIAL_IMPLEMENTATION | 8 | OPEN |
| Macro QA and consumers | 1220544718902349 | PARTIAL_EVIDENCE | 33 | OPEN |

## Actual controlled CLI observations

| Case | Dialect/profile | Native actions | Exact lossless action roundtrip | Result |
|---|---|---:|---|---|
| require_v2_debug | v2/debug | 22 | True | PASS |
| require_v2_release | v2/release | 21 | True | PASS |
| require_legacy_debug | legacy/debug | 21 | True | PASS |
| require_legacy_release | legacy/release | 20 | True | PASS |
| measure_v2_debug | v2/debug | 10 | True | PASS |
| measure_v2_release | v2/release | 4 | True | PASS |
| measure_legacy_debug | legacy/debug | 9 | True | PASS |
| measure_legacy_release | legacy/release | 3 | True | PASS |
| collect_v2_0 | v2/debug | 9 | True | PASS |
| collect_v2_1 | v2/debug | 8 | True | PASS |
| collect_v2_100 | v2/debug | 107 | True | PASS |
| collect_v2_dictionary | v2/debug | 9 | True | PASS |
| measure_legacy_empty_rejected | legacy/debug | 7 | True | PASS |
| measure_legacy_control_value_rejected | legacy/debug | - | expected unavailable/invalid diagnostic | PASS |
| measure_v2_empty_rejected | v2/debug | - | expected unavailable/invalid diagnostic | PASS |
| measure_v2_empty_statement | v2/debug | 7 | True | PASS |
| measure_legacy_empty_binding_rejected | legacy/debug | - | expected unavailable/invalid diagnostic | PASS |
| paginate_v2 | v2/debug | 183 | True | PASS |
| paginate-zero_v2 | v2/debug | 4 | True | PASS |
| paginate_legacy | legacy/debug | 182 | True | PASS |
| paginate-zero_legacy | legacy/debug | 3 | True | PASS |

measure_legacy_empty_rejected: Original rejected suffix retained; exact source is now checked as a legal Void statement.

## Separate label CLI component

These twelve reviewed fixtures use the same freshly built compiler. Their existing helper verifies real graphs and 314 causal mutations. They are separate from the 21 cases and 56 mutations above; no label roundtrip, signed execution or native value/cardinality result is claimed. Native execution remains NOT_RUN and formal acceptance remains false.

| Reviewed fixture | Native actions in unsigned graph | Rejected causal mutations | Native execution |
|---|---:|---:|---|
| qa-debug-label-plain-v2 | 171 | 19 | NOT_RUN |
| qa-debug-label-dated-v2 | 177 | 21 | NOT_RUN |
| qa-debug-label-typed-v2 | 203 | 26 | NOT_RUN |
| qa-debug-label-dated-typed-v2 | 212 | 28 | NOT_RUN |
| qa-debug-label-text-v2 | 204 | 26 | NOT_RUN |
| qa-debug-label-dated-text-v2 | 212 | 28 | NOT_RUN |
| qa-debug-label-dated-text-legacy | 219 | 31 | NOT_RUN |
| qa-debug-label-dated-typed-legacy | 219 | 31 | NOT_RUN |
| qa-debug-label-dated-legacy | 184 | 24 | NOT_RUN |
| qa-debug-label-plain-legacy | 178 | 22 | NOT_RUN |
| qa-debug-label-text-legacy | 211 | 29 | NOT_RUN |
| qa-debug-label-typed-legacy | 210 | 29 | NOT_RUN |

## Limits and next prerequisites

- Typed require: Full false/zero/null/empty/native-object matrix and native failure/cancel proof are not established by this ledger.
- debugLog labels: Per-entry metadata and bounded source contracts are implemented; signed File-carrier byte identity still requires repair and acceptance. External prerequisites: Native file/object identity and six-mode label acceptance.
- Local debugExport: Local export source preserves full default redaction. Explicit disclosure supports Text, native Number display text, ISO 8601 Dates and bounded Dictionary field diagnostics; nonempty literal redactKeys filters fields before value access. Source Bool/null/List boundaries, nested containers, native Date/label behavior and returned File acceptance remain open. Separate feature tests do not grant runtime credit to this ledger; QA debugExportTrace is separate. External prerequisites: Apple returned-file/UTF-8/cardinality runtime evidence.
- Shared typed block facility: V2 and Legacy share once-only typed-value and Void measure bodies with explicit empty-result boundaries. General parameterized blocks, control-result declarations and full early-error/cancellation semantics remain open.
- Measure typed value: Legacy uses a final yield or value-producing call through existing structured tokens. Structural UUID/timer checks are not native elapsed-time, abort or cancellation proof. External prerequisites: Apple timing accuracy and error/cancel observation.
- Typed collection: No standalone collect alias is introduced. Compiled Repeat Results does not prove runtime flattening behavior. External prerequisites: Native empty/mixed/nested collection shape and performance observation.
- Persistent TTL cache: Shared source supports Text/Number/Bool and guarded Date epoch-offset archives. Dictionary/List/Optional/JsonNull/File persistence remains unsupported. This ledger does not exercise persistence, payload round trips, expiry, invalidation or native error behavior; invalid numeric Date archives may abort in native Adjust Date before reconstruction guards. External prerequisites: Stored Content cross-run/restart/global-local readback; Physical iPad persistence/background proof.
- Bounded retry: No evidence establishes catchable transport failures; Alert plus Stop Shortcut is not exception handling. External prerequisites: Recoverable native error/status contract and signed synthetic endpoint probes.
- Bounded pagination: The shared bounded GET implementation has page/item limits, cycle guards, page callbacks and literal Dictionary/Text/Number/Bool item selection with optional key projection. These ledger cases exercise the default Dictionary route only; separate typed-selection tests do not establish native list shape. Response headers/status, transport recovery, redirect limits and generic retry remain outside its proven contract. External prerequisites: Signed synthetic multi-page HTTP/schema/error observation.
- Debug and Release: The controlled compiler comparisons cover declared fixtures; they do not establish every diagnostic or runtime path. External prerequisites: Native result equivalence and debug-mode acceptance.
- Macro QA and consumers: This ledger builds and tests a compiler; it does not install a Skill, update a provider or execute on Apple. External prerequisites: Final pinned consumer tuple and signed/physical acceptance evidence.

Standalone collect is deliberately not added. Existing for/yield evidence belongs to the typed collection requirement; it is not counted as a new canonical macro. Numeric input construction precedes the constant-size native loop and is counted separately. Cache and retry are outside this ledger's executed fixtures; their source and native acceptance require separately scoped evidence. Label and bounded GET pagination have implementations with open native acceptance. Local export has a bounded Text-only/default-redacted source subset; its native acceptance and remaining typed serialization contract stay open.

Reproduce with python3 tools/p10r15/generate_macro_contract_coverage.py --refresh, then --verify. Every refresh builds the source and runs the actual CLI. Verification rejects source/producer drift and edited or incomplete evidence. The source SHA is the compiled revision before committing evidence, avoiding a self-referential commit pin.
