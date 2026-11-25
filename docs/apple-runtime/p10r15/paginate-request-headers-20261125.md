# P10R15 pagination request headers — 25 November 2026

Asana: **1220544665232021, OPEN**. This component extends the existing bounded body-next pagination API. It does not close the full native pagination roadmap.

## Shared implementation

Production commit: `f6cc76f0844eee9b6158d31b38553f7e79b9475e`.
Combined source tested by Go: `68a347d7917dcbc11eb2ba5522659139e55f9072`.
Producer and its corrected independent header decoder: `564c237c952000cc7c50d9e676d1e8d495448b98`.

The optional sixth argument, `headers`, uses the same `WFHTTPHeaders` Dictionary parameter and canonical codec as the existing native `downloadURL` action. The outer Dictionary must be literal; its values may reference runtime Text producers. The macro forwards the encoded Dictionary directly to its single GET node. It does not serialize it as text, copy its values into a new codec, or evaluate header expressions again inside the page loop.

The first five positional arguments keep their meanings. V2 named arguments use the ordinary binder. Omitted headers remain omitted. Either zero cap still produces only the empty native List, its variable store/read and the existing V2 main completion. Page and item limits, original item references, URL-cycle detection, same-origin next-URL checks and fixed diagnostics retain their existing behavior. The native action catalog remains 505 entries.

## Executed evidence

All evidence paths below are relative to:

`/mnt/development/projects/evidence/cherri-p10r15-final-completion-20261125/`

| Evidence | Actual result | Original receipt SHA256 |
| --- | --- | --- |
| `paginate-headers-red-01/result.original.json` | 32 named PASS; 14 new leaves reached the missing argument; four Legacy baseline leaves first exposed a missing test include. The original failure is retained. | `6fd12c9ec6749ec13b7de9309b47cfea123425b1c6d16fe72c0782f58af5e823` |
| `paginate-headers-legacy-red-02/result.original.json` | After adding the established `actions/web` include only to the manual Legacy baseline, all four baseline checks passed and the unchanged pagination compiler rejected the additional argument in all four leaves. | `1273a01f929deda8b071d5600f902c7843fe58634d3153f68ae05284a59413df` |
| `paginate-headers-green-01/result.original.json` | **52 named PASS**, no failed/skipped tests, timeout, capacity abort or source drift. | `87956a34930d32a9d2dcf66ef794c072a5874616bf74e0d733a36d83f0600dee` |
| `paginate-header-model-run-01/result.original.json` | Original producer failure preserved: its new Python decoder omitted the existing V2 direct `WFTextTokenAttachment` representation. Compiler and original Go decoder already handled it. | `0a7dba5b48178ca7c5d985e6fdb45f72848f30e320a8e248868394c0e03824c7` |
| `paginate-header-model-run-02/result.original.json` | **44 actual CLI compiles, 44 exact native-action import/recompile roundtrips, 156 graph-model cases, 44 invalid CLI cases and 14 causal graph faults**; original producer 7.68 seconds. The ten offline boundary/header tests also passed. | `721e0c39675417da5771c4542bcb5983ba8fb64fde20755af7a470e174910741` |

The corrected Go test file has SHA256 `d8847418bbb28e442d1f7d82d7cf77593c1a54a9fe7b8d846b923ae1a7a5fc38` and is unchanged after its test-only correction. All Go source bytes are identical between the qualified Go source and the later Python-only producer correction.

The Go cases cover both dialects and build profiles, an independently compiled manual GET baseline, literal and dynamic leaves, named omissions, both zero budgets, rejection of a dynamic outer container and twelve request/binding corruptions. Header references must resolve the unique actual preceding authored Text producer; an invented binding is not accepted.

The producer interprets the actual compiled graph and sends requests to a real loopback HTTP server. It observes the synthetic Authorization and client headers across three pages, including a middle empty page. It retains all earlier traversal, type, cycle, cap, malformed-body and HTTP-status boundary controls. Seven additional header mutations remove or corrupt the dictionary, field, producer or request body; each corrupted graph fails and its restored control passes. All source files, unsigned shortcuts, exact lossless imports/rebuilds, compiler binary, stdout/stderr and receipts are retained in the isolated producer evidence directory.

## Interpretation and remaining work

This is source, compiler, plist and graph-model evidence. **Apple Shortcuts native execution count is zero in these component runs.** A model performing real HTTP requests does not establish native Content Graph behavior.

Signed native execution, response status and transport/cancellation semantics, Link/cursor/page/offset formats and the proposed trailing-block API remain separate open requirements. The native GET action may follow redirects; same-origin body-next validation and `maxPages` do not prove redirect bounds. No native HTTP-status success, retry or catch contract is inferred here.
