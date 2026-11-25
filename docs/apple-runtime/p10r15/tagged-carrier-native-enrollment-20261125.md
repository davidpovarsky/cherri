# Tagged carrier native engineering enrollment

This additive enrollment prepares the existing native QA owner to acquire one
fixed public observation. It has not executed the fixture on Apple and grants no
T05, L01, F04, physical-device or final179 acceptance.

## Fixed identity and scope

The selector and fixture ID are both `qa-tagged-value-carrier-v2`. The original
source remains SHA-256
`8abc203ba48ba97e6c3c28240dd861e3a6b8b4a0b5ac91f7e82334ca528a46ec`;
the original expected contract remains
`d1d4ce31aefae34870ccf4316d1cde11af08d2751597df8876f6811d0290fb4c`.
Its historical `UNENROLLED_ENGINEERING_FIXTURE` field is unchanged. The separate
`fixtures/qa-tagged-value-carrier-v2.enrollment.json` records the new routing
authority and narrow scope.

The observations cover a dynamic two-number List, a present empty List, a
two-key Text Map and an absent Optional Map cell. Eighteen fields and five real
nonempty UTF-8 type witnesses distinguish decoded payloads from rendered Text.
Map key/value reads remain guarded by the original type relation. False, JSON
null, empty Map, File values, arbitrary recursive values and complete language
acceptance remain outside this fixture.

The historical nested-List IDs, fixture bytes, contracts, modules and their
native results are unchanged.

## Source and runtime joins

`tagged_carrier_runtime.py` invokes the existing complete
`tagged_carrier_graph.validate_compiled` authority, independently recomputes the
prepared manifest row, and binds exact source, unsigned plist and AEA1 bytes to
the signing provenance. It then requires one fresh non-self-test callback with
the exact fixture/name/status and the original four-event trace:

1. `collector_begin` with the original NDJSON sentinel and fixture run ID.
2. `before`, step sequence 3, `is.workflow.actions.setvariable`.
3. `after`, step sequence 218, `is.workflow.actions.comment`.
4. `collector_end` with the same original envelope.

Missing traces are retained as missing. No collector fallback fabricates a trace
for this fixture. The assertion and publication consumers recompute the original
inputs independently instead of accepting a supplied success summary.

Valid contradictory values remain observable, with
`nativeCarrierGoalsMatched=false`. An absent decoded witness cannot become a
type-equivalence witness. Every output retains `diagnosticOnly=true` and
`formalAcceptanceGranted=false`. The payload parser's source/Apple
authentication flags remain false. The adapter likewise leaves actual Apple
run/transport authentication to the independent native owner; AEA1 magic and
synthetic unit callbacks alone cannot supply it.

The existing signing, import, XCTest, callback acquisition, timeouts and final
runtime gates remain in use. The XCTest change adds one allowed fixture string
only: removing that one line reproduces all original file bytes and all four
test method bodies.

## Original retention

The ordinary publisher remains text-only. The new fixed-fixture retention
adapter reuses the existing bounded publication primitives to stage exactly
three reviewed originals: compiled source, unsigned Shortcut and signed Shortcut.
It binds the existing public text manifest, run/attempt/source identity, fixture
contract and enrollment hash. Collection preserves every original byte and
independently recomputes the diagnostic. No private corpus, raster, archive or
arbitrary raw artifact becomes eligible.

The QA checkout inventory additionally binds the new producer, parser, graph,
publication adapter, controls, enrollment, fixture and their shared dependencies
to actual Git blobs. Causal controls mutate physical source files and require
the original `ACQUISITION_SOURCE_BYTES` rejection.

## Local qualification

The bounded combined qualification passed 54 test parents and 224 subtests,
with no failures or skips, in 85.35 seconds. It exercised the two new control
modules and the existing actual shell callback/trace contract suite. These are
offline controls using the retained real cb23 unsigned graph and explicitly
synthetic signatures/callbacks; no Go build or Apple execution occurred.

Original receipt:
`/mnt/development/projects/evidence/cherri-p10r15-apple-completion-20261125/tagged-carrier-enrollment-01/combined-controls-02/result.original.json`

SHA-256:
`945efabd723235847fb2ae8f5ddf2f1e7c41881c4042e01ce283c34ebe373411`.

The first publication run remains preserved. Five source-mutation controls
correctly encountered the existing rejection but expected the wrong diagnostic
literal; their assertions now require the exact original code. A malformed XML
artifact exposed an uncaught ExpatError; the new adapter now normalizes that
specific parse failure to a bounded rejection, with an actual embedded-handler
control. The original graph validator and graph contract were not changed.

Bash syntax, YAML parsing and whitespace checks passed. A fresh integrated
compiler must still produce the fixture and pass the unchanged full 225-action
guard before any signed native execution. The older cb23 graph is not relabelled
as proof for newer compiler changes.
