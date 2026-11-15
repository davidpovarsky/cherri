# Cherri Debug Stdlib: native object vs text display (2026-11-12)

## Primary evidence
The user's exported and manually corrected native Apple Shortcut, "משתני לוג-2.json",
contains TWO actual dictionary producers, a 1s Wait between logs, and a display:
- is.workflow.actions.repeat.each on __cherri_debug_log
- is.workflow.actions.setitemname within the repeat
  * WFInput = WFTextTokenAttachment(Variable: Repeat Item)
  * WFName = WFTextTokenString with Value.string U+FFFC
  * attachmentsByRange {0, 1} = Repeat Item variable with:
     1) WFPropertyVariableAggrandizement: PropertyName Creation Date,
        PropertyUserInfo WFFileCreationDate
     2) WFDateFormatVariableAggrandizement: WFDateFormatStyle Medium,
        WFTimeFormatStyle Long, WFISO8601IncludeTime false
- native repeat end with GroupingIdentifier matching the start
- is.workflow.actions.previewdocument WFInput = Repeat Results ActionOutput.

This is materially DIFFERENT from a Text Action that composes the object's
text and Creation Date into a single string. Native display preserves object
payloads for Quick Look via Set Name. The user expressly reserved the old
text-formatting behavior for the new showLogsAsText parameter.

## Behavior / options
debugLog() still appends native previous output to __cherri_debug_log, not
coerced text; use debugClearLogs() to reset the run-local log. Both variants of
Cherri accept THREE optional flags:
- withDates = false default: no Get Dates from Input at producer position
- showLogs = false default: native object preview only when true
- showLogsAsText = false default: text conversion/preview only when true

showLogs and showLogsAsText are alternative display modes and cannot both
be true. The compiler returns a clear error if both are selected.

withDates | showLogs | showLogsAsText | output
false false false: append original native object, nothing else
true  false false: detect.date immediately after original producer,
  append original native object, no preview
false true  false: append original object, Quick Look on original log variable
true  true  false: date detect immediately after producer; append object;
  Repeat Each -> Set Name(Creation Date) -> End Repeat -> Quick Look results
false false true: append original; Combine Text (New Lines) -> Quick Look
true  false true: date detect immediately after producer; append original;
  Repeat Each -> Text(Repeat Item - Creation Date) -> End Repeat -> Quick Look

The same display choices are available as dedicated preview helpers:
- debugShowLogs()
- debugShowLogs(withDates: true) / debugFinish(withDates: true)
- debugShowLogs(showLogsAsText: true)
- debugShowLogs(withDates: true, showLogsAsText: true)

Legacy Cherri supports positional arguments: debugLog(withDates, showLogs,
showLogsAsText). V2 supports named arguments. Helpers' Legacy argument order is
(withDates, showLogsAsText). The canonical action schema is generated from
debug_stdlib.go, and shared implementation is used by Legacy and V2.

## Wire-level verification
- Go targeted tests TestDebugStdlibSixDisplayModes and
  TestDebugStdlibExplicitDisplayHelpers passed across Legacy and V2.
- Compiled 12 sample shortcuts with actual dictionaries, not just text:
  six modes x Legacy/V2, each with a native Wait(1) between log entries.
- Native plist independent audit: all 12 passed expected action counts,
  date extraction adjacent to original producer, original dictionary typed
  ActionOutput refs, UUID link integrity, and exact Set Name WFName payload.
- In native, undated showLogs mode Quick Look gets the original
  __cherri_debug_log variable directly (no Combine Text).
- Text mode without dates uniquely gets text.combine.
- Dated text mode uses Get Text on Repeat Item + Creation Date.
- There is no separate __cherri_debug_dates collection or index lookup.

## QA gate and release
Apple Shortcuts on-device behavior must still be checked with the signed
samples on iPadOS 27. The local worktree is agent/cherri-debug-stdlib-20261112,
and changes have not been merged, pushed, or synced to Asana. The whole
repository release gate remains separate from targeted unit and wire tests.
