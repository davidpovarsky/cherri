# debugExport payload diagnostics — 2026-11-27

This component implements bounded native payload diagnostics and structural field redaction for original task `1220544665232213`. The original task also requires a real returned local File. The existing Encode → Decode → Set Name carrier is unchanged, and **native File acceptance remains open**. A compiled graph, a `.json` filename, a declared File return type, or this model passing does not establish the native content class.

## Public options and precedence

The existing positional parameters remain in order; `redactKeys` is an optional sixth literal `List<Text>`:

```cherri
debugExport("json", false, true, true, false, [])
// format, includeDates, includeTypes, redactSensitive, includeLabels, redactKeys
```

| redactSensitive | redactKeys | Payload behavior |
| --- | --- | --- |
| omitted or true | omitted or empty | Every payload is `REDACTED`; the original default graph stays unchanged. |
| false | omitted or empty | Native Text, Number presentation, Date presentation, and bounded Dictionary diagnostics are enabled. No sensitive-key policy is inferred. |
| true | nonempty literal list | Only Dictionary field diagnostics are enabled. Protected fields are never fetched; other Text values are still redacted. Top-level Text, Number, Date, and other native values remain redacted. |
| false | nonempty literal list | Payload diagnostics are enabled and protected Dictionary fields are never fetched. Other supported values are disclosed explicitly. |
| either | malformed, dynamic, or over bounds | Compilation fails; the option never silently falls back to disclosure. |

Each nonempty list opts into case-insensitive exact field-name filtering. The supplied names are combined with these built-in names: `authorization`, `proxy-authorization`, `headers`, `cookie`, `cookies`, `set-cookie`, `password`, `passwords`, `secret`, `secrets`, `token`, `access_token`, `refresh_token`, `id_token`, `api_key`, `apikey`, `credentials`, `private_key`, `url`, `uri`, `link`, `query`, `querystring`, and `queryparameters`.

There may be at most 64 entries; each must be nonempty literal Text with at most 128 Unicode code points and no control characters. Duplicate names are folded case-insensitively. Regex metacharacters in a supplied name are escaped. Regex matching examines each enumerated key only; it never rewrites serialized JSON or an object's values.

This is a field-name policy, not detection of every possible secret. Ordinary field names and their diagnostic statuses are exported, with keys encoded as base64 UTF-8. A secret embedded in an unprotected field name is therefore visible. With explicit `redactSensitive:false`, an unprotected free-text field can contain secrets, URLs, headers, or query text. Callers requiring zero payload disclosure must keep the default empty key policy and `redactSensitive:true`.

Dates, types, and labels remain separately controlled metadata. Labels stay redacted whenever `redactSensitive:true`, including with nonempty `redactKeys`. Explicit creation-date metadata is derived from the original item's property; it is never replaced with the current time. A Date payload and an item's Creation Date metadata are different fields.

## Representation and unsupported cases

The exporter observes the native content-item stream already held in `__cherri_debug_log`. It does not mutate that stream, convert the stored objects to Text, change the one-append default `debugLog` expansion, or rebuild source-language types from private compiler representations. Existing label sidecars store item ranges and a label, without original source type data.

| Observed native value | Diagnostic status | Payload |
| --- | --- | --- |
| Text, explicit unredacted mode | `TEXT_PAYLOAD_ENCODED` | base64 UTF-8 Text |
| Native Number | `NATIVE_NUMBER_TEXT_ENCODED` | base64 of native displayed text; `representation: native-number-text` |
| Native Date | `DATE_ISO8601_ENCODED` | base64 of native ISO 8601 formatting with time; `representation: iso8601-native-format` |
| Top-level Dictionary | `DICTIONARY_FIELD_DIAGNOSTICS` | `encoding: diagnostic-fields`, `lossless: false`, an array of key/diagnostic rows |
| Protected Dictionary field | `REDACTED_SENSITIVE_FIELD` | null; value is not fetched |
| Dictionary field containing a dot in its name | `UNSUPPORTED_KEY_PATH` | null; value is not fetched |
| Dictionary field yielding no native items | `ABSENT_NULL_OR_EMPTY_COLLECTION` | null; no distinction is invented |
| Dictionary field yielding several native items | `UNSUPPORTED_MULTIPLE_NATIVE_ITEMS` | null |
| Nested Dictionary value | `UNSUPPORTED_NESTED_DICTIONARY` | null |
| Native Bool, List, null-like object, File, Image, or another unsupported content class | `UNSUPPORTED_NATIVE_PAYLOAD_TYPE` | null |
| Text field under the selective redacted policy | `REDACTED_TEXT_CONTENT` | null |

