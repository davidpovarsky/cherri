# P10R15 signed execution and evidence recovery

## Observed R1 results

The exact R1 source d00bc0b9e9a4d8c460623de3d883680b94289e04 was run on the Apple simulator:

- Legacy run 44223774682 returned the exact expected native summary with a fresh correlated callback, but no trace payload.
- Bool/List run 44223779379 and F01 run 44223783251 returned native error callbacks.
- All three runs failed AX tree persistence. Their sanitized coverage reports and callback artifacts are retained; no complete signed QA PASS is claimed.

## Exact producer identity defect

The actual Legacy synthetic fixture emitted a traceEnvelope producer UUID in uppercase while the following text token referenced the frontend's lowercase identifier. The V2 fixture kept the exact same string in producer and consumer. The shared debug macro expansion had changed supplied result identifiers in debugLog, debugTraceEvent and debugExportTrace.

The fix preserves every frontend-supplied result identifier verbatim. Fresh internal UUIDs keep their existing policy. It does not rewrite imported UUIDs or relax binding validation.

A new test traverses every actual ActionOutput binding in both signed functional fixtures and requires an exact preceding producer. Both fixtures have 16 checked bindings. The original compiler failed Legacy and passed V2; the patched compiler passes both. Direct mixed-case identity cases for all three macros also fail on the original compiler and pass after the fix. This is a proven compiler graph defect and is consistent with the absent native Legacy trace; a new signed Apple run is required to confirm the runtime recovery.

The corrected bounded Go run passed (package 5.500 seconds), including all TestDebug and TestP14 cases. The initial test draft used the wrong parameter key for debugTraceEvent; that test mistake and its failed run were retained. The corrected red/green run uses the real WFDebugTraceStage key. No success was inferred from the initial failed run.

## Finite AX geometry and diagnostics

The old rounding expression checked a finite CGFloat before multiplying it by ten. That multiplication can overflow for a finite extreme value, which JSONEncoder cannot serialize. The new helper preserves finite extreme values, rounds normal coordinates, replaces truly nonfinite components with zero, and marks invalid/null/infinite rectangles as invalid geometry. The same helper is used by nodes, scroll frames and viewport dimensions.

The existing artifacts did not expose the underlying encoding or write error, so this is a concrete overflow repair, not a claim that the exact R1 AX failure cause has been established. Persistence now retains an explicit phase, safe error category/code, bounded coding keys, node count and invalid-geometry count. Raw error values, localized descriptions and file paths remain excluded.

The receiver additionally records only a validated numeric native error code and payload shape counts. Existing result/error redaction, strict callback identity, exact output and trace requirements remain in force.

## Verification boundary

- Actual Windows Go regression suite: PASS.
- Existing Python callback, inspector, input and publication tests: 47 PASS.
- Workflow YAML/Bash checks: recorded with the implementation receipt.
- Swift/Foundation validator: added to the existing Apple QA workflow and executes the exact shared helper; NOT_RUN on the Windows host.
- New signed Legacy/Bool/List/F01 runtime: pending the corresponding immutable Apple runs.
- Physical device: BLOCKED_DEVICE.

The Swift validator includes an old-rounding overflow control, strict JSON nonfinite rejection, actual helper encode/decode, normal geometry controls, numeric error rejection and raw-payload absence checks. It is not a substitute for XCUI or native Shortcuts execution.

## First actual Apple validator result and repair

Run 44235137367 at source 11e3475a52b2bc64db444e917e3623a3161257f1 failed before native execution: the Foundation-only Swift files used CoreGraphics CGRect extensions without importing CoreGraphics. The original failed log is retained. Both the shared geometry helper and its command-line validator now import CoreGraphics explicitly; no assertion, finite-geometry check or runtime gate has been removed. This is a source repair awaiting the next actual Apple compile and execution. No runtime result is attributed to the failed early run.
