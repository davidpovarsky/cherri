# Historical note: this version predates the native Set Name preview and showLogsAsText flag. Current behavior is documented in native-object-debug-display-2026-11-12.md.

# Cherri native debugLog flags, 2026-11-12

## API / source compatibility

Both optional Booleans are independent:
- withDates: true => inject a native Get Dates from Input action (detect.date) IMMEDIATELY after the preceding original output and before Add to Variable. The native input points to that original action's UUID and has WFPropertyVariableAggrandizement: Creation Date.
- showLogs: true => after recording the original ActionOutput, display the full accumulated log. If withDates=false, the output is displayed via Combine Text (New Lines) then Quick Look, without replacing the original run-local log list.
- withDates:true and showLogs:true => after recording the original value, preview using Repeat with Each; each row's WFTextTokenString is Repeat Item, hyphen, the SAME Repeat Item with Creation Date property + date format aggrandizements. This matches the user's exported edited Shortcut. There is no date list or Get Item at Index.
- With both omitted/false, debugLog remains exactly one Add to Variable and there is no date capture and no preview.

Legacy source uses positional flags: debugLog(); debugLog(true); debugLog(false,true); debugLog(true,true).
V2 uses named flags: debugLog(); debugLog(withDates:true); debugLog(showLogs:true); debugLog(withDates:true,showLogs:true).

The explicit convenience helpers debugShowLogs() / debugFinish() now display with Combine Text by default; passing withDates:true (Legacy positional or V2 named) produces the same native dated Repeat Items view. The explicit helpers can backfill native producer-position date capture for earlier logger calls. Neither helper changes the underlying native typed-value log list.

## Four signed iPad fixtures

All fixtures have exactly FOUR logged producer outputs (text, number, list, text) and THREE waits of one second BETWEEN outputs. In previewing scenarios, only the final call has showLogs=true, so Quick Look appears once at the end.

| mode | withDates | showLogs | actions | date capture actions | Combine Text | Repeat loops | Quick Look |
|---|---|---|---|---|---|---|---|
| 00-no-date-no-display | false | false | 13 | 0 | 0 | 0 | 0 |
| 01-dates-only | true | false | 17 | 4 | 0 | 0 | 0 |
| 10-display-only | false | true | 15 | 0 | 1 | 0 | 1 |
| 11-dates-and-display | true | true | 21 | 4 | 0 | 1 | 1 |

## Structural verification

- All four unsigned native plist fixtures were parsed independently with Python plistlib.
- No missing or forward ActionOutput UUID references, no storage operations, and no __cherri_debug_dates list or getitemfromlist.
- Apple native date aggrandizement on the original source immediately after producer validated for each case.
- With dates and display, two Repeat Item attachment tokens with Creation Date and time format on the SECOND are present in the same source row, exactly as manually corrected by user.
- All four signed shortcut outputs passed Apple's AEA1 signed file-header check; signing alone is not a live device runtime test.
- Focused Debug Stdlib and Debug Timer Go unit tests pass in Legacy/V2.
- Native Shortcuts runtime acceptance and date availability on different object types remain pending iPad testing.

Branch: agent/cherri-debug-stdlib-20261112, local uncommitted worktree. No Asana changes / no main merge / no production deployment.

## Files

- Source: samples/debug-flags-00-no-date-no-display-legacy.cherri
- Signed: tests/Cherri-Debug-FLAGS-00-no-date-no-display-SIGNED.shortcut
- Source: samples/debug-flags-01-dates-only-legacy.cherri
- Signed: tests/Cherri-Debug-FLAGS-01-dates-only-SIGNED.shortcut
- Source: samples/debug-flags-10-display-only-legacy.cherri
- Signed: tests/Cherri-Debug-FLAGS-10-display-only-SIGNED.shortcut
- Source: samples/debug-flags-11-dates-and-display-legacy.cherri
- Signed: tests/Cherri-Debug-FLAGS-11-dates-and-display-SIGNED.shortcut
