# R7 File UTF-8 Apple runtime intake — 2026-11-23

## Result and scope

The single requested File run completed successfully on immutable compiler source
`ca316a3dee252456c4bec751594cf249c709b331`.
[GitHub run 45086924979](https://github.com/davidpovarsky/cherri/actions/runs/45086924979),
attempt 1, executed `qa-json-body-file-utf8-v2` on the iPadOS 27 simulator.
The native workflow reported the exact runtime contract verified, and the retained
published evidence independently binds the compiled fixture, signing provenance,
fresh execution identity, successful compiled-Shortcut callback, trace, and HTTP
body observation.

This is a fixed public fixture observation. The diagnostic explicitly retains
`formalAcceptanceGranted: false`; it does not close the 179-case acceptance
matrix, related repair/bridge IDs, physical-device testing, or complete editor
comparison. No compiler, workflow, guard, deadline, or test change was needed for
this run. No new tests were added or rerun for this intake.

## Run identity and dispatch

| Field | Observed value |
| --- | --- |
| Source | `ca316a3dee252456c4bec751594cf249c709b331` |
| Source branch | `audit/cherri-p10r15-r7-integration-20261122` |
| Workflow ID | `419263697` |
| Workflow path | `.github/workflows/cherri-ios27-shortcuts-simulator-qa.yml` |
| Workflow bytes SHA-256 | `71f806bfa314ae4a3041ad14ba9f8c93f9d744db81b12434af006e2810a3962b` |
| Event / attempt | `workflow_dispatch` / `1` |
| Run / job | `45086924979` / `137407571068` |
| Created | `2026-11-23T00:07:40Z` |
| Terminal conclusion | `success` |
| Native job duration | `24m31s` |
| XCTest | `4 passed, 0 failed, 0 skipped` |

Before the one POST, the remote ref matched the exact source and no matching run
existed. One watcher followed that run to its terminal result; there was no
duplicate dispatch or rerun.

The dispatched inputs were `probe_level=full`,
`device_profile=ipad-default`, `sample_id=qa-json-body-file-utf8`,
`require_runtime=true`, `capture_full_editor=false`,
`max_editor_pages=1`, `verify_local_ocr=false`, and
`upload_sanitized_artifacts=true`.

## Runtime observations

The non-self-test event has source `shortcuts-x-callback`, status `SUCCESS`,
and exactly the expected semantic result
`CHERRI_QA_JSON_BODY_FILE_UTF8_RESULT:P15_JSON_BODY_OBSERVED`.
Its fresh execution identity equals both the runtime assertion identity and the
HTTP observation identity. The receiver self-test remains a separate event.

The callback's four parsed NDJSON records equal the four published trace records:
collector begin, the fixture's before event, its after event, and collector end.
The byte serialization differs because the published JSONL is normalized; no
literal-byte equality between the two serializations is claimed.

| Runtime facet | Actual observation |
| --- | --- |
| Primary request | One `POST` to the controlled loopback route, response `200` |
| Request body | Exactly `222` bytes; SHA-256 `310514cc82c5926d5b2f63ff61f23e4d41967f60e83cfd6a60db058656f3bdac` |
| JSON controls | Exact controls, explicit candidate `null`, absent missing key |
| Continuation | One subsequent `GET`, response `200` |
| Rejected requests | `0` |
| Raw UTF-8 body check | `rawUtf8BodyVerified=true` |
| Compiled runtime / callback / trace | `PASS` / `PASS` / `PASS` |

The fixture source and expected contract match their committed R7 bytes.
The compiler manifest, output assertions, signing provenance, and diagnostic bind
the same unsigned plist. This does not prove arbitrary File conversion or JSON
semantics beyond this authored fixture.

## Prerequisite and original source journal

The public prerequisite packet passed the existing exact-publication validator.
Its directory contains only `project-prerequisite.zip.p7m` and `summary.json`.
Source and QA context matched before and after preparation; the fixed host
environment matched macOS `27.0.1 / 26A434`, Xcode `27.1 / 27A9275`, and
`/Applications/Xcode_27.1.app/Contents/Developer`.

| Stage | Result | Observed duration |
| --- | --- | --- |
| Install | `PASS`, exit `0` | `87.037s` |
| Generate | `PASS`, exit `0` | `0.220s` |
| Complete bounded observer | `PREPARED`, complete capture | `91.704s` |

The original encrypted packet was also decrypted and verified **in place in the
existing owner's private Windows workspace**. Only fixed booleans, counts, and
digests left that workspace. The private key, plaintext ZIP, raw journal, private
paths, and command streams were not transferred to Oracle or committed here.

That in-place check verified 23 original members plus the manifest, exact
archive/member hashes and context, all member completeness flags, and the first
three actual source commands: HEAD, clean porcelain status, then the exact
`ls-tree` inventory. Their retained stdout/stderr and process records matched the
journal. All 21 source-file bindings matched the immutable R7 Git objects before
and after preparation. The journal ended at `MATCHED` with no rejection code.
This is source/prerequisite evidence; the distinct runtime evidence above is what
establishes the actual fixture execution.

## Artifact integrity and publication

| Artifact | ID | Download SHA-256 |
| --- | --- | --- |
| Sealed prerequisite | `14781100004` | `64e64bb70f973ae24d3c58a2c53023b72993b40a61899ac51093664a258b0cb8` |
| Sanitized native QA | `14780552216` | `467625960a2a648ae95c7949ef628b496de70c37dcb3105858e08556d4c92b69` |

Both downloaded ZIPs matched GitHub's recorded byte counts and SHA-256 digests.
The native publication has 26 manifest-bound published files plus the publication
manifest; every member's byte count and digest was independently checked.
The publication gate passed with synthetic text only. Raster publication was
disallowed, and no pixel-redaction claim is made.

The originals and API/dispatch/watcher receipts are retained in the private
server evidence directory. This commit contains only this sanitized account and
`intake.json`; it does not republish original archives, private journal contents,
raw logs, callback nonces, device identifiers, or signing credentials.

## Remaining boundaries

- `ACTION_EDITOR_CAPTURED=PARTIAL_EDITOR_CAPTURE` and
  `editorInspectorStatus=NEEDS_REVIEW`: this dispatch deliberately requested one
  editor page. Seven AX containers do not establish the complete 17-action editor.
- `PHYSICAL_DEVICE_TESTED=BLOCKED_DEVICE`: this was simulator execution.
- Trace events retain `typedValueCoverage=UNKNOWN_AT_TRACE_EVENT` and
  `perActionCoverageVerified=false`.
- The overall artifact retains `releaseReadiness=PARTIAL_READY`; this debug
  fixture is not a release-build or full-project acceptance result.
- Legacy, Bool/List, startup UI, N03, Optional, short-circuit and any exact-source
  formal RuntimePoC obligations require their own correctly scoped receipts.
