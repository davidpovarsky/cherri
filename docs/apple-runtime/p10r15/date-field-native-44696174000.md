# Date parameter native observation — run 44696174000

The actual Apple parameter-state probe completed successfully on immutable source `96f67fae308925f7f647bc4c730f2bca1a307cbf`. It establishes why the current Date references disappear at the native parameter boundary; it does not execute `dateDiff` or close a runtime requirement.

## Observed boundary

| Input form | First Date | Second Date | Native reserialization |
| --- | --- | --- | --- |
| Original bare attachments | State absent | State absent | No state to serialize |
| Both token-string references | State present | State present | Exact reference UUID/name preserved across three native-object hops |
| Only first field wrapped | State present | State absent | Wrapped first reference preserved |
| Only second field wrapped | State absent | State present | Wrapped second reference preserved |
| Literal specified Dates | State present | State present | Original literal values preserved |
| Both parameters absent | State absent | State absent | Negative control |

The independent `formatTimestamp` positive control already used a token-string reference: its `WFDate` state was retained. Replacing that input with the corresponding bare attachment removed the state. All observed Date parameters were actual `WFDateFieldParameter` objects with declared `WFDateFieldParameterState`; no action was executed.

The six direct state probes and fourteen action-parameter observations, including all three-hop typed snapshots, are preserved in the adjacent JSON. Each next initializer received the previous native `serializedRepresentation` object directly. Foundation false, zero, null, missing-key and deep-container snapshot controls passed.

## Source and artifact integrity

Run [44696174000](https://github.com/davidpovarsky/cherri/actions/runs/44696174000), attempt 1, job `136153339347`, ran on macOS 15.7.9 Intel from 07:45:47 to 07:46:04 UTC on 2026-11-20. The original artifact `14611107095` is 15,709 bytes with ZIP SHA-256 `37b5e7efdb775e6bff37dac2e5afbdac7288cb9ab4f65f8a71483100575e107d`. All seven published files, pinned source inputs, immutable observer/prelude bytes and combined executable script were verified. The adjacent JSON lists their exact hashes.

The input specimen comes from a retained local compilation at the reviewed source, with UUIDs pinned in `date-field-compiled-controls.json`. It is not a claim that the locally compiled shortcut equals the separately downloaded runtime shortcut byte for byte. The earlier runtime diagnostic still failed its original 27-field oracle; all five date intervals produced no value while the Date formatting and independent numeric-zero controls worked.

## Next gate

Use the shared canonical reference encoder for the two `gettimebetweendates` Date fields, preserve explicit native bytes and the existing format control, then execute the unchanged original runtime diagnostic on the corrected source. No expected value, fixture byte, acceptance membership or native success assertion is changed by this report.
