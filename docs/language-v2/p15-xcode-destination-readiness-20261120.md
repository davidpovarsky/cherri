# Owned Xcode destination readiness component

Implementation: 716f1bd450b3c372dde75d963512baea804bc80a. This report records a tested infrastructure correction. Native execution of the unchanged Optional diagnostic on this component has not yet occurred.

## Original blocker

Optional run 44690018966, attempt 1, used source 596415bba43bdb324b660767d4021e956ed74ab0. Xcode exited 70 because the exact owned simulator was absent from the scheme's eligible destinations. The original log is retained with SHA-256 1b5fd8af4fff241a63eaff7c21714d0628b8be5e2db496599a61731d95e0baec. No Optional callback was observed. That run remains blocked and supplies no semantic verdict.

## Change and bounds

The new gate queries the exact CherriSimulatorQAReceiver project/scheme and matches the owned simulator UUID only in Xcode's eligible iOS Simulator destinations. Missing, ambiguous, incompatible, wrong-device, placeholder, malformed and failed-query observations cannot authorize the test. A missing eligible destination permits one non-reboot Simulator activation and boot-status refresh for the same owned device, followed by one fresh query.

The readiness work is capped at 90 seconds and shares the original 900-second monotonic test phase. It passes only the remaining time to the unchanged Test02 command. It does not repeat the already-issued import, change the device, or retry the actual test. The test's exit code and timeout remain failures.

A durable, exclusive, flushed and fsynced attempt marker is reserved before launching Test02. It remains if later receipt validation or receipt-file writing fails. Two actual pre-fix controls reproduced duplicate execution (2 != 1); the repaired controls reject a second launch even after the synthetic query log is removed.

Raw destination-query outputs are retained once at fixed private builds paths, including oversized originals. Publication verifies their byte counts and SHA-256 values and reparses the exact bounded originals before publishing the small readiness receipt. Raw names and the attempt marker are outside the public allowlist.

## Verification

| Environment | Result |
|---|---|
| Linux local process/shell and parser controls | 58 passed, 0 failed, 0 skipped |
| Clean Windows source, same three Python suites | 55 passed, 0 failed, 3 POSIX-only skipped |
| Clean source Optional graph and corruption suite | 54 named passes, 0 failures, 0 skips; 22.906 seconds |

The Linux controls include real subprocess output, descendant timeout handling, and the actual shell helper route driven by a local Xcode stub. They do not execute Apple APIs. Windows skips are retained rather than reported as passes.

All five implementation Git blobs equal the reviewed source hashes in the accompanying JSON proof. The original Optional fixture and expected contract are byte-identical to source 596415bba43bdb324b660767d4021e956ed74ab0: source SHA-256 e6f195aadcd2d46f25c6fe1de5165798c18a40f4c1942785b9e39171b1cc2139, expected-contract SHA-256 da1335a22389cbe97e72a706192cb6b93ece8eff380080fa0541dcb5cbc50cef.

## Remaining actual gate

A fresh signed run on the published exact component source must establish current destination readiness, actual import, the original source-bound 399-action graph and unchanged 11-field callback contract. A ready destination or zero XCTest exit does not establish Optional semantics. Missing callback, import failure, cancellation, timeout or failed original assertions remain unsuccessful observations. No runtime or formal-179 evidence is inherited from another source.

The complete source hashes, retained-log hashes, two red controls and clean replay receipts are in [the component proof](evidence/xcode-destination-readiness-controls-20261120.json).
