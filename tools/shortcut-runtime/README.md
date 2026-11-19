# Apple Shortcuts Runtime Extraction Tooling (v2)

This directory contains the reliable, sharded, timeout-guarded extraction pipeline for Apple Shortcuts runtime evidence on macOS runners.

## Architecture

- `jxa/`: JavaScript for Automation (JXA) scripts targeting Apple private frameworks (`WorkflowKit.framework`, `ActionKit.framework`).
- `runtime_probe.py`: Environment discovery, private framework load verification, and dyld shared-cache identifier extraction.
- `extract_parameters.py`: Sharded, timeout-guarded parameter orchestrator running isolated child processes with process-group watchdogs and incremental JSONL checkpoints.
- `extract_toolkit.py`: ToolKit SQLite registry extractor and summary generator (fixes shell/Python scoping bug).
- `merge_parameters.py`: Deterministic merger for parameter shard outputs.
- `compare_cherri.py`: Coverage analyzer comparing extracted Apple runtime surfaces against the Cherri action catalog.
- `normalize_snapshot.py`: Generates the versioned, provenance-preserving `apple-runtime-snapshot.json` and `apple-runtime-manifest.json`.
- `tests/`: Automated unit tests covering sharding, merging, watchdogs, sanitization, and comparisons without requiring macOS private frameworks.

## Mandatory REQUIRED encoding gate

`required-encoding-expectations.json` retains the existing gate's 22 required
acceptances and its required rejection of a wrapped subject by
`WFVariableParameterState`. These expectations are authored independently of
the observations. The seven synthetic input forms are passed to WorkflowKit's
parameter-state initializer and serializer by `jxa/verify_encodings.js`.

A missing class, native exception, missing discovery input, or unavailable
engine is `NOT_RUN` with a nonzero process exit. An actual rejected form cannot
satisfy an acceptance; an exception cannot satisfy an expected rejection.
Every failed REQUIRED expectation is retained in the report before the process
raises `E_REQUIRED_ENCODING_EXPECTATION`.

`required_encoding_gate.py` runs three real `/usr/bin/osascript` processes on
macOS: the committed correct set, an isolated copy changing only
`plainString.WFStringParameterState.string` from accept to reject, and the
restored correct set. The corrupt process must exit 1 with that exact
diagnostic and sole failing case. A timeout, signal, absent report, or generic
execution failure does not pass the negative control. The producer verifies
the exact source commit and pinned ShortcutKit prelude, retains actual
per-form matrices and process logs, and verifies that input bytes did not
change.

The macOS `Build & Test` job runs this mandatory step before the Go suite and
uploads `required-encoding-gate` even on failure. Download that artifact into
`artifacts/backend-recovery/ci-artifacts/required-encoding-gate/` for formal
evidence ingestion. CI records for CI01 and RP47 are emitted only after all
three real engine phases pass and the GitHub run/source identity matches.

Formal intake independently reads every phase report and process receipt. It
reconstructs each required verdict from the actual per-form matrix, checks the
canonical EXPECT identity and sole negative case, reconciles process argv and
exit with the summary, and checks the exact diagnostic in actual stderr.
Recomputed artifact hashes cannot hide contradictory contents; duplicate JSON
keys are rejected.

This gate checks WorkflowKit parameter serialization. It does not run a
Shortcut or establish iOS Simulator, physical-device, or editor behavior.
The Node fixture tests exercise process failure and report validation only;
they cannot produce Apple-engine acceptance evidence:

```sh
python3 -B -m unittest discover -s tools/shortcut-runtime/tests -p 'test_required_encoding*.py' -v
```
