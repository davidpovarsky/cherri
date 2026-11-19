# R2 full-suite session deadlock and R3 repair — 2026-11-19

R2 GitHub run [44634808653](https://github.com/davidpovarsky/cherri/actions/runs/44634808653), Linux job 135950466263, failed the full Go suite at the unchanged 15-minute timeout. The exact source is 4e56dbcfe895baa8f944b83715ecff0d07bfd24e. The original 794,724-byte job log has SHA-256 b7ee75c55612baf1151ef2d07ceeeda005230306753fdcc578b22059f9db762c and is retained unchanged with the original API metadata.

The timeout stack shows TestP10_RegexQuantifiersPreserved waiting in compileLegacySourceToRawMapInMemoryWithProfile (hybrid_import.go:461). The test had obtained a V2 CanonicalBackendSession and deferred Close until the end of the test, then entered the independent Legacy compiler. Both correctly own the same shared-state mutex, so this test nested ownership of a non-reentrant resource.

Implementation **9ee950b137071df84a9261b28fb2197fa43ddf54** closes the V2 session immediately after its unchanged V2 regex assertions and before the Legacy compiler call. Deferred Close remains as failure-path cleanup and is idempotent. No production mutex, compiler behavior, fixture, native value expectation or timeout was relaxed.

The clean exact-source replay ran TestP10_RegexQuantifiersPreserved and TestForkActionProvenanceRegistry with a two-minute test timeout. Both passed, no failures/skips, exit 0, clean worktree after; total process 46.938 seconds. The formerly blocked regex test itself completed in 0.03 seconds. Original Go JSON SHA-256: b0fd1e1be896cfe6d6a553dd32cf34761b6fac2633e7dfee60998b67b3b083ad.

This focused result also validates the integrated F02 provenance pointer's ancestry. It does not turn the failed R2 full suite into a pass. Other session call sites are being audited, and a complete combined CI run is still required. Original log whitespace is preserved; it is validated by digest, while authored files receive normal whitespace checks.
