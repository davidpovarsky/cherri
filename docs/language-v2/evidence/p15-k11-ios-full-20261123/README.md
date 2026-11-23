# K11 full iOS attempt: 53/57; original D03 passed

## Actual result and source

Run [45108352089](https://github.com/davidpovarsky/cherri/actions/runs/45108352089), attempt 1, completed **FAILURE**, with `xcodebuild` exit **65**, on `9b4135e705b08f765b7304e01212d3b9658806fb`. The full original suite ran: **53 passed, 4 failed, 57 total**. The observed test identity set is exactly the same 57 as K10.

Job `137474736594` ran **2026-11-23 04:32:27–04:56:39 UTC** (24 minutes 12 seconds). The earlier HTTP403 API-quota observer abort is preserved separately from the later genuine test failure. The resumed sole observer exited 1 without timing out; bounded evidence capture and both artifact uploads succeeded. No test or workflow rerun was performed during intake.

| Failed original test | Source location | Duration | Retained classification |
| --- | --- | ---: | --- |
| `CherriEditorInputTests/testSingleCharacterPasteDoesNotConsumeGeneratedText` | Input test line 210 | 13.270s | UNCLASSIFIED issue text; original expectation precedes final equality |
| `CherriEditorUITests/testBRG27EditorRealWorldWorkflowUI` | UI helper line 369 | 35.867s | Exact startup assertion |
| `CherriEditorUITests/testE01CompletionDisplayAndInsertionUI` | UI helper line 369 | 42.576s | Exact startup assertion |
| `CherriEditorUITests/testGeneratedCloserCompletesAndOvertypesWithRealKeyboard` | Shared startup query line 215 | 58.045s | UNCLASSIFIED issue text; before typing assertions |

The original formal evidence file has **zero records**. Global iOS success, formal acceptance and whole-release acceptance are not granted.

## D03 positive component evidence

`CherriEditorUITests/testD03CurrentDocumentationSearchUI` **passed its full unchanged original test** in **97.277 seconds**. Editor readiness was observed at **7,777ms** within its 28,000ms budget. The original migration toggle/query/result path, Done/dismissal assertion and final source-preservation path completed.

The final retained editor source is exactly `show("D03_EDITOR_PRESERVED")`, **28 UTF-16 units**, with `exactUTF16Match=true`. Both original DocumentationIndex tests also passed. The bundled index has **562 sections**, bound to Docs D11 `365b08c5b4c71c6d22b591babb8d1ab852e5a3b9` and compiler S `ca316a3dee252456c4bec751594cf249c709b331`; its original bytes/hash match the bundled receipt.

This supports the previously applied compact-toggle and toolbar-visibility repair at K11. It remains component evidence inside a failed full workflow.

## Paste observations: correct final text, delayed observation

The unit fixture's original 3,000ms expectation is at line 210; the final text-equality assertion is at line 216. Only line 210 appears in the retained failure metadata.

The K11 synthetic CodeView starts with `text("x")`, UTF-16 length 9, cursor 8, attached to a key window and first responder. The paste call returns at 2ms with the original text. The native-change notification is **observed at 9,685ms** with exact expected `text("x"))`, length 10, cursor 9; the after-wait receipt at **10,905ms** still shows the correct text. There is no reported final equality failure.

For the same K10 source bytes, the prior notification was observed at 1,061ms, predicate match at 1,228ms and after-wait at 1,244ms. The existing K11 stock UITextView diagnostic runs **after** the original assertion, in a warm process: the same verified synthetic clipboard yields the exact text, a change observation at 68ms and a completed wait at 1,113ms.

These are observation times, not proof of the low-level insertion instant. Absence of a main-thread predicate-match receipt does not prove that no off-main predicate evaluation occurred. The receipts do not establish a consumed generated closer, wrong final content, or a particular actor/platform defect.

## Startup observations and missing diagnostics

BRG27's document-control expectation did not complete. Its known-controls query took 4,338ms, followed by a not-hittable creation-control observation; the wait returned false. A later diagnostic at 8,707ms saw Create Document as hittable, but the existing fail-closed policy does not authorize a fallback tap from that diagnostic. No creation tap was issued. At 28,474ms, the editor remained absent.

E01's document predicate completed at 2,745ms and the one recorded tap ran from 2,746–3,386ms. The editor remained absent through the final 28,077ms observation. Its retained app stream contains no document/workspace phase after that tap. No UI hierarchy or screenshots were retained to explain that absence.

Both app captures contain initialization, one main-queue probe and background heartbeats. The probes executed after 1,336ms and 1,962ms respectively. Background heartbeat activity does not prove continuous main-thread responsiveness, and the records do not establish a deadlock.

The generated-closer test has only its window-wait and teardown observations; no typing receipt is retained. Its reported location is the shared AX editor lookup before the source's actual typing assertions. The cause of that query failure remains unclassified.

For BRG27 and E01, each retained compact issue is **113 bytes**, SHA-256 `15c0a18fa451737b12d31fc1b5cdd2f332da2eb4173deb960068499f04f6f268`. That digest exactly matches the source-known static message:

> XCTAssertTrue failed - cherriCodeEditor must exist and be interactive in the foreground within the startup budget

The other two error texts are **not present in the retained originals**. The public sanitized log, original failed-step log, original full job log and original GitHub check annotations were inspected. The annotations add only exit 65 and runner notices. The existing strict issue capture covers only BRG27, D03 and E01; no complete xcresult bundle, screenshot or UI hierarchy was published. Neither an unknown error text nor a 58-second duration is treated as proof of a timeout.

## Source and platform bounds

The independent read-only review and mechanical intake compare **eight exact input/editor/test/dependency/workflow files** between K10 and K11; all are byte-identical. The protected UITest SHA-256 remains `9c1d5b1d3cded108be2decd85131716da143a6e5ed3fe404966c6285684ad4c8`. The only changed iOS source path is `DocumentationSearchView.swift`. No production input, closer or startup correction is justified by the current retained evidence, and none was applied.

The actual full-iOS job used **macOS 15.8.2 (24H232)**, runner image **macos-15-arm64 20261118.0550.1**, the **Xcode 16.4** executable, and an **iPhone 16 Pro Simulator**. An exact Simulator OS version was not explicitly retained in the inspected evidence. This is distinct from the separate `xcode-27` Shortcuts QA/PoC environment; no environment substitution is made.

## Original packet and remaining observers

`originals.zip` preserves both original published artifact ZIPs plus selected original GitHub run/jobs/artifact/check metadata, dispatch binding, quota-abort and resumed-observer receipts. Every member's exact size and SHA-256 is in `originals-manifest.json`.

- `ios-build-evidence` artifact **14791396122**: **82,435 bytes**, SHA-256 `9159a08b9ee9970d4c0efc93fd6d68dd58a61a112eafa8684a5d0e0b80c3a666`.
- `startup-lifecycle-observation` artifact **14790808133**: **7,289 bytes**, SHA-256 `5913707ee0f4f0b83786b810621a9c991799b7873c586364987919eb95014aa4`.

The original full workflow log (132,247 bytes, SHA-256 `17637c70a0c5342a8800cc66fbf26af160dace8dc7ae8e3ff86eba27de9ee2cc`) and failed-step log (22,809 bytes, SHA-256 `2c863de0d4e64a7494bb0191125f9ef34546d863200622d78ae72cafef4f2e1f`) stay in the Oracle evidence location. They are not republished in this packet. The original artifact's already-sanitized native log is preserved.

`remaining-observers.json` is one read-only local-file snapshot at **05:09:36 UTC**, not an additional API poll or final outcome:

| Existing run | Exact source | Last observed state |
| --- | --- | --- |
| F01 QA `45101081393` | K10 `4feadab9…` | Orchestration after 7 completed controls |
| Runtime PoC `45101192445` | K10 `4feadab9…` | Runtime step after 11 completed steps |
| Runtime PoC `45108385296` | K11 `9b4135e7…` | No setup steps yet |

All three original attempt-1 observers were active with fresh output and empty stderr, interval 60 seconds and the existing 5,100-second observation bound. No cancellation, source change or extra dispatch accompanied this intake.

## Change scope

This commit adds documentation only. Tests, assertions, deadlines, workflows, source behavior, protected bytes and release policy remain unchanged. All four failures and the positive D03 component result are retained together.
