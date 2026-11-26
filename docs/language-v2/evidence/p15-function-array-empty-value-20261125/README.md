# Native empty-array serialization and function diagnostic R2

The compiler now always writes an explicit Value array inside compiler-authored
WFArrayParameterState values. Nil and empty slices produce Value: []. Wrapped
nonempty arrays and all raw/import map shapes remain unchanged. The production
repair is exactly three files at 951d1e9f591e7953ad4eadd7102e9ef05872fe3e.

## Original observations and causal qualification

The preserved b82 function diagnostic (unsigned SHA-256
54889840e546acc60ffe05c8399e1d4bd54f0dc846ec3fd520b4fffc0aa6c7d0) failed during
iOS 27 import, before any function invocation. The separate seven-case
WorkflowKit probe actually ran on macOS 26.7.2, source e0ee87f, hosted run
45715308962. At the full Dictionary action boundary, omitting Value from the empty
array caused the nil-insertion exception; explicit Value: [] was preserved.
Wrapped nonempty forms were preserved, whereas plain nonempty arrays lost their
items. Direct array constructors are a different boundary. These observations
do not establish repaired iOS 27 execution. Their complete 271-member originals
remain in the separate published hosted proof at commit6f8ec881c709ec88636e6b546f2b1e66a7fae06e.

The actual paired Go run on29db825 had44 named passes and14 named failures
(12 causal leaves and2 parents). The unchanged58-case selection passed on951d.
Its controls cover Legacy and V2 senders, nil/empty/nested arrays in XML and
binary, and original raw/import fidelity.

The first existing-family run retained977 passes,4 named failures and31
unexecuted declared children. The failures were obsolete omitted-Value graph
expectations. After the reviewed narrow oracle alignment, all1,021 declared
identities and six additional existing Optional corruption children passed
on9b350816f27cf06bf7194e19a1dcc5a97c94f42b. The original owner remains FAILED
because its inventory omitted those six children. The independent intake
verifies all1,027 run/pass identities without rerunning or changing those
tests. The earlier estimate of25 unexecuted children incorrectly subtracted
totals without accounting for those six extras; the exact31 identities are
retained in the intake.

## Separate current and historical identities

Twelve of the1,027 identities replay the exact historical b82 artifact and
grant no current compiler credit. The other1,015 are current compiler
regressions, including fifteen R2 sender controls. This is separate from the
58-case focused run, not a single combined run.

R2 uses workflow CherriFunctionEnvelopePresenceProbeR2 and fixture/trace identity
qa-function-envelope-presence-v2-r2. Its actual379-action unsigned export is
65ebfde24e0653ddf35e6277adde9a2f7e3835207eb2c403f0bed241d17372e9,
269,874 bytes, compiled once at9b3508 and extracted byte-for-byte from its
original Go JSONL. All three self targets and both collector run IDs are bound.
The original b82 fixture, expected contract, plist and full graph validator stay
unchanged.

The separate authored R2 graph at67bb1193fc91d9cfb965e58163f813d9cc7c358e passed
18 Python parents and657 subtests, with31 exact source/input witnesses. It
compares every typed action and edge and all12 exact key-comparison regions.
The controls include379 action-identity mutations,144 comparison faults,
required empty-array Value variants, and cross-version call/trace/artifact
rejections. Actual actions are never rewritten to match the expected graph.

## Scope and retention

The original dispatcher remains represented, including its incomplete envelope
admission. Complete RP26/BRG15 behavior and a first-and-only no-input R2 Apple
invocation remain open. This proof grants no native, full179 or final-source
acceptance.

public-originals.tar.gz preserves the exact available process receipts, raw
streams, source snapshots and independent intakes for the failed and successful
runs above. manifest.original.json hashes each member. The R2 source, native
artifact and compiler-export origin are also committed separately under the QA
fixtures/testdata paths. All evidence records retain the source that actually
ran; none is relabelled to this documentation commit.
