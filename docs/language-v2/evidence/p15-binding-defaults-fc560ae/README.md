# Clipboard capture and typed-default evidence

Implementation source: fc560ae30cdcccd8089b1e742fba6f77a8a05720.
Baseline source: 7a43cb10099d4120c7abf04c630aa13e7407d272 plus the recorded public tests.

The production repair materializes a Clipboard attachment at an immutable let
declaration with the existing native Get Variable action. Later uses and aliases
refer to that one output UUID. Mutable-number captures retain their original
producer across a later assignment. The shared serializer, direct native
references, function envelope, and default dispatcher are unchanged.

## Actual execution

| Run | Named pass | Named fail | Skip | Meaning |
| --- | ---: | ---: | ---: | --- |
| Initial baseline | 2 | 11 | 0 | Unsupported lowercase fixture spelling plus missing Clipboard capture and an overly strict empty-container wire expectation |
| Corrected spelling baseline | 19 | 8 | 0 | V01 controls pass; Clipboard still has zero captures; historical empty wrappers differ from explicit typed literals |
| Candidate build attempt | 0 | 0 | 0 | Test source had a duplicate local name; build failed before any test executed |
| Corrected candidate | 163 | 0 | 0 | Production repair and causal local controls pass |
| Clean fc560ae replay | 163 | 0 | 0 | Exact clean source before and after; 8.165 seconds |

The authoritative exit codes are in the original summaries and JSON streams.
The Sentinel wrapper completes after recording a failed Go exit; wrapper success
is not a test success.

The clean stream is 151,670 bytes with SHA-256
71e3495ee7a10b6fe2553428fc896a88d06aeff4b397812c92787fb8e7446d1c.

The first source snapshot retains the exact authored text before gofmt; the
compiler tests had been formatted by the same gofmt before that initial run.
The corrected-baseline source files were copied byte-for-byte before the
production repair. Original raw JSON streams are unmodified.

## Scope and remaining proof

V01, V03, and T01 each require unit + native + iOS in the immutable contract.
This archive proves local serialized graph properties and actual service,
mobile bridge, and CLI execution. It does not prove iOS execution, publish a
formal requirement pass, or close the release.

V01/V03 negative controls delete, duplicate, reorder, or substitute the capture,
change the source identity, replace output UUIDs, and introduce deferred reads.
The existing independent direct-reference roundtrip and reference operand
controls also passed.

T01 local checks distinguish supplied false/zero/empty Text, empty Array and
empty Dictionary fields from absent arguments. They inspect all five presence
guards and the five default producers and reject flipped guards or argument
presence/value corruption. The current function envelope uses historical empty
native wrappers: Array omits Value; Dictionary has an empty Value map. The
initial strict literal-reader expectations exposed that difference, not proof
that an empty argument became absent. The final test explicitly characterizes
the wire shape and retains the missing runtime proof. The signed
CherriTypedDefaultsPOC fixture must still establish List and Map counts of zero
for explicit empties, one for omitted defaults, and the exact mixed result.

Runtime fixtures and exact expected clipboard payloads are in
tests/runtime_poc/P15_BINDING_DEFAULTS.md. The older comprehensive runtime PoC
and local smoke now use the supported case-sensitive Clipboard identifier.
Spelling correction alone is not runtime or UI proof.

No acceptance count, immutable requirement, native oracle, or private corpus
was modified for this work.
