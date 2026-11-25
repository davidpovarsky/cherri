# Tagged native input snapshots — component qualification, 2026-11-25

## Scope

The observer now follows native variable snapshots at their actual read position. An omitted-input Set Variable captures the current native producer; Get Variable observes that captured value rather than a later write. Explicit inputs, appends, branch dominance, future writes, cycles, ordinary data, and private tagged List/Map carriers retain distinct checks. This is a native-structure guard; it does not authorize exporting a private carrier or establish Apple runtime behavior.

Production is frozen at `d19e4ad2709d3c694f33b72d1cb6aa637fbc88f8`. The corrected test source is `8eca11125862dad5955a7c32dc658b28adbef145`. The production file `tagged_native_input_boundary.go` has SHA-256 `796400bd26e39a6a146bb0875bbf2b44ef483925cec0ff3d9355fe6d13fba65b`.

## Actual observations

| Observation | Source | Result |
| --- | --- | --- |
| Causal snapshot RED | `0ef8176c5cd127eb685d32d55d8a6c00fcf9936f` | 15 failing leaves, two failing parents, seven healthy passing events |
| First production qualification | `92e88d14ea68f3186ac328fb951bf8afa405636c` | 170 passing events; two open-control input leaves and their parent failed |
| Wrapper production qualification | `d19e4ad2709d3c694f33b72d1cb6aa637fbc88f8` | All 149 original guards and 24 snapshot events passed; four newly authored malformed-map expectations and their parent failed |
| Exact wrapper replay on Darwin/arm64, Go 1.27.4 | `8eca11125862dad5955a7c32dc658b28adbef145` | All 15 expected events passed, with zero failures, skips, timeouts, or source drift |

The malformed-map failure was an oracle error: ordinary raw emission correctly represented those authored maps as literal Dictionary data. The corrected tests preserve all ten original case IDs and expected outcomes while injecting descriptor cases through exact native emission. Four additional positive controls retain the original ordinary-map inputs and require their actual Dictionary representation. Exact graph immutability remains checked. No production change was made for this correction.

The independent intake joins the 173 unchanged passing Linux events with the 15 Darwin wrapper events: **188 distinct passing tests across two executions**. This is not a single full green run. The earlier complete run remains a failure in its original receipt. The final integrated cohort must execute again on the integrated candidate.

The Darwin replay retained the original 90-second Go timeout and 120-second outer bound. The actual child elapsed 83.296267 seconds; the owner elapsed 89.257742 seconds. Its complete source snapshot contained 7,430 tracked files. The independent reader verified all 7,430 immutable Git blobs (261,186,329 bytes), the 889 earlier source pins, all fourteen acquired original files, and the exact selection of passing events.

## Original evidence

Evidence root: `/mnt/development/projects/evidence/cherri-p10r15-ci-release-20261125`.

| Evidence | SHA-256 |
| --- | --- |
| `tagged-backend-snapshot-red-01/receipt.original.json` | `000e74dd819f383b5b64f5cc4da19829c42c945d9b7d2422d1f8f36f29f2be46` |
| `tagged-backend-snapshot-green-01/receipt.original.json` | `60e2eb1a5ccd16929657f18d43baef187ca2a20797fb6ada8f984c7eac3a138e` |
| `tagged-backend-snapshot-green-02/receipt.original.json` | `dee093461776d89237ef76c42b468e3e066dd77c8a5be5e565fa1f50eaaa63f6` |
| `tagged-wrapper-exact-input-proposal-01/root-review.original.json` | `f7946ea917d435c444f02efdd2af35e4da21618487dc56d663ef834d9e4b59b6` |
| `tagged-wrapper-mac-replay-selection-01/selection.original.json` | `f61142ebdf4eb37b0456e3ccc68e01f244600aa54445e0b95f95b6b39b16e78f` |
| `tagged-backend-snapshot-qualification-intake-01/result.original.json` | `a11358203bcd94a0c99fd85a95ee47a17efe30b7b6120b28ba5ac9557da3dcd3` |
| `tagged-backend-snapshot-qualification-intake-01/submitted-intake-source.original.py` | `2cbfe352c5d6e2f88d29f4bed1e5ae0ac3d2e6432beb9b018a2fa7ab2a5be1f1` |

The Mac originals were acquired without modification under `/mnt/development/projects/evidence/cherri-p10r15-apple-completion-20261125/mac-wrapper-replay-01/original-intake-01`. Acquisition SHA-256: `5cb2e0447895cf58624ecd3c254b06e73ff5dc37d699a28e1baa71b0df57be8a`; original result: `da84f432ad24ea24878e1cabe29f255d3babd6e5a15c77e6dc28bf90cba4519b`; original Go JSONL: `394e050a04e50696fc69338e4b9797b9208c4420544f12ebabd69c46754a49c9`.

## Remaining gate

Join this guard with the qualified cache snapshot producer, rerun the explicit structured-value cohort on the integrated source, and execute the separately enrolled native tagged-carrier fixture. This report grants no iOS runtime tier, formal 179-requirement acceptance, final release, or deployment status.
