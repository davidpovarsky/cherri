# P10R11 Edit Isolation Matrix

## Overview

The Edit Isolation Matrix tracks the exact delta in compiled native plists when a single natural Cherri statement is edited by the user.

A test achieves `TARGET_EDIT_ISOLATION_PASS` only when:
1. The edited action compiles to the expected changed plist field(s).
2. Exactly 0 unrelated actions differ in the compiled output.
3. The surrounding reference/UUID graph and control flow structures remain consistent.

---

## 1. Targeted Natural Action Edits

| Test ID | Action | Field Edited | Expected Delta | Observed Delta | Unrelated Deltas | Status |
|---|---|---|---|---|---|---|
| `P11-WIRE-09` | `downloadURL` | `showHeaders: true` | `ShowHeaders: true` | `ShowHeaders: true` | 0 | `PASS` |
| `P11-WIRE-21` | `text` | Text literal | `WFTextActionText` | `WFTextActionText` | 0 | `PASS` |
| `P11-WIRE-22` | `comment` | Comment string | `WFCommentActionText` | `WFCommentActionText` | 0 | `PASS` |
| `P11-WIRE-23` | `setVariable` | Source value (`score = Clipboard`) | `WFInput` (token) | `WFInput` (token) | 0 | `PASS` |
| `P11-WIRE-24` | `appendVariable` | Source value (`items += Clipboard`) | `WFInput` (token) | `WFInput` (token) | 0 | `PASS` |
| `P11-WIRE-25` | `setName` | Target name | `WFName` | `WFName` | 0 | `PASS` |
| `P11-WIRE-26` | `replaceText` | Find string | `WFReplaceTextFind` | `WFReplaceTextFind` | 0 | `PASS` |
| `P11-WIRE-27` | `fileRequest` | Endpoint URL | `WFURL` | `WFURL` | 0 | `PASS` |
| `P11-WIRE-28` | `saveFile` | Destination path | `WFFileDestinationPath` | `WFFileDestinationPath` | 0 | `PASS` |
| `P11-WIRE-29` | `prompt` | Prompt text | `WFAskActionPrompt` | `WFAskActionPrompt` | 0 | `PASS` |
| `P11-WIRE-30` | `chooseFromList` | Prompt label | `WFChooseFromListActionPrompt` | `WFChooseFromListActionPrompt` | 0 | `PASS` |
| `P11-WIRE-31` | `conditional` | Condition operator (`!=` -> `==`) | `WFCondition` | `WFCondition` | 0 | `PASS` |
| `P11-WIRE-32` | `repeat.each` | Input collection | `WFInput` | `WFInput` | 0 | `PASS` |
| `P11-WIRE-33` | `choosefrommenu` | Menu title | `WFMenuTitle` | `WFMenuTitle` | 0 | `PASS` |

---

## 2. Adversarial Operations (Structural Safety)

| Test ID | Operation | Safety Mechanism | Outcome | Status |
|---|---|---|---|---|
| `P11-WIRE-35` | Swapping order of two identical-action calls with different UUIDs | Deterministic semantic hash prevents index cross-binding | Actions retain their own distinct parameters; no UUID cross-contamination | `PASS` |
| `P11-WIRE-36` | Inserting a new action in the middle of a workflow | Strict semantic hash match in `applyImportProvenanceToCompiledActions` | Inserted action does not steal provenance from surrounding actions | `PASS` |
| `P11-WIRE-37` | Replacing an action with another action of same ID | Semantic hash mismatch prevents stale metadata transfer | Replacement compiles cleanly with its own native identity | `PASS` |
| `P11-WIRE-38` | Duplicating a natural statement in source | New independent AST node emitted | Fresh UUID allocated without collision | `PASS` |
| `P11-WIRE-40` | Tampering with provenance header hashes | Hash verification fails closed | Stale metadata ignored; clean native compilation | `PASS` |

---

## 3. Real Specimen Audit Upgrade

In P10R10, `editIsolationAllPassed` was marked `false` due to a flawed test selection (`001` had no top-level comment).
In P10R11:
- `001`: Evaluated with statement edit (`NOT_APPLICABLE` for top-level comment; statement edit passes with exact isolation).
- `012`, `027`, `035`: Existing natural comment edits confirmed 100% isolated (single action changed, 0 header drift, 0 count delta).
- **Final Verdict:** `TARGET_EDIT_ISOLATION_PASS`.
