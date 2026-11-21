# P10R15 Apple recovery components â€” 21 November 2026

Project acceptance remains open. These are actual local component checks,
not Apple Simulator, signing, physical-device, or formal 179 acceptance.

## Receiver project prerequisite

Source `124571a4e88f4871fbf5e4947bae23aacc27a01c` repairs the original
three 20-Nov RED controls: nonzero Homebrew and XcodeGen exits and an absent
project could previously be hidden by the later successful Info.plist check.
The shared finite process runner now retains private original stdout/stderr,
rejects an existing generated project, binds helper/spec to actual Git source,
requires the fresh exact project.pbxproj and receiver URL scheme, and stops
before Xcode on any failure. The prerequisite permits 180s installation and 60s
generation within 240s; existing 900s test02 and 45m workflow budgets stay intact.

Clean actual replay: 89 discovered tests, 85 PASS, 4 explicitly existing POSIX
skips, 0 FAIL; 25.172s. Bash syntax passed and source remained clean.
The additional controls include a positive exact fresh project, raw error and
exit-code retention, changed source spec, stale project and wrong URL scheme.
Original 20-Nov RED bytes and 21-Nov candidate output remain separate on disk.

## Main queue observation

Source `9d7c35f00a58fccafe000784942360b0e155c029` adds exactly one event-only
main queue request per existing startup observer. Background heartbeat is still
reported separately. The execution event records actual time and thread; its
absence does not prove a blocked main thread or missing lifecycle event.
The existing 96-event, 64KiB and 120s observer caps remain unchanged. No AX query,
document creation, readiness decision, test method, 28s readiness budget or 3s
paste expectation was changed.

Clean actual replay: Swift 5 compilation succeeded without warnings;
79 standalone assertions passed. The controlled scheduler verifies one pending
request, exact response, no fabricated record and expired response rejection.
62 Python capture/collector tests passed with no skips, including seven causal
probe ordering/identity corruptions. Source remained clean. Windows portable
controls do not establish Apple's main queue or SwiftUI execution.

## Evidence and next gate

The accompanying JSON binds original local process receipts and output files by
size and SHA256. Original private paths/logs stay in the local Apple analysis
workspace; no private device payload was published. The source commits are
separate component identities and do not certify the combined release source.

Next: integrate these components into one candidate, run the original three
startup UI tests once with source-bound observation, and execute signed Apple
QA against that candidate. Previous startup 0/3 and File/Optional cancellations
remain actual failures/unverified observations until new native evidence exists.
