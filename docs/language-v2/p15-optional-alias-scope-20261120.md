# Optional aliases that share primitive spellings

A read-only peer review identified that primitive names were checked before qualified aliases. Clean tests-only source 45ad8982df52c7d7cab8b062649c24c2f01c847d then reproduced all four failures: local Text/Number aliases and qualified exports with conflicting caller aliases. Each source is parser-valid and expects the same complete Optional native graph as the corresponding explicit annotation. The baseline incorrectly selected the primitive type and rejected the otherwise valid argument.

Baseline: 4 leaf failures, no passes/skips, exit 1, 16.531 seconds. Original raw SHA-256: 32a3ddd16e4fc6c0d5dab496f3bb444ed7014c7fcfffbc2d3b7c14fa349464bb.

Production source **882a44ae2d827c3d79f658b8b56603b4cf1cc00b** changes five lines and removes three in Analyzer.resolveType. A declaration in the exact namespace takes precedence. The builtin fallback applies only to an unqualified name with no alias. Imported declarations continue resolving in their own analyzer; the caller's conflicting alias is not substituted.

The dirty focused replay passed all 5 named events. It remains explicitly dirty and is not the final proof. A draft text-loading step also altered an unrelated comment encoding; the committed change was rebuilt from the exact UTF-8 Git source and contains only the intended eight-line precedence delta. No test expectation or native oracle was weakened.

## Clean replay

| Run | Actual result | Elapsed | Original raw SHA-256 |
| --- | --- | --- | --- |
| Prior full affected root selection plus all alias controls | 932 named PASS; 116 top-level; exit 0; no fail/skip | 32.750 s | 8d87bdd1a8ef59be4ee7bd443deb1fa20aa21e793cef23c6fc8a722bacd2d488 |
| Full types/analysis/lower/service tests and modules build | 75 named PASS; exit 0; no named fail/skip; modules has no tests | 10.125 s | 491c408de138adeeafeddbbb3edb585e33f03b1bd8e31793c66ff472d90de684 |

Both replays used the exact source above with clean Git state before and after. The evidence manifest retains raw streams, stderr, process/result receipts, source blobs and the exact patch. Staged Git bytes were compared with their SHA-256 values.

This is an alias-resolution compiler component. It makes no Apple runtime claim and does not complete general Optional forwarding, joins, nested-container transport, or any immutable 179 requirement. It does not change the private function wire or codec descriptor.
