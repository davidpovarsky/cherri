# Verified iOS editor follow-up — d695

[Native run 44248146483](https://github.com/davidpovarsky/cherri/actions/runs/44248146483) completed successfully at exact commit **d695bc028d60c5e0186bc1e765d37d6291f92202**. The retained bounded evidence reports xcodebuild exit 0 and **50 passed tests, zero failures and zero incomplete tests**.

| Native test class | Passed |
| --- | ---: |
| CherriCoreIntegrationTests | 26 |
| CherriEditorInputTests | 12 |
| CherriEditorUITests | 12 |
| Total | 50 |

The declared test names in all three source files match the observed result set exactly. This is an actual Simulator run, including the real compiler bridge, CodeEditorView/UIKit input, and XCUI interaction.

## What the run establishes

The E01 inline completion and the palette path both turned actual typed “sho” into exactly “show(Ask)”, then passed their compilation assertions. Their two source receipts independently show exact UTF-16 equality. The real bridge test also passed the contextual-prefix filtering and stale-source/caret snapshot contract.

The input suite passed generated-closer completion and one-time overtyping, authored-closer preservation, navigation and deletion invalidation, single-character paste, multi-character insertion, and Unicode/UTF-16 handling. All five source cases from the formerly incomplete combined test now have independent passed test records and exact final-source receipts:

| Case | Final UTF-16 length | Final selection |
| --- | ---: | --- |
| Nested call | 16 | caret 16, empty selection |
| List | 16 | caret 16, empty selection |
| Dictionary | 21 | caret 21, empty selection |
| Curly block with newlines | 22 | caret 22, empty selection |
| Block comment followed by a call | 22 | caret 22, empty selection |

Real UI tests also passed diagnostics navigation, rename, rapid edits, Hebrew/emoji input, visual source editing, exact reset/typing, the generated-closer keyboard path, and negative missing-element/syntax-error controls.

There are 19 verified empty-reset receipts and 22 typed/completion receipts; all 22 carry exact UTF-16 matches. The diagnostic collector retained no UI failure and no native fatal-error event.

## Provenance

The artifact is named **ios-build-evidence**. Its three retained files have these SHA-256 hashes:

| Artifact | SHA-256 |
| --- | --- |
| ios-test-diagnostics.json | 1b63f302737af9c1dd5cbde2e340e53af56161b47e79967ddbd02b553b606cd8 |
| ios-test-evidence.json | 890b2115a4a93e5aa70ce1376b9cc08a714aa4f3f051fa383a67c34f3f263ace |
| xcodetest.log | f370e2b19e489fcb5bc2dad3b4d918ccf9b2bf7bc8aedb9d72ec4ad2c018fefc |

All 18 acceptance records bind the exact implementation commit, run ID, exit 0, passed result and retained log hash. The adjacent JSON receipt records all 50 test IDs, source hashes for the nine relevant source/workflow/patch files, acceptance mappings, exact synthetic terminal sources, and the bounded observations.

The CodeEditorView pin remains **5386056ab53d43363083cb96069715a9608aa048** with the existing prepared dependency patch, **cherri-p15-generated-closers-v1**. This run does not replace the pinned dependency or bypass its bootstrap verification.

## Limits of the evidence

The earlier combined test had started without a completion record. These five identical source assertions now pass independently, but the prior incompletion's cause remains unknown. No claim is made that the old combined loop has been reproduced and repaired.

The collector retained 197 input receipts. Two individual dictionary before/after records are absent from the bounded log; the final exact-source receipt and native assertion passed. This is not claimed as a complete per-keystroke transcript.

E03 and E07 teardown receipts compare against their earlier pre-edit sources and consequently report inequality after the intentional, separately asserted rename and visual source edits. Their typing-stage receipts and native tests passed. E04 formatter evidence is at service level; an E04 UI test is not claimed.

This result applies to d695. Later integrated compiler changes require their own final-candidate gates. Simulator evidence does not establish physical-device execution.

Machine-readable receipt: [ios-editor-d695-native-validation.json](ios-editor-d695-native-validation.json).
