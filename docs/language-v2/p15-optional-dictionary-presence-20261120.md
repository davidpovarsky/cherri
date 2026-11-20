# P15 owned Dictionary lookup and Optional presence

## Verified result and scope

Implementation: `d695fa421c551511777290f92b2926dd1c6c6b8b`.
Base: `cf89a1bfd826acc674ec06d6a996556f6c0bcee4`.
Both commands ran against committed source with clean status before and after.
The root selection passed **1,307 named events** in **46.672 seconds**; the complete six-package selection passed **138 named events** in **9.125 seconds**.
Both exited zero, with no failed or skipped named events and no timeout. Counts include parent tests and subtests; they are not requirement counts.

The unchanged `TestP15RequirePreservesInferredSourceType`, Nil diagnostic graph and original Legacy nil parity checks pass in this selection.
This proves bounded compiler/service routes and their independently asserted native structure.
It does not prove Apple runtime behavior, general Optional joins, dynamic Dictionary presence, or completion of T05/L01/FN05/RP20 or the 179-row contract.
No acceptance registrar, requirement status, immutable contract, native fixture, codec descriptor or fingerprint was changed.

## Reproduced failures and retained test corrections

Original process receipts, stdout, stderr and available graphs are retained byte for byte under [the evidence directory](evidence/p15-optional-dictionary-presence-20261120/).

| Attempt | Exact source | Actual result and interpretation |
| --- | --- | --- |
| Initial public baseline | `b1775040a959ffaddf03cc43f5b9c0b47097a6e1` | 11 pass / 100 fail events; exit 1; 19.531 s. Of 96 public leaves, 84 reproduced missing owned presence; 12 new empty-list fixture expectations incorrectly required inferred Text elements. The duplicate negative also needed its existing diagnostic accounted for. The original require test independently failed. |
| Corrected public baseline | `906ab99ac0d9e6d1ee2bb257038a47e5fb92e5ca` | 11 pass / 100 fail events; exit 1; 20.656 s. All 96 public leaves now reached the real E_TARGET_VALUE_SHAPE failure. The duplicate negative's analyzer/service set was correct, but its CLI assertion still assumed the first error was the later shape diagnostic. |
| Initial direct-index oracle | `9c10e2580c788fd9e5ea6ec8de1b098e6f581c5a` | 0 pass / 5 fail events; exit 1; 12.813 s. The new oracle recognized only direct DictionaryKey selection. Its incomplete selector-form assumption is retained and is not a separate compiler defect. |
| Corrected direct-index oracle | `54cf4350d79630f1572246bc027270189264202f` | 0 pass / 5 fail events; exit 1; 12.484 s. Both real exact selector forms are admitted, and four actual value/order defects remain. All four decoded graphs are retained. |
| Repaired precommit replay | Dirty `54cf4350` plus source ZIP | 297 root + 19 core pass events; exit 0. The exact dirty source ZIP and hashes distinguish this observation from clean proof. |
| Repaired committed replay | `d695fa421c551511777290f92b2926dd1c6c6b8b` | 1,307 root + 138 package pass events; both exit 0. Actual clean-source proof. |

An inferred empty List has element type Unknown. The corrected new fixture uses `List<Unknown>` and still asserts exact empty-value preservation.
For duplicate keys, analysis and service must report both E_DUPLICATE_KEY and the shape diagnostic with exact spans. The compiler returns its first E_DUPLICATE_KEY and emits zero bytes. No production diagnostic was altered for either correction.

The prior static-index negative named `map_lookup` used a literal Dictionary that this change deliberately supports. That exact source is covered by the positive matrix.
Its negative slot now uses a mutable Dictionary, retaining the exact E_TARGET_VALUE_SHAPE and zero-byte requirements. All other prior static-index negatives remain intact.

## Causal compiler changes

### Presence from a checked authored slot

Analysis retains an exact literal Dictionary origin only through immutable initialized aliases and the resolved canonical CompilerConstruct `require` passthrough.
A user function named require, arbitrary/opaque call, mutable binding, mutable alias, missing key, Optional/opaque element or computed key does not create that fact.
Every key must be nonempty literal Text/raw Text and unique: a dynamic sibling key could overwrite the selected entry.
The selected element must independently have a DefinitelyPresent semantic type.

The exact-AST map resets per analysis, is copied when exposed, retains module-owned function-body facts and follows scoped-block expression clones.
Production lowering receives the checked map explicitly. It does not infer presence from native truthiness, content count or the eventual value's appearance.
False, zero, empty Text and empty List/Dictionary remain separate from None without broadening unproved lookups.

### Direct indexed Maps use the existing runtime value path

Previously IndexExpr passed its direct Map target through generic expression lowering. Four clean baseline graphs establish the resulting defects:

