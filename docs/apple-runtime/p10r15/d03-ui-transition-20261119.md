# D03 migration UI transition: bounded diagnostic follow-up

Implementation: `111a948f8221f0ed927191369a61aca08aa01848`.

The actual full iOS run `44622064212` at e61 failed while waiting for a migration result. Its original receipt does not expose the switch state after tapping, so it cannot distinguish an unsuccessful control transition from a result-query problem. The exact retained index was executed through the unchanged production `DocumentationIndex.swift` with real Swift/Foundation on Windows: `@count` yielded zero default results and exactly one opted-in migration section, `fork-language/migration.md:46`. This is a search-model observation, not native UI proof.

The follow-up keeps the original current-action, unknown-query, default-hidden migration, explicit-result, and preserved-editor assertions. Before waiting for the migration result it now requires a visible/hittable initially-off switch and observes the real on state while the exact `@count` query remains present. The existing result wait and final switch assertion remain. No product search code or pinned documentation bytes change.

Three checkpoints publish only fixed Boolean control observations: before toggle, after toggle, and after the result wait. They contain no query text, labels, accessibility tree, screenshots, or arbitrary metadata. The existing documentation sanitizer validates the exact active test, exact stage, verified synthetic reset, strict JSON keys and Boolean types. These diagnostic records have their own eight-event bound and cannot substitute for test outcomes, a bundle receipt, or the existing behavioral checkpoint. The original source-receipt limit remains unchanged.

## Verified scope

The implementation checkout was clean before and after the replay. All 48 Python evidence controls passed in 6.071 seconds. The existing actual collector-adapter-to-production-Go-intake and docs-plus-integration scope controls passed 12 events in 12.748 seconds, with no skipped or failed events. Swift syntax parsing of the two changed test files passed in 1.168 seconds on Windows. This is not Apple API type-checking or XCTest execution.

The exact `@count` bundled-index unit assertion is now part of the native test source. Native UI replay of this implementation is NOT_RUN. The failure cause and any effective UI repair remain unproved until the next source-bound iOS run observes the transition. Full iOS, D03 app integration and final179 remain incomplete. The adjacent JSON preserves the exact clean-source control hashes and the original failed-bundle model observation.
