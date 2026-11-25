# Native differential URL dispatch handshake — 2026-11-25

## Observed ordering defect and scope

At `30b299918b5cb6594a65df99b56fced7ec3a53d6`, the native differential host
waited for `simctl openurl` to finish before publishing the import or runtime
command. The Swift helper was idle until that new command ID appeared. A URL
operation that needed the helper to handle the displayed UI could therefore
wait while the helper was never armed.

The preserved Q41 attempt04 reached readiness, verified the receiver image,
launched the receiver, then timed out opening the signed original fixture after
60 seconds. It observed zero cases and retained only the finish acknowledgment.
At 15:58:22 UTC root separately observed the exact `CHERRI_QA_Q41_R3_O` import
sheet and its `+ Add Shortcut` button on the selected simulator. No generic
Open/Allow dialog was reported, and no UI action or semantic result is inferred
from that observation. Its originals remain under
`runtime/text41-pre-handshake-observation/` on the native evidence host.

## Bounded repair

The existing `command()` helper now has an optional launch hook for the two URL
dispatches. It starts one real command in its own process group, retains launch
and process receipts, publishes the existing UI command atomically, and joins
the process within the original 60-second dispatch budget. It requires actual
exit status zero. A UI acknowledgment or matching callback cannot substitute
for that exit. Timeout, nonzero exit, or publication failure remains fatal, with
cleanup limited to the process group created by that dispatch. No implicit
retry, route fallback, simulator restart or deadline expansion was added.

The host retains `dispatch-<id>.json`, `.stdout.log` and `.stderr.log`. Receipts
record the real command, PID, requested deadline, launch, phase-publication and
completion timestamps, observed exit, timeout state and parent reaping. The
Swift helper atomically writes `phase-started-<id>.json` before invoking any
handler, so an idle helper can be distinguished from one blocked in XCUI.
Successful rows retain the dispatch receipts and the summary identifies
`launch-publish-bounded-join-v1`.

The existing signed-byte hash, expected name/library, receiver image,
callback nonce/source/name/fixture, and raw UTF-8 length/digest checks remain.
At the original handshake repair, the Swift import and runtime handler bodies
and deadlines were byte-identical. No generic consent clicks or new polling
were added. Its source comparison receipt also verifies that callback
classification and image validation functions were unchanged. The subsequent
[Add-deadline repair](native-differential-add-deadline-20261125.md) addresses
the observed import timing consequence of overlapping dispatch; it preserves
this host protocol and the runtime handler.

## Causal host checks

Evidence directory:
`/mnt/development/projects/evidence/cherri-differential-30-20261125/compatibility/native-handshake`.

An actual owned Python child refuses to exit until it reads the matching phase
from `command.json`. The same test bytes exercise import and runtime, success,
exit7 after an apparent UI acknowledgment, and timeout after an acknowledgment.
The runtime failure controls also supply an otherwise matching callback. The
child supervision is explicitly shortened to 0.8 seconds for these host-only
controls, while each test asserts that production `run()` requests 60 seconds.
No Apple command executes in these tests.

| Receipt | Actual result | Log SHA256 |
|---|---|---|
| `red.json` / `red.log` | Unchanged host `6b4f8b88…` fails all six controlled phase cases before publication; exit1 | `2ab5d8eb797e713eaa33c3d8ce97d018a50a4ccb5a425909e108e5c304e64132` |
| `green.json` / `green.log` | 24 focused tests pass, including all six actual-child cases; exit0 | `944e513557f33936f8e068031f15a39f10d4e82edce7babd2d1953178b84f2df` |

The causal test file is byte-identical between RED and GREEN, SHA256
`6462692aed80f4a2ad7f56510db967f5fe1fe2793248c6851f795fc8a17e6d50`.
Existing callback, image and restart assertions are unchanged; their command
stub was adapted only to accept and invoke the launch hook.

```sh
python3 -m unittest -v test_differential_batch test_differential_handshake
```

## Native qualification still required

The older harness documents a risk of accessibility requests starving
SpringBoard/Shortcuts IPC during URL dispatch. This repair does not increase the
existing inspection cadence, but it changes its overlap with URL delivery.
The actual native outcome must therefore be measured with the exact rebuilt
harness, retained process/phase timestamps and unchanged fixture expectations.
The runtime helper retains its 40-second timer, now starting while dispatch is
pending; an excessively slow launch can still fail closed. The cleanup receipt
proves direct-child reaping, not universal descendant quiescence. An independent
agent reviewed the sequencing, real-exit requirement, receipt and owned-group
cleanup paths and found no blocking issue.
The controlled host tests prove sequencing and failure handling only. They do
not prove Apple execution, resolve Q41 semantics, or grant a native PASS.
No Mac operation, Swift build or full Go suite was run for this component.
