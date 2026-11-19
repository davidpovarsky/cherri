# Language generation and edit evaluation harness

This runner provides the previously missing executable RP50 harness. Its fixed
synthetic task suite is separate from documentation examples and contains both
generation and edit requests. Every task is submitted once per Legacy/V2 arm,
with the same model ID and input/output budget caps. A fixed random seed controls
order. Model and usage mismatches, missing arms, duplicate tasks, process errors,
timeouts, malformed responses and invalid validator records fail closed.

There is no configured model endpoint in this repository. The default invocation
writes `AI_EVAL_NOT_RUN`, exits 2, and gives setup instructions. It never invents
model scores. Fake mode launches real local JSON adapter and validator subprocesses for every
task/arm, but reports only `HARNESS_CONTROL_PASS` and
`ai_quality_measured=false`; it cannot produce quality measurements.

```sh
python3 tools/language-ai-eval/runner.py --output /tmp/ai-evaluation.json
python3 -B -m unittest discover -s tools/language-ai-eval -p 'test_*.py' -v
python3 tools/language-ai-eval/runner.py --fake --model control-model \
  --legacy-guide docs/agent-technical-reference.md \
  --v2-guide docs/language-v2/language-guide.md \
  --output /tmp/ai-harness-control.json
```

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

No external adapter or native validator is supplied or assumed. Their absence
is a remaining integration/evaluation dependency. Public fixtures, fake usage
counts and harness-unit tests must never be registered as external model quality
evidence or as Apple execution. A task suite being separate from supplied docs
does not establish that a model has never seen its published contents; record
that provenance separately before making a held-out generalization claim.

Adapter and validator files must remain byte-identical through the run. The
request identity includes the complete public model input and budget. Timeouts
terminate the ordinary process group on POSIX and the active child process tree
on Windows. This is lifecycle cleanup for trusted adapters, not an execution
sandbox for malicious adapters that detach themselves or evade process tracking.