Native Get Dictionary Value interprets dots as paths. The exporter therefore rejects a dotted enumerated name before lookup; it cannot accidentally export another nested field under the original literal key. Sensitive-name filtering runs before this path guard and before any value read. Nested Dictionaries are not recursively serialized or scanned.

Number presentation is deliberately not a locale-independent JSON number. It may reflect the native locale and formatting. Date presentation uses the established ISO 8601 native action; preservation of arbitrary source precision is not claimed. Both representations are labeled rather than silently advertised as lossless JSON primitives.

A source Bool may already appear as a native Number. A source singleton List may already appear as its one native item. Source null or an empty collection may have appended no native items at all. Neither the raw buffer nor the label sidecar can recover that lost distinction. Every expanded JSON envelope states `sourceTypePolicy: source-types-not-reconstructed`, `collectionPolicy: native-item-stream`, and `nullPolicy: absence-not-reconstructed`. A missing source event cannot acquire a fabricated row; ambiguous Dictionary lookup cardinalities do receive explicit per-field statuses. The text format contains JSON lines and the same scope description.

## Locality and unchanged behavior

The graph uses existing native actions and the existing Dictionary/Text token encoders. It contains no upload, share, save-to-provider, sync, URL-fetch, shell, or notification action. Export variables are private to one expansion. Exporting never appends diagnostics into the user's raw log or label sidecar. Default release stripping and the actual source consumers remain covered by the original export tests.

The default redacted canonical fixtures in `testdata/p15-debug-export-metadata/default/` are unchanged. Historical Text-only unredacted fixtures are retained there as historical evidence; the expanded opt-in behavior has separately named baselines in `testdata/p15-debug-export-payloads/unredacted/`. The comparison remains exact for each applicable baseline, and existing fault controls are retained.

Metadata and payload expansion identities use the central generated-UUID sequence. This preserves random identities by default and exact repeatability under `--derive-uuids`, including private variable names and control groups.

## Source and model evidence

Evidence for this lane is saved outside the Git checkout at `../export-evidence/`, alongside preserved original failures. It must not be described as signed Apple runtime evidence.

- Initial additive source RED: 30 named failures for the missing payload/key contracts; GREEN: the same 30 named tests passed on both frontends.
- Original export regression plus additive source cohort: `regression-02-original.jsonl`, 148 named tests passed. Earlier failures and the explicit opt-in baseline transition are retained rather than rewritten.
- `metadata-model-02.json`: 48 actual compiler-emitted graphs, 720 small cases, eight stress cases, and 12 causal fault controls passed.
- `payload-model-01.json`: 16 actual compiler-emitted graphs, 64 cases, two exact 1,000-entry stress cases, and nine causal fault controls passed.
- Original verifier boundary tests remain unchanged and passed. The expanded verifier uses the metadata evaluator through the completed Text envelope, before the File carrier.
- `cli-model-02.json`: the updated historical CLI tool passed 16 compiles, 16 exact native-action roundtrips, 80 model cases, 12 invalid-source checks, seven causal faults, 12 native probe compiles, and four admitted historical date-source cases. These were source/model checks only. The first CLI attempt was interrupted by the tool's default 60-second wrapper timeout during build; `cli-model-01-tool-timeout.json` preserves that infrastructure interruption without treating it as a product test verdict.
- The deterministic-identity regression first failed in all six frontend/option cases. Its RED receipt is `uuid-red-receipt.json`; `uuid-green-original.jsonl` contains ten named passes, including both-frontends compilation of the new Cherri fixture.

The semantic evaluators check the actual emitted action edges, native-type witnesses, source/date bindings, output order, raw object identity/content, field read boundaries, nonlocal effects, and JSON parseability. Fault controls remove redaction, change dynamic keys, bypass dotted-key protection, retarget Date sources, weaken formatting/Base64 settings, duplicate a producer, write to the raw buffer, or insert network work. The nine payload fault controls require rejection after positive cases. The 12 metadata and seven CLI fault controls additionally restore and recheck a passing control.

