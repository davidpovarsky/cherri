# P10R15 signed label and export probe registration

## Scope and status

Nine reviewed synthetic V2 fixtures now have explicit choices in the existing iPadOS signed-QA workflow. The local producer compiled all 22 production Phase 0 fixtures with the actual Cherri CLI. No Apple execution or successful signing is claimed by this registration stage. Labels remain representation probes, and the export contract remains a bounded implementation awaiting native acceptance.

| Workflow sample ID | Exact target fixture | Exact callback summary |
| --- | --- | --- |
| qa-label-file-carrier | qa-label-file-carrier-v2 | CHERRI_QA_LABEL_PROBE_RESULT:P15_LABEL_FILE_CARRIER_OK |
| qa-label-raw-empty | qa-label-raw-empty-v2 | CHERRI_QA_LABEL_PROBE_RESULT:P15_LABEL_RAW_EMPTY_OK |
| qa-label-raw-grouping | qa-label-raw-grouping-v2 | CHERRI_QA_LABEL_PROBE_RESULT:P15_LABEL_RAW_GROUPING_OK |
| qa-export-unicode | qa-export-unicode-v2 | CHERRI_QA_EXPORT_PROBE_RESULT:P15_EXPORT_UNICODE_OK |
| qa-export-empty | qa-export-empty-v2 | CHERRI_QA_EXPORT_PROBE_RESULT:P15_EXPORT_EMPTY_OK |
| qa-export-thousand | qa-export-thousand-v2 | CHERRI_QA_EXPORT_PROBE_RESULT:P15_EXPORT_THOUSAND_OK |
| qa-export-redacted | qa-export-redacted-v2 | CHERRI_QA_EXPORT_PROBE_RESULT:P15_EXPORT_REDACTED_OK |
| qa-export-unsupported-file | qa-export-unsupported-file-v2 | CHERRI_QA_EXPORT_PROBE_RESULT:P15_EXPORT_UNSUPPORTED_FILE_OK |
| qa-export-text-file | qa-export-text-file-v2 | CHERRI_QA_EXPORT_PROBE_RESULT:P15_EXPORT_TEXT_FILE_OK |

Each target uses the existing compiler, signing, import, callback receiver and report pipeline. Its fixed relative source path points into p15-label-probes or p15-export-probes. An inherited environment variable cannot replace the selected source. The generic existing Legacy baseline counterpart remains qa-functional-legacy; it is not evidence that a new probe ran in Legacy.

## Runtime and publication requirements

Every new target requires the debug profile and normal REQUIRED_NDJSON trace. Fresh nonce, exact fixture/name, SUCCESS status, exact complete summary, unique callback, correct collector identity and ordered trace remain mandatory. The only trace-absence Release exception remains qa-measure-release-v2. ERROR, CANCELLED, stale or duplicated callbacks, altered summaries and missing traces remain failures.

The publication gate now trusts exactly these nine nested relative filenames in addition to its existing root-level reviewed qa fixtures. It rejects file or directory symlinks, changed hashes, flattened basenames, unreviewed directories, traversal paths and fixture IDs reassigned to another source. It does not recursively trust either probe directory. Trusting a source hash does not authorize publishing its source: .cherri, .shortcut, plist, raster, archive and xcresult files stay quarantined. Only the existing explicit synthetic text paths can be published.

QA accessibility sanitization adds the nine exact shortcut names, with no new arbitrary label/export prefix. New Swift assertions are present but require the Apple CI run; they were not executed on Windows.

## Verified local evidence

The source-bound JSON receipt in signed-label-export-probe-validation.json records 69 Python tests passing with zero failures or skips, including all nine exact shell selections, hostile inherited paths, 108 per-target callback/trace negative controls and real file/directory symlink rejection. The actual compiler producer compiled 22 fixtures and checked all nine new nested source/plist hashes. Its three Release graph faults and five publication provenance faults were rejected. The controlled local publication retained three approved text files and quarantined 64 build/input files. The local producer explicitly labels its own synthetic privacy precondition; it does not assert that a simulator run passed a privacy gate.

Reproduce from this checkout after building the compiler:

~~~sh
go build -o dist/signed-probe-harness .
python3 -m unittest discover -s tools/shortcuts-simulator-qa -p 'test_*.py' -v
python3 tools/shortcuts-simulator-qa/validate_macro_fixtures.py . dist/signed-probe-harness
~~~

The native pilot should select qa-label-file-carrier and qa-export-unicode using require_runtime=true. Broaden to empty/grouping/thousand/redaction/unsupported-File/text-File only after inspecting exact signed runtime evidence. A failed native probe is useful negative evidence and must not be replaced with model success. No macro task closes solely from these local results.
