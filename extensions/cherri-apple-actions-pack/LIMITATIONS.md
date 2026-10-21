# External overlay limitations and follow-up engineering

## Supported and independently compiled

- 20 extension modules are importable together by unmodified original Cherri.
- 48 new action declarations and 54 advanced aliases were accepted in 204/204
  minimal/full example calls with native action identifiers and selected keys checked.
- The pure original Go compiler and official documentation source have not changed.

## NOT proven / NOT ported

1. **15 additional Go-only P10 actions** (some are app intents): `createRemindersList`,
   `editEvent`, `editReminder`, `openBook`, `playAudiobook`,
   `playVoiceMemo`, `recordVoiceMemo`, `run`, `searchSpotlight`,
   `setSilentMode`, `startNotesAudioRecording`, `startStopwatch`,
   `stopStopwatch`, `stopVoiceMemoRecording`, `transcribeText`.
   The original upstream has its own working `run` definition; this is not missing
   functionality. Several app intents need a dynamic UUID and an unwrapped
   `AppIntentDescriptor` native dictionary. Do not substitute a static UUID or
   claim a plain action declaration is equivalent.
2. **Advanced filter predicates.** P10's `buildContentItemFilter()` builds
   `WFContentPredicateTableTemplate` from structured predicate specs.
   Ordinary declarations can expose `WFContentItemFilter` but do not automatically
   build that exact native structure. Only simple/missing filter arguments were
   sampled in this pack's tests. Complex filters require dedicated
   serialization with native acceptance tests.
3. **Four deliberately preserved declarations.** `getFileDetail`, `calculate`,
   `statistic`, and enum `calculationOperation` were not overwritten because
   P10 introduced incompatible native keys or changes needing review.
   Preserve upstream's `statistic()` with `WFInput`, not the fork's invalid
   `Input` key.
4. **Enums and optional combinations.** Passing one sample minimal and full
   parameter list for each callable action is not complete coverage of all
   argument permutations and actual OS-based availability.
5. **Native Apple runtime.** No physical iPadOS/iOS 27/macOS runtime execution
   was performed for the entire pack. Some actions are macOS-only, iOS-version
   limited or require the target app/entitlements. A structurally valid plist
   alone is not enough to establish functional correctness.

## Acceptance gates for future expansion

- Keep official upstream source/commit immutable; add external modules only.
- Compare any new action identifier, raw native params and serialized structures
  to a known-good Apple-generated Shortcut sample (not just P10 output).
- Test minimum, maximum and tricky optional combinations, external App Intent
  metadata, filter predicates, variable parameters and repeated action instances.
- Verify each high-risk action on an actual iPad.
- Preserve a machine-readable categorized unsupported list in the pack manifest.
- Never report complete Apple ShortcutKit coverage until the audit proves it.

This pack is an opt-in overlay, NOT a new Cherri dialect and NOT a forked compiler.
