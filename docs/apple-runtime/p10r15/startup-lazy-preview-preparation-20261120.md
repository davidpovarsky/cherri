# Deferred empty-preview component preparation

Source b61a9cce83bdbb366a5b11fb561b3695d6840d3e changes four Swift/Python paths from the preserved failed fe5 startup component. The regular-width workspace now constructs WebKit only when compiled preview data exists. Empty live source clears stale preview/diagnostics and loads the action catalog without compiling an empty document. Explicit Build and nonempty live compilation remain available.

Three added native SwiftUI/UIKit controls require the actual editor to mount, verify a blank workspace has no WebKit preview, require nonempty source to compile into WebKit and then remove it on clearing, and verify disabled live preview stays uncompiled. They are diagnostic tests, not formal requirement producers. They have not yet run under an Apple SDK or Simulator. A positive view-control test can warm process/framework state; this component does not claim a controlled cold-start performance comparison.

The collector retains only fixed failure categories from immediate XCTest prefixes. Unknown messages remain UNCLASSIFIED; error content, private paths and accessibility hierarchy are not copied. Categories never decide acceptance. The existing 28-second editor policy, final readiness assertion, real three-second paste assertion, timing observations and full workflow selection are byte-identical to fe5.

Clean-source Windows validation passed all 43 collector controls and Swift 6.3.3 syntax parsing. Syntax parsing does not type-check UIKit, SwiftUI or XCTest APIs. The exact raw receipts and unchanged-file hashes are in the companion JSON. An initial partial Linux mirror lacked the workflow file; the complete exact-source replay subsequently passed that control as well.

The production Go/actions source is identical to K8e66254d56ea3e4ac70f6cda6ef9d31217e0937c. Its existing compiler/docs tuple is retained. Actual native replay remains required, and no prior run or local check closes UI requirements.
