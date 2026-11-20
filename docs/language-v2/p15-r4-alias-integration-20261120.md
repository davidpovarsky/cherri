# R4 Optional alias integration checkpoint — 20 November 2026

The exact clean source 6f7ad399991a682059a76ced4245f19202a0861f combines the verified alias resolution and primitive-name precedence components with the prior R4 compiler changes. Type annotations retain the defining module's analyzed meaning through lowering; Optional aliases use the existing tagged representation. This checkpoint also retains the bounded repository preservation scout without manufacturing the missing historical payload evidence.

The combined actual selection passed 1,061 named root tests in 80.531 seconds, with zero failure/skip. It includes the complete prior alias/container selection, existing S01 tests, and provenance registry checks. Original stdout: 1,026,276 bytes, SHA256 01189d96a8847c2dc8b3f8ceb1fb65dd6e39587d6189126cce3b9b7176c0b1ea.

The six affected package selections passed 111 named tests in 6.578 seconds, with zero named failure/skip and empty stderr. The modules package has no test files; it is not counted as a named test. Original stdout: 103,638 bytes, SHA256 3dcb433c2c3c3ba6d2c5ffe5eda8863606c6567f2150b6accb76788a7954b775.

Every original raw hash, command, clean source-before/after check and named test set was independently checked before archival. The three immutable contracts are unchanged from the audited bd5 source. [Index](evidence/p15-r4-alias-integration-20261120/index.json) and [original archive](evidence/p15-r4-alias-integration-20261120/original-evidence.zip) retain the executions and complete eight-commit integration map. Every archived entry and staged Git blob was byte-verified.

This is a reviewable intermediate checkpoint, not a release promotion. Additional S01 boundary work and dynamic Optional forwarding remain on separate component branches. Final compiler/docs/consumer compatibility pins have not been regenerated; their frozen R3 values must not be relabeled as R4 evidence. Full CI, formal179, Apple app/runtime, installed consumers and deployment require their own exact-source results. B01/BRG01 remain open for the historical evidence gap documented in the preservation scout.
