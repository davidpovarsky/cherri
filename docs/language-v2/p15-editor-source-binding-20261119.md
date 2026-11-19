# Editor evidence bound to immutable Git source

## Verified repair

Implementation `16255d2e4394fecf24dbf2d9ec94b7fa2ae24482` closes a reproduced
source-binding gap shared by the S03, E08, and RP29 evidence intake. Previously,
the intake checked each source digest's hexadecimal shape and the exact path
inventory, but did not compare the digest with the implementation commit.
Changing a source digest and reconciling the outer process-artifact digest was
therefore accepted.

The production receipt and its independent intake now hash the same exact
immutable Git blobs. One bounded `git --no-replace-objects cat-file --batch`
process reads the ordered source inventory at the full implementation SHA.
Validated immutable results are cached, and callers receive copies. Missing
objects, invalid paths, truncated or oversized output, non-blob objects, and
incorrect digests fail closed. Producer clean-before/after checks remain in
place. Working-tree newline conversion does not define the source digest.

The permanent archived-receipt controls use the original e61 Linux artifact,
whose ZIP digest is pinned. The corpus workflow now fetches full history so
the historical immutable blobs are actually available to these tests. Tests
perform no hidden network fetch. The original RP29 Windows receipt was also
replayed locally against its original 5b9 source object; its temporary control
source is preserved with the log, without requiring an unpublished historical
object in every CI checkout.

## Causal checks and actual execution

| Execution | Actual result | Provenance |
|---|---|---|
| Original intake, altered actual S03 source digest with reconciled outer hash | Expected regression failure, exit 1, 69.36 seconds: corrupted receipt was accepted | Parent `474ae29d`, new regression test only |
| First repair replay | 140 named passes and one fixture failure; exit 1 | The manually rewritten CRLF fixture had a stale dirty index status, although its content diff was empty; preserved unchanged |
| Repaired focused controls | 172 named passes across six suites, no failures/skips, exit 0, 29.829 seconds | Parent `474ae29d` plus the recorded implementation/test changes; this is not claimed as clean-commit production evidence |
| Clean current producer and permanent controls | 86 named passes across three suites, no failures/skips, exit 0, 95.151 seconds | Exact clean `16255d2e`, source unchanged afterward |

The focused controls first accept the actual archived S03, E08, and RP29
receipts, then alter each of their 67 source-file fields independently. Every
mutation recomputes the outer process-artifact digest and is rejected for the
incorrect immutable source digest. Other controls cover exact binary bytes,
a genuinely clean CRLF working tree whose committed blob is LF, dirty working
bytes, two distinct commits, copy-on-return caching, missing objects, floating
revisions, path traversal, newline injection, duplicate paths, and tree objects.
The CRLF setup fix initializes and commits the converted worktree directly;
the assertion that it is clean is retained.

The clean replay separately executes the actual RP29 producer. Its child runs
all eleven required production tests, with 34 named pass events, no failures
or skips, actual exit 0, and elapsed time 80.913 seconds. All 23 source-file
hashes are bound to the exact clean implementation commit. The unchanged
security, service, and benchmark tiers are accepted; 23 reconciled-hash
corruptions and the nine existing corruption controls are then rejected.

The actual benchmark retains six real input fixtures, twenty measured phases,
400 raw monotonic samples, exact source/input hashes, and read/encoding
counters. This repair changes neither benchmark thresholds nor process
budgets. It does not establish native UI execution, race safety, another
requirement's success, the final integrated 179 result, or release readiness.

## Retained evidence

The [evidence directory](evidence/p15-editor-source-binding-20261119/) preserves
the baseline failure, first fixture failure, focused replay, clean replay,
their summaries and stderr, relevant control source, and the four new actual
production artifacts. `artifact-hashes.json` records SHA-256 for each retained
file. Copies retain their original bytes and provenance; source identifiers,
timestamps, and results are not rewritten.

| Actual production artifact | SHA-256 |
|---|---|
| `editor-rp29-go.jsonl` | `86c823339139c87c700ff97cc3b4aa4709734aabf7249d7fb8b96aa8e96ca3f3` |
| `editor-rp29-benchmark.json` | `9524961142c41a8fc474f77307d7f44d5c45f088a0aa6a75e2183981f0b351fb` |
| `editor-rp29-process.json` | `3e2d3ddc00c328e99ebb867e2360d04e7f8af3f5ef03ce213a7c55a3b167a87a` |
| `editor-rp29-stderr.log` | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` |

Reproduce the clean current production and permanent controls with:

```sh
go test -count=1 -timeout=5m -json ./tools/language-acceptance \
  -run '^(TestP15RP29AssetAuthorizationProductionEvidence|TestEditorActualReceiptSourceDigestBinding|TestEditorImmutableGitSourceFiles)$'
```

The full immutable acceptance, repair, and delivery-gate contracts are unchanged.
The [e61 full179 audit](p15-formal-local-e61-audit-20261119.md) remains a separate,
earlier source-bound report; no count in it is replaced by this focused result.
