# Native cross-run Text storage and actual publication

[Run 44428362458](https://github.com/davidpovarsky/cherri/actions/runs/44428362458) completed successfully on immutable source d4305179ca471e4b6ec8fceb87339955249dc2d5. The exact-source independent audit is PASS_BOUNDED_NATIVE_CONTRACT; all 22 published file hashes and sizes match, and all eight deliberately invalid callback/trace variants are rejected.

## Native observation

The signed 58-action fixture ran twice on the same iPad mini (A17 Pro) Simulator, iOS 27.0 with Xcode 27.1. The writer returned one fresh SUCCESS callback at 00:41:22.306 UTC; the separately dispatched reader returned one at 00:42:03.625 UTC. Both match the exact expected phase-specific TYPE_OK:VALUE_OK summary and normal NDJSON trace. Independent writer and reader oracle replays pass. The source-validated diagnostic records crossRunTextVerified=true and ordered dispatch with a common hashed simulator and storage key.

This establishes the fixture's Unicode Text type/value across two external invocations on this Simulator. Raw launch parameters and key data remain quarantined and were not independently inspected in the published audit. Deletion is only READ_BRANCH_REQUESTED_DELETE_ONLY, not verified deletion. Cache, TTL, arbitrary typed archives, restart, cross-shortcut isolation and physical iPad acceptance remain unproved. Complete editor mapping remains NEEDS_REVIEW. This run is not native proof for every later delivery commit.

The earlier a04c6fe native failure remains in the adjacent storage-44401973737 directory. The new source repairs the missing bounded READ consent-handler path. Its local regression and before/after logs are retained separately; the new positive run does not establish the cause of every earlier timeout.

## Actual privacy publication boundary

The source publisher is byte-identical to decoded-JSON repair 3bcf1f9bacc098485a69281e424e2ea377737126. Its SHA-256 is recorded by both audits. The actual published directory contains exactly the 22 manifest-listed files plus the manifest itself: no extra files, symlinks or quarantined entries. All 16 quarantined paths are absent. Every published file's bytes and SHA-256 match the source-generated manifest.

The manifest explicitly states syntheticOnly=true, pixelRedactionClaimed=false and rasterPublicationAllowed=false. This is actual evidence for fail-closed synthetic publication/quarantine after the repair; it does not claim arbitrary private-data detection, screenshot pixel redaction, private-corpus acceptance or complete OCR safety.

The first audit invocation used GitHub CLI-shaped metadata and was correctly rejected before evaluating runtime. The unchanged auditor then consumed an actual GitHub REST metadata projection and passed. The initial rejection is preserved as initial-cli-format-audit.json; no runtime oracle or expected result was changed.
