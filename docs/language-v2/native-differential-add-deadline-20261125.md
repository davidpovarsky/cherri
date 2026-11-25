# Differential import Add deadline — 2026-11-25

## Observed failure

The coordinator's preserved Q41 attempt08 used the async dispatch harness at
`21ead0aed1031de7a23d5e0155efc02252f02fcb`. Its existing 15-second
`waitForExistence` began while the file URL dispatch was still running.
Accessibility calls blocked long enough for the helper to report
`IMPORT_BUTTON_MISSING` even though delivery subsequently completed.

The dispatch started at `2026-11-25T16:48:39.518635+00:00`; the phase was
published at `16:48:39.521319`. The actual exit-zero observation was
`2026-11-25T16:49:10.556803+00:00`, with a retained elapsed duration of
31.042282 seconds. The helper reported its failure at 16:49:13. The
coordinator then observed the exact `CHERRI_QA_Q41_R7_L` import sheet and
blue `+ Add Shortcut` button. The screenshot SHA-256 is
`45b4c52b667efc39a31a8fbaa6478ec62ee4b389349ee76b97a47a9daf838b67`.

The original evidence remains on the native host under
`runtime/differential-text41-verified-08-isolated-legacy-oldsignature`.
That failed run is the actual old-code RED evidence. Foundation controls
below do not execute or impersonate that old implementation. No failure is
retroactively promoted, and this timing observation does not establish a
compiler defect or a native execution result.

## Dispatch-aware timing

`XcodeProject/Shared/DifferentialImportTiming.swift` reads only the
`dispatch-<current-command-id>.json` selected by the helper. The command ID
and run directory remain those of the existing host protocol.

| Current observation | Add search policy |
|---|---|
| Valid `LAUNCHED` receipt | Inspect and handle the name-verified Add button while the original dispatch deadline remains open. |
| Valid final `EXITED` receipt | Deadline is `max(actual Add-search start, exitObservedUTC) + 15 seconds`. |
| Pending at or beyond `startedUTC + 60 seconds` | Fail closed; no second dispatch or extended host budget. |
| Missing, malformed, changed, failed, nonzero, timed-out or late receipt | Fail closed, including after earlier valid observations. |
| An accessibility result returns at or beyond its applicable deadline | Reject the result; a blocking query does not extend the deadline. |

The pending budget is the existing host's 60 seconds. It is not added to an
unconditional 15-second wait. Fast delivery retains the original 15 seconds
from the actual search start. A delayed exit starts the 15-second Add window
at the host's retained exit observation, not at a later receipt read.

The parser accepts the host's fractional ISO8601 UTC timestamps. A successful
receipt requires schema 1, the existing file-URL dispatch command, the unchanged
60-second budget, a stable command/PID/start identity, exit code zero, process
return code zero, parent reaping and ordered timestamps. Exit time and elapsed
duration must fit the original budget. Initial `LAUNCHED` receipts may omit
`phasePublishedUTC`: the host writes that timestamp after publishing
`command.json`. Final successful receipts must contain it.

The helper rereads the exact receipt and clock after potentially blocking
accessibility queries, and before accepting or tapping the observed Add
control. It continues to require the existing exact fixture identity on the
sheet and exact library identity. Add handling can occur while dispatch is
pending so that the delivery/consent dependency cannot recreate the earlier
serial deadlock. `IMPORTED` additionally requires the final valid exit-zero
receipt. The import acknowledgment includes an `importTiming` record with
the policy identifier, receipt filename, search/check timestamps, dispatch
observation, applicable deadline and observed/tapped times when reached.

The existing activation behavior, 30-second library window and 40-second
runtime-consent handler remain. There is no generic consent fallback, extra
`openurl`, retry, restart or settling sleep. The Python host, signed-byte
binding, receiver image validation and callback nonce/name/fixture/raw UTF-8
digest criteria are unchanged.

## Verification

The Foundation executable runs the production Swift parser and timing policy
against fixed receipts and clock observations. Controls cover delayed
31-second successful delivery, fast delivery, search starting after exit,
the pending deadline, repeated reads, blocking-query boundary observations,
missing/malformed/changed receipts, failed/nonzero/timed-out delivery,
unreaped or inconsistent exit records, late delivery, fractional Python
timestamps and the publication timestamp race. These are policy controls,
not native UI or runtime acceptance.

On the authorized Mac runner:

```sh
swiftc -parse-as-library \
  tools/shortcuts-simulator-qa/XcodeProject/Shared/DifferentialImportTiming.swift \
  tools/shortcuts-simulator-qa/verify_differential_import_timing.swift \
  -o /absolute/task-scratch/qa-differential-import-timing
/absolute/task-scratch/qa-differential-import-timing
```

The control is also included in `verify_foundation_contracts.sh`. The
unchanged host controls remain:

```sh
python3 -m unittest -v test_differential_batch test_differential_handshake
```

A fresh exact-source harness build and native run are separate required
evidence. The coordinator owns those Mac operations. The existing attempt08
failure and screenshot remain evidence even if a later fresh run passes.
