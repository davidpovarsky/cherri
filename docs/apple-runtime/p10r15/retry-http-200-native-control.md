# Native HTTP 200 control: actual continuation verified

Run [44334461645](https://github.com/davidpovarsky/cherri/actions/runs/44334461645), attempt 1, executed the existing signed qa-retry-control-v2 fixture at immutable compiler commit 52284f8ad179997bf35d41dde21a758dfc611059. The workflow completed successfully on the iPad mini (A17 Pro) iOS 27.0 simulator, with Xcode 27.1 (27A9275) on macOS 27.0.1 (26A434).

The independent audit verified all 21 published file hashes and byte counts, the publication manifest, and absence of every quarantined file. The committed JSON receipt lists only the bounded published evidence. No image, raw server receipt, startup state, or server log was copied into this report.

## Observed native behavior

The compiled shortcut produced exactly CHERRI_QA_RETRY_CONTROL_RESULT:P15_HTTP_BODY_OK in one fresh, correlated SUCCESS callback. Its fixture, compiler commit, source hash, compiled plist hash, and invocation nonce agree across the assertion and HTTP diagnostic records. Both inline events (before and after) and both collector events (begin and end) were present and verified.

The loopback server observed exactly one primary GET with status 200 followed by exactly one continuation GET with status 200. Both response writes completed. The independently reported body digests match the fixed synthetic primary and continuation bodies. The finite server then stopped, with serverState STOPPED. The published diagnostic reports CONTROL_VERIFIED and ordinaryRuntimeContractVerified true.

| Evidence | Value |
| --- | --- |
| Source SHA-256 | 60bfbb0a3e96cf736dd6089949b0ff9532600e0dc91b61eae3cc662f426e3ed4 |
| Signed-run unsigned plist SHA-256 | 190c46f75978006af72258a05a4bc4bec31ab7d1cec77a7acd3e1705bd93aed8 |
| Native actions | 16 |
| Primary / continuation requests | 1 / 1 |
| Inline / collector trace events | 2 / 2 |
| Verified published files | 21 |

## Scope and remaining work

This proves the native HTTP success and continuation path for this exact fixture and simulator run. It does not prove that transport failures can be caught, that the same request exposes its HTTP status as a program value, or that any retry macro exists. All three remain explicitly unproved; retryImplemented remains false.

HTTP 500, 404, 429, and deliberate connection-close probes were dispatched only after this control audit, from the same immutable source. Their ordinary runtime and trace expectations remain strict: native ERROR or CANCELLED is a failed runtime contract, even when a separate diagnostic can establish a useful failure boundary. Multiple observed GETs would not by themselves prove compiler retries.

The successful revised helper does not establish the original cause of the earlier unobserved startup failure. Editor Inspector remains NEEDS_REVIEW and physical-device verification remains BLOCKED_DEVICE. No full release readiness or typed per-action inspection is claimed.
