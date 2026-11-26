# R23 review of protected iOS evolution

The owner explicitly requested correction of the historical R23 contradiction
on 2026-11-25 while retaining all tests and the purpose of the protection.

## Exact scope

- Historical baseline: 76d722502e20db977e84a095444d0172442655b8.
- Reviewed source: 57dd62d77f5cc1470d9fecd677efc7ba0ff6f2ae.
- Protected paths remain ios/CherriUITests/CherriEditorUITests.swift and
  ios/CherriApp/Views/CodeEditorView.swift.
- The editor UI suite retains all nine original XCTest methods and adds four.
  The second protected path is absent at both revisions; creating it without
  another explicit review is still rejected.

The companion JSON records Git blob IDs, SHA-256 digests and independently
checked method inventories. The gate verifies these against Git objects and
requires the reviewed commit to be an ancestor of the current source. It also
requires the exact reviewed protected blobs in HEAD and an unchanged working
tree. Editing, staging, committing, deleting or replacing a protected file after
the review fails. Editing the review without committing it also fails.

## Review of changed test behavior

The initial S6 review covered 584 added and 35 removed lines. No test method was
removed. The removed lines come from shared setup/reset helpers, the E01
completion test, and E05 reset handling.

- Startup now requires actual foreground, unique controls, a completed wait and
  the existing total deadline. Failure records diagnostics; it does not grant a
  successful readiness result.
- Reset requires the clear control and an observed empty editor. Synthetic
  keyboard entry requires exact UTF-16 content.
- E01 removes the fallback that could select an arbitrary palette cell and
  manually append missing syntax. It requires the exact completion, checks
  rejection of an unfilled placeholder, then explicitly fills it and checks
  compilation and cleared diagnostics.
- E05 retains the responsiveness checks and adds exact rapid-typing content
  verification.
- New tests cover documentation search, exact palette selection, generated
  delimiter overtyping, and repeated source/reset fidelity.

This review accepts the intentional source evolution only. It does not establish
that any XCTest passed, and it does not close Apple runtime or physical-device
acceptance.

## Historical result and future protection

The original baseline comparison remains available and returns a historical
failure for the reviewed changes. The new gate reports that historical result
separately while evaluating whether the current source exactly preserves the
reviewed evolution. No old CI result or evidence is relabeled.

Future intentional changes require another review pinned to a real commit and
exact blobs. The baseline remains unchanged; a proposed review that removes an
original test method or protected file is rejected. Real-Git mutation tests
exercise both the unchanged historical mode and the reviewed mode.

## Review of the 2026-11-25 UI repair

Source 0788295f8a1dacb7ba84f14a1dc5d86bc5d5d4b7 adds the reviewed UI repair
on compiler source 627c3e7eb4a54a0364b5a5563db89291fa953b97. The protected
editor suite retains all 13 existing XCTest methods, including every one of the
nine historical methods. Its 17-line diff changes only D03 and E07; startup,
reset helpers, test selection and skip behavior remain unchanged.

- D03 retains the exact first-result predicate and five-second deadline. Its
  assertion now records three boolean observations when the original wait fails.
- E07 retains all original visual-edit, rejection, comment-preservation and
  reopen assertions. It additionally waits for exact UTF-16 source equality at
  each of the three Code-view transitions, using the existing five-second
  predicate helper. The expected synthetic-source receipt is updated to match
  the authored parameter edit; the observed editor still must equal that source.
- The other protected path remains absent. Its creation would still require
  explicit review.

The original encrypted and decrypted XCTest evidence and the corresponding
application/RP40 changes are documented in
docs/apple-runtime/p10r15/ios-ui-original-failure-repair-20261125.md.
This source review does not claim that the repaired XCTest or simulator run
passed. The combined native workflow must provide that separate evidence.

## Review of the exact D03 checkpoint emitter

Source 9329aa79121457fd023be45b58c8f3ad4315120e changes one print literal in
the protected editor suite. The collector requires the exact
[CHECKPOINT:D03_CURRENT_DOCS_SEARCH_VERIFIED] token; the previous emitter
appended descriptive text and the original K workflow correctly retained
INVALID_CHECKPOINT instead of granting D03 evidence.

Independent byte comparison against the previously reviewed source proves that
the print literal is the only protected change. All 13 XCTest methods and every
assertion, readiness wait, test boundary and failure path are byte-for-byte
unchanged. The other protected path remains absent. The original nine baseline
methods remain present, and the immutable baseline itself is unchanged.

The actual-source collector regression failed twice before the repair and its
four controls passed afterward, including rejection of trailing text and a token
outside its owned XCTest. The existing 23 documentation controls also passed.
These controls and the source review do not supply a new native D03 execution.
The original failed evidence is unchanged; a future native run must produce its
own exact checkpoint, complete test outcomes and source-bound artifacts.

