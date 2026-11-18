# F01 lazy-logic probe: actual native error

[Run 44325518117, attempt 2](https://github.com/davidpovarsky/cherri/actions/runs/44325518117/attempts/2) executed the signed 65-action fixture from immutable compiler commit 867261a3ad6ceac113690f5be01f3d576d572759. Library import and the run attempt passed. The fresh source-bound callback was **ERROR**, numeric code **1**, with a redacted error-message length of **56**. It contained no result summary or inline trace. Runtime and trace remain **FAIL_RUNTIME**.

The public bundle was audited against its manifest: 20 declared file hashes and byte counts match, and quarantined/raster files are absent. The source hash matches the exact Git source and both compiler/signing evidence boundaries. Editor capture completed, but Inspector remains NEEDS_REVIEW.

This fixture has no Math actions and no external Run Shortcut target. Its failure therefore reproduces the prior combined local-F01 error shape without either dependency. It still combines nested AND/OR right-hand If expressions with a direct If-expression result control. The exact failing native operation is not identified by the available error code or redacted length.

The independent counter and computed-predicate probes passed previously. The conditional-counter control in run 44325510946 was blocked before native execution because import did not complete. These observations do not yet establish whether the remaining boundary is direct If-expression result binding, nested Boolean composition, or another native contract. No emitter was changed based only on this error.

Attempt 1 remains a separate hosted-runner allocation failure with zero steps; attempt 2 is the first actual native outcome for this run ID.
