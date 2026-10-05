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
