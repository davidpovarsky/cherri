# Deterministic UUID root repair — 2026-11-25

## Finding

The optional `--derive-uuids` path had three related defects:

- Legacy hashed only the workflow name and caller salt. Every literal array
  element reused the array name, and every anonymous control block reused an
  empty salt. Distinct producers and groups received the same UUID.
- V2 `Lowerer.GenerateUUID` always used cryptographic randomness. The public
  flag did not reach V2 lowering.
- Canonical debug, require, pagination and export expansions independently
  generated random IDs, leaving macro workflows non-repeatable.

The inherited Legacy mechanism exists in current upstream
`98d5191ae121a6bcf663ed0e55495f4ee4c7a6e2` and fork baseline
`6ce0a3981d2047ba5aec603f185e2d8f922fc0ee`. The actual focused RED used
unchanged production source at quantity-repair successor
`e432f7b9f5f663cc56ba8041e63d2e90317fc756`.

The independent differential suite retains original/fork failures with the flag
enabled. Normal-mode compilation avoids that collision mechanism; this change
repairs the generation policy itself. Both original source checkouts remain
unchanged.

## Shared generation policy

`generated_uuid.go` owns one per-compilation sequence. Derived IDs use the
existing deterministic hash function with an explicit generator domain, current
workflow name, caller salt and increasing occurrence number. Repeated labels
therefore no longer identify different generated occurrences as the same object.

The name is read from the active compilation because metadata can set it while
parsing/lowering. A different workflow name produces different IDs. A new
compilation restarts the sequence.

Legacy configures the policy at `initParse`; authoritative
`resetCompilerState` clears it. Internal `resetParse` deliberately does not
reset it because includes/functions also invoke that helper within one workflow.

V2 adds explicit `CompileOptions.DeriveUUIDs`. The CLI copies its flag into the
option; library callers do not inherit CLI-global arguments. The lowerer accepts
an optional generator callback and retains its existing random path by default.
Cold canonical registry initialization resets globals, so it completes before a
derived V2 workflow's sequence begins.

The same generator serves canonical debug labels/logging/timers, require,
pagination, export, App Intent instance IDs and generated workflow target
placeholders. Existing uppercase/lowercase formatting remains unchanged.
Authored native data and imported explicit IDs bypass generated identity.

## Reference and compatibility contract

Existing producer-reference caches still reuse an allocated producer UUID.
Control end/else/case markers still reuse their corresponding start token's
group UUID. Only a request for a new generated occurrence advances the sequence.

The old colliding deterministic UUID strings intentionally change. Native action
identifiers, parameter keys, control modes and action ordering remain unchanged.
Without the option, generated IDs remain random version 4; derived generated
IDs are version 5.

Imported workflows preserve exact UUIDs, grouping IDs and native data. Tests
also cover both supported explicit V2 identity forms: the `uuid:` argument and
an exact `native.plist` parameter dictionary.

## Permanent regressions

Three actual Cherri fixtures cover repeated literal array elements, anonymous
If/Repeat/For/Menu blocks, and canonical macro expansion:

- `tests/derive-uuids-arrays.cherri`
- `tests/derive-uuids-controls.cherri`
- `tests/derive-uuids-macros.cherri`

`derived_uuid_test.go` compiles both frontends. It requires unique producer
UUIDs, references to actual preceding producers, unique control starts, matching
identifier/group pairs, and balanced control nesting.

The V2 public file entry point is exercised so dropping the CLI flag cannot
pass through an internal-only test. Other cases require exact derived replay,
Legacy sequential parse reset without an intervening full reset, explicit API
option isolation, different workflow names, imported native identity, App Intent
IDs and generated workflow placeholders.

The unchanged strict structural comparator verifies derived/random parity for
the core array/control graphs. A causal mutation retargets an array edge to
another real producer; the comparator must reject it.

Some existing macro attachment display names literally contain a generated
UUID. Those strings differ between random and derived builds and between
independent random builds. They remain visible literal differences; no
comparator normalization was added. Macro qualification establishes exact
derived replay, graph integrity and preserved random-v4 behavior, without
claiming literal display-name equality across modes.

## Actual verification

Actual baseline failures remain retained: Legacy array producer collisions,
Legacy control-start collisions, V2 random IDs despite the flag, and macro
replay failures in both modes. Imported exact identity and the edge-retarget
negative control passed before the production repair.

The final root selector passed:

```text
go test -run '^(TestDerivedUUID|TestDebug|TestP15Debug|TestP15Require|TestP15Paginate|TestP15_LegacyMeasure|TestP15LegacyInline|TestSEM03|TestSEM18|TestBuildProfile|TestQuantity|TestP15SignedFixtureExactProducerReferences)' -count=1 -v .
```

This includes new UUID obligations and existing affected macro, scoped/inline
value, import, profile, quantity and producer-identity regressions. The test
process reported 10.988 seconds; build plus execution took 44.29 seconds.

Affected packages passed separately:

```text
go test ./internal/language/lower ./internal/language/backend
```

A separate process replay used the differential suite's unchanged
`tests/differential_30/run.py` comparator. Cases 13–15 and 25–27 compiled twice
through both fork frontends: **24 separate CLI processes, 12 replay pairs**.
Every pair had strict typed equality, zero raw differences and two valid graphs.
UUID alpha-renaming was not needed to make these replay pairs equal.

The verified precommit candidate binary SHA-256 is
`549f21ea69154b1dccbda60c1195b99967f6aee64038fa230de88e886d763854`.
[The bounded verification record](evidence/deterministic-uuid-20261125/verification.json)
binds final source files, commands, receipts and replay summaries.

Full originals are under
`/mnt/development/projects/evidence/cherri-differential-30-20261125/uuid/`:
baseline/intermediate failures, final focused and package results, compiler, and
24 emitted replay artifacts. Intermediate test setup corrections are identified
in the verification record rather than discarded.

These are compiler/native-graph results. Apple execution, the wider normal-mode
corpus and integrated full-suite qualification remain separate coordinating-task
results. No release or project-wide acceptance is inferred here.
