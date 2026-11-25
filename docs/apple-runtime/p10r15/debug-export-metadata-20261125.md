# P15 debug export metadata: source qualification and native gaps

Implementation source: fa2a204fd45ddcaaaf123ffe3ed20f9df6510f42. Qualified test/model source: a921113944f7e23629eed6198aef52331cd5cac8. Branch: agent/cherri-p10r15-macro-export-metadata-20261125.

This component implements guarded item Creation Date metadata and separately requested labels in both frontends. The qualification combines an original Go run and one narrow oracle replay with unchanged production inputs. It does not claim one all-green run, native Apple execution, a returned File, complete macro acceptance or 179-requirement acceptance.

The original requirement is Asana task 1220544665232213. Its fetched contract and comments remain in `/mnt/development/projects/evidence/cherri-p10r15-macro-export-metadata-20261125/preparation-01/asana-export-contract.original.json`, SHA256 `c1ce96bcc294a751156d121e6f916e82296678b4478756aa000cdfbacc49ba37`.

## API and data contract

The original four arguments retain their defaults. A fifth literal Boolean is optional:

`debugExport(format="json", includeDates=false, includeTypes=true, redactSensitive=true, includeLabels=false)`

- `includeDates:true` reads the same native item's Creation Date property. It does not invent a log or export timestamp. Zero values produce `ABSENT`, multiple values produce `AMBIGUOUS`, and one value of a different native class produces `UNSUPPORTED_DATE_TYPE`. Count and class checks precede ISO8601 formatting. The Current Date witness establishes a class for comparison; it never becomes an entry's date.
- Labels are read only when both `includeLabels:true` and `redactSensitive:false` are explicit. The redacted and omitted paths do not read the label buffer or its values. Empty labels, Unicode, quotes, newlines and braces remain data.
- The label reader validates existing scalar `first`, `last` and `label` records once. It then merges N raw items and M ordered ranges in at most N+M cursor iterations. Gaps remain `ABSENT`; overlap or out-of-order ranges are `AMBIGUOUS`; malformed native classes, cardinalities, fractions and bounds are unsupported. Invalid metadata takes precedence over ambiguity.
- Both frontends use the same expansion and canonical text codec. Dynamic text enters JSON only after base64 UTF-8 encoding. The original raw objects, label records and authored producer references are preserved.
- Private rows start with Nothing followed by input-free Set Variable. Helper loops end with Nothing, preventing incidental values from becoming Repeat Results. The frontend hook only initializes existing label storage; it does not re-expand earlier preview fallbacks or run date detection before the absence guard.
- Release removes the diagnostic macro. Legacy emits no action for the standalone source; V2 retains its exact parameter-free main Nothing boundary. Consuming the removed diagnostic result remains an error.

The final Text → Base64 Encode → Decode → Set Name chain is unchanged. It expresses the intended local artifact boundary but does not prove its native File class.

## Preserved RED and default graphs

The 48 authored feature sources first failed on missing capability at `e1b9d0240a4d82a0b12ddeb2652cffa60f993bd5`: sixteen date cases were unsupported and thirty-two label cases exceeded the old argument signature. Every corresponding old-API source control compiled first. The initial receipt remains `UNEXPECTED_RED_RESULT` because four default comparisons also exposed an oracle serialization defect; it is not relabeled.

The default oracle was corrected to inspect actual plist serialization. A second comparator defect came from the canonical codec's unnamed OutputName fallback. The export-only comparison normalizes that name only on a validated ActionOutput whose original name equals its own OutputUUID. Literal UUID-shaped payloads and another producer's name remain significant.

Actual source `75c6eb49f6900a9d88098826ebca8ea4b8ee3d15` then passed four default controls and twenty-four causal reference/name corruptions. Their real graph snapshots are committed under `testdata/p15-debug-export-metadata/default/`; no golden was synthesized from the new implementation.

The source-bound baseline intake is [debug-export-metadata-baseline-20261125.json](debug-export-metadata-baseline-20261125.json). All earlier failed and capacity-blocked attempts remain preserved.

## Actual source qualification

The complete binding is [debug-export-metadata-qualification-20261125.json](debug-export-metadata-qualification-20261125.json), copied exactly from the original independent intake. The intake rehashes original streams, reports and 48 emitted graphs and verifies 969 source inputs.

