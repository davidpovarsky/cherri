# RP29 actual authorization and measured service evidence

## Verified source and scope

At clean source `5b9c3dd2b6211de39ae071979bc0cf7b1db6f0a9`, the independent RP29
producer executed its exact eleven named production tests and accepted all
three unchanged required tiers: `security`, `service`, and `benchmark`.
The child process passed 34 named test/subtest events in 57.515 seconds, with
no skips or failures. The outer acceptance test and its nine corruption
controls passed ten named events in 66.67 seconds, exit 0, with a clean source
tree before and after execution.

This is evidence for this RP29 source and these local tiers. The complete
179-row integrated candidate, native UI behavior, concurrent race safety,
and release/consumer deployment still require their own executions.
IM04, S03, E08, and other related IDs receive no inherited result.

## What actually ran

The security tests use real, public synthetic outside witness files and actual
file and directory symlinks. They exercise allowed text/binary, missing assets,
existing parent-directory traversal, both symlink escape forms, and an already
cached allowed resource replaced by an outside symlink. Service analysis and
canonical compilation must reject unauthorized resources before content I/O
or artifact emission, retain exact diagnostics, and recover after a legitimate
path is restored. The intake rejects every missing or skipped required
security child, even when its parent test reports success.

The existing service/cache tests run in the same independently selected RP29
process. Its real benchmark records six input fixtures, twenty phases, and
400 raw monotonic timing samples, with source/input hashes and read/encoding
counters. Fixtures include 4 MiB inline text and 8 MiB local text and binary
assets. Cold means a new Cherri service/cache; the OS filesystem cache was not
evicted. No numerical latency threshold or native UI measurement is claimed.

For both 8 MiB local asset kinds, each twenty-sample warm-analysis and
unrelated-edit phase performed zero asset reads, zero bytes read, and zero
encodes. The asset-edit phases each performed twenty reads and encodes of
changed content. The raw measurements, clock ticks, environment, and exact
percentile computations are retained below.

## Intake integrity

The producer preserves the exact command, discovered tests, source file hashes,
process exit/timeout state, raw Go stream, stderr, and complete benchmark JSON.
A clean source revision is required both before and after execution. The
focused clean-source acceptance test rejects missing security tiers, incorrect
producers, missing child bindings, missing or skipped actual symlink events,
stale source, invented runtime tiers, borrowed original IDs, and missing raw
streams. Its stream mutations reconcile the copied outer hashes before
checking rejection, so a digest mismatch cannot substitute for the missing
security execution check.

Before the clean run, five focused inventory/intake suites passed 97 named
events in 11.56 seconds. Those control receipts intentionally retain their
original dirty-working-tree provenance at parent `1c8b5d02`; they are not
represented as execution of the later clean commit. The immutable acceptance,
repair, and delivery-gate contracts were unchanged.

## Retained receipts

The [evidence directory](evidence/p15-rp29-producer-20261119/) contains byte-for-byte
copies of the four production artifacts and the outer test/control logs and
summaries. References inside the outer log retain their original local output
paths; copying these artifacts does not rebind the execution to another source
or convert this focused result into a full179 report.

| Production artifact | SHA-256 |
|---|---|
| `editor-rp29-go.jsonl` | `3bea0cbb00220af9128e02ab6b594f260df62b208cbed4f94e285b05e993b4a6` |
| `editor-rp29-benchmark.json` | `56685288f44953d612f8ec8a7f2eaa9d10462582e64d471d638fd951e46267a9` |
| `editor-rp29-process.json` | `20cca63479c8ff72668e65bd4edb67a48ea472d02a4fe6c9f5abfd325b922412` |
| `editor-rp29-stderr.log` | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` |

Reproduce the focused acceptance producer on a clean checkout with:

```sh
CHERRI_EDITOR_EVIDENCE_DIR=/absolute/retained/output \
  go test -count=1 -timeout=5m ./tools/language-acceptance \
  -run '^TestP15RP29AssetAuthorizationProductionEvidence$'
```

The underlying production test budget remains three minutes. The public
source tests and earlier independent security replay are described in
[p15-rp29-resource-security-20261119.md](p15-rp29-resource-security-20261119.md).
