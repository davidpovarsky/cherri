# Apple Runtime Discovered vs Cherri Supported — Gap & Denominator Report

- **Source Commit**: `c5c59432bab885712a5512babcde378e2ea76854`
- **Input Dependency Fingerprint**: `cf00bedc7ca27b85adfaa0124e89514cfb1c7c04622c7cd02bb73a8603581b0a`
- **Total Raw Discovered Apple Runtime Records**: `2396`
- **Eligible User-Facing Apple Actions**: `1527` (`487` `ELIGIBLE_USER_FACING` + `1040` `ELIGIBLE_APP_INTENT`)
- **Excluded / Deferred Records**: `869` (`596` `EXCLUDED_NONRUNNABLE` + `131` `EXCLUDED_HIDDEN_INTERNAL` + `86` `DEFERRED_NO_EVIDENCE` + `29` `EXCLUDED_CONFIG_ONLY` + `27` `EXCLUDED_THIRD_PARTY_DEPENDENCY`)
- **Covered Unique Apple Identifiers (`COVERED_PARTIAL`)**: `390` (`25.54%` of `1,527` eligible user-facing Apple actions; `COVERED_VERIFIED`: `0`)
- **Uncovered Eligible Apple Identifiers**: `1137` (`97` `NEW_NATIVE_ACTION` + `1040` `MISSING_APP_INTENT_DESCRIPTOR`)

## Authoritative Taxonomy Breakdown (`p10-action-ledger.json`)

| Status | Count | Eligibility Class | Notes |
| :--- | :---: | :--- | :--- |
| `COVERED_VERIFIED` | `0` | `ELIGIBLE_USER_FACING` | Requires verified runtime `outputTypeConcordance == CONCORDANT` |
| `COVERED_PARTIAL` | `390` | `ELIGIBLE_USER_FACING` | `313` `UNASSESSED` + `77` `DIFFERS` output type concordance; `1` parameter `NEEDS_RUNTIME_EVIDENCE` |
| `NEW_NATIVE_ACTION` | `97` | `ELIGIBLE_USER_FACING` | Uncovered native built-in / system actions |
| `MISSING_APP_INTENT_DESCRIPTOR` | `1040` | `ELIGIBLE_APP_INTENT` | Uncovered first-party App Intent actions |
| `DEFERRED_NO_EVIDENCE` | `86` | `DEFERRED` | Insufficient runtime parameter metadata |
| `EXCLUDED_HIDDEN_INTERNAL` | `131` | `EXCLUDED_INTERNAL` | Hidden/internal workflow actions |
| `EXCLUDED_CONFIG_ONLY` | `29` | `EXCLUDED_CONFIG_ONLY` | Configuration-only definitions |
| `EXCLUDED_NONRUNNABLE` | `596` | `EXCLUDED_NONRUNNABLE` | Non-runnable entities and queries |
| `EXCLUDED_THIRD_PARTY_DEPENDENCY` | `27` | `EXCLUDED_THIRD_PARTY` | Requires third-party apps |

## Why All `390` Covered Apple Identifiers Are `COVERED_PARTIAL` (`0` `COVERED_VERIFIED`)

In `p10-action-ledger.json`, `evaluate_action_coverage` (FR-05, enforced by Gate `A07` and `R16`) only promotes an Apple identifier to `COVERED_VERIFIED` when both its parameter contracts and its runtime `outputTypeConcordance` are verified against Apple's runtime metadata. Across the `390` covered Apple identifiers:
- `313` have `outputTypeConcordance == "UNASSESSED"` (Apple runtime metadata or Cherri declaration reports `Unknown`/`None`).
- `77` have `outputTypeConcordance == "DIFFERS"` (e.g., Apple metadata reports `None` while Cherri types the output as `text`, `array`, or `variable`).
- `1` parameter (`WFCalendarDescriptor` picker on `addNewEvent`) requires live runtime specimen confirmation (`NEEDS_RUNTIME_EVIDENCE`, Gate `R15`).

## Historical Discrepancy Note (`97 NEW_NATIVE_ACTION` vs Earlier `100 MISSING_HIGH_PRIORITY`)

An earlier narrative draft referred to `100 MISSING_HIGH_PRIORITY` and `1,037 MISSING_LOW_PRIORITY` actions (`100 + 1,037 = 1,137`). During P10 remediation, `3` native built-in actions were mapped into the `390` covered set (`487 ELIGIBLE_USER_FACING - 390 COVERED_PARTIAL = 97 NEW_NATIVE_ACTION`), and the authoritative `p10-action-ledger.json` taxonomy standardized the `1,137` uncovered eligible records into `97 NEW_NATIVE_ACTION` + `1,040 MISSING_APP_INTENT_DESCRIPTOR`.

