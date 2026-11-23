# Pagination observer repair and original execution evidence

Execution mode: `SUBAGENT`, real agent `/root/ci_source_audit`; coordinator
`/root` independently reviewed implementation before the full replay.

## Actual result and scope

The single complete pagination producer passed at clean implementation
`a8402e05127b87d13b18b960891df3b0a5013de2`, based on frozen K10
`4feadab9db4d351628f78593304373631693869a`.

| Original producer obligation | Actual result |
|---|---:|
| Public CLI source cases | 32 PASS |
| Complete native-action lossless roundtrips | 32 PASS |
| Independent typed-result/HTTP model cases | 144 PASS |
| Invalid CLI sources rejected without an artifact | 44 PASS |
| Causal graph faults rejected, with restored controls | 7 PASS |
| Native Apple executions | 0 |

Producer elapsed time was 12.902 seconds; the external supervisor measured
13.579546 seconds, exit 0, without timeout or source drift. The original
180-second command limits, HTTP/model bounds, source/flavor inventories and
negative cases remain unchanged. The replay ran the producer's original
compiler build. The separate Go pagination parent was not repeated locally.

This is `PASS_SOURCE_AND_MODEL_ONLY`. It does not establish Apple HTTP,
Content Graph behavior, a complete OpenMinis workflow pass, final-candidate
acceptance, or project closure.

## Original failure and causal compile

K10 OpenMinis run [45100808090](https://github.com/davidpovarsky/cherri/actions/runs/45100808090),
job 137450984720, failed in `verify_paginate.py` at the full-graph allowlist
assertion. Its preceding unchanged Go `TestP15Paginate` selection passed in
0.386 seconds. The original failed-job log remains outside Git at
`/mnt/development/projects/evidence/cherri-p10r15-k10-frozen-ci-20261123/openminis-failure-intake/job-137450984720.original.log`,
115,580 bytes, SHA256
`c03edb7e72ce1e27b8beeabac5cafa1ba47d6a3a44e537ede1b3e890b7a98897`.
The original failure report did not retain its compiled graph.

One separately authorized causal compile used the already validated clean R7
Linux ARM64 compiler, source
`ca316a3dee252456c4bec751594cf249c709b331`, binary SHA256
`42f6e42e7b6a8ea6f523a752fd2f06b96265e733fa00ad808190e062d1266301`.
It instantiated the unchanged first V2/debug/base fixture template with a
deterministic loopback origin; no server, signing, action execution or network
request was performed. This is the retained causal compile, not a reconstruction
claimed to be the original CI plist.

That compile took 0.042289 seconds and emitted 183 actions. Exactly one action
was outside the original macro allowlist: action 182, a parameter-free
`is.workflow.actions.nothing`. Action 181 was the typed Paginated Items result
read. The lowering implementation, pagination macro, original Python verifier
and both original Go pagination oracles are byte-identical at R7 and K10.
Their exact hashes and original compile command are retained in this packet.

## Correction and preserved invariants

V2 natural main fallthrough deliberately clears incidental output with
Nothing. The original Go zero-budget oracle already required three macro
actions followed by the fourth, terminal Nothing for V2.

The repair verifies this exact V2 boundary separately from the macro's typed
List result. It rejects missing, duplicate, interior, moved, parameterized or
otherwise malformed Nothing. The original `ALLOWED` set is unchanged; Nothing
is still forbidden inside the macro and in the Legacy graph. The final macro
result must read the same results variable initialized from the actual empty
List producer, with no coercion or overwritten initializer.

All original actions remain in the lossless import/rebuild equality check.
Only the verified macro body is fed into the existing typed-result model.
Zero-budget graphs retain three macro actions, with complete graph counts
of four for V2 and three for Legacy. Reports name that distinction explicitly.

Seven focused offline control parents passed in 0.005 seconds (0.427866-second
process). They reject malformed boundaries, unknown effects, retargeted/coerced
List edges and replaced result reads; they also retain unchanged Legacy behavior
and the zero-network empty List result. The preserved 183-action causal plist
was accepted without mutation after extracting its 182-action macro body.

The source-invariant review confirms every original top-level definition except
`main` remains byte-equivalent as an AST. The original model, process bounds,
allowlist, scenarios, flavor inventory and invalid-source/final-report code
are unchanged. No production compiler, fixture, contract, or any of the seven
AI producer files changed. The workflow only adds the focused Python controls
before its unchanged Go parent and full pagination producer.

## Original artifacts and independent intake

The original complete replay is retained at:

`/mnt/development/projects/evidence/cherri-p10r15-paginate-observer-repair-20261123/full-verifier-attempt-01`

| Artifact | SHA256 |
|---|---|
| Original report | `4cf69cb85a6bbc24b60e8719bfc4760bf3106e24dd7741b764078e21f9120469` |
| Original process result | `78fee3443496e594002d75cbf1004a4a211efe29ae2716879992c0d541fe402e` |
| Original generated archive | `5388fbde58322856fe9d518fe1205adb58b5b9fef242dde4df8494fc213b9f76` |
| Independent mechanical intake | `e3b6ac9e97e8add58e6e868155ff4db094d122bfe649bd805e99b3f2b0f06189` |

The 14,136,608-byte archive stays outside Git. Its 200 original members include
the actual compiler and generated public sources/plists. All member hashes
and CRCs were checked. Independent intake reparsed the 32 original/rebuilt
native action arrays and verified exact equality, all eight zero-budget cases,
16 V2 boundaries, 16 unchanged Legacy graphs, and absence of artifacts for all
44 rejected sources. No producer was rerun for that intake.

The original causal compile is separately retained under
`/mnt/development/projects/evidence/cherri-p10r15-k10-frozen-ci-20261123/openminis-failure-intake/paginate-causal-probe-01`.
Its original plist SHA256 is
`b5a183a0304248954a0630f8e2400f8027d01a3b8594b8d7484d8529e166f1a5`.

## Later-stage review

A read-only audit binds 27 exact K10 files for the later OpenMinis stages.
The three storage and four continuation public fixtures end in explicit
`output(finalSummary)`; the continuation check allowing only its exact initial
debug-clear Nothing remains appropriate. The typed-defaults diagnostic already
verifies its dispatcher closure and parameter-free V2 terminal Nothing.
No equivalent terminal exclusion was found in the specifically reviewed later
signing/package/Alpine paths. No later-stage execution pass is inferred.

All receipts keep their original implementation SHA. This proof does not restamp
K10 failures or the a8402e05 component run as a future candidate's execution.
