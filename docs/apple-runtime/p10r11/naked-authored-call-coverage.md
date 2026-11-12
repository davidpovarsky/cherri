# P10R11 Naked Authored Call Coverage

## Verification Standard

Tests `TestP11_WIRE_41` and `TestP11_WIRE_42` compile clean authored Legacy and Cherri v2 source code without:
- `#workflowheader`
- `native.workflow`
- `__CherriImportProvenance`
- Any imported metadata or UUID mappings.

This proves that the Cherri compiler produces correct Apple action identifiers, parameter keys, and serialized type hierarchies natively from scratch, rather than relying on imported provenance.

---

## Canonical Authored Call Matrix

| Action | Apple Identifier | Legacy Syntax | V2 Syntax | Native Wire Evidence | Wire Status |
|---|---|---|---|---|---|
| `text` | `is.workflow.actions.gettext` | `text("sample")` | `text("sample")` | Observed in 38 specimen corpus | `PASS` |
| `comment` | `is.workflow.actions.comment` | `comment("note")` | `comment("note")` | Observed in 38 specimen corpus | `PASS` |
| `downloadURL` | `is.workflow.actions.downloadurl` | `downloadURL("https://...", nil, true)` | `downloadURL("https://...", showHeaders: true)` | Observed in 38 specimen corpus | `PASS` |
| `fileRequest` | `is.workflow.actions.downloadurl` | `fileRequest("https://...", "POST", nil, nil, true)` | `fileRequest("https://...", method: "POST", showHeaders: true)` | Observed in 38 specimen corpus | `PASS` |
| `saveFile` | `is.workflow.actions.documentpicker.save` | `saveFile("a.txt", input)` | `saveFile("a.txt", content: input)` | Observed in 38 specimen corpus | `PASS` |
| `prompt` | `is.workflow.actions.ask` | `prompt("prompt")` | `prompt("prompt")` | Observed in 38 specimen corpus | `PASS` |
| `chooseFromList` | `is.workflow.actions.choosefromlist` | `chooseFromList(in, "prompt")` | `chooseFromList(in, prompt: "prompt")` | Observed in 38 specimen corpus | `PASS` |
| `setName` | `is.workflow.actions.setitemname` | `setName(in, "name")` | `setName(in, name: "name")` | Observed in 38 specimen corpus | `PASS` |
| `replaceText` | `is.workflow.actions.text.replace` | `replaceText("a", "b", in)` | `replaceText("a", replacement: "b", input: in)` | Observed in 38 specimen corpus | `PASS` |
| `getValue` | `is.workflow.actions.getvalueforkey` | `getValue("key", in)` | `getValue("key", input: in)` | Observed in 38 specimen corpus | `PASS` |
| `setVariable` | `is.workflow.actions.setvariable` | `@x = in` | `x = in` | Observed in 38 specimen corpus | `PASS` |
| `appendVariable` | `is.workflow.actions.appendvariable` | `@x += in` | `x += in` | Observed in 38 specimen corpus | `PASS` |

---

## Truthful Coverage Scope & Unverified Combinations

- **Total Actions in Catalog:** 505 actions (generated registry).
- **Observed Unique Actions in Corpus:** 118 distinct Apple identifiers.
- **Formally Verified Authored Combos:** The 12 representative canonical action families above, plus 17 newly tested Apple built-ins, verified against observed Apple plists.
- **Unverified Catalog Entries:** 387 catalog actions have not been observed in real-world specimen executions. They remain classified as `AUTHORED_ACTION_CONTRACT_PARTIAL` until grounded by hardware or corpus samples. Cherri does NOT claim 100% universal verification of all 505 actions.
