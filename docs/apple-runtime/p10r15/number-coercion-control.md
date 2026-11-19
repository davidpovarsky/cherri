# Explicit numeric coercion control

Native run 44386191992 at 42e7d1c observed equality and inequality both true for the GetVariable-forwarded require zero and its mutable binding. Direct number(0) and nonzero controls behaved as expected. The independent audit preserves this failure and 20 published hashes.

This separate synthetic fixture changes only the four forwarded/bound comparisons to explicit .number operands, using the existing WFNumberContentItem coercion. The original failing fixture is byte-identical. Both operators use the same producer; direct number and nonzero observations remain independent controls. No compiler emitter, comparison policy, expected original result or native registry is changed. Local graph checks establish the controlled comparison shape and reject causal corruptions. Only a new signed Apple run can establish runtime behavior.

## Current-source diagnosis and fresh native observation (2026-11-19)

The [S71225 boundary diagnosis](native-followup-20261119/boundary-diagnostics-71225b7/README.md)
retains the original failures and current compiled graphs. It confirms distinct
numeric operator fields and four explicit Number coercions with independent
causal controls; no production comparison policy changed. Fresh native run
[44436273586](https://github.com/davidpovarsky/cherri/actions/runs/44436273586)
binds exact S71225 after the earlier no-callback infrastructure failure. Dispatch
and a zero-step queued state are not evidence of native success.
