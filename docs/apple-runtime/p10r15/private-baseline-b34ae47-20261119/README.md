# Private regression baseline at b34ae47

The five required private corpus tests executed separately and serially against clean source `b34ae47ba79d15c08e8a8bdec9a761a7a4b722bb`. Three passed; the zero-Stage3 corpus gate and fidelity/source-size audit failed. All five were discovered, none skipped, and every before/after source and input check remained unchanged.

The measured natural count was 23,349 of 27,652 actions. The existing floor of 23,340 passed, but four Stage-3 demotions correctly blocked acceptance. The P13 failure is its unchanged zero-demotion assertion. A temporary host-only diagnostic observer later isolated two `E_TARGET_VALUE_SHAPE` events in imported native repeat regions, accounting for all four demoted control actions.

The cause is an importer representation problem: rendering an existing native Repeat Results attachment as an authored value loop introduces a nested List cardinality promise. The F04 authoring rejection and the observed native flattening remain valid. A corrective importer representation and full verification of a later combined source are still required.

This archive contains aggregate counts, source identity, timings, exact test names, immutable input-set hashes, and raw-log hashes. It contains no private specimen identities, values, generated source, or raw corpus logs. The 38 originals and all 76 input files remained unchanged. `CHERRI_P10R13_WRITE_REPORTS=0` was used throughout.
