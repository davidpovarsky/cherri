# C04 enum fixtures and explicit type evidence

The ae7f877 formal run reported C04 failure. Fresh-process replays at 9b032eb and after registry isolation retained the failures. The production type rules are intentionally strict: an implicit control branch retains AnyContent, and a dictionary field lookup does not prove that a requested key exists merely because its values are Text.

The C04 positive conditional now uses a final explicit yield in both branches, which is the existing typed result contract. The invalid enum-domain fixture also uses yield, so its exact service/CLI diagnostic isolates the invalid domain. Shadowed Map parameters return AnyContent and still must preserve their original native producer/key path; the static enum replacement mutation remains rejected.

The original implicit-branch and Text-return field spellings are retained as negative controls. They must fail with the original type diagnostics and emit no native result. No compiler rule, enum diagnostic, native wire oracle, or acceptance threshold was relaxed. This fixture correction does not claim missing-key presence, implicit branch type proof, Apple runtime, or full release closure.
