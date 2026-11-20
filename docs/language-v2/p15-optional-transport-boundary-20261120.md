# Current optional transport boundary — 20 November 2026

## Verified result and limit

Clean implementation b6501f30acb8be904d6c26c33fbd2f4ffe777ac1 rejects 19 concrete routes that previously emitted a lossy native value for explicit None or a possibly absent value. The rejection is a precise E_TARGET_VALUE_SHAPE error with the source span and function argument, Dictionary key/value, or List element context. The actual compiler produces zero final native bytes, including when a valid effect precedes the unsupported value. The editor service reports the same diagnostic.

This increment stops corruption in the current transport. It does not establish that Apple Shortcuts cannot encode optional values, nor complete T05, L01, FN05, or RP20. Independent per-value presence, distinct from argument omission/default presence, remains active development. No formal evidence producer, requirement status, immutable contract, runtime golden, or Apple receipt is changed here.

## Causal baseline

Tests-only source d5a884ed772ec53d90661dae17e84c1a66631856 first reproduced all 19 lost-value routes, but its separate lazy native-key oracle incorrectly required an OutputName on an intentionally nameless native attachment. Its mutation clone also used a JSON helper that changes integer types; those mutations never ran because the positive oracle failed. The original run is retained, and neither issue is called a compiler defect.

Tests-only correction 5f1108e9cddae03933da190510fdfee4315e228f requires the exact nameless UUID attachment and uses a native type-preserving clone. Every unchanged clone must pass before each mutation. The corrected clean baseline still failed all 19 optional routes; its 25 unrelated/present-value/lazy-key named controls passed, with no skip, in 27.953 seconds. Raw SHA256: 70de7f27bb6b54739d4982c408857004c82221c30ee420fb31e2798eb61a09ca.

All 19 original emitted native plists were extracted from the concatenated original Go test Output events, decoded as plists without rewriting, and retained with byte digests. Cases cover direct/named/function-result/forwarded/immutable/mutable function arguments, ordinary Map values/keys, nested Dictionary-array values, List elements, inferred None aliases, optional control joins, and an earlier effect. These are native byte observations, not Apple executions.

## Implementation

Authored None retains optional type identity instead of collapsing to Unknown, so inferred aliases and control unions cannot erase the absence fact. The ordinary analysis and checked-lowering boundaries reject a value that may be absent. Flow-proven present values continue to use their existing type facts; false, zero and empty Text remain valid, including explicitly optional bindings whose values are known present. A checked non-None branch and an omitted None default remain valid. Existing raw native parameter omission rules stay separate.

The existing codec descriptor and schema fingerprint remain unchanged for this guard: no new wire form is emitted. A subsequent lossless presence protocol will require its own documented descriptor revision and consumer tuple. The existing exported/native wire templates are untouched.

The lazy native.action computed-key controls exercise direct and nested Dictionary keys without an authored UUID. Exact producer order, UUID attachment, typed Dictionary wrappers and final consumer are checked, plus missing UUID, wrong UUID, missing attachment range, literal substitution, changed value and duplicate-producer controls.

## Clean verification

At the exact implementation SHA above, with a clean worktree before and after:

- 86 root parents, 567 named PASS, zero failed or skipped tests; 40.219 seconds. Original finite five-minute Go and 330-second process budgets were retained. Raw SHA256: 08efdbb700403129c1bf7b5ab784ee91a21f8e59e02752b53445a80c8277608f.
- Complete types, analysis and lower packages: 53 named PASS, zero failed or skipped tests; 22.234 seconds. Original finite three-minute Go and 210-second process budgets were retained. Raw SHA256: 5d6ae6ad2bd90d42da79a6d077a284a7348e8cee0ca200537e2ac894bda5562d.
- The earlier modified-source focused check had 102 named PASS; it remains explicitly a precommit check rather than clean-source proof.

The broad root selection includes existing function/default/recursion, argument-order/snapshot, container, computed-key, BRG09 native roundtrip, raw/native codec, JSON-null, None-clearing and empty-list tests. Exact commands, every original event and stderr stream, source identities, source cleanliness, budgets and digests are preserved in the adjacent evidence directory. No runtime credit follows from these local checks.

## Integration

After e9685734c5c6101f244f9c02f5f822373764ad23, apply d5a884ed772ec53d90661dae17e84c1a66631856, then 5f1108e9cddae03933da190510fdfee4315e228f, then b6501f30acb8be904d6c26c33fbd2f4ffe777ac1, then this documentation-only proof commit.

Evidence: [original receipts and manifest](evidence/p15-optional-transport-boundary-20261120/manifest.json).
Manifest SHA256: 4ef8a3560ea59f0e6174c3058a8d43006606524b4ab97b445ce2ada9564facb0
