# Startup timing diagnostic: local controls only

Implementation: `37dae944bb0ba84690b4fad14096db23f1ed290a`, based on the unchanged failed component `0284f41c1783b81d5aac002c097357481f6b2308`. The original native result is run [44663872632](https://github.com/davidpovarsky/cherri/actions/runs/44663872632): 44 passed, 9 failed, including eight startup assertions and the real paste wait.

The existing records did not expose individual AX-query or first text-change times. This additive diagnostic measures the same fixed queries, waiter boundaries and document tap, without changing a query's Boolean result or issuing another interaction. The trace retains actual thread state, overlapping timings, outstanding queries and explicit capacity overflow. It contains no source, document title, screenshot or arbitrary accessibility data.

The original 28-second startup acceptance, completed wait, foreground/window/editor/hittability predicate, one-shot document creation, three-second asynchronous paste wait and exact expected text remain unchanged. All original test methods are present. No workflow, app, compiler, or compatibility pin changed.

## Actual verification on clean source

- Swift 6.3.3 compiled and executed 23 timing controls and all 37 existing deadline controls.
- The actual Swift timing encoder was decoded by the production Python diagnostic reader.
- All 34 collector tests passed, including malformed/private/out-of-case receipts, capacity, overlapping observations, delayed true results and no acceptance from timing receipts.
- The Swift frontend parsed the actual UI test and UIKit input test integration.

The clean replay receipt SHA-256 is `3e4b5dbd7b3aa60b8335e9ee5e7fa43025db847ae46decd93a507bcb31ee1aaf`; the adjacent JSON binds every command and source hash. Two incorrect manual compile invocations of the existing concatenation-based script were retained, then the clean replay used its documented concatenation form with unchanged script bytes.

## Native observation still required

The native paste notification and original predicate-match callback buffer at most five serialized receipts and write them only after the original wait. They never call `fulfill()` or alter the expected text. The optional predicate handler returns true only after the original predicate has matched.

An independent read-only review found no concrete acceptance broadening. Diagnostic observation and JSON work still add overhead; these timings are instrumented observations, not an uninstrumented performance baseline. The final trace is emitted after the actual acceptance decision. UIKit/XCTest typechecking and real Apple execution remain **NOT_RUN** for this component. No requirement or physical-device credit is granted.