| Evidence | Observed result |
| --- | --- |
| Original Go qualification on fa2a | 215 named PASS, one FAIL; preserved as a failed run |
| Original metadata cohort | All 48 source configurations PASS, same identities as RED |
| Original defaults and reference faults | Four defaults and 24 causal faults PASS |
| Profile/storage behavior | Eight cases PASS, including existing effects and repeated export/label storage |
| Narrow replay on a921 | Original failed parent and six added Release leak controls PASS; 22.608 seconds |
| Independent model | 720 small cases over the actual 48 graphs PASS |
| Model stress | Eight 1,000-item cases PASS, with one or 1,000 label ranges |
| Causal model faults | Twelve corrupted graphs rejected and restored controls accepted |

The original Go failure was the test helper comparing nil and empty Go slices for the correctly empty Legacy Release graph. The narrow correction validates the exact frontend-specific graph and rejects added Text, Quick Look and Nothing actions. Production Go files and every other test function remain identical to fa2a. The evidence join covers 222 distinct qualified Go test identities; it is not represented as a new full-suite execution.

The first model attempt failed on `WFWorkflowActionParameters:null`, the actual Go JSON representation of parameter-free Nothing. The corrected decoder permits null only for that action and continues to require a map for every other action. The failed attempt remains retained.

The exact historical `debugExport("json", true)` sources and CLI identities are retained as newly admitted source cases. Invalid formats, unsafe paths, dynamic formats and nil/nonliteral Boolean options remain rejected. The original CLI producer still requires execution on the next integrated compiler; changing an expected feature outcome does not establish that execution.

## Independent model boundary

`tools/p10r15/verify_debug_export_metadata.py` consumes the emitted native action graphs. It models only the path through the completed Text envelope and stops before Base64 Decode/Set Name. It preserves raw item order and identity, validates item-derived dates, verifies label privacy and malformed metadata statuses, and checks linear action growth.

The twelve causal controls include replacing the source date with export time, dropping ISO8601 time, changing Base64 line breaks, writing the raw buffer, creating a synthetic row, stalling the range cursor, leaking helper output, changing a range boundary, substituting range data for label text, reading redacted metadata, changing the artifact name and changing the decode direction.

The model explicitly reports `nativeRuntimeExecuted:false`, `materializationBoundaryExecuted:false` and `fileClassAccepted:false`. Native property absence/cardinality, empty Text preservation, Dictionary field classes, list and control-result behavior, ISO8601 formatting and UTF-8 Base64 require dedicated Apple observations.

Independent source review found no blocking issue in the committed encoder, fifth flag, default path, storage-only hooks or range/privacy checks. The encoder is byte-identical to the reviewed proposal. [Original review](debug-export-metadata-independent-review-20261125.json), SHA256 8834d798d077d1743f6a56728572824397a686c6b29d35fcc5ef2c9a66cd5858, grants no native or formal acceptance.

## Original receipts

Paths are relative to `/mnt/development/projects/evidence/cherri-p10r15-macro-export-metadata-20261125/`.

| Receipt | SHA256 |
| --- | --- |
| `default-controls-06/result.original.json` | `f4112b3b726ecaa863a2059d0a356e5344b933bcf8bb26c3d1e5263a3b3d8e52` |
| `green-01/result.original.json` | `1f9ddf4d1edfdb6e063f7606b90d6fe29151ba306ddc0d5e82e023acf5c065e6` |
| `text-model-02/model.original.json` | `f211cf16ed02dc729e95c63efbec11ee4cc0f6ddc3cb3a79f6e5b7aaa4e7ecf8` |
| `literal-options-replay-02/result.original.json` | `05036b9142def5e22df232a52845d64e0b7caf9b08f078f6e7ae10ac2d84981b` |
| `qualification-intake-01/result.original.json` | `aad532f3a8af2ce1a2c52436a916fc0b14e553da2595cd8625dc6d87931c7e66` |

## Remaining acceptance

The prior Apple observation that JSON Decode/Set Name produced Dictionary while an independent binary witness produced File remains counterevidence. The new source and model do not reinterpret it.

Dedicated signed Date/empty-label/range observations, actual File class and exact UTF-8 bytes, structured payload serialization beyond the existing explicit Text-only policy, physical iPad behavior, integrated CI and final consumer pins remain open. Source support for metadata does not close the complete debugExport task.
