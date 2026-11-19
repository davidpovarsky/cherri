# Bounded editor-entry liveness recovery

## Observed failure and scope

The original typed-defaults run 44608110760 at b2961bbd51e45dd2a1b21d9fd2fb9f3390ed5f74 retained BLOCKED_IMPORT. Job 135857130208 failed while evaluating the Shortcuts editor with the actual XCTest diagnostic that com.apple.shortcuts was not running. The pre-import attempts timed out, no coordinated import request or runtime request was observed, and no non-self callback was recorded. The job log SHA256 is d6a09ebf9297ccceca96aeb5fcdb19613c2c1144a2f2248ece683175762ab490. This identifies an unsafe accessibility entry; it does not identify why Shortcuts became unavailable or prove a compiler defect.

## Change

Startup and editor entry share one activate/conditional-launch helper. Startup keeps its existing 25-second wait allowance. Editor entry permits one recovery sequence with a shared 10-second wait budget: activate once, and launch at most once only if the process remains stopped or unknown. A live background import is never terminated or relaunched. Wait success and an actual foreground-state reread are both required; a late state change, failed wait, or exceeded budget fails closed.

The editor-state function gates its first navigation-bar snapshot with a fresh app-state read. Additional state checks protect its transition to pane evaluation and the caller's next canvas inspection. A process that disappears later can still produce a genuine XCTest failure; the change does not suppress crashes or claim universal crash recovery.

Recovery reads the initial and retry import-request presence before and after the attempt. It does not rewrite requests, reset acknowledgements, re-enter import, or fabricate an imported shortcut. A changed request state or unavailable foreground returns through the existing blocked coverage report; the shell's import/runtime oracles remain unchanged. The existing 900-second host XCTest limit and 45-minute workflow limit are unchanged.

The explicit public text path editor/editor-foreground-recovery.json records fixed application-state names, bounded timings, attempt booleans, and request-presence booleans. No UI labels, user content, screenshot, raw system log, or arbitrary error description is added to publication.

## Actual local verification

Swift 6.3.3 for x86_64-unknown-windows-msvc compiled and executed the Foundation recovery contract: PASS, nine scenarios and 79 assertions. Cases include stopped-process recovery, failed/contradictory waits, background import preservation, deadline exhaustion, a foreground process lost before the query, and unchanged/changed import-request receipts. Output SHA256: ee8b351c84901be02a1705b67524f19963449825be878b728228c83d7c5f6f24.

All 17 existing publication controls passed, retaining private-canary, Unicode-escape, duplicate-key, path, symlink, raster and overwrite rejection. Output SHA256: db677cdd4a5f0e8a4fa0338d4ebac63c6290b5771aabcf9a8bd9a438e6a00e58.

The complete Apple Foundation script was also attempted on Windows and stopped at the pre-existing EvidenceGeometry.swift import of unavailable CoreGraphics. That platform result is retained, SHA256 266f4a80f608fd65f4278672330b96d64137a8bfa35e8476afe93ba3da6c64d3; it is not a successful Apple build. The new Foundation-only helper was independently compiled and executed as described above.

## Remaining gate

Actual Apple XCTest compilation and a source-bound replay of the unchanged binding capture and typed-defaults fixtures remain required. The guard alone is not an import fix, does not prove typed-defaults semantics, and grants no formal acceptance or device credit.

## Review follow-up

A read-only review identified additional AX entry groups after editor-navigation taps/sleeps. The follow-up adds stop-only fresh foreground checks to those groups and inner header counts, without another recovery attempt. The actual Foundation control now executes one ready query, changes the injected state to notRunning, and verifies that all three later query closures remain uncalled. The fresh Swift build/run passed nine scenarios and83 assertions; output SHA256 9b2c8803a3a65dfe271fa0caf6d820197417dcba8c4cdebd5063c2b977735bd0. Apple XCTest compilation and paired runtime replay remain pending.
