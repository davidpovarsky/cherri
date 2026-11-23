# K11 macOS terminal evidence: cumulative package timeout

[Build & Test run45108200176](https://github.com/davidpovarsky/cherri/actions/runs/45108200176),
attempt1, macOS job137474256647; exact source
`9b4135e705b08f765b7304e01212d3b9658806fb`.
The original root package failed at900.153s under its unchanged15-minute limit.

At the alarm, TestP15CanonicalBehaviorSourcePropagation was59s old and its
/codec child2s old. The preserved stack is a real subprocess wait. It does not
establish child-deadline expiration or deadlock. The original baseline, static
mutation/restoration and callback mutation/restoration receipts are retained;
the codec/restoration tail did not complete before the package limit.

All1,180 root parents started:1,169 PASS,10 SKIP, one incomplete. All29
opted-in parallel parents continued,28 passed, and no unstarted parallel queue
remained. Both K04 parents demonstrably used PAUSE/CONT and passed in92.55s
and45.27s. This proves the hooks were active, not a successful full-suite result
or a controlled global-speedup measurement.

All30 package invocations are accounted for. The ordinary invocation exited1
after918.709s; acceptance exited0 after1711.823s, with package duration1704.855s.
BOM-aware, section-bounded named-result counts are16,665 PASS/14 SKIP/0 named
FAIL. **The root-package timeout is an actual failure despite the absence of a
named FAIL event.** Exact per-phase, parent/subcase and skip detail is retained.

See `originals/INTAKE.md` and `originals/intake.json` for the full analysis,
clock distinctions and original limits. The proposed further scheduling hooks
are a separate unexecuted review at this checkpoint; this packet applies no
source fix and predicts no successful runtime.

Root verified all19 original-manifest members and the complete full-suite
archive CRC/source/digest. Eighteen bounded originals plus the original complete
manifest are copied here. The10,576,046-byte full job log remains unchanged
at the Oracle path/hash in `retained-large-originals.json`; the exact timeout
stack and relevant receipts are included. No second encoding or editor artifact
download was made, and no test or workflow was rerun. The later P10 gate step
did not execute. No overall CI, R23, native,179 or release acceptance is granted.
