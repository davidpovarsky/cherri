# R23 review of protected iOS evolution

The owner explicitly requested correction of the historical R23 contradiction
on 2026-11-25 while retaining all tests and the purpose of the protection.

## Exact scope

- Historical baseline: 76d722502e20db977e84a095444d0172442655b8.
- Reviewed source: 6dca0432e5887221ff0e4a27c70ed16025b24a8f.
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

The historical diff contains 584 added and 35 removed lines. No test method was
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
