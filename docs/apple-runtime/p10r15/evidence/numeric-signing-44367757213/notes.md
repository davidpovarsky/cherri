# Numeric diagnostic blocked before native execution

Exact source 42e7d1cb94542557ada88201178bf9edfb4fa994, workflow run 44367757213. All 20 manifest-listed published artifact files were independently hashed against the source-validated publication policy. The committed synthetic fixture hash matches its compiler entry and immutable source.

The target signer command exited 1: Shortcuty Sign returned HTTP 403 with an HTML challenge; RoutineHub HubSign returned HTTP 500. The macOS fallback also failed, but its published error is truncated, so its cause is not inferred. The Legacy counterpart subsequently has a RoutineHub HubSign provenance record and AEA1 artifact; that does not sign or execute the numeric target.

The target has zero signed bytes and no signing provenance. Only the receiver self-test callback exists. The strict runtime auditor fails closed at missing target source provenance; no numeric flags or native result are asserted. Phase 2 was observed at 18:38:22.4930940Z and Phase 3 began at 18:38:57.3076300Z. Exact hashes, public artifact inventory and time observations are in receipt.json. No retry was dispatched for this receipt.
