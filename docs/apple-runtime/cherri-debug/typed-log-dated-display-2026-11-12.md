# Cherri Debug: capture original output dates at producer position

Date: 2026-11-12 | branch: agent/cherri-debug-stdlib-20261112 (local worktree; not released).

## Apple-source evidence / observed behavior

The user's uploaded Shortcuts JSON places is.workflow.actions.detect.date immediately after a native output action. It sends that original action's ActionOutput token through WFPropertyVariableAggrandizement with PropertyName = Last Modified Date and PropertyUserInfo = WFFileModificationDate. The user tested that later delay actions do not change the original value's date. The previous code that ran properties.files only at the end of a shortcut was incorrect for this requirement.

## Correct compilation semantics

- debugLog() still emits exactly ONE Add to Variable, linked to the preceding native ActionOutput, preserving the original object type.
- debugShowLogs() / debugFinish() and withDates:false emit only Quick Look of original typed log items; no Date/Detect action and no Combine Text.
- For dated output (debugShowLogs(true), debugFinish(true) on Legacy; withDates: true on V2), the shared backfillDebugLogDates compiler pass inserts native detect.date IMMEDIATELY AFTER each distinct original producer, BEFORE that value's Add to Variable. Native WFInput contains exact OutputUUID/OutputName and WFPropertyVariableAggrandizement as exported by Apple. Date captures execute at source time, not preview time.
- After original Add to Variable, a second Add to Variable saves the previously captured Dates result in private __cherri_debug_dates. Both value and timestamp lists advance together, including auto timer logs and conditional/repeat execution.
- Existing debugClearLogs() resets both lists when dated mode is requested. The backfill is idempotent across several dated previews, and leaves producer UUID/casing and all control-flow grouping identifiers unchanged.
- Display traverses __cherri_debug_log with one Repeat with Each; getitemfromlist indexes __cherri_debug_dates at the native Repeat Index; each display-only gettext produces a WFTextTokenString containing <time> - <original item>, using Apple's WFDateFormatVariableAggrandizement on the saved date. Quick Look displays Repeat Results. No overwrite of original log and no properties.files.

## Sources / tests / evidence

- Source: debug_stdlib.go (backfillDebugLogDates, debugLogDateSourceRef, debugDatedLogRow), Legacy action.go, V2 language_backend_adapter.go.
- Tests: TestDebugStdlibShowLogsWithDatesNativeWire, TestDebugStdlibDatedPassIdempotentAndNoDateOnPlainPreview, TestDebugStdlibDatedClearAndTimerSynchronized, plus existing debug and timer tests.
- The first full fixture compiled to 141 native actions: 29 detect.date at original producers, 29 original typed Add to Variable, 29 timestamp Add to Variable. Independent Python plistlib audit found 115 ActionOutput refs, zero missing/forward refs, zero misplaced date extractions.
- Sample sources in samples/debug-previous-only-api-ipad-dated.cherri and samples/debug-previous-only-api-ipad-dated-with-waits.cherri (25 explicit debugLog + 4 timer entries; adds 29 wait(1) actions to test timestamp preservation). Additional minimal and V2 samples also available.
- On-device runtime remains UNVERIFIED for this precise generated shortcut; importantly, some Apple object classes may not provide Last Modified Date or may return differing metadata. Must verify on physical iPad and report failures, not claim universal support.

## Repository safety

Worktree is local and dirty. No merge to main or push of raw user Shortcuts. Prior project full Go test suite had unrelated Language Acceptance failures. Do not mark any deployment or CI-green claim before verification.
## Revision 2026-11-12: use Creation Date

The current debugLog(withDates: true) implementation now captures and displays Apple's native Creation Date / WFFileCreationDate instead of Last Modified Date / WFFileModificationDate. Both native code sites are updated: detect.date immediately after each producer and the dated Repeat Item property aggrandizement. This was verified against native Shortcuts fixture 012.plist. The older Last Modified Date experiments above remain historical context.
