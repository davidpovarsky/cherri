# Darwin process-group observation and cumulative CI budget

## Qualified scope

The source checkpoint is e082d1430fc519af98c31f19fe12c30a9d9ad629, based on the original b50aedb203cdbadb1d478dc7a6c6157261896946 Build & Test source. These are source-bound component repairs. The original whole workflow remains failed; a new full integration CI execution is still required. No provider request or AI quality measurement is part of this work.

## Exact owned-group observation

A real owned process group can disappear between the initial kernel probe and the bounded process-table snapshot. A missing owned row can also occur while that group is still live. Fourteen controls use actual new sessions, readiness, kernel probes, SIGKILL and wait; only the independent ps snapshot is substituted.

The original production source failed six disappearance cases and incorrectly accepted the live group with an omitted row. The repair accepts a fresh ESRCH for that exact group only within the existing deadline. Failed snapshots with an existing group still fail closed. An omitted row now also requires kernel absence. The original one-second snapshot and five-second complete cleanup limits, fixed public error categories, original TimeoutExpired identity after established quiescence, zombie/live-thread guards and descendant lock/effect assertions are unchanged. There is no retry loop or larger cleanup budget.

| Execution | Actual result |
| --- | --- |
| Oracle causal RED, test-only 3b2be6b | 7 parents; 1 failure and 6 errors; 2.11 seconds |
| Mac causal RED, identical source | 7 parents; 32 passed events and 7 failed events; 2.07 seconds |
| Oracle GREEN, source 3b398d0f | 20 original parents; 80 passed events; 4.09 seconds |
| Mac GREEN, identical source | 20 original parents; 80 passed events; 3.40 seconds |

All four source files match their immutable Git objects before and after execution. The original unmodified Mac descendant-timeout test also passed once before the repair. Consequently this reproduces and fixes concrete observation races, but does not claim that the historically unretained ps failure mode in b50 has been identified.

## Cumulative acceptance budget

Original run 45604092670 completed all 29 ordinary packages on Linux and macOS; Nix also passed. Linux executed all 302 acceptance parent tests in 1545.707 seconds with the retained editor failures. macOS completed 272 source-owned parents, whose reported durations sum to 1842.02 seconds, then reached the cumulative 35-minute Go timeout with the supplemental parent active for 258 seconds. These sum to 2100.02 seconds, consistent with the actual 2100.075-second package timeout. This is aggregate exhaustion, not evidence of a child producer timeout.

The 30 remaining parent executions required 403.78 seconds on Linux. The weighted duration ratio over the 272 common completed parents is 1.613288, giving an estimated complete macOS duration of 2493.43 seconds (41 minutes 33 seconds). This extrapolation is explicitly not an execution. A 50-minute cumulative acceptance budget provides about 20 percent headroom over that observed-source estimate; the next whole run must measure its actual margin, including new scenarios.

The helper still runs every package once, always runs the second group after first-group failure, and preserves failure statuses. Ordinary packages retain 20 minutes and the reviewed parallelism of two. Acceptance remains sequential with the inherited subprocess-parallel marker removed. All test selections, assertions and per-producer child bounds are unchanged. Existing BRG30 contracts require independent fresh joined execution, so no cross-scenario memoization substitutes previous results.

Both Full Go steps and their Linux/macOS jobs have no explicit shorter outer timeout. GitHub documents a default job limit of 360 minutes in its [workflow syntax reference](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax#jobsjob_idtimeout-minutes), checked on 25 November 2026. The separate 12-minute WorkflowKit step is unchanged. CI04 continues to validate the unchanged ordinary 20-minute command.

Ten Python orchestration integrity tests pass. Sixteen causal mutations reject shorter historical, longer, zero/unbounded, absent, duplicated, split, metadata-mismatched and alternate timeout flags before execution. The initial 17 failing assertions are retained. No Go test was invoked by those controls.

## Original evidence

All paths below are on Oracle under /mnt/development/projects/evidence/.

- cherri-p10r15-ci-release-20261125/darwin-observer-race-red-01/result.original.json — 16dd8efd7efd590397407907b50277b8e14b771b3c8afec15a04cf5f65bb65f9
- cherri-p10r15-ci-release-20261125/darwin-observer-race-green-01/result.original.json — a1e9baf3fd19422fd354802a67a633acb3e0a1f4e54e70558e81e894837ad23c
- cherri-p10r15-apple-completion-20261125/ai-observer-race-native-01/originals/red/result.original.json — aaa42906a15b933f752cd8ad7712cbdf84505205bce8d53be7cca684397728c9
- cherri-p10r15-apple-completion-20261125/ai-observer-race-native-01/originals/green/result.original.json — 95aed635b9bd15bd7e1a0a326aadad63039fbf3d80778ab28fffea58e29d2a00
- cherri-p10r15-ci-release-20261125/acceptance-cumulative-budget-analysis-01/analysis.original.json — 14cd6c3cec976099731723304edc4aaa510c754cb4f7894d4f3844a3e0d12cd1
- cherri-p10r15-ci-release-20261125/acceptance-budget-controls-red-01/result.original.json — dafa6f9e4ce12b6d7578a9e3a21c2ad815e16e7cd0f13dd38b6a0986f590a749
- cherri-p10r15-ci-release-20261125/acceptance-budget-controls-green-01/result.original.json — 66cfe3892fef8058c00670399b9607471f5aec7053ca352e71bfbe25886b5d53
- cherri-p10r15-ci-release-20261125/ci-execution-repair-qualification-intake-01/result.original.json — 1b5df0ba6f31b0cc787f61bce6f0205e216f823ecde078feb2ed939478a19cd3
