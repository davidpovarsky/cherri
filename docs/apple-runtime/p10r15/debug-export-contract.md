# Local debug export: bounded source implementation

Status: **PARTIAL SOURCE IMPLEMENTATION; NATIVE ACCEPTANCE OPEN**. Asana
[1220544665232213](https://app.asana.com/1/1219597879016953/project/1219603572114432/task/1220544665232213)
remains open. This implements a small, explicit part of the export contract.
It does not replace the separate QA callback helper, debugExportTrace.

## Supported source contract

Both frontends expand the same implementation into existing native actions:

- debugExport() returns one named File, cherri-debug.json.
- format:"text" returns cherri-debug.txt containing a human-readable header and
  one JSON record per native log item.
- Payloads are completely redacted by default. No token, header, URL query,
  filename, document content, or original payload bytes are exported implicitly.
- Explicit redactSensitive:false exports only a native Text item, represented
  as {"encoding":"base64-utf8","value":"..."}.
- All non-Text payloads report UNSUPPORTED_PAYLOAD_TYPE with payload:null.
  Number, Bool, null, Dictionary, List, Date, File and Image are not guessed or
  coerced into Text. Native log flattening can already affect item boundaries;
  this export does not claim to reconstruct original source-level collections.
- includeTypes:true (default) includes the native type label with the same
  explicit base64-utf8 encoding. The label may be localized. A Text payload
  branch compares its type with a separately created native Text witness.
- includeTypes:false omits type metadata. With default redaction this removes
  both the type lookup and base64 operation from each exported row.
- All options are compile-time literals. includeDates:true is rejected with
  DEBUG_EXPORT_DATES_UNSUPPORTED; labels and dates are recorded as not-collected.
  No unverified timestamp or entry association is fabricated.
- Output encodes the completed UTF-8 Text envelope with Base64 Encode and
  decodes those exact bytes into a new File, then uses Set Name. No Save File, provider destination, network, Share, clipboard,
  Stored Content or Run Shortcut action is emitted.
- The raw __cherri_debug_log references remain untouched. A diagnostic export
  does not rename or convert original objects. Existing six display modes are
  unchanged.
- Release removes debugExport just like other diagnostic macros. A source
  consumer of the removed File result is rejected by the existing
  RELEASE_DIAGNOSTIC_VALUE contract.

Start a debug session with debugClearLogs before logging and exporting.
Native behavior of the cleared empty buffer is an explicit pending acceptance
case; source/model checks alone do not establish that native cardinality.

V2 example:

~~~cherri
debugClearLogs()
text("Synthetic diagnostic text")
debugLog()
let artifact = debugExport()
quicklook(artifact)
~~~

The quicklook above is an explicit caller action. The macro itself never previews,
saves, shares, or uploads. An explicit unredacted Text export is:

~~~cherri
let artifact = debugExport(format: "json", redactSensitive: false)
~~~

Legacy uses the same positional contract:
debugExport("json", false, true, false) means JSON, no dates, include types,
and explicitly opted-in Text payloads.

## JSON representation and privacy

The envelope declares schemaVersion cherri.debug-export/1, run-local-native-items
scope, payloadPolicy, dates/labels status, and entries. Each entry has a status
and payload; nativeType is optional. Payload statuses are REDACTED,
TEXT_PAYLOAD_ENCODED, or UNSUPPORTED_PAYLOAD_TYPE. An unsupported payload is
never presented as successful typed serialization.

Base64 is an encoding, not encryption or redaction. The explicit opt-in exports
the complete Text value, including any secrets it contains. Default full
redaction avoids pretending that a limited pattern filter can identify every
sensitive value. This first subset does not offer selective field redaction.

Only Base64 Encode output with line breaks disabled is inserted inside a quoted
JSON field. User quotes, backslashes, line breaks, control characters, Hebrew,
combining marks, emoji and U+FFFC therefore do not require a guessed native JSON
serializer or a chain of per-character escaping actions. An independent JSON
parser and strict base64 decoder verify model results. The native Base64/UTF-8
behavior still needs the signed fixture.

There is no caller-supplied path or filename. The only filenames are
cherri-debug.json and cherri-debug.txt. The returned File stays under the
caller’s control; no persistence is promised.

## Evidence and reproducibility

Run:

~~~text
go test -count=1 -timeout=5m -run '^(TestP15DebugExport|TestDebugStdlibSixDisplayModes|TestDebugStdlibMustNotRewriteExistingProducerUUID|TestDebugStdlibLoggingDoesNotBreakLaterConsumer|TestP15DebugMacroPreservesFrontendResultIdentity|TestP15SignedFixtureExactProducerReferences)' -v .
python3 tools/p10r15/verify_debug_export.py
~~~

Use py -3 -X utf8 on Windows. The producer builds the actual checkout, records
source and compiler hashes plus schema/macro fingerprints, and refuses source
drift during its run. The report is debug-export-validation.json.

The producer independently checks:

- 16 actual CLI configurations across both dialects, both formats, redaction
  on/off and native type metadata on/off.
- Exact import/recompile equality of every native action graph.
- 80 bounded graph-model cases covering 0, 1, 1,000 and empty Text entries,
  control characters/Unicode, and a nine-category type matrix.
- Actual CLI failure/no-artifact behavior for invalid formats, paths, unproved
  dates and dynamic format, in Debug and Release for both dialects.
- Causal corruptions of default redaction, type witness, base64 line breaks,
  file encoding, filename and output UUID, followed by restored controls.
- Standalone synthetic native probes, with exact source-byte checks for
  Hebrew/emoji and exact lossless graph roundtrips.
- Independent bundled Apple metadata for Get Type, Base64 Encode/Decode
  and Set Name. This is macOS metadata evidence, not iPad execution proof.

The Go suite also compiles 0/1/1,000 logged items in both frontends and checks
constant export graph size, immutable original UUID references, returned File
identity, literal option failures, Release behavior and the canonical macro
descriptor. Existing six-mode and signed-fixture graph tests remain controls.

The bounded evaluator is explicitly a model of emitted actions, not an Apple
runtime emulator or a device substitute. Its assumptions are named in the
report. No report or status converts model success into native acceptance.

## Native acceptance still required

The standalone probes live under
tools/shortcuts-simulator-qa/fixtures/p15-export-probes/manifest.json. They are not
registered as replacements for existing signed fixtures. Each has its own
expected verdict, before/after QA trace and explicit source identity. The
existing harness must select one deliberately and enforce its normal
signature, fresh nonce, callback, source and trace checks.

The pending cases are empty-after-clearing a prior record, Unicode Text, 1,000 distinct ordered entries (every position checked),
default secret redaction, unsupported in-memory File with unchanged bytes,
and exact text-file bytes, in both dialects. Their inputs are synthetic and
local. Signing alone does not establish any of these outcomes.

Native acceptance must establish empty-list behavior, Text/UTF-8 bytes and
empty strings, Repeat/If result collection, one newly produced File, its
extension/content, and actual latency/memory on the target platform. The source
action budget is 10 actions for default export, 8 with types omitted, 17 for
opted-in Text with types, and 16 with types omitted. These are compiled graph
counts, not runtime timing measurements.

This first subset leaves structured Dictionary/List/Number/Bool/null/Date
serialization, File/Image metadata or binary opt-in, per-entry dates/labels,
selective redaction and physical iPad acceptance open. No external API or
catchable HTTP error mechanism is required for this local subset.

## Retained failed observations

The first Go format attempt failed because the editing transport had expanded
newline escapes inside Go string literals; it was corrected before the compiler
build and actual CLI checks. A producer run initially rejected the Legacy native
probe because actions/text was not loaded. The producer now uses the existing
--load-all-actions option for standalone native probes. Neither failure was
reported as native success or as a compiler regression.


A causal privacy test caught an initial default-parser error: an explicit nil
redaction flag was treated like false by a shared helper intended for
false-default flags. The added test failed before the correction. Export now
requires actual Boolean literals for every present flag; nil, strings, numbers
and variable attachments are rejected. The same test and the complete focused
suite pass after the correction. The source-drift guard also rejected a producer
whose input glob accidentally included its generated unsigned files. Source
fingerprinting now covers only actual inputs; native probes compile isolated
copies under dist with signing skipped. Neither failed run was accepted.


## File materialization repair, 19 November 2026

The prior model assumed Get File of Type could convert any Text envelope to the required bytes. The direct signed native control failed, so that assumption is removed from the local model and production path. Final File materialization now uses Base64 Encode/Decode after the existing payload policy and JSON/Text envelope construction. Default payload redaction remains in the row construction; the final encoding of the already-redacted envelope is not a payload opt-in.

Independent signed run44429796917 at0c286b6 proved name-plus-File-Extension, native File type, exact Base64 bytes and Get Text for the reviewed decoded File. The earlier Get Name-only full-name expectation was false on this runtime; original run44401954515 remains archived as a failure. The export probes now reconstruct the full name with that independent property and compare the output type to an actual decoded File witness.

These control results do not execute debugExport. The revised production macro still requires exact-source signed native export validation before any runtime success claim. The native action graph regression fails on the original source and the bounded local model now rejects the old unproved conversion rather than fabricating a File value.
