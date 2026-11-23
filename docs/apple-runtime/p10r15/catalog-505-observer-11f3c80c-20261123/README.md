# Corrected catalog observer: actual 505 execution and identity controls

The narrowly scoped test-observer correction is committed at
**11f3c80ca261dd59f198cb3693f893c22e6c340f**, tree
c2cf287ab67ad5fc4c1a55e99a631ce3a70b1ec3. One focused execution at that clean
source passed the existing 505 catalog producer and the new exact-identity
controls. This packet contains the original events and fresh execution bytes.

## Defect and correction

The original C0 run passed while recording rawAction.v2EmittedIdent as
is.workflow.actions.nothing. Its bytes, original events and limitation remain
preserved unchanged in commit 2483e4bb9b27d7769b1cac50cde028edfdad19f9 and the
sibling catalog-505-oracle-9ce44842-20261123 packet.

The existing observer had accepted the last emitted action for the raw/rawaction
catalog placeholders. The correction resolves those placeholders to the existing
canonical fixture ID, is.workflow.actions.custom.p10test, and requires an exact
match. The arbitrary-last-action fallback was removed. All three existing
callers are within executeSingleCatalogEntry: Legacy, V2 and recompile.
The separate P15 native-output witness already requires the same explicit ID.

Only the existing test helper and a new control-test file changed. Production
compiler code, the existing producer bodies, both fixture-builder functions,
the separate parameter producer, catalog counts, deadlines and acceptance
contracts are unchanged. This packet establishes an observer correction; it
does not identify a production compiler defect.

## Actual focused execution

- Started: 2026-11-23T00:45:50.751069+00:00.
- Finished: 2026-11-23T00:47:39.758349+00:00.
- Process wall time, including changed-package compilation: 109.00729367509484
  seconds.
- Oracle Linux arm64; actual selected Go 1.24.13.
- Exit 0, no timeout, no failing or skipped events.
- Two parent tests and 14 identity-control subcases ran once and passed;
  one package PASS.
- Source HEAD, tree, clean status and complete dependency witness were
  identical before and after.

    go test -count=1 -json -timeout=15m -parallel=2 \
      -run '^(TestP10_VerifyAndRecord505CatalogExecution|TestP10_CatalogRawActionIdentifierRequiresFixtureTarget)$' .

The unchanged 15-minute Go test deadline and finite 20-minute driver ceiling
were retained. No separate parameter-producer rerun, full Go suite, formal
acceptance suite, release-gate run or derived-ledger generation occurred here.

Each control below ran for both raw and rawaction catalog placeholder spellings:

| Input actions | Required observation | Cases |
| --- | --- | ---: |
| Correct custom target alone | Exact target and its TestKey payload key | 2 |
| Correct custom target, then normalization | Exact target and its TestKey payload key | 2 |
| Wrong custom target | Rejection, no payload keys | 2 |
| Wrong custom target, then normalization | Rejection, no payload keys | 2 |
| Normalization only | Rejection, no payload keys | 2 |
| Empty action list | Rejection, no payload keys | 2 |
| Catalog placeholder action only | Rejection, no payload keys | 2 |

Independent compiler-free intake passed 73 checks covering original manifest
bytes, the frozen driver and source, selected toolchain, exact test events,
catalog and dimension consistency, the explicit rawAction identity, unchanged
parameter evidence and the one-field execution delta. The intake did not run
the compiler or tests again.

## Fresh execution bytes and reused parameter measurement

| Artifact | Measurement source | Bytes | SHA-256 |
| --- | --- | ---: | --- |
| p10-505-execution-evidence.json | 11f3c80ca261dd59f198cb3693f893c22e6c340f | 717835 | 6a99ea3f0e541b4dca416f6cfdef775a7492eef118ddc33f9cc5632398d4aeb8 |
| p10-505-parameter-contract-evidence.json | 9ce44842761ec4bb639e1f2c80c08d86c022b778, reused | 1814832 | 74c5ed6ffb65594937aefcf248cef7f1d5ed47388fc57352c148f478e14f8eb5 |

The execution artifact was copied byte-for-byte from this focused run. Both
rawAction dialect fields now contain is.workflow.actions.custom.p10test.
Against the C1 artifact, only rawAction.v2EmittedIdent changes; all other
records, aggregate counts and dimension summaries are identical. These fresh
bytes happen to match the pre-C0 tracked execution file. Their current
provenance comes from this preserved actual run, not a restamped old result.

The separate parameter result retains its C0 measurement identity and all
original bytes. The explicit caller/dependency comparison establishes:

- The entire p10_structural_roundtrip_test.go file is byte-identical to C0,
  SHA-256 09185a1678c3b346575a51645019399f841b32af3651b990ddf7f135bc2dc0e7.
- All existing non-document source is unchanged except the catalog observer
  function. Every existing caller of that function belongs to the affected
  execution producer, not the separate parameter producer.
- The new control test has no package-level initialization and does not call
  the compiler.
- Both canonical fixture-builder functions are byte-identical. Their hashes,
  the exact caller list and the complete test-only patch are retained.

The 505 execution rows still comprise 497 callable actions and 8 constructs.
All existing local producer counters are 505/505. The six parameter dimensions
retain PASS counts 347, 347, 202, 98, 397 and 65, with no failure, untested or
blocked dimension. Native specimen parity remains 0/505 (NEEDS_EVIDENCE);
Apple runtime remains 0/505 (BLOCKED_DEVICE). No native/device acceptance is
granted by this local result.

## Original packet and next integration boundary

originals.zip contains 18 original or independently derived intake files:
focused stdout/stderr and execution receipts, source/dependency witnesses,
actual toolchain observations, the fresh execution artifact, the exact
driver, the independent intake program/report, the source-equivalence witness
and the complete test-only source patch. manifest.json binds every member and
the archive. The earlier parameter originals remain in the unchanged C0
packet, whose archive and manifest hashes are explicitly referenced.

- Focused driver SHA-256:
  c67b51a24bae7909257366c75a6f542f36613d7e46e779f1eeef564c29aa6639.
- Independent intake program SHA-256:
  f3596079e19b86811a4059de1a0d9df987b13a1caa0afc4ca410fd1e4aac1888.
- Independent intake report SHA-256:
  4123c1b4a854b313efa7c77ba5da2769f38294b2637870541efb65b7e46b6135.
- Test-only patch SHA-256:
  6ed2549240421d3f07edfffdf861b55dcecbee26a4e9acef0062f282cc9b38d9.

The actual input fingerprint at measured source 11f3c80c was
1862ae93d10bae5688fdb7cedd221f2c01020a86aa23a0ac3df4506f7eb3ac6c
before and after. It includes the tracked C1 raw execution artifact; fresh
outputs were external until committed afterward. Derived metadata must be
generated from a subsequent clean ancestor containing the corrected raw
bytes and all agreed receipts. This packet changes no release-gate outcome
and does not resolve the independent protected-path R23 failure.
