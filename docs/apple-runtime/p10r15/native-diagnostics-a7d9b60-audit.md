# Native require and File controls at a7d9b60

The three original strict diagnostic fixtures ran on the exact frozen compiler source. All60 published synthetic text files were independently rehashed; source, unsigned-plist metadata, signed-artifact metadata, fixture and fresh callback IDs agree.

| Run | Control | Observed output | Strict verdict |
|---|---|---|---|
|44333755039|Number-zero literal/direct/require/bound controls|L=ok:D=ok:R=ok:B=ok;2inline+2collector events|PASS for this diagnostic|
|44333772194|File transport and Dictionary payload|B=ok:V=ok:VT=ok:L=ok:T=bad:R=bad:S=ok;2inline+2collector events|FAIL_RUNTIME|
|44334287258|Existing Text-to-File/name/readback path|ERROR code0, redacted message length82, no result or inline trace|FAIL_RUNTIME|

The first run verifies equality comparisons of the Number-zero outputs. The earlier full require fixture used an inequality guard; that original acceptance source is rerunning unchanged. This diagnostic does not close all typed require cases.

The second run shows that Get Variable preserves the File type and bytes for the reviewed source, and the label and original input survive. The failure is at the Dictionary payload restoration boundary. The earlier broad Get Variable hypothesis is not supported by these controls.

The third run removes debugExport and JSON parsing from the failing path. It does not yet distinguish Get File of Type, Set Name and readback. A separate existing base64Decode-to-File control is being prepared; no serializer or compiler workaround is claimed.

All three keep ordinary callback/trace/result gates. Inspector identity remains NEEDS_REVIEW, physical testing BLOCKED_DEVICE, and pixel redaction unverified. Detailed metadata and hashes: native-diagnostics-a7d9b60-audit.json.
