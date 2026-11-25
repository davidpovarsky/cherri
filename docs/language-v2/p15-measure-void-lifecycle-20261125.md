# P15 measure: shared value-or-Void scoped lifecycle (2026-11-25)

This checkpoint implements statement-only `measure` bodies in Legacy and V2 using the shared backend lifecycle. The actual focused GREEN run passed on implementation source a7847eb575e17c701a2b0a44434c0997f30f14fd after the preserved RED attempts.

## Contract

A measured body is generated once in source order. Existing value bodies keep their original producer/snapshot/forwarding contract. A statement-only body has analyzed Void type and no synthetic result binding or native result reference. It ends at one parameter-free native Nothing after timer diagnostics, in both build profiles. This boundary prevents timing output or preceding implicit input from becoming Repeat Results. Nothing actions already present in the body remain untouched.

Debug retains the original timer start and stop/log chain. Release strips that instrumentation while retaining source effects and the semantic Nothing boundary. Native abort actions retain their usual control behavior; these compiler callbacks do not establish runtime try/finally, cancellation cleanup, or guaranteed timer completion after a native stop.

Void may be a statement but cannot initialize a variable, become an argument/return/yield, or be forwarded as a value. A possible implicit control value (including Unknown) retains the existing explicit-result diagnostic. This does not grant new support for parameterized blocks, closures, or Legacy control expressions as typed values.

## Actual RED receipts

- Original source e16be592e07c6bfab6093b0fa3cea07d7bdb9872: 72 semantic RED leaves, 4 ordinary-control setup failures, and 4 existing value controls passed. The attempt remains UNEXPECTED_RED_RESULT and is not relabeled.
- The 4 Legacy fixture corrections at 911f74df633a09ac0d06dfaea6b105ca8d6e8ef8 were replayed separately: all 4 produced the expected semantic RED. The correction adds the existing sharing include and changes the ordinary condition to an authored named constant. The literal-left Legacy condition panic was separately handed to the root parser owner.
- Original RED receipt: /mnt/development/projects/evidence/cherri-p10r15-macro-void-blocks-20261125/red-01/result.original.json (SHA-256 3d4190a1254a99e76de2685bfeb6bedec665a6c32e7f77f8110f5df66e17d58e).
- Corrected-four RED receipt: /mnt/development/projects/evidence/cherri-p10r15-macro-void-blocks-20261125/red-corrected-02/result.original.json (SHA-256 f476177dbebd951006540522ded8c880a4a2cfa46025445eb670b70b7d829a5b).

## Test evolution and actual qualification

The 76 new body/loop sources and 4 existing value controls remain present. The oracle compares the complete real native action graph to an ordinary-source graph containing the exact semantic Nothing boundary; it only removes paired, causally bound timer instrumentation. It checks timer intervals, references, exact wire shapes, effects, and order. Deliberate graph faults must be rejected after the healthy producer is accepted.

The two exact historical bare Legacy empty/wait sources now test positive graph admission under their original test IDs. Historical bound empty/wait sources remain rejected with the existing E_BLOCK_VOID_VALUE contract. Original source files and expectation changes are preserved in the evidence directory. All other old rejection cases remain.

The actual run passed 254 named tests in the root, backend, analysis, and lower packages with zero failures or skips. Its inventory includes all 76 original body/loop leaves, 4 existing value controls, 40 consumption/control negative leaves, and 52 rejected causal graph faults. Backend stage-error/order controls, exact Void type checks, and original measure regressions also passed. The 254 count includes parent tests; these figures are not additive independent acceptance requirements.

Original receipt: /mnt/development/projects/evidence/cherri-p10r15-macro-void-blocks-20261125/green-01/result.original.json, SHA-256 fbddaff2d6c92548deb8535112779a014e1501f013acf26ae3573370095abcd4. Sentinel job job_8b1cc31db90d completed in 29.47 seconds. All 835 tracked Go/go.mod/go.sum input hashes matched before and after. The preserved receipt and qualification summary are in evidence/p15-measure-void-lifecycle-20261125/. This checkpoint does not claim macro acceptance, native Apple runtime validation, final CI, 179/179, or an updated live compiler/Skill.
