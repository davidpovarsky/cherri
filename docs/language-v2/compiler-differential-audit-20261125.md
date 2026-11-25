# Compiler differential audit — 2026-11-25

## Source and scope

This source checkpoint records the compiler and validation repairs qualified at
`2add2627de17633bab56aca8a1c4b5651a1ced3d`. It is a finite differential audit, not
proof that every possible program or every Apple action is correct. Historical
P10R15 release/179-ID acceptance remains independent.

The comparison pins pristine upstream `98d5191ae121a6bcf663ed0e55495f4ee4c7a6e2`,
frozen upstream `bcc0095754e2cc0baaeb4d93dae172b8153b4716`, and the fork's Legacy
and V2 frontends. The actual final compiler executable was built at clean
`21ead0aed1031de7a23d5e0155efc02252f02fcb`, SHA-256
`c0b230055fbee75d315e81333112e0d5a2367c14bd9ee048427f4ee636352107`.
Later commits change the native test harness and RP52 acceptance inventory.
The source reuse bridge checks the actual dependency inventory, all compiler
input/embedded-file blobs, and all 82 fork artifacts. It proves compiler-input
identity through 2add; it does not relabel the older executable as a new build.

## Authored programs and exact comparisons

`tests/differential_30/` contains 39 main cases plus Q40 dynamic quantities and
Q41 dynamic Text states: 41 workflows, 90 authored source files. They cover
Unicode/text transforms, regex, split/join, numeric expressions, statistics,
rounding, lists, dictionaries, branches, loops, nested control, dates, Base64,
URLs, properties, captures and two timer structure cases.

There are 162 compilation-attempt rows and 159 successful artifacts: 82 newly
compiled fork artifacts and 77 hash-verified original artifacts. Both fork
frontends compile all 41 cases. Current upstream does not support main case31;
frozen upstream also rejects case36. Those failures remain visible.

The comparator retains raw differences and uses a bijective mapping only for
actual identity-bearing UUID slots. It rejects invalid producer/control graphs
and preserves ordinary UUID-valued text, scalar types, defaults and action order.
All 36 comparator controls pass. The 117 main comparison slots contain 113
actual wire differences and four compile-blocked slots. Four additional Q40/Q41
comparisons are retained separately. Action-tree equivalence to current upstream
holds for 21/38 Legacy and 8/38 V2 main cases; equality of all files is not claimed.
All 82 final fork graphs are valid and their typed workflows match the preceding
qualified compiler revision.

## Reproduced defects and repairs

| Defect | Original compiler | Shared repair and evidence |
| --- | --- | --- |
| Dynamic `qty` magnitude lost its reference type | Both checked originals preserve the reference | Fork V2 now carries the typed value to the canonical quantity builder; all four catalog slots are covered in both frontends; Q40 ran successfully in all three runtime lanes. |
| Repeated derived UUID collisions | Inherited from upstream; V2 also ignored the option | One per-compilation generator covers frontend, canonical and macro paths; 24 real CLI processes yield 12 exact replay pairs with valid graphs. |
| Statistics input emitted under `WFInput` | Current original has the same native output failure | The shared definition emits Apple's `Input` key. Both fork runtime cases pass; original case15 remains an actual failure. Provenance records the real implementation commit. |
| Authored dynamic Text used the wrong token carrier | Generic item outputs require explicit Text materialization in the common fixture | Shared serialization covers 18 Text slots/72 observations and preserves explicit imported raw-reference fidelity. Main case33 passes in both fixed native lanes. Q41 retains separate source-bound attempts. |
| Loop alias/compound producer handling and constructor validation | Compatibility differs by pinned upstream version; malformed original constructors can panic | Root-level frontend/lowering repairs retain producer identity and argument evaluation order, and reject unsupported values before silently generating a misleading workflow. |
| Compilation API discarded later semantic errors | A fork API boundary | Joined errors preserve all reported semantic diagnostics. |

The older case35 expected value was also wrong: the pre-existing filename
contract produces `qa:qa`. The original mistaken oracle and failures remain;
all three corrected native observations agree. This is recorded as a test-oracle
correction, not a compiler defect.

## Actual native observations and harness qualification

