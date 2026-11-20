# Value-control function discovery: causal repair

## Verified result

Clean source 8196315a89ba3e0b13d5424d93f859fabe51e657 passed 246 focused root tests and 69 tests in the entire lower package, with zero failures or skips. New coverage comprises 17 complete native graphs and 128 deliberate graph corruptions. Independent peer review found no concrete regression or oracle gap and did not substitute for execution.

## Cause and change

hasFunctionCalls visited statement-role controls but missed the same AST nodes used as expression values. Actual compilation emitted self-targeting Run Shortcut calls without a function dispatcher. Four expression switch cases now forward If/For/Repeat/Menu to the existing statement traversal. No native serializer, payload or dispatcher is duplicated.

Tests-only source 5bfa6823ac05b24a67ecb2d0d9bc70a679057f56 reproduced ten failing value-control cases and seven passing direct, statement and unused-function controls. Conditions, iterable/count/prompt calls, nested controls and Optional function results are included. Explicit main returns keep these fixtures independent of M02 fallthrough. Original failures and the uncommitted draft replay are retained.

Implementation 9d675c8ac60bf5f5716b15730185d8d1ffa1e082 precedes the provenance commit. The first checkout launcher stopped before mutation because its absent-ref exit-code check was wrong; that record is no test execution, not a compiler failure.

## Evidence and limits

The manifest binds every archived original byte to its retained path. Twenty explicit production/oracle dependencies are bound to immutable Git blobs at the clean source. CLI baseline observations use authenticated compiler S fd691b96fb661697a1cbaddab92476d457636ba9, binary SHA256 1ac6e6d6c55664a1c05545f7c6429ffb43448c20b3d4e4ad486ebb88e4e89370, and are not attributed to the repaired source.

This verifies generated structure. It does not execute Apple's runtime, establish Optional/List collection cardinality, complete T05/L01/F04/FN05, add formal179 credit, change the published consumer tuple, or replace integrated CI. Calls inside other lexical constructs are outside this bounded repair unless separately demonstrated.
