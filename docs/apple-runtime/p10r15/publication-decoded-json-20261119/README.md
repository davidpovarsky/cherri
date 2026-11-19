# Decoded JSON publication gate

Source 3bcf1f9bacc098485a69281e424e2ea377737126 fixes five reproduced
publication bypasses against baseline 908831b55480dd152dc411042f4f0a6e9a40b0ee.
Legal JSON Unicode escapes could hide canaries in keys and values from the
serialized-text scanner. Nested LOCAL_ONLY_RAW_OCR markers were also accepted.

The publisher now inspects decoded JSON and JSONL, rejects duplicate decoded
keys rather than discarding earlier values, and rejects local-only OCR markers
at every nesting depth. Failure diagnostics never echo the matched secret.
The exact original bytes are retained locally; no partially approved output is
published after a failure.

The focused 17-test suite passes, including the five baseline failures, duplicate
keys, symlinks, synthetic-source provenance, large complete AX trees, raster-only
English/Hebrew/low-contrast canaries, and preservation of raw evidence.
The clean source then passes all 76 publication/OCR/native-audit/storage/HTTP
regressions with no skipped tests. The initial broader 60-second attempt timed
out and is retained as incomplete; the bounded 300-second rerun completed.

This proves the synthetic text allowlist and quarantine gate. It does not claim
arbitrary private-data detection or pixel redaction. Fresh native workflow
publication on the combined source remains a separate integration observation.
