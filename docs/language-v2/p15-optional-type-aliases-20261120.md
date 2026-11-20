# Optional type aliases — source-bound compiler proof

## Result and scope

Clean source **15386a38ee97e67b9167b766de2a957dab9ecf97** passed **927 named root tests (115 top-level)** and **75 named package tests**, with no named failures or skips. The modules package was compiled and reported no test files; it is not counted as executed module tests. Public module scenarios run in the root selection.

The analyzer now resolves type aliases, including forward chains, qualified imported aliases and aliases on imported function declarations. It records semantic results by exact AST annotation identity, retaining the defining module's meaning. Lowering consumes those analyzed results, so a caller's same-named alias cannot change a callee's parameter or return transport. The existing tagged Optional representation is selected from semantic optionality. Its wire fields and codec descriptor remain unchanged in this component.

This proof covers explicit None, empty Text, false, zero, empty List and Map literals, supplied versus omitted defaults, default effects, named arguments, Optional aliases and an Optional union, local and imported declarations, unused cycles/duplicates, wrong argument types, exact caller diagnostics, and analyzer reuse. Twelve corruptions of actual alias-produced graphs are rejected after unchanged cloned graphs pass.

General dynamic Optional forwarding, value-expression joins, full structured-container transport and Apple value-class behavior remain separate obligations. No immutable requirement is marked complete here.

## Causal baselines and retained intermediate results

The tests-only source 55870fae598c993e3aa0a18b6fdd17ef69ba43db exposed incorrect alias native graphs and unresolved argument types. It passed one leaf and failed 35 leaves. Three declaration fixtures also contained an unrelated getVariable literal-input rejection; their first results are retained without treating them as native-output leakage.

The tests-only correction 0b2657205b0925f086a4b1cdcddb5f7fad1cad5f replaces only that unrelated literal consumer with Text. Its five-case clean replay failed all five intended assertions: unused self/mutual cycles and duplicate aliases emitted 2,189 bytes, wrong Number input to an aliased Text? parameter emitted 23,610 bytes, and a present-only Text alias reported Unknown instead of the resolved type. Raw SHA-256: 71922221282032ea7ce77244a109a84bcc1efdb288318889cd69c56a450af7e4.

The first dirty implementation replay passed 40 named tests. A subsequent dirty replay passed 59 and failed one newly added test because it reused an import-loader privacy helper for a semantic error in the caller's own path. The final test instead requires the exact caller argument span, defining-module type, error code and zero native bytes. The production implementation did not change for that test correction. Both dirty attempts remain in the evidence manifest and are not represented as clean runs.

## Final clean replay

Source: 15386a38ee97e67b9167b766de2a957dab9ecf97. Git was clean before and after both commands. The exact commands, process receipts and UTC intervals are preserved.

| Run | Observed result | Elapsed | Original raw SHA-256 |
| --- | --- | --- | --- |
| Affected root regressions, new aliases, original inferred-List guards | 927 named PASS; 115 top-level; exit 0; no failure/skip | 53.562 s | 21db0d72beeea849f384b9d15675a9d88c2291142c720be542695bb6a6b906da |
| Complete types, analysis, lower and service packages; modules build | 75 named PASS; exit 0; no named failure/skip; modules has no tests | 9.719 s | 8dcc277b8a962496df43b0d40eb782d2dfa95319bc4ab3fe4c76db7c610964e4 |

The root selection includes earlier Map key/value capture, native opaque/null boundaries, None clearing, immutable and mutable Optional flags, default evaluation, function native graphs and round trips, as well as all 60 new alias events and the complete 16-event inferred-List test. It does not rerun the full 179-requirement acceptance program.

## Provenance

The evidence directory contains unchanged raw outputs, stderr, process and summary receipts, both tests-only baseline source blobs, the exact final source blobs and the production/test diff. Each retained byte sequence is indexed by SHA-256, and staged Git blobs were compared with those hashes before this documentation commit. The proof commit does not relabel its own SHA as the tested implementation.

No signed Shortcut was generated or run for this component. Static Dictionary/reference structure is not a substitute for observed Apple behavior. Existing runtime fixture sources, expected outputs, immutable 179 rows and external acceptance evidence are unchanged.
