# Formal e61 runtime attempt: observed timeout and loop-index mismatch

The actual iOS 27 PoC run [44622055945](https://github.com/davidpovarsky/cherri/actions/runs/44622055945), source `e61e0d306bb363c39ade612ef35ccfc8051ddf3c`, was cancelled at its 45-minute job limit. It was executing the fifth fixture's runtime permission helper when cancellation arrived. The privacy publisher then failed closed and uploaded no artifact. There is no completed formal acceptance or final owned-device cleanup receipt from this attempt.

The original job log is retained unchanged: 1,095,638 bytes, SHA-256 `a428b4be0d3876d62a8ce4e24f5f183914672e3d3280d6f08bc93a9721f820a5`. The [bounded proof](evidence/formal-e61-timeout-20261119.json) records the exact source/run/job binding, GitHub timeout annotation, phase timestamps, public synthetic observations and raw receipt digests.

## Actual elapsed stages

| Stage | Observed time until the next stage |
| --- | ---: |
| Owned Simulator boot | 5 min 29 s |
| Shortcuts availability and initial helper | 4 min 13 s |
| Fresh compiler build | 2 min 57 s |
| Original runtime fixture | 13 min 50 s |
| Dictionary fixture | 4 min 55 s |
| Interactive menu fixture | 6 min 4 s |
| Legacy If fixture | 2 min 52 s |

The fifth S01/F01 fixture began at 22:42:05Z. Its import-helper XCTest succeeded at 22:44:44Z, and its runtime permission helper began at 22:45:05Z. Cancellation arrived at 22:45:15Z before the required exact clipboard observation and final receipt. The workflow now permits 75 minutes for this five-fixture job. This is an outer execution budget adjustment based on the actual cold boot/build/UI duration; every source hash, expected value, runtime assertion, cleanup requirement and publication gate is unchanged. A fresh run remains required.

## Actual loop-index regression

Both the first successful clipboard retrieval (poll 4) and the final retrieval (poll 60) contained:

```text
LOOPS=1:A:1:1|1:A:2:2|2:B:1:1|2:B:2:2|
```

The original source/oracle still requires:

```text
LOOPS=0:A:0:1|0:A:1:2|1:B:0:1|1:B:1:2|
```

This is a native diagnostic of a V2 loop-index mismatch. The golden is retained; a compiler repair and fresh native replay are needed. The other logged scalar, capture, indexing, number and function fields matched their original values, but those log fragments do not grant independent formal acceptance from this incomplete attempt.

The original dictionary assertion, exact Alpha/Beta menu tap plus output assertions, and Legacy If runtime assertion did execute successfully in this log. They remain bounded diagnostic observations with their original scope; they do not create the stale requirement memberships removed by the formal exporter. No S01/F01 coverage, physical-device, signature-trust, complete cleanup, final CI or 179-requirement acceptance is claimed here.
