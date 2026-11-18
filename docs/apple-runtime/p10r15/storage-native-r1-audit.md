# Native Storage Text pilot — 44337212109

The signed iPadOS 27 Simulator pilot returned the exact fresh Shortcuts callback `CHERRI_QA_STORAGE_TEXT_RESULT:TYPE_OK:VALUE_OK`. This proves the reviewed fixture's same-run Text type comparison and Unicode/escape value readback through the existing Stored Content actions. It does not close the cache requirement.

## Immutable evidence

- Source: `ff2d530379aadd3f08ade00a68c96033b14755ee`.
- [GitHub run 44337212109](https://github.com/davidpovarsky/cherri/actions/runs/44337212109).
- Fixture: `tools/shortcuts-simulator-qa/fixtures/qa-storage-text-same-run-v2.cherri`; 32 native actions. Its source SHA-256 matches the exact Git object, probe manifest, compiler manifest, output assertions and signing provenance.
- All 20 published artifact files match their recorded byte counts and SHA-256 hashes. Raster images, archives and raw/private inputs were excluded from publication.
- A distinct receiver self-test precedes the actual `shortcuts-x-callback` event. The actual event has the expected fixture/name, unique fresh run nonce, `SUCCESS`, exact semantic marker, and four parsed NDJSON events: collector begin, before, after, collector end.
- The auditor reruns the callback verifier extracted from that exact source commit. The real event passes; seven local fault controls reject wrong type/value flags, stale nonce, receiver self-test substitution, native error, absent trace and duplicate callback. These are local oracle fault controls, not seven native executions.

The detailed bindings, public artifact hashes, callback and trace are in `storage-native-r1-audit.json`. The AEA1 header check alone is not cryptographic/runtime proof; actual Shortcuts import and execution provide the separate acceptance evidence. The initial pre-import timeout is retained in the artifact's handshake diagnostics; the later independent callback supplies the observed runtime result.

## Remaining scope

This was a single signed Text write/read/delete sequence in one execution using an owned synthetic key. The fixture's delete action ran before the terminal callback, but missing-key readback and deletion semantics were not independently checked. Global/local separation, cross-run persistence, TTL, Dictionary/List/File preservation, and a general typed cache remain unproven. The trace records `UNKNOWN_AT_TRACE_EVENT`; it does not establish typed coverage for every action. Physical-device execution remains open.

The editor persisted 4,400 AX nodes, but the 58 page observations do not establish a 1:1 mapping to 32 native actions. Inspector status remains `NEEDS_REVIEW`.

## Reproduce the bounded audit

Download the named run's bounded artifact, then run:

```text
py -3 -X utf8 tools/p10r15/audit_storage_text_native.py --artifact <bounded-artifact-directory> --commit ff2d530379aadd3f08ade00a68c96033b14755ee --run-id 44337212109 --output <audit.json>
```

The probe manifest is the immutable source-era contract, whose `NOT_EXECUTED` fields describe its creation state. This separate run receipt records the later observation without rewriting the historical source or widening the acceptance scope.
