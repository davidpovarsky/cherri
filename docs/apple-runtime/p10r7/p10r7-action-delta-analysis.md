# Cherri P10R7 — Action Count Delta & Control-Flow Reduction Analysis

**Compiler Binary SHA-256:** `316e4bc858c2daddc658dd95fa645f3fceeab83d0d2ce5da45ed02916b0c2e2c`  
**Audit Scope:** 38 real-world Apple Shortcuts (`001`–`038`), **27,652** total source actions  

---

## 1. Executive Delta Reduction (`+6,703 -> +6`)

| Metric | P10R6 Baseline | P10R7 Final | Reduction / Improvement |
| --- | ---: | ---: | --- |
| **Total Source Actions** | 27,652 | 27,652 | Fixed baseline |
| **Total Compiled Actions** | 34,355 | 27,658 | **-6,697 actions** |
| **Net Corpus Action Delta** | **+6,703** (+24.24%) | **+6** (+0.022%) | **-99.91% action bloat** |
| **`STRICT_ACTION_SEQUENCE_PASS`** | 3 / 38 | **21 / 38** | **+18 workflows** |
| **`CONTROL_FLOW_PASS`** | 3 / 38 | **21 / 38** | **+18 workflows** |

---

## 2. Why P10R6 Inflated by +6,703 Actions (And How P10R7 Eliminated It)

1. **Empty Control-Flow `nothing()` Padding (`collectEndStatement`, `collectMenuItem` in `parser.go`):**
   - P10R6 unconditionally injected `is.workflow.actions.nothing` into empty `if`/`otherwise`/`menu`/`repeat`/`for` branches and after closed blocks (~6,300 synthetic `nothing` actions across large workflows like `012`, `018`, `021`, `025`, `027`, `035`).
   - P10R7 removed unconditional synthetic `addNothing()` padding in `parser.go` (`L1528-1595`), restoring exact block membership for all 21 sequence-clean shortcuts.
2. **Unreferenced `gettext` / `number` / `dictionary` / `math` Constant Drop & Synthetic Wrapping (`decompile.go`):**
   - P10R6 dropped unreferenced pure-value actions (`text`, `number`, `dictionary`, `math`, `calculateexpression`) or wrapped standalone `gettext` values incorrectly, losing structural alignment.
   - P10R7 routes unreferenced `gettext` (`emitUnreferencedTextValue`), `number`, `dictionary`, `math`, and `calculateexpression` instances cleanly so standalone/effectful occurrences survive without bloating unrelated assignments, and guards `skipDecompAction` against collapsing live value-carrying actions.
3. **No-Input `setvariable` Synthetic `nothing()` Pairs (`decompVariable` in `decompile.go`):**
   - In P10R6, `is.workflow.actions.setvariable` without `WFInput` ("Clear Variable") decompiled to `@var = nil`, which recompiled to `nothing()` + `setvariable`, adding two actions or corrupting variable identity across 10 workflows.
   - P10R7 routes `SetVariableIdentifier` with `WFInput == nil && WFVariableName != nil` through `rawAction("is.workflow.actions.setvariable", { "WFVariableName": "..." })` (`decompile.go:1398-1404`), eliminating phantom `nothing()` actions while preserving variable registration in `parser.go`.
4. **Menu `{var}` Interpolation & Leading Blank-Line Parser Glitch (`shortcutgen.go`, `shared_control.go`, `control.go`, `parser.go`):**
   - Fixed `{var}` interpolated menu items emitting literal Go map strings `"{{map[..."` instead of `WFTextTokenString` (`menuItemValueAndTitle`), and fixed `firstChar()` resetting `char = 0` (`parser.go:1987`) + skipping `#include <actions/basic>` (`actions_std.go:2929`) so all 38 decompiled `.cherri` files compile cleanly under `--legacy`.
5. **`appendVariable` Fidelity Routing (`decompile.go`, `raw_actions.go`):**
   - Cherri's `@var += val` only emits `is.workflow.actions.appendvariable` when `isArrayVariable(var)` holds (`shortcutgen.go:181`), and drops `UUID` on `appendvariable`. Routing `appendvariable` via `rawAction` preserves all 254 corpus `appendvariable` actions and downstream `UUID` references without extra `list()` initializers.

---

## 3. Exact Remaining `+6` Action-Family Delta (`27,652 -> 27,658`)

Every single action difference in the final build (`316e4bc8...`) is accounted for by summing `docs/apple-runtime/p10r7/p10r7-sanitized-diff-matrix.json`:

