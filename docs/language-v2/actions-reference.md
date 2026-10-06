# Cherri Language v2.0 — Action Reference

**Language Version:** 2.0  
**Catalog Schema:** 3.0  
**Schema Fingerprint:** `32cd14d86ebf5c6ebbe67a6f435468fd9647a3382cf47c09d845b74b9d263694`  
**Total Actions:** 461  

This documentation is generated automatically from the canonical Cherri v2 ActionSchema registry.

## Table of Modules

- [Basic](#module-basic) (2 actions)
- [Builtin](#module-builtin) (2 actions)
- [Calendar](#module-calendar) (8 actions)
- [Contacts](#module-contacts) (4 actions)
- [Controlflow](#module-controlflow) (7 actions)
- [Device](#module-device) (9 actions)
- [Documents](#module-documents) (5 actions)
- [General](#module-general) (402 actions)
- [Intelligence](#module-intelligence) (1 actions)
- [Mac](#module-mac) (1 actions)
- [Math](#module-math) (3 actions)
- [Pdf](#module-pdf) (1 actions)
- [Settings](#module-settings) (5 actions)
- [Shortcuts](#module-shortcuts) (4 actions)
- [Text](#module-text) (4 actions)
- [Web](#module-web) (3 actions)

---

## Module: Basic

### `list`

**Title:** List  
**Apple Identifier:** `is.workflow.actions.list`  

Create an immutable array of text.

```cherri
list(listItem: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `listItem` | `Text` | Yes | - | `` | - |

---

### `prompt`

**Title:** Ask for Input  
**Apple Identifier:** `is.workflow.actions.ask`  

Ask for input with prompt, with optional inputType and defaultValue.

```cherri
prompt(prompt: Text, inputType?: Text = "Text", defaultValue?: Text, multiline?: Text = "true", allowsDecimal?: Bool = "true", allowsNegative?: Bool = "true")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `prompt` | `Text` | Yes | - | `WFAskActionPrompt` | - |
| `inputType` | `Text` | No | `Text` | `WFInputType` | Text, Number, URL, Date, Time, Date and Time |
| `defaultValue` | `Text` | No | - | `` | - |
| `multiline` | `Text` | No | `true` | `WFAllowsMultilineText` | - |
| `allowsDecimal` | `Bool` | No | `true` | `WFAskActionAllowsDecimalNumbers` | - |
| `allowsNegative` | `Bool` | No | `true` | `WFAskActionAllowsNegativeNumbers` | - |

---

## Module: Builtin

### `embedFile`

**Title:** Base 64 Embed File  
**Apple Identifier:** `is.workflow.actions.gettext`  

Embed file at path as base 64 text.

```cherri
embedFile(filePath: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `filePath` | `Text` | Yes | - | `` | - |

---

### `makeVCard`

**Title:** Make VCard  
**Apple Identifier:** `is.workflow.actions.gettext`  
```cherri
makeVCard(title: Text, subtitle: Text, base64Image?: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `title` | `Text` | Yes | - | `` | - |
| `subtitle` | `Text` | Yes | - | `` | - |
| `base64Image` | `Text` | No | - | `` | - |

---

## Module: Calendar

### `createAlarm`

**Title:** Create Alarm  
**Apple Identifier:** `com.apple.mobiletimer-framework.MobileTimerIntents.MTCreateAlarmIntent`  

Creates an alarm at a specific time with a name, snooze allowance, and applicable weekdays.

```cherri
createAlarm(name: Text, time: Text, allowsSnooze?: Bool = "true", repeatWeekdays?: List<AnyContent>)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `name` | `Text` | Yes | - | `name` | - |
| `time` | `Text` | Yes | - | `dateComponents` | - |
| `allowsSnooze` | `Bool` | No | `true` | `allowsSnooze` | - |
| `repeatWeekdays` | `List<AnyContent>` | No | - | `` | - |

**App Intent:** `CreateAlarmIntent` (Bundle: `com.apple.clock`)

---

### `createRemindersList`

**Title:** Create Reminders List  
**Apple Identifier:** `com.apple.reminders.TTRCreateListAppIntent`  

Creates a new list in Reminders.

```cherri
createRemindersList()
```

**App Intent:** `TTRCreateListAppIntent` (Bundle: `com.apple.reminders`)

---

### `deleteAlarm`

**Title:** Delete Alarm  
**Apple Identifier:** `com.apple.clock.DeleteAlarmIntent`  

Deletes an alarm.

```cherri
deleteAlarm(alarm: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `alarm` | `AnyContent` | Yes | - | `entities` | - |

**App Intent:** `DeleteAlarmIntent` (Bundle: `com.apple.clock`)

---

### `startStopwatch`

**Title:** Start Stopwatch  
**Apple Identifier:** `com.apple.clock.StartStopwatchIntent`  

Starts the stopwatch.

```cherri
startStopwatch()
```

**App Intent:** `StartStopwatchIntent` (Bundle: `com.apple.clock`)

---

### `stopStopwatch`

**Title:** Stop Stopwatch  
**Apple Identifier:** `com.apple.clock.StopStopwatchIntent`  

Stops the stopwatch.

```cherri
stopStopwatch()
```

**App Intent:** `StopStopwatchIntent` (Bundle: `com.apple.clock`)

---

### `toggleAlarm`

**Title:** Toggle Alarm  
**Apple Identifier:** `is.workflow.actions.togglealarm`  

Toggle an alarm.

```cherri
toggleAlarm(alarm: AnyContent, showWhenRun?: Bool = "true")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `alarm` | `AnyContent` | Yes | - | `alarm` | - |
| `showWhenRun` | `Bool` | No | `true` | `ShowWhenRun` | - |

**App Intent:** `ToggleAlarmIntent` (Bundle: `com.apple.clock`)

---

### `turnOffAlarm`

**Title:** Turn Off Alarm  
**Apple Identifier:** `com.apple.mobiletimer-framework.MobileTimerIntents.MTToggleAlarmIntent`  

Turn off an alarm.

```cherri
turnOffAlarm(alarm: AnyContent, showWhenRun?: Bool = "true")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `alarm` | `AnyContent` | Yes | - | `alarm` | - |
| `showWhenRun` | `Bool` | No | `true` | `ShowWhenRun` | - |

**App Intent:** `ToggleAlarmIntent` (Bundle: `com.apple.clock`)

---

### `turnOnAlarm`

**Title:** Turn On Alarm  
**Apple Identifier:** `com.apple.mobiletimer-framework.MobileTimerIntents.MTToggleAlarmIntent`  

Turn on an alarm.

```cherri
turnOnAlarm(alarm: AnyContent, showWhenRun?: Bool = "true")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `alarm` | `AnyContent` | Yes | - | `alarm` | - |
| `showWhenRun` | `Bool` | No | `true` | `ShowWhenRun` | - |

**App Intent:** `ToggleAlarmIntent` (Bundle: `com.apple.clock`)

---

## Module: Contacts

### `emailAddress`

**Title:** Email Address  
**Apple Identifier:** `is.workflow.actions.email`  

Create an email address value.

```cherri
emailAddress(email: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `email` | `Text` | Yes | - | `` | - |

---

### `newContact`

**Title:** Add New Contact  
**Apple Identifier:** `is.workflow.actions.addnewcontact`  

Create a new contact.

```cherri
newContact(firstName: Text, lastName: Text, phoneNumber: Text, emailAddress: Text, company: Text, notes: Text, prompt?: Bool = "false")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `firstName` | `Text` | Yes | - | `WFContactFirstName` | - |
| `lastName` | `Text` | Yes | - | `WFContactLastName` | - |
| `phoneNumber` | `Text` | Yes | - | `` | - |
| `emailAddress` | `Text` | Yes | - | `` | - |
| `company` | `Text` | Yes | - | `WFContactCompany` | - |
| `notes` | `Text` | Yes | - | `WFContactNotes` | - |
| `prompt` | `Bool` | No | `false` | `ShowWhenRun` | - |

---

### `phoneNumber`

**Title:** Phone Number  
**Apple Identifier:** `is.workflow.actions.phonenumber`  

Create a phone number value.

```cherri
phoneNumber(number: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `number` | `Text` | Yes | - | `` | - |

---

### `updateContact`

**Title:** Update Contact  
**Apple Identifier:** `is.workflow.actions.setters.contacts`  
```cherri
updateContact(contact: AnyContent, detail: Text, value: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `contact` | `AnyContent` | Yes | - | `WFInput` | - |
| `detail` | `Text` | Yes | - | `WFContentItemPropertyName` | First Name, Middle Name, Last Name, Birthday, Prefix, Suffix, Nickname, Phonetic First Name, Phonetic Last Name, Phonetic Middle Name, Company, Job Title, Department, File Extension, Creation Date, File Path, Last Modified Date, Name, Random |
| `value` | `Text` | Yes | - | `` | - |

---

## Module: Controlflow

### `appendVariable`

**Title:** Add to Variable  
**Apple Identifier:** `is.workflow.actions.appendvariable`  
```cherri
appendVariable()
```

---

### `conditional`

**Title:** If  
**Apple Identifier:** `is.workflow.actions.conditional`  
```cherri
conditional()
```

---

### `dictionaryValue`

**Title:** Dictionary  
**Apple Identifier:** `is.workflow.actions.dictionary`  
```cherri
dictionaryValue()
```

---

### `menu`

**Title:** Choose from Menu  
**Apple Identifier:** `is.workflow.actions.choosefrommenu`  
```cherri
menu()
```

---

### `repeat`

**Title:** Repeat  
**Apple Identifier:** `is.workflow.actions.repeat.count`  
```cherri
repeat()
```

---

### `repeat.each`

**Title:** Repeat with Each  
**Apple Identifier:** `is.workflow.actions.repeat.each`  
```cherri
repeat.each()
```

---

### `setVariable`

**Title:** Set Variable  
**Apple Identifier:** `is.workflow.actions.setvariable`  
```cherri
setVariable()
```

---

## Module: Device

### `hideAllApps`

**Title:** Hide Apps  
**Apple Identifier:** `is.workflow.actions.hide.app`  

Hide multiple apps. Allows exception.

```cherri
hideAllApps(except?: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `except` | `Text` | No | - | `` | - |

---

### `hideApp`

**Title:** Hide App  
**Apple Identifier:** `is.workflow.actions.hide.app`  

Hide an app.

```cherri
hideApp(appID: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `appID` | `Text` | Yes | - | `` | - |

---

### `killAllApps`

**Title:** Kill All Apps  
**Apple Identifier:** `is.workflow.actions.quit.app`  

Kills all apps. Allows exceptions.

```cherri
killAllApps(except?: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `except` | `Text` | No | - | `` | - |

---

### `killApp`

**Title:** Kill App  
**Apple Identifier:** `is.workflow.actions.quit.app`  

Kill an app.

```cherri
killApp(appID: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `appID` | `Text` | Yes | - | `` | - |

---

### `openApp`

**Title:** Open App  
**Apple Identifier:** `is.workflow.actions.openapp`  

Open an app.

```cherri
openApp(appID: Text, slideOver?: Bool)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `appID` | `Text` | Yes | - | `WFAppIdentifier` | - |
| `slideOver` | `Bool` | No | - | `WFOpenInSlideOver` | - |

---

### `quitAllApps`

**Title:** Quit All Apps  
**Apple Identifier:** `is.workflow.actions.quit.app`  

Quits all apps. Allows exceptions.

```cherri
quitAllApps(except?: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `except` | `Text` | No | - | `` | - |

---

### `quitApp`

**Title:** Quit App  
**Apple Identifier:** `is.workflow.actions.quit.app`  

Quit an app.

```cherri
quitApp(appID: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `appID` | `Text` | Yes | - | `` | - |

---

### `searchSpotlight`

**Title:** Search Spotlight  
**Apple Identifier:** `com.apple.Spotlight.SearchSpotlightIntent`  

Opens Spotlight search, optionally with search criteria.

```cherri
searchSpotlight(criteria: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `criteria` | `Text` | Yes | - | `criteria` | - |

**App Intent:** `SearchSpotlightIntent` (Bundle: `com.apple.Spotlight`)

---

### `splitApps`

**Title:** Split Apps  
**Apple Identifier:** `is.workflow.actions.splitscreen`  

Split apps across the screen.

```cherri
splitApps(firstAppID: Text, secondAppID: Text, ratio?: Text = "half")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `firstAppID` | `Text` | Yes | - | `` | - |
| `secondAppID` | `Text` | Yes | - | `` | - |
| `ratio` | `Text` | No | `half` | `WFAppRatio` | half, thirdByTwo |

---

## Module: Documents

### `filterFiles`

**Title:** Filter Files  
**Apple Identifier:** `is.workflow.actions.filter.files`  

Filter the provided files with various filters.

```cherri
filterFiles(files: AnyContent, limit?: Number, sortBy?: Text, orderBy?: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `files` | `AnyContent` | Yes | - | `WFContentItemInputParameter` | - |
| `limit` | `Number` | No | - | `WFContentItemLimitNumber` | - |
| `sortBy` | `Text` | No | - | `WFContentItemSortProperty` | File Size, File Extension, Creation Date, File Path, Last Modified Date, Name, Random |
| `orderBy` | `Text` | No | - | `WFContentItemSortOrder` | Smallest First, Biggest First, Latest First, Oldest First, A to Z, Z to A |

---

### `getFileFromFolder`

**Title:** Get File From Folder  
**Apple Identifier:** `is.workflow.actions.documentpicker.open`  

Get a file from a folder. Prepend path with a `~` to access the home folder on macOS.

```cherri
getFileFromFolder(folder: Text, path: Text, errorIfNotFound?: Bool = "true")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `folder` | `Text` | Yes | - | `` | - |
| `path` | `Text` | Yes | - | `WFGetFilePath` | - |
| `errorIfNotFound` | `Bool` | No | `true` | `WFFileErrorIfNotFound` | - |

---

### `labelFile`

**Title:** Label File  
**Apple Identifier:** `is.workflow.actions.file.label`  

Label a file.

```cherri
labelFile(file: AnyContent, color: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `file` | `AnyContent` | Yes | - | `WFInput` | - |
| `color` | `Text` | Yes | - | `` | red, orange, yellow, green, blue, purple, gray |

---

### `openBook`

**Title:** Open Book  
**Apple Identifier:** `com.apple.iBooksX.OpenBookIntent`  

Opens a book in Books. `target` is expected to be a book reference.

```cherri
openBook(target: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `target` | `AnyContent` | Yes | - | `target` | - |

**App Intent:** `OpenBookIntent` (Bundle: `com.apple.iBooksX`)

---

### `playAudiobook`

**Title:** Play Audiobook  
**Apple Identifier:** `com.apple.iBooksX.PlayAudiobookIntent`  

Plays an audiobook in Books. `target` is expected to be a book or audiobook reference.

```cherri
playAudiobook(target: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `target` | `AnyContent` | Yes | - | `target` | - |

**App Intent:** `PlayAudiobookIntent` (Bundle: `com.apple.iBooksX`)

---

## Module: General

### `DNDOff`

**Title:** Turn Off Do Not Disturb  
**Apple Identifier:** `is.workflow.actions.dnd.set`  
```cherri
DNDOff()
```

---

### `DNDOn`

**Title:** Turn On Do Not Disturb  
**Apple Identifier:** `is.workflow.actions.dnd.set`  
```cherri
DNDOn()
```

---

### `addCalendar`

**Title:** Add Calendar  
**Apple Identifier:** `is.workflow.actions.addnewcalendar`  

Create a calendar with `name`.

```cherri
addCalendar(name: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `name` | `Text` | Yes | - | `CalendarName` | - |

---

### `addEvent`

**Title:** Add Event  
**Apple Identifier:** `is.workflow.actions.addnewevent`  

Add a new calendar event.

```cherri
addEvent(title: Text, startDate?: AnyContent, endDate?: AnyContent, allDay?: Bool, location?: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `title` | `Text` | Yes | - | `WFCalendarItemTitle` | - |
| `startDate` | `AnyContent` | No | - | `WFCalendarItemStartDate` | - |
| `endDate` | `AnyContent` | No | - | `WFCalendarItemEndDate` | - |
| `allDay` | `Bool` | No | - | `WFCalendarItemAllDay` | - |
| `location` | `Text` | No | - | `WFCalendarItemLocation` | - |

---

### `addQuickReminder`

**Title:** Add Quick Reminder  
**Apple Identifier:** `is.workflow.actions.addquickreminder`  
```cherri
addQuickReminder()
```

---

### `addReminder`

**Title:** Add Reminder  
**Apple Identifier:** `is.workflow.actions.addnewreminder`  

Add a new reminder.

```cherri
addReminder(title: Text, alert?: Bool)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `title` | `Text` | Yes | - | `WFCalendarItemTitle` | - |
| `alert` | `Bool` | No | - | `WFAlertEnabled` | - |

---

### `addToBooks`

**Title:** Add to Books  
**Apple Identifier:** `com.apple.iBooksX.openin`  

Add `input` to books. `input` is expected to be a PDF or epub file.

```cherri
addToBooks(input: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `input` | `AnyContent` | Yes | - | `BooksInput` | - |

---

### `addToGIF`

**Title:** Add To GIF  
**Apple Identifier:** `is.workflow.actions.addframetogif`  

Add a frame to a GIF.

```cherri
addToGIF(image: AnyContent, gif: AnyContent, delay?: Text = "0.25", autoSize?: Bool = "true", width?: Text, height?: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `image` | `AnyContent` | Yes | - | `WFImage` | - |
| `gif` | `AnyContent` | Yes | - | `WFInputGIF` | - |
| `delay` | `Text` | No | `0.25` | `WFGIFDelayTime` | - |
| `autoSize` | `Bool` | No | `true` | `WFGIFAutoSize` | - |
| `width` | `Text` | No | - | `WFGIFManualSizeWidth` | - |
| `height` | `Text` | No | - | `WFGIFManualSizeHeight` | - |

---

### `addToMusic`

**Title:** Add to Music  
**Apple Identifier:** `is.workflow.actions.addtoplaylist`  

Add music to library.

```cherri
addToMusic(songs: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `songs` | `AnyContent` | Yes | - | `WFInput` | - |

---

### `addToPlaylist`

**Title:** Add to Playlist  
**Apple Identifier:** `is.workflow.actions.addtoplaylist`  
```cherri
addToPlaylist(playlistName: Text, songs: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `playlistName` | `Text` | Yes | - | `WFPlaylistName` | - |
| `songs` | `AnyContent` | Yes | - | `WFInput` | - |

---

### `addWeatherLocation`

**Title:** Add Location to List  
**Apple Identifier:** `com.apple.weather.AddSavedLocationIntent`  

Add a location to Weather app.

```cherri
addWeatherLocation(location: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `location` | `AnyContent` | Yes | - | `placemark` | - |

---

### `adjustDate`

**Title:** Adjust Date  
**Apple Identifier:** `is.workflow.actions.adjustdate`  

Adjust a date or get the start of a time period.

```cherri
adjustDate(date: Text, operation: Text, unit?: Unknown)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `date` | `Text` | Yes | - | `WFDate` | - |
| `operation` | `Text` | Yes | - | `WFAdjustOperation` | Add, Subtract, Get Start of Minute, Get Start of Hour, Get Start of Day, Get Start of Week, Get Start of Month, Get Start of Year |
| `unit` | `Unknown` | No | - | `WFDuration` | sec, min, hr, days, weeks, months, yr |

---

### `adjustTextTone`

**Title:** Adjust Text Tone  
**Apple Identifier:** `com.apple.WritingTools.WritingToolsAppIntentsExtension.AdjustToneIntent`  
**Returns:** `Text`  

Adjust the tone of the text using Apple Intelligence Writing Tools.

```cherri
adjustTextTone(text: Text, tone: Text) -> Text
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `text` | `Text` | Yes | - | `text` | - |
| `tone` | `Text` | Yes | - | `tone` | friendly, professional, concise |

---

### `airdrop`

**Title:** AirDrop  
**Apple Identifier:** `is.workflow.actions.airdropdocument`  

Prompt the user to AirDrop `input`.

```cherri
airdrop(input: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `input` | `AnyContent` | Yes | - | `WFInput` | - |

---

### `alert`

**Title:** Alert  
**Apple Identifier:** `is.workflow.actions.alert`  
**Returns:** `Void`  

Shows an alert with text and optional title and an OK button to proceed.

```cherri
alert(alert: Text, title?: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `alert` | `Text` | Yes | - | `WFAlertActionMessage` | - |
| `title` | `Text` | No | - | `WFAlertActionTitle` | - |

---

### `alternatingCase`

**Title:** Alternating Case  
**Apple Identifier:** `is.workflow.actions.text.changecase`  
**Returns:** `Text`  

Capitalizes the text with alternating case.

```cherri
alternatingCase(text: Text) -> Text
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `text` | `Text` | Yes | - | `text` | - |

---

### `appendNote`

**Title:** Append Note  
**Apple Identifier:** `is.workflow.actions.appendnote`  

Append text to a note.

```cherri
appendNote(note: Text, input: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `note` | `Text` | Yes | - | `WFNote` | - |
| `input` | `Text` | Yes | - | `WFInput` | - |

---

### `appendToFile`

**Title:** Append File  
**Apple Identifier:** `is.workflow.actions.file.append`  

Append text to a file.

```cherri
appendToFile(filePath: Text, text: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `filePath` | `Text` | Yes | - | `WFFilePath` | - |
| `text` | `AnyContent` | Yes | - | `WFInput` | - |

---

### `askChatGPT`

**Title:** Ask Chat GPT  
**Apple Identifier:** `is.workflow.actions.askllm`  

Ask Chat GPT using a prompt. Follow up will open a follow-up prompt to the model.

```cherri
askChatGPT(prompt: Text, followUp?: Bool = "false", resultType?: Text = "Automatic")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `prompt` | `Text` | Yes | - | `WFLLMPrompt` | - |
| `followUp` | `Bool` | No | `false` | `FollowUp` | - |
| `resultType` | `Text` | No | `Automatic` | `WFGenerativeResultType` | Text, Number, Date, Boolean, List, Dictionary |

---

### `askCloudLLM`

**Title:** Ask Private Cloud Compute LLM  
**Apple Identifier:** `is.workflow.actions.askllm`  

Ask a private cloud compute LLM using a prompt. Follow up will open a follow-up prompt to the model.

```cherri
askCloudLLM(prompt: Text, followUp?: Bool = "false", resultType?: Text = "Automatic")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `prompt` | `Text` | Yes | - | `WFLLMPrompt` | - |
| `followUp` | `Bool` | No | `false` | `FollowUp` | - |
| `resultType` | `Text` | No | `Automatic` | `WFGenerativeResultType` | Text, Number, Date, Boolean, List, Dictionary |

---

### `askDeviceModel`

**Title:** Ask On-Device Model  
**Apple Identifier:** `is.workflow.actions.askllm`  

Ask an On-Device LLM using a prompt. Follow up will open a follow-up prompt to the model.

```cherri
askDeviceModel(prompt: Text, followUp?: Bool = "false", resultType?: Text = "Automatic")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `prompt` | `Text` | Yes | - | `WFLLMPrompt` | - |
| `followUp` | `Bool` | No | `false` | `FollowUp` | - |
| `resultType` | `Text` | No | `Automatic` | `WFGenerativeResultType` | Text, Number, Date, Boolean, List, Dictionary |

---

### `askLLM`

**Title:** Ask LLM  
**Apple Identifier:** `is.workflow.actions.askllm`  

Ask a Cloud AI model using a prompt. Follow up will open a follow-up prompt to the model.

```cherri
askLLM(prompt: Text, model?: Text = "Private Cloud Compute", followUp?: Bool = "false", resultType?: Text = "Automatic")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `prompt` | `Text` | Yes | - | `WFLLMPrompt` | - |
| `model` | `Text` | No | `Private Cloud Compute` | `WFLLMModel` | Private Cloud Compute, Apple Intelligence on Device, ChatGPT |
| `followUp` | `Bool` | No | `false` | `FollowUp` | - |
| `resultType` | `Text` | No | `Automatic` | `WFGenerativeResultType` | Text, Number, Date, Boolean, List, Dictionary |

---

### `askModel`

**Title:** Ask Cloud Model  
**Apple Identifier:** `is.workflow.actions.askllm`  

Ask a Private Cloud AI model using a prompt. Follow up will open a follow-up prompt to the model. Allow search uses Broad World Knowledge, allowing the model to search the web for up-to-date information.

```cherri
askModel(prompt: Text, followUp?: Bool = "false", allowSearch?: Bool = "false", resultType?: Text = "Automatic")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `prompt` | `Text` | Yes | - | `WFLLMPrompt` | - |
| `followUp` | `Bool` | No | `false` | `FollowUp` | - |
| `allowSearch` | `Bool` | No | `false` | `WFAllowWebSearch` | - |
| `resultType` | `Text` | No | `Automatic` | `WFGenerativeResultType` | Text, Number, Date, Boolean, List, Dictionary |

---

### `askProModel`

**Title:** Ask Cloud Pro Model  
**Apple Identifier:** `is.workflow.actions.askllm`  

Ask a Private Cloud Pro AI model with increased reasoning using a prompt. May require subscription. Follow up will open a follow-up prompt to the model. Allow search uses Broad World Knowledge, allowing the model to search the web for up-to-date information.

```cherri
askProModel(prompt: Text, followUp?: Bool = "false", allowSearch?: Bool = "false", resultType?: Text = "Automatic")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `prompt` | `Text` | Yes | - | `WFLLMPrompt` | - |
| `followUp` | `Bool` | No | `false` | `FollowUp` | - |
| `allowSearch` | `Bool` | No | `false` | `WFAllowWebSearch` | - |
| `resultType` | `Text` | No | `Automatic` | `WFGenerativeResultType` | Text, Number, Date, Boolean, List, Dictionary |

---

### `base64Decode`

**Title:** Base 64 Decode  
**Apple Identifier:** `is.workflow.actions.base64encode`  
**Returns:** `AnyContent`  

Base 64 decodes input.

```cherri
base64Decode(input: AnyContent, lineBreakMode?: Text) -> AnyContent
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `input` | `AnyContent` | Yes | - | `WFInput` | - |
| `lineBreakMode` | `Text` | No | - | `WFBase64LineBreakMode` | - |

---

### `base64Encode`

**Title:** Base 64 Encode  
**Apple Identifier:** `is.workflow.actions.base64encode`  
**Returns:** `Text`  

Base 64 encodes input.

```cherri
base64Encode(encodeInput: AnyContent, lineBreakMode?: Text) -> Text
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `encodeInput` | `AnyContent` | Yes | - | `WFInput` | - |
| `lineBreakMode` | `Text` | No | - | `WFBase64LineBreakMode` | - |

---

### `calculate`

**Title:** Calculate  
**Apple Identifier:** `is.workflow.actions.math`  
**Returns:** `Number`  

Perform various calculation operations using one or two operands.

```cherri
calculate(operation: Text, operandOne: Number, operandTwo?: Number) -> Number
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `operation` | `Text` | Yes | - | `WFScientificMathOperation` | x^2, х^3, x^у, e^x, 10^x, In(x), log(x), √x, ∛x, x!, sin(x), cos(X), tan(x), abs(x), Modulus |
| `operandOne` | `Number` | Yes | - | `WFInput` | - |
| `operandTwo` | `Number` | No | - | `WFScientificMathOperand` | - |

---

### `call`

**Title:** Call  
**Apple Identifier:** `com.apple.mobilephone.call`  

Call a contact.

```cherri
call(contact: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `contact` | `AnyContent` | Yes | - | `WFCallContact` | - |

---

### `capitalize`

**Title:** Capitalize  
**Apple Identifier:** `is.workflow.actions.text.changecase`  
**Returns:** `Text`  

Capitalizes the text with sentence case.

```cherri
capitalize(text: Text) -> Text
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `text` | `Text` | Yes | - | `text` | - |

---

### `capitalizeAll`

**Title:** Capitalize All  
**Apple Identifier:** `is.workflow.actions.text.changecase`  
**Returns:** `Text`  

Capitalizes every word in the text.

```cherri
capitalizeAll(text: Text) -> Text
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `text` | `Text` | Yes | - | `text` | - |

---

### `ceil`

**Title:** Round Up  
**Apple Identifier:** `is.workflow.actions.round`  
**Returns:** `Number`  

Always round a number up to a specified rounding place.

```cherri
ceil(number: Number, roundTo?: Text = "Integer") -> Number
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `number` | `Number` | Yes | - | `WFInput` | - |
| `roundTo` | `Text` | No | `Integer` | `WFRoundTo` | Millions, Hundred Thousands, Ten Thousands, Thousands, Hundreds, Tens, Integer, Tenths, Hundredths, Thousandths, Ten-Thousandths, Hundred-Thousandths, Millionths, Ten-Millionths, Hundred-Millionths, Billionths, 10^ |

---

### `chooseFromList`

**Title:** Choose from List  
**Apple Identifier:** `is.workflow.actions.choosefromlist`  

Prompts the user to choose from a list.

```cherri
chooseFromList(list: AnyContent, prompt?: Text, selectMultiple?: Bool = "false", selectAll?: Bool = "false")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `list` | `AnyContent` | Yes | - | `WFInput` | - |
| `prompt` | `Text` | No | - | `WFChooseFromListActionPrompt` | - |
| `selectMultiple` | `Bool` | No | `false` | `WFChooseFromListActionSelectMultiple` | - |
| `selectAll` | `Bool` | No | `false` | `WFChooseFromListActionSelectAll` | - |

---

### `clearUpNext`

**Title:** Clear Up Next  
**Apple Identifier:** `is.workflow.actions.clearupnext`  

Clear the queue.

```cherri
clearUpNext()
```

---

### `combineImages`

**Title:** Combine Images  
**Apple Identifier:** `is.workflow.actions.image.combine`  
```cherri
combineImages(images: AnyContent, mode?: Text = "Vertically", spacing?: Number = "1")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `images` | `AnyContent` | Yes | - | `WFInput` | - |
| `mode` | `Text` | No | `Vertically` | `WFImageCombineMode` | Vertically, In a Grid |
| `spacing` | `Number` | No | `1` | `WFImageCombineSpacing` | - |

---

### `comment`

**Title:** Comment  
**Apple Identifier:** `is.workflow.actions.comment`  

Add an explicit comment.

```cherri
comment(text: Unknown)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `text` | `Unknown` | Yes | - | `WFCommentActionText` | - |

---

### `confirm`

**Title:** Confirm  
**Apple Identifier:** `is.workflow.actions.alert`  

Shows an alert with text and optional title. It displays an OK button to proceed, and a cancel button that stops the Shortcut.

```cherri
confirm(alert: Text, title?: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `alert` | `Text` | Yes | - | `WFAlertActionMessage` | - |
| `title` | `Text` | No | - | `WFAlertActionTitle` | - |

---

### `connectToServer`

**Title:** Connect to Server  
**Apple Identifier:** `is.workflow.actions.connecttoservers`  

Connect to file server at `url`.

```cherri
connectToServer(url: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `url` | `Text` | Yes | - | `WFInput` | - |

---

### `connectedToCharger`

**Title:** Connected to Charger  
**Apple Identifier:** `is.workflow.actions.getbatterylevel`  
**Returns:** `Bool`  

Determines if the device is currently connected to a charger.

```cherri
connectedToCharger() -> Bool
```

---

### `contentGraph`

**Title:** Content Graph  
**Apple Identifier:** `is.workflow.actions.viewresult`  

Display input as a content graph.

```cherri
contentGraph(input: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `input` | `AnyContent` | Yes | - | `WFInput` | - |

---

### `convertImage`

**Title:** Convert Image  
**Apple Identifier:** `is.workflow.actions.image.convert`  
**Returns:** `Image`  

Convert image to another format, compression quality, and/or remove metadata.

```cherri
convertImage(image: Image, format: Text, quality?: Number, preserveMetadata?: Bool = "true") -> Image
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `image` | `Image` | Yes | - | `WFInput` | - |
| `format` | `Text` | Yes | - | `WFImageFormat` | TIFF, GIF, PNG, BMP, PDF, HEIF |
| `quality` | `Number` | No | - | `WFImageCompressionQuality` | - |
| `preserveMetadata` | `Bool` | No | `true` | `WFImagePreserveMetadata` | - |

---

### `convertToJPEG`

**Title:** Convert to JPEG  
**Apple Identifier:** `is.workflow.actions.image.convert`  

Convert image to a JPEG.

```cherri
convertToJPEG(image: AnyContent, compressionQuality?: Number, preserveMetadata?: Bool = "true")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `image` | `AnyContent` | Yes | - | `WFInput` | - |
| `compressionQuality` | `Number` | No | - | `WFImageCompressionQuality` | - |
| `preserveMetadata` | `Bool` | No | `true` | `WFImagePreserveMetadata` | - |

---

### `convertToUSDZ`

**Title:** Convert to USDZ  
**Apple Identifier:** `com.apple.HydraUSDAppIntents.ConvertToUSDZ`  
```cherri
convertToUSDZ(file: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `file` | `AnyContent` | Yes | - | `file` | - |

---

### `correctSpelling`

**Title:** Correct Spelling  
**Apple Identifier:** `is.workflow.actions.correctspelling`  
**Returns:** `Text`  

Corrects the spelling of the provided text.

```cherri
correctSpelling(text: Text) -> Text
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `text` | `Text` | Yes | - | `text` | - |

---

### `count`

**Title:** Count  
**Apple Identifier:** `is.workflow.actions.count`  
**Returns:** `Number`  

Returns a count of `type` of items in `input`.

```cherri
count(input: AnyContent, type?: Text = "Items") -> Number
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `input` | `AnyContent` | Yes | - | `Input` | - |
| `type` | `Text` | No | `Items` | `WFCountType` | Items, Characters, Words, Sentences, Lines |

---

### `createAlbum`

**Title:** Create Album  
**Apple Identifier:** `is.workflow.actions.photos.createalbum`  
```cherri
createAlbum(name: Text, images?: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `name` | `Text` | Yes | - | `AlbumName` | - |
| `images` | `AnyContent` | No | - | `WFInput` | - |

---

### `createFolder`

**Title:** Create Folder  
**Apple Identifier:** `is.workflow.actions.file.createfolder`  

Create a folder within the Shortcuts folder or within a `#ref` to a folder.

```cherri
createFolder(path: Text, folder?: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `path` | `Text` | Yes | - | `WFFilePath` | - |
| `folder` | `AnyContent` | No | - | `WFFolder` | - |

---

### `createPlaylist`

**Title:** Create Playlist  
**Apple Identifier:** `is.workflow.actions.createplaylist`  

Create a new music playlist.

```cherri
createPlaylist(name: Text, author?: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `name` | `Text` | Yes | - | `WFPlaylistName` | - |
| `author` | `Text` | No | - | `WFPlaylistAuthor` | - |

---

### `cropImage`

**Title:** Crop Image  
**Apple Identifier:** `is.workflow.actions.image.crop`  
```cherri
cropImage(image: AnyContent, width?: Text = "100", height?: Text = "100", position?: Text = "Center", customPositionX?: Text, customPositionY?: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `image` | `AnyContent` | Yes | - | `WFInput` | - |
| `width` | `Text` | No | `100` | `WFImageCropWidth` | - |
| `height` | `Text` | No | `100` | `WFImageCropHeight` | - |
| `position` | `Text` | No | `Center` | `WFImageCropPosition` | Center, Top Left, Top Right, Bottom Left, Bottom Right, Custom |
| `customPositionX` | `Text` | No | - | `WFImageCropX` | - |
| `customPositionY` | `Text` | No | - | `WFImageCropY` | - |

---

### `currentDate`

**Title:** Current Date  
**Apple Identifier:** `is.workflow.actions.date`  

Get the current date.

```cherri
currentDate()
```

---

### `currentLocation`

**Title:** Current Location  
**Apple Identifier:** `is.workflow.actions.location`  

Get current user location.

```cherri
currentLocation()
```

---

### `customImageMask`

**Title:** Custom Image Mask  
**Apple Identifier:** `is.workflow.actions.image.mask`  

Mask an image with a custom image mask.

```cherri
customImageMask(image: AnyContent, customMaskImage: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `image` | `AnyContent` | Yes | - | `WFInput` | - |
| `customMaskImage` | `AnyContent` | Yes | - | `WFCustomMaskImage` | - |

---

### `customImageOverlay`

**Title:** Custom Image Overlay  
**Apple Identifier:** `is.workflow.actions.overlayimageonimage`  

Specify custom image overlay configuration.

```cherri
customImageOverlay(image: AnyContent, overlayImage: AnyContent, width?: Text, height?: Text, rotation?: Text = "0", opacity?: Text = "100", position?: Text = "Center", customPositionX?: Text, customPositionY?: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `image` | `AnyContent` | Yes | - | `WFInput` | - |
| `overlayImage` | `AnyContent` | Yes | - | `WFImage` | - |
| `width` | `Text` | No | - | `WFImageWidth` | - |
| `height` | `Text` | No | - | `WFImageHeight` | - |
| `rotation` | `Text` | No | `0` | `WFRotation` | - |
| `opacity` | `Text` | No | `100` | `WFOverlayImageOpacity` | - |
| `position` | `Text` | No | `Center` | `WFImagePosition` | Center, Top Left, Top Right, Bottom Left, Bottom Right, Custom |
| `customPositionX` | `Text` | No | - | `WFImageX` | - |
| `customPositionY` | `Text` | No | - | `WFImageY` | - |

---

### `darkMode`

**Title:** Set Appearance to Dark  
**Apple Identifier:** `is.workflow.actions.appearance`  
```cherri
darkMode()
```

---

### `date`

**Title:** Date  
**Apple Identifier:** `is.workflow.actions.date`  
**Returns:** `Date`  

Create a date value from `date`. Example: October 5, 2022.

```cherri
date(date: Text) -> Date
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `date` | `Text` | Yes | - | `WFDateActionDate` | - |

---

### `define`

**Title:** Define  
**Apple Identifier:** `is.workflow.actions.showdefinition`  
**Returns:** `Text`  

Returns the definition of the word.

```cherri
define(word: Text) -> Text
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `word` | `Text` | Yes | - | `Word` | - |

---

### `deleteFiles`

**Title:** Delete Files  
**Apple Identifier:** `is.workflow.actions.file.delete`  

Delete a file or multiple files.

```cherri
deleteFiles(input: AnyContent, immediately?: Bool = "false")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `input` | `AnyContent` | Yes | - | `WFInput` | - |
| `immediately` | `Bool` | No | `false` | `WFDeleteImmediatelyDelete` | - |

---

### `deletePhotos`

**Title:** Delete Photos  
**Apple Identifier:** `is.workflow.actions.deletephotos`  
```cherri
deletePhotos(photos: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `photos` | `AnyContent` | Yes | - | `photos` | - |

---

### `deleteStoredValue`

**Title:** Delete Stored Content  
**Apple Identifier:** `is.workflow.actions.deletestoredcontent`  

Delete previously stored content, optionally globally or specific to the Shortcut.

```cherri
deleteStoredValue(key: Text, global?: Bool = "false")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `key` | `Text` | Yes | - | `WFStoredContentKey` | - |
| `global` | `Bool` | No | `false` | `WFStoredContentGlobalValue` | - |

---

### `detectLanguage`

**Title:** Detect Language  
**Apple Identifier:** `is.workflow.actions.detectlanguage`  
**Returns:** `Text`  

Detect the language of the text in `input`.

```cherri
detectLanguage(input: Text) -> Text
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `input` | `Text` | Yes | - | `WFInput` | - |

---

### `dismissSiri`

**Title:** Dismiss Siri  
**Apple Identifier:** `is.workflow.actions.dismisssiri`  

Dismisses Siri.

```cherri
dismissSiri()
```

---

### `displaySleep`

**Title:** Display Sleep  
**Apple Identifier:** `is.workflow.actions.displaysleep`  

Only puts the Mac display to sleep, device remains awake.

```cherri
displaySleep()
```

---

### `downloadURL`

**Title:** Download URL  
**Apple Identifier:** `is.workflow.actions.downloadurl`  
**Returns:** `AnyContent`  

Download the contents of a URL.

```cherri
downloadURL(url: Text, headers?: Map<Text, AnyContent>) -> AnyContent
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `url` | `Text` | Yes | - | `WFURL` | - |
| `headers` | `Map<Text, AnyContent>` | No | - | `WFHTTPHeaders` | - |

---

### `editEvent`

**Title:** Edit Event  
**Apple Identifier:** `is.workflow.actions.setters.calendarevents`  

Edit a detail of an event. Provide an event, a detail to modify, and a new value for that detail.

```cherri
editEvent(event: AnyContent, detail: Text, newValue: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `event` | `AnyContent` | Yes | - | `WFInput` | - |
| `detail` | `Text` | Yes | - | `WFContentItemPropertyName` | Start Date, End Date, Is All Day, Location, Duration, My Status, Attendees, URL, Title, Notes, Attachments |
| `newValue` | `Text` | Yes | - | `WFCalendarEventContentItemStartDate` | - |

---

### `encodeAudio`

**Title:** Encode Audio  
**Apple Identifier:** `is.workflow.actions.encodemedia`  

Encode audio to a different format and/or speed.

```cherri
encodeAudio(audio: AnyContent, format?: Text = "M4A", speed?: Text = "Normal")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `audio` | `AnyContent` | Yes | - | `WFMedia` | - |
| `format` | `Text` | No | `M4A` | `WFMediaAudioFormat` | M4A, AIFF |
| `speed` | `Text` | No | `Normal` | `WFMediaCustomSpeed` | 0.5X, Normal, 2X |

---

### `encodeVideo`

**Title:** Encode Video  
**Apple Identifier:** `is.workflow.actions.encodemedia`  

Encode a video to a different format, size and/or speed.

```cherri
encodeVideo(video: AnyContent, size?: Text = "Passthrough", speed?: Text = "Normal", preserveTransparency?: Bool = "false")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `video` | `AnyContent` | Yes | - | `WFMedia` | - |
| `size` | `Text` | No | `Passthrough` | `WFMediaSize` | 640×480, 960×540, 1280×720, 1920×1080, 3840×2160, HEVC 1920×1080, HEVC 3840x2160, ProRes 422 |
| `speed` | `Text` | No | `Normal` | `WFMediaCustomSpeed` | 0.5X, Normal, 2X |
| `preserveTransparency` | `Bool` | No | `false` | `WFMediaPreserveTransparency` | - |

---

### `expandURL`

**Title:** Expand URL  
**Apple Identifier:** `is.workflow.actions.url.expand`  

Get the expanded version of a URL. For instance, returns the full URL for a short URL, or the URL which a URL immediately redirects to, etc.

```cherri
expandURL(url: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `url` | `Text` | Yes | - | `URL` | - |

---

### `extractArchive`

**Title:** Extract Archive  
**Apple Identifier:** `is.workflow.actions.unzip`  

Extract files from the archive `file`.

```cherri
extractArchive(file: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `file` | `AnyContent` | Yes | - | `WFArchive` | - |

---

### `extractImageText`

**Title:** Extract Image Text  
**Apple Identifier:** `is.workflow.actions.extracttextfromimage`  
**Returns:** `Text`  
```cherri
extractImageText(image: AnyContent) -> Text
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `image` | `AnyContent` | Yes | - | `WFImage` | - |

---

### `facetimeCall`

**Title:** FaceTime Call  
**Apple Identifier:** `com.apple.facetime.facetime`  

Starts a FaceTime audio or video call with the contact.

```cherri
facetimeCall(contact: AnyContent, type?: Text = "Video")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `contact` | `AnyContent` | Yes | - | `WFFaceTimeContact` | - |
| `type` | `Text` | No | `Video` | `WFFaceTimeType` | Video, Audio |

---

### `file`

**Title:** File  
**Apple Identifier:** `is.workflow.actions.file`  
**Returns:** `AnyContent`  

Insert a `#ref` to a file.

```cherri
file(file: AnyContent) -> AnyContent
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `file` | `AnyContent` | Yes | - | `WFFile` | - |

---

### `fileRequest`

**Title:** File Request  
**Apple Identifier:** `is.workflow.actions.downloadurl`  

Send a `method` file request to `url` with `body` and optional `headers.

```cherri
fileRequest(url: Text, method?: Text, body?: AnyContent, headers?: Map<Text, AnyContent>)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `url` | `Text` | Yes | - | `WFURL` | - |
| `method` | `Text` | No | - | `WFHTTPMethod` | POST, PUT, PATCH, DELETE |
| `body` | `AnyContent` | No | - | `WFRequestVariable` | - |
| `headers` | `Map<Text, AnyContent>` | No | - | `WFHTTPHeaders` | - |

---

### `fileSize`

**Title:** File Size  
**Apple Identifier:** `is.workflow.actions.format.filesize`  

Returns the size of a file.

```cherri
fileSize(file: Text, format: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `file` | `Text` | Yes | - | `WFFileSize` | - |
| `format` | `Text` | Yes | - | `WFFileSizeFormat` | Closest Unit, Bytes, Kilobytes, Megabytes, Gigabytes, Terabytes, Petabytes, Exabytes, Zettabytes, Yottabytes |

---

### `filterContacts`

**Title:** Filter Contacts  
**Apple Identifier:** `is.workflow.actions.filter.contacts`  
```cherri
filterContacts(contacts: AnyContent, sortBy?: Text, sortOrder?: Text = "A to Z", limit?: Number)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `contacts` | `AnyContent` | Yes | - | `WFContentItemInputParameter` | - |
| `sortBy` | `Text` | No | - | `WFContentItemSortProperty` | First Name, Middle Name, Last Name, Birthday, Prefix, Suffix, Nickname, Phonetic First Name, Phonetic Last Name, Phonetic Middle Name, Company, Job Title, Department, File Extension, Creation Date, File Path, Last Modified Date, Name, Random |
| `sortOrder` | `Text` | No | `A to Z` | `WFContentItemSortOrder` | A to Z, Z to A |
| `limit` | `Number` | No | - | `WFContentItemLimitNumber` | - |

---

### `findConversation`

**Title:** Find Conversation  
**Apple Identifier:** `com.apple.MobileSMS.ConversationEntity`  

Find SMS/iMessage conversation.

```cherri
findConversation(search: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `search` | `Text` | Yes | - | `WFContentItemInputParameter` | - |

---

### `findEmail`

**Title:** Find Email  
**Apple Identifier:** `com.apple.mobilemail.MailMessageEntity`  

Find email.

```cherri
findEmail(search: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `search` | `Text` | Yes | - | `WFContentItemInputParameter` | - |

---

### `findMessage`

**Title:** Find Message  
**Apple Identifier:** `com.apple.MobileSMS.MessageEntity`  

Find SMS/iMessage message.

```cherri
findMessage(search: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `search` | `Text` | Yes | - | `WFContentItemInputParameter` | - |

---

### `flashlightOff`

**Title:** Turn Off Flashlight  
**Apple Identifier:** `is.workflow.actions.flashlight`  

Turn off the flashlight on the device.

```cherri
flashlightOff()
```

---

### `flashlightOn`

**Title:** Turn On Flashlight  
**Apple Identifier:** `is.workflow.actions.flashlight`  

Turn on the flashlight on the device with optional brightness setting.

```cherri
flashlightOn(brightness: Number)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `brightness` | `Number` | No | `0.5` | `WFFlashlightLevel` | - |

---

### `flipImage`

**Title:** Flip Image  
**Apple Identifier:** `is.workflow.actions.image.flip`  
```cherri
flipImage(image: AnyContent, direction: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `image` | `AnyContent` | Yes | - | `WFInput` | - |
| `direction` | `Text` | Yes | - | `WFImageFlipDirection` | Horizontal, Vertical |

---

### `floor`

**Title:** Round Down  
**Apple Identifier:** `is.workflow.actions.round`  
**Returns:** `Number`  

Always round a number down to a specified rounding place.

```cherri
floor(number: Number, roundTo?: Text = "Integer") -> Number
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `number` | `Number` | Yes | - | `WFInput` | - |
| `roundTo` | `Text` | No | `Integer` | `WFRoundTo` | Millions, Hundred Thousands, Ten Thousands, Thousands, Hundreds, Tens, Integer, Tenths, Hundredths, Thousandths, Ten-Thousandths, Hundred-Thousandths, Millionths, Ten-Millionths, Hundred-Millionths, Billionths, 10^ |

---

### `formRequest`

**Title:** Form Request  
**Apple Identifier:** `is.workflow.actions.downloadurl`  

Send a `method` request to `url` with `body` and optional `headers.

```cherri
formRequest(url: Text, method?: Text, body?: Map<Text, AnyContent>, headers?: Map<Text, AnyContent>)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `url` | `Text` | Yes | - | `WFURL` | - |
| `method` | `Text` | No | - | `WFHTTPMethod` | POST, PUT, PATCH, DELETE |
| `body` | `Map<Text, AnyContent>` | No | - | `WFFormValues` | - |
| `headers` | `Map<Text, AnyContent>` | No | - | `WFHTTPHeaders` | - |

---

### `formatDate`

**Title:** Format Date  
**Apple Identifier:** `is.workflow.actions.format.date`  
**Returns:** `Text`  

Format a date using a standard or custom format.

```cherri
formatDate(date: Date, dateFormat?: Text = "Short", customDateFormat?: Text) -> Text
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `date` | `Date` | Yes | - | `WFDate` | - |
| `dateFormat` | `Text` | No | `Short` | `WFDateFormatStyle` | None, Short, Medium, Long, Relative, RFC 2822, ISO 8601, Custom |
| `customDateFormat` | `Text` | No | - | `WFDateFormat` | - |

---

### `formatNumber`

**Title:** Format Number  
**Apple Identifier:** `is.workflow.actions.format.number`  
**Returns:** `Number`  

Format number based on decimal place.

```cherri
formatNumber(number: Number, decimalPlaces?: Number = "2") -> Number
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `number` | `Number` | Yes | - | `WFNumber` | - |
| `decimalPlaces` | `Number` | No | `2` | `WFNumberFormatDecimalPlaces` | - |

---

### `formatTime`

**Title:** Format Time  
**Apple Identifier:** `is.workflow.actions.format.date`  
**Returns:** `Text`  

Format a time using a standard or custom format.

```cherri
formatTime(time: Text, timeFormat?: Text = "Short") -> Text
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `time` | `Text` | Yes | - | `WFDate` | - |
| `timeFormat` | `Text` | No | `Short` | `WFTimeFormatStyle` | None, Short, Medium, Long, Relative |

---

### `formatTimestamp`

**Title:** Format Timestamp  
**Apple Identifier:** `is.workflow.actions.format.date`  
**Returns:** `Text`  

Format a timestamp using standard formats and/or a custom date format.

```cherri
formatTimestamp(date: Text, dateFormat?: Text = "Short", timeFormat?: Text = "Short", customDateFormat?: Text) -> Text
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `date` | `Text` | Yes | - | `WFDate` | - |
| `dateFormat` | `Text` | No | `Short` | `WFDateFormatStyle` | None, Short, Medium, Long, Relative, RFC 2822, ISO 8601, Custom |
| `timeFormat` | `Text` | No | `Short` | `WFTimeFormatStyle` | None, Short, Medium, Long, Relative |
| `customDateFormat` | `Text` | No | - | `WFDateFormat` | - |

---

### `generateKeyPoints`

**Title:** Summarize Text Key Points  
**Apple Identifier:** `com.apple.WritingTools.WritingToolsAppIntentsExtension.SummarizeTextIntent`  
**Returns:** `Text`  

Generate a summary of key points of text using Apple Intelligence Writing Tools.

```cherri
generateKeyPoints(text: Text) -> Text
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `text` | `Text` | Yes | - | `text` | - |

---

### `generateList`

**Title:** Make List From Text  
**Apple Identifier:** `com.apple.WritingTools.WritingToolsAppIntentsExtension.FormatListIntent`  
**Returns:** `Text`  

Generate list from text  using Apple Intelligence Writing Tools.

```cherri
generateList(text: Text) -> Text
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `text` | `Text` | Yes | - | `text` | - |

---

### `generateProofread`

**Title:** Proofread Text  
**Apple Identifier:** `com.apple.WritingTools.WritingToolsAppIntentsExtension.ProofreadIntent`  
**Returns:** `Text`  

Generate a proofread version of text using Apple Intelligence Writing Tools.

```cherri
generateProofread(text: Text) -> Text
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `text` | `Text` | Yes | - | `text` | - |

---

### `generateRewrite`

**Title:** Rewrite Text  
**Apple Identifier:** `com.apple.WritingTools.WritingToolsAppIntentsExtension.RewriteTextIntent`  
**Returns:** `Text`  

Generate a rewritten version of text using Apple Intelligence Writing Tools.

```cherri
generateRewrite(text: Text) -> Text
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `text` | `Text` | Yes | - | `text` | - |

---

### `generateSummary`

**Title:** Summarize Text  
**Apple Identifier:** `com.apple.WritingTools.WritingToolsAppIntentsExtension.SummarizeTextIntent`  
**Returns:** `Text`  

Generate a summarized version of text using Apple Intelligence Writing Tools.

```cherri
generateSummary(text: Text) -> Text
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `text` | `Text` | Yes | - | `text` | - |

---

### `generateTable`

**Title:** Make Table From Text  
**Apple Identifier:** `com.apple.WritingTools.WritingToolsAppIntentsExtension.FormatTableIntent`  
**Returns:** `Text`  

Generate table from text using Apple Intelligence Writing Tools.

```cherri
generateTable(text: Text) -> Text
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `text` | `Text` | Yes | - | `text` | - |

---

### `getAddresses`

**Title:** Get Addresses  
**Apple Identifier:** `is.workflow.actions.detect.address`  

Get addresses from `input`.

```cherri
getAddresses(input: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `input` | `AnyContent` | Yes | - | `WFInput` | - |

---

### `getAlarms`

**Title:** Get Alarms  
**Apple Identifier:** `com.apple.mobiletimer-framework.MobileTimerIntents.MTGetAlarmsIntent`  

Returns all of the alarms on the device.

```cherri
getAlarms()
```

---

### `getAllWallpapers`

**Title:** Get Wallpapers  
**Apple Identifier:** `is.workflow.actions.posters.get`  
**Returns:** `List<AnyContent>`  

Get device wallpapers.

```cherri
getAllWallpapers() -> List<AnyContent>
```

---

### `getAppStoreDetail`

**Title:** Get App Store Detail  
**Apple Identifier:** `is.workflow.actions.properties.appstore`  

Get a detail about an App Store app.

```cherri
getAppStoreDetail(app: AnyContent, detail: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `app` | `AnyContent` | Yes | - | `WFInput` | - |
| `detail` | `Text` | Yes | - | `WFContentItemPropertyName` | - |

---

### `getApps`

**Title:** Get Apps  
**Apple Identifier:** `is.workflow.actions.filter.apps`  
**Returns:** `List<AnyContent>`  

Get list of applications.

```cherri
getApps() -> List<AnyContent>
```

---

### `getArticle`

**Title:** Get Article  
**Apple Identifier:** `is.workflow.actions.getarticle`  

Get article from webpage.

```cherri
getArticle(webpage: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `webpage` | `Text` | Yes | - | `WFWebPage` | - |

---

### `getArticleDetail`

**Title:** Get Article Detail  
**Apple Identifier:** `is.workflow.actions.properties.articles`  

Get a detail about an article.

```cherri
getArticleDetail(article: AnyContent, detail: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `article` | `AnyContent` | Yes | - | `WFInput` | - |
| `detail` | `Text` | Yes | - | `WFContentItemPropertyName` | - |

---

### `getBatteryLevel`

**Title:** Get Battery Level  
**Apple Identifier:** `is.workflow.actions.getbatterylevel`  

Get the current level of charge of the device battery.

```cherri
getBatteryLevel()
```

---

### `getCellularDetail`

**Title:** Get Cellular Detail  
**Apple Identifier:** `is.workflow.actions.getwifi`  

Get a detail of the current Cellular network.

```cherri
getCellularDetail(detail: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `detail` | `Text` | Yes | - | `WFCellularDetail` | Carrier Name, Radio Technology, Country Code, Is Roaming Abroad, Number of Signal Bars |

---

### `getChargeLimit`

**Title:** Charge Limit  
**Apple Identifier:** `is.workflow.actions.getbatterylevel`  
**Returns:** `Number`  

Returns the current charge limit of the device battery.

```cherri
getChargeLimit() -> Number
```

---

### `getClipboard`

**Title:** Get Clipboard  
**Apple Identifier:** `is.workflow.actions.getclipboard`  

Get the contents of the clipboard.

```cherri
getClipboard()
```

---

### `getContactDetail`

**Title:** Get Detail of Contact  
**Apple Identifier:** `is.workflow.actions.properties.contacts`  

Get a detail about a contact.

```cherri
getContactDetail(contact: AnyContent, detail: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `contact` | `AnyContent` | Yes | - | `WFInput` | - |
| `detail` | `Text` | Yes | - | `WFContentItemPropertyName` | First Name, Middle Name, Last Name, Birthday, Prefix, Suffix, Nickname, Phonetic First Name, Phonetic Last Name, Phonetic Middle Name, Company, Job Title, Department, File Extension, Creation Date, File Path, Last Modified Date, Name, Random |

---

### `getContacts`

**Title:** Get Contacts  
**Apple Identifier:** `is.workflow.actions.detect.contacts`  
**Returns:** `List<AnyContent>`  

Get contacts from input.

```cherri
getContacts(input: AnyContent) -> List<AnyContent>
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `input` | `AnyContent` | Yes | - | `WFInput` | - |

---

### `getCurrentLocation`

**Title:** Get Current Location  
**Apple Identifier:** `is.workflow.actions.getcurrentlocation`  
```cherri
getCurrentLocation()
```

---

### `getCurrentSong`

**Title:** Get Current Song  
**Apple Identifier:** `is.workflow.actions.getcurrentsong`  

Gets the currently playing song.

```cherri
getCurrentSong()
```

---

### `getCurrentURL`

**Title:** Get Current URL  
**Apple Identifier:** `is.workflow.actions.safari.geturl`  

Get current URL in Safari.

```cherri
getCurrentURL()
```

---

### `getCurrentWeather`

**Title:** Get Current Weather  
**Apple Identifier:** `is.workflow.actions.weather.currentconditions`  

Get the current weather for a location.

```cherri
getCurrentWeather(location: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `location` | `Text` | No | `Current Location` | `WFWeatherCustomLocation` | - |

---

### `getDates`

**Title:** Get Dates  
**Apple Identifier:** `is.workflow.actions.detect.date`  

Get dates from input.

```cherri
getDates(input: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `input` | `AnyContent` | Yes | - | `WFInput` | - |

---

### `getDeviceDetail`

**Title:** Get Device Detail  
**Apple Identifier:** `is.workflow.actions.getdevicedetails`  

Get a detail about current device.

```cherri
getDeviceDetail(detail: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `detail` | `Text` | Yes | - | `WFDeviceDetail` | Device Name, Device Hostname, Device Model, Device Is Watch, System Version, Screen Width, Screen Height, Current Volume, Current Brightness, Current Appearance |

---

### `getDeviceUsage`

**Title:** Get Website & App Activity  
**Apple Identifier:** `com.apple.intelligenceplatform.IntelligencePlatform.IntelligencePlatformDataActionsAppIntentsExtension.CalculateAppUsageIntent`  
```cherri
getDeviceUsage(usageType: Text, device?: AnyContent, during?: Text = "today", startTime?: Text, startTime?: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `usageType` | `Text` | No | `all` | `activityType` | all, app, website |
| `device` | `AnyContent` | No | - | `selectedDevice` | - |
| `during` | `Text` | No | `today` | `during` | today, yesterday, lastWeek, thisWeek, thisMonth, thisYear, specifiedDay, inBetween |
| `startTime` | `Text` | No | - | `startTime` | - |
| `startTime` | `Text` | No | - | `endTime` | - |

---

### `getDictionary`

**Title:** Get Dictionary  
**Apple Identifier:** `is.workflow.actions.detect.dictionary`  
**Returns:** `Map<Text, AnyContent>`  

Get the dictionary from `input`.

```cherri
getDictionary(input: AnyContent) -> Map<Text, AnyContent>
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `input` | `AnyContent` | Yes | - | `WFInput` | - |

---

### `getDirections`

**Title:** Get Directions  
**Apple Identifier:** `is.workflow.actions.getdirections`  

Get directions to a destination.

```cherri
getDirections(destination: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `destination` | `AnyContent` | No | - | `WFDestination` | - |

---

### `getDistance`

**Title:** Get Distance  
**Apple Identifier:** `is.workflow.actions.getdistance`  

Get distance to a destination.

```cherri
getDistance(destination: AnyContent, mode?: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `destination` | `AnyContent` | No | - | `WFGetDistanceDestination` | - |
| `mode` | `Text` | No | - | `WFGetDirectionsActionMode` | - |

---

### `getEmails`

**Title:** Get Emails  
**Apple Identifier:** `is.workflow.actions.detect.emailaddress`  
**Returns:** `List<AnyContent>`  

Get emails from input.

```cherri
getEmails(input: Text) -> List<AnyContent>
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `input` | `Text` | Yes | - | `WFInput` | - |

---

### `getEmojiName`

**Title:** Get Name of Emoji  
**Apple Identifier:** `is.workflow.actions.getnameofemoji`  
**Returns:** `Text`  

Detects emoji in the text and returns its name.

```cherri
getEmojiName(emoji: Text) -> Text
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `emoji` | `Text` | Yes | - | `WFInput` | - |

---

### `getEventDetail`

**Title:** Get Event Detail  
**Apple Identifier:** `is.workflow.actions.properties.calendarevents`  

Get a detail of an event.

```cherri
getEventDetail(event: AnyContent, detail: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `event` | `AnyContent` | Yes | - | `WFInput` | - |
| `detail` | `Text` | Yes | - | `WFContentItemPropertyName` | Start Date, End Date, Is All Day, Calendar, Location, Has Alarms, Duration, Is Canceled, My Status, Organizer, Organizer Is Me, Attendees, Number of Attendees, URL, Title, Notes, Attachments, File Size, File Extension, Creation Date, File Path, Last Modified Date, Name |

---

### `getExternalIP`

**Title:** Get External IP  
**Apple Identifier:** `is.workflow.actions.getipaddress`  
**Returns:** `Text`  

Get the users external IP address.

```cherri
getExternalIP(type: Text) -> Text
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `type` | `Text` | No | `IPv4` | `WFIPAddressTypeOption` | IPv4, IPv6 |

---

### `getFile`

**Title:** Get File  
**Apple Identifier:** `is.workflow.actions.documentpicker.open`  

Get file from a path in the Shortcuts folder or within a `#ref` to a folder.

```cherri
getFile(path: Text, folder?: AnyContent, errorIfNotFound?: Bool = "true")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `path` | `Text` | Yes | - | `WFGetFilePath` | - |
| `folder` | `AnyContent` | No | - | `WFFile` | - |
| `errorIfNotFound` | `Bool` | No | `true` | `WFFileErrorIfNotFound` | - |

---

### `getFileDetail`

**Title:** Get File Detail  
**Apple Identifier:** `is.workflow.actions.properties.files`  

Get a detail about a file.

```cherri
getFileDetail(file: AnyContent, detail: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `file` | `AnyContent` | Yes | - | `WFInput` | - |
| `detail` | `Text` | Yes | - | `WFContentItemPropertyName` | File Size, File Extension, Creation Date, File Path, Last Modified Date, Name |

---

### `getFileLink`

**Title:** Get File Link  
**Apple Identifier:** `is.workflow.actions.file.getlink`  

Get a link for the provided file.

```cherri
getFileLink(file: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `file` | `AnyContent` | Yes | - | `WFFile` | - |

---

### `getFirstItem`

**Title:** Get First Item  
**Apple Identifier:** `is.workflow.actions.getitemfromlist`  
**Returns:** `AnyContent`  

Get first item in a list.

```cherri
getFirstItem(list: AnyContent) -> AnyContent
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `list` | `AnyContent` | Yes | - | `WFInput` | - |

---

### `getFocusMode`

**Title:** Get Focus Mode  
**Apple Identifier:** `is.workflow.actions.dnd.getfocus`  

Get the current Focus Mode.

```cherri
getFocusMode()
```

---

### `getFolderContents`

**Title:** Get Folder Contents  
**Apple Identifier:** `is.workflow.actions.file.getfoldercontents`  

Get contents of a folder.

```cherri
getFolderContents(folder: AnyContent, recursive?: Bool = "false")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `folder` | `AnyContent` | Yes | - | `WFFolder` | - |
| `recursive` | `Bool` | No | `false` | `Recursive` | - |

---

### `getGifs`

**Title:** Get GIFs from Giphy  
**Apple Identifier:** `is.workflow.actions.giphy`  

Gets a number of GIFs from Giphy for a search query.

```cherri
getGifs(query: Text, gifs?: Number = "1")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `query` | `Text` | Yes | - | `WFGiphyQuery` | - |
| `gifs` | `Number` | No | `1` | `WFGiphyLimit` | - |

---

### `getHalfwayPoint`

**Title:** Get Halfway Point  
**Apple Identifier:** `is.workflow.actions.gethalfwaypoint`  

Get the halfway point between two locations.

```cherri
getHalfwayPoint(firstLocation: AnyContent, secondLocation: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `firstLocation` | `AnyContent` | Yes | - | `WFGetHalfwayPointFirstLocation` | - |
| `secondLocation` | `AnyContent` | Yes | - | `WFGetHalfwayPointSecondLocation` | - |

---

### `getHolidayDate`

**Title:** Get Holiday Date  
**Apple Identifier:** `is.workflow.actions.date`  

Get the date of a holiday, optionally specifically for a few past or future years.

```cherri
getHolidayDate(holiday: Text, occurrenceMode?: Text = "Next Occurrence", forYear?: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `holiday` | `Text` | Yes | - | `WFDateActionMode` | April Fools' Day, Ash Wednesday, Christmas Day, Christmas Eve, Cinco de Mayo, Columbus Day, Day of the Dead, Daylight Saving Time, Daylight Saving Time End, Diwali, Earth Day, Easter Sunday, Eid al-Adha, Eid al-Fitr, Election Day, Father's Day, First Night of Ramadan, Flag Day, Good Friday, Groundhog Day, Halloween, Holi, Inauguration Day, Independence Day, Indigenous Peoples' Day, Juneteenth, Martin Luther King Jr. Day, Memorial Day, Mother's Day, New Year's Day, New Year's Eve, Palm Sunday, Presidents' Day, St. Patrick's Day, Tax Day, Thanksgiving, Valentine's Day, Veterans Day, Workers' Day |
| `occurrenceMode` | `Text` | No | `Next Occurrence` | `WFEventOccurrenceMode` | Next Occurrence, Specified Year |
| `forYear` | `Text` | No | - | `WFEventOccurrenceSpecifiedYear` | 2023, 2024, 2025, 2026, 2027 |

---

### `getHotspotPassword`

**Title:** Get Hotspot Password  
**Apple Identifier:** `is.workflow.actions.personalhotspot.password.get`  
**Returns:** `Text`  

Get the password for your personal hotspot.

```cherri
getHotspotPassword() -> Text
```

---

### `getImageDetail`

**Title:** Get Image Detail  
**Apple Identifier:** `is.workflow.actions.properties.images`  

Get a detail about an image.

```cherri
getImageDetail(image: AnyContent, detail: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `image` | `AnyContent` | Yes | - | `WFInput` | - |
| `detail` | `Text` | Yes | - | `WFContentItemPropertyName` | Album, Width, Height, Date Taken, Media Type, Photo Type, Is a Screenshot, Is a Screen Recording, Location, Duration, Frame Rate, Orientation, Camera Make, Camera Model, Metadata Dictionary, Is Favorite, File Size, File Extension, Creation Date, File Path, Last Modified Date, Name |

---

### `getImageFrames`

**Title:** Get Image Frames  
**Apple Identifier:** `is.workflow.actions.getframesfromimage`  

Get frames from a GIF.

```cherri
getImageFrames(image: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `image` | `AnyContent` | Yes | - | `WFImage` | - |

---

### `getImages`

**Title:** Get Images  
**Apple Identifier:** `is.workflow.actions.detect.images`  
**Returns:** `List<AnyContent>`  

Detect images in input

```cherri
getImages(input: AnyContent) -> List<AnyContent>
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `input` | `AnyContent` | Yes | - | `WFInput` | - |

---

### `getKeys`

**Title:** Get Keys from Dictionary  
**Apple Identifier:** `is.workflow.actions.getvalueforkey`  
**Returns:** `List<AnyContent>`  

Get only the keys from the `dictionary`.

```cherri
getKeys(dictionary: Map<Text, AnyContent>) -> List<AnyContent>
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `dictionary` | `Map<Text, AnyContent>` | Yes | - | `WFInput` | - |

---

### `getLastImport`

**Title:** Get Last Import  
**Apple Identifier:** `is.workflow.actions.getlatestphotoimport`  
```cherri
getLastImport()
```

---

### `getLastItem`

**Title:** Get Last Item  
**Apple Identifier:** `is.workflow.actions.getitemfromlist`  
**Returns:** `AnyContent`  

Get the last item in a list.

```cherri
getLastItem(list: AnyContent) -> AnyContent
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `list` | `AnyContent` | Yes | - | `WFInput` | - |

---

### `getLatestBursts`

**Title:** Get Latest Bursts  
**Apple Identifier:** `is.workflow.actions.getlatestbursts`  
```cherri
getLatestBursts(count: Number)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `count` | `Number` | Yes | - | `WFGetLatestPhotoCount` | - |

---

### `getLatestLivePhotos`

**Title:** Get Latest Live Photos  
**Apple Identifier:** `is.workflow.actions.getlatestlivephotos`  
```cherri
getLatestLivePhotos(count: Number)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `count` | `Number` | Yes | - | `WFGetLatestPhotoCount` | - |

---

### `getLatestPhotos`

**Title:** Get Latest Photos  
**Apple Identifier:** `is.workflow.actions.getlastphoto`  
```cherri
getLatestPhotos(count: Number, includeScreenshots?: Bool = "true")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `count` | `Number` | Yes | - | `WFGetLatestPhotoCount` | - |
| `includeScreenshots` | `Bool` | No | `true` | `WFGetLatestPhotosActionIncludeScreenshots` | - |

---

### `getLatestScreenshots`

**Title:** Get Latest Screenshots  
**Apple Identifier:** `is.workflow.actions.getlastscreenshot`  
```cherri
getLatestScreenshots(count: Number)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `count` | `Number` | Yes | - | `WFGetLatestPhotoCount` | - |

---

### `getLatestVideos`

**Title:** Get Latest Videos  
**Apple Identifier:** `is.workflow.actions.getlastvideo`  
```cherri
getLatestVideos(count: Number)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `count` | `Number` | Yes | - | `WFGetLatestPhotoCount` | - |

---

### `getListItem`

**Title:** Get List Item  
**Apple Identifier:** `is.workflow.actions.getitemfromlist`  
**Returns:** `AnyContent`  

Get item from `list` at `index`. Keep in mind Shortcuts starts counting indexes at 1.

```cherri
getListItem(list: AnyContent, index: Number) -> AnyContent
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `list` | `AnyContent` | Yes | - | `WFInput` | - |
| `index` | `Number` | Yes | - | `WFItemIndex` | - |

---

### `getListItems`

**Title:** Get List Items  
**Apple Identifier:** `is.workflow.actions.getitemfromlist`  
**Returns:** `List<AnyContent>`  

Get items from a `list` between two indexes. Keep in mind Shortcuts starts counting indexes at 1.

```cherri
getListItems(list: AnyContent, start: Number, end: Number) -> List<AnyContent>
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `list` | `AnyContent` | Yes | - | `WFInput` | - |
| `start` | `Number` | Yes | - | `WFItemRangeStart` | - |
| `end` | `Number` | Yes | - | `WFItemRangeEnd` | - |

---

### `getLocalIP`

**Title:** Get Local IP  
**Apple Identifier:** `is.workflow.actions.getipaddress`  
**Returns:** `Text`  

Get the users local IP address.

```cherri
getLocalIP(type: Text) -> Text
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `type` | `Text` | No | `IPv4` | `WFIPAddressTypeOption` | IPv4, IPv6 |

---

### `getLocationDetail`

**Title:** Get Location Detail  
**Apple Identifier:** `is.workflow.actions.properties.locations`  

Get a detail about a location.

```cherri
getLocationDetail(location: AnyContent, detail: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `location` | `AnyContent` | Yes | - | `WFInput` | - |
| `detail` | `Text` | Yes | - | `WFContentItemPropertyName` | Name, URL, Label, Phone Number, Region, ZIP Code, State, City, Street, Altitude, Longitude, Latitude |

---

### `getMapsLink`

**Title:** Get Maps Link  
**Apple Identifier:** `is.workflow.actions.getmapslink`  

Get a link for a location.

```cherri
getMapsLink(location: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `location` | `AnyContent` | Yes | - | `WFInput` | - |

---

### `getMatchGroup`

**Title:** Get Match Group  
**Apple Identifier:** `is.workflow.actions.text.match.getgroup`  

Get match group at `index` in `matches`.

```cherri
getMatchGroup(matches: AnyContent, index: Number)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `matches` | `AnyContent` | Yes | - | `matches` | - |
| `index` | `Number` | Yes | - | `WFGroupIndex` | - |

---

### `getMatchGroups`

**Title:** Get Match Groups  
**Apple Identifier:** `is.workflow.actions.text.match.getgroup`  

Get all groups in `matches`.

```cherri
getMatchGroups(matches: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `matches` | `AnyContent` | Yes | - | `matches` | - |

---

### `getMultitaskingMode`

**Title:** Get Multitasking Mode  
**Apple Identifier:** `com.apple.ShortcutsActions.GetMultitaskingModeAction`  
**Returns:** `Text`  

Get the current multitasking mode.

```cherri
getMultitaskingMode() -> Text
```

---

### `getMusicDetail`

**Title:** Get Music Detail  
**Apple Identifier:** `is.workflow.actions.properties.music`  

Get a detail about a song.

```cherri
getMusicDetail(music: AnyContent, detail: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `music` | `AnyContent` | Yes | - | `WFInput` | - |
| `detail` | `Text` | Yes | - | `WFContentItemPropertyName` | Title, Album, Artist, Album Artist, Genre, Composer, Date Added, Media Kind, Duration, Play Count, Track Number, Disc Number, Album Artwork, Is Explicit, Lyrics, Release Date, Comments, Is Cloud Item, Skip Count, Last Played Date, Rating, File Path, Name |

---

### `getName`

**Title:** Get Name  
**Apple Identifier:** `is.workflow.actions.getitemname`  

Get the name of an item.

```cherri
getName(item: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `item` | `AnyContent` | Yes | - | `WFInput` | - |

---

### `getNumbers`

**Title:** Get Numbers  
**Apple Identifier:** `is.workflow.actions.detect.number`  
**Returns:** `Number`  

Get numbers from input.

```cherri
getNumbers(input: AnyContent) -> Number
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `input` | `AnyContent` | Yes | - | `WFInput` | - |

---

### `getObjectOfClass`

**Title:** Get Object of Class  
**Apple Identifier:** `is.workflow.actions.getclassaction`  

Get the object of `class` from a variable.

```cherri
getObjectOfClass(class: Text, from: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `class` | `Text` | Yes | - | `Class` | - |
| `from` | `AnyContent` | Yes | - | `Input` | - |

---

### `getOnScreenContent`

**Title:** Get On-Screen Content  
**Apple Identifier:** `is.workflow.actions.getonscreencontent`  

Get the content currently on-screen.

```cherri
getOnScreenContent()
```

---

### `getOrientation`

**Title:** Get Orientation  
**Apple Identifier:** `com.apple.ShortcutsActions.GetOrientationAction`  
**Returns:** `Text`  

Get the current orientation of the device.

```cherri
getOrientation() -> Text
```

---

### `getParentDirectory`

**Title:** Get Parent Directory  
**Apple Identifier:** `is.workflow.actions.getparentdirectory`  

Get the parent directory of the input directory.

```cherri
getParentDirectory(input: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `input` | `AnyContent` | Yes | - | `WFInput` | - |

---

### `getPhoneNumbers`

**Title:** Get Phone Numbers  
**Apple Identifier:** `is.workflow.actions.detect.phonenumber`  
**Returns:** `List<AnyContent>`  

Get phone numbers from input.

```cherri
getPhoneNumbers(input: AnyContent) -> List<AnyContent>
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `input` | `AnyContent` | Yes | - | `WFInput` | - |

---

### `getPlaylistSongs`

**Title:** Get Playlist  
**Apple Identifier:** `is.workflow.actions.get.playlist`  
**Returns:** `List<AnyContent>`  

Gets songs from a playlist

```cherri
getPlaylistSongs(playlistName: AnyContent) -> List<AnyContent>
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `playlistName` | `AnyContent` | Yes | - | `WFPlaylistName` | - |

---

### `getPodcastDetail`

**Title:** Get Podcast Detail  
**Apple Identifier:** `is.workflow.actions.properties.podcastshow`  
```cherri
getPodcastDetail(podcast: AnyContent, detail: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `podcast` | `AnyContent` | Yes | - | `WFInput` | - |
| `detail` | `Text` | Yes | - | `WFContentItemPropertyName` | Feed URL, Genre, Episode Count, Artist, Store ID, Store URL, Artwork, Artwork URL, Name |

---

### `getPodcasts`

**Title:** Get Podcasts  
**Apple Identifier:** `is.workflow.actions.getpodcastsfromlibrary`  

Get users podcasts.

```cherri
getPodcasts()
```

---

### `getRSS`

**Title:** Get RSS  
**Apple Identifier:** `is.workflow.actions.rss`  

Get RSS feed contents at URL. Limited by number of items.

```cherri
getRSS(items: Number, url: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `items` | `Number` | Yes | - | `WFRSSItemQuantity` | - |
| `url` | `Text` | Yes | - | `WFRSSFeedURL` | - |

---

### `getRSSFeeds`

**Title:** Get RSS Feeds  
**Apple Identifier:** `is.workflow.actions.rss.extract`  

Get feeds from multiple URLs.

```cherri
getRSSFeeds(urls: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `urls` | `Text` | Yes | - | `WFURLs` | - |

---

### `getRandomItem`

**Title:** Get Random Item  
**Apple Identifier:** `is.workflow.actions.getitemfromlist`  
**Returns:** `AnyContent`  

Get random item from list.

```cherri
getRandomItem(list: AnyContent) -> AnyContent
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `list` | `AnyContent` | Yes | - | `WFInput` | - |

---

### `getRichTextFromHTML`

**Title:** Make Rich Text from HTML  
**Apple Identifier:** `is.workflow.actions.getrichtextfromhtml`  
**Returns:** `Text`  
```cherri
getRichTextFromHTML(html: Text) -> Text
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `html` | `Text` | Yes | - | `WFHTML` | - |

---

### `getRichTextFromMarkdown`

**Title:** Get Rich Text from Markdown  
**Apple Identifier:** `is.workflow.actions.getrichtextfrommarkdown`  
**Returns:** `Text`  
```cherri
getRichTextFromMarkdown(markdown: Text) -> Text
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `markdown` | `Text` | Yes | - | `WFInput` | - |

---

### `getSelectedFiles`

**Title:** Get Selected  
**Apple Identifier:** `is.workflow.actions.finder.getselectedfiles`  
```cherri
getSelectedFiles()
```

---

### `getShazamDetail`

**Title:** Get Shazam Detail  
**Apple Identifier:** `is.workflow.actions.properties.shazam`  

Get a detail about a Shazam result.

```cherri
getShazamDetail(input: AnyContent, detail: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `input` | `AnyContent` | Yes | - | `WFInput` | - |
| `detail` | `Text` | Yes | - | `WFContentItemPropertyName` | Apple Music ID, Artist, Title, Is Explicit, Lyrics Snippet, Lyric Snippet Synced, Artwork, Video URL, Shazam URL, Apple Music URL, Name |

---

### `getShortcutDetail`

**Title:** Get Shortcut Detail  
**Apple Identifier:** `is.workflow.actions.properties.workflow`  

Get a detail about a Shortcut.

```cherri
getShortcutDetail(shortcut: AnyContent, detail: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `shortcut` | `AnyContent` | Yes | - | `WFInput` | - |
| `detail` | `Text` | Yes | - | `WFContentItemPropertyName` | Folder, Icon, Action Count, File Size, File Extension Creation Date, File Path, Last Modified Date, Name |

---

### `getShortcuts`

**Title:** Get Shortcuts  
**Apple Identifier:** `is.workflow.actions.getmyworkflows`  
**Returns:** `List<AnyContent>`  

Get all of the Shortcuts on the device.

```cherri
getShortcuts() -> List<AnyContent>
```

---

### `getStoredValue`

**Title:** Get Stored Content  
**Apple Identifier:** `is.workflow.actions.getstoredcontent`  

Get previously stored content, optionally globally or specific to the Shortcut.

```cherri
getStoredValue(key: Text, global?: Bool = "false")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `key` | `Text` | Yes | - | `WFStoredContentKey` | - |
| `global` | `Bool` | No | `false` | `WFStoredContentGlobalValue` | - |

---

### `getText`

**Title:** Get Text  
**Apple Identifier:** `is.workflow.actions.detect.text`  
**Returns:** `Text`  

Get text from input.

```cherri
getText(input: AnyContent) -> Text
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `input` | `AnyContent` | Yes | - | `WFInput` | - |

---

### `getTextFromImage`

**Title:** Get Text from Image  
**Apple Identifier:** `is.workflow.actions.extracttextfromimage`  
**Returns:** `Text`  
```cherri
getTextFromImage(image: AnyContent) -> Text
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `image` | `AnyContent` | Yes | - | `WFImage` | - |

---

### `getTimeBetweenDates`

**Title:** Get Time Between Dates  
**Apple Identifier:** `is.workflow.actions.gettimebetweendates`  
**Returns:** `Number`  

Get the time between two dates.

```cherri
getTimeBetweenDates(startDate: AnyContent, endDate?: AnyContent, unit?: Text) -> Number
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `startDate` | `AnyContent` | No | - | `WFTimeUntilFromDate` | - |
| `endDate` | `AnyContent` | No | - | `WFInput` | - |
| `unit` | `Text` | No | - | `WFTimeUntilUnit` | Minutes, Hours, Days, Weeks, Months, Years, Seconds |

---

### `getTravelTime`

**Title:** Get Travel Time  
**Apple Identifier:** `is.workflow.actions.gettraveltime`  

Get travel time to a destination.

```cherri
getTravelTime(destination: AnyContent, customLocation?: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `destination` | `AnyContent` | No | - | `WFDestination` | - |
| `customLocation` | `AnyContent` | No | - | `WFGetDirectionsCustomLocation` | - |

---

### `getType`

**Title:** Get Type  
**Apple Identifier:** `is.workflow.actions.gettypeaction`  
**Returns:** `Text`  

Get the type of input.

```cherri
getType(input: AnyContent, fileType?: Text) -> Text
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `input` | `AnyContent` | Yes | - | `WFInput` | - |
| `fileType` | `Text` | No | - | `WFFileType` | - |

---

### `getURLDetail`

**Title:** Get URL Detail  
**Apple Identifier:** `is.workflow.actions.geturlcomponent`  

Get a detail about a URL.

```cherri
getURLDetail(url: Text, detail: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `url` | `Text` | Yes | - | `WFURL` | - |
| `detail` | `Text` | Yes | - | `WFURLComponent` | Scheme, User, Password, Host, Port, Path, Query, Fragment |

---

### `getURLHeaders`

**Title:** Get URL Headers  
**Apple Identifier:** `is.workflow.actions.url.getheaders`  

Get the headers from a URL.

```cherri
getURLHeaders(url: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `url` | `Text` | Yes | - | `WFInput` | - |

---

### `getURLs`

**Title:** Get URLs  
**Apple Identifier:** `is.workflow.actions.detect.link`  
**Returns:** `List<AnyContent>`  

Get URLs from `input`.

```cherri
getURLs(input: Text) -> List<AnyContent>
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `input` | `Text` | Yes | - | `WFInput` | - |

---

### `getUpcomingEvents`

**Title:** Get Upcoming Events  
**Apple Identifier:** `is.workflow.actions.getupcomingevents`  
**Returns:** `AnyContent`  

Get upcoming events from the calendars on this device.

```cherri
getUpcomingEvents(count: Number, dateSpecifier?: Text, specifiedDate?: Text) -> AnyContent
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `count` | `Number` | No | - | `WFGetUpcomingItemCount` | - |
| `dateSpecifier` | `Text` | No | - | `WFDateSpecifier` | - |
| `specifiedDate` | `Text` | No | - | `WFSpecifiedDate` | - |

---

### `getValue`

**Title:** Get Value from Dictionary  
**Apple Identifier:** `is.workflow.actions.getvalueforkey`  
**Returns:** `AnyContent`  

For constants only, otherwise `dictionary['key']` syntax should be used.

```cherri
getValue(dictionary: Map<Text, AnyContent>, key: Text) -> AnyContent
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `dictionary` | `Map<Text, AnyContent>` | Yes | - | `WFInput` | - |
| `key` | `Text` | Yes | - | `WFDictionaryKey` | - |

---

### `getValues`

**Title:** Get Values from Dictionary  
**Apple Identifier:** `is.workflow.actions.getvalueforkey`  
**Returns:** `List<AnyContent>`  

Get only the values from the `dictionary`.

```cherri
getValues(dictionary: Map<Text, AnyContent>) -> List<AnyContent>
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `dictionary` | `Map<Text, AnyContent>` | Yes | - | `WFInput` | - |

---

### `getVariable`

**Title:** Get Variable  
**Apple Identifier:** `is.workflow.actions.getvariable`  
**Returns:** `AnyContent`  

Retrieves the value of a variable.

```cherri
getVariable(variable: AnyContent) -> AnyContent
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `variable` | `AnyContent` | Yes | - | `WFVariable` | - |

---

### `getWallpaper`

**Title:** Get Wallpaper  
**Apple Identifier:** `is.workflow.actions.posters.get`  

Get current device wallpaper.

```cherri
getWallpaper()
```

---

### `getWeatherDetail`

**Title:** Get Weather Detail  
**Apple Identifier:** `is.workflow.actions.properties.weather.conditions`  

Get a detail about a weather forecast.

```cherri
getWeatherDetail(weather: AnyContent, detail: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `weather` | `AnyContent` | Yes | - | `WFInput` | - |
| `detail` | `Text` | Yes | - | `WFContentItemPropertyName` | Name, Air Pollutants, Air Quality Category, Air Quality Index, Sunset Time, Sunrise Time, UV Index, Wind Direction, Wind Speed, Precipitation Chance, Precipitation Amount, Pressure, Humidity, Dewpoint, Visibility, Condition, Feels Like, Low, High, Temperature, Location, Date |

---

### `getWeatherForecast`

**Title:** Get Weather Forecast  
**Apple Identifier:** `is.workflow.actions.weather.forecast`  

Get various types of weather forecast for a location.

```cherri
getWeatherForecast(type: Text, location?: Text = "Current Location")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `type` | `Text` | No | `Daily` | `WFWeatherForecastType` | Daily, Hourly |
| `location` | `Text` | No | `Current Location` | `WFInput` | - |

---

### `getWebPageDetail`

**Title:** Get Webpage Detail  
**Apple Identifier:** `is.workflow.actions.properties.safariwebpage`  

Get a detail about a provided webpage.

```cherri
getWebPageDetail(webpage: AnyContent, detail: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `webpage` | `AnyContent` | Yes | - | `WFInput` | - |
| `detail` | `Text` | Yes | - | `WFContentItemPropertyName` | Page Contents, Page Selection, Page URL, Name |

---

### `getWebpageContents`

**Title:** Get Webpage Contents  
**Apple Identifier:** `is.workflow.actions.getwebpagecontents`  

Get contents of Webpage from Safari.

```cherri
getWebpageContents(url: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `url` | `Text` | Yes | - | `WFInput` | - |

---

### `getWifiDetail`

**Title:** Get WiFi Detail  
**Apple Identifier:** `is.workflow.actions.getwifi`  

Get a detail of the current WiFi network.

```cherri
getWifiDetail(detail: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `detail` | `Text` | Yes | - | `WFWiFiDetail` | Network Name, BSSID, Wi-Fi Standard, RX Rate, TX Rate, RSSI, Noise, Channel Number, Hardware MAC Address |

---

### `hash`

**Title:** Hash  
**Apple Identifier:** `is.workflow.actions.hash`  
**Returns:** `Text`  

Generate a hash of type using input.

```cherri
hash(input: AnyContent, type?: Text = "MD5") -> Text
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `input` | `AnyContent` | Yes | - | `WFInput` | - |
| `type` | `Text` | No | `MD5` | `WFHashType` | MD5, SHA1, SHA256, SHA512 |

---

### `isCharging`

**Title:** Is Charging  
**Apple Identifier:** `is.workflow.actions.getbatterylevel`  
**Returns:** `Bool`  

Determines if the device is currently charging.

```cherri
isCharging() -> Bool
```

---

### `isOnline`

**Title:** Is Online  
**Apple Identifier:** `is.workflow.actions.getipaddress`  

Determine if the user is online. Alias for get Get External IP.

```cherri
isOnline()
```

---

### `jsonRequest`

**Title:** JSON Request  
**Apple Identifier:** `is.workflow.actions.downloadurl`  

Send a `method` JSON request to `url` with `body` and optional `headers.

```cherri
jsonRequest(url: Text, method?: Text, body?: Map<Text, AnyContent>, headers?: Map<Text, AnyContent>)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `url` | `Text` | Yes | - | `WFURL` | - |
| `method` | `Text` | No | - | `WFHTTPMethod` | POST, PUT, PATCH, DELETE |
| `body` | `Map<Text, AnyContent>` | No | - | `WFJSONValues` | - |
| `headers` | `Map<Text, AnyContent>` | No | - | `WFHTTPHeaders` | - |

---

### `lightMode`

**Title:** Set Appearance to Light  
**Apple Identifier:** `is.workflow.actions.appearance`  
```cherri
lightMode()
```

---

### `listen`

**Title:** Dictation  
**Apple Identifier:** `is.workflow.actions.dictatetext`  
**Returns:** `Text`  

Transcribes user recorded audio to text, optionally in another language.

```cherri
listen(stopListening: Text, language?: Text) -> Text
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `stopListening` | `Text` | No | `After Pause` | `WFDictateTextStopListening` | After Pause, After Short Pause, On Tap |
| `language` | `Text` | No | - | `WFSpeechLanguage` | ar-AE, zh-CN, zh-TW, nl-NL, en-GB, en-US, fr-FR, de-DE, id-ID, it-IT, jp-JP, ko-KR, pl-PL, pt-BR, ru-RU, es-ES, th-TH, tr-TR, vn-VN |

---

### `location`

**Title:** Location  
**Apple Identifier:** `is.workflow.actions.location`  

Create a location value.

```cherri
location(location: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `location` | `Text` | Yes | - | `WFLocation` | - |

---

### `lockScreen`

**Title:** Lock Screen  
**Apple Identifier:** `is.workflow.actions.lockscreen`  

Lock the device screen.

```cherri
lockScreen()
```

---

### `lowercase`

**Title:** Lowercase  
**Apple Identifier:** `is.workflow.actions.text.changecase`  
**Returns:** `Text`  

Transforms the text to all lowercase.

```cherri
lowercase(text: Text) -> Text
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `text` | `Text` | Yes | - | `text` | - |

---

### `magnifierDescribe`

**Title:** Describe This  
**Apple Identifier:** `com.apple.Magnifier.DescribeThisIntent`  
```cherri
magnifierDescribe()
```

---

### `magnifierPointAndSpeak`

**Title:** Start Point & Speak  
**Apple Identifier:** `com.apple.Magnifier.PointAndSpeakIntent`  
```cherri
magnifierPointAndSpeak()
```

---

### `magnifierReader`

**Title:** Open Reader  
**Apple Identifier:** `com.apple.Magnifier.ReaderModeIntent`  
```cherri
magnifierReader()
```

---

### `makeArchive`

**Title:** Make Archive  
**Apple Identifier:** `is.workflow.actions.makezip`  

Create an archive of `files` named `name` in `format`.

```cherri
makeArchive(files: AnyContent, format?: Text = ".zip", name?: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `files` | `AnyContent` | Yes | - | `WFInput` | - |
| `format` | `Text` | No | `.zip` | `WFArchiveFormat` | .zip, .tar.gz, .tar.bz2, .tar.xz, .tar, .gz, .cpio, .iso |
| `name` | `Text` | No | - | `WFZIPName` | - |

---

### `makeDiskImage`

**Title:** Make Disk Image  
**Apple Identifier:** `is.workflow.actions.makediskimage`  

Create a new disk image.

```cherri
makeDiskImage(name: Text, contents: AnyContent, encrypt?: Bool = "false")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `name` | `Text` | Yes | - | `VolumeName` | - |
| `contents` | `AnyContent` | Yes | - | `WFInput` | - |
| `encrypt` | `Bool` | No | `false` | `EncryptImage` | - |

---

### `makeGIF`

**Title:** Make GIF  
**Apple Identifier:** `is.workflow.actions.makegif`  
```cherri
makeGIF(input: AnyContent, delay?: Text = "0.3", loops?: Number, width?: Text, height?: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `input` | `AnyContent` | Yes | - | `WFInput` | - |
| `delay` | `Text` | No | `0.3` | `WFMakeGIFActionDelayTime` | - |
| `loops` | `Number` | No | - | `WFMakeGIFActionLoopCount` | - |
| `width` | `Text` | No | - | `WFMakeGIFActionManualSizeWidth` | - |
| `height` | `Text` | No | - | `WFMakeGIFActionManualSizeHeight` | - |

---

### `makeHTML`

**Title:** Make HTML  
**Apple Identifier:** `is.workflow.actions.gethtmlfromrichtext`  
**Returns:** `Text`  

Make HTML from Rich Text.

```cherri
makeHTML(input: Text, makeFullDocument?: Bool = "false") -> Text
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `input` | `Text` | Yes | - | `WFInput` | - |
| `makeFullDocument` | `Bool` | No | `false` | `WFMakeFullDocument` | - |

---

### `makeImageFromPDFPage`

**Title:** Make Image from PDF Page  
**Apple Identifier:** `is.workflow.actions.makeimagefrompdfpage`  

Returns an image of the provided PDF.

```cherri
makeImageFromPDFPage(pdf: AnyContent, colorSpace?: Text = "RGB", pageResolution?: Text = "300")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `pdf` | `AnyContent` | Yes | - | `WFInput` | - |
| `colorSpace` | `Text` | No | `RGB` | `WFMakeImageFromPDFPageColorspace` | RGB, Gray |
| `pageResolution` | `Text` | No | `300` | `WFMakeImageFromPDFPageResolution` | - |

---

### `makeImageFromRichText`

**Title:** Make Image from Rich Text  
**Apple Identifier:** `is.workflow.actions.makeimagefromrichtext`  
```cherri
makeImageFromRichText(pdf: AnyContent, width: Text, height: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `pdf` | `AnyContent` | Yes | - | `WFInput` | - |
| `width` | `Text` | Yes | - | `WFWidth` | - |
| `height` | `Text` | Yes | - | `WFHeight` | - |

---

### `makeMarkdown`

**Title:** Make Markdown  
**Apple Identifier:** `is.workflow.actions.getmarkdownfromrichtext`  
**Returns:** `Text`  

Make Markdown from Rich Text.

```cherri
makeMarkdown(richText: Text) -> Text
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `richText` | `Text` | Yes | - | `WFInput` | - |

---

### `makePDF`

**Title:** Make PDF  
**Apple Identifier:** `is.workflow.actions.makepdf`  

Make a PDF file using the provided input.

```cherri
makePDF(input: AnyContent, includeMargin?: Bool = "false", mergeBehavior?: Text = "Append")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `input` | `AnyContent` | Yes | - | `WFInput` | - |
| `includeMargin` | `Bool` | No | `false` | `WFPDFIncludeMargin` | - |
| `mergeBehavior` | `Text` | No | `Append` | `WFPDFDocumentMergeBehavior` | Append, Shuffle |

---

### `makeQRCode`

**Title:** Make QR Code  
**Apple Identifier:** `is.workflow.actions.generatebarcode`  
```cherri
makeQRCode(input: Text, errorCorrection?: Text = "Medium", foregroundColor?: Unknown = "[{float 0} {float 0} {float 0} {float 1}]", backgroundColor?: Unknown = "[{float 2} {float 2} {float 2} {float 1}]")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `input` | `Text` | Yes | - | `WFText` | - |
| `errorCorrection` | `Text` | No | `Medium` | `WFQRErrorCorrectionLevel` | Low, Medium, Quartile, High |
| `foregroundColor` | `Unknown` | No | `[{float 0} {float 0} {float 0} {float 1}]` | `WFQRForegroundColor` | - |
| `backgroundColor` | `Unknown` | No | `[{float 2} {float 2} {float 2} {float 1}]` | `WFQRBackgroundColor` | - |

---

### `makeShortcut`

**Title:** Make Shortcut  
**Apple Identifier:** `com.apple.shortcuts.CreateWorkflowAction`  

Create a shortcut.

```cherri
makeShortcut(name: Text, open?: Bool = "true")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `name` | `Text` | Yes | - | `name` | - |
| `open` | `Bool` | No | `true` | `OpenWhenRun` | - |

---

### `makeSizedDiskImage`

**Title:** Make Sized Disk Image  
**Apple Identifier:** `is.workflow.actions.makediskimage`  

Create a new disk image of a specific size.

```cherri
makeSizedDiskImage(name: Text, contents: AnyContent, diskSize?: Unknown = "[{number 1} {text GB}]", encrypt?: Bool = "false")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `name` | `Text` | Yes | - | `VolumeName` | - |
| `contents` | `AnyContent` | Yes | - | `WFInput` | - |
| `diskSize` | `Unknown` | No | `[{number 1} {text GB}]` | `ImageSize` | bytes, KB, MB, GB, TB, PB, EB, ZB, Y |
| `encrypt` | `Bool` | No | `false` | `EncryptImage` | - |

---

### `makeSpokenAudio`

**Title:** Make Spoken Audio  
**Apple Identifier:** `is.workflow.actions.makespokenaudiofromtext`  

Creates custom spoken audio from text with controls for rate and pitch.

```cherri
makeSpokenAudio(text: Text, rate?: Number, pitch?: Number)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `text` | `Text` | Yes | - | `WFInput` | - |
| `rate` | `Number` | No | - | `WFSpeakTextRate` | - |
| `pitch` | `Number` | No | - | `WFSpeakTextPitch` | - |

---

### `makeVideoFromGIF`

**Title:** Make Video From GIF  
**Apple Identifier:** `is.workflow.actions.makevideofromgif`  
```cherri
makeVideoFromGIF(gif: AnyContent, loops?: Number = "1")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `gif` | `AnyContent` | Yes | - | `WFInputGIF` | - |
| `loops` | `Number` | No | `1` | `WFMakeVideoFromGIFActionLoopCount` | - |

---

### `markup`

**Title:** Markup  
**Apple Identifier:** `is.workflow.actions.avairyeditphoto`  

Opens document in a markup editor.

```cherri
markup(document: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `document` | `AnyContent` | Yes | - | `WFDocument` | - |

---

### `maskImage`

**Title:** Mask Image  
**Apple Identifier:** `is.workflow.actions.image.mask`  

Mask an image with another image.

```cherri
maskImage(image: AnyContent, type: Text, radius?: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `image` | `AnyContent` | Yes | - | `WFInput` | - |
| `type` | `Text` | Yes | - | `WFMaskType` | Rounded Rectangle, Ellipse, Icon |
| `radius` | `Text` | No | - | `WFMaskCornerRadius` | - |

---

### `matchText`

**Title:** Match Text  
**Apple Identifier:** `is.workflow.actions.text.match`  

Use regular expressions to match text. Use raw text (single quotes) to match using an regular expression with braces to avoid conflicts with the regular expression syntax.

```cherri
matchText(regexPattern: Text, text: Text, caseSensitive?: Bool = "true")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `regexPattern` | `Text` | Yes | - | `WFMatchTextPattern` | - |
| `text` | `Text` | Yes | - | `text` | - |
| `caseSensitive` | `Bool` | No | `true` | `WFMatchTextCaseSensitive` | - |

---

### `moveWindow`

**Title:** Move Window  
**Apple Identifier:** `is.workflow.actions.movewindow`  

Move a window to a defined position.

```cherri
moveWindow(window: AnyContent, position: Text, bringToFront?: Bool = "true")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `window` | `AnyContent` | Yes | - | `WFWindow` | - |
| `position` | `Text` | Yes | - | `WFPosition` | Top Left, Top Center, Top Right, Middle Left, Center, Middle Right, Bottom Left, Bottom Center, Bottom Right, Coordinates |
| `bringToFront` | `Bool` | No | `true` | `WFBringToFront` | - |

---

### `mustOutput`

**Title:** Must Output  
**Apple Identifier:** `is.workflow.actions.output`  

Stop and output `output`. Respond with response if there is nowhere to output.

```cherri
mustOutput(output: Text, response: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `output` | `Text` | Yes | - | `WFOutput` | - |
| `response` | `Text` | Yes | - | `WFResponse` | - |

---

### `nothing`

**Title:** Nothing  
**Apple Identifier:** `is.workflow.actions.nothing`  

Clear the current output.

```cherri
nothing()
```

---

### `number`

**Title:** Number  
**Apple Identifier:** `is.workflow.actions.number`  
**Returns:** `Number`  

Create a number value.

```cherri
number(number: AnyContent) -> Number
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `number` | `AnyContent` | Yes | - | `WFNumberActionNumber` | - |

---

### `openFile`

**Title:** Open File  
**Apple Identifier:** `is.workflow.actions.openin`  

Open a file.

```cherri
openFile(file: AnyContent, askWhenRun?: Bool = "false")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `file` | `AnyContent` | Yes | - | `WFInput` | - |
| `askWhenRun` | `Bool` | No | `false` | `WFOpenInAskWhenRun` | - |

---

### `openInMaps`

**Title:** Open in Maps  
**Apple Identifier:** `is.workflow.actions.searchmaps`  

Open a location in maps.

```cherri
openInMaps(location: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `location` | `AnyContent` | Yes | - | `WFInput` | - |

---

### `openNote`

**Title:** Open Note  
**Apple Identifier:** `is.workflow.actions.shownote`  

Open note in the Notes app.

```cherri
openNote(note: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `note` | `AnyContent` | Yes | - | `WFInput` | - |

---

### `openRemindersList`

**Title:** Open Reminders List  
**Apple Identifier:** `is.workflow.actions.showlist`  
```cherri
openRemindersList(list: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `list` | `AnyContent` | Yes | - | `WFList` | - |

---

### `openURL`

**Title:** Open URL  
**Apple Identifier:** `is.workflow.actions.openurl`  

Open URL in default browser.

```cherri
openURL(url: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `url` | `Text` | Yes | - | `WFInput` | - |

---

### `openXCallbackURL`

**Title:** Open X-Callback URL  
**Apple Identifier:** `is.workflow.actions.openxcallbackurl`  
```cherri
openXCallbackURL(url: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `url` | `Text` | Yes | - | `WFXCallbackURL` | - |

---

### `optimizePDF`

**Title:** Optimize PDF  
**Apple Identifier:** `is.workflow.actions.compresspdf`  

Returns a compressed version of the PDF.

```cherri
optimizePDF(pdfFile: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `pdfFile` | `AnyContent` | Yes | - | `WFInput` | - |

---

### `output`

**Title:** Stop and Output  
**Apple Identifier:** `is.workflow.actions.output`  

Stop and output `output`. Do nothing if there is nowhere to output.

```cherri
output(output: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `output` | `Text` | Yes | - | `WFOutput` | - |

---

### `outputOrClipboard`

**Title:** Output or Copy to Clipboard  
**Apple Identifier:** `is.workflow.actions.output`  

Stop and output `output`. Copy to the clipboard if there is nowhere to output.

```cherri
outputOrClipboard(output: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `output` | `Text` | Yes | - | `WFOutput` | - |

---

### `overlayImage`

**Title:** Overlay Image  
**Apple Identifier:** `is.workflow.actions.overlayimageonimage`  

Overlay an image on top of another image.

```cherri
overlayImage(image: AnyContent, overlayImage: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `image` | `AnyContent` | Yes | - | `WFInput` | - |
| `overlayImage` | `AnyContent` | Yes | - | `WFImage` | - |

---

### `pause`

**Title:** Pause  
**Apple Identifier:** `is.workflow.actions.pausemusic`  

Press pause.

```cherri
pause()
```

---

### `play`

**Title:** Play  
**Apple Identifier:** `is.workflow.actions.pausemusic`  

Press play.

```cherri
play()
```

---

### `playLater`

**Title:** Play Next  
**Apple Identifier:** `is.workflow.actions.addmusictoupnext`  

Add music to the end of the queue.

```cherri
playLater(music: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `music` | `AnyContent` | Yes | - | `WFMusic` | - |

---

### `playMusic`

**Title:** Play Music  
**Apple Identifier:** `is.workflow.actions.playmusic`  

Set music to play with modes for shuffle and repeat.

```cherri
playMusic(music: AnyContent, shuffle?: Text, repeat?: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `music` | `AnyContent` | Yes | - | `WFMediaItems` | - |
| `shuffle` | `Text` | No | - | `WFPlayMusicActionShuffle` | Off, Songs |
| `repeat` | `Text` | No | - | `WFPlayMusicActionRepeat` | None, One, All |

---

### `playNext`

**Title:** Play Next  
**Apple Identifier:** `is.workflow.actions.addmusictoupnext`  

Add music to play next in the queue.

```cherri
playNext(music: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `music` | `AnyContent` | Yes | - | `WFMusic` | - |

---

### `playPodcast`

**Title:** Play Podcast  
**Apple Identifier:** `is.workflow.actions.playpodcast`  
```cherri
playPodcast(podcast: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `podcast` | `AnyContent` | Yes | - | `WFPodcastShow` | - |

---

### `playSound`

**Title:** Play Audio  
**Apple Identifier:** `is.workflow.actions.playsound`  

Play a sound.

```cherri
playSound(input: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `input` | `AnyContent` | Yes | - | `WFInput` | - |

---

### `prependToFile`

**Title:** Prepend File  
**Apple Identifier:** `is.workflow.actions.file.append`  

Prepend text to a file.

```cherri
prependToFile(filePath: Text, text: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `filePath` | `Text` | Yes | - | `WFFilePath` | - |
| `text` | `AnyContent` | Yes | - | `WFInput` | - |

---

### `print`

**Title:** Print  
**Apple Identifier:** `is.workflow.actions.print`  

Print input to a printer.

```cherri
print(input: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `input` | `AnyContent` | Yes | - | `WFInput` | - |

---

### `quicklook`

**Title:** Quick Look  
**Apple Identifier:** `is.workflow.actions.previewdocument`  

Preview `input` in Quick Look.

```cherri
quicklook(input: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `input` | `AnyContent` | Yes | - | `WFInput` | - |

---

### `randomNumber`

**Title:** Random Number  
**Apple Identifier:** `is.workflow.actions.number.random`  
**Returns:** `Number`  

Returns a random number between `min` and `max`.

```cherri
randomNumber(min: Number, max: Number) -> Number
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `min` | `Number` | Yes | - | `WFRandomNumberMinimum` | - |
| `max` | `Number` | Yes | - | `WFRandomNumberMaximum` | - |

---

### `rawAction`

**Apple Identifier:** `is.workflow.actions.rawaction`  
```cherri
rawAction(identifier: Text, parameters?: Map<Text, AnyContent>)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `identifier` | `Text` | Yes | - | `` | - |
| `parameters` | `Map<Text, AnyContent>` | No | - | `` | - |

---

### `reboot`

**Title:** Reboot  
**Apple Identifier:** `is.workflow.actions.reboot`  

Power off the device, then power it on again.

```cherri
reboot()
```

---

### `recordAudio`

**Title:** Record Audio  
**Apple Identifier:** `is.workflow.actions.recordaudio`  

Prompt the user to record audio.

```cherri
recordAudio(quality: Text, start?: Text = "On Tap")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `quality` | `Text` | No | `Normal` | `WFRecordingCompression` | Normal, Very High |
| `start` | `Text` | No | `On Tap` | `WFRecordingStart` | On Tap, Immediately |

---

### `removeBackground`

**Title:** Remove Image Background  
**Apple Identifier:** `is.workflow.actions.image.removebackground`  
```cherri
removeBackground(image: AnyContent, crop?: Bool = "false")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `image` | `AnyContent` | Yes | - | `WFInput` | - |
| `crop` | `Bool` | No | `false` | `WFCropToBounds` | - |

---

### `removeContactDetail`

**Title:** Remove Contact Detail  
**Apple Identifier:** `is.workflow.actions.setters.contacts`  

Remove a detail from a contact.

```cherri
removeContactDetail(contact: AnyContent, detail: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `contact` | `AnyContent` | Yes | - | `WFInput` | - |
| `detail` | `Text` | Yes | - | `WFContentItemPropertyName` | First Name, Middle Name, Last Name, Birthday, Prefix, Suffix, Nickname, Phonetic First Name, Phonetic Last Name, Phonetic Middle Name, Company, Job Title, Department, File Extension, Creation Date, File Path, Last Modified Date, Name, Random |

---

### `removeEvents`

**Title:** Remove Events  
**Apple Identifier:** `is.workflow.actions.removeevents`  

Remove an event.

```cherri
removeEvents(events: AnyContent, includeFutureEvents?: Bool = "false")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `events` | `AnyContent` | Yes | - | `WFInputEvents` | - |
| `includeFutureEvents` | `Bool` | No | `false` | `WFCalendarIncludeFutureEvents` | - |

---

### `removeFromAlbum`

**Title:** Remove Photo from Album  
**Apple Identifier:** `is.workflow.actions.removefromalbum`  
```cherri
removeFromAlbum(photo: AnyContent, album: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `photo` | `AnyContent` | Yes | - | `WFInput` | - |
| `album` | `Text` | Yes | - | `WFRemoveAlbumSelectedGroup` | - |

---

### `removeReminders`

**Title:** Remove Reminders  
**Apple Identifier:** `is.workflow.actions.removereminders`  
```cherri
removeReminders(reminders: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `reminders` | `AnyContent` | Yes | - | `WFInputReminders` | - |

---

### `removeWeatherLocation`

**Title:** Remove Location from List  
**Apple Identifier:** `com.apple.weather.RemoveSavedLocationIntent`  

Remove a location from the Weather app.

```cherri
removeWeatherLocation(location: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `location` | `AnyContent` | Yes | - | `entities` | - |

---

### `rename`

**Title:** Rename File  
**Apple Identifier:** `is.workflow.actions.file.rename`  
```cherri
rename(file: AnyContent, newName: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `file` | `AnyContent` | Yes | - | `WFFile` | - |
| `newName` | `Text` | Yes | - | `WFNewFilename` | - |

---

### `renameAlbum`

**Title:** Rename Album  
**Apple Identifier:** `is.workflow.actions.renamealbum`  
```cherri
renameAlbum(album: AnyContent, newTitle: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `album` | `AnyContent` | Yes | - | `album` | - |
| `newTitle` | `Text` | Yes | - | `title` | - |

---

### `replaceText`

**Title:** Replace Text  
**Apple Identifier:** `is.workflow.actions.text.replace`  
**Returns:** `Text`  

Replace `find` in `subject`, optionally using a regular expression or case insensitive search. Use raw text (single quotes) to match using an regular expression with braces to avoid conflicts with the regular expression syntax.

```cherri
replaceText(find: Text, replacement: Text, subject: Text, caseSensitive?: Bool = "true", regExp?: Bool = "false") -> Text
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `find` | `Text` | Yes | - | `WFReplaceTextFind` | - |
| `replacement` | `Text` | Yes | - | `WFReplaceTextReplace` | - |
| `subject` | `Text` | Yes | - | `WFInput` | - |
| `caseSensitive` | `Bool` | No | `true` | `WFReplaceTextCaseSensitive` | - |
| `regExp` | `Bool` | No | `false` | `WFReplaceTextRegularExpression` | - |

---

### `resizeImage`

**Title:** Resize Image  
**Apple Identifier:** `is.workflow.actions.image.resize`  
**Returns:** `Image`  
```cherri
resizeImage(image: Image, width: Number, height?: Number) -> Image
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `image` | `Image` | Yes | - | `WFImage` | - |
| `width` | `Number` | Yes | - | `WFImageResizeWidth` | - |
| `height` | `Number` | No | - | `WFImageResizeHeight` | - |

---

### `resizeImageByLongestEdge`

**Title:** Resize Image By Longest Edge  
**Apple Identifier:** `is.workflow.actions.image.resize`  
```cherri
resizeImageByLongestEdge(image: AnyContent, length: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `image` | `AnyContent` | Yes | - | `WFImage` | - |
| `length` | `Text` | Yes | - | `WFImageResizeLength` | - |

---

### `resizeImageByPercent`

**Title:** Resize Image By Percent  
**Apple Identifier:** `is.workflow.actions.image.resize`  
```cherri
resizeImageByPercent(image: AnyContent, percentage: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `image` | `AnyContent` | Yes | - | `WFImage` | - |
| `percentage` | `Text` | Yes | - | `WFImageResizePercentage` | - |

---

### `resizeWindow`

**Title:** Resize Window  
**Apple Identifier:** `is.workflow.actions.resizewindow`  

Resize a window to defined size.

```cherri
resizeWindow(window: AnyContent, configuration: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `window` | `AnyContent` | Yes | - | `WFWindow` | - |
| `configuration` | `Text` | Yes | - | `WFConfiguration` | Fit Screen, Top Half, Bottom Half, Left Half, Right Half, Top Left Quarter, Top Right Quarter, Bottom Left Quarter, Bottom Right Quarter, Dimensions |

---

### `returnToHomescreen`

**Title:** Return to Home Screen  
**Apple Identifier:** `is.workflow.actions.returntohomescreen`  

Returns to the device home screen.

```cherri
returnToHomescreen()
```

---

### `reveal`

**Title:** Reveal in Finder  
**Apple Identifier:** `is.workflow.actions.file.reveal`  
```cherri
reveal(files: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `files` | `AnyContent` | Yes | - | `WFFile` | - |

---

### `rotateMedia`

**Title:** Rotate Media  
**Apple Identifier:** `is.workflow.actions.image.rotate`  

Rotate an image or a video

```cherri
rotateMedia(media: AnyContent, degrees: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `media` | `AnyContent` | Yes | - | `WFImage` | - |
| `degrees` | `Text` | Yes | - | `WFImageRotateAmount` | - |

---

### `round`

**Title:** Round  
**Apple Identifier:** `is.workflow.actions.round`  
**Returns:** `Number`  

Rounds number to specified rounding place.

```cherri
round(number: Number, roundTo?: Text = "Integer") -> Number
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `number` | `Number` | Yes | - | `WFInput` | - |
| `roundTo` | `Text` | No | `Integer` | `WFRoundTo` | Millions, Hundred Thousands, Ten Thousands, Thousands, Hundreds, Tens, Integer, Tenths, Hundredths, Thousandths, Ten-Thousandths, Hundred-Thousandths, Millionths, Ten-Millionths, Hundred-Millionths, Billionths, 10^ |

---

### `runAppleScript`

**Title:** Run Apple Script  
**Apple Identifier:** `is.workflow.actions.runapplescript`  
```cherri
runAppleScript(script: Unknown, input?: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `script` | `Unknown` | Yes | - | `Script` | - |
| `input` | `AnyContent` | No | - | `Input` | - |

---

### `runJSAutomation`

**Title:** Run JavaScript for Automation  
**Apple Identifier:** `is.workflow.actions.runjavascriptforautomation`  
```cherri
runJSAutomation(script: Text, input?: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `script` | `Text` | Yes | - | `Script` | - |
| `input` | `AnyContent` | No | - | `Input` | - |

---

### `runJavaScriptOnWebpage`

**Title:** Run JavaScript on Webpage  
**Apple Identifier:** `is.workflow.actions.runjavascriptonwebpage`  

Run some custom JavaScript on the current webpage in Safari.

```cherri
runJavaScriptOnWebpage(javascript: Text, input?: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `javascript` | `Text` | Yes | - | `WFJavaScript` | - |
| `input` | `AnyContent` | No | - | `WFInput` | - |

---

### `runSSHScript`

**Title:** Run SSH Script  
**Apple Identifier:** `is.workflow.actions.runsshscript`  

Run an SSH script using connection details.

```cherri
runSSHScript(script: Text, input: AnyContent, host: Text, port: Text, user: Text, authType: Text, password: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `script` | `Text` | Yes | - | `WFSSHScript` | - |
| `input` | `AnyContent` | Yes | - | `WFInput` | - |
| `host` | `Text` | Yes | - | `WFSSHHost` | - |
| `port` | `Text` | Yes | - | `WFSSHPort` | - |
| `user` | `Text` | Yes | - | `WFSSHUser` | - |
| `authType` | `Text` | Yes | - | `WFSSHAuthenticationType` | Password, SSH Key |
| `password` | `Text` | Yes | - | `WFSSHPassword` | - |

---

### `runShellScript`

**Title:** Run Shell Script  
**Apple Identifier:** `is.workflow.actions.runshellscript`  
```cherri
runShellScript(script: Text, input?: AnyContent, shell?: Text = "/bin/zsh", inputMode?: Text = "to stdin")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `script` | `Text` | Yes | - | `Script` | - |
| `input` | `AnyContent` | No | - | `Input` | - |
| `shell` | `Text` | No | `/bin/zsh` | `Shell` | - |
| `inputMode` | `Text` | No | `to stdin` | `InputMode` | - |

---

### `saveFile`

**Title:** Save File  
**Apple Identifier:** `is.workflow.actions.documentpicker.save`  

Save `content` to `path`, optionally into a referenced `folder`.

```cherri
saveFile(path: Text, content: AnyContent, overwrite?: Bool = "false", folder?: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `path` | `Text` | Yes | - | `WFFileDestinationPath` | - |
| `content` | `AnyContent` | Yes | - | `WFInput` | - |
| `overwrite` | `Bool` | No | `false` | `WFSaveFileOverwrite` | - |
| `folder` | `AnyContent` | No | - | `WFFolder` | - |

---

### `saveFilePrompt`

**Title:** Save File Prompt  
**Apple Identifier:** `is.workflow.actions.documentpicker.save`  

Prompt the user to choose a location to save the file.

```cherri
saveFilePrompt(file: AnyContent, overwrite?: Bool = "false")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `file` | `AnyContent` | Yes | - | `WFInput` | - |
| `overwrite` | `Bool` | No | `false` | `WFSaveFileOverwrite` | - |

---

### `savePhoto`

**Title:** Save Photo to Album  
**Apple Identifier:** `is.workflow.actions.savetocameraroll`  
```cherri
savePhoto(image: AnyContent, album?: Text = "Recents")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `image` | `AnyContent` | Yes | - | `WFInput` | - |
| `album` | `Text` | No | `Recents` | `WFCameraRollSelectedGroup` | - |

---

### `saveToDropbox`

**Title:** Save File to Dropbox  
**Apple Identifier:** `is.workflow.actions.dropbox.savefile`  

Save a file to Dropbox. Requires user to setup Dropbox account in Shortcuts app.

```cherri
saveToDropbox(file: AnyContent, path: Text, overwrite?: Bool = "false")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `file` | `AnyContent` | Yes | - | `WFInput` | - |
| `path` | `Text` | Yes | - | `WFFileDestinationPath` | - |
| `overwrite` | `Bool` | No | `false` | `WFSaveFileOverwrite` | - |

---

### `saveToDropboxPrompt`

**Title:** Save File to Dropbox Prompt  
**Apple Identifier:** `is.workflow.actions.dropbox.savefile`  

Prompt to save a file to Dropbox. Requires user to setup Dropbox account in Shortcuts app.

```cherri
saveToDropboxPrompt(file: AnyContent, overwrite?: Bool = "false")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `file` | `AnyContent` | Yes | - | `WFInput` | - |
| `overwrite` | `Bool` | No | `false` | `WFSaveFileOverwrite` | - |

---

### `search`

**Title:** Search/Spotlight  
**Apple Identifier:** `is.workflow.actions.spotlightsearch`  
**Returns:** `List<AnyContent>`  

Get results from search on iOS or iPadOS, and Spotlight on macOS.

```cherri
search(query: Text, limit?: Number = "5", resultType?: List<AnyContent> = "[Calendar Events Contacts Mail Messages Notes Photos Reminders Voice Recordings Bookmarks]") -> List<AnyContent>
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `query` | `Text` | Yes | - | `WFInputText` | - |
| `limit` | `Number` | No | `5` | `WFSpotlightSearchLimit` | - |
| `resultType` | `List<AnyContent>` | No | `[Calendar Events Contacts Mail Messages Notes Photos Reminders Voice Recordings Bookmarks]` | `WFSpotlightSearchResultType` | - |

---

### `searchAppStore`

**Title:** Search App Store  
**Apple Identifier:** `is.workflow.actions.searchappstore`  
```cherri
searchAppStore(query: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `query` | `Text` | Yes | - | `WFSearchTerm` | - |

---

### `searchGiphy`

**Title:** Search Giphy  
**Apple Identifier:** `is.workflow.actions.giphy`  

Gets GIFs from Giphy for a search query.

```cherri
searchGiphy(query: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `query` | `Text` | Yes | - | `WFGiphyQuery` | - |

---

### `searchPasswords`

**Title:** Search Passwords  
**Apple Identifier:** `is.workflow.actions.openpasswords`  

Searches passwords in the Passwords app.

```cherri
searchPasswords(query: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `query` | `Text` | Yes | - | `WFShowPasswordsSearchTerm` | - |

---

### `searchPhotos`

**Title:** Search Photos  
**Apple Identifier:** `com.apple.mobileslideshow.PhotosSearchAssistantIntent`  
**Returns:** `List<AnyContent>`  
```cherri
searchPhotos(criteria: Text) -> List<AnyContent>
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `criteria` | `Text` | Yes | - | `criteria` | - |

---

### `searchPodcasts`

**Title:** Search Podcasts  
**Apple Identifier:** `is.workflow.actions.searchpodcasts`  
```cherri
searchPodcasts(query: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `query` | `Text` | Yes | - | `WFSearchTerm` | - |

---

### `searchShortcuts`

**Title:** Search Shortcuts  
**Apple Identifier:** `com.apple.shortcuts.SearchShortcutsAction`  

Search the users Shortcuts.

```cherri
searchShortcuts(query: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `query` | `Text` | Yes | - | `searchPhrase` | - |

---

### `searchVoiceMemos`

**Title:** Search Voice Memos  
**Apple Identifier:** `com.apple.VoiceMemos.SearchRecordings`  
**Returns:** `List<AnyContent>`  
```cherri
searchVoiceMemos(search: Text) -> List<AnyContent>
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `search` | `Text` | Yes | - | `searchPhrase` | - |

---

### `searchWeb`

**Title:** Search Web  
**Apple Identifier:** `is.workflow.actions.searchweb`  

Search the web using a provided search engine and query.

```cherri
searchWeb(engine: Text, query: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `engine` | `Text` | Yes | - | `WFSearchWebDestination` | Amazon, Bing, DuckDuckGo, eBay, Google, Reddit, Twitter, Yahoo!, YouTube |
| `query` | `Text` | Yes | - | `WFInputText` | - |

---

### `searchiTunes`

**Title:** Search iTunes  
**Apple Identifier:** `is.workflow.actions.searchitunes`  

Search for music or media on iTunes.

```cherri
searchiTunes(query: Text, limit?: Number)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `query` | `Text` | Yes | - | `WFSearchTerm` | - |
| `limit` | `Number` | No | - | `WFItemLimit` | - |

---

### `seek`

**Title:** Seek  
**Apple Identifier:** `is.workflow.actions.seek`  

Seek the currently playing media.

```cherri
seek(timeInterval: Unknown, behavior?: Text = "To Time")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `timeInterval` | `Unknown` | No | `[{number 0} {text sec}]` | `WFTimeInterval` | hr, min, sec |
| `behavior` | `Text` | No | `To Time` | `WFSeekBehavior` | To Time, Forward By, Backward By |

---

### `selectContact`

**Title:** Select Contact  
**Apple Identifier:** `is.workflow.actions.selectcontacts`  

Prompt the user to select a contact or multiple contacts. Returns selected contact.

```cherri
selectContact(multiple: Bool)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `multiple` | `Bool` | No | `false` | `WFSelectMultiple` | - |

---

### `selectContacts`

**Title:** Contacts  
**Apple Identifier:** `is.workflow.actions.contacts`  

Select contacts or specify a contact.

```cherri
selectContacts(contact: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `contact` | `AnyContent` | No | - | `WFContact` | - |

---

### `selectEmailAddress`

**Title:** Select Email Address  
**Apple Identifier:** `is.workflow.actions.selectemail`  

Prompt the user to select an email address from their contacts.

```cherri
selectEmailAddress()
```

---

### `selectFile`

**Title:** Select File  
**Apple Identifier:** `is.workflow.actions.file.select`  

Prompt the user to select one or optionally multiple files.

```cherri
selectFile(selectMultiple: Bool)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `selectMultiple` | `Bool` | No | `false` | `SelectMultiple` | - |

---

### `selectFolder`

**Title:** Select Folder  
**Apple Identifier:** `is.workflow.actions.file.select`  

Prompt the user to select one or optionally multiple folders.

```cherri
selectFolder(selectMultiple: Bool)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `selectMultiple` | `Bool` | No | `false` | `SelectMultiple` | - |

---

### `selectMusic`

**Title:** Select Music  
**Apple Identifier:** `is.workflow.actions.exportsong`  

Prompt the user to select music.

```cherri
selectMusic(selectMultiple: Bool)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `selectMultiple` | `Bool` | No | `false` | `WFExportSongActionSelectMultiple` | - |

---

### `selectPhoneNumber`

**Title:** Select Phone Number  
**Apple Identifier:** `is.workflow.actions.selectphone`  

Prompt the user to select a phone number from their contacts.

```cherri
selectPhoneNumber()
```

---

### `selectPhotos`

**Title:** Select Photos  
**Apple Identifier:** `is.workflow.actions.selectphoto`  

Prompt the user to select from their photo library.

```cherri
selectPhotos(selectMultiple: Bool)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `selectMultiple` | `Bool` | No | `false` | `WFSelectMultiplePhotos` | - |

---

### `sendEmail`

**Title:** Send Email  
**Apple Identifier:** `is.workflow.actions.sendemail`  

Send an email to a contact.

```cherri
sendEmail(contact: AnyContent, from: Text, subject: Text, body: Text, prompt?: Bool = "true", draft?: Bool = "false")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `contact` | `AnyContent` | Yes | - | `WFSendEmailActionToRecipients` | - |
| `from` | `Text` | Yes | - | `WFSendEmailActionFrom` | - |
| `subject` | `Text` | Yes | - | `WFSendEmailActionSubject` | - |
| `body` | `Text` | Yes | - | `WFSendEmailActionInputAttachments` | - |
| `prompt` | `Bool` | No | `true` | `WFSendEmailActionShowComposeSheet` | - |
| `draft` | `Bool` | No | `false` | `WFSendEmailActionSaveAsDraft` | - |

---

### `sendMessage`

**Title:** Send Message  
**Apple Identifier:** `is.workflow.actions.sendmessage`  

Send an SMS/iMessage to a contact.

```cherri
sendMessage(contact: AnyContent, message: Text, prompt?: Bool = "true")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `contact` | `AnyContent` | Yes | - | `WFSendMessageActionRecipients` | - |
| `message` | `Text` | Yes | - | `WFSendMessageContent` | - |
| `prompt` | `Bool` | No | `true` | `ShowWhenRun` | - |

---

### `setAirdropReceiving`

**Title:** Set AirDrop Receiving  
**Apple Identifier:** `is.workflow.actions.setairdropreceiving`  

Set the AirDrop receiving setting.

```cherri
setAirdropReceiving(state: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `state` | `Text` | No | `Everyone` | `WFAirDropState` | No One, Contacts Only, Everyone |

---

### `setAirplaneMode`

**Title:** Set Airplane Mode  
**Apple Identifier:** `is.workflow.actions.airplanemode.set`  
```cherri
setAirplaneMode(status: Bool)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `status` | `Bool` | Yes | - | `OnValue` | - |

---

### `setAutoAnswerCalls`

**Title:** Set Auto Answer Calls  
**Apple Identifier:** `com.apple.AccessibilityUtilities.AXSettingsShortcuts.AXToggleAutoAnswerCallsIntent`  
```cherri
setAutoAnswerCalls(status: Bool)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `status` | `Bool` | Yes | - | `state` | - |

---

### `setBackgroundSound`

**Title:** Set Background Sound  
**Apple Identifier:** `com.apple.UniversalAccess.UASettingsShortcuts.UASetBackgroundSoundIntent`  

Set the background sound that plays.

```cherri
setBackgroundSound(sound: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `sound` | `Text` | No | `BalancedNoise` | `backgroundSound` | BalancedNoise, BrightNoise, DarkNoise, Ocean, Rain, Stream, Babble, Steam, Airplane, Boat, Bus, Train, RainOnRoof, QuietNight, Fire, Night |

---

### `setBackgroundSounds`

**Title:** Set Background Sounds  
**Apple Identifier:** `com.apple.UniversalAccess.AXSettingsShortcuts.AXToggleBackgroundSoundsIntent`  
```cherri
setBackgroundSounds(status: Bool)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `status` | `Bool` | Yes | - | `state` | - |

---

### `setBackgroundSoundsVolume`

**Title:** Set Background Sound Volume  
**Apple Identifier:** `com.apple.UniversalAccess.UASettingsShortcuts.UASetBackgroundSoundsVolumeIntent`  
```cherri
setBackgroundSoundsVolume(volume: Number)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `volume` | `Number` | Yes | - | `volumeValue` | - |

---

### `setBluetooth`

**Title:** Set Bluetooth  
**Apple Identifier:** `is.workflow.actions.bluetooth.set`  
```cherri
setBluetooth(status: Bool)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `status` | `Bool` | Yes | - | `OnValue` | - |

---

### `setBrightness`

**Title:** Set Brightness  
**Apple Identifier:** `is.workflow.actions.setbrightness`  
```cherri
setBrightness(brightness: Number)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `brightness` | `Number` | Yes | - | `WFBrightness` | - |

---

### `setCellularData`

**Title:** Set Cellular Data  
**Apple Identifier:** `is.workflow.actions.cellulardata.set`  
```cherri
setCellularData(status: Bool)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `status` | `Bool` | Yes | - | `OnValue` | - |

---

### `setClassicInvert`

**Title:** Set Classic Invert  
**Apple Identifier:** `com.apple.AccessibilityUtilities.AXSettingsShortcuts.AXToggleClassicInvertIntent`  
```cherri
setClassicInvert(status: Bool)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `status` | `Bool` | Yes | - | `state` | - |

---

### `setClipboard`

**Title:** Set Clipboard  
**Apple Identifier:** `is.workflow.actions.setclipboard`  

Set the contents of the clipboard.

```cherri
setClipboard(value: AnyContent, local?: Bool = "false", expire?: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `value` | `AnyContent` | Yes | - | `WFInput` | - |
| `local` | `Bool` | No | `false` | `WFLocalOnly` | - |
| `expire` | `Text` | No | - | `WFExpirationDate` | - |

---

### `setClosedCaptionsSDH`

**Title:** Set Closed Captions SDH  
**Apple Identifier:** `com.apple.AccessibilityUtilities.AXSettingsShortcuts.AXToggleCaptionsIntent`  
```cherri
setClosedCaptionsSDH(status: Bool)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `status` | `Bool` | Yes | - | `state` | - |

---

### `setColorFilters`

**Title:** Set Color Filters  
**Apple Identifier:** `com.apple.AccessibilityUtilities.AXSettingsShortcuts.AXToggleColorFiltersIntent`  
```cherri
setColorFilters(status: Bool)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `status` | `Bool` | Yes | - | `state` | - |

---

### `setContrast`

**Title:** Set Contrast  
**Apple Identifier:** `com.apple.AccessibilityUtilities.AXSettingsShortcuts.AXToggleContrastIntent`  
```cherri
setContrast(status: Bool)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `status` | `Bool` | Yes | - | `state` | - |

---

### `setHotspot`

**Title:** Set Personal Hotspot  
**Apple Identifier:** `is.workflow.actions.personalhotspot.set`  
```cherri
setHotspot(status: Bool)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `status` | `Bool` | Yes | - | `state` | - |

---

### `setHotspotPassword`

**Title:** Set Personal Hotspot Password  
**Apple Identifier:** `is.workflow.actions.personalhotspot.password.set`  

Update the password to your personal hotspot.

```cherri
setHotspotPassword(newPassword: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `newPassword` | `Text` | Yes | - | `WFInput` | - |

---

### `setLEDFlash`

**Title:** Set LED Flash  
**Apple Identifier:** `com.apple.AccessibilityUtilities.AXSettingsShortcuts.AXToggleLEDFlashIntent`  
```cherri
setLEDFlash(status: Bool)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `status` | `Bool` | Yes | - | `state` | - |

---

### `setLeftRightBalance`

**Title:** Set Left-Right Balance  
**Apple Identifier:** `com.apple.AccessibilityUtilities.AXSettingsShortcuts.AXSetLeftRightBalanceIntent`  
```cherri
setLeftRightBalance(status: Bool, value?: Number)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `status` | `Bool` | Yes | - | `state` | - |
| `value` | `Number` | No | - | `value` | - |

---

### `setLiveCaptions`

**Title:** Set Live Captions  
**Apple Identifier:** `com.apple.AccessibilityUtilities.AXSettingsShortcuts.AXToggleLiveCaptionsIntent`  
```cherri
setLiveCaptions(status: Bool)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `status` | `Bool` | Yes | - | `state` | - |

---

### `setLowPowerMode`

**Title:** Set Low Power Mode  
**Apple Identifier:** `is.workflow.actions.lowpowermode.set`  
```cherri
setLowPowerMode(status: Bool)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `status` | `Bool` | Yes | - | `OnValue` | - |

---

### `setMediaBackgroundSounds`

**Title:** Set Media Background Sounds  
**Apple Identifier:** `com.apple.UniversalAccess.AXSettingsShortcuts.AXToggleBackgroundSoundsIntent`  
```cherri
setMediaBackgroundSounds(status: Bool)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `status` | `Bool` | Yes | - | `state` | - |

---

### `setMetadata`

**Title:** Set Media Metadata  
**Apple Identifier:** `is.workflow.actions.encodemedia`  
```cherri
setMetadata(media: AnyContent, artwork?: AnyContent, title?: Text, artist?: Text, album?: Text, genre?: Text, year?: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `media` | `AnyContent` | Yes | - | `WFMedia` | - |
| `artwork` | `AnyContent` | No | - | `WFMetadataArtwork` | - |
| `title` | `Text` | No | - | `WFMetadataTitle` | - |
| `artist` | `Text` | No | - | `WFMetadataArtist` | - |
| `album` | `Text` | No | - | `WFMetadataAlbum` | - |
| `genre` | `Text` | No | - | `WFMetadataGenre` | - |
| `year` | `Text` | No | - | `WFMetadataYear` | - |

---

### `setMonoAudio`

**Title:** Set Mono Audio  
**Apple Identifier:** `com.apple.AccessibilityUtilities.AXSettingsShortcuts.AXToggleMonoAudioIntent`  
```cherri
setMonoAudio(status: Bool)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `status` | `Bool` | Yes | - | `state` | - |

---

### `setName`

**Title:** Set Name  
**Apple Identifier:** `is.workflow.actions.setitemname`  

Set the name of an item.

```cherri
setName(item: AnyContent, name: Text, includeFileExtension?: Bool = "false")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `item` | `AnyContent` | Yes | - | `WFInput` | - |
| `name` | `Text` | Yes | - | `WFName` | - |
| `includeFileExtension` | `Bool` | No | `false` | `WFDontIncludeFileExtension` | - |

---

### `setNightShift`

**Title:** Set Night Shift  
**Apple Identifier:** `is.workflow.actions.nightshift.set`  
```cherri
setNightShift(status: Bool)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `status` | `Bool` | Yes | - | `OnValue` | - |

---

### `setOrientationLock`

**Title:** Set Orientation Lock  
**Apple Identifier:** `is.workflow.actions.orientationlock.set`  
```cherri
setOrientationLock(status: Bool)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `status` | `Bool` | Yes | - | `OnValue` | - |

---

### `setReduceMotion`

**Title:** Set Reduce Motion  
**Apple Identifier:** `com.apple.AccessibilityUtilities.AXSettingsShortcuts.AXToggleReduceMotionIntent`  
```cherri
setReduceMotion(status: Bool)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `status` | `Bool` | Yes | - | `state` | - |

---

### `setReduceTransparency`

**Title:** Set Reduce Transparency  
**Apple Identifier:** `com.apple.AccessibilityUtilities.AXSettingsShortcuts.AXToggleTransparencyIntent`  
```cherri
setReduceTransparency(status: Bool)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `status` | `Bool` | Yes | - | `state` | - |

---

### `setSmartInvert`

**Title:** Set Smart Invert  
**Apple Identifier:** `com.apple.AccessibilityUtilities.AXSettingsShortcuts.AXToggleSmartInvertIntent`  
```cherri
setSmartInvert(status: Bool)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `status` | `Bool` | Yes | - | `state` | - |

---

### `setSoundRecognition`

**Title:** Set Sound Recognition  
**Apple Identifier:** `com.apple.AccessibilityUtilities.AXSettingsShortcuts.AXToggleSoundDetectionIntent`  
```cherri
setSoundRecognition(operation: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `operation` | `Text` | No | `activate` | `operation` | pause, activate, toggle |

---

### `setStageManager`

**Title:** Set Stage Manager  
**Apple Identifier:** `is.workflow.actions.stagemanager.set`  
```cherri
setStageManager(status: Bool, showDock?: Bool = "true", showRecentApps?: Bool = "true")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `status` | `Bool` | Yes | - | `state` | - |
| `showDock` | `Bool` | No | `true` | `showDock` | - |
| `showRecentApps` | `Bool` | No | `true` | `showRecentApps` | - |

---

### `setSwitchControl`

**Title:** Set Switch Control  
**Apple Identifier:** `com.apple.AccessibilityUtilities.AXSettingsShortcuts.AXToggleSwitchControlIntent`  
```cherri
setSwitchControl(status: Bool)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `status` | `Bool` | Yes | - | `state` | - |

---

### `setTextSize`

**Title:** Set Text Size  
**Apple Identifier:** `com.apple.AccessibilityUtilities.AXSettingsShortcuts.AXSetLargeTextIntent`  

Sets the system text size.

```cherri
setTextSize(size: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `size` | `Text` | Yes | - | `textSize` | Accessibility Extra Extra Extra Large, Accessibility Extra Extra Large, Accessibility Extra Large, Accessibility Large, Accessibility Medium, Extra Extra Extra Large, Extra Extra Large, Extra Large, Default, Medium, Small, Extra Small |

---

### `setTrueTone`

**Title:** Set True Tone  
**Apple Identifier:** `is.workflow.actions.truetone.set`  
```cherri
setTrueTone(status: Bool)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `status` | `Bool` | Yes | - | `OnValue` | - |

---

### `setValue`

**Title:** Set Value in Dictionary  
**Apple Identifier:** `is.workflow.actions.setvalueforkey`  
**Returns:** `Map<Text, AnyContent>`  

Set the value of `key` to `value` in `dictionary`.

```cherri
setValue(dictionary: AnyContent, key: Text, value: Text) -> Map<Text, AnyContent>
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `dictionary` | `AnyContent` | Yes | - | `WFDictionary` | - |
| `key` | `Text` | Yes | - | `WFDictionaryKey` | - |
| `value` | `Text` | Yes | - | `WFDictionaryValue` | - |

---

### `setVoiceControl`

**Title:** Set Voice Control  
**Apple Identifier:** `com.apple.AccessibilityUtilities.AXSettingsShortcuts.AXToggleVoiceControlIntent`  
```cherri
setVoiceControl(status: Bool)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `status` | `Bool` | Yes | - | `state` | - |

---

### `setVolume`

**Title:** Set Volume  
**Apple Identifier:** `is.workflow.actions.setvolume`  
```cherri
setVolume(volume: Number)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `volume` | `Number` | Yes | - | `WFVolume` | - |

---

### `setWallpaper`

**Title:** Set Wallpaper  
**Apple Identifier:** `is.workflow.actions.wallpaper.set`  

Sets the device wallpaper to `input`.

```cherri
setWallpaper(input: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `input` | `AnyContent` | Yes | - | `WFInput` | - |

---

### `setWhitePoint`

**Title:** Set White Point  
**Apple Identifier:** `com.apple.AccessibilityUtilities.AXSettingsShortcuts.AXToggleWhitePointIntent`  
```cherri
setWhitePoint(status: Bool)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `status` | `Bool` | Yes | - | `state` | - |

---

### `setWifi`

**Title:** Set Wifi  
**Apple Identifier:** `is.workflow.actions.wifi.set`  
```cherri
setWifi(status: Bool)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `status` | `Bool` | Yes | - | `OnValue` | - |

---

### `setZoom`

**Title:** Set Zoom  
**Apple Identifier:** `com.apple.AccessibilityUtilities.AXSettingsShortcuts.AXToggleZoomIntent`  
```cherri
setZoom(status: Bool)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `status` | `Bool` | Yes | - | `state` | - |

---

### `share`

**Title:** Share  
**Apple Identifier:** `is.workflow.actions.share`  

Prompt the user to share `input`.

```cherri
share(input: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `input` | `AnyContent` | Yes | - | `WFInput` | - |

---

### `show`

**Title:** Show Result  
**Apple Identifier:** `is.workflow.actions.showresult`  
**Returns:** `Void`  

Show `input`.

```cherri
show(input: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `input` | `Text` | Yes | - | `Text` | - |

---

### `showControlCenter`

**Title:** Show Control Center  
**Apple Identifier:** `com.apple.ShortcutsActions.ShowControlCenterAction`  

Open the control center on the device.

```cherri
showControlCenter()
```

---

### `showInCalendar`

**Title:** Open Event in Calendar  
**Apple Identifier:** `is.workflow.actions.showincalendar`  

Show `event` in the calendar app.

```cherri
showInCalendar(event: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `event` | `AnyContent` | Yes | - | `WFEvent` | - |

---

### `showIniTunes`

**Title:** Show In iTunes  
**Apple Identifier:** `is.workflow.actions.showinstore`  
```cherri
showIniTunes(product: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `product` | `AnyContent` | Yes | - | `WFProduct` | - |

---

### `showNotification`

**Title:** Show Notification  
**Apple Identifier:** `is.workflow.actions.notification`  

Shows a custom notification message.

```cherri
showNotification(body: Text, title?: Text, playSound?: Bool = "true", attachment?: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `body` | `Text` | Yes | - | `WFNotificationActionBody` | - |
| `title` | `Text` | No | - | `WFNotificationActionTitle` | - |
| `playSound` | `Bool` | No | `true` | `WFNotificationActionSound` | - |
| `attachment` | `AnyContent` | No | - | `WFInput` | - |

---

### `showQuickNote`

**Title:** Show Quick Note  
**Apple Identifier:** `com.apple.mobilenotes.ShowQuickNoteIntent`  

Show quick note.

```cherri
showQuickNote()
```

---

### `showWebpage`

**Title:** Show Webpage  
**Apple Identifier:** `is.workflow.actions.showwebpage`  

Show webpage using Safari.

```cherri
showWebpage(url: Text, useReader?: Bool)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `url` | `Text` | Yes | - | `WFURL` | - |
| `useReader` | `Bool` | No | - | `WFEnterSafariReader` | - |

---

### `shutdown`

**Title:** Shut Down  
**Apple Identifier:** `is.workflow.actions.reboot`  

Power off the device.

```cherri
shutdown()
```

---

### `skipBack`

**Title:** Skip Back  
**Apple Identifier:** `is.workflow.actions.skipback`  

Go to the previous song.

```cherri
skipBack()
```

---

### `skipFwd`

**Title:** Skip Forward  
**Apple Identifier:** `is.workflow.actions.skipforward`  

Go to the next song.

```cherri
skipFwd()
```

---

### `sleep`

**Title:** Sleep  
**Apple Identifier:** `is.workflow.actions.sleep`  

Puts the Mac to sleep.

```cherri
sleep()
```

---

### `speak`

**Title:** Speak  
**Apple Identifier:** `is.workflow.actions.speaktext`  

Speaks the provided text, optionally in another language.

```cherri
speak(prompt: Text, waitUntilFinished?: Bool = "true", language?: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `prompt` | `Text` | Yes | - | `WFText` | - |
| `waitUntilFinished` | `Bool` | No | `true` | `WFSpeakTextWait` | - |
| `language` | `Text` | No | - | `WFSpeakTextLanguage` | - |

---

### `splitPDF`

**Title:** Split PDF  
**Apple Identifier:** `is.workflow.actions.splitpdf`  
**Returns:** `List<AnyContent>`  

Splits a PDF into pages.

```cherri
splitPDF(pdf: AnyContent) -> List<AnyContent>
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `pdf` | `AnyContent` | Yes | - | `WFInput` | - |

---

### `startScreensaver`

**Title:** Start Screensaver  
**Apple Identifier:** `is.workflow.actions.startscreensaver`  
```cherri
startScreensaver()
```

---

### `startShazam`

**Title:** Start Shazam  
**Apple Identifier:** `is.workflow.actions.shazamMedia`  

Prompt the user to play music for Shazam to recognize. Returns Shazam result.

```cherri
startShazam(show: Bool, showError?: Bool = "true")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `show` | `Bool` | No | `true` | `WFShazamMediaActionShowWhenRun` | - |
| `showError` | `Bool` | No | `true` | `WFShazamMediaActionErrorIfNotRecognized` | - |

---

### `startTimer`

**Title:** Start Timer  
**Apple Identifier:** `is.workflow.actions.timer.start`  

Creates a new timer.

```cherri
startTimer(duration: Unknown)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `duration` | `Unknown` | No | `[{number 0} {text min}]` | `WFDuration` | hr, min, sec |

---

### `statistic`

**Title:** Statistic  
**Apple Identifier:** `is.workflow.actions.statistics`  

Perform a statistical operation on `input`.

```cherri
statistic(operation: Text, input: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `operation` | `Text` | Yes | - | `WFStatisticsOperation` | Average, Minimum, Maximum, Sum, Median, Mode, Range, Standard Deviation |
| `input` | `AnyContent` | Yes | - | `WFInput` | - |

---

### `stop`

**Title:** Stop Shortcut  
**Apple Identifier:** `is.workflow.actions.exit`  

Stops the shortcut.

```cherri
stop()
```

---

### `storeValue`

**Title:** Store Content  
**Apple Identifier:** `is.workflow.actions.setstoredcontent`  

Store content by a key, optionally globally or specific to the Shortcut.

```cherri
storeValue(key: Text, value: Text, global?: Bool = "false")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `key` | `Text` | Yes | - | `WFStoredContentKey` | - |
| `value` | `Text` | Yes | - | `WFInput` | - |
| `global` | `Bool` | No | `false` | `WFStoredContentGlobalValue` | - |

---

### `streetAddress`

**Title:** Street Address  
**Apple Identifier:** `is.workflow.actions.address`  

Create a location value with an address.

```cherri
streetAddress(addressLine2: Text, city: Text, state: Text, country: Text, zipCode: Number)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `addressLine2` | `Text` | Yes | - | `WFAddressLine1` | - |
| `addressLine2` | `Text` | Yes | - | `WFAddressLine2` | - |
| `city` | `Text` | Yes | - | `WFCity` | - |
| `state` | `Text` | Yes | - | `WFState` | - |
| `country` | `Text` | Yes | - | `WFCountry` | - |
| `zipCode` | `Number` | Yes | - | `WFPostalCode` | - |

---

### `stripImageMetadata`

**Title:** Strip Image Metadata  
**Apple Identifier:** `is.workflow.actions.image.convert`  
```cherri
stripImageMetadata(image: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `image` | `AnyContent` | Yes | - | `WFInput` | - |

---

### `stripMediaMetadata`

**Title:** Strip Media Metadata  
**Apple Identifier:** `is.workflow.actions.encodemedia`  
```cherri
stripMediaMetadata(media: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `media` | `AnyContent` | Yes | - | `WFMedia` | - |

---

### `takeInteractiveScreenshot`

**Title:** Take Interactive Screenshot  
**Apple Identifier:** `is.workflow.actions.takescreenshot`  
```cherri
takeInteractiveScreenshot(selection: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `selection` | `Text` | No | `Window` | `WFTakeScreenshotActionInteractiveSelectionType` | Window, Custom |

---

### `takePhoto`

**Title:** Take Photo  
**Apple Identifier:** `is.workflow.actions.takephoto`  

Prompt the user to take one or more photos.

```cherri
takePhoto(count: Number, showPreview?: Bool = "true")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `count` | `Number` | No | `1` | `WFPhotoCount` | - |
| `showPreview` | `Bool` | No | `true` | `WFCameraCaptureShowPreview` | - |

---

### `takeScreenshot`

**Title:** Take Screenshot  
**Apple Identifier:** `is.workflow.actions.takescreenshot`  
```cherri
takeScreenshot(mainMonitorOnly: Bool)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `mainMonitorOnly` | `Bool` | No | `false` | `WFTakeScreenshotMainMonitorOnly` | - |

---

### `takeVideo`

**Title:** Take Photo  
**Apple Identifier:** `is.workflow.actions.takevideo`  

Prompt the user to start recording a video.

```cherri
takeVideo(camera: Text, quality?: Text = "High", recordingStart?: Text = "Immediately")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `camera` | `Text` | No | `Front` | `WFCameraCaptureDevice` | Front, Back |
| `quality` | `Text` | No | `High` | `WFCameraCaptureQuality` | Low, Medium, High |
| `recordingStart` | `Text` | No | `Immediately` | `WFRecordingStart` | On Tap, Immediately |

---

### `text`

**Apple Identifier:** `is.workflow.actions.gettext`  
**Returns:** `Text`  
```cherri
text(text: Text) -> Text
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `text` | `Text` | Yes | - | `WFTextActionText` | - |

---

### `titleCase`

**Title:** Title Case  
**Apple Identifier:** `is.workflow.actions.text.changecase`  
**Returns:** `Text`  

Capitalizes the text with Title Case.

```cherri
titleCase(text: Text) -> Text
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `text` | `Text` | Yes | - | `text` | - |

---

### `toggleAirplaneMode`

**Title:** Toggle Airplane Mode  
**Apple Identifier:** `is.workflow.actions.airplanemode.set`  
```cherri
toggleAirplaneMode()
```

---

### `toggleAppearance`

**Title:** Toggle Appearance  
**Apple Identifier:** `is.workflow.actions.appearance`  
```cherri
toggleAppearance()
```

---

### `toggleAutoAnswerCalls`

**Title:** Toggle Auto Answer Calls  
**Apple Identifier:** `com.apple.AccessibilityUtilities.AXSettingsShortcuts.AXToggleAutoAnswerCallsIntent`  
```cherri
toggleAutoAnswerCalls()
```

---

### `toggleBackgroundSounds`

**Title:** Toggle Background Sounds  
**Apple Identifier:** `com.apple.UniversalAccess.AXSettingsShortcuts.AXToggleBackgroundSoundsIntent`  
```cherri
toggleBackgroundSounds()
```

---

### `toggleBluetooth`

**Title:** Toggle Bluetooth  
**Apple Identifier:** `is.workflow.actions.bluetooth.set`  
```cherri
toggleBluetooth()
```

---

### `toggleCellularData`

**Title:** Toggle Cellular Data  
**Apple Identifier:** `is.workflow.actions.cellulardata.set`  
```cherri
toggleCellularData()
```

---

### `toggleClassicInvert`

**Title:** Toggle Classic Invert  
**Apple Identifier:** `com.apple.AccessibilityUtilities.AXSettingsShortcuts.AXToggleClassicInvertIntent`  
```cherri
toggleClassicInvert()
```

---

### `toggleClosedCaptionsSDH`

**Title:** Toggle Closed Captions SDH  
**Apple Identifier:** `com.apple.AccessibilityUtilities.AXSettingsShortcuts.AXToggleCaptionsIntent`  
```cherri
toggleClosedCaptionsSDH()
```

---

### `toggleColorFilters`

**Title:** Toggle Color Filters  
**Apple Identifier:** `com.apple.AccessibilityUtilities.AXSettingsShortcuts.AXToggleColorFiltersIntent`  
```cherri
toggleColorFilters()
```

---

### `toggleContrast`

**Title:** Toggle Contrast  
**Apple Identifier:** `com.apple.AccessibilityUtilities.AXSettingsShortcuts.AXToggleContrastIntent`  
```cherri
toggleContrast()
```

---

### `toggleDND`

**Title:** Toggle Do Not Disturb  
**Apple Identifier:** `is.workflow.actions.dnd.set`  
```cherri
toggleDND()
```

---

### `toggleFlashlight`

**Title:** Toggle On Flashlight  
**Apple Identifier:** `is.workflow.actions.flashlight`  

Toggle the flashlight on the device with optional brightness setting.

```cherri
toggleFlashlight(brightness: Number)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `brightness` | `Number` | No | `0.5` | `WFFlashlightLevel` | - |

---

### `toggleHotspot`

**Title:** Toggle Personal Hotspot  
**Apple Identifier:** `is.workflow.actions.personalhotspot.set`  
```cherri
toggleHotspot()
```

---

### `toggleLEDFlash`

**Title:** Toggle LED Flash  
**Apple Identifier:** `com.apple.AccessibilityUtilities.AXSettingsShortcuts.AXToggleLEDFlashIntent`  
```cherri
toggleLEDFlash()
```

---

### `toggleLeftRightBalance`

**Title:** Toggle Left-Right Balance  
**Apple Identifier:** `com.apple.AccessibilityUtilities.AXSettingsShortcuts.AXSetLeftRightBalanceIntent`  
```cherri
toggleLeftRightBalance(value: Number)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `value` | `Number` | No | - | `value` | - |

---

### `toggleLiveCaptions`

**Title:** Toggle Live Captions  
**Apple Identifier:** `com.apple.AccessibilityUtilities.AXSettingsShortcuts.AXToggleLiveCaptionsIntent`  
```cherri
toggleLiveCaptions()
```

---

### `toggleLowPowerMode`

**Title:** Toggle Low Power Mode  
**Apple Identifier:** `is.workflow.actions.lowpowermode.set`  
```cherri
toggleLowPowerMode()
```

---

### `toggleMediaBackgroundSounds`

**Title:** Toggle Media Background Sounds  
**Apple Identifier:** `com.apple.UniversalAccess.AXSettingsShortcuts.AXToggleBackgroundSoundsIntent`  
```cherri
toggleMediaBackgroundSounds()
```

---

### `toggleMonoAudio`

**Title:** Toggle Mono Audio  
**Apple Identifier:** `com.apple.AccessibilityUtilities.AXSettingsShortcuts.AXToggleMonoAudioIntent`  
```cherri
toggleMonoAudio()
```

---

### `toggleNightShift`

**Title:** Toggle Night Shift  
**Apple Identifier:** `is.workflow.actions.nightshift.set`  
```cherri
toggleNightShift()
```

---

### `toggleOrientationLock`

**Title:** Toggle Orientation Lock  
**Apple Identifier:** `is.workflow.actions.orientationlock.set`  
```cherri
toggleOrientationLock()
```

---

### `togglePlayPause`

**Title:** Toggle Play/Pause  
**Apple Identifier:** `is.workflow.actions.pausemusic`  
```cherri
togglePlayPause()
```

---

### `toggleReduceMotion`

**Title:** Toggle Reduce Motion  
**Apple Identifier:** `com.apple.AccessibilityUtilities.AXSettingsShortcuts.AXToggleReduceMotionIntent`  
```cherri
toggleReduceMotion()
```

---

### `toggleReduceTransparency`

**Title:** Toggle Reduce Transparency  
**Apple Identifier:** `com.apple.AccessibilityUtilities.AXSettingsShortcuts.AXToggleTransparencyIntent`  
```cherri
toggleReduceTransparency()
```

---

### `toggleSmartInvert`

**Title:** Toggle Smart Invert  
**Apple Identifier:** `com.apple.AccessibilityUtilities.AXSettingsShortcuts.AXToggleSmartInvertIntent`  
```cherri
toggleSmartInvert()
```

---

### `toggleStageManager`

**Title:** Toggle Stage Manager  
**Apple Identifier:** `is.workflow.actions.stagemanager.set`  
```cherri
toggleStageManager(showDock: Bool, showRecentApps?: Bool = "true")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `showDock` | `Bool` | No | `true` | `showDock` | - |
| `showRecentApps` | `Bool` | No | `true` | `showRecentApps` | - |

---

### `toggleSwitchControl`

**Title:** Toggle Switch Control  
**Apple Identifier:** `com.apple.AccessibilityUtilities.AXSettingsShortcuts.AXToggleSwitchControlIntent`  
```cherri
toggleSwitchControl()
```

---

### `toggleTrueTone`

**Title:** Toggle True Tone  
**Apple Identifier:** `is.workflow.actions.truetone.set`  
```cherri
toggleTrueTone()
```

---

### `toggleVoiceControl`

**Title:** Toggle Voice Control  
**Apple Identifier:** `com.apple.AccessibilityUtilities.AXSettingsShortcuts.AXToggleVoiceControlIntent`  
```cherri
toggleVoiceControl()
```

---

### `toggleWhitePoint`

**Title:** Toggle White Point  
**Apple Identifier:** `com.apple.AccessibilityUtilities.AXSettingsShortcuts.AXToggleWhitePointIntent`  
```cherri
toggleWhitePoint()
```

---

### `toggleWifi`

**Title:** Toggle Wifi  
**Apple Identifier:** `is.workflow.actions.wifi.set`  
```cherri
toggleWifi()
```

---

### `toggleZoom`

**Title:** Toggle Zoom  
**Apple Identifier:** `com.apple.AccessibilityUtilities.AXSettingsShortcuts.AXToggleZoomIntent`  
```cherri
toggleZoom()
```

---

### `translate`

**Title:** Translate  
**Apple Identifier:** `is.workflow.actions.text.translate`  
**Returns:** `Text`  

Translate text.

```cherri
translate(text: Text, to: Text, from?: Text = "Detected language") -> Text
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `text` | `Text` | Yes | - | `WFInputText` | - |
| `to` | `Text` | Yes | - | `WFSelectedLanguage` | ar_AE, zh_CN, zh_TW, nl_NL, en_GB, en_US, fr_FR, de_DE, id_ID, it_IT, jp_JP, ko_KR, pl_PL, pt_BR, ru_RU, es_ES, th_TH, tr_TR, vn_VN |
| `from` | `Text` | No | `Detected language` | `WFSelectedFromLanguage` | ar_AE, zh_CN, zh_TW, nl_NL, en_GB, en_US, fr_FR, de_DE, id_ID, it_IT, jp_JP, ko_KR, pl_PL, pt_BR, ru_RU, es_ES, th_TH, tr_TR, vn_VN |

---

### `trimVideo`

**Title:** Trim Video  
**Apple Identifier:** `is.workflow.actions.trimvideo`  

Prompts the user to trim the video.

```cherri
trimVideo(video: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `video` | `AnyContent` | Yes | - | `WFInputMedia` | - |

---

### `trimWhitespace`

**Title:** Trim Whitespace  
**Apple Identifier:** `is.workflow.actions.text.trimwhitespace`  
**Returns:** `Text`  

Trim any whitespace from the start and end of `text`.

```cherri
trimWhitespace(text: Text) -> Text
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `text` | `Text` | Yes | - | `WFInput` | - |

---

### `typeOf`

**Title:** Get Type  
**Apple Identifier:** `is.workflow.actions.getitemtype`  
**Returns:** `Text`  

Get the type of input.

```cherri
typeOf(input: AnyContent) -> Text
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `input` | `AnyContent` | Yes | - | `WFInput` | - |

---

### `uppercase`

**Title:** Uppercase  
**Apple Identifier:** `is.workflow.actions.text.changecase`  
**Returns:** `Text`  

Transforms the text to all uppercase.

```cherri
uppercase(text: Text) -> Text
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `text` | `Text` | Yes | - | `text` | - |

---

### `urlDecode`

**Title:** URL Decode  
**Apple Identifier:** `is.workflow.actions.urlencode`  
**Returns:** `Text`  

Decode text from URL encoding.

```cherri
urlDecode(input: Text) -> Text
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `input` | `Text` | Yes | - | `WFInput` | - |

---

### `urlEncode`

**Title:** URL Encode  
**Apple Identifier:** `is.workflow.actions.urlencode`  
**Returns:** `Text`  

Encode text for a URL.

```cherri
urlEncode(input: Text) -> Text
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `input` | `Text` | Yes | - | `WFInput` | - |

---

### `vibrate`

**Title:** Vibrate Device  
**Apple Identifier:** `is.workflow.actions.vibrate`  

Vibrate the device. Only applies to Apple devices with the haptic engine.

```cherri
vibrate()
```

---

### `wait`

**Title:** Wait  
**Apple Identifier:** `is.workflow.actions.delay`  

Wait a specified number of seconds.

```cherri
wait(seconds: Number)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `seconds` | `Number` | Yes | - | `WFDelayTime` | - |

---

### `waitToReturn`

**Title:** Wait to Return  
**Apple Identifier:** `is.workflow.actions.waittoreturn`  

Wait for the user to return to Shortcuts.

```cherri
waitToReturn()
```

---

## Module: Intelligence

### `generateImage`

**Title:** Create Image using Image Playground  
**Apple Identifier:** `com.apple.GenerativePlaygroundApp.GenerateImageIntent`  

Generate an Image with a prompt using the Image Playground app.

```cherri
generateImage(prompt: Text, image?: AnyContent, style?: Text = "animation", saveToPlayground?: Text = "always")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `prompt` | `Text` | Yes | - | `prompt` | - |
| `image` | `AnyContent` | No | - | `image` | - |
| `style` | `Text` | No | `animation` | `` | animation, illustration, sketch, chatgpt, chatgpt_oil_painting, chatgpt_watercolor, chatgpt_vector, chatgpt_anime, chatgpt_print |
| `saveToPlayground` | `Text` | No | `always` | `saveToPlayground` | always, askWhenRun, never |

---

## Module: Mac

### `getWindows`

**Title:** Get Windows  
**Apple Identifier:** `is.workflow.actions.filter.windows`  
```cherri
getWindows(sortBy: Text, orderBy?: Text, limit?: Number)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `sortBy` | `Text` | No | - | `WFContentItemSortProperty` | Title, App Name, Width, Height, X Position, Y Position, Window Index, Name, Random |
| `orderBy` | `Text` | No | - | `WFContentItemSortOrder` | asc, desc |
| `limit` | `Number` | No | - | `WFContentItemLimitNumber` | - |

---

## Module: Math

### `calculateExpression`

**Title:** Calculate Expression  
**Apple Identifier:** `is.workflow.actions.calculateexpression`  
```cherri
calculateExpression()
```

---

### `convertMeasurement`

**Title:** Convert Measurement  
**Apple Identifier:** `is.workflow.actions.measurement.convert`  
```cherri
convertMeasurement(measurement: AnyContent, unitType: Text, unit: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `measurement` | `AnyContent` | Yes | - | `WFInput` | - |
| `unitType` | `Text` | Yes | - | `WFMeasurementUnitType` | Acceleration, Angle, Area, Concentration Mass, Dispersion, Duration, Electric Charge, Electric Current, Electric Potential Difference, V Electric Resistance, Energy, Frequency, Fuel Efficiency, Illuminance, Information Storage, Length, Mass, Power, Pressure, Speed, Temperature, Volume |
| `unit` | `Text` | Yes | - | `` | - |

---

### `measurement`

**Title:** Create Measurement  
**Apple Identifier:** `is.workflow.actions.measurement.create`  
```cherri
measurement(magnitude: Text, unitType: Text, unit: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `magnitude` | `Text` | Yes | - | `` | - |
| `unitType` | `Text` | Yes | - | `WFMeasurementUnitType` | Acceleration, Angle, Area, Concentration Mass, Dispersion, Duration, Electric Charge, Electric Current, Electric Potential Difference, V Electric Resistance, Energy, Frequency, Fuel Efficiency, Illuminance, Information Storage, Length, Mass, Power, Pressure, Speed, Temperature, Volume |
| `unit` | `Text` | Yes | - | `` | - |

---

## Module: Pdf

### `getPDFText`

**Title:** Get PDF Text  
**Apple Identifier:** `is.workflow.actions.gettextfrompdf`  

Get text from PDF.

```cherri
getPDFText(pdfFile: AnyContent, richText?: Bool = "false", combinePages?: Bool = "true", headerText?: Text, footerText?: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `pdfFile` | `AnyContent` | Yes | - | `WFInput` | - |
| `richText` | `Bool` | No | `false` | `` | - |
| `combinePages` | `Bool` | No | `true` | `WFCombinePages` | - |
| `headerText` | `Text` | No | - | `WFGetTextFromPDFPageHeader` | - |
| `footerText` | `Text` | No | - | `WFGetTextFromPDFPageFooter` | - |

---

## Module: Settings

### `setFocusMode`

**Title:** Set Focus Mode  
**Apple Identifier:** `is.workflow.actions.dnd.set`  

Set a default focus mode on or off. If setting to on, optionally set until with optional arguments for time or event.

```cherri
setFocusMode(focusMode: Text, until?: Text = "Turned Off", time?: Text, event?: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `focusMode` | `Text` | No | `Do Not Disturb` | `` | Do Not Disturb, Personal, Work, Sleep, Driving |
| `until` | `Text` | No | `Turned Off` | `AssertionType` | Turned Off, Time, I Leave, Event Ends |
| `time` | `Text` | No | - | `Time` | - |
| `event` | `AnyContent` | No | - | `Event` | - |

---

### `setSilentMode`

**Title:** Set Silent Mode  
**Apple Identifier:** `com.apple.ShortcutsActions.SetSilentModeAction`  

Turns silent mode on or off.

```cherri
setSilentMode(state: Number)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `state` | `Number` | Yes | - | `state` | - |

**App Intent:** `SetSilentModeAction` (Bundle: `com.apple.ShortcutsActions`)

---

### `setStageManagerMultitasking`

**Title:** Set Stage Manager Multitasking Mode  
**Apple Identifier:** `com.apple.ShortcutsActions.SetMultitaskingModeAction`  

Set the multitasking mode to Stage Manager. Applies to iPadOS and macOS only.

```cherri
setStageManagerMultitasking(automaticallyShowAndHideDock: Bool, showRecentApps?: Bool)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `automaticallyShowAndHideDock` | `Bool` | No | - | `automaticallyShowAndHideDock` | - |
| `showRecentApps` | `Bool` | No | - | `showRecentApps` | - |

**App Intent:** `SetMultitaskingModeAction` (Bundle: `com.apple.ShortcutsActions`)

---

### `setWindowedMultitasking`

**Title:** Set Windowed Multitasking Mode  
**Apple Identifier:** `com.apple.ShortcutsActions.SetMultitaskingModeAction`  

Set the multitasking mode to windowed. Applies to iPadOS and macOS only.

```cherri
setWindowedMultitasking(automaticallyShowAndHideDock: Bool)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `automaticallyShowAndHideDock` | `Bool` | No | - | `automaticallyShowAndHideDock` | - |

**App Intent:** `SetMultitaskingModeAction` (Bundle: `com.apple.ShortcutsActions`)

---

### `toggleFocusMode`

**Title:** Toggle Focus Mode  
**Apple Identifier:** `is.workflow.actions.dnd.set`  

Toggle a focus mode.

```cherri
toggleFocusMode(focusMode: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `focusMode` | `Text` | No | `Do Not Disturb` | `` | Do Not Disturb, Personal, Work, Sleep, Driving |

---

## Module: Shortcuts

### `createShortcutLink`

**Title:** Create Shortcut Link  
**Apple Identifier:** `com.apple.shortcuts.CreateShortcutiCloudLinkAction`  
```cherri
createShortcutLink(shortcut: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `shortcut` | `AnyContent` | Yes | - | `shortcut` | - |

**App Intent:** `CreateShortcutiCloudLinkAction` (Bundle: `com.apple.shortcuts`)

---

### `openShortcut`

**Title:** Open Shortcut  
**Apple Identifier:** `com.apple.shortcuts.OpenWorkflowAction`  

Open a shortcut in the Shortcuts app.

```cherri
openShortcut(shortcutName: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `shortcutName` | `Text` | Yes | - | `` | - |

---

### `run`

**Title:** Run Shortcut  
**Apple Identifier:** `is.workflow.actions.runworkflow`  

Run a shortcut from this shortcut, with optional input.

```cherri
run(shortcutName: Text, input?: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `shortcutName` | `Text` | Yes | - | `WFWorkflowName` | - |
| `input` | `AnyContent` | No | - | `WFInput` | - |

---

### `runSelf`

**Title:** Run Self  
**Apple Identifier:** `is.workflow.actions.runworkflow`  

Run the current Shortcut with optional output.

```cherri
runSelf(output: AnyContent)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `output` | `AnyContent` | Yes | - | `WFInput` | - |

---

## Module: Text

### `containsText`

**Title:** Contains Text  
**Apple Identifier:** `is.workflow.actions.text.match`  

Uses Match Text to check if text is within subject.

```cherri
containsText(subject: Text, text: Text, caseSensitive?: Bool = "true")
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `subject` | `Text` | Yes | - | `text` | - |
| `text` | `Text` | Yes | - | `WFMatchTextPattern` | - |
| `caseSensitive` | `Bool` | No | `true` | `WFMatchTextCaseSensitive` | - |

---

### `joinText`

**Title:** Join Text  
**Apple Identifier:** `is.workflow.actions.text.combine`  
**Returns:** `Text`  

Join text by a combiner.

```cherri
joinText(text: AnyContent, glue?: Text = "\n") -> Text
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `text` | `AnyContent` | Yes | - | `text` | - |
| `glue` | `Text` | No | `
` | `` | - |

---

### `splitText`

**Title:** Split Text  
**Apple Identifier:** `is.workflow.actions.text.split`  
**Returns:** `List<AnyContent>`  

Split text by a separator.

```cherri
splitText(text: Text, separator?: Text = "\n") -> List<AnyContent>
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `text` | `Text` | Yes | - | `text` | - |
| `separator` | `Text` | No | `
` | `` | - |

---

### `transcribeText`

**Title:** Transcribe Audio  
**Apple Identifier:** `com.apple.ShortcutsActions.TranscribeAudioAction`  
**Returns:** `Text`  

Transcribes text from the provided audio.

```cherri
transcribeText(audio: AnyContent) -> Text
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `audio` | `AnyContent` | Yes | - | `audioFile` | - |

**App Intent:** `TranscribeAudioAction` (Bundle: `com.apple.ShortcutsActions`)

---

## Module: Web

### `addToReadingList`

**Title:** Add to Reading List  
**Apple Identifier:** `is.workflow.actions.readinglist`  

Add a link to the reading list.

```cherri
addToReadingList(url: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `url` | `Text` | Yes | - | `` | - |

---

### `openCustomXCallbackURL`

**Title:** Open Custom X-Callback URL  
**Apple Identifier:** `is.workflow.actions.openxcallbackurl`  
```cherri
openCustomXCallbackURL(url: Text, successKey?: Text, cancelKey?: Text, errorKey?: Text, successURL?: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `url` | `Text` | Yes | - | `WFXCallbackURL` | - |
| `successKey` | `Text` | No | - | `WFXCallbackCustomSuccessKey` | - |
| `cancelKey` | `Text` | No | - | `WFXCallbackCustomCancelKey` | - |
| `errorKey` | `Text` | No | - | `WFXCallbackCustomErrorKey` | - |
| `successURL` | `Text` | No | - | `WFXCallbackCustomSuccessURL` | - |

---

### `url`

**Title:** URL  
**Apple Identifier:** `is.workflow.actions.url`  

Create a URL value.

```cherri
url(url: Text)
```

| Parameter | Type | Required | Default | Wire Key | Choices |
|-----------|------|----------|---------|----------|---------|
| `url` | `Text` | Yes | - | `` | - |

---

