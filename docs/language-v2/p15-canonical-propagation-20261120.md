# BRG24 — canonical source behavior propagation, 2026-11-20

## Verified result

Clean source `a846e9046a400806dfb44eaf3bd82d3879bb7ca2` passed the actual BRG24 production producer and nine detached/incomplete receipt controls in 104.265 seconds. The selected report contains **1 PASSED, 0 FAILED, 0 skipped**, with `closure_complete: false`.

This proves the repository, integration and native-structure tiers of the distinct BRG24 scenario on that source. It does not change the previously published R2 report, grant other IDs credit, or establish Apple runtime behavior.

## Actual production exercise

An owned detached Git worktree starts from the committed source. A freshly compiled child uses the real canonical Legacy reference compiler and the production V2 compiler for three fixed public programs. Their complete native parameter maps must be equal, apart from the top-level generated UUID.

| Actual source edit | Observed native effect |
| --- | --- |
| Static declaration in `actions/crypto.cherri` | Both rebuilt compilers change only `WFEncodeMode` on Base64 Encode. |
| Real callback in `actions_std.go` | Both rebuilt compilers change only the returned PDF text type. |
| Shared reference encoder in `shared_codec.go` | Both rebuilt compilers change the exact ExtensionInput reference type in all three native programs. |

These marker values are deliberate source propagation controls. They are never sent to Apple or presented as valid shipping action metadata.

Each independent edit is followed by a fresh process and actual native-tree assertions, then restoration and another rebuilt process. The restored trees are exactly equal to the baseline. V2 encoding and the generated registry are not edited. The tracked diff is checked to contain exactly the one intended canonical path, and both the disposable and original working trees are checked after cleanup.

Controls reject a stale canonical route, a stale V2 route, and matching unrelated corruption in both routes. The formal producer separately rejects zero selection, a different producer, missing required-test assertion, stale source, each missing tier, invented runtime evidence, and another requirement ID. The full 179-row audit cannot grant an unexecuted row a PASS.

## Original run history

| Stage | Exact source | Actual outcome | Raw Go JSON SHA-256 |
| --- | --- | --- | --- |
| Initial baseline | `4d93d34ba04c5caff13a2a99858baa1bdd3bd9f3` | Failed before mutations: the new test included `actions/basic` a second time. This was a test setup error, not a compiler defect. | `1a4474a247b572adcbe48f5fbb62869ab4d5d8e1a0015c4de6ac40887d848d03` |
| Corrected complete source exercise | `33ca6f7cc2a523d4cf1647c534705e3bbdd9ffaf` | Four named PASS events, no failure or skip, 70.984 seconds, clean before and after. | `16b29d8df42e5212ac286b69ceaea5548ff393ad67ee087b775514e55c963d19` |
| Formal producer and exact-path check | `a846e9046a400806dfb44eaf3bd82d3879bb7ca2` | Ten named PASS events; one full three-tier requirement, nine receipt corruption controls, clean before and after. | `1e4318714ce1b159e2e8a41139f25d669c78713279fd0331b6bd688e66adf97e` |

Original process starts, results, Go JSON and stderr are retained under [the evidence directory](evidence/p15-canonical-propagation-20261120/). The derived producer receipt is extracted from the joined original Go output, not substituted for execution. `sha256-manifest.json` binds every retained original byte.

The unchanged original contract digests remain:
- acceptance: `9fdfa8910b09824dc61395d8ddf4aa8fe8914619af0fdde59c63b67ab10b895e`
- repair: `eba070df0e1ff229a94e52d977a09c2a0c125a31b1a69caf32787d41600074f5`
- recovery: `e7fbb3ad177c5142935f71a46ca7e89b84822061bfc0919eec59708811ab7e7c`
