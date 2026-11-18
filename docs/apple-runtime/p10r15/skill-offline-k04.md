# K04 offline cache and documentation validation

Exact source22040eaddba7adfbce82dd80c0e071c439ff2964 is clean before and after production verification. The named formal producer executes the real compiled CLI and the existing shell setup/doctor paths; the immutable K04 integration contract passes. Its179-row execution audit explicitly denies inferred success to every unexecuted requirement.

## Causal repair

The previous offline setup reported ready when its docs directory did not exist. The new test reproduced that actual behavior before changing source. Offline setup now always validates required docs and returns the precise CHERRI_DOCS_MISSING diagnostic.

The existing compatibility suite now has18actual checks: the original9causal compatibility controls plus healthy offline cache, isolated schema and docs-commit faults with restores, incompatible cache with restore, missing docs, and observable network denial. The binary is real, its cached bytes must remain unchanged, and an unrelated bundled binary cannot mask the tested cache.

A separate healthy oracle and three intentional bypasses (schema check, missing-doc check, unexpected network command) were executed. Every bypass is rejected by the exact expected causal diagnostic. No network request was allowed to leave those fault controls.

## Exact verification and limits

The clean formal producer passed in137.843seconds including the parent process, and the existing10-scenario Skill self-test passed in186.969seconds. The latter uses its established controlled installation fixtures; its result is not relabeled as complete K02/K03 real-binary installation proof.

Initial finite-budget failures are retained. One full language-acceptance package run stopped at5minutes in the combined179-contract regression while runningCLI. A nested producer later returned failure after205seconds; the old runner discarded its nested diagnostic, so the exact cause remains unclassified. The runner now retains a bounded failure tail, and the real K04 shell producer has a finite6-minute Go budget /5-minute shell budget. Assertions, statuses and no-network barriers were not weakened.

K01/K02/K03/K05 remain separate. Full integrated package testing and the complete179 report must run on the combined candidate.

Receipts and original logs: evidence/skill-k04-22040ea/.
