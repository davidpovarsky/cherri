# Signed Legacy runtime: observed pass, Inspector still needs review

[Run 44238251950](https://github.com/davidpovarsky/cherri/actions/runs/44238251950)
completed successfully on exact compiler source
`23ea902b21b9049b6a89873e985c902073ed2d41`.
This is an actual signed Shortcuts run on the iPadOS 27 simulator. It verifies
the selected Legacy fixture's semantic callback and required trace export.
It does not establish complete Editor Inspector or project release acceptance.

The audit reads only the run's
[published synthetic artifact](https://github.com/davidpovarsky/cherri/actions/runs/44238251950/artifacts/14399413787).
All 20 published file SHA-256 values and byte lengths were verified. The detailed
hashes and observations are retained in
[legacy-runtime-44238251950-audit.json](legacy-runtime-44238251950-audit.json).

## Runtime and provenance

The selected fixture is `qa-functional-legacy`, compiled using the Legacy
frontend with 34 native actions. Its source SHA-256 is
`e09dc606678740e5bdba32d6dd0c98b02da9cbb88b45cd2bfa419f8d1ed76b72`;
the unsigned plist SHA-256 is
`7410df5b33f26304b6405f1a20669ad905521d34fb6bf057bbb0729a6c82e4cf`.
The compiler manifest and runtime assertion receipt agree on both values.

One compiled-Shortcut callback returned exactly:

```text
CHERRI_QA_RESULT:ok:count=2:mode=legacy
```

Its fresh correlation ID is
`qa-functional-legacy-44238251950-78221881d4aca3cb0a68011191487254`.
The callback is `SUCCESS`, timestamped `2026-11-18T05:05:48.569Z`, and is distinct
from the single receiver self-test. The callback's decoded trace payload equals
the published NDJSON events, including their values and order:

1. `COLLECTOR_EVENT / collector_begin`
2. `INLINE_DEBUG_ACTION / before`
3. `INLINE_DEBUG_ACTION / after`
4. `INLINE_DEBUG_ACTION / end`
5. `COLLECTOR_EVENT / collector_end`

The earlier R1 run `44223774682` used the same fixture source hash and the same
34-action count. Its semantic callback passed, but trace verification failed
with no begin/end or inline events. This run restores the actual trace output.
That observation is consistent with the separately tested correction preserving
exact producer/consumer UUID spelling in the Legacy debug macros. Other harness
changes were also made; the run comparison alone does not isolate every change.

The target and counterpart were signed in separate HubSign calls. Their signed
hashes therefore differ. At this exact implementation, the target is copied into
the retained import path before the candidate path is reused for the counterpart.
The distinct signatures do not establish an import/provenance defect.

## Exact acceptance scope

| Area | Observation | Scope |
| --- | --- | --- |
| Shortcut execution | `PASS` | Selected signed Legacy fixture |
| Compiled callback | `PASS` | Fresh correlation ID and exact semantic output |
| NDJSON trace | `PASS` | Two collector events and three inline events |
| Typed values at each trace event | `UNKNOWN_AT_TRACE_EVENT` | No typed per-action claim |
| AX tree persistence | `PASS` | 4,403 nodes, zero invalid geometry, error category `NONE` |
| Editor traversal | `EDITOR_CAPTURE_COMPLETE` | Seven pages, all three anchors, bottom confirmed by multiple signals |
| Native action comparison | `NEEDS_REVIEW` | 34 expected actions, eight observed card candidates, zero unambiguous matches |
| Physical device | `BLOCKED_DEVICE` | This run used the simulator |

The Inspector reports `ACTION_COUNT_MISMATCH` and
`PAGE_OVERLAP_WITHOUT_STABLE_IDENTITY`. Per-action statuses remain eight
`AX_UNEXPOSED` and 26 `NOT_FOUND`. The 34 heuristic labels are explicitly not a
verified card count. Successful scrolling and AX serialization do not establish
one-to-one native action identity, parameter visibility or token connectivity.
OCR was not used in this run.

## Publication boundary

All 17 quarantined paths were absent from the downloaded publication. No PNG
or other raster evidence was published. The privacy claim is restricted to
allowlisted synthetic text and byte checks; pixel redaction is not verified.
No private host paths, images or raw OCR are included in this handoff.

This evidence applies to the exact source and fixture above. Other dialects,
new fixtures, new compiler source, complete Inspector coverage and final release
status require their own matching evidence.
