# Per-program registry lifetime

## Reproduced defect

On baseline da2e6ced5449f452600eb7c348320376cd85ebf9, analyzing a source-defined action named url replaced the process-wide canonical url schema. A later unrelated source using the canonical named url parameter failed E_UNKNOWN_ARGUMENT. A source that failed after registering its declarations, and diagnostic-only analysis, caused the same contamination.

The shared service also used its base registry for every document's completions, hover, signatures and visual edits. After opening a second source-defined url action, earlier documents displayed the second document's identifier and parameter. A visual edit could add the unrelated parameter instead of replacing the requested field. Replacing one callable repeatedly also left stale and duplicate native-identifier index entries.

The original C03 contamination receipt is retained in p15-variadic-3992d87/singleton-contamination-red.jsonl. The focused registry baseline is retained separately; no reset occurs between the inputs whose independence is under test.

## Implementation

- Each analyzer snapshots the supplied catalog. Each Analyze call creates a fresh effective registry and clears prior diagnostics and expression proofs.
- Source-defined actions remain valid local declarations, including canonical-name shadows and typed output references. Both compiler entry paths and the automatic lowerer analysis retain the effective program registry.
- A language service owns a stable base snapshot. Contextual completion, enum completion, hover and signatures use the analyzed document's registry. Visual edits analyze their supplied source into an isolated context, even when that source is not an open document.
- RegisterCustomAction removes only the replaced callable's old native index entries, preserves sibling variants and recomputes the effective registry fingerprint.
- Snapshot copying preserves typed metadata without JSON conversion: parameter metadata, enum values, static map/slice values, App Intent fields and structural type containers have separate storage. Canonical scalar types and compiler macro definitions remain read-only shared values.
- Reusing an analyzer restores explicit target/version API defaults. A prior source header does not become the next source's configuration. The semantic version comparison and native availability facts are unchanged.

The public service compatibility fingerprint still describes its stable canonical base, not a document's custom declarations. No generated action schema or catalog facet changed.

## Actual validation

Focused baseline job_d0ed9dfed9bb failed all eight new top-level regression families across the root, schema, analysis and service packages. The first candidate fixed the contamination. A new canonical visual-edit control initially requested a native serializer key that the variadic parameter schema does not advertise; that test was corrected to the canonical accepted label. The actual cross-document custom-field edit failure remained covered.

Working-tree job_1b7744aaeffd passed:

- 305 named root tests, including real compiler output/producer-reference checks, C01/C02/C03, unchanged nil and menus fixtures, and automatic action propagation.
- 85 named tests across the full schema, analysis, service, lower and migrate packages.
- No skipped tests in those commands.

Commands:

    go test . -run "^(TestP15(Registry|C03|C02|C01|Nil|VCard|Menus)|TestP10_AutomaticActionPropagation)" -count=1 -json
    go test ./internal/language/schema ./internal/language/analysis ./internal/language/service ./internal/language/lower ./internal/language/migrate -count=1 -json

A separate clean immutable-source receipt follows the implementation commit. Working-tree checks alone are not represented as that receipt.

## Scope

This repairs a local compiler/service lifetime defect. It does not add a formal acceptance ID, claim the entire 179-contract suite is closed, or establish Apple import/runtime behavior. No signing, native simulator dispatch or private-corpus run was requested by this component. The existing 47 original public fixtures remain unchanged. The separately documented general Legacy inline-action omission remains assigned for the next atomic repair.
