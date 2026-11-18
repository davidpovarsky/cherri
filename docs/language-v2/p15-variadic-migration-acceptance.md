# C03: explicit named variadic lists

## Scope and source behavior

C03 requires unit and native-structure evidence that a Legacy variadic source API migrates to an explicit named list. Its required cases are empty, single and multiple inputs, with their structure preserved. It does not require Apple execution.

Migration now uses the parsed source tree and canonical registry. For example:

    // Legacy
    url("https://example.test/first", "https://example.test/second")
    list("first", "second")
    hideAllApps()

    // Migrated V2
    url(url: ["https://example.test/first", "https://example.test/second"])
    list(listItem: ["first", "second"])
    hideAllApps(except: [])

A named list in a variadic slot is a source argument pack. The compiler expands exactly its outer list expression into the existing canonical argument vector. Each element keeps its original expression identity, source span and written order. Ordinary positional and single named scalar calls retain their existing compatibility.

This syntax is not a runtime spread. A nested list remains one value and undergoes the repeated parameter's ordinary type check; it is never recursively flattened. A runtime list variable does not acquire a compile-time item vector. The original syntax tree remains available to formatting and source services.

User functions and custom action declarations can shadow a canonical variadic name. Migration preserves those declarations and calls. The existing lexical scope walker also prevents a same-named local binding or parameter from being treated as the native callable.

## Native constructor boundary

The current canonical registry contains eight variadic constructors. The test matrix reconciles that inventory, so a newly added variadic requires explicit coverage.

| Callable | Explicit list label | Existing native field | Supported structural forms |
| --- | --- | --- | --- |
| addToReadingList | url | WFURL | Literal vector or individual text references/templates |
| emailAddress | email | WFEmailAddress | Literal contact entries or one dynamic text field |
| hideAllApps | except | WFAppsExcept | Literal application entries or one dynamic text field |
| killAllApps | except | WFAppsExcept | Literal application entries or one dynamic text field |
| list | listItem | WFItems | Text rows with literal or individual reference/template values |
| phoneNumber | number | WFPhoneNumber | Literal contact entries or one dynamic text field |
| quitAllApps | except | WFAppsExcept | Literal application entries or one dynamic text field |
| url | url | WFURLActionURL | Literal vector or individual text references/templates |

Contact and application constructors have separate existing encodings for literal vectors and a single dynamic text field. A mixture of a dynamic field and other items now fails with an explicit diagnostic. The previous implementation could discard app exclusions or serialize a private reference value as an empty dictionary. This change does not invent a mixed native encoding.

The compiler preserves zero arguments separately from one empty string. App exclusions remain omitted for the empty case. URL/List/contact vectors retain their native array shape. Empty contact constructors now use the existing empty contact-field encoding rather than indexing a nonexistent first argument.

The native List constructor preserves a List<Text> source guarantee only when every analyzed item is exactly Text. CurrentDate, unknown references and other content compatibility do not establish that guarantee. Opaque items keep an opaque element type. This prevents the named constructor route from weakening T04/T09 or losing the established type of migrated literal-text lists.

## Causal defects observed

The baseline was 58c7b0d67fb8917c9a1305101d66293c4e476ec4.

- All explicit named list cases failed the old scalar Text check; the migrator did not produce the required form.
- Empty emailAddress and phoneNumber calls failed with an index-out-of-range error. The other 22 Legacy zero/single/multiple literal controls passed.
- The old list-constructor regular expression could rewrite string contents and terminate at the first nested closing parenthesis.
- Repeated canonical text arguments with an ActionOutput descriptor were reinterpreted as Legacy source names. A nested Text result failed with "Variable or Global 'Text' does not exist", despite its producer UUID being available to the existing serializer.
- Mixed literal/reference contact and app constructors were accepted while dropping or corrupting items. Failing observations include real CLI calls, not only a model of their behavior.
- A custom action declaration named url or list was initially packed according to the builtin label. Actual original-source controls compiled, while migrated forms failed E_UNKNOWN_ARGUMENT. The declaration prepass now excludes functions and custom actions.
- Removing the List regex without preserving proven text element knowledge exposed Unknown in the unchanged repeats.cherri fixture. Exact Text evidence restores that source contract without a broad cast.

## Implementation

