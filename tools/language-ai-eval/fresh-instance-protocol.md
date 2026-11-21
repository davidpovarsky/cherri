# Six paired fresh instances: bounded AI01 evaluation

This profile measures **six pairs, twelve outcomes** using one pinned provider
model and the existing runner. It does not edit `tasks.json`, AI01, RP50, or
BRG32. It does not establish that the six public task patterns were unseen by
the model. It does not execute Apple Shortcuts or prove native runtime behavior.

## Source and instance freeze

Freeze the generator, adapter, runner, validator, intake and both guides in a
clean Git commit before preparing a real bundle. The model is the canonical
`claude-sonnet-5-5` identity retrieved through the provider Models API. The
original metadata response and its receipt are required; key presence alone is
insufficient. The exact S3 compiler binary is separately pinned to source
`9c8daf721aa241127df8d1e3f91bbadf90110e73`, not the evaluation implementation
commit. All hashes are checked again when loading a bundle.

`evaluation_bundle.py --prepare` creates a new output directory exactly once,
obtains a 32-byte seed from the operating system CSPRNG, and deterministically
derives six task instances. Nonce-bearing literal data and binding names provide
fresh exact instances of the original six contract shapes. Numeric inputs stay
inside the exact integer range. There is no corpus sampling, model feedback,
search among bundles, or post-outcome regeneration.

The generated suite and its private oracles retain the original task IDs,
generation/edit kinds and oracle keys. Legacy and V2 have semantically paired
inputs. Their only intentional context differences are language arm, starter
syntax and the previously reviewed guide. The fixed shuffle seed is derived
from the same committed random seed; the order is frozen before any call.

The manifest contains hashes of the seed, suite, oracle, compiler, guides and
producer files, plus the exact model/API/settings/caps and expected outcome
inventory. OpenSSL Ed25519 signs those canonical manifest bytes. The public key
fingerprint and signature are part of the commitment text. The private signing
key must never be published or supplied to a model.

## Independent pre-call anchor

Post `asana-commitment.txt` to the designated project task and read the task's
comments through the authenticated Asana connector. Save the unmodified returned
JSON as `anchor-story.json`, then obtain a second authenticated read and save it
as `anchor-reread.json`. The exact task, comment ID, text and server-created time
must agree. Both reads precede the first provider call. A Git author date or a
local `verified=true` field cannot substitute for that server observation.

At intake, the operator performs another authenticated Asana read and supplies
it from a path outside the bundle. The offline intake verifies agreement with
the anchored commitment; it cannot authenticate an arbitrary JSON file as an
Asana server response. The source of this trusted input remains explicit in its
output. HTTP response Date values support provider chronology and are not
described as cryptographic timestamps.

For formal project intake, the operator supplies two independent authenticated
Asana observations from outside the bundle: one collected before the first
provider request, and one after all provider and validator work has finished.
Their original response hashes and observation UTC values are pinned separately
by the caller. Invoke the canonical checker against each response and require
the same unchanged anchor and measured content. Its safe `execution_interval`
contains `started_utc` and `finished_utc`, derived only from the original
validated provider and validator receipts. Provider start precedes its validated
count request; the end includes the latest validator completion. An unfinished
provider attempt leaves `finished_utc` null and cannot establish a completed
measurement. These host receipt times support the caller's chronology checks;
they do not authenticate the Asana observations or assert cryptographic time.

The seed and oracle stay out of the provider payload. After every outcome is
sealed, enough generator/seed/suite/oracle material can be disclosed for
reproduction. The signing private key remains private. No private project
fixture, user document, credential, tool session or earlier model output enters
the task context.

## Same budgets, one attempt

Both arms use the same model and API revision, input cap 12,000 tokens and output
cap 2,000 tokens. Thinking is disabled; streaming, tools, MCP, provider cache,
fallbacks, server state, retries and shared conversations are absent. There is
one stateless request per task/arm. A count request precedes generation, with a
15-second transport ceiling; generation has a 90-second ceiling. The existing
runner's per-process ceiling remains 120 seconds. The independent compiler
validation subprocess has a 35-second ceiling and uses `--skip-sign`.

The provider's preflight count is an estimate, not actual usage or a provider
guarantee about an input cap. The actual response must carry the exact model,
HTTP request ID, message ID, normal completion and actual token usage. All
token components are reconciled; unexpected cache, thinking, tools, iterations
or state invalidate successful-generation evidence. Refusals, truncations,
timeouts, cancellations and errors are preserved as outcomes and never retried.

