# R23 actual-Git fixture: one scoped execution

On 22 November 2026, the exact reviewed test candidate completed one offhost execution. The parent `TestP10CoverageAndGates.test_c5_06_r23_protected_ios_file_modified_fails_closed` passed, and each of its fixed eleven subcases reached a passing terminal event. The process ran from 02:39:56.031674Z to 02:39:56.397100Z, elapsed 0.36541682 seconds, exit 0, without timeout, under the authorized 60-second limit. Unittest reports 0.165 seconds for the test itself.

The source is the reviewed candidate SHA256 `d991e5ef311935d33be7b38a0c7ec18de332410e5a57b38a33e8b7717786c8bd`, patch SHA256 `2963c2eb362ad65e84795b361ba91a9346861d7cc314169b3dc72558f862ddc2`. It is not yet a published source commit. The ten import/data dependencies were fetched at immutable `db222ea190b90e875290b76fea5c6a5925ad89e9`; all eleven staged files were verified against their exact Git blobs and SHA256 before and after execution. Original `setUpClass` loaded its six original JSON inputs. The other 21 parent method ASTs remain unchanged and were not executed.

The test used a real ordinary Git repository with an explicit fixture baseline. No subprocess was mocked. A passive Python audit hook retained all 63 actual Git launches, all in the single owned temporary repository; it does not replace or modify the subprocess implementation. The only test patches set the owned `REPO_ROOT` and isolated environment. The fixture was removed by its owner, and the supervisor observed no remaining process-group members, no cleanup signal and no remaining fixture entry.

## Observed subcases

1. Explicit clean baseline: gate PASS.
2. First protected path, unstaged mutation: gate FAIL.
3. First protected path, staged mutation: gate FAIL.
4. First protected path, committed mutation with clean worktree: gate FAIL.
5. First protected path, committed exact restoration: gate PASS.
6. Second protected path, unstaged mutation: gate FAIL.
7. Second protected path, staged mutation: gate FAIL.
8. Second protected path, committed mutation with clean worktree: gate FAIL.
9. Second protected path, committed exact restoration: gate PASS.
10. Unprotected committed change: gate PASS.
11. Unavailable baseline: gate FAIL.

For the first ten subcases, the independent actual `git diff`/`git status` observations and the production gate's combined-output SHA256 agree. Both configured protected paths and the production fixed baseline were asserted before the test, and the production definition was asserted unchanged afterward.

## Retained originals

`actual-fixture-first-60s/receipt.json`: SHA256 `cc332d22e6a8da082d0c7d81d5b6c5f3fe911f06fe17d9d9971d89c5936395ab`.

| Original | Bytes | SHA256 |
|---|---:|---|
| stdout and identical structured events | 3,847 | `9799fea2fc7a5861373aac6153fa86bdb70f586aba624a1fc92588e8024d7c12` |
| stderr, complete unittest output | 266 | `0f9b232fa1f6f998dc1177278f485f90da62fe45a49c7f892659e63ef44ac0f3` |
| passive subprocess launch audit | 23,363 | `8ce534b5a56b36a1fd21c7b5dececc717ebf7f30a655f47e47a7c88c7feb7061` |

The intent, source-before/source-after lists, started-PID receipt, exact driver, source manifest, exact source inputs and all GitHub retrieval carriers are retained beside this report. This was the first and only execution of this fixture candidate.

## Scope remains open

This establishes the corrected test fixture's positive and negative behavior. It does not establish a passing result for the full 22-parent suite or for R23 on the actual Cherri repository. The production R23 gate, its two protected paths and fixed `76d722502e20db977e84a095444d0172442655b8` baseline are unchanged. The actual repository's committed 54,532-byte UI test file still differs from that baseline and still makes the historical gate fail. The source-based policy/disposition distinction in `SCOPE-MAP.md` remains necessary; this fixture does not grant a waiver or rewrite the rule.