Original review and R23 observations are retained on Oracle under
/mnt/development/projects/evidence/cherri-p10r15-ci-release-20261125/r23-d03-reviewed-evolution-01/.
The protected source SHA-256 is
7f79536538fbaaf1dfe4a4a6a375332d43bf8d18f5c9c83ccbd1ac90646cba26.

## Review of the 2026-11-26 E07 pane transitions

Reviewed Swift source: 0471fbfb735ede0ceb291761383ded2e7785fc6e, based on
the integrated diagnostic source f1eaa704d15d21835facb5e12b8cfe3f057274e5.
The protected test blob is 8bc5e6c2c79aeb2fd91bed691f370708fc408841 and
its SHA-256 is 04983a012593f32222e0410592ae80d72a1ad62edbecce0aa911fdcedfc5f3ec.

The original iOS run 45770108822 retains 77 passing tests and one E07 failure.
E07 passed its exact edited-source and rejected-edit preservation assertions,
then failed its final return-to-Code source assertion. Teardown recorded that
the editor was absent; it did not provide a final source value. This does not
establish source corruption or prove whether the final conditional tap occurred.

The reviewed change replaces only six silent, unscoped pane-selection blocks
and adds a helper used only by E07. It requires a unique segmented control with
one Code and one Preview button, waits for the target to be hittable, issues
one tap and observes the selected target. Readiness and selection share the
original three-second allowance. If no selector exists, the alternative
requires both editor and preview to be present and hittable. A missing control
or failed transition records a test failure; no retry, skip, input injection or
application-state override was added.

All 13 existing XCTest methods remain. All 17 original E07 assertions,
synthetic fixture bytes, comments, checkpoint and downstream five/eight-second
deadlines are preserved. Removing the exact added helper and reversing the six
declared substitutions reproduces the complete previous protected file
byte-for-byte. Thirteen source-corruption controls reject changed assertions,
fixture/deadline/checkpoint loss, ambiguous or global selection, an extra tap
and unproved navigation success. The other protected path remains absent,
and both historical baseline snapshots are unchanged.

The bounded transition observations contain only fixed E07 stage/control
metadata, booleans, counts and elapsed time. They contain no document names,
source values, screenshots or broad accessibility tree.

Original source retention receipt:
`/var/cache/cherri-e07-pane-transition-20261126/source-retention-01/result.original.json`,
SHA-256 2b2805e9c1f4e37130ceb95c3819fc3dd0696f26809cf59875d01d0d236dd75a.
Independent CI source review:
`/var/cache/cherri-p10r15-openminis-recovery-20261126/e07-r23-source-review-01/review.original.json`,
SHA-256 3d4335a13c36db99ab9fb7dcdb50680e2d912431ba22e41c8bb3fa89006bd2ad.

Before this successor review, the unchanged R23 gate correctly rejected the
committed Swift change as unreviewed. That failure is retained; its historical
baseline result remains FAIL. This review grants only exact source retention.
Swift compilation and the focused E07 native replay have not yet run; the
complete iOS workflow on the joined candidate remains independently required.

## Review of the 2026-11-26 E07 startup observation

Reviewed Swift source: 57dd62d77f5cc1470d9fecd677efc7ba0ff6f2ae.
The protected source adds only
`CherriEditorUITests/testE07VisualParameterEditingCanvasUI` to the existing
startup-observation allowlist. Removing that one line restores the previously
reviewed protected file byte-for-byte. All 13 current methods, nine immutable
baseline methods, E07 fixture bytes, assertions, checkpoints, pane transitions,
and readiness deadlines remain unchanged. Both baseline snapshots and both
protected paths remain exact; the second path remains absent.

This follows the actual startup failure in full iOS run 45780580372 on 1caf:
the original E07 startup assertion failed before any pane transition or source
reset. The existing trace had completed repeated polls without finding an
editor. This does not establish application source corruption or a functional
navigation defect. The new app-side observation uses the existing nonce,
96-event, 65,536-byte and 120-second bounds, with no additional AX query or
readiness condition.

The same source includes closed observations of already-read Sign properties
and initial WebKit field counts outside the protected test file. Their exact
scope, real Python RED/GREEN and source-preservation receipt are documented in
`docs/apple-runtime/p10r15/ui-readiness-boundary-observations-20261126.md`.
The implementation preserves every original assertion and original 5/8-second
predicate. No workflow selection, skip, retry or acceptance mapping changes.

The unchanged R23 gate rejected this committed protected-source change before
the successor review; its original receipt is
`/var/cache/cherri-e07-pane-transition-20261126/ui-readiness-r23-before-review-01/result.original.json`,
SHA-256 b39f2a1b2544ded82fe89472881a0302e8493f1959017ddf3bc14d76a1999a5a.
The historical baseline result remains FAIL. This review concerns exact source
retention only. Swift compilation and actual observation of these additions
remain requirements of the next complete integrated iOS workflow.
