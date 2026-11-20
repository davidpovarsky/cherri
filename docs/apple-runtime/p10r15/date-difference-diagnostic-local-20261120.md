# Native Date difference diagnostic — local construction proof

The first original label case produced a fresh Shortcuts callback whose two Date differences were empty instead of the required zero. The independent direct source-Date difference was also empty. The remaining 48 fields matched. This is a failed exact oracle; it does not yet establish corruption by debugLog.

This additive public fixture isolates Date creation and the interval action from logging. It observes three specified dates, equal and positive intervals in Seconds and Minutes, a same-producer interval, and an independent number(0) text control. All 27 fields retain actual type/count/format/text outputs. Original label fixtures and expected values are unchanged.

## Actual local verification

Clean source 1f1d8293f12ce885470dd1c16c86a167fca7cc66 built a fresh compiler, guarded by exact Git HEAD and clean status before and after execution. The real source compiled to 67 native actions. Source and expected-contract hashes are fixed, with exact operand UUID and result-consumer checks before signing.

The clean local replay passed 23 mutations of the compiled graph, five source/contract tests and eight strict callback/dispatch tests. The callback controls reject empty or changed values in each of the 27 fields, stale/foreign identity and missing trace. The immutable-source case resolver was exercised from the committed Git object. Companion JSON records the original receipts and digests.

An additional build-info check found no embedded VCS revision fields, including after one explicit buildvcs=true rebuild. That rebuild produced the same compiler digest. The absence is retained; source provenance rests on the real checkout/command/artifact receipts, not a claimed embedded stamp.

During draft construction, the graph guard rejected the unreviewed getType action and an omitted count-unit parameter. The reviewed fixture uses the already exercised typeOf action and explicit Items counts. These were draft diagnostic corrections, not observed new Apple failures or production compiler repairs.

## Native scope remains open

The existing strict QA workflow has one additive selector. The public source/hash gate, nonce, reviewed signing path, callback/trace oracle and artifact publication policy remain intact. No native result is recorded in this local proof.

The formatted Date target is explicitly UTC. A different native offset or value remains visible and fails the exact target; it is never normalized into success. Interval equality alone does not prove Date log retention. The fixture deliberately makes no debugLog call.

The original twelve-case label matrix and startup timing replay remain untouched. This diagnostic grants no formal requirement, final-candidate or physical-device credit.

- [Original label run](https://github.com/davidpovarsky/cherri/actions/runs/44672249369)
- [Public diagnostic source](../../../tools/shortcuts-simulator-qa/fixtures/qa-date-difference-diagnostic-v2.cherri)
- [Independent expected contract](../../../tools/shortcuts-simulator-qa/fixtures/qa-date-difference-diagnostic-v2.expected.json)
- [Local proof](date-difference-diagnostic-local-20261120.json)
