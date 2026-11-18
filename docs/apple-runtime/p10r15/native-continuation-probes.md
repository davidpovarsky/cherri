# Native continuation probes

These are four synthetic, explicitly selected diagnostics. They reuse the existing
compiler, native actions, signed QA workflow, callback receiver and publication
quarantine. They do not change compiler or macro behavior. No native result is
claimed by the source/CLI receipt.

| Selector | Actual operation | Required callback |
|---|---|---|
| qa-storage-text-cross-run | Two external executions of the same installed shortcut on the same Simulator | Writer and reader each return their exact phase, TYPE_OK and VALUE_OK, with independent fresh nonces and normal traces |
| qa-http-json-control | Literal JSON Text through the existing Get Dictionary action | DICTIONARY_OK:NEXT_OK:ITEMS_OK |
| qa-http-json-fetch | One GET to immutable synthetic public data, then the same Get Dictionary/field checks | DICTIONARY_OK:NEXT_OK:ITEMS_OK |
| qa-number-comparison-diag | Paired equality/inequality on identical direct, required and variable-bound Number inputs | DEQ=Y:DNE=N:REQ=Y:RNE=N:BEQ=Y:BNE=N:ONE_NE_ZERO=Y:ZERO_NE_ONE=Y:ONE_EQ_ONE=Y |

## Storage execution boundary

The reviewed source receives only a WRITE/READ phase and a fresh 128-bit synthetic
run token through ShortcutInput. A key under cherri.qa.p15.storage.cross-run.
and a Unicode Text value both derive from that token. The writer stores and reads
the Text in its own invocation. The reader is a second external Receiver launch;
it does not invoke a nested runShortcut() and does not execute the store branch.

The existing strict callback oracle runs after each phase. A writer callback must
be a unique, correlated native SUCCESS with exact phase/value/type flags and an
actual normal NDJSON trace before the reader can launch. The reader receives a
different fresh nonce and the same run token on the same Simulator. Re-entry is
rejected before another launch, and input-free fallback URLs are disabled. The
reader checks native type and value before requesting deletion of its owned key.

A writer-only PASS, missing reader, wrong phase, stale/replayed callback, ERROR,
CANCELLED, missing/changed trace, different Simulator, wrong source/manifest, or
incorrect dispatch order cannot satisfy the two-run contract. Native command
timeouts remain failures. The normal runtime output assertion refers to the
reader after dispatch; the bounded storage diagnostic retains both phases.

The report distinguishes cleanup requested by the read branch from independent
proof of deletion. Even a successful pilot establishes only this synthetic Text
value across these two invocations using the existing native global flag. It does
not prove cross-shortcut isolation, typed Dictionary/List/Date/File archival,
TTL, clock behavior, atomicity, process/device restart durability, or physical
device behavior. cache() remains unimplemented and its broader acceptance task
remains open.

## Public HTTP conversion control

The fetch URL is pinned to commit
38c7098772f4b822d6a3555137c0c7cf8e5b1e5e, file
docs/apple-runtime/p10r15/paginate-pages/one.json in the existing public
repository. There is one GET, no custom header, no credentials, no body, and no
user-provided endpoint. This adds no backend infrastructure.

Both HTTP probes compare the raw response type to independently constructed
native Text and Dictionary witnesses. Their executed branch contributes exactly
one literal trace label: http-json-raw-type-text,
http-json-raw-type-dictionary, or http-json-raw-type-other. The probes then
pass that same response producer to canonical getDictionary, check its type
against the Dictionary witness, and check next == "two.json" and one item.
The expected callback does not guess the raw type; its trace records the category
actually observed.

The prior host GET observed text/plain; charset=utf-8, HTTP 200 and exact pinned
body bytes. That host observation is not the native Shortcuts response type. The
new controls test conversion causally; they do not establish a paginate fix,
status/transport exception handling, pagination completion, or redirect policy.

## Numeric operator diagnostic

The earlier require fixture and its strict expected marker are unchanged. This
additional source creates direct Number 0, required Number 0, a mutable variable
assigned from that exact required producer, and Number 1. Each zero input is
consumed by both == 0 and != 0; the three positive controls use 1 != 0,
0 != 1, and 1 == 1. Each condition has its own initially N flag, changed to Y
only inside that native conditional body.

The graph producer records actual WFCondition, WFNumberValue and WFInput.
It verifies each zero pair consumes the same complete native input map, the bound
variable receives the exact required result, and every flag reaches the final
summary. The current compiler emits 4 for equality and 5 for inequality with a
numeric plist WFNumberValue; this is a wire observation, not proof that these
codes explain the native discrepancy. No operator swap, production condition
change, inferred root cause, or relaxation of the old oracle is included.

## Reproduction and evidence scope

From a clean committed checkout:

    python3 tools/p10r15/verify_native_continuation_probes.py dist/cherri --build --output artifacts/p15-continuation-probes/validation.json

The producer binds all four source files and relevant harness code to Git bytes,
records the actual compiler binary identity, executes all four CLI compilations,
inspects real decoded native action graphs, rejects causal graph corruptions and
performs exact native action roundtrips through the existing lossless importer.
It then executes the real callback oracle and two-run orchestration policy tests.
The same producer is wired into the existing Linux Skill workflow.

The regular QA Phase 0 producer also compiles the new explicit fixtures and
verifies debug profile plus required trace metadata. The whole Python QA suite
exercises existing selectors, the original runtime oracle, bounded diagnostic
publication and raw-file quarantine. The shell syntax gate remains required.

The storage public diagnostic is recomputed from actual phase callbacks, exact
oracle receipts, traces, source manifest and dispatch order before publication.
The raw plan, input token, key, trigger URLs and command output are not published.
The published diagnostic contains only hashes, reviewed phase/status fields and
bounded source-bound results; its claims cannot be changed independently of the
evidence. Native runtime evidence requires a separate fresh signed workflow.
