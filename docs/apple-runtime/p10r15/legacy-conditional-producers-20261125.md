# Legacy conditional producer qualification — 25 November 2026

This checkpoint qualifies the actual compiler paths for Legacy conditional inputs and operands. It is not a full 179-requirement acceptance result, an Apple runtime result, or a release approval.

## Source

- Branch: agent/cherri-p10r15-legacy-conditional-inputs-20261125
- Input materialization: 4272601210a00c4bd9f090a3b8dd403660e6a145
- Operand, expression-boundary and source-order production fix: 30dc1f13e676fc8246c1154e6997ae1d9e2df03c
- Qualified timer identity test correction: ddca18e3c1a9e3dc151722383e1552c406215a3d

All 337 production Go/module inputs are identical between the last two commits. The latter changes test code only.

## Production behavior

Literal, raw text, Boolean, number, dictionary, arithmetic and action inputs now use the existing typed constant/action producer path before the conditional consumes them. Native references are checked rather than blindly cast. Unsupported or absent internal values produce a compiler diagnostic.

Written action and arithmetic operands on the right, including both range bounds, are evaluated once. The conditional expression reader stops at the matching closing parenthesis. Earlier mutable operands are captured before a later operand can change them. Float equality uses the existing numeric encoding. Void operands fail at the value boundary.

The original authored fixtures are retained. Both iOS 17 and 27 targets and Debug/Release profiles are covered. Ordinary single-condition native encodings, explicit modern pure multi-condition encodings, import round trips and existing control placement checks remain in the selected regressions.

## Actual executions

Evidence root: /mnt/development/projects/evidence/cherri-p10r15-final-completion-20261125

| Original receipt | Actual result | SHA-256 |
| --- | --- | --- |
| legacy-conditional-inputs-red-02/result.original.json | 70 failing leaves before the input fix | 8444f662c8915001dffd4ec20143e216f02c8f0e32f5ba9b64a0c1fcc22d6f44 |
| legacy-conditional-inputs-green-02/result.original.json | 81 named test events passed, including 70 input/placement leaves and eight graph faults | 5d5b638c01337ced19200fe9e79f91c3c39a030f14e8f677ad8150cc7c80a305 |
| legacy-conditional-operands-red-01/result.original.json | 58 failing leaves before the operand fix | 8e1f959e24b5f51cf2654bdbcdfb65bc468eaa2e48fbed4a4d5bcb63e2314cd0 |
| legacy-conditional-operands-green-01/result.original.json | 196 named events passed; two Debug snapshot leaves failed on private timer names | 2e8ab9962aae5126a1a57025ca75813f7cf9c013b50c207634287255155ac6cf |
| legacy-conditional-debug-timer-green-01/result.original.json | All 14 selected snapshot/timer events passed in 26.664 seconds | 062a9a728cda2b3740df1f8ee4afa3de26aac24a7d6ea584a81fa2204ac42a4b |

The broad operand execution remains recorded as failed. Its two Debug differences were the compiler-owned timer declaration and its matching native Variable reference: their hygiene salt depends on source position. The subsequent oracle correction recognizes exactly one Current Date timer in Debug, no timer in Release, a unique declaration and exactly one later matching read. It never rewrites equal-looking literal text. Six malformed graph controls are rejected; consistent private renaming is accepted; a literal spelling difference remains visible. All four original snapshot sources are unchanged.

No listed successful execution skipped tests, exceeded its time limit, crossed the live capacity floor, or changed source during the run. Named-event totals include parent tests and are not requirement counts.

## Resource continuity

The last execution used the exact verified warm Go cache at /dev/shm/cherri-p10r15-go-build-cache-20261125. All 7,764 original cache files, including executable entries, were copied with matching hashes, owners, modes and timestamps before their disk copies were retired. The old default cache directory remains valid and empty; no persistent environment or symlink depends on RAM lifetime. Source, evidence, modules and Go temporary work stayed on disk.

Relocation receipt: reproducible-go-cache-ram-relocation-01/result.original.json, SHA-256 c5da832ac9213f54152cbc88cc9240149658a5173db499e83ddbea48e6d411f6. Every new run explicitly selects this cache and checks disk, shared-memory and available-RAM admission and live floors.

## Remaining work

Compound Legacy conditions on old targets and lazy evaluation of later effectful conditions need a separate causal qualification. These are not covered by the existing pure-modern filter parity check. The final integrated source must still run the complete source/report chain, release gates, CI, private corpus and original acceptance suite. Actual Apple runtime evidence remains separate.
