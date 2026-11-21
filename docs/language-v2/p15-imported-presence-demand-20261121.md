# Imported-variable optional presence demand

## Proven cause and repair

The clean tests-only source b5164f1ea289863dbb74e54705b54b875d3e6717 reproduces E_TARGET_VALUE_SHAPE on a second opaque dictionary lookup assigned to the same provenance-only variable. There is no authored binding declaration or semantic presence consumer. The first assignment changes the lowered Type; the second assignment previously attempted to maintain absence metadata unconditionally.

Source fix 61614095633e50cbc5377d36ca01061a4a36a100 retains explicit presence-demand decisions for analyzed binding/assignment names. Imported Variable provenance alone consumes this map. Unknown/generated names retain tracking, and authored bindings still use the original AST-pointer map. All existing None, return and user-function demand seeds and the fail-closed unknown-presence boundary remain unchanged.

Independent read-only peer review by docs_consumers found no blocker in this scope. It specifically checked conservative fallback and that the name map is not applied to generated/authored binding pointers.

## Actual execution

The original causal RED remains in the archive: one unwanted lowering failure, while the literal positive and three genuine presence consumers (None comparison, alias comparison and optional AnyContent user function) retain their expected behavior.

On the exact clean fixed source:

- Six public synthetic events passed in 21.812 seconds.
- Twenty demand-analysis events passed in 4.563 seconds, including aliases, returns, positional/named user calls, inert literal text and conservative unknown-name/pointer controls.
- The broader Optional regression passed 1,686 named events in 200.812 seconds, with zero failures or skips. It used the existing six-minute Go process budget.
- Git source state was clean before and after every recorded run.

The archive retains both exploratory attempts and marks their limitations: the first used an incorrect expected native action count; the second used a speculative none assignment that failed earlier semantic analysis. Neither is presented as the causal compiler regression.

## Scope limits

Fresh private 38-input / 27,652-action corpus runs are ongoing on the same fixed source and unchanged input manifest. This document does not claim those five tests passed, does not publish private source or logs, and does not claim Apple runtime or final acceptance of all 179 requirements. The immutable contracts and all original failure evidence remain unchanged.

[Summary and original digests](evidence/p15-imported-presence-demand-20261121/summary.json) and [public raw evidence](evidence/p15-imported-presence-demand-20261121/public-originals.zip) bind commands, clean source identities, measured intervals and unmodified raw outputs.
