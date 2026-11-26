# E07 pane-transition observation retention — 2026-11-26

This successor retains the fixed synthetic diagnostics emitted by the E07 navigation repair. It adds no acceptance tier and changes no Swift fixture, assertion, deadline, test selection, or workflow.

## Source and original failure

The original full iOS run 45770108822 on provisional source f1eaa704d15d21835facb5e12b8cfe3f057274e5 observed 77 passed tests and one failed test, E07's final return-to-Code assertion. Its earlier visual edit and rejected-edit assertions passed. That observation does not establish a document-data corruption bug.

The navigation repair is commit 0471fbfb735ede0ceb291761383ded2e7785fc6e. Its R23 successor is 1cafc8cea0950edc182b1a0656c7db11a975df47. All 13 protected UI test methods and the original assertions and fixture bytes were preserved. Full iOS run 45780580372 was dispatched exactly once on that source; its native result is separate from this Python qualification.

The collector at 1caf does not recognize CHERRI_E07_PANE_TRANSITION. Consequently that original run cannot be credited with retaining the new transition JSON, even if its original XCTest assertions pass.

## Narrow change

The new diagnostic guard accepts only the protected E07 test in its exact UI test target. It binds the six original stage names to their Code/Preview targets and their ready/terminal order. The closed schema permits only fixed enums, Boolean states, bounded integer counts and timing, with at most 12 rows. Raw UTF-8 bytes are bounded before whitespace is removed. Accepted rows retain their original bytes.

Duplicate keys or stages, extra or private fields, malformed JSON, a wrong test/target, unproved selected/dual-pane states, reversed timing and oversized padded rows are rejected with a fixed public marker. Failures and original XCTest terminals remain available. Historical E07 logs without the new diagnostic rows retain their existing treatment. The existing requirement mapping and acceptance constructors are unchanged.

## Actual Python evidence

All originals are retained under /var/cache/cherri-e07-pane-transition-20261126.

| Original | Actual result | Receipt SHA-256 |
| --- | --- | --- |
| collector-red-01/result.original.json | Original collector; 15 new methods executed, 46 failed assertions/subtests demonstrating missing retention/rejection diagnostics | 05004c6296739e8dc77666c7a928adb436d3949d872e2f971801eb81babc7f22 |
| collector-padding-red-01/result.original.json | One additional raw-padding method; both leading/trailing padding cases failed before the raw-byte guard | 6e8d7b6806969c956a29044ab9eb962225712bfe7e97a0965afe73481e55a1fb |
| collector-green-01/result.original.json | Two original workflow commands: 76 collector tests and 3 upload-contract tests passed; zero skips, source unchanged, process groups absent | fda453be7786c2d3b666f3f3e041dd330fd7276635c030fe6a7531505189c7cd |

The first 15 test methods were not changed between their RED and GREEN. The additional padded-row test was also unchanged after its RED. The GREEN includes all 16 additive methods in the unchanged original collector-test discovery route.

Commands:

    python3 -m unittest discover -s ios/scripts -p 'test_collect_ios_test_evidence.py' -v
    python3 -m unittest discover -s ios/scripts -p 'test_ios_evidence_upload_contract.py' -v

Execution used the existing Oracle Python, a task-owned rootfs temporary directory, and no Go, Xcode, Simulator, signing, or additional workflow dispatch.

The original R23 review JSON and protected Swift blob remain unchanged. A full iOS run on a later joined source is still required to retain these observations from actual native execution. This source/Python component is not final M, native acceptance, or 179/179 completion.
