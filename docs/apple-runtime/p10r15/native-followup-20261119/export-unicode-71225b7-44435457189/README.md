# Actual production Unicode export failure at 71225b7

[Run 44435457189](https://github.com/davidpovarsky/cherri/actions/runs/44435457189) completed FAILURE at `71225b71f5353a0f661722d154a8b670b52e76f6`. Its 79-action signed production `qa-export-unicode-v2` shortcut returned a correlated SUCCESS callback at 03:28:14.617 UTC, but the payload was `CHERRI_QA_EXPORT_PROBE_RESULT:P15_EXPORT_UTF8_DECODE_FAIL`. The unchanged expected payload was `CHERRI_QA_EXPORT_PROBE_RESULT:P15_EXPORT_UNICODE_OK`. The runtime and trace acceptance gates correctly remained FAIL_RUNTIME.

This is an actual semantic oracle failure after target invocation. It is not a runner queue, receiver self-test or generic import failure. The original fixture is retained exactly and matched its recorded source digest. Signing provenance binds the same source and unsigned plist; the signed AEA1 digest is in audit.json.

The fixture assigns one mutable verdict across successive checks. Therefore the final UTF8 decode mismatch does not prove that earlier File type, name, cardinality, status or base64 payload checks passed. A future causal probe must expose those independent outcomes and bounded public byte witnesses before attributing the failure or changing production. The original failure and acceptance oracle must remain intact.

All 20 published file digests and byte counts were independently verified. The 13 quarantined paths are absent from the downloaded publication. No raster redaction, complete editor oracle or physical-device acceptance is claimed. The earlier literal File/name/extension control has a separate scope; this new production failure does not inherit that control's success.
