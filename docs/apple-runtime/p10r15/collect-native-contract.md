# Existing native Repeat Results: bounded collection acceptance

Asana collect task1220544651247337 requires benchmarking the existing loop before adding an alias. This stage retains the existing Legacy/V2 loop implementations and adds no collect macro, storage, serializer or action catalog.

## Measured production graphs

| Dialect | Input N | Total native actions | Repeat region actions | Repeat-region append actions | Input construction appends |
|---|---:|---:|---:|---:|---:|
| V2 |0|6|4|0|0|
| V2 |1|6|4|0|0|
| V2 |100|6|4|0|0|
| Legacy |0|7|4|0|0|
| Legacy |1|9|4|0|1|
| Legacy |100|207|4|0|100|

Both existing loops compile one native Repeat Each region and bind its end UUID directly to Repeat Results. Legacy separately constructs authored array literals before the loop, with two native actions per element. That real cost is retained in the table; it is not a loop accumulator and is not hidden behind a constant-total-graph claim. A preliminary test assuming constant total Legacy graph size correctly failed and was narrowed to the actual collection region after inspecting the emitted graph. No production optimization is inferred from that corrected test assumption.

## Four signed acceptance fixtures

The explicit selections are qa-collect-counts-v2, qa-collect-counts-legacy, qa-collect-dictionaries-v2 and qa-collect-dictionaries-legacy. Count fixtures observe N0/N1/N100, singleton value and the two end values of the ordered100-item result. Dictionary fixtures observe two ordered dictionary results, the actual native typeOf result and two nested list values. The type action is canonical getitemtype; getType is the separate existing File conversion API.

Each callback interpolates actual native count/item/type producer UUIDs. Replacing it with a hard-coded success marker fails the source oracle. The existing callback validator rejects wrong counts, order, type, run identity, errors and missing trace. These fixtures use the ordinary Debug NDJSON contract; no Release exception or publication relaxation is added. Source bytes remain reviewed static fixtures and all raw inputs/raster artifacts remain quarantined.

The expected native count marker is CHERRI_QA_COLLECT_COUNTS:empty=0:single=1:one:many=100:0:99. The expected Dictionary marker is CHERRI_QA_COLLECT_DICTIONARIES:count=2:first=first:last=second:type=Dictionary:nested=2:a:b. These are acceptance expectations, not observed Apple results until a signed run is attached.

## Local verification and scope

The actual compiler built and compiled all four fixtures, and every lossless V2 import/recompile preserved the complete native action array exactly. Two top-level production Go tests (six pass events) passed. All83 existing/new Python runtime, selector, Inspector and publication tests passed. Native execution is still NOT_RUN in this source commit. The source receipt is a later independent metadata commit.

This proves existing source/wire behavior and prepares real runtime checks. It does not prove nested Repeat Results flattening, null omission, measured native memory/latency, universal content families or physical-device behavior. Bool values are preserved in the fixture Dictionary wire; the current callback does not claim native Bool type identity. These remaining distinctions stay open. Rollback reverts only fixtures/harness/tests/docs; compiler behavior is unchanged.
