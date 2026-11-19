# Contextual signature regression repair — 2026-11-19

## Observed failure and cause

The frozen candidate `e61e0d306bb363c39ade612ef35ccfc8051ddf3c` failed the actual Linux Build & Test run 44621720770 in the existing `TestP15T08ConditionalCardinalityService` and `TestP15PromptNumberService`. The full local acceptance run 44621856878 independently recorded 84 PASSED, one FAILED (T08), and 94 pending/not-run obligations. Its original complete report is retained in the separate e61 acceptance evidence directory.

The lexer-based signature context introduced in the RP38 repair correctly found the active callable, but passed the argument cursor position to `inferredCallResultAt`. That function intentionally accepts a position on the callee identifier, so the call-specific result was lost and the broad schema result was shown as Unknown.

The repair carries the exact callee token span in `enumCallContext` and passes its start position to the existing analyzed-call lookup. It preserves the lexer's nested-call, multiline and Unicode handling, and does not broaden the AST lookup to unrelated same-name calls.

## Actual verification

All final runs executed from `30fd0c92531f2ade689c5eea6b8f5453b8227f63`; the working tree remained clean after execution.

| Actual selection | Result | Duration |
| --- | --- | --- |
| Existing T08 and PromptNumber service tests, plus six new contextual cursor cases | 9 named PASS events; zero failed/skipped | 24.828 s |
| Entire service and LSP packages | 51 named PASS events; zero failed/skipped | 7.032 s |
| Actual T08 formal production evidence test | PASS; zero failed/skipped | 13.359 s |

The new cases cover scalar/list literal selection across lines, inner and outer calls sharing the same name, Unicode before the cursor, and keeping an unrelated nonnumeric prompt result Unknown.

## Preserved unsuccessful attempts

The tests-only baseline commit `efed962f548a5aac07d0d048a753330aafd1928c` reproduced the two existing failures and the newly added cases before production was changed. Raw baseline output and its source-bound summary are preserved.

The first repaired run at `3ef08319c49a892863a70d2d497b829942e917e6` passed both existing tests and five new cases. One new case had an incorrect expected result: it expected Text for `prompt(inputType: .Text)`. The established implementation in `internal/language/analysis/cardinality.go` deliberately refines only a proved Number mode, and the pre-existing `TestP15PromptNumberService` explicitly asserts Unknown after changing that mode to Text. The new case was corrected to that existing contract in `30fd0c9`. The original failed attempt is preserved, and its incorrect expectation is not counted as evidence of a product defect. The negative control now verifies that a previous Number call cannot lend its type to the separate Text-mode call.

No immutable acceptance, repair or recovery contract, existing test expectation, compiler output rule, or native runtime oracle changed.

## Scope and remaining gates

This proof covers the shared service and LSP regression and the actual local T08 producer. The combined candidate still requires fresh full CI and complete 179-obligation execution. It supplies no Apple runtime, device, deployment, consumer-installation or release-closure credit.

All raw files in `evidence/p15-signature-callee-regression-20261119/` are retained byte-for-byte. The local JSON summaries carry exact commands, source commits, observed test events and SHA-256 digests of their corresponding logs. Git attributes disable newline rewriting for these evidence files.
