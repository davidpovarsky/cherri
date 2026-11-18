# B02 / AC01 catalog identity acceptance

These producers replace the old count-only observations for the two immutable local contracts. B02 requires a captured baseline, definition identities and representative native outputs; AC01 requires accounting for every baseline definition, wrapper variant and syntax construct.

## Actual production path

The archived `docs/language-v2/baseline-catalog.json` remains the baseline input. The test loads current declarations through the same live catalog builder and reconciles the resulting identities against the generated default schema. It then uses the existing `executeSingleCatalogEntry` runner for actual Legacy/V2 compilation, parameter checks, decompilation and native-action structural roundtrip. The optional observer receives a copy of each genuinely decoded compiler plist; old callers and historical 505 report formats are unchanged.

Every definition receives an explicit disposition. The report retains both definition hashes, every changed metadata field, and each prior parameter occurrence's before/after contract and matching position. Names sharing a native Apple identifier remain separate wrapper records and are grouped from the canonical catalog. Added definitions are reported separately. Compiler macros remain outside the native catalog denominator.

The full generated receipt includes actual synthetic source, source and plist hashes, native action identifiers and decoded target-action payloads for both frontends. It also records the compiler commit, clean-tree observation, baseline hash, native schema fingerprint and separate macro fingerprint. Success counts come from actual captures and verified roundtrips.

## Causal controls and constructs

Same-count substitution, duplicate definition, changed native identity, construct reclassification, removed parameter occurrence, collapsed duplicate label and a retargeted generated-schema row all reject. Native-capture controls reject changed source, malformed plist, changed target action, absent target action and a declaration-only success claim without an actual captured frontend output.

All eight canonical construct insertion snippets compile as real language syntax. The append-variable snippet explicitly initializes its required binding. Zero-argument callable forms must fail at the syntax/canonical-definition boundary, rather than merely a missing-required-argument check; populated one/two-argument forms also reject. The `calculateExpression` historical V2 wire fixture remains explicitly identified as raw transport. Its independent ordinary arithmetic insertion snippet emits the actual Math action, so a raw escape hatch is not represented as an ordinary callable construct.

AC01 additionally executes the existing U02 producer: actual regeneration must match the checked-in schema, input drift must reject the stale output, failed live extraction must fail closed, and restoration must pass.

## Honest boundaries

The archived catalog does not include historical native plist specimens. This producer captures current representative outputs; it does not claim retrospective equality to missing historical native bytes. A recorded metadata change is not a promise that every old source signature remains valid.

The baseline and current declaration catalogs both contain duplicate labels for `getDeviceUsage` (`startTime` on two distinct wire keys) and `streetAddress` (`addressLine2` on two distinct wire keys). Parameter occurrence matching preserves and reports them separately. This stage does not silently rewrite those definitions or assert coverage for every optional-parameter combination.

The formal tiers are repository/generation only. Apple runtime, provider installation, complete native option combinations, Pure/Pack acceptance, physical devices and overall release closure remain separate obligations. The formal report still accounts for all 179 obligations; these two producers cannot mint success for an unexecuted requirement.

## Generate and verify

From a clean commit, set `CHERRI_CATALOG_ACCEPTANCE_OUTPUT` to the intended JSON destination and run:

```text
go test -count=1 -json -timeout 6m . -run '^(TestP15B02AC01CatalogMigrationAndNativeOutputs|TestP15B02AC01RejectsCatalogIdentityFaults|TestP15B02AC01RejectsNativeCaptureFaults|TestP15AC01ConstructGrammarAudit)$'
```

The producer writes a detailed ledger only when that environment variable is set. Ordinary tests do not rewrite checked-in 505 reports. Run the source-bound formal producer and anti-mint controls with:

```text
go test -count=1 -json -timeout 8m ./tools/language-acceptance -run '^TestCatalog(Baseline|Accounting)ProductionEvidence$'
```
