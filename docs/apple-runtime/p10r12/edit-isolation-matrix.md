# P10R12 Edit Isolation Matrix (Control Regions & Complex Parameters)

**Generated:** `2026-11-15T05:11:58Z`  
**Compiler Binary SHA-256:** `7407548dd6a768c86414ff1495a14d3cfb2ee669bd1e6bd72a08e59a01fe8bad`  
**Verdict:** **`TARGET_EDIT_ISOLATION_PASS`** across both Legacy and Cherri v2 dialects.

---

## 1. P10R12 Targeted Control-Region & Complex-Parameter Edit Tests

| Test Function | Edit Scenario | Mutated Native Field(s) | Unrelated Deltas | Status |
|---|---|---|---:|---|
| `TestP12_EDIT_01_ReplaceSingleParameterIsolated` | comment parameter inside workflow with conditional region | `WFCommentActionText` | 0 | `PASS` |
| `TestP12_EDIT_02_EditConditionOperatorAndValueIsolated` | conditional region header comparator & value | `WFCondition (== -> !=) + WFConditionalActionString` | 0 | `PASS` |
| `TestP12_EDIT_03_EditMenuPromptAndCaseTitleIsolated` | choosefrommenu prompt and one case label | `WFMenuPrompt + single WFMenuItemTitle` | 0 | `PASS` |
| `TestP12_EDIT_04_EditRepeatCountIsolated` | repeat.count loop header literal | `WFRepeatCount (3 -> 9)` | 0 | `PASS` |
| `TestP12_EDIT_05_EditPromptMultilineBoolAndDefaultIsolated` | prompt() text and multiline boolean flag | `WFAskActionPrompt + WFAllowsMultilineText` | 0 | `PASS` |
| `TestP12_EDIT_06_EditListItemAddReorderIsolated` | list() items add + reorder | `WFItems` | 0 | `PASS` |
| `TestP12_EDIT_07_EditInterpolatedTextWithBracesAndEmojiIsolated` | text() UTF-16 interpolation around literal braces, RTL Hebrew, and emoji | `WFTextActionText (string + UTF-16 attachmentsByRange)` | 0 | `PASS` |
| `TestP12_EDIT_08_DuplicateReorderRemoveNaturalActionPreservesAdjacentFallback` | remove & duplicate natural action around third-party fallback | `workflow action ordering/count around native.action` | 0 | `PASS` |
| `TestP12_CF_22_ControlRegionEditAfterBoundaryMove` | move natural comment into balanced conditional region across boundary | `action position + balanced GroupingIdentifier` | 0 | `PASS` |

- Plus all **`14`** P10R11 targeted action edits (`P11-WIRE-09`, `P11-WIRE-21..33`), **`5`** adversarial reorder/insert/replace/duplicate/tamper operations (`P11-WIRE-35..38,40`), and **`TestP11_WIRE_39`** boundary move verified passing in the shared Go suite.