## All `97` `NEW_NATIVE_ACTION` Apple Actions

| # | Apple Identifier | Title | Category | OS Availability |
| :---: | :--- | :--- | :--- | :--- |
| 1 | `com.apple.NanoSettings.NPRFPingMyPhoneIntent` | Ping My iPhone | `builtin` | `unknown` |
| 2 | `com.apple.NanoSettings.NPRFSetAlwaysOnIntent` | Set Always On | `builtin` | `unknown` |
| 3 | `com.apple.NanoSettings.NPRFSetAutoLaunchAudioAppsIntent` | Set Auto-Launch Audio Apps | `builtin` | `unknown` |
| 4 | `com.apple.NanoSettings.NPRFSetFlashLightIntent` | Set Flashlight | `builtin` | `unknown` |
| 5 | `com.apple.NanoSettings.NPRFSetSchoolTimeIntent` | Set Schooltime | `builtin` | `unknown` |
| 6 | `com.apple.NanoSettings.NPRFSetSilentModeIntent` | Set Silent Mode | `builtin` | `unknown` |
| 7 | `com.apple.NanoSettings.NPRFSetTheaterModeIntent` | Set Theater Mode | `builtin` | `unknown` |
| 8 | `com.apple.NanoSettings.NPRFSetWakeOnWristRaiseIntent` | Set Wake on Wrist Raise | `builtin` | `unknown` |
| 9 | `com.apple.NanoSettings.NPRFSetWaterLockIntent` | Set Water Lock | `builtin` | `unknown` |
| 10 | `com.apple.Notes.ICNotesFolderIntent` | Show Notes Folder | `builtin` | `unknown` |
| 11 | `com.apple.TVRemoteUIService.LaunchApplicationIntent` | Unknown Action | `builtin` | `unknown` |
| 12 | `com.apple.TVRemoteUIService.LaunchRemoteIntent` | Unknown Action | `builtin` | `unknown` |
| 13 | `com.apple.TVRemoteUIService.LaunchScreenSaverIntent` | Unknown Action | `builtin` | `unknown` |
| 14 | `com.apple.TVRemoteUIService.PauseContentIntent` | Unknown Action | `builtin` | `unknown` |
| 15 | `com.apple.TVRemoteUIService.ReduceLoudSoundsIntent` | Unknown Action | `builtin` | `unknown` |
| 16 | `com.apple.TVRemoteUIService.SkipContentIntent` | Unknown Action | `builtin` | `unknown` |
| 17 | `com.apple.TVRemoteUIService.SleepAppleTVIntent` | Unknown Action | `builtin` | `unknown` |
| 18 | `com.apple.TVRemoteUIService.SwitchUserAccountIntent` | Unknown Action | `builtin` | `unknown` |
| 19 | `com.apple.TVRemoteUIService.ToggleCaptionsIntent` | Unknown Action | `builtin` | `unknown` |
| 20 | `com.apple.TVRemoteUIService.ToggleSystemAppearanceIntent` | Set Appearance on Apple TV | `builtin` | `unknown` |
| 21 | `com.apple.TVRemoteUIService.WakeAppleTVIntent` | Unknown Action | `builtin` | `unknown` |
| 22 | `com.apple.UniversalAccess.UASettingsShortcuts.UASetBackgroundSoundsTimerIntent` | Set Background Sounds Timer Interval | `builtin` | `unknown` |
| 23 | `com.apple.UniversalAccess.UASettingsShortcuts.UAToggleAccessibilityKeyboardIntent` | Set Accessibility Keyboard | `builtin` | `unknown` |
| 24 | `com.apple.UniversalAccess.UASettingsShortcuts.UAToggleAlternatePointerActionsIntent` | Set Alternate Pointer Actions | `builtin` | `unknown` |
| 25 | `com.apple.UniversalAccess.UASettingsShortcuts.UAToggleAudioDescriptionsIntent` | Set Audio Descriptions | `builtin` | `unknown` |
| 26 | `com.apple.UniversalAccess.UASettingsShortcuts.UAToggleBackgroundSoundsIntent` | Toggle Background Sounds | `builtin` | `unknown` |
| 27 | `com.apple.UniversalAccess.UASettingsShortcuts.UAToggleCaptionsIntent` | Set Closed Captions+SDH | `builtin` | `unknown` |
| 28 | `com.apple.UniversalAccess.UASettingsShortcuts.UAToggleClassicInvertIntent` | Set Classic Invert | `builtin` | `unknown` |
| 29 | `com.apple.UniversalAccess.UASettingsShortcuts.UAToggleColorFiltersIntent` | Set Color Filters | `builtin` | `unknown` |
| 30 | `com.apple.UniversalAccess.UASettingsShortcuts.UAToggleContrastIntent` | Set Increase Contrast | `builtin` | `unknown` |
| 31 | `com.apple.UniversalAccess.UASettingsShortcuts.UAToggleFullKeyboardAccessIntent` | Set Full Keyboard Access | `builtin` | `unknown` |
| 32 | `com.apple.UniversalAccess.UASettingsShortcuts.UAToggleHeadPointerIntent` | Set Head Pointer | `builtin` | `unknown` |
| 33 | `com.apple.UniversalAccess.UASettingsShortcuts.UAToggleHoverTextIntent` | Set Hover Text | `builtin` | `unknown` |
| 34 | `com.apple.UniversalAccess.UASettingsShortcuts.UAToggleHoverTypingIntent` | Set Hover Typing | `builtin` | `unknown` |
| 35 | `com.apple.UniversalAccess.UASettingsShortcuts.UAToggleLiveCaptionsIntent` | Set Live Captions | `builtin` | `unknown` |
| 36 | `com.apple.UniversalAccess.UASettingsShortcuts.UAToggleLiveSpeechIntent` | Set Live Speech | `builtin` | `unknown` |
| 37 | `com.apple.UniversalAccess.UASettingsShortcuts.UAToggleMonoAudioIntent` | Set Mono Audio | `builtin` | `unknown` |
| 38 | `com.apple.UniversalAccess.UASettingsShortcuts.UAToggleMotionCuesIntent` | Set Motion Cues | `builtin` | `unknown` |
| 39 | `com.apple.UniversalAccess.UASettingsShortcuts.UAToggleMouseKeysIntent` | Set Mouse Keys | `builtin` | `unknown` |
| 40 | `com.apple.UniversalAccess.UASettingsShortcuts.UAToggleReduceMotionIntent` | Set Reduce Motion | `builtin` | `unknown` |
| 41 | `com.apple.UniversalAccess.UASettingsShortcuts.UAToggleSlowKeysIntent` | Set Slow Keys | `builtin` | `unknown` |
| 42 | `com.apple.UniversalAccess.UASettingsShortcuts.UAToggleSmartInvertIntent` | Set Smart Invert | `builtin` | `unknown` |
| 43 | `com.apple.UniversalAccess.UASettingsShortcuts.UAToggleStickyKeysIntent` | Set Sticky Keys | `builtin` | `unknown` |
| 44 | `com.apple.UniversalAccess.UASettingsShortcuts.UAToggleSwitchControlIntent` | Set Switch Control | `builtin` | `unknown` |
| 45 | `com.apple.UniversalAccess.UASettingsShortcuts.UAToggleTransparencyIntent` | Set Reduce Transparency | `builtin` | `unknown` |
| 46 | `com.apple.UniversalAccess.UASettingsShortcuts.UAToggleVoiceControlIntent` | Set Voice Control | `builtin` | `unknown` |
| 47 | `com.apple.UniversalAccess.UASettingsShortcuts.UAToggleVoiceOverIntent` | Set VoiceOver | `builtin` | `unknown` |
| 48 | `com.apple.UniversalAccess.UASettingsShortcuts.UAToggleZoomIntent` | Set Zoom | `builtin` | `unknown` |
| 49 | `com.apple.mobileslideshow.StreamShareService` | Post to Shared Album | `builtin` | `unknown` |
| 50 | `com.apple.musicrecognition.RecognizeMusicIntent` | Recognize Music | `builtin` | `unknown` |
| 51 | `com.apple.news.TagIntent` | Show Topic | `builtin` | `unknown` |
| 52 | `com.apple.news.TodayIntent` | Show Today Feed | `builtin` | `unknown` |
| 53 | `com.apple.weather.WeatherIntent` | Show Weather | `builtin` | `unknown` |
| 54 | `com.panic.iOS.Transmit.Share` | Save with Transmit | `builtin` | `unknown` |
| 55 | `is.workflow.actions.debug.contentattribution` | contentattribution | `builtin` | `unknown` |
| 56 | `is.workflow.actions.display.always-on.set` | Set Always On Display | `builtin` | `unknown` |
| 57 | `is.workflow.actions.filter.health.quantity` | quantity | `builtin` | `unknown` |
| 58 | `is.workflow.actions.gethomeaccessorystate` | Get state | `builtin` | `unknown` |
| 59 | `is.workflow.actions.getonscreencontext` | Get What’s On Screen | `builtin` | `unknown` |
| 60 | `is.workflow.actions.goodreader.open` | Open in GoodReader | `builtin` | `unknown` |
| 61 | `is.workflow.actions.handoff` | Continue in Shortcuts App | `builtin` | `unknown` |
| 62 | `is.workflow.actions.handoffplayback` | Hand Off Playback | `builtin` | `unknown` |
| 63 | `is.workflow.actions.health.quantity.log` | Log Health Sample | `builtin` | `unknown` |
| 64 | `is.workflow.actions.health.workout.log` | Log Workout | `builtin` | `unknown` |
| 65 | `is.workflow.actions.homeaccessory` | Control Home | `builtin` | `unknown` |
| 66 | `is.workflow.actions.image.convert.finder` | Convert Image | `builtin` | `unknown` |
| 67 | `is.workflow.actions.imgur.upload` | Upload to Imgur | `builtin` | `unknown` |
| 68 | `is.workflow.actions.importaudiofiles` | Import Audio Files into Music | `builtin` | `unknown` |
| 69 | `is.workflow.actions.input` | Input | `builtin` | `unknown` |
| 70 | `is.workflow.actions.lightroom.import` | Import to Lightroom | `builtin` | `unknown` |
| 71 | `is.workflow.actions.pinboard.add` | Add to Pinboard | `builtin` | `unknown` |
| 72 | `is.workflow.actions.pinboard.get` | Get Pinboard Bookmarks | `builtin` | `unknown` |
| 73 | `is.workflow.actions.podcasts.subscribe` | Follow Podcast | `builtin` | `unknown` |
| 74 | `is.workflow.actions.posters.switch` | Switch Between Wallpapers | `builtin` | `unknown` |
| 75 | `is.workflow.actions.postonfacebook` | Post on Facebook | `builtin` | `unknown` |
| 76 | `is.workflow.actions.properties.appearance` | Get Details of Appearance | `builtin` | `unknown` |
| 77 | `is.workflow.actions.properties.health.quantity` | quantity | `builtin` | `unknown` |
| 78 | `is.workflow.actions.properties.itunesartist` | Get Details of iTunes Artist | `builtin` | `unknown` |
| 79 | `is.workflow.actions.properties.itunesstore` | Get Details of iTunes Product | `builtin` | `unknown` |
| 80 | `is.workflow.actions.properties.ridestatus` | Get Details of Ride Status | `builtin` | `unknown` |
| 81 | `is.workflow.actions.properties.trello` | Get Details of Trello Item | `builtin` | `unknown` |
| 82 | `is.workflow.actions.properties.ulysses.sheet` | Get Details of Ulysses Sheets | `builtin` | `unknown` |
| 83 | `is.workflow.actions.reminders.showlist` | Open Reminders List | `builtin` | `unknown` |
| 84 | `is.workflow.actions.ride.requestride` | Request Ride | `builtin` | `unknown` |
| 85 | `is.workflow.actions.runextension` | Share with Apps | `builtin` | `unknown` |
| 86 | `is.workflow.actions.searchlocalbusinesses` | Find Places | `builtin` | `unknown` |
| 87 | `is.workflow.actions.showinblindsquare` | Open in BlindSquare | `builtin` | `unknown` |
| 88 | `is.workflow.actions.sirikit.donation.handle` | handle | `builtin` | `unknown` |
| 89 | `is.workflow.actions.slack.send` | send | `builtin` | `unknown` |
| 90 | `is.workflow.actions.todoist.add` | Add Todoist Item | `builtin` | `unknown` |
| 91 | `is.workflow.actions.tweet` | Tweet | `builtin` | `unknown` |
| 92 | `is.workflow.actions.useractivity.open` | open | `builtin` | `unknown` |
| 93 | `is.workflow.actions.venmo.pay` | Send Payment | `builtin` | `unknown` |
| 94 | `is.workflow.actions.venmo.request` | Request Payment | `builtin` | `unknown` |
| 95 | `is.workflow.actions.watchmedo` | Watch Me Do | `builtin` | `unknown` |
| 96 | `is.workflow.actions.workout.end` | end | `builtin` | `unknown` |
| 97 | `is.workflow.actions.workout.start` | start | `builtin` | `unknown` |