1. An early mutable reference remained live until a later Map entry had reassigned it.
2. Ordinary keys WFSerializationType and Value were reinterpreted as token-string metadata and emitted a Text action.
3. An empty List field omitted its explicit native Value array.
4. An empty Map field omitted its explicit WFDictionaryFieldValueItems array.

IndexExpr now lowers its target through existing `lowerRuntimeLiteral`, which emits the typed Dictionary and performs sibling-write captures.
This reuses established encoding and capture logic. It does not modify serializers, raw-native templates, literal Text carrier code or the codec descriptor.
The independent complete-graph oracle retains both exact lookup forms: getvalueforkey followed by its consumed result, and direct DictionaryKey aggrandizement.

## Actual coverage

The public matrix executes eight values across four origins and three consumers: **96 source cases**, each through analysis, actual compiler and service.
Values: present/empty Text, false, zero, empty/populated List and empty/populated Dictionary.
Origins: direct literal, immutable aliases, canonical require primary input, and named-input require plus alias.
Consumers: Optional binding/predicate, written function argument/forwarding, and Optional return.

Twelve unproved-origin controls require exact diagnostics/spans and zero bytes.
Thirty-two independently authored whole native graphs assert original Dictionary shape, require count/type rejection paths and exact passthrough, selected key, separate supplied-argument/value-presence flags, payload and return carrier.
Sixteen corruption controls reject wrong key/kind, lost empty arrays, incorrect presence flags, changed type/count inputs, ignored rejection, wrong passthrough, deferred capture and lost payload.
The four direct-index defects each have an independent complete native graph.

An actual imported qualified-function call verifies that mutable None still demands its flag. Eight module-defined require/index return bodies verify transfer of defining analysis.
Syntax controls require complete unique literal keys. Analyzer reuse, getter-copy isolation and lexical shadowing prevent cross-source fact reuse.
A scoped-block test checks fact/type transfer to cloned expressions without mutating the source AST.

The clean root selection includes existing V02, forwarding, immutable/mutable Optional, aliases, indexes, functions, containers, computed keys, require, None clearing, original Nil/Legacy and raw-native boundaries.
Full syntax, analysis, lower, service, types and backend package suites ran afterward.
Parity's independent read-only review found no concrete provenance/copy/order gap; review is not counted as execution or runtime evidence.

## Exact commands and evidence

```text
go test -count=1 -json -timeout=5m -run ^TestP15(Optional|Function|V02|DictionaryList|NumericList|EmptyList|L01|F04|Nil|Require|Computed|SemanticContainer|ExplicitNativeContainer|F03NoneYield) .
go test -count=1 -json -timeout=3m ./internal/language/syntax ./internal/language/analysis ./internal/language/lower ./internal/language/service ./internal/language/types ./internal/language/backend
```

These are recorded argument vectors; the regular expression is one argument.
Environment: Go 1.25.6, Windows/amd64, CGO_ENABLED=0, GOMAXPROCS=2.
Receipts preserve commands, finite outer budgets, timestamps, all named events, source hashes, clean status and output hashes.

| Artifact | SHA-256 |
| --- | --- |
| Clean root stream | `b43cf2bcc669b974bf516d495b2bb57a4a3d222acb071f488d925e7f11e233d6` |
| Clean package stream | `c554af774e1ee86a29d349dd61bcd23e0e2cfc94fe4914dc3cab2e60deb0c028` |
| Corrected failing public baseline | `9e303cd8547ec02e318009d29dc30ef4a1e1a4e4c4ceeba5e653d4dcc5125262` |
| Corrected failing direct-index baseline | `f5e56b20faf233676c89a9cf4cc3595e66601e486ed11c0107e96083fc48ff95` |

`manifest.json` hashes every retained evidence file. `source-binding.json` binds 15 source/test files to immutable Git blobs and verifies nine original files/contracts/codec descriptor remain byte-identical to the base.
`component.patch` retains the exact base-to-implementation delta. Neither manifest nor report grants acceptance credit.

## Integration order

After `cf89a1bfd826acc674ec06d6a996556f6c0bcee4`:

1. `b1775040a959ffaddf03cc43f5b9c0b47097a6e1` — public regression sources.
2. `906ab99ac0d9e6d1ee2bb257038a47e5fb92e5ca` — precise empty-list/duplicate-diagnostic baseline corrections.
3. `9c10e2580c788fd9e5ea6ec8de1b098e6f581c5a` — direct-index native graph regressions.
4. `54cf4350d79630f1572246bc027270189264202f` — both exact native key selectors.
5. `d695fa421c551511777290f92b2926dd1c6c6b8b` — bounded repair and full positive/negative checks.
6. The documentation-only evidence commit containing this report.

General Optional control joins, arbitrary action-result presence, dynamic/opaque lookup, native U+FFFC fields, lossless None inside ordinary native containers, and nested-list cardinality on Apple remain separate outstanding boundaries.
