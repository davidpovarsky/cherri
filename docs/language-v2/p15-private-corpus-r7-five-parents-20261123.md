# R7 private corpus: five original parents — 2026-11-23

## Verified result

All **five distinct original private-corpus parents** passed on clean source `ca316a3dee252456c4bec751594cf249c709b331`. Each parent executed once on this source with its original **Go 900-second / driver 930-second** budget. The lifecycle parent retained its earlier completed run; the four other parents subsequently ran as four separate processes. No production/test source or private input was changed.

The [complete review](evidence/p15-private-corpus-r7-five-parents-20261123/review.json) binds all five original process receipts, independent audits and retained stdout/stderr hashes. The review SHA-256 is `988ce8e362434a0982cb60864e90a1af1046e45748d07bc26c9a5ce60256a243`. Each original metadata file in this evidence directory is a byte-identical copy; raw inputs and logs remain private.

| Original parent | Parent seconds | Package seconds | Driver seconds | Go JSON events | Outcome |
|---|---:|---:|---:|---:|---|
| P12 complex parameters / zero demotions | 215.35 | 215.383 | 218.226 | 8096 | PASS |
| P13 fidelity / source-size audit | 208.69 | 208.734 | 213.114 | 5404 | PASS |
| P10 sequential plan/render measurement | 267.61 | 267.678 | 274.644 | 5473 | PASS |
| P11 fresh38x2 native action wire | 221.38 | 221.423 | 240.362 | 8096 | PASS |
| P12 lifecycle / reconciliation | 361.1 | 361.243 | 394.088 | 10800 | PASS |

Every parent has exactly one run/PASS, one package PASS, exit 0 and no failed/skipped/incomplete/orphan/duplicate-case/parse-error events. Neither a Go timeout nor a process-guard timeout occurred. Source was clean and exactly bound before/after each actual process.

## Corpus observations and preserved inputs

The manifest remains `766ac82ef7d92f50a894e50bc84d3fed9cc2d375f81ffe2771dbe8a5a6de5f39`: **38 inputs**, **62,980,131 bytes**, **27,652 actions**, all hashes unchanged. The explicit P12/P13 summaries report **23,353 natural**, **4,298 exact-native fallback**, **1 original source-invalid**, and **0 Stage-3 demotions**. The original invalid input is preserved.

P11 executes the explicit 38x2 wire/count/identifier/parameter checks. P12 lifecycle reports matching fresh and sequential natural totals of 23,353, empty per-file differences and empty demotion summaries. P10 supplies its own sequential planner/render observations; P13 supplies its own fidelity/source-size audit. These separate accepted parents resolve the four missing R7 receipts recorded by [the earlier lifecycle intake](p15-private-s4-lifecycle-r7-20261123.md).

The earlier lifecycle driver finished at `2026-11-23T00:23:53.564627+00:00`; its independent audit completed at 00:23:54 UTC. A stale Sentinel notification remained running afterward. Its original server receipt/audit, retained unchanged, establish the finish. The lifecycle test was not repeated.

## Execution environment and temporary files

All parents used Go 1.24.13 Linux ARM64 with `GOMAXPROCS=2`, `GOFLAGS=-p=2`, unchanged shared development-volume caches and private TMPDIR. The existing compile-only preparation was reused; it executed zero tests. The unchanged Linux adapter hash is `9c296224c67b6c9abd16b043f78c763eb1c0edd0cb6aa5266b834823fbd3accc`. Its source/path, platform termination and environment differences from the original Windows driver are documented in the earlier intake.

Each of the four new processes retained before/after capacity observations and a 970-second outer owner guard in addition to the unchanged 900/930-second limits. Admission required 2 GiB free; this did not change any test deadline. Unrelated concurrent workloads were observed and left untouched. Runtime values are observations from this shared host.

| New parent | Free GiB before | Free GiB after | Task temp bytes after | Shared Go temp bytes after |
|---|---:|---:|---:|---:|
| P12 complex parameters / zero demotions | 16.495 | 16.461 | 0 | 0 |
| P13 fidelity / source-size audit | 15.988 | 15.151 | 0 | 0 |
| P10 sequential plan/render measurement | 14.972 | 13.718 | 0 | 0 |
| P11 fresh38x2 native action wire | 12.907 | 13.025 | 0 | 0 |

The owner receipts and all private originals remain on the authorized host. No additional full repository clone was created for this evidence; the existing proof worktree was reused.

## Historical evidence and scope

The last historical accepted five-parent batch was S3 `9c8daf721aa241127df8d1e3f91bbadf90110e73`, preserved at [a892b0e6](https://github.com/davidpovarsky/cherri/tree/a892b0e6c1b71c0da039aca608c549e3bad3a7b9/docs/ai-team/evidence/20261121-private-s3-five-gates). The five later S4 `b791ce0616d1821cebfc57909a6bc50a195ca68c` attempts remained unaccepted: three supervisor timeouts, one external timeout without an original terminal receipt, and one Go timeout. Those records retain their original outcomes.

All five parent test files are byte-identical across S3/S4/R7, while three shared production dependencies changed. The earlier [source-dependency map](evidence/p15-private-s4-lifecycle-r7-20261123/source-map.json) explains why the four R7 parents needed actual execution. This review adds that execution; it does not reinterpret an old timeout as success.

Acceptance is limited to these five original corpus parents on exact R7. It supplies no automatic later-source, formal179, Apple/device, installed-consumer, main/Pure or release acceptance. No privacy-sensitive corpus payload, filename or per-input report is included in this public metadata bundle.
