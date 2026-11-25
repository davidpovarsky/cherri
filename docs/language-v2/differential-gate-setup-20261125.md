# Differential gate setup corrections — 2026-11-25

## Retained complete-run findings

The original ca26086b full run failed and remains retained. The successor full
run at 2f4a592b exposed nine top-level ordinary-package failures and a 900.272s
root-package timeout; the remaining ordinary packages passed or had no tests.
Its acceptance phase continues independently. No failed receipt is overwritten.

The concrete causes include the statistics provenance entry being in the wrong
registry collection, the preserved U01 boundary not yet including the proven
statistics key correction, a stale generated comparison manifest, missing
`python`, permission-denial tests executed with effective UID zero, and Go1.27
rejecting a non-object destination when a cache test forces a real link.
These have separate owners and focused repairs. This document does not claim
those repairs or a later complete run have passed.

## Reuse the qualified cumulative budgets

A read-only review of the current integration and Darwin CI completion branches
found an existing reviewed policy: 20 minutes for ordinary packages and
50 minutes for sequential cumulative acceptance. The four exact files are
reused from the qualified component visible at
f169069641ca97ddd0797183ff1f493e0be54f73:

- tools/p10r15/run_full_go_suite.py
- tools/p10r15/test_run_full_go_suite.py
- tools/language-acceptance/publication_security_execution.go
- tools/language-acceptance/publication_security_execution_test.go

The CI04 intake changes together with the runner, so command and receipt
budgets remain exact. Every package still executes once, ordinary parallelism
remains two, and acceptance producers remain sequential. Child timeouts,
selectors and assertions are unchanged. Ambiguous, absent, unlimited or changed
budgets reject before spawning Go. The earlier component's complete-CI and
formal-acceptance fields remain false; its scoped qualification is not promoted.

The four source hashes were independently joined to the retained actual20m/50m
qualification receipts. Evidence is under
`evidence/cherri-differential-30-20261125/ci-reuse-review/` on Oracle. The exact
four-file patch has SHA256
cbe843407652fa1fe501d926fd53ccb61353312a7cce18e329919efc319fcde0.
The source qualification join has SHA256
7858d7d7e3306a4c835f88ebce52d973f560fec9948eeb49ef25afb4a764e22c.
Only this bounded component was reused; the active integration branch's other
132 commits are outside the earlier2f source claim.

## Bounded native runner startup

Q41's first signed batch observed zero cases because XCTest startup exceeded
the host's existing120-second readiness wait. Its second attempt reached the
verified Receiver but timed out opening the first signed file. A subsequent
screenshot request also timed out. The dedicated simulator was shut down;
its boot request timed out and it remained Shutdown. No other simulator or
global service was reset. A third attempt on the other task-owned Booted
simulator again observed zero cases before the readiness deadline.

All attempts retain their original manifests, summaries, process logs and
signed bytes. These are infrastructure outcomes, not semantic passes/failures.
The host now accepts `--startup-timeout-seconds` as an explicitly recorded,
bounded integer1..600; its default remains120. An extended startup allowance
only waits for the actual new run's ready file. Installed-image identity,
import identity, nonce, exact UTF-8 length/digest, per-case timeout, observed
case count and XCTest outcome remain required and unchanged.

Ten existing Go-plan Python tests and21 existing native-host controls passed
on the actual five-file candidate, including deadline corruption controls and
stale Receiver image rejection. Exact commands/log hashes are retained in
`evidence/cherri-differential-30-20261125/gate-setup/focused-controls.json`.
This proof does not grant Q41 native acceptance or complete Go acceptance.
