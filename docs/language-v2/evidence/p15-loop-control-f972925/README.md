# F05 unsupported loop-control contract

Exact clean f972925 passed the actual production producer and strict intake tests (5 named test events, 0 failures/skips, 35.953 seconds). The producer executes two required root tests: one independently checks break, continue and while at top level and inside both supported repeat/for forms, and the other compiles healthy loops and ordinary keyword text. Each unsupported case requires an explicit diagnostic on the exact keyword and zero native output bytes; Stop Shortcut cannot be silently substituted.

The immutable F05 contract requires unit evidence only. The producer preserves that scope, checks the full 179-row accounting, and rejects missing/wrong producer/contract evidence. The separate intake controls reject the wrong tier, unrelated command and partial test selection. No compiler behavior, immutable contract, Apple runtime result or final-project status was changed.

The exact clean log, runner and summary are retained in the byte-verified archive. This scoped PASS will affect the overall total only after a complete later-source runner execution.
