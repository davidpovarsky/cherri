# Original full iOS component replay: 44683546015

The exact fe5c937d76ed58204496a76d121c326cde3241cc component completed with **41 passed and 12 failed tests**. All twelve positive editor UI tests failed before their substantive body; the negative missing-element control passed. The original collector emitted zero formal acceptance records. D03 UI acceptance remains unverified.

The original real UIKit paste test passed its unchanged three-second wait and exact-text assertion. Actual matching text was observed at 2294 ms, the predicate at 2458 ms and completed wait at 2657 ms. The later stock UITextView diagnostic is a separate warm-process observation, not a causal performance comparison.

## Exact failure boundary

Five failures are at the filtered accessibility array query (source line 120), one at reading the returned element type (line 122), and six at the unchanged final readiness assertion (line 247). The original privacy sanitizer did not publish the XCTest error descriptions for those six query failures. No stale-element, ambiguity or application-crash explanation is asserted.

Post-tap known-control queries took 12.4–13.7 seconds for D03, palette and diagnostics scenarios. Two creation taps took approximately seventeen seconds; their editors became visible only after the 28-second acceptance budget. These late observations remain failures. One typed-source scenario never issued the creation tap because its later observation found the button non-hittable.

## Provenance and scope

The companion JSON retains all 53 test outcomes, twelve source locations, 26 startup receipts, six timing traces, original paste and separate stock receipts, and every published artifact hash. Original ZIP, API metadata and job log remain retained unchanged. No screenshot, restored document text or broad accessibility hierarchy is added.

- Run: https://github.com/davidpovarsky/cherri/actions/runs/44683546015
- Artifact: 14609760852, ZIP SHA-256 c5d935e892abf654c39136b081bd6aad3897c9fecc3f4eceed9b97b0baafdbb1
- Result: FAILURE, exit 65, zero formal records

Eager empty-preview WebKit construction is a code-level optimization candidate. This run does not prove it caused the failures.
