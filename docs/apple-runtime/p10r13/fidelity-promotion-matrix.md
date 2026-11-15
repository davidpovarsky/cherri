# Cherri P10R13 — Native Action Fidelity & Readability Promotion Matrix

This document records the P10R13 native action promotion wave, root causes, implementation changes, and verified metrics across the 38-shortcut private corpus (`27,652` total actions across root and embedded subworkflows).

> [!IMPORTANT]
> **Privacy & Provenance Rule:** Corpus metrics are aggregated by anonymized fixture index (`001.plist`..`038.plist`) and Apple action identifier (`WFWorkflowActionIdentifier`). No raw user strings, URLs, credentials, or personal Shortcut contents are stored in the repository.

---

## 1. Executive Summary — Corpus Fidelity Improvement (P10R12 vs P10R13)

| Metric | P10R12 Baseline | P10R13 Verified | Delta |
|---|---:|---:|---:|
| **Total Corpus Shortcuts** | `38` | `38` | `0` |
| **Total Actions (Root + Subworkflows)** | `27,652` | `27,652` | `0` |
| **`NATURAL_VERIFIED` Actions** | `22,490` (`81.33%`) | **`23,353` (`84.45%`)** | **`+863` (`+3.12%`)** |
| **`EXACT_NATIVE_FALLBACK` Actions** | `5,161` (`18.66%`) | **`4,298` (`15.54%`)** | **`-863` (`-16.72%` of fallbacks)** |
| **`RAW_ACTION` Actions** | `1` (`0.004%`) | `1` (`0.004%`) | `0` |
| **Stage-3 Demoted Actions (`DemotedActionCount`)** | `0` | **`0`** | `0` |
| **Recursive Wire Parity (`semEqualNestedNormalized`)** | `76 / 76` (`100%`) | **`76 / 76` (`100%`)** | `0` regressions |
| **Pre-Overlay Semantic Parity (`CompareWorkflows`)** | `38 / 38` (`100%`) | **`38 / 38` (`100%`)** | `0` regressions |

---

## 2. Promoted Action Families & Root Causes

