# Apple preflight recovery — 21 November 2026

The verified repair source is `93b0711aa8eb100702092aca6671e4312517dcf1`.
This report grants no Apple application, signed Shortcut, or formal requirement acceptance.

## Original integrated-source attempts

Both attempt-1 runs used the independently verified remote source
`2b84116ecf0395fbec73231b02f4aaca6963e30d`:

- [Startup observation 44936763355](https://github.com/davidpovarsky/cherri/actions/runs/44936763355)
  failed before app build. `TestCherriNoSign` passed, but `go test ./...`
  discovered incomplete historical Go evidence as executable packages.
  The separate integration owner repairs evidence organization while preserving
  every original byte. The startup observer controls passed on Apple: 19 Python
  tests and 79 Swift assertions, explicitly `nativeAppExecution=false`.
- [Signed FileUTF8 44936768275](https://github.com/davidpovarsky/cherri/actions/runs/44936768275)
  failed in Python discovery: 371 tests, eight failures and one error.
  The orchestrator, signing and Shortcut execution did not start.
  The finalizer rejected missing acquisition identity and publication stayed blocked.

The startup collector preserved zero acceptance records and
`NOT_RUN_OR_INCOMPLETE`, with no test log or xcodebuild exit code. The precise
startup capture error was `TEST_LOG_BOUNDS`; it stopped on the absent test log
before checking the also-uncreated Simulator identity. No startup event archive
was fabricated. Original artifact 14719719375 is retained unchanged alongside
its member hashes; its SHA256 matches the GitHub artifact digest.
Original workflow logs, run/jobs/artifacts JSON and the exact failed-step logs
remain at the external original paths and hashes in the accompanying JSON.

## Repairs and actual replay

`95ffd4f1` aligns the stale Optional positive control with the reviewed actual
400-action artifact, including the explicit terminal return. Obsolete count 399
and numeric 400.0 remain negative controls. The original two-test RED output is
retained; no fixture, contract, expected runtime value or production oracle was
relaxed.

`93b0711a` also repairs the paginate test's extracted scope so it ends before
the separately tested acquisition checkpoint, makes the fake successful Xcode
project generator use its pinned Python interpreter instead of an unprovided
host `mkdir`, and installs pinned PyYAML 6.0.3 in a workflow-owned virtual
environment. Apple reported generator exit 1; the private inner stderr was not
uploaded, so the source-level PATH diagnosis is distinguished from that raw fact.

A fresh Windows virtual environment installed the declared dependency and ran
the complete QA Python suite on clean 93b0711a: **386 discovered, 382 passed, four
existing POSIX skips, zero failures/errors**, in 462.281 test seconds
(465.828 process seconds). The exact HEAD and clean worktree were checked both
before and after. Original stderr SHA256:
`bf6a54c82e90a44c28da948c4046d3a4013a80477b460dd1370f53c74be6ec97`.

The actual Optional graph gate on the same clean 93b0711a produced **66 named PASS
records across four parent tests, zero failures**, in 19.938 seconds.
Original stdout SHA256:
`4ed0e47f7e613a3a2d902256b65274e70775fe2f532415baaced81ca33d1fccb`.
All original process receipts and output hashes are retained.

## Scope still open

These are preparation and infrastructure results. Current-source Apple startup
and three original UI tests, the signed FileUTF8 runtime, POSIX controls, separate
nested-list control/candidate observations, physical iPad and formal 179 gates
remain open. A successful shape observation will not grant native nested-List
semantics or general language acceptance.