| Action Identifier (`WFWorkflowActionIdentifier`) | Added (`+`) | Removed (`-`) | Net Delta | Affected Specimens | Technical Root Cause |
| --- | ---: | ---: | ---: | --- | --- |
| `is.workflow.actions.setvariable` | +46 | 0 | **+46** | `012 (+2), 018 (+6), 021 (+6), 022 (+4), 024 (+1), 025 (+6), 027 (+3), 030 (+1), 034 (+1), 035 (+14), 037 (+2)` | Decompiler `uninitVars` safety pass emits `@Var = nil` declarations when a variable is read or appended before its first `setvariable` in linear scan order (ensures legacy compiler symbol definition). |
| `is.workflow.actions.nothing` | +45 | -1 | **+44** | `+`: `012 (+3), 018 (+6), 021 (+6), 022 (+4), 025 (+6), 027 (+3), 034 (+1), 035 (+14), 037 (+2)`; `-`: `036 (-1)` | `+44` paired directly with `@Var = nil` pre-declarations (`+1` standalone in `012`, `-1` folded in `036`). |
| `is.workflow.actions.getvalueforkey` | 0 | -83 | **-83** | `002 (-1), 012 (-1), 014 (-2), 016 (-15), 024 (-15), 030 (-15), 031 (-2), 035 (-28), 036 (-1), 037 (-2), 038 (-1)` | Inline dictionary subscript folding (`dict['key']`) merges standalone `getvalueforkey` actions when referenced inline. Wait-safe for DSL ergonomics, intentionally non-1:1 for wire parity. |
| `is.workflow.actions.calculateexpression` | 0 | -11 | **-11** | `012 (-1), 022 (-10)` | `decompExpression` lowers `calculateexpression` into Cherri arithmetic expressions, which legacy compile re-emits as `is.workflow.actions.math` (`+11`, net `0`). |
| `is.workflow.actions.math` | +11 | 0 | **+11** | `012 (+1), 022 (+10)` | Counterpart to `calculateexpression: -11` lowering above. |
| `is.workflow.actions.conditional` | 0 | -1 | **-1** | `035 (-1)` | Source specimen `035` contains a malformed duplicate closing `End If` (`WFControlFlowMode=2`) targeting an already-closed `GroupingIdentifier`; Cherri emits balanced control flow. |
| **TOTAL ACROSS CORPUS (`001`–`038`)** | **+102** | **-96** | **+6** | **17 specimens non-zero** | **21/38 exact sequence match; remaining +6 fully mapped** |

---

## 4. Specimen-Level Delta Summary (Non-Zero Specimens Only)

*(The other 21 specimens — `001, 003, 004, 005, 006, 007, 008, 009, 010, 011, 013, 015, 017, 019, 020, 023, 026, 028, 029, 032, 033` — have `actionDelta = 0` and `STRICT_ACTION_SEQUENCE_PASS = PASS`.)*

| ID | Source | Compiled | Delta | Exact Identifier Delta Breakdown |
| :---: | ---: | ---: | ---: | --- |
| `002` | 205 | 204 | -1 | `-1 getvalueforkey` |
| `012` | 2,474 | 2,478 | +4 | `+2 setvariable`, `+3 nothing`, `-1 getvalueforkey`, `-1 calculateexpression`, `+1 math` |
| `014` | 519 | 517 | -2 | `-2 getvalueforkey` |
| `016` | 850 | 835 | -15 | `-15 getvalueforkey` |
| `018` | 2,683 | 2,695 | +12 | `+6 setvariable`, `+6 nothing` |
| `021` | 2,683 | 2,695 | +12 | `+6 setvariable`, `+6 nothing` |
| `022` | 1,831 | 1,839 | +8 | `+4 setvariable`, `+4 nothing`, `-10 calculateexpression`, `+10 math` |
| `024` | 850 | 836 | -14 | `+1 setvariable`, `-15 getvalueforkey` |
| `025` | 2,728 | 2,740 | +12 | `+6 setvariable`, `+6 nothing` |
| `027` | 2,398 | 2,404 | +6 | `+3 setvariable`, `+3 nothing` |
| `030` | 850 | 836 | -14 | `+1 setvariable`, `-15 getvalueforkey` |
| `031` | 957 | 955 | -2 | `-2 getvalueforkey` |
| `034` | 521 | 523 | +2 | `+1 setvariable`, `+1 nothing` |
| `035` | 2,155 | 2,154 | -1 | `+14 setvariable`, `+14 nothing`, `-28 getvalueforkey`, `-1 conditional` (source orphan `End If`) |
| `036` | 521 | 519 | -2 | `-1 getvalueforkey`, `-1 nothing` |
| `037` | 829 | 831 | +2 | `+2 setvariable`, `+2 nothing`, `-2 getvalueforkey` |
| `038` | 210 | 209 | -1 | `-1 getvalueforkey` |
