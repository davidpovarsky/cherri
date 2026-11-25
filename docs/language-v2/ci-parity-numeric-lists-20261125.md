# CI parity inventory and typed numeric lists — 2026-11-25

## Observed failures

The published source `754a4dc7ed2b4622f96e267193244db4ac63077f` passed its full Go CI job, including the legacy-parity package tests. The separate real `go run ./tools/legacy-parity` step then failed before comparison because six new root fixtures were absent from `tests/fixtures-manifest.json`:

- `derive-uuids-arrays.cherri`
- `derive-uuids-controls.cherri`
- `derive-uuids-macros.cherri`
- `loop-reference-compat.cherri`
- `qty-dynamic.cherri`
- `value-constructor-defaults.cherri`

The manifest had 47 entries while the actual repository contained 53 root fixtures. No manifest entry named a missing file. This repair adds all six entries and a package test that checks the actual repository inventory in both directions, including duplicate, malformed, missing and nonregular entries. It adds no exemption or fixed list to the regression. The CLI's existing unknown-fixture and semantic acceptance checks are unchanged.

The first real gate after the inventory correction exposed two normalized differences: the derived-UUID array and control fixtures. Exact compiled artifacts showed intact numeric dataflow on both sides. Legacy emitted Number actions followed by Add to Variable references; V2 emitted the same ordered Number values followed by native List references. Legacy Number values were plist reals and V2 values were plist integers, handled by the comparator's existing numeric-parameter policy.

The old procedural-list normalizer removed Legacy Number producers and substituted raw numeric scalars in text rows. Its resulting 2-versus-5 and 21-versus-22 normalized action counts were not the raw compiler graphs. The raw counts were 9-versus-5 and 26-versus-24. The migrator and both compilers preserved numeric values and reference flow.

## Bounded correction

The shared normalizer now recognizes a complete cleared-variable construction made of adjacent literal-producer/Add-to-Variable pairs. It retains Number actions, their exact scalar values and UUIDs, and their typed `WFTextTokenAttachment` references inside List rows. A literal Text producer may become a literal text row. Numeric-looking text remains text.

The witness requires an actual Nothing input, unique nonempty producer identities, one exact reference per consumed producer across the complete document, matching declared/reference names, no other variable writes or earlier reads, and exact allowed action/parameter/reference shapes. Unknown fields, disabled actions, aggrandizements, malformed containers, external uses, invalid identities and interrupted chains are rejected conservatively. The previous numeric-to-scalar branches are removed, so rejection cannot fall back to the old transformation. Existing List-plus-numeric-append constructions are not newly certified.

Native List references to Number producers also require internally consistent output names before the existing display-name normalization. Consistent renaming remains allowed. Import questions retain exact action origins; a removed or ambiguous target leaves the original document unnormalized.

A subsequent real gate caught an overly broad first recognizer: `types.cherri` contains independent uninitialized declarations and no Add to Variable actions. Recognition now requires a confirmed producer/append pair before treating a Number as list construction. The real declaration artifacts are retained as a regression.

## Executed evidence

The focused regressions use six exact compiler-produced workflow files under `internal/shortcutcompare/testdata/numeric-lists`; their provenance and original artifact hashes are retained. No workflow plist was handwritten for the positive cases. Mutation controls alter copies to test values, types, order, cardinality, identity, coercions, names, external uses, side-effect boundaries, unknown fields and import-question bindings.

The actual final candidate commands were:

```text
go test -count=1 -timeout=5m -json ./internal/shortcutcompare ./tools/legacy-parity
go build -o <evidence>/cherri-parity-candidate-v4 .
CHERRI_BIN=<evidence>/cherri-parity-candidate-v4 go run ./tools/legacy-parity -output <evidence>/final-candidate-parity-results.json
```

Both affected packages passed: 158 named tests/subtests, including all 60 numeric-list controls, with no failures or skips. The actual CLI gate passed all 53 fixtures: 9 direct structural parity results and 44 witnessed normalized results. All 17 action probes passed. Baseline compilation, migration, V2 compilation and parity failure counts were zero.

Evidence is versioned under `cherri-differential-30-20261125/ci-parity-inventory`. The final candidate execution receipt SHA-256 is `8efa946f18ad1c5f69a19a3ddf9a4a50c87ffb6ea1ee18af8619909a2b5c4887`; the actual parity report hash is `2cb550a1bd67e8aa7ca7fbb865b62b050a6e951197a8fd8da45330c73e66d434`.

The original CI failure, inventory-only two-difference result, complete baseline regression failure, intermediate malformed-container/name failures, declaration-recognition failure and the execution-reporter schema error all remain retained. They are not restamped as passes.

## Qualification boundary

This change does not alter language analysis, migration, compiler emission, existing source fixtures or the strict `tests/differential_30` comparator. It does change `internal/shortcutcompare`, which is linked into the main executable. The candidate gate therefore records its freshly built compiler and uncommitted build identity explicitly. A clean final build and separate strict comparison of the 82 fork artifacts establish any subsequent artifact reuse.

No Apple execution, signing operation or full Go suite is claimed by this scoped repair. Existing native observations retain their original artifact, compiler and proof identities.
