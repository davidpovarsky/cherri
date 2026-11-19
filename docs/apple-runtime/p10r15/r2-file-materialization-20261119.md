# R2 native JSON File materialization observation

## Actual result

Run [44634901025](https://github.com/davidpovarsky/cherri/actions/runs/44634901025) compiled and executed the original public `qa-file-materialization-diagnostic-v2` fixture from compiler commit `4e56dbcfe895baa8f944b83715ecff0d07bfd24e`. It imported successfully, reached the callback, and failed the original exact File-type oracle. The source fixture and all expected values remain unchanged.

| Observed stage | Actual native type | Expected type |
| --- | --- | --- |
| JSON source text | Text | Text |
| Base64-decoded JSON | Dictionary | File |
| Renamed decoded JSON | Dictionary | File |
| Empty debugExport | Dictionary | File |
| One-entry debugExport | Dictionary | File |
| Independent AQ== binary decode | File | File |

The other fourteen fields match exactly: the three item counts are 1; empty export has zero entries and populated export one; JSON, decoded JSON and named JSON have identical Base64 byte witnesses; the Unicode payload restores exactly; the name is `boundary.json`; and payload status is `TEXT_PAYLOAD_ENCODED`. These are observed comparisons against the original expected fields, not a replacement oracle.

This isolates native content classification at the JSON Base64-decode boundary. Naming the result does not establish a File carrier. A replacement carrier still needs its own type and byte observations. This diagnostic does not prove a production repair, all generic content types, six logging modes, physical-device behavior, or formal requirement completion.

## Evidence integrity

- Original fixture SHA256: `f008fe73c47c9d90695b965e585b748a62698f6e4bb8cad97b21ddf158717863`.
- Original artifact: `14584558732`, 273771 bytes, SHA256 `2693620d7e6f6ae43a9f85a9f989246b555829e20eaf002a0f5931885ecddcdc`.
- Existing strict `audit_native_continuation.py` verified all 21 published file hashes and original source/run/nonce bindings; exit 2 preserved `FAIL_RUNTIME_PRESERVED`.
- The fresh callback has one matching identity and status SUCCESS, but its exact semantic output differs in the four type fields above. The original callback contains two inline events and two collector events. The canonical runtime/trace gates remain FAIL_RUNTIME because the exact output oracle failed.
- Published artifacts contain approved synthetic evidence only; raster and raw system log publication remained disabled.
- The unchanged original ZIP, GitHub metadata, full job log, publisher files, audit and comparison receipt remain in the local run evidence directory.

See `r2-file-materialization-20261119-audit.json` and `r2-file-materialization-20261119-observation.json` for exact source, signed/unsigned artifact, callback and file digest bindings. No passing acceptance record was created.
