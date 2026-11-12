# Pure Original (`bcc0095`) vs Pure+Pack (`141b9cc`) vs P10 Native Portability Report

- **Source Commit**: `dd9bd664571d53cc558956481ef79e1760615063`
- **Input Dependency Fingerprint**: `9d11e5125a9570d2c1e32acea55bf8b98a3b97cfb2b061aee7321a4e846b9524`
- **Upstream Pure Baseline Commit**: `bcc0095754e2cc0baaeb4d93dae172b8153b4716` (`441` catalog entries: `433` callable actions + `8` compiler constructs)
- **Pure+Pack Overlay Commit**: `141b9cc96cf1df3f9d1593548dc49ae8a46a256a` (`441` Pure Original names + `102` declarative Pack entries [`48` new actions + `54` `*Advanced` aliases]; `487` of the `505` P10 catalog entries are supported in Pure+Pack)
- **P10 Native Branch Commit**: `dd9bd664571d53cc558956481ef79e1760615063` (`505` catalog entries: `441` Pure Original [with `editEvent` and `transcribeText` upgraded to Go] + `48` Pack new declarative actions + `4` declarative `toggleSet` actions in `actions/settings.cherri` + `12` net-new Go-only actions in `actions_std.go`)

## Exact Classification of the 15 Go-Only Actions Evaluated Between Pack and P10

| # | Action Name | Apple Identifier | Classification | Pure Original (`bcc0095`) | Pure+Pack (`141b9cc`) | P10 Native |
| :---: | :--- | :--- | :--- | :---: | :---: | :---: |
| 1 | `createRemindersList` | `com.apple.reminders.TTRCreateListAppIntent` | `REQUIRES_APPINTENT_DESCRIPTOR` | NO | NO (Excluded from Declarative Pack) | YES (Native Go) |
| 2 | `editEvent` | `is.workflow.actions.setters.calendarevents` | `P10_ONLY_GO_REQUIRED` | YES (Declarative Stub) | NO (Excluded from Declarative Pack) | YES (Native Go) |
| 3 | `editReminder` | `is.workflow.actions.setters.reminders` | `P10_ONLY_GO_REQUIRED` | NO | NO (Excluded from Declarative Pack) | YES (Native Go) |
| 4 | `openBook` | `com.apple.iBooksX.OpenBookIntent` | `REQUIRES_APPINTENT_DESCRIPTOR` | NO | NO (Excluded from Declarative Pack) | YES (Native Go) |
| 5 | `playAudiobook` | `com.apple.iBooksX.PlayAudiobookIntent` | `REQUIRES_APPINTENT_DESCRIPTOR` | NO | NO (Excluded from Declarative Pack) | YES (Native Go) |
| 6 | `playVoiceMemo` | `com.apple.VoiceMemos.PlaybackVoiceMemoIntent` | `REQUIRES_APPINTENT_DESCRIPTOR` | NO | NO (Excluded from Declarative Pack) | YES (Native Go) |
| 7 | `recordVoiceMemo` | `com.apple.VoiceMemos.RecordVoiceMemoIntent` | `REQUIRES_APPINTENT_DESCRIPTOR` | NO | NO (Excluded from Declarative Pack) | YES (Native Go) |
| 8 | `run` | `is.workflow.actions.runworkflow` | `ALREADY_UPSTREAM` | YES (Native) | YES (Inherited from Upstream) | YES (Native Go) |
| 9 | `searchSpotlight` | `com.apple.Spotlight.SearchSpotlightIntent` | `REQUIRES_APPINTENT_DESCRIPTOR` | NO | NO (Excluded from Declarative Pack) | YES (Native Go) |
| 10 | `setSilentMode` | `com.apple.ShortcutsActions.SetSilentModeAction` | `REQUIRES_APPINTENT_DESCRIPTOR` | NO | NO (Excluded from Declarative Pack) | YES (Native Go) |
| 11 | `startNotesAudioRecording` | `com.apple.Notes.StartRecordingLinkAction` | `REQUIRES_APPINTENT_DESCRIPTOR` | NO | NO (Excluded from Declarative Pack) | YES (Native Go) |
| 12 | `startStopwatch` | `com.apple.clock.StartStopwatchIntent` | `REQUIRES_APPINTENT_DESCRIPTOR` | NO | NO (Excluded from Declarative Pack) | YES (Native Go) |
| 13 | `stopStopwatch` | `com.apple.clock.StopStopwatchIntent` | `REQUIRES_APPINTENT_DESCRIPTOR` | NO | NO (Excluded from Declarative Pack) | YES (Native Go) |
| 14 | `stopVoiceMemoRecording` | `com.apple.VoiceMemos.StopRecording` | `REQUIRES_APPINTENT_DESCRIPTOR` | NO | NO (Excluded from Declarative Pack) | YES (Native Go) |
| 15 | `transcribeText` | `com.apple.ShortcutsActions.TranscribeAudioAction` | `REQUIRES_APPINTENT_DESCRIPTOR` | YES (Declarative Stub) | NO (Excluded from Declarative Pack) | YES (Native Go) |
