# P10R11 Control Region Matrix

## Architecture Principle: Region Atomicity

In Cherri, control flow blocks are **strictly atomic**:
- A control block (`if/otherwise/end`, `menu/case/end`, `repeat/end`) consists of multiple Apple Actions sharing a `GroupingIdentifier`.
- Promotion to natural Cherri syntax must promote the **entire balanced region together**. Partial promotion (e.g. promoting the header while leaving an `End If` raw) is illegal and strictly blocked.
- If any single action inside a control region cannot be represented cleanly in natural Cherri, the entire region is safely demoted to raw action fallbacks.

---

## 1. Observed Region Families (Corpus Breakdown)

| Control Flow Family | Apple Identifier | Specimen Actions | Block Structure | Invariant Handled |
|---|---|---:|---|---|
| `conditional` | `is.workflow.actions.conditional` | 8,880 | `if / otherwise / end` | GroupingIdentifier & Mode (0=Start, 1=Else, 2=End) matched |
| `choosefrommenu` | `is.workflow.actions.choosefrommenu` | 976 | `menu / case / end` | GroupingIdentifier & Mode (0=Start, 1=Item, 2=End) matched |
| `repeat.each` | `is.workflow.actions.repeat.each` | 264 | `repeat.each / end` | GroupingIdentifier & Mode (0=Start, 2=End) matched |
| `repeat.count` | `is.workflow.actions.repeat.count` | 40 | `repeat(count) / end` | GroupingIdentifier & Mode (0=Start, 2=End) matched |

---

## 2. Specimen `035:723` (Source Invalid Specimen)

- **Issue:** Specimen `035` at action index 723 contains an orphan `End If` marker (`WFControlFlowMode: 2`, `GroupingIdentifier: "07C9A4B8-B549-4328-9121-71EBE3CD08DF"`). The original authoring of this Shortcut in iOS had corrupted block markers: there is no matching Mode 0 Start block.
- **Handling in P10R11 (`TestP11_WIRE_34`):**
  - Cherri detects the unmatched control marker and marks it `SOURCE_CONTROL_FLOW_INVALID`.
  - Rather than silently dropping or crashing, Cherri renders this orphan action cleanly as a `rawAction` / `native.action`.
  - Round-trip re-compilation preserves the exact byte-for-byte original document.
  - This is documented as `SOURCE_CONTROL_FLOW_INVALID` and preserves full fidelity.
