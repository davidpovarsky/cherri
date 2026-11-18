# Require and measurement runtime acceptance fixtures

This stage adds three synthetic targets to the existing signed Simulator QA
workflow. It reuses its signer, receiver, fresh callback nonce, editor capture,
and publication quarantine. This document records local compiler/oracle proof;
the Apple execution gates remain open until the integrated candidate is run.

| `sample_id` | Compiled fixture | Build profile | Expected callback summary | Trace contract |
|---|---|---|---|---|
| `qa-require` | `qa-require-v2` | Debug | `CHERRI_QA_REQUIRE_RESULT:P15_REQUIRE_OK` | Complete NDJSON |
| `qa-measure` | `qa-measure-v2` | Debug | `CHERRI_QA_MEASURE_RESULT:P15_MEASURE_OK` | Complete NDJSON |
| `qa-measure-release` | `qa-measure-release-v2` | Release | `CHERRI_QA_MEASURE_RELEASE_RESULT:P15_MEASURE_RELEASE_OK` | Absent |

The require fixture checks zero Number, false Bool, nonempty Text, Dictionary,
a singleton List containing an empty Text, and a native File. Its type witness
uses the corrected canonical `typeOf`/Apple `getitemtype` contract. It must not
be run against the earlier `gettypeaction` implementation, which means Get
File of Type in the captured native metadata.

The Debug measurement fixture measures nested blocks with equal labels. It
checks a preserved Dictionary and its nested List, increments separate outer
and inner counters once, and requires its independent outer timer to observe
1..8 seconds around a one-second Wait. The Release fixture checks the same
body counters and typed result with the Wait retained. It does not consume a
diagnostic result or emit trace events.

The existing `qa-short-circuit` wrapper is also synchronized with the expanded
canonical F01 fixture: guarded missing targets, false Bool branching, helper
predicate values, negated `isTrue`/`isFalse`, negated prefix, and `isBetween`
as a value. Its existing output marker and NDJSON requirement remain in force.

## Fail-closed Release exception

The compiler command uses the registered `--build-profile=release` argument
only for `qa-measure-release-v2`; all other synthetic fixtures explicitly use
`--build-profile=debug`. Legacy fixtures still use the actual `--legacy`
frontend selector.

Before signing, the production compile phase independently inspects the
Release graph. It rejects retained debug/trace storage or elapsed-time actions,
and requires one Wait and one Dictionary body producer. The manifest records
the explicit profile, trace expectation, compiler commit, source/plist hashes,
and these structural checks. Compiler and fixture-phase failures stop the
orchestrator immediately.

The callback oracle requires that manifest to match the current compiler
commit and exact fixture. It then enforces one callback, the fresh invocation
nonce, expected Shortcut name, non-self-test source, SUCCESS status, and the
exact semantic output. For this one allowlisted fixture, a missing/nil or empty
trace yields `NOT_APPLICABLE_RELEASE`; any unexpected trace or nontext field
fails. That state is not a trace PASS. The trace file is empty and no synthetic
collector event is invented.

The output assertions, summary, and final `require_runtime` check carry this
same contract. Changing a normal fixture's profile or trace state cannot opt
it into the exception. Wrong compiler hashes, missing profile proof, duplicated
callbacks, stale nonces, error/cancel callbacks, wrong results, and unexpected
Release trace all remain failures.

Typed per-action coverage remains `UNKNOWN_AT_TRACE_EVENT`. A valid runtime
callback does not turn unavailable AX identity or values into an editor PASS.
`RUNTIME_VERIFIED_EDITOR_NEEDS_REVIEW` remains the honest outcome where the
existing Inspector cannot establish a one-to-one native card mapping. Physical
device status is unchanged by these Simulator fixtures.

## Reproducible local validation

The actual production Python heredocs are executed by
`test_runtime_callback_contract.py`, including callback validation, output
assertions, summary generation, and the final required-runtime exit condition.
The harness's full Python suite includes inspector corruption, privacy canary,
dispatch literal-argument, and Release contract mutations.

`validate_macro_fixtures.py` executes the production compile phase using a
real locally built CLI. It compiles all 12 fixtures, then makes three causal
in-memory mutations: omit the Release flag, remove the Wait body effect, and
duplicate it. Each must fail for the graph-contract rejection rather than an
unrelated exception. It does not sign or start a simulator.

```sh
go build -o artifacts/cherri .
python3 tools/shortcuts-simulator-qa/validate_macro_fixtures.py . artifacts/cherri
python3 -m unittest discover -s tools/shortcuts-simulator-qa -p 'test_*.py' -v
```

The associated JSON evidence distinguishes local compiler/oracle verification
from Apple runtime evidence. Source hashes and unsigned hashes are local
artifacts; they do not replace signed artifact provenance on the final SHA.

## Required next run

On the final integrated revision, dispatch the existing
`cherri-ios27-shortcuts-simulator-qa.yml` workflow once per required target with
`probe_level=full`, `device_profile=ipad-default`, `capture_full_editor=true`,
`max_editor_pages=20`, `require_runtime=true`, and
`upload_sanitized_artifacts=true`. `sample_id=all` compiles the matrix but only
runs its primary functional fixture; it is not proof that every target ran.

Publication remains restricted to the existing allowlisted synthetic text
artifacts. Screenshots, archives, raw inputs, and private payloads remain
quarantined. No pixel-redaction success is claimed.

Rollback is a revert of this harness commit; it does not change the signer,
production services, or physical devices. The scoped-block feature and require
emitter have separate commits and acceptance records.
