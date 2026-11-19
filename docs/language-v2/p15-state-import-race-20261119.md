# P15 import and cold-catalog state isolation

The expanded production state scenarios pass under the actual Go race detector
at `87ca5dfc5194633eb4223a6983704593c34e113a`. This extends the
[core state repair proof](p15-state-race-core-20261119.md) with cold catalog
initialization, hybrid import, in-memory Legacy compilation and a real mobile
emitter panic. Formal BRG06/RP51 intake and the independent RP51 benchmark tier
remain separate obligations.

## Causal runs with unchanged scenarios

| Phase | Clean source commit | GitHub Actions run | Result | Actual race warnings |
| --- | --- | --- | --- | ---: |
| baseline | `a06b9c92751cbcfc832d921f4cec4ed401a6c3f3` | [44629555814](https://github.com/davidpovarsky/cherri/actions/runs/44629555814) | FAILED | 108 |
| intermediate | `2847b3bba70c92baf26fb06d90842a9095472d74` | [44630396283](https://github.com/davidpovarsky/cherri/actions/runs/44630396283) | FAILED | 1 |
| fixed | `87ca5dfc5194633eb4223a6983704593c34e113a` | [44631364786](https://github.com/davidpovarsky/cherri/actions/runs/44631364786) | PASSED | 0 |

All three source-bound executions used Linux amd64, Go 1.24.13, gcc 15.2.0,
CGO_ENABLED=1, GOMAXPROCS=2, GOFLAGS=-p=2, and an empty GORACE.
The exact environment receipts are retained. The final process exited 0 in
55.677004 seconds including build, without
timeout, skip, failure, or race warning. This elapsed time is not a performance
benchmark or a latency-budget result.

The scenario source files and their oracles did not change between the three
executions. Manifest comparison identified exactly six changed production
files; retained-files.json records that comparison.

The actual command was:

    go test -race -count=1 -json -timeout=8m -run ^(TestBackendRecovery06MixedEntryPoints|TestBackendRecovery06EmitterErrorAndCancellation|TestBackendRecovery06ImportEntryPoints|TestBackendRecovery06MobilePanicRecovery)$ .

Four top-level production tests and two error/cancellation child tests passed.
The completion witnesses report 64 concurrent mixed-entry cases and 32
additional import/cold-catalog cases, plus explicit fault and recovery checks.
These are case counts; a case may perform more than one API request.

## What failed and what changed

The expanded baseline found 108 race warnings. In-memory Legacy compilation
wrote shared arguments and parser state while canonical action initialization
parsed embedded action definitions through those same globals. That failure
also produced an actual invalid type diagnostic in the isolated child process.
The earlier core scenarios and the new panic control still passed.

The first follow-up repair put canonical definition initialization, the
in-memory Legacy compiler and the semantic decompile prepass under the common
native-state mutex. It added explicit already-owned lookup and initialization
helpers so canonical emission never acquires its own lock recursively. The
prepass copies its observations into the semantic context before releasing
native state ownership.

The intermediate execution completed all 32 import cases, but the race detector
still reported one conflict: reverse-registry construction read
actionDefinition.builtin while Legacy markBuiltins wrote it. A shallow map copy
retains those definition pointers. The final change therefore keeps the
default reverse-registry construction under the same mutex, using the
already-owned initialization helper. Existing reverse candidates copy scalar
definition metadata by value. No fixture, parameter encoding or expected
native output changed.

A read-only review also investigated updateContact parameter writes. The
callback already clones its parameter slice immediately before the write;
that suspicion was not established as a defect and prompted no code change.

## Actual recovery checks

The mobile panic control injects a panic on the second real emitter call,
after the first native action was emitted. It requires the public mobile
boundary to report the sentinel error, return no partial or signed artifact,
stop before the third call and then successfully execute fresh canonical and
mobile requests. The error and context.Canceled controls independently check
that their errors remain discoverable and do not leave compiler state behind.
Cancellation here is injected at the emitter boundary; no new public context
API is claimed.

The import fixture is independently authored native input. No compiler call
warms canonical definitions before the competing workers start. Hybrid output
is recompiled and its exact native Text action and synthetic payload are
checked. The child process boundary makes a real exit, panic or deadlock fail
the parent test instead of silently losing discovery.

## Retained receipts

The complete downloaded environment, source-file, process/result, stdout and
stderr artifacts are stored under
evidence/p15-state-import-race-20261119/. The large baseline stdout is losslessly
gzipped. The manifest records both stored and original byte hashes.

| Raw Go JSON stream | Bytes | SHA-256 |
| --- | ---: | --- |
| baseline | 1453633 | `7e5bce539978dc74715ba3bbda5346dacef48fac6f5620372db219d853027a55` |
| intermediate | 21238 | `7ed8ac91ae5f43f4c2ed7b5a0c50756f0f342e37d16cf3725e6cb412da08ff67` |
| fixed | 6900 | `27a6e4be10dc72cf318914253d3df135793b1b8d6419197ec1b1ee0cfa209c23` |

The workflow's ten synthetic negative/shape controls test its evidence
validator. They are not counted as production or native runtime execution.
The process receipts intentionally leave formal_requirements_proved empty.

## Integration scope

This proof covers the named in-process unsigned compile/decompile, hybrid
import, catalog, service and error-containment paths. It does not establish
concurrent signed CompileFileV2 safety or Apple runtime behavior. It does not
supply RP51 benchmark measurements or close any unrelated acceptance row.
The combined integration SHA must run its own race and required CI gates.
