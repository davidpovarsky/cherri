# Language generation and edit evaluation harness

This runner provides the previously missing executable RP50 harness. Its fixed
synthetic task suite is separate from documentation examples and contains both
generation and edit requests. Every task is submitted once per Legacy/V2 arm,
with the same model ID and input/output budget caps. A fixed random seed controls
order. Model and usage mismatches, missing arms, duplicate tasks, process errors,
timeouts, malformed responses and invalid validator records fail closed.

The default invocation deliberately makes no provider request: it writes
`AI_EVAL_NOT_RUN`, exits 2, and gives setup instructions. The repository supplies
an Anthropic adapter and a compiler/graph validator for an explicitly selected,
sealed evaluation profile; it supplies no credentials and does not enable
provider access automatically. Fake mode launches real local JSON adapter and
validator subprocesses for every task/arm, but reports only
`HARNESS_CONTROL_PASS` and `ai_quality_measured=false`; it cannot produce quality
measurements.

```sh
python3 tools/language-ai-eval/runner.py --output /tmp/ai-evaluation.json
python3 -B -m unittest discover -s tools/language-ai-eval -p 'test_*.py' -v
python3 tools/language-ai-eval/runner.py --fake --model control-model \
  --legacy-guide docs/agent-technical-reference.md \
  --v2-guide docs/language-v2/language-guide.md \
  --output /tmp/ai-harness-control.json
```

## Supplied sealed evaluation profile

The implemented evaluation path consists of these components:

| Component | Responsibility |
|---|---|
| [`evaluation_bundle.py`](evaluation_bundle.py) | Seal fresh exact instances of six public task patterns, both language arms, immutable source/guide/compiler identities, budgets and the pre-call commitment. |
| [`anthropic_adapter.py`](anthropic_adapter.py) | Send the approved public request through the pinned Anthropic endpoint, retain the actual provider response and usage, and enforce one attempt without retries. |
| [`graph_validator.py`](graph_validator.py) | Compile generated source with the bundle's exact compiler and check the supported six-task graph contracts against the independent oracle. It does not execute Apple Shortcuts. |
| [`runner.py`](runner.py) | Run and account for the twelve declared task/arm outcomes under the existing process limits. |
| [`evaluation_intake.py`](evaluation_intake.py) | Independently recompute outcomes from original provider/compiler/validator receipts and the authenticated pre-call commitment observations. |

The reviewed v2 profile is selected explicitly as
`fresh-six-exact-instances/v2`; the historical v1 default remains unchanged.
V2 pins `claude-sonnet-5-5`, API version `2026-10-21`,
`thinking.type=between_tools` and `output_config.effort=high`. Both arms retain
the same 12,000-input/2,000-output token caps, twelve declared outcomes and zero
retries. The bundle's reviewed compiler is a separate identity; it is not
silently replaced by the currently installed or development compiler.

Follow the [v2 protocol](fresh-instance-protocol-v2.md) for exact source and
compiler calibration, bundle sealing, authenticated commitment observations and
the explicitly authorized execution handoff. The [adapter contract](anthropic-adapter.md)
describes original-response retention and the generic transport boundary;
v2's selected settings are defined by the v2 protocol and sealed manifest.
Offline controls and compiler/graph calibration qualify those components. They
do not establish a completed external-model measurement or Apple-native
execution. A graph validator is supplied; a native Shortcuts execution validator
is not part of this harness.

### Dated provider-readiness observation

The retained token-count probe at 05:57 UTC on 2026-11-27 returned HTTP 400 for
provider account credit. It made one count request, no generation request, and
produced no evaluation outcomes. That attempt was blocked by account readiness,
not by an absent adapter or validator. This is a dated operational observation,
not a permanent property of the provider or a model-quality result; any later
continuation must use its own authorized readiness and execution evidence.

## Explicit external adapters

For an actual measurement, supply a trusted model adapter executable and an
independent semantic validator executable, a pinned model ID, immutable guide
snapshots, and `--allow-external`. The adapter owns the authorized provider
connection and credential loading. The runner does not discover credentials or
contact any provider itself. Python adapter paths are run through the current
Python interpreter; other executables are invoked directly, without a shell.
Adapter file bytes, guide bytes and task suite bytes are hashed in the receipt.
Secrets and provider stderr are not published. Generated source is untrusted:
the independent validator must execute it only through the project's protected
workspace/runtime paths and must not weaken source or privacy controls.

Both processes receive one JSON request on stdin and return one JSON response
on stdout. The model request contains `schema`, `request_id`, `model`, `budget`,
`arm`, `task_id`, `kind`, `prompt`, `starter`, `guide`, `seed`, and the suite hash. **It does not include
the task oracle.** The response must preserve schema/request/model identity,
contain nonempty `code`, `synthetic=false`, a nonempty `provider_request_id`, and
actual integer `usage.input_tokens` and `usage.output_tokens` within the caps.

The independent validator receives schema/request/task/arm identity, generated
`source`, its SHA-256, the task's semantic `oracle`, and `required_checks`. It returns matching
schema/request/task/arm/source-hash identity, `synthetic=false`, and a `checks` list whose entries contain
`name` and boolean `passed`. The check names must cover `compiles` and every
`oracle.<key>` exactly once, for both arms; missing, duplicate and unknown checks
fail closed. Only these allowed check fields and the two numeric token counts
enter the report; provider/validator extras are discarded. A valid failed semantic check is a measured failure;
a broken validator is a harness failure. Only complete external model and
validator runs produce per-arm measured pass counts. These are measurements of
the named validator checks, not a universal quality or native-runtime claim.

The supplied sealed adapter/graph-validator path does not remove the generic
adapter interface or provide a native execution claim. Public fixtures, fake usage
counts and harness-unit tests must never be registered as external model quality
evidence or as Apple execution. A task suite being separate from supplied docs
does not establish that a model has never seen its published contents; record
that provenance separately before making a held-out generalization claim.

Adapter and validator files must remain byte-identical through the run. The
request identity includes the complete public model input and budget. Timeouts
terminate the ordinary process group on POSIX and the active child process tree
on Windows. This is lifecycle cleanup for trusted adapters, not an execution
sandbox for malicious adapters that detach themselves or evade process tracking.
