# Intermediate full Go run: incomplete after orchestrator timeout

Source: `4ff1d089ec49528bb796fe315a74777d69008d9b`, an ordinary non-shallow clone with historical oracle objects present. The tree remained clean. The definitive host job timed out after 2700.3 seconds and did not write a completed test summary. This archive is explicitly **INCOMPLETE_ORCHESTRATOR_TIMEOUT**, never a full-suite pass.

The preserved raw JSON contains 3165 distinct named tests started, 3151 pass events, seven failure events, and five skips. Two tests have no flushed final event. Buffered output means this is a lower bound on later progress, not a claim that the last printed test hung. Process observations after host timeout found no surviving Go/test/runner processes; no manual termination was necessary.

The seven failure events are two already diagnosed defect families: native rawAction integer/real category loss, and the stale T06 expected diagnostic after stricter yield-path validation. Both were subsequently repaired and focused-tested before combined source `b34ae47`; this old partial run cannot establish the final source result. Private corpus, macOS signing, and opted-in live signing were excluded. The external docs fixture interval is disclosed in its own note, so this run is not final docs acceptance.

`raw-evidence.zip` retains exact original bytes including the runner, raw test2json log, host timeout, setup metadata, and process observations. `raw-file-sha256.json` binds every zipped member. Readable JSON copies preserve semantic content with repository line endings. No requirement or acceptance count was edited.
