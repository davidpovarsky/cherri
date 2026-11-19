# FN01–FN03: actual local function acceptance producers

Clean implementation `3b3711f968464a3f42ee3bc8ebdfde73bc299a14` replaces
three historical action-count/nonempty-IR probes with executed compiler tests.
Each unchanged contract requires `unit+native+iOS`. The new producers supply
only `unit` and `native-structure`; FN01, FN02 and FN03 remain
`PENDING_EXTERNAL` until genuine matching-source Apple runtime is accepted.

## Behavior and independent oracles

| Requirement | Actual locally verified scope |
|---|---|
| FN01 | AST parameter/return/call/continuation typing, precise invalid argument and return diagnostics before emission, complete function envelope/dispatch, numeric argument payloads, local output and caller continuation through the final consumer; ordinary, zero and fractional inputs |
| FN02 | Typed scalar defaults and invalid-default diagnostics, explicit false/zero/empty Text versus omitted/mixed arguments, exact presence tags and branches, selected branch output through parameter coercion/body return/final consumer; independently runs the existing typed-envelope and default-guard controls too |
| FN03 | Exact outer-capture and consequent return-type diagnostics for immutable, mutable, later-declared and nested capture, no invalid emitted bytes, explicit parameter typing, recursive Number/Bool AST types and complete native factorial base-case/decreasing-argument/product/local-return/final consumer graph for inputs 0/1/3/5 |

The graph oracle authors the expected native operations, full semantic
parameters, control boundaries and producer edges independently of the
compiler. UUID/group names are alpha identities; duplicate, missing and
forward producer references are rejected. It permits only irrelevant output
label metadata to vary. It checks serialized native structure, not Apple
execution or an emulated runtime result.

Thirty-eight causal graph corruptions are rejected: 13 for FN01, 13 for FN02
and 12 for FN03. These include a local return replaced by global exit, wrong
payload type/value/function, a non-self call, detached return/continuation
edges, altered presence flags and defaults, reversed branch boundaries,
changed recursion predicate/decrement/product, duplicate identities and
literal substituted final observations. Valid controls pass before these
mutations are tested.

The real analyzer/compile routes additionally verify exact diagnostic codes,
source spans and messages, no emitted artifact on invalid input, and healthy
subsequent compilation. The public compile API intentionally returns its
first semantic error; the analyzer separately verifies the complete capture
error pair. Existing container-envelope controls do not grant FN05/T01 or
structured-runtime completion.

## Actual executions and retained failures

The six new root tests passed **60 named test/subtest events**, exit 0, no
skips/failures, in 21.407 seconds. This direct log retains its original parent
`17867426` plus the two then-untracked test files. The subsequent producer and
intake selection passed **118 named events** before commit (34.187 seconds)
and again on exact clean implementation (103.594 seconds, zero fail/skip,
actual exit 0, source clean before and after). The unchanged five-minute
process budget was retained. The longer clean replay was the same original
process after Sentinel moved it to a background job; it was not rerun.

The acceptance selection executes the actual N01/FN01/FN02/FN03 production
tests, including both existing presence/default controls for FN02. It verifies
eight local records, distinct producer IDs and the complete selected command.
Seventy-two causal receipt corruptions across these four scenarios cannot
pass. All 179 audit rows remain enumerated and no related FN04/FN05/T01,
repair or recovery gate inherits success. Missing actual Apple evidence still
prevents final closure. The pre-existing synthetic intake model was updated
to the new protocol fields and remains explicitly synthetic.

Two initial oracle-setup failures are retained unchanged. The first expected
an empty parameter dictionary where the valid zero-parameter exit omits it,
and expected one capture diagnostic where the analyzer reports its exact
consequent return-type error too. The second incorrectly expected the public
first-error compile API to return both analyzer errors. Corrections bind the
actual documented layers separately; they do not modify compiler semantics,
contracts or expected user behavior. The final graph/diagnostic controls and
all mutations passed afterward.

Original JSONL, stderr, summaries, exact clean local receipt and all hashes
are in [evidence](evidence/p15-function-producers-20261119/). Clean JSONL
SHA-256: `148f50c8e9ef41d88d8639c707f0db5464da2966b7787c6da0880de8baccfb18`.
The separate frozen R2 full179 report is unchanged and does not contain these
subsequent producers. No native-device, installed-consumer or release
completion is claimed by this stage.
