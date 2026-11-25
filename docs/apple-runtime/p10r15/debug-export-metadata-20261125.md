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
