# P15 debug export metadata: proposed scope and RED fixtures

Base source: 63ccc00efd29887ca2fae9a32cddf9a8080d780a.
Branch: agent/cherri-p10r15-macro-export-metadata-20261125.

This checkpoint adds only tests and this proposal. No export production code has changed, and the new tests have not run.

The original Asana requirement is 1220544665232213. The fetched task/comments are preserved under /mnt/development/projects/evidence/cherri-p10r15-macro-export-metadata-20261125/preparation-01/asana-export-contract.original.json, SHA256 c1ce96bcc294a751156d121e6f916e82296678b4478756aa000cdfbacc49ba37.

## Planned metadata contract

- includeDates:true refers explicitly to the same native item's Creation Date property. It is not the time at which logging or export executes. Native cardinality/type guards must precede ISO8601 date formatting. ABSENT, AMBIGUOUS, and UNSUPPORTED_DATE_TYPE remain observable; no fallback time is invented.
- Add an optional fifth literal flag includeLabels:false. It requests the existing compiler-owned label-range metadata without changing the original four-argument API or omitted metadata behavior.
- Default redaction does not extract label values or read the label buffer. Explicit redactSensitive:false is required to export label content. Literal labels, including empty labels, Unicode, newlines, and braces, must remain data.
- The unredacted label association follows the existing first/last native buffer ranges. Missing, ambiguous, and unsupported label metadata must remain explicit.
- Both frontends use the same native expansion and shared text codec. Dynamic text is encoded as base64 UTF-8 before it enters JSON. The raw log, original producer UUIDs, and authored consumers are not replaced.
- Default envelopes and graphs will be compared against actual pre-change graph receipts. Four original default controls are prepared to write exclusive canonical graph snapshots during RED; no golden is invented before execution.
- The existing File materialization chain and strict native File oracle remain unchanged. Actual prior Apple evidence observed Dictionary for JSON decode/naming while an independent binary witness was File. No new File, date-runtime, or final acceptance claim is made.

The new matrix contains 48 requested metadata source cases (three date/label modes, two frontends, two formats, type metadata on/off, redaction on/off) plus four original-default controls. Each source case first compiles a corresponding old-API control, preserving its declarations and label literals. This separates an actual missing metadata feature from a broken source setup.

The initial graph oracle checks metadata fields, date origin/format/count/type observations, default label privacy, raw-state mutation prohibition, native references, and exact preservation of the original final materialization chain. Further branch/causal controls and independent bounded model checks are required before declaring the implementation qualified. Models will stop at the completed UTF-8 envelope and cannot reclassify the known native File boundary as successful.

## Initial actual RED and default-wire oracle correction

Original job job_e9794d283de4 completed33.45s on e1b9d0240a4d82a0b12ddeb2652cffa60f993bd5. All48 feature sources failed at the intended missing date/label API; every ordinary source control compiled first. The four default controls failed because JSON encoding of the intermediate Go Value struct included empty OutputUUID fields even on Variable tokens. Native plist uses omitempty tags. The strict reference oracle correctly rejected those intermediate fields; it has not been relaxed.

The initial receipt remains UNEXPECTED_RED_RESULT: /mnt/development/projects/evidence/cherri-p10r15-macro-export-metadata-20261125/red-01/result.original.json SHA256033153857c4b76236776618a3e821bf6a03932ca6daa172270455d0986e66fa1. This correction passes only those four default controls through the actual plist serializer before inspection. The48 authored sources, their expectations and all production files are unchanged. A bounded four-control replay is pending; no golden graph has been invented.

## Producer-name comparison correction (unverified)

The corrected native-plist replay `default-controls-03` remained a real FAIL: all four old-default graphs contained unnamed text-token references whose `OutputName` is set to the producer UUID by the canonical shared codec. The borrowed measure graph comparator normalized `OutputUUID` but retained that fallback `OutputName`, so independent builds compared unequal. Export production and all 48 semantic RED sources remained unchanged.

The export-only oracle now normalizes the name only on an actual, independently validated `ActionOutput` node where the original `OutputName` equals that node's own `OutputUUID`. It first runs all original producer-order and duplicate checks. Eight further causal controls keep actual UUID-shaped text payloads and names of a different real producer semantically significant. Together with the original sixteen reference corruptions, the next four-default replay must pass all twenty-four faults and produce four real baseline graphs. No replay success is claimed by this source checkpoint.

## Implemented candidate; qualification pending

The old production is now guarded by four real native-plist graph baselines
from source75c6, produced by default-controls-06 after all24 causal controls
passed. The original48 semantic RED sources are unchanged. The source-bound
intake is debug-export-metadata-baseline-20261125.json; failed and capacity
attempts remain preserved.

The candidate adds literal includeDates and optional fifth includeLabels
(default false). Date output reads each native item's Creation Date; it does
not substitute export time. Native count and type guards precede one ISO8601
formatter. Absent, multiple and unsupported values have explicit statuses.
The runtime Date witness establishes a class for comparison and never becomes
an entry timestamp.

Labels remain redacted unless both includeLabels:true and
redactSensitive:false are explicit. The redacted/omitted path does not read the
label buffer. The opt-in path validates the original scalar first/last/label
metadata once, then merges N items and M ordered ranges in at most N+M cursor
iterations. Gaps are ABSENT, overlapping or out-of-order ranges are AMBIGUOUS,
and bad native classes, cardinalities, fractions or bounds are unsupported.
Private rows begin with Nothing and input-free Set Variable; helper loops end
with Nothing. Original native log objects and label records are not rewritten.

The two frontends request only existing storage-clear backfill for this
export. They do not re-expand earlier preview fallback graphs, and they do not
run detect.date before the absence guard. Source-owned literal labels and the
existing label preview implementation retain their contracts.

The exact historical debugExport("json", true) case is retained and promoted
from an unsupported-date negative to an admitted source-graph check in both
profiles and dialects. The original CLI producer keeps the four historical
identities and reports them as admitted source-only cases. Invalid format,
path, dynamic format and nil/nonliteral Boolean controls remain; the new label
flag receives the same literal validation. This is a documented feature
evolution, not a reinterpretation of old failed receipts.

The new independent bounded evaluator consumes the actual emitted graphs and
stops at the completed Text envelope. It does not execute or model the
Base64 Decode/Set Name native File-class boundary. It checks all48 source
configurations, malformed/empty metadata, Unicode, source-item order and
identity, explicit date statuses, label privacy, N+M action growth, and causal
graph changes. Its source is tools/p10r15/verify_debug_export_metadata.py.

This checkpoint has not run the new implementation. It grants no native
Creation Date, empty-label, File-class, iPad, macro-wide or179 acceptance.
Dedicated signed native evidence remains required after source qualification.
