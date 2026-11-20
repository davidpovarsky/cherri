# Editor startup orchestration and paste diagnostic — 2026-11-20

Status: local source checks passed; Apple SDK build and corrected UI execution remain pending. No requirement is closed by this report.

## Original native failure

The unchanged original [iOS run 44675053289](https://github.com/davidpovarsky/cherri/actions/runs/44675053289) at source b62a18469c66028d3310aecda5b0b9abbd43ddff ended with xcodebuild exit 65: 41 passed tests and 12 failed tests. Eleven UI failures occurred in the shared startup readiness assertion; the other failure was the real UIKit single-character CodeView paste expectation. Its original ZIP and all published bytes remain retained.

Artifact 14603654217 is 74801 bytes, SHA256 8d9375ff575725da9a9b9d9f419a927f7252e5ed7df90c215afe276b0745183e. The accompanying JSON records exact metadata, file digests, all twelve startup observations and the four original paste receipts.

Repeated absent-editor accessibility queries took 5–15 seconds. Six failed starts exposed an interactive editor only in a late final snapshot. The successful E03 startup completed at 24,927ms; late snapshots do not replace completed readiness inside the original 28,000ms budget.

Paste returned without editing at 31ms, emitted the exact expected text at 3,592ms, and was observed after the failed wait at 8,283ms. This does not prove generated-closer corruption, and it does not satisfy the original 3,000ms expectation.

## Source correction

Implementation: 0f2898941246245040221d0b2753ebaee4e60113. The production app, compiler, dependency pins, workflow and documentation compatibility tuple are unchanged.

A single query restricted to the fixed Create Document button and cherriCodeEditor text view supplies current control presence. Results are bound to accessibility identities. The same unique matched creation control is used for the one allowed tap. Zero matches, duplicates, unknown controls, a co-present editor, missing foreground/window or an expired deadline cannot authorize creation. After an actual wait, controls are observed again before acting.

The final real wait still requires foreground, window, editor existence and editor hittability. A completed wait and a fresh final observation must satisfy the same monotonic 28-second acceptance deadline. The change removes redundant negative control queries; it does not accept late state or fabricate readiness.

The original CodeView paste test body through its three-second wait and exact-text assertion was byte-compared unchanged. Only afterwards does a new stock UITextView/window receive the same synthetic text, selection and clipboard. Its separate three-second expectation is explicitly a diagnostic in an already warm process. It neither primes nor replaces the original case, and its receipts cannot promote failed tests to acceptance.

## Verified local scope

On a clean implementation checkout, Swift 6.3.3 executed 81 state/deadline controls and 24 timing controls. Both XCTest files parsed. All 40 production collector tests passed, including private-content, wrong-scope, stage, ordering, count and failed-run rejection controls. The real Swift encoder output was also passed through the production Python collector; the synthetic failure remained a failure with zero acceptance records.

Clean receipt SHA256: 1100f2da04c08f9779c2144c42f6e74c3058aa736ee32ce6217b65b4b17a78b9. Source hashes were identical before and after the run.

Windows parsing does not provide an Apple SDK typecheck. The next required evidence is one exact-source full Intel iOS workflow, preserving the existing 28-second startup and three-second CodeView paste assertions. Its result must be retained independently; no local or earlier native result can be inherited as completion.

## API references

- [Immediate accessibility-bound query](https://developer.apple.com/documentation/xcuiautomation/xcuielementquery/allelementsboundbyaccessibilityelement)
- [Format-string query predicates](https://developer.apple.com/documentation/xcuiautomation/xcuielementquery/matching(_:))
- [Bounded asynchronous XCTWaiter](https://developer.apple.com/documentation/xctest/xctwaiter/fulfillment(of:timeout:enforceorder:)-swift.type.method)
