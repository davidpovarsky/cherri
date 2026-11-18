# Signed Bool/List and local F01 checkpoint

Exact compiler256f6a639a6f2a47eee5350ffc062746da9bd752 was used by both runs.

- Bool/List run44239174499: actual correlated success with exact first/last output;4 trace events (2inline+2collector), decoded callback payload exactly matches the parsed NDJSON file. Textual JSON formatting differs, as retained in the audit; values and event order do not. This repairs the historical failed Bool/List run at d00.
- Local F01 run44239178590: correlated ERROR/code1, no verified compiled completion or inline trace. The source has no external shortcut dependency. The error text stays redacted and its cause is not inferred. F01 remains open pending causal diagnosis.

Every published file hash/size was independently verified against the publication manifest; quarantined paths and PNGs were absent. Neither run proves full per-action types, Inspector equivalence or physical-device execution. Final integrated-source reruns remain required. The accompanying audit JSON files preserve exact fresh run IDs, source hashes, outcomes and artifact hashes.
