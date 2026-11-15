# Cherri Debug — native object preview and opt-in text preview

Implementation: agent/cherri-debug-stdlib-20261112, local working tree, 2026-11-12.
Native object display is based directly on the user-supplied
משתני לוג-2.json Apple Shortcuts export.

## Three independent Boolean switches

- withDates: capture the Apple Creation Date / WFFileCreationDate for each
  original output. The native is.workflow.actions.detect.date is placed
  IMMEDIATELY after its original output-producing action, not at preview time.
  Runs even if there is no preview.
- showLogs: native object preview, not text coercion.
  - withDates false: Quick Look of original __cherri_debug_log list directly.
  - withDates true: Repeat with Each(__cherri_debug_log), then Set Name on
    Repeat Item with its Creation Date, then End Repeat, then Quick Look
    of Repeat Results. This is the user's exact corrected native pattern.
- showLogsAsText: separate, explicit text-formatting preview.
  - withDates false: Combine Text (New Lines) then Quick Look.
  - withDates true: Repeat with Each, a display-only Text action
    containing the two tokens Repeat Item and the SAME Repeat Item with
    Creation Date/date-time formatting separated by a hyphen, then
    Quick Look of Repeat Results.
- showLogs and showLogsAsText are ALTERNATIVE display modes. Both true
  is a compile-time error, not a silent choice.
- Without showLogs or showLogsAsText: zero preview/formatting actions.

## Native Set Name fidelity: exact exported plist

- Action identifier: is.workflow.actions.setitemname.
- WFInput: WFTextTokenAttachment(Value: Variable, Repeat Item).
- WFName: WFTextTokenString whose Value.string is the single U+FFFC token.
- WFName.Value.attachmentsByRange["{0, 1}"] is the Repeat Item variable
  with exactly two aggrandizements:
    1. WFPropertyVariableAggrandizement, PropertyName=Creation Date,
       PropertyUserInfo=WFFileCreationDate.
    2. WFDateFormatVariableAggrandizement, WFDateFormatStyle=Medium,
       WFTimeFormatStyle=Long, WFISO8601IncludeTime=false.
- After Set Name, repeat.each end with identical GroupingIdentifier and
  Quick Look WFInput = that end action's Repeat Results.
- Set Name runs in a display-only pass. The original __cherri_debug_log
  values retain their original native type and variable identity.
- No __cherri_debug_dates list, no getitemfromlist, and no
  properties.files at preview time.

## Cherri source syntax

Legacy positional order: withDates, showLogs, showLogsAsText.
- debugLog() : log native previous output only
- debugLog(true) : log native output and capture Creation Date
- debugLog(false, true) : display native objects
- debugLog(true, true) : display native objects named by Creation Date
- debugLog(false, false, true) : display combined text
- debugLog(true, false, true) : display per-item dated text

Cherri V2 uses optional named arguments:
- debugLog(withDates: true)
- debugLog(showLogs: true)
- debugLog(withDates: true, showLogs: true)
- debugLog(showLogsAsText: true)
- debugLog(withDates: true, showLogsAsText: true)

The same presentation modes are available from debugShowLogs and debugFinish:
these explicitly request display. Their optional arguments are withDates
and showLogsAsText (legacy positional or V2 named); default is native
object preview. Existing log entries can be instrumented retroactively at
compile-time if a later explicit display call requests dates.

## Validation
- TestDebugStdlibSixDisplayModes runs six cases on both Legacy and V2,
  with explicit one-second delays and checks exact action IDs, native
  object references, Creation Date properties, Set Name token metadata,
  Repeat grouping, results, and no unwanted conversions.
- TestDebugStdlibExplicitDisplayHelpers checks both explicit helper
  APIs with both preview formats and dates.
- TestDebugStdlibRejectsConflictingDisplayModes fails ambiguous flags.
- Independent Python plistlib audit parsed twelve compiled Shortcut
  fixtures (6 combinations x 2 compilers) using actual native dictionary
  values and a one-second wait. All UUID references are valid and
  native Set Name matches the uploaded Apple-exported shape.
- Signed Legacy iPad .shortcut fixtures have AEA1 headers.
- Native iPadOS runtime behavior remains unverified for this new version;
  imported/renamed dictionary preview requires real device testing.

## Repo changes
- debug_stdlib.go: canonical schema declarations, preview modes,
  capture and formatting helpers.
- internal/language/schema/generated_registry.go regenerated from 514
  canonical actions for the V2 type checker / iOS action catalog.
- Tests updated in debug_stdlib_test.go and debug_timer_native_test.go.
- sources and signed fixtures in docs/apple-runtime/cherri-debug/samples
  and tests, respectively.
- No push, merge, release, deployment, or Asana modification.
