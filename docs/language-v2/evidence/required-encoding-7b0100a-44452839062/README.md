# Actual WorkflowKit REQUIRED encoding gate at S2

The mandatory macOS step in [Build & Test run 44452839062](https://github.com/davidpovarsky/cherri/actions/runs/44452839062) executed the real WorkflowKit parameter-state API at exact source 7b0100a2a9ce5991ca8ffe82c805278e5aa7754e. The source-bound gate and independent strict intake **passed**. The overall Build & Test run **failed** because LANG002/ValidEnum retained an obsolete literal-only Quick Look assertion; that failure is preserved separately.

This is actual Apple parameter-state execution, not Shortcut runtime execution or a mocked engine. It addresses the CI01/RP47 required negative-CI contract at S2. It does not transfer this execution to any later compiler/release source.

## Actual process triplet

| Phase | Process exit | Required results | Observation |
| --- | --- | --- | --- |
| Positive | 0 | 23 passed, 0 failed | Authored REQUIRED set, including its expected rejection |
| Isolated corruption | 1 | 22 passed, 1 failed | Only plainString.WFStringParameterState.string EXPECT was changed from accept to reject; E_REQUIRED_ENCODING_EXPECTATION identified that exact case |
| Restored | 0 | 23 passed, 0 failed | Original set restored; report bytes exactly match the first phase |

Each phase retained the actual seven-form matrix over 50 discovered state classes (350 observed state/form cells); 23 explicitly authored expectations are mandatory. Canonical inputs remained unchanged. The observed host reports Darwin arm64, macOS 26.7.2, and the prelude is pinned to ee52c32937053268e4a035eba4b372fe77ec6411.

## Independent intake and provenance

The downloaded GitHub artifact 14496169352, required-encoding-gate, is 13,061 bytes with SHA256 f25a40b8519168cc4bbf831f63da87008f797626e86ec40652860911420053c9. Its exact 15-member allowlist was checked before extraction. The archived ZIP and all phase/receipt/stdout/stderr bytes are preserved unchanged.

All five repository input digests in the actual gate summary match their exact S2 Git blobs. The production strict intake was executed on a clean checkout of that same S2 using an external observation-only Go test overlay. CI01 and RP47 both passed the real per-form/process reconciliation; a changed implementation SHA was rejected for both records. Three named Go results passed with zero failures/skips. The archived harness relocates artifact paths only for local reading, verifies every file digest, and does not alter the original producer JSON.

The CI record paths still reflect the producer's artifact layout. Consumers may transport the bytes and rebind local paths, while retaining S2, run44452839062, and the original hashes. No source relabeling or automatic credit to K2/S3 is authorized by this archive.

The manifest binds each raw file; CRLF and process diagnostics are preserved. The exact-source observation receipt also retains the independently reproduced LANG002 baseline, which was expected to fail.