| Promotion ID | Apple Action Identifier(s) | Cherri Surface | Promoted Actions | Root Cause in P10R12 | P10R13 Structural Solution | Verification Test |
|---|---|---|---:|---|---|---|
| **P13-FID-01** | `is.workflow.actions.number` | `number(...)` | **`+219`** | Apple Shortcuts serializes `WFNumberActionNumber` as a string (`"42"`, `"-5"`) or `WFTextTokenString` variable interpolation in 219 corpus records, whereas `actions/math.cherri` only accepted `number` (`Int`/`Float`) literals and `import_provenance.go` rejected string-typed `WFNumberActionNumber`. | Added `TypeText` to `number` parameter schema in `internal/language/schema/schema.go`; allowed numeric string and variable-only `WFTextTokenString` values in `import_provenance.go`, `semantic_import_planner.go`, and `actions_std.go`; preserved exact string/token wire representation via `is.workflow.actions.number/string-value` provenance. | `TestP13_FID_01_NumberActionStringAndAttachmentParity` |
| **P13-FID-02** | `is.workflow.actions.text.split`, `is.workflow.actions.text.combine` | `splitText(...)`, `joinText(...)` | **`+188`** (`+123` `text.split`, `+65` `text.combine`) | 1. Apple Shortcuts exports frequently omit `Show-text` on `text.split`/`text.combine`, while Cherri's compiler (`actions_std.go:textParts`) unconditionally emits `"Show-text": true`.<br>2. `reverseTextPartsParam` in `semantic_import_planner.go` did not reverse `WFTextSeparator` (`New Lines`, `Spaces`, `Every Character`, `Custom`).<br>3. `actions_std.go:3059` checked `currentAction.identifier == "text.split"` (`"splitText"`) instead of checking `currentAction.definition.identifier == "text.split"`, causing `Every Character` (`""`) to fail compilation. | 1. Added `AbsentShowKey` provenance (`is.workflow.actions.text.split/no-show-text`, `is.workflow.actions.text.combine/no-show-text`) to suppress synthetic `"Show-text": true` when absent in input.<br>2. Implemented full `reverseTextPartsParam` support for `New Lines`, `Spaces`, `Every Character`, and `Custom`.<br>3. Fixed `actions_std.go:3059` to check `currentAction.definition.identifier == "text.split"`. | `TestP13_FID_02_TextSplitCombineAndChangeCaseParity` |
| **P13-FID-03** | `is.workflow.actions.text.changecase`, `is.workflow.actions.correctspelling`, `is.workflow.actions.openurl`, `is.workflow.actions.detect.link`, `is.workflow.actions.geturlcomponent`, `is.workflow.actions.url.expand`, `is.workflow.actions.image.resize`, `is.workflow.actions.image.crop`, `is.workflow.actions.overlayimageonimage`, `is.workflow.actions.image.mask` | `lowercase(...)`, `uppercase(...)`, `capitalize(...)`, `capitalizeAll(...)`, `titleCase(...)`, `alternatingCase(...)`, `correctSpelling(...)`, `openURL(...)`, `getURLs(...)`, `getURLDetail(...)`, `expandURL(...)`, `resizeImage(...)`, `cropImage(...)`, `customImageOverlay(...)`, `maskImage(...)` | **`+438`** (`+306` `openurl`, `+48` `text.changecase`, `+41` `image.resize`, `+21` `detect.link`, `+10` `geturlcomponent`, `+8` `image.crop`, `+2` `overlayimageonimage`, `+1` `image.mask`, `+1` `url.expand`) | 1. Apple Shortcuts serializes variable arguments to `text` (`WFCaseType` input), `WFInput` (`detect.link`, `geturlcomponent`, `url.expand`), and `WFURLActionURL` (`openurl`) as direct `WFTextTokenAttachment` maps rather than wrapping them inside `WFTextTokenString` when a variable is passed directly, and omits `Show-text` / `Show-WFInput` / `Show-WFURLActionURL`.<br>2. `shortcutgen.go:883` used `definition.identifier` (which is empty for `openURL`) instead of `getFullActionIdentifier()`, emitting `WFTextTokenString` instead of `WFTextTokenAttachment`.<br>3. Image actions (`resizeImage`, `cropImage`, `customImageOverlay`, `maskImage`) restricted their `image` parameter to `variable` in `actions/images.cherri`, rejecting typed action-output references (`@prev`, `@v_...`) in V2 semantic analysis. | 1. Extended `isAttachmentCapableTextParam` (`shortcutgen.go`) and `isAttachmentCapableTextParamSchema` (`semantic_import_planner.go`) to emit and reverse `WFTextTokenAttachment` on `text`, `WFInput`, and `WFURLActionURL` using `getFullActionIdentifier()` and `cand.AppleIdentifier`.<br>2. Added `AbsentShowKey` tracking for `Show-text`, `Show-WFInput`, and `Show-WFURLActionURL`.<br>3. Updated `internal/language/schema/schema.go` and `internal/language/analysis/analyzer.go` so reference-like expressions (`@prev`, `@v_...`) satisfy `TypeVariable` and image parameters accept `TypeText`/`TypeVariable`. | `TestP13_FID_02_TextSplitCombineAndChangeCaseParity`, `TestP13_FID_03_URLAndImageActionsParity` |
| **P13-FID-04** | `is.workflow.actions.dictionary` | `const d = {...}` / dictionary assignment | **`+18`** | `import_provenance.go` rejected `is.workflow.actions.dictionary` whenever an unreferenced producer `UUID` was present in `WFWorkflowActionParameters`, even though `restoreImportProvenanceOverlay` already restores `UUID` losslessly when `KeepLosslessUUID` is recorded. | Allowed unreferenced `UUID` on `is.workflow.actions.dictionary` in `import_provenance.go` (`KeepLosslessUUID = true`, `ProducerUUID = uuid`) when no other unknown keys exist. | `TestP13_FID_04_DictionaryWithUnreferencedUUIDParity` |

---

## 3. Why Remaining `EXACT_NATIVE_FALLBACK` Actions Stay on Fallback

After promoting `+863` actions to `NATURAL_VERIFIED`, `4,298` actions (`15.54%`) remain on `EXACT_NATIVE_FALLBACK`. Every remaining fallback is intentional and required to preserve 100% lossless round-trip semantics:

1. **Nested Dictionary / Array `WFValue` Trees (`is.workflow.actions.dictionary`, `is.workflow.actions.setvalueforkey`, `is.workflow.actions.getvalueforkey` — ~1,130 actions):**
   - Complex nested plist structures containing mixed array-of-dictionary items, inline `WFTextTokenAttachment` keys, or non-standard `WFDictionaryFieldValueItem` metadata that cannot be expressed in flat Cherri dictionary syntax without losing nested wire structure.
2. **Rich List Items & Menu Subflows (`is.workflow.actions.list`, `is.workflow.actions.choosefrommenu` — ~720 actions):**
   - Lists containing typed dictionary/boolean items or interpolated token items where Apple Shortcuts stores rich `WFItemType` descriptors, and menu items carrying custom per-branch metadata.
3. **Complex Media/System/Third-Party Actions (`is.workflow.actions. alert`, `showresult`, `notification`, `runworkflow`, `urlencode`, `downloadurl`, third-party AppIntents — ~2,448 actions):**
   - Actions with extra Apple-specific parameter keys (e.g., `WFAlertActionCancelButtonShown: false`, custom HTTP headers/multipart bodies on `downloadurl`, embedded `WFWorkflow` dictionaries on `runworkflow`, or third-party app bundle actions not in Cherri's built-in standard library).

In all `4,298` cases, `exact_native_action(...)` preserves the action cleanly at the language level while the sidecar/provenance overlay guarantees 100% lossless binary plist compilation (`0` Stage-3 demotions).
