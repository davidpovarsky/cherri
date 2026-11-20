# Unknown expression nodes fail closed — 2026-11-20

## Verified repair and limit

The clean tests-only source `fe952222c62b78fbb1c1b30d89d1d6ed4471f0a2` reproduced five silent successes in actual checked lowering: an unknown expression alone, inside a Map value, List item, interpolated text, and an invoked function body. Unknown declaration and statement controls already failed explicitly.

The production repair `7d767d08f0fc5c0687de306593f5ee135f8227ea` changes only the final unhandled-expression fallback in `lowerExpression` to return an explicit error. Legitimate absent expressions and the existing explicit None/Null cases retain their own handling. No native fixture, golden, contract or production mutex was changed.

A complete fresh `go test -count=1 -json -timeout=5m ./internal/language/lower` on that clean repair passed **38 named events**, with zero failures or skips, in 44.734 seconds. Both before/after source checks were clean. The original JSONL SHA-256 is `d94e1f329d9bf25d673da148d079cd9d4b4d4f8f03e6406fc1fcda4698270b1c`.

The initial red command exited 1 in 12.313 seconds; raw SHA-256 `f2f5015a81fa456bc9f98b6e3edbd9b46c96b116caa9125e4626660465ba68d7`. Its five failed child cases and two passing controls are retained. The tests embed the sealed syntax interfaces so that the injected concrete nodes are truly absent from the production AST switches; they do not rename a normal import failure as an unknown-node test.

## Acceptance boundary

This is a real local defect repair and a complete lower-package regression run. It does **not** yet establish all of RP16: the independent positive inventory for every approved expression, declaration and statement kind, together with the canonical native boundary and formal source-bound producer, still needs completion. RP16 remains open; related requirement IDs do not inherit a pass.

R3 integrates the tests as `261183e579ab3104deff3e3111ec84e943fe2214` and the production repair as `8cfaca263c90034914f5f3540774113332fc5c07`. These are implementation ancestors; the original receipts continue to identify their actual source commits. Full combined CI must execute on the later coherent candidate.

## Original receipts

[The evidence directory](evidence/p15-exhaustive-node-fail-closed-20261120/) retains the original JSONL, stderr and process receipts for the red and green runs, plus a manifest of exact byte hashes. There is no fabricated Apple execution or complete 179-row result.
