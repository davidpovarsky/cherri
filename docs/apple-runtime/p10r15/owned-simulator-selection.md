# Exact simulator ownership for a shared Mac

The normal dedicated GitHub runner still uses the existing automatic simulator
selection. That entire selector is unchanged.

For an existing shared Mac, the local execution owner may set
`QA_OWNED_SIMULATOR_SELECTION` to an original JSON record. The record is the
operator's explicit destination authority; a simulator name is not evidence
that a device belongs to the current task.

The record has exactly these fields:

```json
{
  "schema": "cherri-owned-simulator-selection/1",
  "sourceCommit": "<exact checked-out 40-character commit>",
  "runId": "<exact current QA run ID>",
  "udid": "<explicitly authorized existing simulator UUID>",
  "deviceName": "<exact name from the current device inventory>",
  "deviceTypeIdentifier": "<exact current CoreSimulator device type>",
  "runtimeId": "<exact installed iOS 27 runtime identifier>",
  "runtimeName": "<exact installed runtime name>",
  "runtimeVersion": "<exact installed runtime version>"
}
```

The selector reads the original runtime, device-type and device inventories
captured by the existing shell. It requires one available matching runtime,
one matching device across the entire inventory, and the exact source, run,
device name, type and profile. A missing, duplicated, unavailable or mismatched
device is rejected before boot. No alternative device is selected or created.
An explicitly set empty or missing record is a rejection, not the normal
automatic-selection route.

The create-only private receipt
`owned-simulator-selection.original.json` retains the input hashes and either
the exact selection or its rejection. Selection is not a native execution or
runtime acceptance result. The public-artifact allowlist is unchanged.

For the current Cherri Apple lane the reviewed owner may bind only its existing
D631D98E-2271-4948-B465-8EDBA9BD4D68 simulator. Other project devices remain
untouched. A future owner must establish its own explicit authorization and
original source/run record before using this local option.

Controls cover competing first-listed devices, absent/ambiguous destinations,
runtime and type/profile mismatch, source/run relabels, malformed records,
duplicate keys, symlinks, bounded inputs, create-only failure receipts and
physical working-byte tampering. They do not invoke Apple software.
