# K10 full iOS original-result intake — 2026-11-23

## Actual outcome

[Run 45100858894](https://github.com/davidpovarsky/cherri/actions/runs/45100858894)
executed the full original iOS suite at immutable K10
`4feadab9db4d351628f78593304373631693869a`: **56 passed, 1 failed**.
All 57 terminal test identities are unique and match the earlier full K8 suite.
The sole failure is `CherriEditorUITests/testD03CurrentDocumentationSearchUI`,
after 92.556 seconds, at protected test source line 542. No full iOS or complete
D03 acceptance is granted.

The earlier compact native switch repair progressed past its original failure:
the actual after-toggle and result-wait observations have the switch on, the
exact `@count` query preserved, the migration result present, and the empty-result
state absent. The original assertions at lines 531 and 537 passed. The unchanged
test stops at the next dismissal assertion:

    XCTAssertTrue(done.waitForExistence(timeout: 5) && done.isHittable)

The retained compound assertion does not distinguish a missing Done button from
a present but non-hittable button. Neither retained log includes a Done query
trace or button frame. No UIKit defect, platform tap bug, or timeout cause is
claimed.

## Exact execution identity

| Field | Actual value |
| --- | --- |
| Workflow | `.github/workflows/ios-build.yml` / `337107435` |
| Event / attempt | `workflow_dispatch` / `1` |
| Branch | `agent/cherri-p10r15-k10-verification-20261123` |
| Tested source | `4feadab9db4d351628f78593304373631693869a` |
| Tested tree | `b53b53b9671b682944c8c5609ae66755b4635c1f` |
| Job | `137451144842` |
| Job interval | `2026-11-23T02:38:23Z`–`02:58:15Z` |
| Job duration | `19m52s` |
| xcodebuild exit / run conclusion | `65` / `failure` |
| Inputs | `startup_observation_only=false`, `documentation_component_only=false`, `runner_label=macos-15` |
| Watcher | One owner; exited `1` at `02:58:31.700722Z`; no watcher timeout |

One POST was accepted and bound to this exact source/run. The diagnostic capture
was part of this full execution, not an additional startup-only replay.
The documentation-component job was intentionally skipped by the original input.

## Bounded diagnostic reconciliation

The capture contains one invocation each for BRG27, D03, and E01, with zero rejected
metadata. All three original app event files were revalidated using the exact
committed `validate_app_events` function; their events and observer-health summaries
matched the published originals. All 11 capture source-file hashes matched K10
Git objects. The protected UI-test SHA-256 remains
`9c1d5b1d3cded108be2decd85131716da143a6e5ed3fe404966c6285684ad4c8`.

D03 reached editor readiness in 7,404 ms within the unchanged 28,000 ms budget,
with zero in-flight observations and no dropped timing entries. Its observer
recorded 45 heartbeats spanning 87,830 ms, with a largest observed gap of 3,116 ms.
These finite observations do not prove responsiveness outside their observation
interval or turn absent events into a deadlock diagnosis.

The one captured issue binds `CherriEditorUITests.swift:542`. Its compact
description is 20 bytes, is not truncated, and its digest matches the known
standard XCTest assertion prefix. The safe report has no underlying error/domain
or diagnostic signature. Detailed description and error fields have zero bytes.
No private description was republished by this intake.

| Public state | Before toggle | After toggle / result wait |
| --- | --- | --- |
| Switch | Off | On |
| Search query | Exact `@count` | Exact `@count` |
| Migration result | Absent | Present |
| Empty-result state | Present | Absent |
| Keyboard | Present | Present |

Done was not successfully activated, the final editor-source preservation
assertion was not reached, and the final D03 checkpoint was absent.
The retained `ios-test-evidence.json` has zero formal records;
documentation evidence remains `NOT_VERIFIED / RUN_NOT_ELIGIBLE`.
The startup capture also grants no acceptance records.

## Production repair, separately committed

Commit `f79ee85a6cf5b8691684d141c0127420b5e59a80` adds only the native search-toolbar
visibility policy to `ios/CherriApp/Documentation/DocumentationSearchView.swift`.
The existing Done action/label/identifier, query, migration binding, compact
switch, protected test source, assertions, and deadlines remain unchanged.

Apple documents that an active search presentation may hide toolbar content, and
provides `searchPresentationToolbarBehavior(.avoidHidingContent)` to retain it.
The API is available from iOS/iPadOS 17.1, so a small availability guard preserves
the app's existing iOS 17.0 compatibility without raising its deployment target.

Primary references:

- [Search presentation toolbar behavior](https://developer.apple.com/documentation/swiftui/view/searchpresentationtoolbarbehavior%28_%3A%29)
- [Avoid hiding content](https://developer.apple.com/documentation/swiftui/searchpresentationtoolbarbehavior/avoidhidingcontent)

This is a supported causal hypothesis for the observed active-search dismissal
failure. **The repair's native effectiveness remains unverified.** The original
D03 assertion must pass on a later explicitly bound source before closure.
The iOS 17.0 fallback retains its previous behavior; no new 17.0 runtime result is
claimed. Source review, official API availability, protected-byte comparison,
and `git diff --check` passed. No new test or native workflow was dispatched for
this intake or repair.

## Original packet and integrity

| Published artifact | ID | Original bytes | SHA-256 |
| --- | --- | --- | --- |
| iOS build evidence | `14786790193` | `75,667` | `48388483f3288ebcc07a7b9bd62977e82227c29ab9e70dbb095f429e19db94d1` |
| Bounded startup observations | `14786177571` | `8,151` | `41db3cc1431a83d0be1da2f7a5277f02a084ac27e9513e33df8e722e48c571cd` |

Both downloads matched GitHub's recorded sizes/digests and exact run/source
bindings. All ten published member byte counts/digests were checked independently.
`originals.zip` preserves both complete original published ZIPs plus selected
original GitHub/dispatch/watch metadata; `originals-manifest.json` binds every
packet member. It contains no full application/IPA, raw workflow failure log,
private XCTest description, screenshot, or unrelated historical evidence.
The original failed workflow log remains on Oracle and is represented only by
its size/digest here.

The K8 full failure at line 215 and the K8 targeted failure at line 531 remain
unchanged and separately scoped. This K10 outcome is not relabeled as evidence
for its later repair commit or another candidate. It does not establish physical
Apple-device execution, complete Shortcuts runtime acceptance, or a policy waiver.
