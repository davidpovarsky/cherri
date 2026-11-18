# P10R15 production CLI acceptance

The CL01, CL02 and CL03 acceptance producers now execute a newly built current-source binary. They no longer derive formal success from in-process capability structs, directory entry counts or a migration helper. U04 remains separately unverified because its broader historical contract is not replaced by these scenarios.

## Executed checks

- CL01 parses exactly one JSON document from each of capabilities, catalog, complete action schema and single action schema commands. It checks schema provenance, valid/invalid source analysis agreement with exit status, missing source/action errors, and a filename containing --json.
- CL02 observes the actual check command with signing/open/output flags. Instrumented native shortcuts/open executables have positive invocation controls; a loopback signing server has an actual compile/sign positive control. Analysis must leave all input-directory file hashes unchanged, create no Shortcut or command marker, issue no signing request and return valid analysis JSON. The control signer intentionally rejects output; no workflow is sent externally.
- CL03 executes direct-file, compile and check routes on three Legacy-only forms. Every default V2 route must exit nonzero with E_LEGACY_SYNTAX and migration guidance and must not create a Shortcut. A separate explicitly selected Legacy compilation remains supported and produces an independently parsed native plist.
- A shared profile integration test checks that Release preserves require's guard branches, Alert/Stop actions and original producer -> passthrough -> consumer UUID chain while removing diagnostic log actions. This is structural safety evidence, not Apple runtime validation of type witnesses.

The first actual CLI run exposed that a source filename containing --json selected JSON output even without the flag. The adapter now reads only the parsed json flag, and the causal regression passes. The initial signer positive control used a classic signing URL and therefore reached the endpoint but received a Content-Type diagnostic; the test was corrected to use the existing HubSign /sign?response=json route before accepting its diagnostic.

Focused production run after correction: all four top-level tests passed, package9.012s, total16.2s. Full language-acceptance package suite passed, package37.200s, total39.06s. Raw logs are in the authorized local evidence directory. No environment override may substitute an older CLI for these formal producers.

The formal runner discovers and executes each named production test and retains exact source provenance. Its next complete report must be regenerated on the integrated candidate; this document does not relabel historical reports. Linux/macOS CI and Apple runtime/physical gates remain separate.

Independent follow-up: the require type-inspection action was corrected to native getitemtype after comparison with canonical typeOf and the Apple action-class/output metadata. The Release guard assertion uses that independently verified identity; both the producer-tracing native test and Release preservation test passed together on the integration branch.
