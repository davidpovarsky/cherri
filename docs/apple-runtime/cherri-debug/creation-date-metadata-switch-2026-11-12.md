# Historical note: this version predates the native Set Name preview and showLogsAsText flag. Current behavior is documented in native-object-debug-display-2026-11-12.md.

# Cherri Debug — Creation Date (2026-11-12)

This build changes only timestamp metadata: Last Modified Date ->
Creation Date / WFFileCreationDate (native Apple Shortcuts property).

Change applies in TWO places:
1. The per-output Get Dates from Input action (detect.date), generated
   IMMEDIATELY after its native producer with
   WFPropertyVariableAggrandizement and Creation Date / WFFileCreationDate.
2. Dated Quick Look display row's SECOND Repeat Item variable token,
   using Creation Date / WFFileCreationDate plus time formatting.

The original output is always stored unconverted in __cherri_debug_log.
No __cherri_debug_dates parallel list or Get Item at Index actions.

FOUR SIGNED SHORTCUTS
- 00 no dates, no display: 13 actions, no date operations, no Quick Look.
- 01 dates, no display: 17 actions, 4 creation-date detectors, no Quick Look.
- 10 display, no dates: 15 actions, Combine Text -> Quick Look.
- 11 dates with display: 21 actions, 4 creation-date detectors,
  Repeat with Each using two Repeat Item tokens -> Quick Look.

All four examples have four native outputs (text/number/list/text)
and three independent Wait 1 Second actions between output records.

Compiled and structurally inspected native plist using plistlib:
0 broken or forward ActionOutput UUID refs, correct native date
aggrandizement in all 4 captures, exact two Repeat Item binding in row,
no obsolete date metadata, and signed AEA1 files.

Go focused debug/timer tests passed Legacy and V2.
These builds are NOT YET verified on physical iPad after the metadata change.

Branch: agent/cherri-debug-stdlib-20261112. Local uncommitted changes;
not pushed or merged; Asana not modified.
