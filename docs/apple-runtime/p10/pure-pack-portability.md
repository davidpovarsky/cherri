# Pure Original (`bcc0095`) vs Pure+Pack (`141b9cc`) vs P10 Native Portability Report

- **Source Commit**: `8e91ea9004e37581bf05c9752d39fe66dc0b4697`
- **Input Dependency Fingerprint**: `a604df996cb65fd056d17de942671e23848ed65b57058d68a2dfbacb7c40a768`
- **Upstream Pure Baseline Commit**: `bcc0095754e2cc0baaeb4d93dae172b8153b4716` (`441` catalog entries: `433` callable actions + `8` compiler constructs)
- **Pure+Pack Overlay Commit**: `141b9cc96cf1df3f9d1593548dc49ae8a46a256a` (`441` Pure Original names + `102` declarative Pack entries [`48` new actions + `54` `*Advanced` aliases]; `487` of the `505` P10 catalog entries are supported in Pure+Pack)
- **P10 Native Branch Commit**: `8e91ea9004e37581bf05c9752d39fe66dc0b4697` (`505` catalog entries: `441` Pure Original [with `editEvent` and `transcribeText` upgraded to Go] + `48` Pack new declarative actions + `4` declarative `toggleSet` actions in `actions/settings.cherri` + `12` net-new Go-only actions in `actions_std.go`)

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
