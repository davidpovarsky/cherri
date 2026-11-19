# P15 backend state: causal race baseline and core repair

The bounded unsigned mobile/canonical state scenario passes under the real Go
race detector after the shared native-state lock repair. The same unchanged Go
tests failed under the race detector before the repair. This is source-bound
execution evidence; it does not by itself close BRG06, RP51, or the 179-row
acceptance inventory.

## Exact source and executions

| Phase | Clean implementation commit | GitHub Actions run | Result |
| --- | --- | --- | --- |
| Tests-only baseline | `21ed8858ce363080436964ba85c469db7cfb83b6` | [44626278954](https://github.com/davidpovarsky/cherri/actions/runs/44626278954) | FAIL, exit 1, actual data races |
| Core synchronization repair | `abc83b8626ecd86ceadca878b94799cfd6e70c1d` | [44629009283](https://github.com/davidpovarsky/cherri/actions/runs/44629009283) | PASS, exit 0, no timeout, no skipped tests, no race warning |

Both executions used Linux amd64, Go 1.24.13, gcc 15.2.0, `CGO_ENABLED=1`,
`GOMAXPROCS=2`, `GOFLAGS=-p=2`, and an empty `GORACE`. The exact environment
receipts retain the runner kernel and Python versions. No required CI job was
replaced: this independently named workflow runs the isolated `probe/` branch.

The actual command was identical in both executions:

```text
go test -race -count=1 -json -timeout=8m -run ^(TestBackendRecovery06MixedEntryPoints|TestBackendRecovery06EmitterErrorAndCancellation)$ .
```

The two top-level tests and the `emitter_error` and `cancellation` child tests
were discovered. The successful instrumented child observed three sequential
rounds followed by eight concurrent workers completing eight rounds each.
Request-specific native Text parameters, native action identity, platform
metadata, exact decompile output, catalog digest, document completions,
diagnostics, document versions and error-to-success recovery were checked.
Nonempty output or a successful process alone could not satisfy these checks.

The fault test injected an error after the first native action had been emitted
and rejected partial output, an extra third emitter call and retained compiler
state. It then made real canonical and mobile requests. The cancellation case
injects `context.Canceled` at that same emitter boundary; it does not claim a
public request-context cancellation API.

## Observed cause and repair

The baseline raw stream reports `WARNING: DATA RACE` between
`resetCompilerState` / `BeginBackendTransaction` in canonical compilation and
the legacy parser running through `decompileForMobile`. The canonical path used
`compilerSessionMutex`; the mobile paths changed the same globals under
`mobileCompileMu` alone. The independent Windows baseline also returned an
actual decompile `index out of range [5] with length 1` failure under concurrent
requests. Its original local receipts remain retained separately.

The production repair changes four files. Mobile compile and decompile now
hold `compilerSessionMutex` before changing shared arguments, embedded-error
mode or parser/decompiler state, and retain that ownership through recovery.
An explicit private session constructor lets the mobile V2 path reuse its
owned lock. Public canonical construction still acquires and releases the lock
once. Session cleanup is deferred immediately after construction, before
configuration. The lock order is mobile mutex, then native-state mutex;
deferred recovery completes before either mutex is released.

No action definition, parameter encoding, serialization contract, fixture or
comparison oracle changed. This is shared transaction synchronization, so no
fork-action provenance entry is added for an individual action.

## Retained raw evidence

The `baseline` and `fixed` directories under
`evidence/p15-state-race-core-20261119/` contain the downloaded environment,
source-file, process/result, stdout and stderr artifacts. The large failed
stdout stream is retained losslessly as `go-test.jsonl.gz`; decompressing it
must produce the exact hash and byte count below. The successful stream is
retained as plain `go-test.jsonl`. `retained-files.json` records the stored and
uncompressed hashes. Receipt bytes are copied without newline normalization.

| Artifact | Bytes | SHA-256 |
| --- | ---: | --- |
| Baseline raw Go JSON | 3,289,400 | `e13f5f1d912b2dbe527e271a9f6828f969cf87e4a887ff983c10bc70a5e2e71c` |
| Fixed raw Go JSON | 4,591 | `c7dac548523cb15168be1f1745ac72a404183c5c9c78acfa7de603a1eec9fa3c` |

The source-file manifests bind the identical scenario files to each clean
implementation. Each process receipt retains the exact command, source SHA,
clean-before/after observations, actual exit code and timeout status. The
workflow's seven synthetic intake controls exercise its validator and produce
no production race credit.

## Remaining obligations and scope

This proof covers `CompileSourceToPlist`, unsigned mobile V2/Legacy compile,
mobile decompile, the mobile catalog and concurrent language-service requests
exercised by the named tests. It does not establish concurrent signed
`CompileFileV2` safety: that CLI wrapper has process-global signing metadata
outside the inner session. No native Apple runtime execution is claimed.

Hybrid/in-memory import and cold canonical-catalog initialization are separate
shared-state boundaries undergoing additional causal tests. Formal BRG06/RP51
intake is still required, and RP51 also requires the independently measured
cold/warm benchmark evidence. The receipt intentionally leaves
`formal_requirements_proved` empty. The combined integration SHA must rerun the
relevant race and required CI gates after these commits are integrated.
