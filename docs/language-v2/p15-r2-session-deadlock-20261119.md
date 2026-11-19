# R2 full-suite session deadlock and R3 repair — 2026-11-19

R2 GitHub run [44634808653](https://github.com/davidpovarsky/cherri/actions/runs/44634808653), Linux job 135950466263, failed the full Go suite at the unchanged 15-minute timeout. The exact source is 4e56dbcfe895baa8f944b83715ecff0d07bfd24e. The original 794,724-byte job log has SHA-256 b7ee75c55612baf1151ef2d07ceeeda005230306753fdcc578b22059f9db762c and is retained unchanged with the original API metadata.

The timeout stack shows TestP10_RegexQuantifiersPreserved waiting in compileLegacySourceToRawMapInMemoryWithProfile (hybrid_import.go:461). The test had obtained a V2 CanonicalBackendSession and deferred Close until the end of the test, then entered the independent Legacy compiler. Both correctly own the same shared-state mutex, so this test nested ownership of a non-reentrant resource.

Implementation **9ee950b137071df84a9261b28fb2197fa43ddf54** closes the V2 session immediately after its unchanged V2 regex assertions and before the Legacy compiler call. Deferred Close remains as failure-path cleanup and is idempotent. No production mutex, compiler behavior, fixture, native value expectation or timeout was relaxed.

The clean exact-source replay ran TestP10_RegexQuantifiersPreserved and TestForkActionProvenanceRegistry with a two-minute test timeout. Both passed, no failures/skips, exit 0, clean worktree after; total process 46.938 seconds. The formerly blocked regex test itself completed in 0.03 seconds. Original Go JSON SHA-256: b0fd1e1be896cfe6d6a553dd32cf34761b6fac2633e7dfee60998b67b3b083ad.

This focused result also validates the integrated F02 provenance pointer's ancestry. It does not turn the failed R2 full suite into a pass. Other session call sites are being audited, and a complete combined CI run is still required. Original log whitespace is preserved; it is validated by digest, while authored files receive normal whitespace checks.

## Session-helper audit follow-up

A read-only audit of the direct session constructors and compile helpers found no further actual nested in-process compiler call before Close. Four negative compileV2Helper call sites discard the returned session because they expect rejection; if a future regression unexpectedly succeeds, their Fatal path must still release it. Implementation 9cdc4b9926fad5a08172919eae8da1069f4a0fcc registers the existing idempotent Close with t.Cleanup at construction. Existing explicit closes remain.

The clean selected replay executed all five required parent tests (regex, array rejection, conditional rejection, JSON/native/local HTTP contract, provenance integrity), with ten named passes, no failures/skips/missing selections, exit 0 and clean worktree after. Actual process 51.219 seconds; raw SHA-256 ffe6d20f8627f7a6b8e911019998fa39899508147bc8b216683404d4735e7852. This validates the existing positive/negative contracts and new integrated label/guard provenance. The original failed R2 CI remains historical failure; full combined CI is still required.

## macOS terminal result and cumulative acceptance budget

The same R2 workflow also failed macOS job 135950465890. Its original 806,550-byte log has SHA-256 9c8d91d6a60bfe1d33153f75fdd4092e25b21ab825a56e4c5331d340a127559b and is retained with original API metadata. It independently shows the same regex session lock, and a separate 900-second timeout of the tools/language-acceptance package as TestP15VerificationCLIInvalidInputs had just started (0 seconds). The complete 179-contract production test alone had passed in 426.98 seconds; many other complete producer families passed before the cumulative package budget expired. This second timeout is not attributed to the regex mutex.

Implementation 1bc91c97aa255e0f7d07a1aec1d9e23ad16cb301 lists every actual Go package and runs each exactly once. All ordinary packages retain 15 minutes; only tools/language-acceptance gets 35 minutes for its cumulative full suite. The runner executes the second group even if the first fails, propagates either failure, and retains original command/exit/time/inventory/source-status receipts. No test selector, assertion, child producer budget, matrix gate or private-corpus boundary changed.

Six causal runner controls passed, including dropped/duplicated inventory and failure propagation. Actual go list discovered 30 packages; YAML validation confirms both Linux and macOS use the exact runner. These are planning/process controls, not an actual full Go suite: actual_go_suite_executed=false is retained. Fresh full CI is still required. Verification including actual package enumeration took 9.469 seconds; raw controls SHA-256 04bdf73c51c559e696a61114cd429e1589f0c2dd7d257f6db2df1c1ef95eaa04.
