# Project Board

This is a living working-memory document for this repository.
It is intentionally lightweight. Humans and coding agents should update it when useful discoveries, ideas, plans, optimizations, problems, or follow-up work arise during normal development.
Do not turn this into a duplicate issue tracker or a dump of temporary thoughts.

## Inbox

Quick captures that still need classification.

## Ideas & Opportunities

Potential improvements, features, optimizations, or architectural ideas.

## Discoveries & Tips
### Cherri investigation notes (2026-10-06)

- **Named Magic Variable optimization:** Real Shortcut evidence shows that an action output can use `CustomOutputName`, and subsequent references can point directly to the original `ActionOutput`, allowing some usages of `Set Variable` to potentially be eliminated. Investigate compiler/decompiler support and when this transformation is semantically safe. This may affect the compiler, decompiler, corpus analyzer, structural comparator, generated action metadata/docs, Skill behavior, and iOS/app consumers if shared metadata needs extension. This is an investigation opportunity, not an implemented optimization.
- **Inline type coercion / Magic Variable coercion:** Real Shortcut evidence shows that a downstream input can reference an existing action output with `WFCoercionVariableAggrandizement`, for example `CoercionItemClass: WFContactContentItem`. This allowed a vCard/file output to feed Choose from List as contacts without a separate `is.workflow.actions.detect.contacts` action. Investigate whether this generalizes to other Shortcuts content types and whether Cherri can use it to avoid redundant conversion/detection actions. This is an investigation opportunity, not an implemented feature.


Useful technical discoveries, undocumented behavior, shortcuts, implementation tricks, platform behavior, or reusable knowledge discovered while working.

## Experiments / Investigations

Things worth testing or researching before deciding whether to implement them.

## Open Questions

Important unresolved questions or uncertainties.

## Planned / Todo

Concrete work that is worth doing but is not part of the current task.

Use Markdown checkboxes where useful:

- [ ] Example item

## Done

- [x] Close statically inferred List-row guard bypasses (2026-11-19). Fourteen parser-valid reference/function/Optional/Union cases now fail explicitly before an artifact is returned; scalar/Dictionary controls and native import round trips pass. Dynamic AnyContent hierarchy is still unverified, so L01/F04/FN05 remain incomplete. See [boundary and executed controls](docs/language-v2/p15-inferred-list-shape-boundary-20261119.md).

- [x] Preserve prior artifacts when P15 migration or signing fails (2026-11-19). Shared migration validation now rejects unparsable generated text before writing; public signing stages output and commits only after validation. Real failed/restored CLI and OS rename controls pass locally. Final clean-SHA acceptance and Apple gates remain separate; see [causal evidence and scope](docs/language-v2/p15-migration-signing-output-preservation-20261119.md).

Completed items that are still useful to retain because they document an important decision, discovery, or implementation.

Example:

- [x] Example improvement
  - Implemented: YYYY-MM-DD
  - Commit/PR: ...
  - Notes: ...

## Archive

Older completed/superseded items that still have historical value.
