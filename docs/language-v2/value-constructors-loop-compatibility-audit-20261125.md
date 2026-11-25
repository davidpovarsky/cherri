# Loop reference and value-constructor compatibility audit — 2026-11-25

Execution: real subagent `/root/compatibility_fix`, isolated branch
`agent/cherri-differential-compat-20261125`, from
`e432f7b9f5f663cc56ba8041e63d2e90317fc756`. Root task: Asana
`1220897415739298`. No original/upstream source was modified.

## Reproduced failures

Current upstream `98d5191ae121a6bcf663ed0e55495f4ee4c7a6e2` accepts
`repeat i for 5 { @sum += i }`; the fork rejected bare `i`. The fork's previously
accepted `@i` variant also emitted an assignment from `i` without the required
Math action. Merely accepting the syntax would therefore not repair the result.
The native upstream graph supplies the expected `sum + i` producer followed by
an assignment consuming that exact producer UUID.

V2 accepted `color(red, 0.5, 0.75, 1)` for an immutable binding, action result,
or mutable variable, and silently serialized `redComponent: 0`. Original
`collectColorValue` declares every component literal-only. Numeric text was
also silently parsed as a number in V2 while the original rejects it.

V2 accepted `qty()`, `qty(5)`, extra arguments, nonnumeric magnitudes, nontext
units and bare runtime unit references. Reordered named arguments produced
`Magnitude: "sec", Unit: 5`. Original constructors reject the wrong types and
extra arguments; original and fork Legacy both panicked for missing quantity
arguments and short color argument lists instead of applying their definitions.

## Repair boundaries

- Bare loop aliases and the fork's existing `@` spelling both remain accepted.
  Ordinary mutable variables still require `@`, including a reassigned former
  loop alias.
- Compound references resolve their definition type, preserve Float and numeric
  action/expression results, and assign from the computed output. The shared
  output-name path follows current upstream's plain-assignment distinction.
  Array appends keep the reference itself; text references preserve both
  concatenation operands. Alias traversal is cycle bounded.
- A shared V2 intrinsic binder validates positional/named arguments, requiredness,
  duplicate labels and unknown labels. Both analysis and lowering use it.
- Color remains a numeric CGColor representation with the existing four fields
  and defaults. Unsupported runtime/nonfinite/nonnumeric components diagnose;
  no dynamic Apple color encoding was introduced. Legacy still uses its existing
  `WFColorValue` serializer, including omission of zero RGB fields.
- Quantity keeps the previously repaired typed magnitude carrier and canonical
  encoder. Magnitudes remain numeric literals or references; units keep the
  original text-literal/template boundary. No new unit whitelist, nonempty-unit
  rule or color range constraint was imposed.
- Named quantity binding retains authored evaluation order and the prior
  capture-before-later-write behavior. Date-duration evidence resolves the same
  binding plan. Legacy intrinsics now invoke existing requiredness checks and
  materialize declared defaults before indexing arguments.

## Verification

The baseline RED focused tests were executed before production edits. The
current-upstream CLI probes and serialized outputs are preserved separately
from those tests. GREEN coverage includes:

- All four numeric compound operators with both loop reference spellings;
  exact native variable operands, the Math action and producer UUID consumption.
- Integer, Float, action, expression and copied numeric references; array append
  and text concatenation controls; nested loops and ordinary mutable syntax.
- Public analyzer diagnostics and refusal to return plist bytes for invalid
  constructors, all four dynamic color slots, named binding, literal color
  wire values, defaults, arbitrary/empty units and unit text templates.
- Direct lowerer rejection without an analyzer for malformed intrinsic input.
- The five existing dynamic-quantity tests, including catalog coverage and
  argument snapshot identity.
- Existing `TestP15DateComparisonMigrationCLIParity`, plus a named date quantity.
- **38 final CLI executions** with exact exit/artifact expectations. The two
  Legacy loop spellings each emit a verified native sum/assignment graph.
- `go build` and `git diff --check` passed.

Commands and complete logs:

```text
go test -run 'Test(LegacyLoopReference|LegacyCompoundNumeric|ValueConstructors|Quantity|P15.*Date.*Quantity)' -count=1 -v . ./internal/language/lower
go test -run 'Test(LegacyCompoundText|ValueConstructorsNamedQuantity|P15DateComparisonMigrationCLIParity)' -count=1 -v .
```

Evidence root on Oracle:
`/mnt/development/projects/evidence/cherri-differential-30-20261125/compatibility`.

| Receipt | SHA-256 |
|---|---|
| `red-receipt.json` | `d1d415747952881ffed3341bad06ed550b47590308c2ebd730ae3142a0faefa0` |
| `qty-red-receipt.json` | `9813ca289b933fa59df1e130989ac35d483177a27169c724d0523916abaea7a1` |
| `tests-red.log` | `274edca2957c8b1d0eac8be366076719acfa2086897d4a28f9ac285425090237` |
| `tests-green.log` | `e3fc0f6a437e2aad8be8c50f005f6821e6b1ea047fb67afc953ed0a67a34565f` |
| `extended-controls.log` | `7d772224cb8db0bc92537bb2c1cfea60b49018ea4d4ae176ea342a0c620c5350` |
| `green-receipt.json` | `feddda9d2610856d2653b9126999414706d0fbe118cbd13cb425279937aa5535` |

Integration/full-suite/native execution remain assigned to the root coordinator.
These focused Linux results do not claim Apple execution or project closure.
