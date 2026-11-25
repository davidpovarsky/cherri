# Dynamic quantity reference audit — 2026-11-25

## Finding

The V2 compiler at `6ce0a3981d2047ba5aec603f185e2d8f922fc0ee` converted a
dynamic quantity magnitude into a Go debug string. The compiler returned success
and emitted a valid plist, but the quantity no longer referenced its numeric
producer. Both the frozen original and independently fetched current upstream
emit a bare native reference dictionary in the same field.

This is a V2 lowering regression. The fork's Legacy path already preserves the
reference through its shared quantity builder.

## Independent original baselines

| Source | Exact revision | Binary SHA-256 |
| --- | --- | --- |
| Frozen original | `bcc0095754e2cc0baaeb4d93dae172b8153b4716` | `32df2e3e8380128039b605616e86602184469b521473fed31d621e8109d419c8` |
| Current `electrikmilk/cherri` HEAD observed on this date | `98d5191ae121a6bcf663ed0e55495f4ee4c7a6e2` | `feda44491d942061ea70353d3120429c3fa5fa1604a2059fa03a24eca56d4ee5` |

Each original was built from a separate clean checkout with Go 1.27.4 on Linux
arm64. The current upstream was fetched by its observed full SHA. Neither
original source tree nor the frozen original branch was modified.

The original source reproducer is:

```cherri
#include 'actions/calendar'
const seconds = 1 + 1
startTimer(qty(seconds, "sec"))
```

The equivalent V2 source uses `let seconds = 1 + 1` and the built-in action
catalog without the Legacy include directive. Compilation used
`--skip-sign --debug --derive-uuids --no-ansi`; the fork additionally selects
`--mode=legacy` or `--mode=v2`. These original compilers can emit their
`_unsigned.shortcut` beside the source despite a requested output path, so
inspection uses the actual emitted artifact.

Both originals and fork Legacy emitted this structure, with the actual UUID of
the preceding arithmetic action:

```json
{
  "WFSerializationType": "WFQuantityFieldValue",
  "Value": {
    "Magnitude": {
      "Type": "ActionOutput",
      "OutputUUID": "<preceding producer UUID>",
      "OutputName": "seconds"
    },
    "Unit": "sec"
  }
}
```

Baseline V2 instead emitted:

```json
{
  "WFSerializationType": "WFQuantityFieldValue",
  "Value": {
    "Magnitude": "&{ActionOutput <producer UUID> seconds []}",
    "Unit": "sec"
  }
}
```

The mutable case similarly became `"&{Variable  seconds []}"`. Literal
magnitudes remained strings such as `"2"`, which explains why a literal-only
fixture passed. A dynamic magnitude is a **bare reference**: wrapping it in
`WFTextTokenAttachment` would differ from the independently compiled originals.

Frozen original positive probes covered arithmetic, a mutable number, an action
result, and a literal. Current upstream positive probes covered arithmetic,
mutable storage, input and clipboard globals, a device property, and a
dictionary key. Current upstream adds `PropertyUserInfo: 0` to its property
aggrandizement; the frozen/fork shared codec does not. That existing property
metadata difference is retained as a separate observation, not attributed to
the quantity repair.

## Root cause and repair

The old `lowerCall` implementation of `qty` called `lowerExpression`, then
applied `fmt.Sprintf("%v", ...)` to both arguments. Dynamic magnitudes lower to
reference carriers, so formatting them destroyed their native identity before
the shared backend had a chance to serialize them.

The repair introduces a small `backend.QuantityValue` carrier containing the
evaluated magnitude and unit as semantic values. It remains typed until the
existing backend adapters call `makeQuantityFieldValue`. That existing shared
builder already chooses the canonical bare reference representation through
`variableValueWithSerialization(..., "")`. The original `action.go` builder
and action definitions are unchanged.

The same carrier is recognized by the date-condition paths that previously
recognized the eagerly serialized quantity map. Their native conversion calls
the same shared builder. Existing opaque native maps retain their prior route.

Argument capture also traverses quantity magnitude and unit. If a later argument
mutates the magnitude's binding, the earlier quantity captures its original
value before the mutation. A regression fixture uses
`makeSizedDiskImage(..., diskSize: qty(seconds, "MB"), encrypt: measure(...))`
and checks the actual producer and action order. Nested producers and
`native.action` numeric results retain one producer and a reference to its
actual UUID.

This changes no public catalog schema, native action identifier, unit enum,
fingerprint domain, or Legacy action signature.

## Why the existing tests missed it

At the baseline revision:

- `tests/qty-color.cherri` exercised `qty(5, "min")`.
- The generated catalog fixture in `p10_structural_roundtrip_test.go` also
  constructed a literal quantity.
