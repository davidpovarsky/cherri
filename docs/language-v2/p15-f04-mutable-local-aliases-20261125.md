# P10R15 F04 mutable local aliases — 25 November 2026

## Implemented and verified scope

The shared V2 loop-result cell planner now admits explicitly typed, straight-line mutable aliases initialized and reassigned from immutable, source-owned cell sequences of exactly the same List type. Reaching-definition copies preserve immutable snapshots of earlier versions. Native actions read the actual current variable; planner origins never replace runtime data. The analyzer and lowerer separately require exact AST enrollment, matching element types and whole assignment before emitting a right-hand side or store. This increment does not change Legacy parsing or the Pure Original repository.

The previously prepared nine-file proposal was executed rather than treated as evidence. The first run exposed an overly strict new graph oracle: standalone Count actions execute without an output UUID when their result is unused. The oracle now checks the complete action and parameters without demanding an unused output, while consumed Count results retain the UUID check. The original failure and its complete output are retained.

The next run exposed a real parser issue: `List<List<Number>>=first` lexes the final `>=` as one comparison token. A type-context-only close operation splits that token into `>` and `=`, preserving source positions, nested types and parameter defaults. Expression `>=` remains unchanged. The original F04 fixtures retain their adjacent syntax; they were not rewritten to hide the parser defect.

## Executed qualification

- Original exact 330 named identities: root 260, lower 44, schema 26; all pass, zero failures or skips, 27 parent tests.
- Complete syntax package: 49 named tests pass, zero failures or skips, including nested/map/optional types, default parameters, Unicode source offsets, comparison preservation and malformed declarations.
- Go 1.24.13, Oracle Linux arm64, two processes maximum, offline existing dependency cache. Candidate files were hashed before and after execution and did not change.
- Independent read-only review by the CI/release lane found no blocker in the admitted slice. No duplicate tests were run by the reviewer.

Original receipt: `/mnt/development/projects/evidence/cherri-p10r15-final-completion-20261125/f04-mutable-qualified-03/receipt.original.json`.
Receipt SHA-256: `663f0e3876f9d2247c8b9a8b53688a3e8df2e7ccd8d81c4a113fe6dc35e594de`.
Independent review: `/mnt/development/projects/evidence/cherri-p10r15-ci-release-20261125/f04-independent-source-review-01/review.original.json`, SHA-256 `1d3081f56b60492d25ab5068a33d7e0c669fea15a868ff2a8ddb013fca3785b6`.
Earlier unsuccessful runs remain under `f04-mutable-local-alias-focused-overlay-qualification-01` and `f04-mutable-qualified-02`.

## Remaining acceptance boundary

This source and graph proof does not close full F04 or the 179-requirement contract. Branch/loop writes, mutable RHS aliases, direct mutable producers, inferred cell aliases and arbitrary representation joins remain explicitly rejected by this increment. Signed Apple execution and general structured-value flow remain separate outstanding gates. No acceptance contract, native expected output, private corpus, protected iOS test, canonical coverage report or historical result was changed.
