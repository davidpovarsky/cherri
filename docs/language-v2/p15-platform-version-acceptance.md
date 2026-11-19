# P15 T07 — component versions scoped to a platform

T07 requires component comparison (`18.10` is newer than `18.2`) and independent platform facts. The former legacy float fields cannot represent all minor or patch versions and cannot establish availability on another operating system.

## Production changes

Original action definitions can carry an optional `platformVersions` map with exact string minimum and maximum bounds for each platform. The existing machine-readable catalog copies this data without sharing mutable maps. The existing schema generator validates it, emits it in stable order, and fails before publishing malformed metadata. No Apple availability facts are added by this change. Existing descriptors with no such map retain their current behavior and generated representation.

Analyzer, language service, and canonical emission use the same component comparison policy. An explicit map owns its decision: a missing platform or missing required bound produces an unverified warning and cannot inherit a generic bound or another platform's bound. The `mac` source alias selects the `macos` facet. Invalid component strings, reversed ranges, and noncanonical metadata keys are rejected. Fingerprints preserve exact platform identities and component strings; registry clones own their maps.

Canonical emission confirms that the analyzed descriptor identifies the original action before applying its platform decision. It clears superseded generic version checks only on the copied action in the existing backend transaction. Native identifiers, parameters, argument checks, and canonical builders remain in use.

## Causal baseline

Production source `69c8dcd2f866f2b57ad49128a2c563c482f765ea` with the new five initial T07 tests but without the platform implementation failed `go test -p=1 -json -count=1 -timeout=3m -run ^TestP15T07 .` on 19 November 2026. There were 3 named passing and 28 named failing test events (including parent tests). The failures demonstrated ignored component bounds, a macOS minimum incorrectly constrained by the generic legacy bound, missing unverified warnings, ignored malformed metadata, and missing semantic fingerprint data. The raw JSON-lines SHA-256 is `e8a02db95d5430668cd646d1e7a089533fd10c98ed5040b061a80e0d5a324962`.

## Acceptance producer

The T07 producer executes named production tests covering independent synthetic bounds, exact source diagnostic spans, canonical emitted fields, malformed metadata, missing-fact warnings, service document isolation, semantic identity and deep clone behavior. A sixth test follows synthetic facts from the original action definition through the actual catalog and schema-generator executable, checks the resulting action's parsed Go syntax, and verifies malformed upstream facts produce no output file.

The bounds in these tests are explicitly synthetic; they do not certify any Apple action's real availability. T07's formal contract requires a unit tier. Native runtime availability, release readiness, and all other acceptance requirements are separate obligations. Final verification receipts are recorded against the committed source rather than inferred from this document.
