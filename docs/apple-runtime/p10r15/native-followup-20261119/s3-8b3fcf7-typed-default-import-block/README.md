# S3 typed defaults: observed import block

The exact S3 native run 44459414583, typed-defaults job 135342271508, failed at the QA orchestrator on 2026-11-19 at 05:27:15 UTC. Xcode build-for-testing succeeded; the later test process exited 65. The actual XCUITest error was that com.apple.shortcuts was not running when the library cell count was requested at line 561, after import consent handling.

The signed 89-action shortcut was prepared, but the compiled shortcut was never observed to run. The only callback is the receiver self-test. The runtime oracle therefore remains BLOCKED_IMPORT with no compiled callback and no semantic trace. This is not a measured false, zero, or empty-container result, and does not resolve the typed-defaults requirement. Why the app stopped is unverified; the source independently shows no activation/state wait between the consent transition and the failing library query.

The exact sanitized public ZIP and selected raw members are preserved. All 16 published file hashes match; seven inputs remain quarantined, and no raster image is in this ZIP. The full official job log remains outside Git with its digest bound in observed-outcome.json; a bounded exact-line failure excerpt is included.

The separate native-boundary dispatch receipt records one already authorized causal diagnostic on f7f1447263ff99deb891e6044b62b270b6e8797c, child of S4. It does not retry typed-defaults and supplies no acceptance credit.

Raw bytes remain unchanged, including original CRLF; every indexed blob is checked against its on-disk digest. Original runtime oracles and source fixtures are unchanged.
