# Four native causal splits: reviewed source and local graph proof

These are additional strict synthetic fixtures. They preserve the original collection, F01, File, and export sources and expectations. No production emitter, serializer, signing path, HTTP registry, or native error-catching behavior changes in this stage. Native execution is still required for every new fixture.

The fixtures use the existing signed Simulator QA harness, exact source hashes, a fresh invocation nonce, required before/after inline events and collector begin/end events, and an exact callback result. They do not use the Release trace exemption. Their selectors explicitly bind reviewed source files and disable the HTTP probe. No arbitrary source or sample suffix is accepted. The normal synthetic-text publication boundary remains in force.

## Collection emptiness

Select qa-collect-empty-split. Its exact expected callback summary is:

    CHERRI_QA_COLLECT_EMPTY_SPLIT:direct=0:repeat=0:cleared=0:emptyText=1

The fixture measures four independent paths:

1. Native List with zero rows, consumed directly by Count.
2. That same empty List passed through the existing V2 for/yield lowering, then Repeat Results consumed by Count. This path appears first, in the same List / Repeat / Count order as the unresolved original.
3. The existing Legacy empty-variable mechanism: Nothing followed by Set Variable with no explicit input, then Repeat Each, item store/read, and Count of that Repeat result. It is represented using existing native.action/native.ref syntax with explicit synthetic group/result IDs. No new empty-list emitter or alias is introduced.
4. A native List containing exactly one empty Text row, consumed by Count as a positive control.

The original exact-source comparison between 38c7098 and 71a3e2d found identical first 17 normalized actions despite different observed empty counts. Consequently this fixture tests native boundaries; it does not assume a proven compiler regression. The complete previous audit remains in collection-r5-71a3e2d-native-audit.

The 29-action graph has four separate Count consumers and four separate summary references. The graph validator rejects ten causal corruptions: missing/changed empty rows, redirected direct/collected/cleared counts, replaced Nothing, a clear that gains an input, a changed Repeat group, removed singleton Text, and a missing producer UUID. All real output references have producers. An early draft that bound raw exact plist parameters without an explicit output UUID was rejected locally before dispatch; the final fixture uses canonical producers and explicit raw Repeat-result identity.

## F01 If-result binding

Select qa-f01-ifresult-let or qa-f01-ifresult-var. Both require:

    CHERRI_QA_F01_IFRESULT:P15_F01_IFRESULT_OK

Their semantic bodies differ by exactly one token: let controlValue versus var controlValue. Both derive truth from the same native Number and comparison, evaluate the same typed If expression yielding Bool, consume its result in the same negated condition, and report the same verdict. The only other source difference is their trace fixture identity.

The let graph contains a named End If result consumed directly by ActionOutput. The var graph contains an unnamed End If result consumed by Set Variable, followed by the named-variable path. The graphs contain 32 and 33 actions respectively, with no Math or external Run Shortcut. The focused oracle verifies this precise distinction and complete UUID references.

The successful predicate pilot and failed logic pilot support this as a useful differential; they do not establish its cause. The failed logic's exact source was independently recompiled with 867 and 71 and its normalized 65-action graphs were identical. No native If-result repair is made before the new results exist.

## Literal bytes to named File

Select qa-export-file-bytes. It requires:

    CHERRI_QA_EXPORT_FILE_BYTES:NAME=ok:TEXT=ok

The input File is produced by decoding the fixed Base64 string eyJvayI6dHJ1ZX0=, the UTF-8 bytes of the synthetic text {"ok":true}. Existing Set Name assigns cherri-export-control.json. Existing Get Name and Get Text each consume that same named File, and independent NAME and TEXT flags must both pass.

This 32-action control excludes Get File of Type, debugExport, and JSON parsing. Its graph validator checks the decoded-File to Set Name to both readback UUID edges. It isolates those existing primitives from the earlier Text-to-File control that returned native ERROR. No successful File conversion or export is claimed until signed execution.

## Executed local verification

All four final sources compiled through both actual frozen binaries: 867261a (binary SHA-256 be7d917420789b6bbe4e9ba985eb77c751c246217f3a3f33d1aeb2c73b45061f) and exact-source 71a3e2d (binary SHA-256 2b2a0e273ab340bbe13e687dd1af3eced828b57e27ada1793c5a8fc46121ba8f). These are eight real CLI compilations using --skip-sign and --build-profile=debug. Every graph is identical between the two binaries after normalizing only UUID/group identities; every graph has zero duplicate producer IDs and zero dangling output references. Fresh local plist hashes are recorded as local evidence, not substituted for future signed-run hashes.

| Fixture | Actions | Source SHA-256 |
| --- | --- | --- |
| qa-collect-empty-split-v2 | 29 | 15d1e0ab8bae11e87cc91d81ea1a9a3cc0466429956ad7bff9b6b18570854a3a |
| qa-export-file-bytes-v2 | 32 | 4f095910b9af28d6016cd23ba06ad845f884e17941303edde4d1497bc5a144b2 |
| qa-f01-ifresult-let-v2 | 32 | ca087dae7c6728647763a2b22ccc405b335ce8c3a5cb8d444b9eb6379fbd6d04 |
| qa-f01-ifresult-var-v2 | 33 | e0f49a9d2138da7f4ac49cbd1dac97d353e340218385d185b374421fb950702a |

The three focused Go cases passed (0.878 seconds), including the ten collection graph corruptions. All 107 Simulator QA Python tests passed (103.689 seconds), including positive callback cases and rejection of each independently wrong numeric/ok flag, changed result, stale nonce, wrong identity, duplicate callback, ERROR/CANCELLED, missing trace, and invalid Release exemption. Shell syntax validation passed. The existing signed workflow now includes TestP15RuntimeDiagnostic in its Go preflight gate.

The complete local validation receipt is runtime-causal-split-fixtures.json. Simulator runtime, Inspector one-to-one correspondence, and physical-device coverage remain separate gates. Native outcomes will be recorded against the dispatched immutable source; no current local result closes those gates.