At the 113-selection checkpoint, main cases plus Q40 contain 112 passes and the
preserved original statistics failure. Each fork has 37 passing main runtime
cases plus Q40. Two timer cases are structure-only because the required Clock
executable is unavailable in the selected simulator runtime. No alarm/timer
native acceptance is inferred from those structures.

Q41's complete chronological record is maintained in the final native ledger
and delivery report. Partial successful callbacks from a failed XCTest batch
are retained as observations, not promoted into a successful complete batch.
The final ledger may contain later, separately qualified observations; this
source checkpoint does not forecast their result. Every accepted observation
must preserve exact artifact/source lineage, expected output, callback identity,
and the evidence quality actually captured at the time.

The harness fixes are separate from compiler behavior. The host now launches an
owned URL dispatch, publishes the phase, and joins the real process within the
existing 60-second budget; 24 actual host controls passed. The native Add window
is anchored to max(search start, successful delivery exit) plus 15 seconds, with
receipt/time checks after blocking UI calls. Missing/changed/late/nonzero/unreaped
dispatches fail closed. All 50 actual Swift boundary controls passed on Mac and
the regenerated Xcode project built for testing successfully. This does not turn
any failed historical import or callback into a pass. Exact image, name, nonce,
output-byte count and SHA-256 checks remain in place where supported by the
original observation; older evidence is explicitly qualified.

## Complete Go run and separate recoveries

The full Gate3 run executed all 30 packages exactly once on clean 21ead under
an ordinary user. It finished naturally with exit1 and two top-level failures.
The ordinary invocation took 1041.479 seconds (its root package986.448 seconds),
the acceptance invocation1807.385 seconds, and the wrapper2853.754 seconds.
No invocation timed out. The complete original result remains FAIL.

1. The owned checkout lacked historical Pack commit141b9cc9. Importing the
   genuine history alone fixed the exact failing parent and all ten children:
   actual exit0,46.856 seconds, unchanged clean21ead source.
2. RP52's expected-event inventory omitted the six new statistics controls.
   The reviewed repair reuses the existing U01 inventories and hashes the shared
   source dependency. On clean component8721f692 the actual producer and focused
   controls passed:360 outer named events,104 required children each run/passed
   exactly once,74 source blobs. Missing/unknown/duplicate or altered evidence
   still rejects; the strict parsers and root assertions were not relaxed.

The initial RP52 replay also encountered ENOSPC in the host's /tmp. Its failed
receipt is preserved. The successful replay changes only task-private temporary
paths to the development disk. Earlier full-run failures and their diagnosis
remain available; none is overwritten by supplemental evidence.

## Evidence and ongoing use

Task evidence root:
`/mnt/development/projects/evidence/cherri-differential-30-20261125`.
The final user report packages the sources, signed/unsigned artifacts,
per-case expected and observed outputs, structural diffs, original failures,
source pins and SHA-256 indexes. The native ledger identifies evidence reused by
exact bytes separately from evidence joined by unchanged typed programs.

| Receipt | SHA-256 |
| --- | --- |
| Gate3 full-go-suite.json | `657c562c60f6c99bb6030e4209105933abe4d87186e7ef61089d1bad4a706676` |
| Gate3 full log | `a2abbd4e1c2605477f80f0a4ea1ab27db5ba0a4405d3e31355a204484f05d5b5` |
| Pack supplemental recovery | `108b6002b1762565efbe2c89496208d1725dd7dbe8b49b1204b123ca93ab60ec` |
| RP52 clean actual audit | `3d1ce8ff9e6f16bd9a86571d5bced5855ae7870e0db10c08824a644deaf4f14c` |
| Compiler-source bridge21ead→2add | `ee7b89857cda796fd8737f5e9557d74dcdd34e9c3d7829dad62543436b682558` |
| Native timing Swift50 output | `dee42ba37af817ee09426f23353db2932df4704c2b50cfe7a05c4e9a8751eafe` |

Progress and final handoff:
https://app.asana.com/1/1219597879016953/project/1219603572114432/task/1220897415739298

For a new regression, retain the failing program and original result, pin both
compilers, compare typed graphs and actual output, repair the shared cause, then
record the new execution separately. Agreement with an upstream bug is not a
correctness oracle, and this finite corpus does not establish universal parity.
