# R23 review of protected iOS evolution

The owner explicitly requested correction of the historical R23 contradiction
on 2026-11-25 while retaining all tests and the purpose of the protection.

## Exact scope

- Historical baseline: 76d722502e20db977e84a095444d0172442655b8.
- Reviewed source: 9329aa79121457fd023be45b58c8f3ad4315120e.
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
