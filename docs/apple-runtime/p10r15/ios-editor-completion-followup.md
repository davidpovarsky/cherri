# iOS completion follow-up after actual a503 runtime

[Run 44242289253](https://github.com/davidpovarsky/cherri/actions/runs/44242289253) built the app and the original pinned CodeEditorView dependency with the reviewed patch. It passed all 24 core tests, seven of eight CodeView tests and 11 of 12 UI tests. All six earlier extra-closing-parenthesis regressions passed, as did exact palette insertion, automatic completion/overtype, real nested keyboard fixtures, Hebrew and emoji input.

The workflow still failed. The inline completion test could not select an exact visible show candidate, and the combined nested CodeView fixture test had a start event without a completion event. Neither missing result was counted as a pass. The bounded evidence hashes and exact observed scope are in ios-editor-input-a503-validation.json.

## Inline completion correction

The existing completion service returns a full contextual candidate set, ordered by semantic priority and then label. The Swift consumer displayed its first eight items without applying the typed prefix, while the fallback catalog path did filter the prefix. Thus the populated service result suppressed the fallback and omitted show from the visible row for sho.

The consumer now filters the current prefix before limiting the row. A shared request snapshot binds the asynchronous result and its display to the exact UTF-16 source and caret; source changes now invalidate the task even if the caret position has not changed. Cancelled or stale bridge results cannot publish candidates.

The new native service test calls the actual bridge with sho, confirms that truncating the unfiltered set loses show, then uses the same consumer filter and compiles the exact show(Ask) insertion. Snapshot tests cover source/caret changes, invalid selection and canonically equivalent strings with different UTF-16 storage.

## Missing nested-test outcome

The published bounded log does not contain a crash frame or failure reason for the incomplete nested test. Its cause remains unknown. The same five source fixtures now run as independent native tests with unchanged exact UTF-16 assertions. Each fresh synthetic CodeView records bounded before/after key state, allowing an incomplete test to identify the last executed input. No speculative dependency algorithm change was made.

The collector retains only reviewed fixture receipts tied to an active test and fixed native error categories, optionally with a reviewed source basename and line number. It excludes arbitrary exception text, paths, raw logs and screenshots. E01 candidate identifiers are retained only after the test's verified empty reset and exact sho source, under a restricted identifier shape and count.

## Validation boundary

All 27 local Python installer and evidence-collector tests pass, including privacy, bounds, active-test provenance and incomplete-run rejection. The pinned dependency patch is unchanged. The new Swift/UI behavior requires another immutable native run; this document does not claim its outcome.
