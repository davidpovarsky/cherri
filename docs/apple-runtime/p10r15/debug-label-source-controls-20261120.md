# Per-entry debug labels: verified source component

Implementation: `e324045e999202e5c3411e6a852aaf3502f1ccf0`. This is an isolated component, not the frozen K candidate and not Apple runtime acceptance.

The original `debugLog` and `debugLogPrevious` now accept an optional fourth literal Text label. V2 also accepts the named form `debugLog(label: "API Response", withDates: true, showLogs: true)`. The original three positional Boolean flags keep their order. Omitting the label keeps the existing one raw append contract; an explicit empty label remains distinct from omission.

The raw buffer receives the exact original native ActionOutput token. A separate Dictionary contains only the scalar label and first/last native buffer positions. It never boxes the logged File, Dictionary, List or other object. Counts are taken from the raw append and its original source. Typed presentation snapshots the outer item/index before looking up label metadata, and adds a name only in the presentation path. Explicit text mode remains an explicit choice. Empty labels do not request an empty Set Name.

Owned clear and preview sites earlier in source are updated when the first label is encountered, because an earlier source site can execute later in another loop iteration. Only exact compiler-owned expansions are eligible. Raw lookalikes, source consumers and source effects stay intact. Release removes label helpers and rejects consumed diagnostic state; assertions remain.

## Clean-source verification

The implementation SHA was clean before and after every recorded check. Original receipts and logs are retained unchanged; the adjacent JSON binds their hashes.

| Check | Actual result | Duration |
| --- | --- | --- |
| Label controls plus existing DebugStdlib/BuildProfile | 109 pass events; 54 label events; 0 failed/skipped | 4.469 s |
| Schema, analysis, lowering and documentation packages | 114 pass events; 0 failed/skipped | 6.688 s |
| Generated complete action reference check | Exit 0; 505 native definitions and 15 macros | 0.703 s |
| Existing macro-contract ledger regeneration | Exit 0; label parameter observed, task remains OPEN | 17.203 s |

Controls compile both frontends and cover omitted/empty/Unicode labels, multiple distinct entries, the six existing flag modes, Number/Text/Map/List/Date and native File producer references, original UUID/output names, later consumers, delayed Creation Date capture, nested branches/loops, exact zero-output clears, Release behavior, exact UTF-16 token text/ranges and corrupted causal graphs. Provenance is observed while the actual lowering session still owns its manifest; the bytes-only API closes that session.

## Remaining acceptance

Native Count Items/append cardinality across content classes, File type/bytes and original-name fidelity after presentation, six-mode actual Quick Look display, signed simulator behavior and a physical device remain unverified for this component. No formal requirement is closed by these source tests. The canonical macro ledger is source-bound: the new ledger says `LABEL_PARAMETER_PRESENT_REQUIRES_NEW_ACCEPTANCE`; historical/frozen candidate ledgers are not rewritten.

Neither the original export/File oracle nor `debugExport` was changed. The independent native carrier experiment is tracked separately.
