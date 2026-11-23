# Frozen K8 full iOS run and missing startup diagnostic

## Actual result

[Run 45093282117](https://github.com/davidpovarsky/cherri/actions/runs/45093282117), attempt 1, executed the full ios-build.yml selection at exact K8 2c54ebd50fa30597cbdd6337e6ecc6b65274a903. Job 137427414760 finished in 24m24s with failure; xcodebuild exited 65.

The original sanitized log and diagnostic JSON reconcile exactly on 57 unique terminal test results: **56 passed, one failed**. The UI subset is 12 passed and one failed. BRG27 and E01 original startup methods passed; testD03CurrentDocumentationSearchUI failed in 19.273 seconds. This is a failed full run. Its formal evidence records correctly remain empty. Individual observed passes are not promoted into a full-run or final-acceptance claim.

Simulator app build/upload and the bounded evidence collector succeeded. The unsigned iPhone build was skipped after the test failure.

## Sole observed failure

The D03 failure occurred before documentation interaction, at line 215 of CherriEditorUITests.swift, inside the typed cherriCodeEditor query using allElementsBoundByAccessibilityElement during document-wait. The initial snapshot observed a foreground app and a window but no creation/editor controls.

The last startup trace records 7,611 ms elapsed, one in-flight observation, and the unchanged 28,000 ms startup budget. The error classifier reports UNCLASSIFIED. These observations identify the failing call; they do not establish a timeout, main-thread deadlock, an absent editor after the full budget, or a documentation-search defect.

The original XCTIssue description and xcresult were not retained in the completed run's published artifacts. The complete terminal artifact metadata lists only the pre-test simulator app and the six-member bounded evidence ZIP. That ZIP contains a sanitized log, test diagnostics, an empty formal-record envelope and three Docs bundle files. No original private failure text can be recovered from those retained bytes.

The Docs bundle is nevertheless bound to D11 365b08c5b4c71c6d22b591babb8d1ab852e5a3b9 and R7 compiler ca316a3dee252456c4bec751594cf249c709b331: 535,208 bytes, 562 sections, SHA-256 169cd0831c58b639a5c443625b0b1e792a84a704314e9e08b3a36fa0b357b9cc. The two existing Docs service tests passed; the D03 UI method did not reach its documentation assertions.

## Minimal observability repair

The existing UI override already emits bounded, nonce-bound XCTIssue/session observations for exactly three fixed original UI tests. The existing safe capture converts them into allowlisted issue type, error domain/code, diagnostic signatures, description digests and bounded lifecycle events. Private descriptions and container-query output remain quarantined.

The workflow previously ran that capture only for startup_observation_only=true. The requested full run generated the source observations but skipped the capture. The one-line condition change runs the same existing capture after full runs as well. It changes no UI/source observer, test identity, assertion, readiness deadline, token/nonce rule, data class, privacy filter or acceptance condition.

The capture/collector/policy source bytes are unchanged from K8, where their existing control steps passed in this very run. No additional tests were introduced or rerun for this reversible condition change. The next run must supply actual issue metadata before any AX-query repair can be claimed.

The protected UI file remains SHA-256 9c1d5b1d3cded108be2decd85131716da143a6e5ed3fe404966c6285684ad4c8. The inactive R23 proposal is unaffected; no waiver or policy change is applied.

## Targeted diagnostic selection

documentation_component_only=true performs generator tests and standalone SwiftUI type-checking, and skips the app/UI job. It cannot reach this failure.

startup_observation_only=true selects the original BRG27, D03 and E01 UI methods, preserves their assertions and budgets, and emits diagnostic-only evidence. It is the existing suitable bounded selection for collecting the missing cause after exact source/ref coordination. This intake did not dispatch another run or repeat the other 56 tests.

## Originals

The complete small ios-build-evidence artifact is retained unchanged: artifact 14783475575, 75,116 bytes, SHA-256 5dc25d444c97931145b8dfd9a9bf55faf96961e016f4d35c1f1adafe94a56d25. The original run/jobs/artifacts metadata and failed-step log are also retained, with file digests in originals-manifest.json. No simulator-app binary, screenshot, private text or historical archive is republished here.

The failed-step log is archived without changing its original trailing whitespace; its member digest remains f61b144586f6bddc6ab88411526c23e2bb4dbc2b1e3bb2bb5834f6f9010e0b03.
