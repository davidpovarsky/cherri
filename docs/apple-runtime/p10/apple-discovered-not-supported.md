# Apple Runtime Discovered vs Cherri Supported — Gap & Denominator Report

- **Source Commit**: `55d7800dc01cd52ca3d6a5a17b3159ffe5a28089`
- **Total Raw Discovered Apple Runtime Records**: `2396`
- **Eligible User-Facing Apple Actions**: `1527` (`487` builtin + `1040` first-party AppIntents)
- **Excluded Records**: `869` (`0` internal/config-only + `0` third-party conditional + `0` non-runnable definitions + `0` deprecated legacy)
- **Covered Unique Apple Identifiers (`COVERED_VERIFIED`)**: `390` (`25.54%` of `1,527` eligible user-facing Apple actions)
- **Uncovered Eligible Apple Identifiers**: `1137` (`0` `MISSING_HIGH_PRIORITY` + `0` `MISSING_LOW_PRIORITY`)

## Why `505` Cherri Catalog Entries != `1,527` Eligible Apple Actions

1. **Cherri Split/Variant Actions (`505` entries -> `390` unique Apple identifiers)**: Cherri exposes multiple ergonomic callables that compile to the same underlying Apple `WFWorkflowActionIdentifier` with different parameter presets (for example, `setWiFi` and `toggleWiFi` both emit `is.workflow.actions.wifi.set`).
2. **Uncovered Eligible Apple Actions (`1,137` identifiers)**: Out of `1,527` eligible user-facing actions discovered in the macOS 27 / iOS 27 runtime corpus, `390` are natively covered by Cherri and `1,137` remain uncovered (`100` high-priority candidates + `1,037` low-priority/specialized AppIntents). Any uncovered Apple action can still be authored losslessly via `rawAction("identifier", { ... })`.

## All `100` `MISSING_HIGH_PRIORITY` Apple Actions

| # | Apple Identifier | Name | Source Kind | OS Availability |
| :---: | :--- | :--- | :--- | :--- |
