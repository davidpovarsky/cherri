# Cherri Debug: Current Date without native Date actions (2026-11-12)

## Source evidence

A user-provided iPadOS 27 shortcut JSON export demonstrates that the Current Date magic variable is valid directly as the input to Set Variable and to Get Time Between Dates. This is native Apple Shortcuts behavior, not an assumption about implicit input chains.

### debugStartTimer(label)

Before: Date (Current Date) -> Set Variable (date output).

After: **one** native Set Variable action, with:
  WFInput.WFSerializationType = WFTextTokenAttachment
  WFInput.Value = {Type: CurrentDate}

### debugStopTimer(label)

Before: Date (Current Date) -> Get Time Between Dates -> Append to Variable -> Get Variable.

After: Get Time Between Dates -> Append to Variable -> Get Variable.
The Get Time Between Dates end date is a single native WFTextTokenString:
  Value.string = U+FFFC
  Value.attachmentsByRange['{0, 1}'] = {Type: CurrentDate}

The start date continues to reference the timer's saved variable using a different WFTextTokenString attachment. No Date output UUID is needed.

## Verified acceptance

- Both Legacy and V2 tests pass:
  TestDebugTimerNativeDateInputsAcrossLegacyAndV2
  TestDebugTimerTwoIndependentLabels
  TestDebugStdlibTimerNativeDateFieldSerialization
- Current iPad API fixture: 89 -> **81** actions; exactly 8 Date actions removed.
- All remaining 81 native action identifiers match the previous file with the eight Date actions omitted.
- Four direct CurrentDate start tokens, four direct CurrentDate stop tokens.
- Three API calls, 29 debug-log appends, no Get Text actions.
- Independent Python plistlib checks found 56 valid ActionOutput references and 0 broken refs.
- The signed Apple AEA1 shortcut is present in tests/Cherri-API-Debug-CURRENTDATE-OPTIMIZED-SIGNED.shortcut.

The user's manually edited export contains 80 actions: this compiler source preserves an additional Get Variable after debugStopTimer to preserve its return-value contract. The optimization scoped here removes Current Date producers only.

Apple on-device execution of this exact new signed build remains to be confirmed by the user. A passing compiler test or valid signature is not equivalent to iPad runtime verification.
