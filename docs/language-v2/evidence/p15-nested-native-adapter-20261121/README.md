# Nested-list native diagnostic adapter — local verification, 2026-11-21

Execution mode: [SUBAGENT /root/apple_runtime].

Source: `3c118b8722a36c0eff618ab2d5ca70d54036e786`.
Test-fixture correction: `e2fea1fee7d89349ab37892c8402010141a7dbd0`.
Base: K1 `d90b69a92e83b5034728c22e062a3dd96153a2a8`.

## What the adapter establishes

The two existing reviewed public nested-list fixtures can be selected independently by the native harness. Their source/contract pins and exact compiled graph are checked by the existing probe. The new adapter binds source, unsigned graph, signed object, compiler/run/fixture, a fresh callback nonce and the required four-event trace. Independent fixed goals and actual type-witness observations remain separate; contradictory runtime values are retained.

The two new Receiver names are explicitly allowed. Publication recomputes the original files and rejects disagreement or promotion. The new route has no synthetic trace fallback.

All runtime observations remain diagnostic. `semanticContractVerified`, `nativeListTypeProved` and `formalAcceptanceGranted` remain false. A diagnostic acquisition flag is not full language or native list acceptance. Signed bytes and callbacks used in unit controls are synthetic test inputs, never Apple evidence.

## Original results and correction

The development replay completed 54 Python tests (12 new adapter + 15 existing graph-probe + 27 existing callback controls), including the real Go compiler and actual harness oracle/emitter. The actual Foundation Receiver policy verifier completed 26 controls. The independent read-only acceptance-agent review found no blocker in the diagnostic-only boundary.

A full clean replay of 3c discovered 398 cases: 386 passed, four existing POSIX-only controls skipped and eight prerequisite controls failed during setup. Adding the Receiver source to the exact source binding caused its parent directory to exist before the old test setup's unconditional mkdir. No prerequisite control body started in those eight cases.

The test-only correction e2 asserts that Info.plist does not exist, then permits the already source-bound Receiver parent directory. It changes no prerequisite production rule or fixture bytes. All eight actual shell prerequisite controls then passed in 23.201 s.

## Complete case inventory on clean e2

The original full clean e2 process reached its unchanged 900-second limit. It remains TIMEOUT, with no final exit code and no full-process green claim. Its exact 55,074-byte stderr SHA-256 is `f280f61ab253b5cabb5b2c058e7fbf933b9dfeda73e361d395ef9b3e38af0982`.

A fixed inventory enumerated all 398 actual unittest IDs from the same clean source. The original log explicitly completed 311 IDs: 309 pass and two skips. The remaining 87 IDs include the interrupted case, later cases and one case whose multiline expected Git diagnostic prevented conservative attribution of its final status.

Only those 87 unresolved IDs were replayed at the same clean e2 source. They completed in 197.451 s: 85 pass, two existing skips, zero errors/failures. The process exited 0 in 201.078 s. The stderr SHA-256 is `ccf459e7c4cfe986d4ca67a05cacf40883edf74df0d54b7ceeeb2cfcb58b1429`.

The disjoint, explicit result inventory therefore covers all 398 IDs at e2: **394 pass, four existing POSIX-only skips, zero unresolved IDs and zero assertion failures**. Source HEAD and status were identical and clean before/after both bounded attempts. The complete inventory does not erase or relabel the original timeout. A later integrated CI run is still required to establish a successful single-process workflow gate.

The original development, 3c RED, eight-control correction, e2 timeout, remaining-case driver/inventory and successful remaining-case logs are preserved byte-for-byte in `originals.zip`, with a recomputed hash for every member. Manifest records explicitly keep `singleProcessFullSuitePass: false` and `formalAcceptanceGranted: false`.

No Apple nested-list run has been completed by this packet. Native dispatch remains dependent on the repaired common destination gate and a frozen integrated source.
