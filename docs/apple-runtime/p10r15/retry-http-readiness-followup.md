# HTTP probe readiness follow-up

Run [44253843533](https://github.com/davidpovarsky/cherri/actions/runs/44253843533), at exact commit 76ad2497cacfc476a6d009163e152b7fa0846a10, failed before native orchestration. Swift evidence contracts and Go QA tests passed. The Python suite ran 98 tests with one error: the transport-helper client received ConnectionRefusedError errno61 while connecting on macOS/Python 3.15.2, before sending its GET. Shortcuts did not execute this control fixture. No native HTTP conclusion can be drawn from that run.

The follow-up found a concrete readiness defect: constructing the server immediately published a ready file and started its lifetime, before the serving loop was active. Two causal tests failed on the old helper. One showed the premature ready file; the other delayed startup beyond the active lifetime, then reproduced a real connection refusal from the expired listener.

The helper now publishes readiness from the running service loop and starts its finite active lifetime there. Unit callers wait for the same exact fixture/nonce/port readiness receipt as the native orchestrator. All 17 focused HTTP, native-graph and publication tests pass, including the new controls.

This does not accept connection refusal as the intended connection-close result, add retries, increase the 1800-second limit, relax trace/result gates, or change fixture source, native actions, signing or publication fields. The old macOS failure retained no listener state, so its exact trigger remains unknown. The causal test establishes and repairs a possible stale-readiness mechanism; the next actual control run must verify macOS execution.

Only the strict 200 control may be dispatched next. 500/404/429/transport experiments still require its actual successful callback, normal trace, primary request and continuation receipt first.

Machine-readable evidence: [retry-http-readiness-followup.json](retry-http-readiness-followup.json).
