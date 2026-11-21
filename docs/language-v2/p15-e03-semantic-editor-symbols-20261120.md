# E03 semantic editor symbols — actual local component

Tested source: 456e4dfce3535e100ad5a9ec36cff2b5adc9d371. Base: 90ec31a82efdc2542fee6567a0cebcc30b510fb8.

This component repairs actual shared-service symbol operations. It does not complete E03's service+UI requirement. Apple UI, cross-document rename, a new full179 report and final consumer identity are not proved here. Dedicated strict acceptance registration follows separately.

## Actual failures and repair

Clean tests-only source 99b33e29 returned local definitions/hover and changed real identifiers when the requested position was inside a quoted string, comment, f-string literal portion, Dictionary literal key, member name, type annotation or canonical action parameter label. Invalid columns were silently clamped. Canonical action names overrode actual local variables or authored functions. Rename accepted same-scope duplicates and both selected and untouched reference capture. Renaming a function parameter also omitted its own named call label; clicking that label could select an unrelated local.

The repair requires an exact AST-owned identifier span before any operation. Lexical declarations precede canonical registry metadata. User-function argument labels bind explicitly to the actual function parameter, including calls before declarations and underscore external labels. Before edits are created, resolution of every selected and untouched reference is checked against a hypothetical renamed declaration set at the original source positions; duplicate lexical declarations and parameter labels reject. Only then are exact span edits returned. No compiler, schema, codec or immutable acceptance contract is changed.

The full source oracle independently marks every allowed occurrence, compares the complete renamed text and every byte/rune/UTF-16/document span, then reanalyzes valid renamed programs. Ten mutations of real returned edits/text must fail. Other cases preserve comments, strings, shadowed bindings, sibling scopes, computed Dictionary key references and later parameter defaults.

## Test expectation correction retained

The initial 8adbb96b attempt incorrectly required rendering default values in two signature strings. Existing parameterSignature deliberately presents names/types/optional markers, and the immutable clause does not prescribe default-expression rendering. The tests-only 99b33e29 correction preserves this presentation, adds named-label and unused-duplicate cases, and still reproduces the real ownership failures. Both original runs are retained. No production change was made to satisfy the initial presentation assumption.

## Actual runs

| Attempt | Source state | Exit | Named PASS | Named FAIL | Named SKIP | Seconds |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
| e03-baseline-8adbb96b | 8adbb96b clean | 1 | 17 | 22 | 0 | 14.328 |
| e03-corrected-baseline-99b33e29 | 99b33e29 clean | 1 | 19 | 22 | 0 | 12.328 |
| e03-first-repair-draft | 99b33e29 dirty | 0 | 41 | 0 | 0 | 10.844 |
| e03-clean-456e4dfc | 456e4dfc clean | 0 | 235 | 0 | 0 | 21.641 |

The final command is go test -count=1 -json -timeout=3m ./internal/language/service ./internal/language/analysis ./internal/language/lsp with GOMAXPROCS=2. It completed235 named PASS, zero failures/skips, exit0 in21.641s, with clean source before/after. Raw217,428B SHA256 c4b78cd18f47c7459e530425dfaf60aaa1479d320b48c7490dcff6ba135c55c1. The first repair replay was explicitly a dirty draft, preserved with both exact source files and tracked diff; it is not clean-source evidence.

## Provenance and remaining scope

The evidence directory contains byte-preserved process logs, receipts, submitted scripts, draft source, the original ZIP and immutable Git source witnesses for the tested and baseline revisions. provenance.json verifies original hashes, actual event counts and all three unchanged179 contracts. The proof commit is separate from implementation, and every retained original and witness is re-read from that committed object after sealing.

A separate read-only peer review found no concrete capture or namespace false-success in these bounded paths; it is not substituted for the actual test run. Qualified-module hover/definition and workspace rename are not inferred from this local-symbol component. Required UI behavior remains unexecuted and open.
