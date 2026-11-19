# Formatting and workspace resource boundary repairs

These are actual public production-route tests and causal failures, followed by clean exact-source replays. The immutable acceptance contracts were not changed.

## E04 formatting
The original formatter changed raw backslashes, emitted unsupported escape spellings for control bytes, lost comment bytes, and could return formatted partial syntax through the service. The preserved baseline includes five failing syntax run events and four failing actual-production run events (parent events included). The repair uses Cherri-supported string spellings, preserves comment text, and refuses invalid service input.

The working candidate passed 35 syntax/service checks and 24 actual CLI/service/strict native graph checks. The exact clean c621786 replay passed both formal producer tests, which execute the actual production scenario and reject cross-tier/truncated evidence. The clean raw log is SHA256 9d92aa31d0decd433f5c3bb981adf02f40f68475d647f229c30c647de7751d7e.

## IM04 workspace reads
The baseline actually read a synthetic file outside the importing document directory through a Windows junction, both with and without an extension, and incorrectly rejected a legal directory beginning with two dots. The repair holds a Go rooted directory handle throughout open/read, rejects nonregular resources, and keeps extension fallback limited to missing paths. It does not disclose the outside resolved path in its diagnostic.

The candidate passed 27 actual module checks. The exact clean 8418ac2 replay passed both formal producer tests, executing real local link escape/healthy controls and rejecting unrelated commands. Its clean raw log is SHA256 03868f82f8ec79c7156e995bc6b1964233b5ef4e82d53f8496802153dbd3ab5a.

## Provenance and limits
The archive contains exact baseline/candidate/clean raw logs, summaries, the public test snapshots and runner. The manifest lists every archived byte count and digest. Candidate receipts before committing remain identified as such; only the separately recorded clean replays have a clean immutable source identity.

These gates do not establish Apple runtime, iOS editor behavior, the other module requirements, or complete final-source acceptance. The full 179-case matrix must be run on the combined source.
