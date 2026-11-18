# Exact native action envelopes

The existing raw escape can now carry the whole native action dictionary. This preserves unknown fields beside `WFWorkflowActionIdentifier` and `WFWorkflowActionParameters`, as well as the existing exact parameter payload.

This is needed when Apple or a third-party action adds an outer field that Cherri does not model. Keeping only the identifier and parameters would discard that field during import and rebuild. The importer now chooses the explicit envelope form for such actions; ordinary raw actions retain their existing source form.

## V2 source

```cherri
native.action("com.example.future", envelope: native.plist({
    "WFWorkflowActionIdentifier": "com.example.future",
    "WFWorkflowActionParameters": {
        "UUID": "11111111-1111-4111-8111-111111111111",
        "Payload": "synthetic"
    },
    "FutureOuterField": {"enabled": false}
}))
```

The first identifier and the envelope identifier must agree. The envelope must use `native.plist`, and its parameters field must either be absent or be a dictionary. Supplying both an envelope and a separate parameters argument is an error. Other catch-all parameter arguments cannot be mixed into an envelope.

A bound output uses the same existing canonical output-identity checks as exact raw parameters. An actually consumed output needs one real, unambiguous native producer UUID. Binding does not insert a UUID into an exact envelope or rename its output. Unused exact data and preexisting opaque native references retain their original wire data.

## Legacy source

The existing `rawAction` gains one optional third argument:

```cherri
rawAction("com.example.future", nil, rawPlist("<plist version=\"1.0\"><dict><key>WFWorkflowActionIdentifier</key><string>com.example.future</string><key>FutureOuterField</key><false/></dict></plist>"))
```

The second argument must be omitted with `nil` when the exact envelope is supplied. The third argument must be a `rawPlist` dictionary. Existing one-argument and two-argument calls keep their behavior, including ordinary parameter keys named `envelope`, `WFWorkflowActionIdentifier`, or `WFWorkflowActionParameters`.

Legacy assignment of this new exact-envelope form is rejected with `NATIVE_ACTION_ENVELOPE_BINDING`; use an explicit native reference to an existing UUID. This restriction prevents the Legacy assignment path from rewriting exact source identity. It does not change existing two-argument raw-action bindings.

## Preservation boundary

The existing `NativeActionRecord`, IR node, canonical backend, and `ShortcutAction` serializer carry this data. There is no second emitter or semantic metadata reconstruction. Both XML and binary input, both hybrid and lossless imports, both language frontends, and the older map-based lossless renderers preserve the full action.

Unknown outer fields are already present in the compiled actions before the import overlay audit hook runs. A source edit to an outer field is emitted from the edited source. Full-workflow native escapes also preserve outer action fields when an ordinary action follows them.

Absence of the parameters field remains distinct from an empty dictionary. The exact plist path retains ordered arrays, false, zero, empty values, integers above 2^53, binary data, dates, and original UUID spelling. Compiler-private output handles cannot escape through an unknown outer field. A literal string that happens to match a private handle remains ordinary data.

## Validation and limits

The source tests exercise actual serialized compilation and import pipelines:

- `TestP15R01UnknownNativeFieldsRoundTrip` reproduces the original loss and covers XML/binary × hybrid/lossless × Legacy/V2.
- `TestP15NativeActionEnvelope*` covers explicit presence, old reserved-key parameters, malformed or conflicting envelopes, pre-overlay capture, compatibility renderers, full-workflow append/edit behavior, and private-identity rejection.
- `TestP15R05RawExportsAndInvalidBoundary` and `TestP15R05OpaqueRegionsAndWorkflowFallback` cover real output references, missing/ambiguous authored outputs, atomic opaque control markers, outward region references, and unchanged preexisting invalid raw input.
- The existing raw-result, native escape, hybrid, lossless, and control-region regression tests remain required.
- `go run ./tools/language-schema` and `--verify-generated` keep the canonical registry synchronized; the registry entry count is unchanged.

These are local source, import, and serialized native-format proofs. They do not claim that an unknown action exists on a device, that an opaque action succeeds at runtime, or that an arbitrary preexisting native reference is valid. Formal acceptance is recorded separately by the named production producers and their source-bound receipts.
