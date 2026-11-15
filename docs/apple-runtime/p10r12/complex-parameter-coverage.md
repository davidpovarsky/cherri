# P10R12 Typed Complex Parameter Coverage & Remaining Fail-Closed Fallbacks

**Generated:** `2026-11-15T05:11:58Z`  
**Compiler Binary SHA-256:** `7407548dd6a768c86414ff1495a14d3cfb2ee669bd1e6bd72a08e59a01fe8bad`  
**Dialect Parity:** Identical natural complex parameter coverage across Legacy and V2 (`164` distinct action identifiers with complex parameter shapes promoted).

---

## 1. Complex Native Parameter Shapes Verified Inside `NATURAL_VERIFIED` Actions

| Parameter / Codec Shape | Natural Action Observations (Per Dialect) |
|---|---:|
| `plain_dict` | 22,490 |
| `WFTextTokenAttachment` | 10,029 |
| `list` | 4,690 |
| `WFTextTokenString_with_attachments` | 4,418 |
| `Aggrandizements` | 4,337 |
| `WFDictionaryFieldValue` | 89 |
| `WFContentPredicateTableTemplate` | 55 |
| `serialized:WFNumberSubstitutableState` | 2 |

- Total third-party actions preserved untouched (`THIRD_PARTY_NO_CANONICAL_DEF`): **`137 / 137`** exact in Legacy and **`137 / 137`** exact in V2.
- Forward `ActionOutput` reference bug fixed cleanly via `rejectUnseededForwardActionOutputCandidates` (e.g. `027:1556 -> 1557` fails closed at Stage 1 instead of producing forward-declared `@var`/`const` compile failures), plus CLI `--import` determinism Parity (`args.Args["import"]` isolated identically in CLI and `go test`).

---

## 2. Top Remaining `UNREPRESENTABLE_PARAM` Actions (`4,273` Total, Down from `5,836`)

| Action Identifier | Remaining Fallbacks |
|---|---:|
| `is.workflow.actions.dictionary` | 410 |
| `is.workflow.actions.runworkflow` | 316 |
| `is.workflow.actions.text.replace` | 272 |
| `is.workflow.actions.text.split` | 271 |
| `is.workflow.actions.number` | 219 |
| `is.workflow.actions.getitemfromlist` | 218 |
| `is.workflow.actions.downloadurl` | 202 |
| `is.workflow.actions.url` | 183 |
| `is.workflow.actions.calculateexpression` | 159 |
| `is.workflow.actions.base64encode` | 156 |
| `is.workflow.actions.setvariable` | 147 |
| `is.workflow.actions.text.combine` | 135 |
| `is.workflow.actions.image.resize` | 133 |
| `is.workflow.actions.openurl` | 131 |
| `is.workflow.actions.documentpicker.open` | 81 |
| `is.workflow.actions.list` | 77 |
| `is.workflow.actions.documentpicker.save` | 76 |
| `is.workflow.actions.math` | 71 |
| `is.workflow.actions.filter.files` | 67 |
| `is.workflow.actions.gettext` | 61 |

## 3. Top Remaining `UNREPRESENTABLE_PARAM` Parameter Shapes

| Identifier.Key:Shape | Count |
|---|---:|
| `is.workflow.actions.dictionary.WFItems:WFDictionaryFieldValue` | 408 |
| `is.workflow.actions.text.replace.WFInput:WFTextTokenString` | 272 |
| `is.workflow.actions.text.split.text:WFTextTokenAttachment` | 271 |
| `is.workflow.actions.runworkflow.WFInput:WFTextTokenAttachment` | 241 |
| `is.workflow.actions.getitemfromlist.WFInput:WFTextTokenAttachment` | 218 |
| `is.workflow.actions.calculateexpression.Input:WFTextTokenString` | 159 |
| `is.workflow.actions.base64encode.WFInput:WFTextTokenAttachment` | 156 |
| `is.workflow.actions.runworkflow.WFWorkflowName:WFTextTokenAttachment` | 141 |
| `is.workflow.actions.url.WFURLActionURL:WFTextTokenString` | 138 |
| `is.workflow.actions.image.resize.WFImage:WFTextTokenAttachment` | 133 |
| `is.workflow.actions.text.combine.text:WFTextTokenAttachment` | 131 |
| `is.workflow.actions.image.resize.WFImageResizeWidth:WFTextTokenAttachment` | 115 |
| `is.workflow.actions.openurl.WFInput:WFTextTokenAttachment` | 109 |
| `is.workflow.actions.image.resize.WFImageResizeHeight:WFTextTokenAttachment` | 102 |
| `is.workflow.actions.downloadurl.WFURL:WFTextTokenString` | 95 |
| `is.workflow.actions.downloadurl.WFHTTPHeaders:WFDictionaryFieldValue` | 92 |
| `is.workflow.actions.runworkflow.WFWorkflow:WFTextTokenAttachment` | 83 |
| `is.workflow.actions.downloadurl.WFJSONValues:WFDictionaryFieldValue` | 82 |
| `is.workflow.actions.list.WFItems:list` | 77 |
| `is.workflow.actions.documentpicker.save.WFInput:WFTextTokenAttachment` | 76 |
