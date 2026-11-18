# File bytes native control at 4e7956d

Run [44346951190](https://github.com/davidpovarsky/cherri/actions/runs/44346951190), attempt 1, compiled immutable source `4e7956d2519307861fe703a11fbe401fc9faca05`. The workflow and strict runtime contract **failed**. Staging and uploading the bounded synthetic artifacts succeeded.

The independently correlated callback was native `SUCCESS`, with exact output:

```text
CHERRI_QA_EXPORT_FILE_BYTES:NAME=bad:TEXT=ok
```

The unchanged expected output requires `NAME=ok:TEXT=ok`. Both inline before/after events and collector BEGIN/END are present in the callback (2 inline + 2 collector records). Replaying the oracle extracted from the exact Git source returns `FAIL_RUNTIME` for runtime and trace. The published strict trace contains one terminal failure record, distinct from the four records carried by the callback.

The fixture uses literal base64 bytes for `{"ok":true}`, then existing Base64 Decode, Set Name, Get Name and Get Text actions with separate comparisons. This result proves only that the text comparison passed and the name comparison failed in this invocation. It does not locate the defect in Set Name, Get Name, extension handling or comparison lowering. Original File/export fixtures and their expected results remain unchanged.

## Evidence verification

The companion JSON records every published path, SHA-256 and byte count. All 20 published files were independently rehashed; the exact allowed path set, absence of symlinks, source Git blob, compiler/signing identities, fresh nonce, separate receiver self-test and exact-source oracle replay were checked. The fixture source hash is `4f095910b9af28d6016cd23ba06ad845f884e17941303edde4d1497bc5a144b2`; its 32-action unsigned plist hash in published provenance is `17b237a49154f9b201b69dadfee7791a41f2640888d81cbb18a06494fc82050e`.

Only bounded published text artifacts were rehashed. Raw signed/unsigned Shortcut bytes remain quarantined, so their hashes are provenance records rather than a new raw-artifact verification. The native environment was iPad mini (A17 Pro), iOS 27.0, Xcode 27.1 build 27A9275, macOS 27.0.1 build 26A434. Inspector remains `NEEDS_REVIEW`; no physical-device, typed File preservation, debug-export or release completion is claimed.
