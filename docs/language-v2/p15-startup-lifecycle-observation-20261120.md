# Owned startup lifecycle observation — 2026-11-20

## Scope and original failure

This component adds diagnostics for three existing UI tests. It does not repair or
declare the editor startup successful. The preserved native baseline is
[run 44716854171](https://github.com/davidpovarsky/cherri/actions/runs/44716854171),
source `fb3b6434e2d338d5595a589416d15078cfeacc6a`: 44 observed passes and
12 failures among 56 executed tests. The UI source still declares all 13 original
test methods; the negative missing-element test passed and the 12 feature tests
failed before verified source reset.

The reviewed implementation chain is `e86a794066faf4c29323c271477ec0cb52fa8475`
followed by `e0a50b8f5d49e0f5cf55c74b5da557d3e2b89a4d`. The latter adds actual-Git-object
source binding and its causal controls. Native execution of this new component is
**NOT_RUN** at this proof checkpoint.

## What the component records

For the original BRG27, D03 and E01 UI tests, the test runner supplies a fresh
32-character nonce and exact test identity to its app launch. Only those identities
plus the existing `--uitesting` argument enable the app observer.

The observer records app initialization, document value construction/read/write,
Workspace and editor-host appearance, and attachment of an owned zero-size,
noninteractive UIKit witness. Opaque construction identifiers travel with the
document value; they are not filesystem identities. The witness updates its current
document association explicitly, and reports a change separately from an actual
window attachment. It does not inspect the upstream editor internals or prove that
a text view is present, hittable or ready.

An independent utility queue records heartbeat events. Each app observation is
limited to 96 events, 65,536 bytes and a 120-second recording lifetime. Missing
heartbeats or a missing file remain missing evidence. They do not establish that
the app failed to reach a stage. Instrumentation adds some overhead; its timestamps
are diagnostic observations, not an uninstrumented performance baseline.

XCTest issue metadata retains the actual error code, reviewed domain, source line,
fixed diagnostic signatures and digests/lengths of bounded description bytes.
Descriptions and the container-query output stay in the private job quarantine.
No document text, filename, arbitrary accessibility tree or raster is read by this
new capture path.

## Boundaries preserved

The entire `resetEditor` suffix, including every feature-test body, is byte-identical
to the baseline. Its SHA-256 is
`5c96450d7dd7a7ae2861656c481c7f7242bbea8527e0d1c2fd67124862a1f9fb`.
The original deadline helper, input tests (including the three-second paste
assertion), and compatibility manifest are also byte-identical.

The readiness decision still uses the original 28-second allowance, completed
wait, current foreground/window/editor/hittability conditions and one Create tap.
No observation satisfies an expectation or changes a control result.

The existing workflow's new `startup_observation_only=true` input selects:

- `CherriEditorUITests/testBRG27EditorRealWorldWorkflowUI`
- `CherriEditorUITests/testD03CurrentDocumentationSearchUI`
- `CherriEditorUITests/testE01CompletionDisplayAndInsertionUI`

That component run retains its actual test/process results and emits **zero formal
acceptance records**, even if all selected tests pass. The default full selection
remains available; the new observation is not full-suite or final-candidate proof.

## Source and privacy checks

The collector resolves only each fresh nonce's fixed cache filename in the
selected Simulator app container. It rejects symlinks, path escapes, excess bytes,
unknown fields, duplicate JSON keys, borrowed source/run/test identities and
impossible clock/sequence metadata. The original admitted JSONL bytes are retained
unchanged alongside their digests.

All eleven acquisition dependencies are checked against the actual claimed Git
objects using `git --no-replace-objects`. A real temporary replacement-object
control demonstrated the earlier source-binding defect before the fix, then
verified both rejection of replacement bytes and continued ownership by the
original objects after the fix. The control's temporary repository explicitly
uses LF checkout; its first Windows attempt exposed a separate test-fixture
line-ending cleanup error, which is retained.

A direct `bounded_log` canary verifies that both private issue lines and observation
session lines are absent from the ordinary public iOS evidence. The new public
observation artifact contains only validated fixed metadata and admitted event
files. The private capture directory and raw xcodebuild log are excluded from
artifact uploads.

## Actual local verification

| Check | Actual result |
|---|---|
| Swift 6.3.3 compiler in Swift 5 mode, observer controls | 68 assertions passed |
| New Python capture/source/privacy controls on clean final source | 16 tests passed, no skips |
| Existing iOS evidence collector controls on clean final source | 43 tests passed, no skips |
| Real clean-checkout source-object binding | Passed on final implementation SHA |
| YAML parsing and all 15 extracted workflow Bash blocks | Passed |
| Modified Swift syntax parsing | Passed; this is not UIKit/XCTest SDK type checking |
| Source/body/deadline/paste/manifest byte guards | Passed |

The observer draft's expiry timestamp flaw was independently reproduced: an event
captured at 200 seconds was processed after the one-second recording lifetime at
202 seconds, but the old row reused 200. The corrected observer records the actual
202-second expiry decision. The failing binary output and later passing control
are both retained. The witness A-to-B association control exercises the actual
Foundation state used by the UIKit witness; it does not claim SwiftUI execution.

## Retained evidence and next gate

The companion JSON file contains exact source hashes, commands, raw output digests,
failed controls and successful replay receipts. Original host evidence is retained
under the owned `startup-lifecycle-observation-20261120/` evidence directory.
No Swift source snapshot is added to an executable Go package.

The next gate is one source-bound run of the existing iOS workflow with
`runner_label=macos-15-intel`, `documentation_component_only=false` and
`startup_observation_only=true`. Actual native SDK compilation, app lifecycle
observations and test outcomes must be inspected before drawing a cause or result.
The original 44/12 failure evidence remains unchanged.
