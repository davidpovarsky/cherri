# E02 incomplete-code diagnostics: exact duplicate suppression

Tested source c0f9bdc95d9e6d9fc363680dbb0f167afe6fbc72; component base4049876b88f5be9f02625f5745917c3f7b797ac1. This is a local service component. E02 requires service plus UI, so no complete requirement or native/UI credit is granted.

## Causal failure and scope

A clean incomplete Number-returning call chain produced24 diagnostics, including20 byte-identical missing-closing-parenthesis messages at the same EOF position. The independent immutable-assignment and two unknown-name errors, plus the innermost missing argument, were genuine. The shared service now keeps one copy of an identical complete Diagnostic value, preserving first-observed order. Identity includes code, severity, full document/range and message. There is no count cap, blanket parse-error suppression or semantic type relaxation.

The first exploratory fixture used nested show calls and produced43 messages. That source also has19 legitimate Void-as-argument errors, so its original maximum-count expectation was too broad. The retained tests-only follow-up uses nested round calls and independently proves that the fully closed Number chain is healthy. A separate closed nested-show control requires all7 distinct Void errors to remain. The original failed attempts and the correction are preserved.

## Actual verification

Five incomplete sources exercise a missing initializer, call close, named value, nested calls and Unicode string. They retain three independent errors, complete UTF-8 byte/rune/UTF-16 ranges, stable repeated responses and clean recovery after a subsequent edit. The position oracle computes all offsets independently of the production source conversion helpers.

Eleven mutations of actual diagnostics reject a missing/duplicated error, wrong code/severity/document, changed byte/rune/UTF-16 boundaries and lost message. Repeated names at different locations and different missing tokens at the same EOF remain distinct.

The clean final command was:

    go test -count=1 -json -timeout=3m ./internal/language/service ./internal/language/analysis ./internal/language/lsp

Actual187 named PASS,0 FAIL,0 SKIP, exit0 in20.391s, clean source before/after. Original raw172808B SHA2569efe3c2e2c65010ea45f673abb54aa1a76aa0e1630d1736c0c0ad87c892d3ecd. GOMAXPROCS=2 is the only recorded environment override.

## Original process stages

| Attempt | Source state | Exit | Named PASS | Named FAIL | Named SKIP | Seconds |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
| e02-baseline-0fd899ca | 0fd899ca clean | 1 | 16 | 2 | 0 | 7.531 |
| e02-detailed-baseline-50d6bf69 | 50d6bf69 clean | 1 | 16 | 2 | 0 | 3.844 |
| e02-corrected-baseline-53f2b02f | 53f2b02f clean | 1 | 19 | 2 | 0 | 8.016 |
| e02-clean-c0f9bdc9 | c0f9bdc9 clean | 0 | 187 | 0 | 0 | 20.391 |

## Proof and limitations

The adjacent evidence/p15-e02-editor-diagnostics-20261120/provenance.json binds every original script, raw stream, process receipt and immutable Git witness. Archive members and individually committed originals are byte-identical; the seal verifies the committed objects after adding them. All three immutable179 contracts remain unchanged.

No lowerer, native encoding, codec descriptor, consumer tuple, CLI diagnostic policy, UI execution, fresh179 report or linked requirement is changed or credited. Strict formal service evidence registration and the remaining editor scenarios are separate work.
