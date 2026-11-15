# Cherri native object debugging: 2026-11-12

## Source of truth
The user-provided iPadOS native Shortcut export - Mishtanei Log 2 - demonstrates
the EXACT dated native display chain:
1. Repeat with Each on the __cherri_debug_log native object list.
2. Set Name (is.workflow.actions.setitemname) with
   WFInput=WFTextTokenAttachment(Type=Variable, VariableName=Repeat Item).
3. WFName=WFTextTokenString with string=U+FFFC and attachment
   attachmentsByRange at {0, 1} = the SAME Repeat Item with
   WFPropertyVariableAggrandizement:
   PropertyName=Creation Date, PropertyUserInfo=WFFileCreationDate;
   and WFDateFormatVariableAggrandizement:
   WFDateFormatStyle=Medium, WFTimeFormatStyle=Long,
   WFISO8601IncludeTime=false.
4. End Repeat, then Quick Look of Repeat Results.

The original __cherri_debug_log list is NOT overwritten or coerced into
a combined text. Only the display-loop results are named for preview.
Apple object classes may differ in support for editable names or native
Creation Date; test on the physical iPad. The signed file and plist
structure are verified, not the native runtime behavior on every type.

## Independent flags
- withDates: Capture each original producer Creation Date immediately
  after its output, even without any preview, with detect.date.
- showLogs: Preview native objects; with withDates use the native
  Set Name loop above, otherwise directly Quick Look the original list.
- showLogsAsText: Explicit opt-in to text only; without dates use
  Combine Text with New Lines; with dates use Repeat with Each creating
  Repeat Item - Creation Date rows and Quick Look of repeat results.
- When both showLogs and showLogsAsText are true, compilation rejects
  ambiguous requests.
- Without preview flags debugLog remains exactly one native appendvariable
  bound to the previous real ActionOutput.
- debugShowLogs(withDates:...) and debugFinish(withDates:...) use
  the native mode by default; they also accept showLogsAsText:true.
  Legacy supports positional arguments, V2 supports named arguments.

## Illustrative V2 calls
- debugLog()
- debugLog(withDates: true)
- debugLog(showLogs: true)
- debugLog(withDates: true, showLogs: true)
- debugLog(showLogsAsText: true)
- debugLog(withDates: true, showLogsAsText: true)

## Legacy calls
- debugLog()
- debugLog(true)
- debugLog(false, true)
- debugLog(true, true)
- debugLog(false, false, true)
- debugLog(true, false, true)

## Integration
- Shared compiler macro builder: debug_stdlib.go
- Frontends: action.go and language_backend_adapter.go
- Canonical action catalog: internal/language/schema/generated_registry.go
- Unit tests: TestDebugStdlibSixDisplayModes,
  TestDebugStdlibExplicitDisplayHelpers,
  TestDebugStdlibRejectsConflictingDisplayModes, plus timer
  and idempotency cases in both Legacy and V2.
- 12 independently compiled native plist fixtures: six scenarios in
  both Legacy and V2. All verified expected action shapes,
  typed-Object references, producer-adjacent Creation Date,
  zero missing/forward UUID refs, and Set Name token structure.
- All contain TWO native dictionaries separated by a one-second wait.

## Safety / release
Local branch agent/cherri-debug-stdlib-20261112. No push, merge,
deployment or Asana updates. Physical iPad execution is still required.