The model uses explicit synthetic native values. It does not discover what Apple will actually coerce, localize, flatten, omit, or return on a supported device. Both semantic reports state `nativeRuntimeExecuted:false` and `fileClassAccepted:false`. The original File contract must remain open until an authorized signed native run demonstrates the returned class and subsequent File consumer behavior. No speculative plist class or undocumented conversion was added to make that result appear complete.

## Unexecuted native File candidate and probe plan

**Hypothesis only:** create an archive from the completed original Text, extract its single item, then assign the final filename. The production Encode → Decode → Set Name carrier remains unchanged and native File acceptance remains open.

The canonical definitions in `actions/documents.cherri:16–20` already provide `makeArchive(files, ".zip", name)` through `is.workflow.actions.makezip` with `WFInput`, `WFArchiveFormat`, and `WFZIPName`, and `extractArchive(file)` through `is.workflow.actions.unzip` with `WFArchive`. Their entries in `docs/apple-runtime/macos-27/builtin-actions.json` declare `WFGenericFileContentItem` outputs: one archive, then multiple extracted files. Neither action requires a provider destination. These declarations do not prove actual File class, exact UTF-8 bytes, or naming behavior.

Preserve the original failures recorded in `docs/apple-runtime/p10r15/debug-export-contract.md`: run `44466065158` observed `TYPE_FILE=N`, `TYPE_DICT=Y`, and `COERCED_FILE=N`; Get File of Type run `44334287258` failed. Run `44429796917` established bytes/name and relative type preservation, not independent File acceptance.

A separately named `qa-file-archive-materialization-v2` experiment should compile original Text → Make Archive → Extract Archive → count-one check → first item → Set Name. Record the existing carrier baseline independently. Use fixed synthetic JSON with significant whitespace, numeric spelling, escaped characters, Unicode, and a trailing newline; independently fix its expected bytes. Require exactly one extracted item, native class observations before/after naming, exact Base64 UTF-8 bytes, and reconstructed Name plus File Extension. Preserve the original independent binary/Text/Dictionary type witnesses and alias checks. Observe the candidate before conversion consumers; retain failed observations unchanged.

Hosted routing remains blocked: `.github/workflows/cherri-ios27-shortcuts-simulator-qa.yml` has a closed `sample_id` choice, and `scripts/ios27_shortcuts_simulator_qa.sh` separately allowlists selectors. The original file diagnostic binds its immutable fixture hash; `all` selects `qa-functional-v2`. The current GitHub credential rejected workflow changes for missing OAuth workflow scope. No original case may be repurposed.

A separate macOS diagnostic route exists: unchanged `tools/shortcut-runtime/jxa/observe_original_action_values.js` constructs a transient, save-disabled workflow from the original compiler-emitted action array. `workflowkit-stored-values-20261127.json` and its retained originals bind the ShortcutKit prelude at `ee52c32937053268e4a035eba4b372fe77ec6411`, existing owner/process wrapper, and prior Text observation. Preserve its 16-action, 8-second engine, 20-second command, 60-second owner, and 1 MiB stream limits, source hashes, create-only records, and owned-process cleanup. Fresh owner admission and graph effect review are required; the observer itself does not restrict action effects. Archive temporary-file behavior remains unobserved. This investigation executed no candidate, created no fixture/runner, and changed no workflow. Any macOS result remains separate from original signed/iPad gates.

## Reproduction

```sh
CHERRI_EXPORT_PAYLOAD_GRAPHS=/absolute/artifacts/payload-graphs \
  go test -count=1 -run '^TestP15DebugExport(TypedPayloadSource|SelectiveFieldsSource)$' .
python3 tools/p10r15/verify_debug_export_payloads.py \
  --graphs /absolute/artifacts/payload-graphs --report /absolute/artifacts/payload-report.json
CHERRI_EXPORT_METADATA_GRAPH_OUT=/absolute/artifacts/metadata-graphs \
  go test -count=1 -run '^TestP15DebugExportMetadataSourceGraphs$' .
python3 tools/p10r15/verify_debug_export_metadata.py \
  --graphs /absolute/artifacts/metadata-graphs --report /absolute/artifacts/metadata-report.json
go test -count=1 -run '^TestP15DebugExport(DerivedUUIDReplay|PayloadCherriFixture)$' .
```

`tests/debug-export-payload-diagnostics.cherri` is a real compile-only fixture for the default repository test runner. It uses synthetic values, keeps the original Dictionary available after export, and does not assert native File acceptance. All native signing/import/runtime work is owned by the separately coordinated Apple gate.
