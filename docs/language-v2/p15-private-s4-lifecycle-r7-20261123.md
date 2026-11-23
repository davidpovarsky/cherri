# R7 private lifecycle replay ? 2026-11-23

## Result and scope

The single original lifecycle parent passed on clean compiler source `ca316a3dee252456c4bec751594cf249c709b331`:

    go test -count=1 -json -timeout=15m -run ^TestP12_BASE_02_LifecycleStagesAndReconcileCorpus$ .

The Go budget remained **900 seconds**, and the original process guard remained **930 seconds**. The parent passed in **361.100 seconds**, the package in **361.243 seconds**, and the complete driver in **394.088 seconds**, with exit 0 and neither timeout. There were **10,800** valid Go JSON events: one parent run/PASS and one package PASS, with no failures, skips, incomplete cases, orphan terminals, duplicate case events or parse errors.

The driver finished at **2026-11-23T00:23:53.564627+00:00**. The independent audit finished at **2026-11-23T00:23:54.774070+00:00**. A later Sentinel notification still displayed `running`; the on-server original receipt and audit establish the actual finish. The test process was subsequently confirmed exited. Transport status supplied no acceptance credit.

Exactly **38** unchanged inputs contain **62,980,131 bytes** and **27,652 actions**. Fresh and sequential natural counts both equal **23,353**, the per-file differences are empty, and both demotion summaries are empty. Every manifest hash was checked before and after execution. The original invalid input remains unchanged.

This is **one accepted R7 parent out of the five original corpus parents**. `allFiveAcceptedAtR7` remains false. The four other parents have no accepted R7 receipt in this intake. No further corpus parent, full suite, formal179, Apple, device, consumer or release gate was executed by this intake.

## Exact retained evidence

| Evidence | SHA-256 |
|---|---|
| Original input manifest | `766ac82ef7d92f50a894e50bc84d3fed9cc2d375f81ffe2771dbe8a5a6de5f39` |
| Current private stdout | `9db3a93009922680eb7437da08c28bebbe8a1aa2ed5a5eddb01c34a6710b82de` |
| Current private stderr, empty | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` |
| Original R7 process receipt | `fcae4084e7a3017ae891419e833e78b6d30336865df27f95b60c51046a417161` |
| Independent R7 audit | `150d3ac0e4c0d9ac031a18da92aceda8d9b83f1d4156b4afad55b9b8b2fe2d90` |
| Original failed lifecycle receipt | `973e7c03012b9fa257ea42c71dfaf9a720df925667d8f88e8acbef4f154df3a4` |
| Original failed lifecycle stdout | `31b7309adf3b5c0dec446502570ad3c1010e4e0cc9b7f2b915427adf19ab05a0` |

[The original process receipt](evidence/p15-private-s4-lifecycle-r7-20261123/receipt.original.json) and [the independent audit](evidence/p15-private-s4-lifecycle-r7-20261123/audit.json) are byte-identical metadata copies. Raw inputs, private filenames, per-input reports and stdout/stderr remain on the authorized private host.

The recovered archive hash was `fde66e47b0f461a5b1295edbd57d0aab992b32180f5bf007805b2e9d74a8c960`. Recovery selected 97 evidence files, including the exact 38 inputs and three byte-identical manifest copies. No Windows transfer was necessary.

## Execution conditions

Execution used Go 1.24.13 on Linux ARM64, `GOMAXPROCS=2` and `GOFLAGS=-p=2`. Cache and temporary directories were on the development volume. A separate `go test -c` preparation completed in 90.552 seconds with zero tests executed; its test-binary SHA-256 was `9166459e7633d49700b4df935b272243844dfda66cdd9c6039507fde88c8e1dc`.

The original Windows driver, hash `c38d2a7681f2b972632e00626ac602df0f5c2153bb8646c737497065abd40be9`, is retained unchanged. The Linux adapter, hash `9c296224c67b6c9abd16b043f78c763eb1c0edd0cb6aa5266b834823fbd3accc`, changes the exact source/path constants, platform process-tree termination, Linux bash path, pinned toolchain and cache/resource environment. It preserves the original selected test, Go timeout, process guard and event-accounting acceptance rules. Independent intake additionally requires the explicit 38-file reconciliation and exact directory inventory.

The lifecycle test file is byte-identical to both prior S3 and failed S4, SHA-256 `eb8b70716337cba945409df4d60edb23063c2998330bc15da8c1ec8e08c87658`. Current `functions.go` is byte-identical to scanner repair `0f50e542a20a580db1f62d99dbb96a90149762a8`, SHA-256 `e8473d0fac595f107563c5d2f524630f1d407791aea0f24f986ab73635760d28`. Source stayed clean and exact before and after execution. No production or test repair was made during this replay. These durations describe this host and invocation; this intake does not establish a controlled cross-platform performance comparison.

## Existing receipts and remaining parents

All five parents previously passed on **S3 `9c8daf721aa241127df8d1e3f91bbadf90110e73`**, retained in [proof commit a892b0e6](https://github.com/davidpovarsky/cherri/tree/a892b0e6c1b71c0da039aca608c549e3bad3a7b9/docs/ai-team/evidence/20261121-private-s3-five-gates). The original six-member public metadata ZIP was re-read and matched SHA-256 `6d0cdaab9a22d1afc7c9545588d52b7be11642de700515f886eaecf0d6fd17d0`; its aggregate matched `cf9df9e48ced135325549e6187006a686d145ce22a6e4ff28a6b93d4507106f6`.

All five subsequent **S4 `b791ce0616d1821cebfc57909a6bc50a195ca68c`** attempts remained unaccepted. Their original failures are preserved:

| Original parent | S3 result | S4 result | R7 result in this intake |
|---|---|---|---|
| P12 complex parameters / zero demotions | PASS | Supervisor timeout | VERIFY_REGRESSION |
| P13 fidelity / source-size audit | PASS | Supervisor timeout | VERIFY_REGRESSION |
| P10 sequential plan/render measurement | PASS | Supervisor timeout | VERIFY_REGRESSION |
| P11 fresh 38x2 native action wire | PASS | External timeout; no original terminal receipt | VERIFY_REGRESSION |
| P12 lifecycle / reconciliation | PASS | Go timeout | **PASS** |

The [source-dependency map](evidence/p15-private-s4-lifecycle-r7-20261123/source-map.json) records each unchanged test-file hash, every prior accepted receipt hash, the retained S4 failure hashes, and distinct parent contracts. `shortcutgen.go` changed from S3 to S4; `functions.go` and `internal/language/lower/lower.go` changed from S4 to R7. These are shared compiler dependencies of the selected parents. Test-file identity and overlapping lifecycle assertions do not supply exact-dependency transfer or a missing parent's original receipt. The four distinct remaining parents are scheduled separately under their original budgets; today's lifecycle parent is not repeated.