- `verifyCatalogActionParameterContract` chose only the first eligible dynamic
  parameter, explicitly skipped `qty` and enum parameters, then stopped.
- That dynamic matrix compiled through Legacy. Removing its quantity skip alone
  would still leave this V2 defect untested.
- The date-quantity positive fixtures were literal. Dynamic date-quantity cases
  primarily checked analyzer rejection, not an accepted timer magnitude.
- The native debug timer tests exercised the debug timer macro rather than
  the `startTimer` quantity field.

The new `TestQuantityCatalogDynamicParameters` iterates **every** quantity
parameter in the live catalog and compiles both dialects. It currently covers:

| Action | Parameter | Native field |
| --- | --- | --- |
| `adjustDate` | `unit` | `WFDuration` |
| `makeSizedDiskImage` | `diskSize` | `ImageSize` |
| `seek` | `timeInterval` | `WFTimeInterval` |
| `startTimer` | `duration` | `WFDuration` |

Every result must contain the bare reference, point to exactly one earlier
arithmetic producer, and preserve the native field and unit. New quantity
parameters are included automatically. The unrelated enum contract is unchanged.

## Verification and evidence

The actual baseline failed the new V2 dynamic shape checks while corresponding
Legacy and literal cases passed. After the catalog setup was corrected, all four
catalog quantity cases independently failed on baseline V2 with the same
stringification defect. These original failures remain preserved.

The final focused command passed:

```text
go test -run 'Quantity|T04|TestP15C01ArgumentValueSnapshot' -count=1 -v .
```

It covers seven dynamic reference families in both modes, zero/integer/decimal/
negative literals, all four catalog quantity parameters in both modes, nested
and native producer identity, quantity argument capture, and existing date
quantity / opaque value / argument snapshot regressions.

The affected packages passed separately:

```text
go test ./internal/language/lower ./internal/language/backend ./internal/language/ir
```

The fixed CLI built successfully. Both new Cherri fixtures compiled successfully
with it, and all three quantity magnitudes in each artifact were dictionaries.
The original `tests/qty-color.cherri` fixture was preserved unchanged.

[The bounded verification record](evidence/dynamic-quantity-20261125/verification.json)
contains source hashes, exact baseline pins, original/failing/fixed quantity
shapes, command return codes, receipt hashes and artifact hashes. Full original
artifacts remain under
`/mnt/development/projects/evidence/cherri-differential-30-20261125/`:

- `original/`: pristine build receipts and original/fork comparison cases;
- `quantity/`: actual baseline failures, intermediate failed setup/build,
  final focused/affected-package receipts, fixed build, and compiled Cherri
  fixture artifacts.

The fixed candidate binary at this verification point has SHA-256
`f574df8ee305dd8c92068404633f9a68ca6853e98461aa1e6e72f0a45a5b6bf5`.
Its implementation source hashes are recorded directly because the focused
tests and build preceded the coherent implementation commit.

These results establish compiler and native plist behavior. Apple native
execution, the wider differential corpus, integration, and release acceptance
are separate work owned by the coordinating task. No native runtime outcome is
inferred from a successful Linux compile or from the Go regression tests.

## Root fixture include contract follow-up

The first integrated whole-Go run stopped in `TestCherriNoSign` at the added
`tests/qty-dynamic.cherri`: its explicit `actions/calendar` include redeclared
`eventDetail`. The root fixture runner already loads all standard categories,
then preserves definition caches between fixtures while resetting parse scratch
state. The technical reference explicitly requires root fixtures to rely on
those injected definitions; explicit category includes belong in standalone
compilation inputs.

At frozen integration source `ca26086ba81ed7247b7b009aff2f59d5943dc9d6`, the
focused command `go test -run '^TestCherriNoSign$' -count=1 -v .` reproduced exit1
and the same duplicate-enumeration diagnostic. Removing only that redundant
include makes the identical command pass and compile all52 root fixtures.
The quantity fixture still contains the action-output, mutable-variable, and
coerced-global quantity cases. No compiler state/reset behavior or semantic
assertion was changed. The six newly added root fixtures were inspected; none
of the three deterministic-UUID fixtures contains a category include, and all
six compile in the successful run. Nested differential fixtures use their own
standalone runner and retain their required includes.

The original RED log, pre-fix fixture, GREEN log, source hashes, and command
receipts are retained under
`/mnt/development/projects/evidence/cherri-differential-30-20261125/fixture/`.
The committed receipt is
`evidence/dynamic-quantity-fixture-20261125/verification.json`.
This focused unsigned-fixture proof is separate from the root-owned whole-Go
rerun and from native signing/runtime acceptance.
