# Macro coverage producer validation

The checked-in macro-contract-coverage.json and compact Markdown are an interim executed receipt for clean compiler source ab9a264ba370890397d74042e00c11a3b4c84ffd. They are not a claim about a later integration head. Regenerate after the final compiler revision and catalog correction are frozen.

Sentinel job job_5e58ebbb8bde passed on 2026-11-18 in 13.77 seconds. It built the source, read its actual 13 macro descriptors and 505-action count, generated canonical docs, ran all 12 controlled CLI cases, and verified the output with independent native-metadata and UUID checks. The ten successful synthetic compiles each imported as V2 lossless expanded native source and recompiled with exact equality of WFWorkflowActions. No private corpus, signer or Apple runtime was used.

Five causal mutations rejected a wrong native type action, lost producer UUID, duplicated measure body, retained Release timer and wrong Repeat Results consumer. The separate actual-process verifier audit passed 19/19 checks: one healthy receipt, nine deliberately bad receipts and a restored healthy control after each. It rejects source drift, edited observations, missing child/compile/mutation evidence, premature task closure, false native action counts, false roundtrip claims and stale Markdown. YAML parsing and git diff --check passed.

The executed action counts are visible in the generated table. Require is 21 Debug / 20 Release in both dialects; measure is 7 Debug / 3 Release. Collection remains six actions for zero, one and 100 input items and for the typed dictionary-body case. These are emitted structural counts, not performance timings on Apple.

An initial producer probe used an incorrect --profile argument; its Release graph checks failed because instrumentation remained. The tool now uses the declared --build-profile option and the full real-CLI run passed. That failed probe was a harness correction, not evidence of a Release compiler defect.

## Final-source reproduction

From the frozen compiler checkout:

    python3 tools/p10r15/generate_macro_contract_coverage.py --refresh
    python3 tools/p10r15/generate_macro_contract_coverage.py --verify
    python3 tools/p10r15/verify_macro_ledger_evidence.py --repo . --ledger-dir docs/apple-runtime/p10r15 --report docs/apple-runtime/p10r15/macro-ledger-verifier-audit.json

The producer also accepts --repo and --out-dir for isolated read-only source builds. --refresh always builds and executes; --verify never silently regenerates stale evidence. It records the compiler revision before committing generated evidence and binds the actual source tree plus producer, schema/docs generators and native metadata by hash.

The existing OpenMinis CI workflow runs the producer, verifier and corruption audit into dist/macro-contracts and uploads the directory even if a gate fails. All eleven Asana tasks remain OPEN. Final native/consumer receipts must come from their actual execution, and the separate feasibility audit identifies source work that can proceed before those receipts exist.
