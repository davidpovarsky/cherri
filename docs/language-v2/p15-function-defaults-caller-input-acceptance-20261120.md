# T01 / FN04 actual local acceptance evidence — 2026-11-20

## Outcome and scope

Implementation `403e05fc4f479a52e2d0098670b1d6e85a27aee7` replaces two historical smoke probes with independent, fresh production executions. The immutable T01 row requires `unit+native+iOS`; FN04 requires `native+iOS`. This component supplies T01 unit/native-structure and FN04 native-structure only.

Both rows remain open for actual Apple execution of their complete scenarios. The Runner reports `PENDING_EXTERNAL`. The unchanged local manifest validator reports `NOT_RUN` with pending external tiers: total 2, passed 0, failed 0, skipped 2, closure false. The complete 179-row execution audit contains these two externally blocked rows and does not promote any unexecuted row.

The old `ios27-runtime-poc:functions` trace does not observe the full empty List/Map defaults or all caller-input edges. Only its T01/FN04 memberships are removed; the original source, trace, remaining memberships and runtime artifacts are unchanged. No native-runtime evidence is manufactured by this registrar.

## Actual executions

All commands below ran from a clean source before and after execution, on Windows/amd64, Go 1.25.6, CGO disabled, GOMAXPROCS=2. Every named event passed, with no skipped/failed test, no timeout and actual process exit 0.

| Execution | Named passes | Seconds | Original raw SHA-256 |
| --- | ---: | ---: | --- |
| Complete producer/intake/negative suite, plus existing function inventory and final-iOS meta control | 649 | 31.625 | `94bd993576ff76dcac565fb566bd2b21e62bd60d8345bf31169264f8af460e50` |
| T01 actual child compiler scenarios | 290 | 4.0372736 | `fbf500815f0c5f4f1fbb3f7354a609e381d4e2aaa59055150afce7f4a68735e8` |
| FN04 actual child compiler scenarios | 33 | 2.6095144 | `f64ada0d6ffab50172c5d9e6ada8fd801849b7855b8183cb466beff8976620ab` |

Outer command:

```sh
go test -count=1 -json -timeout=9m -run '^Test(FunctionDefault|NumberFunctionProducerInventory|Meta_UnitAndNativeCannotSatisfyIOSInFinal)' ./tools/language-acceptance
```

Each requirement executes its own fresh root-package `go test -count=1 -json -timeout=3m` selection through the existing finite production command helper. The exact original commands and every selected child name are retained in the two process receipts. A separate bounded four-minute process budget covers each child, including build overhead; no existing timeout was increased.

An initial dirty-source parser/inventory check passed 363 named events in 9.265 seconds. Its original raw SHA-256 is `f513b19d3c54880315a987285711bc22f9d840e2776afa29f43edc791d34a1b2`. It is retained as precommit verification and supplies no formal execution record.

## Independent scenario coverage

T01 executes all nine ordinary typed-default cases with supplied positional, omitted and supplied named arguments: false, zero, empty Text/List/Map, empty List/Map defaults, reserved-key Dictionary data and nested empty containers. It independently inspects every emitted action and edge, and rejects twelve general graph faults plus three explicit empty-List clear/identity faults. Five wrong-typed defaults must reject before output.

The same T01 execution includes twelve Optional Map cases (four shapes × omitted/None/present-empty), nine causal graph faults per case, and ordinary/Optional fallback effect-order scenarios. The latter bind the preceding parameter and require each authored effect exactly once, in written order, inside the absent-argument branch. These assertions distinguish argument omission from Optional value presence. They do not establish general Optional value joins or native execution.

FN04 independently executes complete direct and nested call graphs, the no-function source path, and all marker/envelope/caller-input corruption controls. Authored `system.input` and `ShortcutInput` reads, nested payloads and final consumers must retain the preserved caller reference; the private dispatcher envelope is read only at its explicitly guarded boundary. Other recursion, Void or related repair requirements do not inherit these results.

The underlying compiler/graph component is separately retained in `p15-function-defaults-caller-input-20261120.md` at proof commit `b27257b2f52b99fd1715271af810f012f719558c`. This registrar changes no compiler behavior, native oracle, runtime fixture, codec descriptor or immutable requirement.

## Intake controls and retained provenance

The new producer and validator bind all of the following together:

- Exact independent requirement ID, local tier, authorized producer, full command, scope and complete required event inventory.
- Clean source before/after the real subprocess, the exact implementation commit, and 63 hashes computed from immutable Git blob bytes.
- Original stdout/stderr bytes and hashes, process start/end/elapsed/exit/timeout, required and discovered events, and complete package success.
- Every selected nested ancestor's lifetime; parent or package success cannot conceal a missing, skipped, failed, reordered or foreign child.

The actual positive receipts pass production intake before mutation. Hash-reconciled controls then reject missing producer/tier/artifact/source fields, stale or wrong source, dirty source, failed exits/timeouts, forged benchmark claims, wrong IDs/tiers, fabricated iOS promotion, dropped graph or corruption events, malformed/reordered/invalid-UTF-8 raw bytes, invalid timing, and each of the 63 immutable source-file digests separately for each requirement. These controls run in local validation so a separate missing final-release gate cannot mask a faulty intake.

Synthetic streams test parser control flow only. They never become Runner evidence. Linked FN01/FN02/FN03/FN05/T05/RP25/RP26/BRG11/BRG15 records are explicitly rejected as substitutes.

All 19 original component files are copied byte-for-byte under `evidence/p15-function-defaults-caller-input-acceptance-20261120/component/`. Original artifact paths in receipts and manifests remain unchanged. `source-binding.json` independently records the 63 immutable blobs, the six changed source files and unchanged contracts/compiler/fixture boundaries. `manifest.json` hashes every retained artifact. The raw streams remain the authoritative observations.

Parity agent performed a read-only review of the registrar and exact immutable rows and found no concrete intake or scope defect. That review neither reran the tests nor supplies native-runtime evidence.

## Remaining obligations

T01 and FN04 still need actual source-bound Apple executions matching these complete scenarios. This component does not execute the full 179-requirement runner or claim release closure.

T05 and RP20 still require Optional/JSON-null semantics beyond a precise fail-closed rejection. FN05's complete structured/Optional transport and exact limitation boundary needs its own scenario; Optional control-value joins remain a separately tracked gap. The native serialization observer that preserves a nested Dictionary/JSON matrix is evidence for that exact wire boundary only, not a universal nested-List or JSON-null runtime result.
