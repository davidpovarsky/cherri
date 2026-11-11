# P10R9 Core Engine & Catalog Parity Matrix

| Surface | Backing Implementation | Total Entries | Callable Actions | Compiler Constructs | Status |
| :--- | :--- | ---: | ---: | ---: | :--- |
| Canonical Action Map | `EnsureCanonicalStandardActions()` (`actions/*.cherri` + `actions_std.go`) | 497 | 497 | — | **PASS (100%)** |
| CLI Action Catalog | `buildActionCatalog()` (`--actions-json`) | 505 | 497 | 8 | **PASS (100%)** |
| V2 Schema Registry | `schema.DefaultRegistry()` (`generated_registry.go`) | 505 | 497 | 8 | **PASS (100%)** |
| iOS Mobile Bridge Catalog | `currentMobileActionCatalog()` (`CherriActionCatalog`) | 505 | 497 | 8 | **PASS (100%)** |
| Reverse Action Registry | `DefaultReverseActionRegistry()` (`reverse_action_registry.go`) | 496 | 496 (`381` unique Apple IDs: `303` single, `78` multi-variant) | — | **PASS (100%)** |

## Category Distribution

| Category | Action Count |
| :--- | ---: |
| `basic` | 442 |
| `builtin` | 2 |
| `calendar` | 10 |
| `contacts` | 4 |
| `controlflow` | 7 |
| `device` | 9 |
| `documents` | 6 |
| `intelligence` | 1 |
| `mac` | 1 |
| `math` | 3 |
| `media` | 3 |
| `pdf` | 1 |
| `settings` | 5 |
| `shortcuts` | 4 |
| `text` | 4 |
| `web` | 3 |
