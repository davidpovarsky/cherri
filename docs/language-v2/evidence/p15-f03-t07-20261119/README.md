# F03/T07 verification receipts — 19 November 2026

The clean committed source `27547c55b330dac9085d710597ce9420aab5ab8c` passed both registered production evidence producers and the F04 non-promotion guard. The production receipts bind F03 to unit and serialized native structure, and T07 to unit. They include exact named commands, producer IDs, source SHA, exit codes, and bounded reports. The reports each prove one selected immutable contract and do not claim project closure.

The affected analyzer, lowerer, schema and service packages passed 75 named tests on `7bb99c41eeb20efeccfecdfd198d847eeeb444a7`. The next commit only aliases the Go syntax token import in the added propagation test. The first formal attempt failed on that import collision; its raw log is retained. The clean retry passed all three gate tests.

Causal baseline logs and their exact test sources are retained. The initial F03 baseline predates two corrected wire-oracle assumptions; its direct None and missing-path failures demonstrate production defects, while the working-copy fix log records the corrected 51-test outcome. T07's baseline tests run against unchanged production 69c8dcd and reject the legacy behavior in 28 named failing events. The final T07 producer also executes the original-definition/catalog/generator propagation and rejection test.

`artifact-manifest.json` hashes every stored raw log, summary, baseline source, and extracted complete producer receipt. `production-receipts.json` reconstructs the complete JSON receipts from Go's split output events. The raw summary preserves the original split chunks; use the extracted receipt or raw JSON-lines log for full content.

F04's nested and empty-list cardinality remains unverified pending signed Apple runtime execution. These files do not update the repository's full 179-requirement aggregate.
