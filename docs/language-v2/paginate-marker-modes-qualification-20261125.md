# Pagination marker modes: bounded source qualification, 25 November 2026

## Implemented component

Implementation dd3b5023bf020835087ae7f27fccee8236e5dc2d extends the existing shared Legacy/V2 paginator with optional literal mode and queryKey. Modes are url (the unchanged default), cursor, page, and offset. The first request retains the authored URL; query modes replace only the configured query value and preserve surrounding raw query bytes. Duplicate decoded key aliases and malformed source markers fail before emission.

The runtime graph checks singleton content category. Cursor mode uses native URL-component encoding plus a decoded digest comparison and delimiter guard. Numeric modes require exact integer/range equality and canonical decimal text before inserting the next marker. Initial and subsequent marker identities participate in cycle detection. Existing page/item budgets, ordered native Dictionary items, header references and fixed privacy-preserving diagnostics remain.

## Original execution and source identity

The immutable RED source was c1d893a2e9366f229e147c7513a7212e06f4f7c6. The original 63 selected Mac Go events yielded 4 expected default-URL passes and 59 semantic failures. Independent intake:
- /mnt/development/projects/evidence/cherri-p10r15-paginate-completion-20261125/modes-mac-red-causal-intake-01/result.original.json
- SHA256 ad1cce351cb5fead30eddc94204c5e0f2381a24e305c86162ba0400cfdec9ff3

Actual production qualification on dd3b5023bf020835087ae7f27fccee8236e5dc2d produced 158 named passes and 7 failed events (six leaves plus their parent). The six sources correctly emit an authored authorization Text before the pagination List initializer. A historical test helper assumed that initializer always occupied action index zero. Its FAILED original receipt remains unchanged:
- /mnt/development/projects/evidence/cherri-p10r15-apple-completion-20261125/mac-paginate-green-01/original-intake-01/originals/paginate-green-01/result.original.json
- SHA256 b4490edbc79969a327e7fe16760ab050be89e2a7ba2b0633d6b1de29891e00b6

Test-only checkpoint 13011a5cbe9b33973d35af188e01c0cd68b9dd67 binds the final result read to its unique native declaration and the immediately preceding owned empty List. The full graph, authored prefix, unique UUIDs, append provenance and original detached-result protections remain. All four production files and the original RED test file are byte-identical. Twenty-four causal corruptions cover both dialects' header/initializer/store/read/append edges.

The focused replay passed all 40 named events with no failures, skips or aborts, preserving all eight expected native graph/source products. It includes the six unchanged source fixtures, the new causal controls and five original detached-result faults. Original receipt:
- /mnt/development/projects/evidence/cherri-p10r15-apple-completion-20261125/mac-paginate-header-replay-01/original-intake-01/originals/paginate-header-replay-01/result.original.json
- SHA256 1fa0377df0a2da9c2030e6fd36b701100ec590cd5fc6b4c8ef147160df972778

Independent intake rehashed both original streams, all 62+8 products, both unchanged physical source manifests and 7,437 source pins against Git blobs, and compared the six repeated authored sources exactly:
- /mnt/development/projects/evidence/cherri-p10r15-paginate-completion-20261125/modes-joined-qualification-intake-01/result.original.json
- SHA256 fa309c29eeee8964fb0cad18e0ecec28e379f58cda62602c86e8094232deed87

These are **158 prior passes plus 40 focused passes from two runs**, covering 198 distinct named identities. They are not one combined GREEN165 run. Both actual runs used Darwin arm64 / Go1.27.4; the earlier Oracle attempt was NOT_RUN due to its unchanged capacity floor. Final Linux CI remains required.

## Remaining original scope

The new CLI/model producer is committed but has not yet executed. Its modeled URL encoding, numeric conversion and HTTP-body assumptions cannot establish native Apple behavior. Typed page callback blocks, returned item selectors, Link headers, HTTP status handling, transport catch/retry, cancellation, signed native simulator/device execution, final documentation/Skill generation and release acceptance remain separate work. No task-wide completion or native credit is granted here.
