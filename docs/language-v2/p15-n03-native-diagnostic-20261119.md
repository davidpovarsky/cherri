# N03 text equality: public native diagnostic

## Verified scope

The immutable N03 requirement is **Exact case-sensitive text equality and legacy comparison migration**, at unit plus native level. The existing acceptance smoke only requires nonempty IR; its assertion-scope guard correctly prevents formal credit.

This change adds a public diagnostic and an independent native-graph oracle. It changes no compiler, migration, native action definition, acceptance mapping, or existing fixture. It does not establish Apple runtime behavior or legacy migration, and it does not close N03.

Implementation source: `a2cc53fc5fc9e827e65420266ce2c0848fa527b9`.

## Why this diagnostic is necessary

The current compiler folds literal `"abc" == "ABC"` using exact Go string equality. Dynamic Text comparisons use established native If condition codes 4 and 5, with the existing compatibility flag. The same flag is also used by ordinary authored If conditions. Source inspection alone does not establish whether Apple compares those Text operands case-sensitively.

Migration currently maps the old explicit `if legacy` spelling to ordinary V2 `if`. Any future change to ordinary equality must therefore also preserve old comparison behavior explicitly. The native representation flag must not be treated as an invented case-sensitivity switch.

No case-only native observation was available in the inspected existing receipts. The first experiment must preserve actual values and identify the cause before changing this shared lowering path.

## Source and independent controls

Fixture: `tools/shortcuts-simulator-qa/fixtures/qa-n03-text-equality-diagnostic-v2.cherri`.

It uses only synthetic operands `abc`, another independently produced `abc`, and `ABC`. Three Base64 actions observe the same producer UUIDs as the comparisons. The 18 output fields comprise:

- Three independent Base64 witnesses: `YWJj`, `YWJj`, `QUJD`.
- Four constant-folded equality/inequality flags for identical and case-only pairs.
- Four dynamic equality/inequality flags for those pairs.
- Four ordinary native If flags with literal right-hand operands.
- Three canonical Match Text counts with case sensitivity explicitly true or false.

The Match Text pattern `^abc$` is a control for these fixed ASCII subjects. It is not a proposed general implementation of exact Text equality. Every flag starts as `N` and changes to `Y` only inside its actual conditional branch. The output transports the original flag variables, counts, Base64 witnesses, and before/after trace.

The marker `CHERRI_QA_N03_TEXT_EQ_DIAG` identifies a diagnostic. It does not assert successful equality acceptance. The checked-in expected JSON records the exact-equality target separately from observed runtime data, with `runtime_status: NOT_RUN` and an empty `formal_requirements_proved` list. Contradictory Apple results must remain available for diagnosis.

## Verification

The public source compiled successfully using the independently identified frozen b296 compiler (SHA256 `d732c4cad8937d4b0cfe06c3f001704642ad16a5029474c997a53ff20b223042`). A separate Go production compilation on the committed source emits 121 native actions. The graph oracle checks the complete comparison and observation path, and constrains the surrounding trace transport actions.

The focused command was:

```text
go test -count=1 -json -timeout=8m -run ^TestP15N03TextEqualityDiagnostic .
```

The clean replay at the implementation commit exited 0 in 2.875 seconds, with **27 named tests / 3 top-level tests passed**, no failures or skips, and a clean worktree before and after. The final package event is PASS.

The graph oracle independently specifies the exact operand producers, Base64 inputs, folded values, comparison codes, native compatibility field, typed right-hand inputs, numeric Boolean yields, branch groups, flag initialization and stores, Match Text settings, counts, output token positions, and final producer edge. It rejects 17 causal mutations, including replacing the uppercase operand, redirecting witnesses, swapping Boolean yields, changing an inequality to equality, inventing a case-sensitivity field, changing Match Text sensitivity, and returning a constant summary. Seven additional controls reject changed source/expectations and unearned runtime or formal credit.

Clean receipts are in `docs/language-v2/evidence/p15-n03-native-diagnostic-20261119/`. The earlier uncommitted test run also passed all 27 named tests; it is retained as a draft run outside the repository and is not substituted for the clean replay.

## Identities and next decision

- Source SHA256: `e4acefccc8a81c77a8336fa7a5f07eacaf84fc25a6dc251eb8150c607fac80ab`.
- Expected contract SHA256: `007b9eb4edaf6822e587cb72c753aa2b7a1b112810834ea6593df54a82dbb98c`.
- Clean raw log SHA256: `1b0c14cbad11de0b2546e916b4c32d2e7896f94ab1d766160dcb53217d11b669`.

The Apple agent owns the additive strict callback selector, signing, nonce, callback correlation, and publication for a separate diagnostic candidate. This work does not modify the frozen e61 candidate's S01/F01 or formal 179 runs.

If actual dynamic comparisons contradict the exact-equality target while the byte witnesses and canonical controls are valid, preserve that baseline and implement a bounded compiler correction together with explicit legacy migration coverage. If the observations agree, retain the concrete runtime proof and complete the missing legacy migration proof. Neither outcome may be inferred from this structural test.
