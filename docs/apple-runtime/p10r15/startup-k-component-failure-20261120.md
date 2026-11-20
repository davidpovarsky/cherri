# Startup component: original failed native result

Source: `0284f41c1783b81d5aac002c097357481f6b2308`, based on immutable K `8e66254d56ea3e4ac70f6cda6ef9d31217e0937c` with the focused startup component. Run: [44663872632](https://github.com/davidpovarsky/cherri/actions/runs/44663872632), attempt 1, `ios-build.yml`, `runner_label=macos-15-intel`, `documentation_component_only=false`.

## Actual result

The native job failed with xcodebuild exit 65: **44 tests passed and 9 failed**. Job 136048349087 ran from 02:59:52Z to 03:35:13Z on 2026-11-20. It was not cancelled or stopped by the workflow time limit. The evidence collector and artifact upload completed.

All 26 core integration tests and both documentation-index unit tests passed. Eleven of twelve editor-input tests passed. The D03 UI search body was not reached; its documentation evidence remains `NOT_VERIFIED / RUN_NOT_ELIGIBLE`. The original formal evidence contains zero records.

## Eight startup failures

All eight failures occurred at the strict readiness assertion in `CherriEditorUITests.swift:191`. One document-creation action was issued, but the completed wait and interactive-editor state were not both observed within the original 28,000 ms allowance.

| Test | Final elapsed (ms) | Editor present | Completed wait |
|---|---:|---|---|
| testBRG27EditorRealWorldWorkflowUI | 30353 | false | false |
| testD03CurrentDocumentationSearchUI | 41288 | true | false |
| testE01CompletionDisplayAndInsertionUI | 34117 | true | false |
| testE01PaletteSearchSelectsExactActionAndReplacesPrefixUI | 28754 | false | false |
| testE02DiagnosticsRenderingAndNavigationUI | 29256 | true | false |
| testGeneratedCloserCompletesAndOvertypesWithRealKeyboard | 28879 | false | false |
| testNegativeSyntaxErrorRejectsCompilationFailsClosed | 43183 | true | false |
| testTypedSourceAndResetPreserveExactUTF16 | 39729 | true | false |

A late editor is retained as diagnostic observation and is never treated as successful startup. These results show that the prior one-shot fast path has not resolved the native startup failures; they do not yet establish the underlying document or accessibility delay.

## Paste wait failure

`testSingleCharacterPasteDoesNotConsumeGeneratedText` failed at the existing three-second asynchronous fulfillment assertion, line 171. The isolated view was attached to a key window and was first responder. Before paste and 9 ms after the real `view.paste(nil)` call it contained `text("x")`. At 7,030 ms after the failed wait it contained the exact expected `text("x"))`, and the final text assertion passed. The first actual text-change timestamp was not observed. Neither a three-second success nor a production paste defect is inferred from that final observation.

## Retention and scope

Original artifact 14598293526 (`ios-build-evidence`, 65,969 bytes) has SHA-256 `a0c7b020601f6eb8fabdc1350dcd8ebda98ae143fe671d8a4b6747a50c7ac97b`. Original API metadata, ZIP, job log, and six published members were retained without mutation. The adjacent JSON binds their hashes and preserves exact bounded startup/paste receipts. No screenshot or private source was added.

This failed component result grants no formal requirement or physical-device credit. Assertions and time budgets remain unchanged while the cause is investigated.