| Path | Responsibility |
| --- | --- |
| internal/language/schema/variadic.go | Expands only an explicit named outer list; rejects combining that pack with another argument for the same slot |
| internal/language/analysis/variadic.go | Derives only the source-proven List element guarantee |
| internal/language/analysis/analyzer.go | Runs normal binding/type checks on original pack elements; aligns variadic empty arity with the existing canonical definition |
| internal/language/lower/lower.go | Reuses the same expansion before existing source-order evaluation and canonical emission |
| internal/language/migrate/variadic.go | Rewrites Legacy calls through parsed nodes with declaration and lexical shadow protection |
| internal/language/migrate/optional_bindings.go | Exposes a postorder call visitor through the existing scope walker |
| action.go | Accepts complete canonical descriptors at the repeated nonliteral, nonenum Text boundary |
| actions_std.go | Preserves the existing literal/single-dynamic encodings; handles empty contacts and rejects lossy mixtures |
| tools/language-acceptance/calls_execution.go | Binds C03 to actual named production tests and normalized required tiers |

The native action catalog, generated schema data, parameter labels and Apple identifiers are unchanged. There is one serializer path. Original public fixtures remain unchanged.

## Verification and formal producer

The C03 producer invokes fifteen named production test families in p15_variadic_migration_test.go and p15_variadic_structure_test.go. They cover:

- The complete eight-constructor inventory, with zero, one and three literal items through Legacy, migrated and authored V2 paths.
- Exact native item count, order, field types, contact entry kinds, app mode flags and the difference between omission and an empty value.
- Once-only Text producers, distinct UUIDs, source order and each consumer's exact reference.
- A mutable first item captured before a later pack element assigns the same source variable.
- Singleton references/templates and explicit rejection of mixed contact/app fields through actual CLI exit status and absent output.
- Nested calls, text/comment preservation, canonical formatting, function/custom-action shadowing, and source-proven List element typing.
- Rejection of nested list flattening, ambiguous duplicate/mixed packs, wrong labels, wrong element types and a declared Void result.
- Causal mutations that drop, duplicate, reorder, stringify or retarget native values and producers.
- Actual migration and both compiler CLI routes for unchanged arrays, functions, math, menus and repeats fixtures, checked by the established normalized comparator.

The formal entry point is:

    go test ./tools/language-acceptance -run '^TestVariadicMigrationProductionEvidence$' -count=1 -v -json

It uses the existing named producer audit, immutable source/clean-tree binding and causal evidence rejection controls. Only the C03 smoke assertion is replaced. The immutable 179-ID contract and other requirement statuses are unchanged. The native tier is normalized to native-structure; no native runtime or release closure follows from these results.

## Independent defects retained

Two observations are outside the C03 closure claim:

1. General Legacy inline actions outside a scoped body can lose their producers. The observed url(text("FIRST"), text("SECOND"), text("THIRD")) baseline emitted one URL action rather than three Text producers plus URL. C03's Legacy reference parity control uses explicit const producer bindings. The AST migrator can preserve and compile nested expressions in V2, but this does not certify the old Legacy inline parser.
2. Action declarations currently mutate the shared default registry. Compiling a custom action named url in one process can contaminate a later unrelated compilation. The complete RED log is retained. The C03 custom declaration proof uses actual independent CLI processes; the separate registry-lifetime repair must prove repeated-compilation and multi-document independence.

No signing request, shortcut installation, Apple execution, private corpus upload or release action is part of this component's tests.


## Clean immutable execution receipt

Implementation source: **3992d87cfe0f884b0d01268907dc64df7a0da6bf**. Base: **58c7b0d67fb8917c9a1305101d66293c4e476ec4**. The checkout was clean before and after job **job_b4f524a8e8a8**.

| Actual command scope | Result | Wall time |
| --- | --- | --- |
| Named C03 formal producer, executing all fifteen C03 production families | PASS; unit and native-structure evidence accepted | 11.596 s |
| Nearby C01/C02/nil/VCard/menus production regressions | 125 named results PASS | 10.464 s |
| Complete analysis, migrate, schema and lower package tests | 64 named results PASS | 4.654 s |

All **47** original root public fixture byte hashes stayed identical. The clean formal receipt contains only C03's requirement result; it does not close the release. The earlier working-tree validation had 299 selected production results PASS, including the C03 scenarios; it is retained separately and is not relabeled as a clean-source receipt.

[The evidence audit](evidence/p15-variadic-3992d87/evidence-audit.json) binds eleven complete payload files by SHA-256. They include the complete formal Go JSON stream, its reconstructed source-bound receipt, clean regression/package streams, the exact public fixture hashes, and the original causal RED logs. Go output fragments were concatenated only within their owning test before extracting the receipt JSON.

The singleton contamination RED and Legacy inline-producer loss remain visible in those logs. The former is assigned to the following registry-lifetime repair; this C03 result neither resets the singleton to hide it nor claims that it is fixed.
