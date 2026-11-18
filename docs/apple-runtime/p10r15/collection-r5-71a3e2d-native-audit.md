# Collection R5: numeric repair observed, empty cardinality unresolved

[Counts run 44326198585](https://github.com/davidpovarsky/cherri/actions/runs/44326198585) executed the signed V2 target from exact compiler commit 71a3e2d67751bccd762b9627b148c28ece60621d. Its fresh, source-bound native SUCCESS callback returned:

CHERRI_QA_COLLECT_COUNTS:empty=1:single=1:one:many=100:0:99

The required result has empty=0. The ordinary runtime and trace gates correctly remain **FAIL_RUNTIME**. The callback contains two inline and two collector events, but that does not override the semantic failure. In the output assertions, compiledShortcutRuntimeExecuted is part of the verified-contract result and remains false; the actual correlated native callback and trace still establish that this attempted target produced the wrong output.

## Observed change from the previous native attempt

| Field | Previous run 44253547965, compiler 38c7098 | R5 run 44326198585, compiler 71a3e2d |
| --- | --- | --- |
| Empty result count | 0 | 1 |
| Singleton count/value | 1 / one | 1 / one |
| Many result count | 100 | 100 |
| First/last numeric values | Both empty | 0 / 99 |
| Emitted action count | 34 | 134 |
| Ordinary runtime result | FAIL_RUNTIME | FAIL_RUNTIME |

The repair of numeric rows has the expected observed endpoints. Empty cardinality remains a separate unresolved failure. Both source files have the same content hash, and both public bundles were audited independently: **20/20 file hashes and byte counts per run**, with exactly the declared files plus the publication manifest and no quarantined files present.

## Exact compiler graph comparison

Both immutable compiler snapshots were rebuilt from Git archives in an isolated ignored artifact directory. The actual CLI compiled the unchanged fixture. Source archive, executable, source and plist hashes are recorded in the companion JSON; these local plists have fresh native UUIDs and are not presented as the exact bytes signed by CI.

The first **17 native actions are identical after UUID normalization**. That includes the empty List with WFItems=[], its Repeat Each input, body Set/Get Variable, End Repeat, Count, and the singleton path. The final summary's emptyCount attachment points to the same Count producer at index 10 in both builds. Neither graph has duplicate producer UUIDs or dangling output references.

There is therefore no demonstrated change to the empty prefix that explains the new empty=1 observation. The later numeric row producers do not justify a speculative rewrite of the earlier empty path. The exact 71a3e2d Legacy compiler instead represents an empty collection using Nothing followed by Set Variable with no WFInput, then iterates that named variable. That difference supplies an existing native mechanism for a causal diagnostic; it is not proof of the current failure's cause.

## Environment and second R5 target

The recorded environments match: macOS 27.0.1 build 26A434; Xcode 27.1 build 27A9275; arm64 iPad mini (A17 Pro), iOS Simulator 27.0 build 24A434. Matching versions do not establish identical simulator or editor state.

[Legacy Dictionary run 44326208135](https://github.com/davidpovarsky/cherri/actions/runs/44326208135) was cancelled with runner ID 0, zero steps and no published artifact. GitHub's check annotation reports failure to acquire a hosted runner after multiple attempts. Its native outcome is **NOT_RUN**; it is not a Dictionary semantic failure.

The next bounded split keeps the original acceptance source and expected empty=0 unchanged and compares direct native List[], collected results, the existing cleared-variable mechanism and a singleton empty-Text list. Expected independent counts are 0/0/0/1. No production empty-list encoding was changed by this audit.
