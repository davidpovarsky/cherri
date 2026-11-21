# Bounded Simulator QA acquisition and terminal evidence

## Verified scope

The source component is `3c433277442cc3407e189b35d8b9b8e462ec1ac5` followed by `6ca4493a8d2fac1cf0ba27675dd40c365d2f412a`, based on `f05a6221b44582cb04bb085d254df9ff25cf64a8`. The clean final-source replay discovered 98 tests: **94 passed, four explicitly POSIX-only controls were skipped, and none failed**, in 117.609 seconds including subprocess startup. All six implementation/test file hashes match the reviewed snapshot. The worktree was clean before and after the replay.

This is a local acquisition, privacy and provenance repair. It grants no Apple execution, signing, UI, Shortcut result, or formal 179-requirement credit. The original 900-second test02 phase, 45-minute workflow job, test commands, selected fixtures, callback/value/trace oracles, and signed-provenance boundary are unchanged.

## Reproduced problems

At the unchanged base, two actual shell/collector controls demonstrated incompatible treatment of a retained destination-query original. The final summary privacy scan treated its quarantined device identifier as public content. The final cleanup then removed the original build log, although the strict destination receipt validator must reopen that exact file to check its byte hash and parsed observation. The original failure stderr is retained with SHA256 `055fbf96b43d875087fca00eaf54cf57848d29358a8ea934f3582725466804ef`.

The initial component passed its then-existing controls, but independent peer review found a further evidence-identity gap. Four actual controls at `3c433277` produced 11 failures: existing assertion files could carry another nonce, run, compiled source, unsigned plist or profile; existing summaries could carry another fixture or profile; the exclusive-write race had the same assertion gap; and the always-running publisher could continue after finalization rejected identity. The original stderr SHA256 is `6baa0c52f8d96ae33482e48aaa892a630cdb6129006c26be28deb200ad9f409c`. The follow-up repairs those boundaries without rewriting any existing evidence file.

## Acquisition behavior

Three exclusive private checkpoints record observed acquisition: compilation, test02 start and test02 completion. They bind the exact compiler manifest, source commit, workflow run, fixture, source and unsigned-plist hashes, profile, current nonce, observed setup gates and original monotonic phase clock. A test process exit of zero remains an observed process result; it is never substituted for successful import or callback execution.

The three previously unbounded post-test02 inspections now use the original phase's remaining time: two `xcresulttool get test-results summary` queries and the receiver's `simctl get_app_container` query. Each gets at most 20 seconds and never extends the original 900-second allowance. Separate original stdout and stderr files preserve partial bytes, actual exit, timeout or cancellation, stage and elapsed time. Missing, invalid, oversized or late output stays incomplete. The process runner terminates its owned process group on interruption; the actual POSIX descendant-control replay remains explicitly pending on this Windows host.

The strict receipt validator reopens the original files, checks their exact path/size/hash, replays parsing and checks deadline arithmetic. It rejects forged success, duration understatement, reordered stages, traversal, unavailable originals and a changed acquisition context. A durable attempt marker prevents another execution after a receipt-write exception.

## Terminal behavior and evidence identity

An interrupted acquisition may produce a clearly incomplete terminal record and conservative summary. It never manufactures a callback or native trace, infers Void from absence, promotes test02 exit zero, or overwrites a completed original receipt. Receipt publication uses an exclusive hard link only after a private temporary file has been fully written and fsynced; interrupted writes remain quarantined.

The existing storage cross-run route legitimately advances from the writer nonce to its reader nonce after test02. The shell therefore writes one immutable, source/run/fixture-bound output-emitter context immediately before the existing assertion writer. This records its actual current nonce; it does not introduce a fixture exception or permit an arbitrary borrowed nonce. If the writer was never reached, the existing test02 checkpoint remains the nonce boundary.

Both normal and exclusive-write-race paths validate the complete assertion identity. Existing summaries validate their fixture/profile and corresponding assertion identity before being retained. The output-emitter original is hash-bound into the terminal receipt. Changed, missing, foreign or future context is rejected. The workflow also stops publication when this finalizer rejects identity; an existing JSON file cannot bypass that failed gate.

## Privacy and original retention

The normal summary and terminal finalizer use the existing explicit synthetic-text publication boundary. Original destination and post-test query logs remain at their original private paths for strict validation. They are quarantined by the existing publisher and are never added to the public allowlist. No raster, system log, raw plist, archive, symlink, secret, custom signing endpoint or device identifier is admitted by this change.

Public artifacts add only the bounded post-test inspection and incomplete terminal JSON receipts. Their own validators still require the exact original bytes and source/run/fixture scope. Decoded JSON canaries, duplicate keys and escaped private values remain blocking controls. Missing required data remains blocked.

## Actual verification

The focused peer-repair replay passed 39 tests with one explicit POSIX skip. The complete precommit replay passed 94 of 98 discovered tests, with four explicit POSIX skips. A separate clean-source replay at `6ca4493a` repeated that same result; original clean stderr SHA256 is `65179bbb8f4199b38b38311eaaaab3f31d18ca33aaba41c482449237a028aed9`.

The actual command was:

```text
python -m unittest test_terminal_receipts test_simulator_session_recovery test_xcode_destination_readiness test_public_artifacts -v
```

The tests use the actual Git Bash executable on Windows, not an accidental WSL interpreter. They exercise the actual summary heredoc, cleanup command, checkpoint wrapper, output-context shell call, source-owned CLI, publisher-step guard and publication policy. Real subprocess controls preserve independent stdout/stderr and timeout behavior. Git replacement objects, dirty helper bytes, borrowed checkout identity, interrupted fsync, stale assertions, late completion and changed raw inputs are rejected. Git diff validation, Bash parsing and workflow YAML parsing passed; the job budget remains 45 minutes.

The four skipped tests are individually recorded in the JSON proof. They concern POSIX process-group interruption and actual POSIX shell routes. They are not counted as passes. Peer review inspected the actual follow-up diff `e3812e82485906de11bb4cc86d10b2b560213803ae9faad4724664c747d56489` and found no remaining concrete regression in this bounded change; that review is distinct from execution.

## Original native timeout remains a failure

The separate unchanged File UTF-8 diagnostic run [44734439652](https://github.com/davidpovarsky/cherri/actions/runs/44734439652), source `e4b648d4d30dae265e02a5be325e167c81934231`, remains **CANCELLED**. Authenticated GitHub metadata reports the 45-minute job limit. Its publisher reported `SUMMARY_MISSING`, and the run has zero artifacts. The original job log is 140789 bytes with SHA256 `169d70659cbc9920e9de949f7427791ae89c526d653e295179a3d601780bcf7f`.

The last observed phase was the consent-confirmation helper. That helper has its own 120-second limit and kills its process group on its own timeout; the GitHub cancellation appeared about 65 seconds after the phase marker, before that allowance expired. This does not establish a helper timeout or a JSON/body/null result. The earlier Optional c0 cancellation has a different retained stopping point and remains unlocalized. Neither failure was converted into success or rerun unchanged.

The accompanying JSON proof contains source-file digests, exact local commands and retained receipt digests. It contains no private log payload. A changed-source Apple run is still required to verify the new acquisition route on macOS.