Each provider and validator stage claims an exclusive directory. The runner
predeclares all twelve outcome rows and saves progress before requests. An
incomplete run retains its partial records and the original six-per-arm
denominator. It cannot publish a completed AI-quality score. If source integrity
changes, remaining requests do not start. An operator must not restart the same
bundle after a failure or select a different bundle based on outcomes.

## Independent semantic adjudication

The validator compiles the exact unmodified provider text using the pinned S3
binary, in the requested language mode. It rejects file directives, native
escapes and unsupported source calls before compilation. A conservative lexer
and statement/expression grammar permits only scalar literals, bindings,
arithmetic, lists, indexing and the four explicit Show/list-selection calls.
Imports, includes, embedding, native escapes, declarations of actions/functions,
member or indirect calls, unknown tokens and unclosed strings fail before the
compiler binary is read or launched. Raw include/embed directives are rejected
even inside quoted text because the Legacy include pass runs before parsing.
Rejected source remains an explicit unadjudicated outcome. This grammar is a
bounded source policy, not a claim of operating-system sandboxing.

The compiler receives only OS runtime/locale variables and a new temporary
directory inside its own attempt. Provider credentials, inherited search paths,
home/profile directories, original temporary paths and Cherri feature/corpus
overrides are absent from its environment. Its working directory, source,
output and executable paths are fixed by the sealed bundle. It interprets the
original typed plist bytes through a small explicit pure-action subset:
Text, Number, arithmetic, lists, known list selection, variable binding and the
single requested Show Result effect. It preserves integer/real/Boolean kinds,
UUID identity, producer-before-consumer order and UTF-16 attachment ranges.

The six checks use independently generated oracles. A disconnected addition,
an unused list plus a literal result, or a hard-coded duplicate of a named
binding cannot supply causal proof. Numeric and binding provenance follows the
actual displayed value. The retained edit binding must still have its expected
number. Every graph action is read and classified; unrecognized actions,
coercions, external inputs or source forms are **unadjudicated**, rather than
automatically counted as a model error or success. This limited graph evaluator
does not replace the compiler or a general Shortcuts semantic specification.

Before freezing a real evaluation, calibrate this subset against twelve real
public baseline compilations, one per task/arm, using the pinned S3 binary.
Synthetic graph and transport controls alone cannot establish compiler
compatibility or measure model quality. Preserve every calibration failure and
correct only a demonstrated mismatch; freeze the corrected implementation
before drawing the real evaluation seed.

## Evidence and intake

The adapter retains raw requests/responses, exact generated text, actual IDs,
usage and status locally. The validator retains the exact source, command,
compiler stdout/stderr, original plist and recomputable checks. The runner also
retains process originals. It signs each completed outcome's record and complete
original-file inventory using the public key anchored before the requests.
Later modifications cannot be hidden merely by updating unsigned file hashes.

The intake verifies the signature and original inventories, the independently
reread Asana commitment, source Git blobs, both HTTP input bodies, original
provider identity/usage/text, compiler/source/graph bindings, chronology and all
twelve ordered outcomes. It recomputes graph checks from original plist bytes;
it does not trust a model explanation or a validator's claimed PASS. Its own
tamper controls use synthetic data and mocked transport, never live requests.

Only twelve adjudicated outcomes yield `VERIFIED_MEASUREMENT` and six-per-arm
success counts. A complete provider refusal or output-cap truncation with valid
model identity, original response and actual usage is an observed task failure;
it receives no invented compiler or semantic checks. Transport/protocol errors
or unadjudicated validator forms keep an explicit incomplete outcome account.
Correctly hash-bound partial response bytes remain valid evidence of a failed
attempt; they cannot become a completed generation. A completed refusal or
truncation containing thinking/tool/other unsupported response blocks is also
unadjudicable under this no-tools profile. File reads have a byte limit and
reconcile the opened file's identity and size before and after reading.
The intake leaves `formal_acceptance_granted=false`; the
project's existing acceptance layer must independently consume this reviewed
measurement under the original requirement. Fresh exact-instance provenance,
unseen-pattern generalization and Apple execution remain separate claims.

## Execution gate

Preparing a real bundle and setting `CHERRI_AI_EVAL_ALLOW_GENERATION=1` occur only
after the project owner's source/protocol review. The environment flag is
required even for the token-count POST. Tests mock every provider transport and
do not execute the compiler. No credential is put in command arguments, report
text, source control or published artifacts.

The runner must be invoked with the exact bundle suite, guide paths, seed,
model, caps, timeout and the source-bound adapter/validator. `--sealed-bundle`
requires the report path to be `<bundle>/report.json`. The same absolute bundle
path must be in `CHERRI_AI_EVAL_BUNDLE`. Changing an option after commitment
fails before the next provider request.
