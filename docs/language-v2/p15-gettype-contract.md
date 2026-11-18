# P10R15 getType contract correction

Prepared 2026-11-18. The existing `getType` alias remains available and keeps
its native identifier, input attachment and optional file-type parameter.

## Native contract

The checked-in Apple specimen at
`docs/apple-runtime/macos-27/builtin-actions.json` identifies two different
actions. The tests locate them by native action class, independently of
Cherri's alias names and generated schema:

| Cherri alias | Apple identifier | Native class | Native output |
| --- | --- | --- | --- |
| `getType` | `is.workflow.actions.gettypeaction` | `WFGetTypeAction` | `public.data`, named File of Type |
| `typeOf` | `is.workflow.actions.getitemtype` | `WFGetItemTypeAction` | `NSString`, named Type |

The `getType` input key is `WFInput`. Its optional UTI is `WFFileType`; Apple's
recorded default is `public.rtf`. Cherri continues to omit that key when the
argument is omitted and emits the supplied UTI when present. This change
does not inject a new default or replace file content with a type-name string.

## Correction

The canonical declaration in `actions/basic.cherri` now describes **Get File
of Type** and declares Legacy output `variable`. The existing language-schema
facet mechanism provides the more precise V2 output category `File`.
No unsupported Legacy `file` keyword was introduced. `typeOf` remains the
separate Text-producing inspection action.

The generated registry and action reference were regenerated from the live
compiler catalog. The historical parity report's incorrect prose identifier
was corrected with an explicit dated note; historical runtime claims were
not promoted to current evidence.

## Cardinality limits

Apple records `Input.Multiple = true` and `Output.Multiple = true` for both
actions. The current schema's `File` and `Text` output categories describe
content types; they do not encode collection cardinality or promise exactly
one item. The tests include a collection input and verify the existing File
category without claiming `List<File>` inference or a particular result count.
Conversion success, actual output cardinality and device behavior still
require native runtime evidence.

## Causal verification

`TestP15GetTypeCatalogMatchesAppleFileMetadata` failed before the correction
because the canonical catalog advertised Get Type/Text and the semantic
registry also advertised Text. `TestP15GetTypeInferencePreservesNativeFileCategory`
independently showed that both the action result and `require` passthrough
were inferred as Text for native ShortcutInput and collection input
(SentinelX receipt `job_21dace8012e4`).

After correction, `job_ecbea0d4e818` passed all three getType producers and the
existing independent require/typeOf native-metadata check:

```sh
go test -count=1 -timeout 90s . -run '^(TestP15GetType|TestP15RequireTypeInspectionMatchesCanonicalAndAppleMetadata$)'
```

The four Legacy/V2 wire cases check omitted and explicit UTI arguments, the
original input producer UUID, and the file result's direct attachment to
Quick Look. A wrong-input UUID mutation is rejected. This proves compilation
and parameter preservation; it does not execute Quick Look or file conversion.

`job_9d01be0c5577` verified generated-source determinism and passed the complete
schema, analyzer, schema-generator and reference-generator packages:

```sh
go run ./tools/language-schema --verify-generated
go test -count=1 -timeout 90s ./internal/language/schema ./internal/language/analysis ./tools/language-schema ./tools/language-docs
```

## Consumer compatibility

The actual `--capabilities-json` command reports:

- Public native-catalog entries: **505**.
- Separate compiler macros: **13**.
- Native schema fingerprint: `6eb0ef0129bba916af7cfdeae85bdbb2e42245390ccaa1137670aeb46cb62cce`.
- Compiler macro fingerprint, unchanged: `f63d8b0a323c7045af4abf20723684506c9f63b9d31776a408c39766e7c2cf44`.
- Codec fingerprint, unchanged: `3e7e3555eefe1e030d254df0a22c62fdc3a139f92e39da34f6d841f8bfa1689e`.

The native fingerprint changes because the output category was corrected.
Consumer compatibility manifests must be repinned to a verified compiler
containing this correction before final packaging and consumer CI. Historical
receipts retain their original source and schema identities.
