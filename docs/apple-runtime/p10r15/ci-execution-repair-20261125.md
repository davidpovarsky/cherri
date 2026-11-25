# Complete-suite scheduling and owned process observation

## Observed failures

Build & Test 45545264693 on K1e02755 retained a real macOS root-package timeout at 900 seconds, while Linux's 30 packages and Nix passed. The root package had 1,210 completed top-level tests with 1,316.02 seconds of summed reported elapsed time; the longest 120.68-, 75.28- and 72.81-second subprocess tests already used the reviewed parallel-two scheduler. Several ordinary tests were still active or queued when the package deadline expired. This is a cumulative package budget issue; the preserved original does not establish a new child timeout.

That same run recorded an AI harness worker-cleanup failure with only a generic error. Its exact inner observation failure was not retained. A separate causal test demonstrates a general observer defect: an unrelated process group's platform-specific or transient state could reject termination of the adapter's own group. This defect is repaired here, but its role in the historical GitHub failure remains unproven until a fresh native execution.

## Changes

The complete ordinary suite now has an explicit 20-minute Go timeout per package. Every package still runs exactly once, with count=1 and parallel=2. The acceptance package keeps its separate 35-minute cumulative budget; all selected producer and child timeouts, assertions and fixtures are unchanged. The strict CI04 intake requires the exact new invocation and rejects both a substituted 30-minute value and the historical 15-minute invocation. Separate legacy-selector and OpenMinis 15-minute budgets are unchanged. Go's package timeout is not a whole-command wall-clock limit.

The AI adapter observer first identifies each numeric PGID, then validates state only for the exact owned group. Unknown owned states, ambiguous process identities, live processes and threaded zombies still fail or keep waiting under the original five-second cleanup limit. Unrelated host status fields cannot become evidence of a surviving adapter. Failure messages expose only fixed observer categories, never arbitrary exception text or captured provider stderr.

## Actual controls

- Complete-suite planner/environment/inventory controls: 9 Python tests passed.
- Exact CI04 original-invocation controls: 10 named Go events passed, zero failures/skips, 10.384 seconds including compilation. Receipt: /mnt/development/projects/evidence/cherri-p10r15-final-completion-20261125/ci-cumulative-budget-controls-03/result.original.json; SHA-256 5dfa87f824e34f89c46280a97127c2266fb4c43d511a46def5c564a46c224b81.
- The original observer rejected all three new unrelated-group causal cases. Its first experiment receipt miscounted matching traceback text; a separate read-only intake counted the three exact ValueError records without rerunning or replacing originals. Intake SHA-256 a8a594559ad1ddec7ac9200c4f1999ef288826e603b72a871650e8ca20326111, under ai-process-group-scope-red-intake-01.
- All 15 original offline AI harness tests passed after the repair, including actual owned worker lock/release/effect checks, the new unrelated/owned-state controls and safe exception-category control. Unittest duration 4.099 seconds. Receipt: /mnt/development/projects/evidence/cherri-p10r15-final-completion-20261125/ai-process-group-scope-green-01/receipt.original.json; SHA-256 107eea75ef5bdf36f53ad0d36ae0fe43fef38a1870b0d17168f93cc3566f890a.

No external AI evaluation was performed by these controls. No original failed workflow or acceptance report was upgraded. New full native CI, complete source-chain regeneration where required, and final acceptance remain outstanding.
