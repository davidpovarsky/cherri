# REQUIRED encoding gate: local causal and intake evidence

Source: `50aa0d298bbff416dbea14cb8e44c15b5c5e3f0e`.
Baseline: `ddc6c020806e8e2edad727d07d7d78ea13d7bfe7`.

## What actually ran

The real canonical JXA source was executed in separate Node child processes
using a clearly test-only engine fixture. With the same modeled rejected
`WFStringParameterState/string` input, the original source exited 0 and printed
WARNING; the repaired source exited 1 and reported the exact
`E_REQUIRED_ENCODING_EXPECTATION` case. The fixture never loads Apple
frameworks. These are process-level causal observations, not native acceptance.

On the exact clean source commit, 75 named Go intake/verification cases passed
with zero failures/skips, and 12 Python/Node process/protocol/actual CLI tests
passed. They cover incomplete/contradictory per-form reports, missing classes,
engine exceptions, generic failures/timeouts/signals, no discovery, altered
expectation identities, missing artifact/run/hash bindings, a missing positive
exit field, and preservation of existing output evidence. Working-tree logs
are retained separately from the clean-commit repeat.

The generated map remains 179 requirements: 94 acceptance, 52 repair, 33 gates.
Only CI01/RP47 have a new source mapping hook. Regenerating the full map also
updated 47 previously stale mappings; the initial two-row-only diff assertion
failed and that finding is preserved in `map-refresh-observation.json`.

## Native status and scope

Actual WorkflowKit execution is **NOT_RUN locally**. CI01 and RP47 remain
unverified until the mandatory macOS Build & Test producer executes the real
correct/corrupted/restored expectation triplet on a pinned prelude and emits
source/run/digest-bound evidence. Local fixtures emit no formal native record.
No Shortcut runtime, Simulator, editor, or physical-device pass is claimed.

The source retains 23 independently authored required cases: 22 acceptances and
one required rejection. The actual macOS producer changes exactly one EXPECT
in a fresh isolated input, requires its sole exact diagnostic with exit 1,
runs the original correct set again, preserves all phase reports/logs, and
checks that source and input bytes did not change. Infrastructure failures and
missing reports cannot satisfy the negative control.

All archived files are public synthetic tests, gate source, or their bounded
logs. Raw bytes are preserved by the local Git attributes and listed with
SHA-256 in `manifest.json`.
