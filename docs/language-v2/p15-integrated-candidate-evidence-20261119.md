# P15 integrated candidate evidence - 2026-11-19

This record concerns the frozen candidate
`b2961bbd51e45dd2a1b21d9fd2fb9f3390ed5f74`.
Later commits do not inherit its execution results.

## Actual bounded preflight

The fresh production compiler, affected parity packages, seven selected causal
root tests, the full legacy parity runner, thirteen privacy controls, and fifteen
AI harness controls each exited successfully. The full parity result was 47/47
fixtures and 17/17 action mappings. AI controls measure the harness only; no
external-model quality score was measured.

## Private corpus

Five required top-level local checks were observed passing against the same 38
unchanged specimens and compiler. The initial invocation passed two checks before
its overall 20-minute Go timeout. Only the three uncompleted checks were replayed;
that separate invocation exited zero in 1596.39 seconds. The original timeout is
retained and is not relabelled as a successful full invocation.

No private source, specimen name, private log, or private signing request is
included here. The two original aggregate input digests use different explicit
serializations of the same stored snapshot: sorted hash values versus the sorted
filename-to-hash mapping. The snapshot and all 38 files were compared locally
again before this public summary was written.

## Observed failures and limits

Build & Test run 44607346129 failed on Linux and macOS; Nix passed. The same
storage/F01 observer and producer-inventory checks failed on both platforms.
Skipped downstream steps receive no credit. Those failures have separate fixes
and require a fresh run on the fixed candidate.

The full local179 Windows attempt ended at its 3000-second outer limit
(3004.687 seconds observed) without a completed 179-row report. Its source,
documentation pin, manifest, and three immutable contract digests remained
unchanged. No new full acceptance counts or final closure are claimed.

The additive P10R15 Local Acceptance workflow checks out complete source history
and the exact documented consumer pin, verifies immutable contracts and clean
source, records the compiler hash, executes the actual local runner under an
80-minute process limit within a 90-minute job, and preserves the real exit
status and artifacts. Push registers the workflow on the authorized work branch;
manual dispatch runs the expensive check and explicitly opts into the existing
public K05 signing fixture when requested. The workflow grants no automatic
Apple, private-corpus, consumer-propagation, deployment, or AI-quality credit.

## Public receipts

See [the evidence directory](evidence/p15-integrated-b2961bbd-20261119/):
preflight-summary.json, private-corpus-summary.json,
formal-attempt-summary.json, and ci-failure-summary.json. Private raw records
remain outside the repository; their hashes permit local integrity checks.
