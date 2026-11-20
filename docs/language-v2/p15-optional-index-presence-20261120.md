# Optional indexed-value presence and List evaluation order

This component repairs the actual V02 regression without changing any existing V02 source or expected diagnostic. A literal List and an in-bounds integral Number literal identify an authored slot. Presence is proved only when that slot's checked element type is definitely present. Native truthiness and native Count are not used as the presence oracle. The existing actual List construction, authored effects, native bounds guards, and indexed selection still execute exactly once.

The same baseline exposed a separate evaluation-order defect: an earlier List element referencing a mutable binding remained deferred until a later sibling had overwritten it. Native List construction now applies the existing argument-capture helper at that element's evaluation point. Raw inline native metadata retains its separate path.

## Immutable component commits

- Tests-only baseline: 8e056e38e2ccab03775f0a6841e54b427b55b29d.
- Corrected boundary characterization: 03389e7a63acde1aef8feacc3d825ea62488206a.
- Repaired production and independent native oracle: 1113c27c581e21a4bc0fe68b17558d4694aef412.

The earlier Optional native diagnostic component is the base; its signed execution is a separate task. This report does not acquire native credit from it.

## Actual retained runs

| Source | Actual result | Scope |
| --- | --- | --- |
| Clean 8e056e38 | Exit 1; 35 named passes, 61 fail events | Static-index bindings/arguments/returns, owned presence predicates, unchanged V02 narrowing, and missing early List capture reproduce. One new Optional-element negative also had a test-expectation error, explicitly corrected below. |
| Clean 03389e7a | Exit 1; 11 named passes, 1 fail | All ten precise unsupported-boundary controls pass. The new known-present Optional element still reproduces missing indexed presence. |
| Dirty 03389e7a plus retained four-file source ZIP | Exit 0; 28.484 seconds | All unchanged V02 tests and new source/service/boundary/order tests pass. This is a development run, not clean-source acceptance. |
| Dirty 03389e7a plus retained five-file source ZIP | Exit 0; 29 named passes; 16.875 seconds | Ten complete native graphs and seventeen corruption controls pass. This is a development run. |
| Clean 1113c27c | Exit 0; 934 named passes; no failures or skips; 50.485 seconds | All V02, Optional, Optional function, function Map/container, Dictionary List, numeric List, empty List, L01 and F04 tests selected by the exact retained command. |
| Clean 1113c27c | Exit 0; 93 named passes; no failures or skips; 9.704 seconds | Entire syntax, analysis, lower and service packages. |

The first baseline's initialized empty-Text Optional negative incorrectly assumed that an explicitly present immutable initialization remains unknown. Analysis already proves that value present. The follow-up changes this negative to actual None and separately requires the former source as a positive. The original failure log is retained unchanged; it is not counted as a compiler defect.

The clean source was unchanged before and after both clean commands. Original JSON event streams, stderr, process receipts, development source ZIPs and the failing native capture graph are preserved byte for byte under evidence/p15-optional-index-presence-20261120/. The manifest binds every retained artifact.

## Independent scenarios

Nine indexed sources cover ordinary Text, empty Text, false, zero, empty Dictionary, populated Dictionary, a second slot, integral decimal and exponent spellings. Each executes actual analyzer, compiler and editor-service routes for binding, user-function argument and user-function return. Exact native presence predicates use an owned Text flag 1, separately from the selected data.

Ten complete native graphs independently author the dispatcher, typed argument/result carriers, native List rows, source effects, bounds checks, selected result edges and final consumer. UUIDs are alpha names only. Seventeen causal mutations alter cardinality, order, early capture, row type, selected value, bounds, native List origin, presence flag, carrier or value edge; unchanged clones pass first and each mutation is rejected. The early-read graph requires capture before the later authored assignment, with no duplicate evaluation.

Negative, out-of-range, fractional, dynamic-call, dynamic-binding and aliased-target indices do not acquire this proof. A Map lookup, opaque content, absent Optional element and Optional function result element also remain precise E_TARGET_VALUE_SHAPE rejections with zero emitted bytes. A known-present Optional element is accepted only because its checked slot value is present.

## Boundaries

This is compiler and native-structure evidence. Apple execution of these specific indexed values remains NOT_RUN. It does not prove native nested-List transport, arbitrary dynamic indexing, general Optional control joins, or full L01/T05/FN05/RP20 completion. No requirement ID, immutable contract, Legacy fixture, golden native output or shared codec descriptor was changed.

The separate Linux CI failures concerning unconsumed Optional presence metadata in Nil/Legacy paths and a source-owned require Dictionary lookup remain open and are the next component. The 934 clean passes are the exact selected scope, not a full repository or final179 pass.

## Clean raw hashes

- Root: a87829e76958403932eb8deb5b297f452ab69755d2afd96277c5b7d77c2a679f.
- Packages: b0070eea6ea4a95222caffd485ab28cf92ae59a890b6764a2413b5fcebabce7d.
