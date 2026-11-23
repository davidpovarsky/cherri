# Bounded production migration-switch layout repair

The original diagnostic packet and its failure remain unchanged. The new one-file candidate keeps the migration opt-in bound to the same local Boolean state, default OFF. A visible descriptive label and row padding now surround an intrinsic-size native switch whose original label remains available to accessibility through SwiftUI. The original docsMigrationToggle identifier stays on that switch.

The retained evidence supports a failure to observe the state action after a normal element tap. The source contains no separate migration-state reset or competing gesture in this path. It does not retain the control frame or chosen touch point, so the hit-target explanation remains a hypothesis. The repair does not claim a UIKit/SwiftUI platform defect or exclude a transient state reset.

Apple documents that labelsHidden separates a Toggle's visual label from its control while the supplied label still serves accessibility. XCTest tap uses a system-computed hittable point. Those API facts motivate the narrow layout change; they do not establish this run's touch coordinates.

No protected test, assertion, observer, wait, source fixture, query predicate, default preference or acceptance gate changed. No parallel tap gesture or manual accessibility-value override was added. Native compilation and the original D03 assertion remain pending the final combined-source full iOS workflow; no new test or CI run was launched for this commit.

Official sources:
- https://developer.apple.com/documentation/swiftui/view/labelshidden()
- https://developer.apple.com/documentation/xcuiautomation/xcuielement/tap()
