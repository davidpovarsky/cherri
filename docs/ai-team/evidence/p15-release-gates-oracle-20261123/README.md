# Oracle release gate execution, 2026-11-23

The unchanged 56-gate registry ran sequentially on clean source
1c07e3fee1368d7379616143168c072c79ffe8ad, using Go 1.24.13 on Linux ARM64.
The source tree remained unchanged throughout the execution. The original
runner exited 1 after 451.427 seconds; it did not time out.

## Result

- 54 gates passed.
- R22 is BLOCKED_PLATFORM: this Oracle host has no macOS/iOS Simulator runtime.
  Physical iPad acceptance remains a separate requirement.
- R23 failed: the protected iOS UI test file differs from the unchanged
  76d722502e20db977e84a095444d0172442655b8 baseline.
- Overall verdict: NOT_READY. This packet does not grant release acceptance.

The original JSON report is copied byte for byte into the existing
docs/apple-runtime/p10/p10r3-gate-results.json consumer path. Its historical
milestone field and all failed/blocked results are retained without alteration.
The source commit is a real ancestor, and the dependency fingerprint matches
the actual catalog and ledger generation. The R23 diagnosis is recorded
separately in ../p15-r23-oracle-disposition-20261123/.

The manifest binds the original execution start/end receipts, stdout, stderr,
and result. No private corpus run, full cross-platform CI run, or formal
179-requirement acceptance pass is implied by this gate execution.
