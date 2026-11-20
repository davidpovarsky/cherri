# U01 original Git object reads — 20 November 2026

## Result and scope

The clean source 06c5c2fa58183dc8ca3cad923eccc8c854eeed54 passed all 19 required root events through the actual U01 producer and all 140 outer producer/parser/intake controls. An independent audit checked all 50 source blobs, three artifact references, exact process windows, required event inventories, the historical declaration matrix, and all three immutable contracts. The one-row local U01 repository report passes with closure_complete=false. This does not supply B01/BRG01, Apple runtime, other requirement IDs, or final release credit.

## Actual counterexample and repair

Peer review found that a full Git SHA alone does not disable replacement refs. A new test creates its own temporary Git repository, writes two public synthetic commits, activates a replacement, and first proves that an unhardened git show returns the replacement bytes. It never installs replacements in the shared project repository.

The clean tests-only source 8cb4c316cdefe2b6790595e0449d071d415a3dff then fails all four actual helper operations: show, cat-file --batch, ls-tree, and diff (five failed named events including the parent). Its raw SHA256 is f579c1a9a17c3ec471f6dcbb12b1b133e7c9b67ddc82fd29f76059ac7e59c0f9.

The helper now always invokes Git with --no-replace-objects. Its existing 45-second command bound is unchanged. The strict producer requires the original 14 events plus the replacement test and all four children. Missing, truncated, stale, or unbound evidence remains rejected.

## Retained execution evidence

The clean outer replay took 46.813 seconds, with zero failed or skipped events. Its 161,493-byte original stream has SHA256 dac8614f0a466225dad83942d0842cf847ab637d821b997bf5a12cd88893bf38. The actual child contains 19 successful named events; its 36,374-byte stream has SHA256 d3f40aca2084bd63827ab2b4b059b627a24faef1a7cfd67a4a35469bd17e501a.

The declaration result remains 24 of 26 files unchanged since the continuation base, with the same two reviewed single-token parameter-label corrections. Six of 26 are byte-identical to both the upstream and language baselines. Original earlier U01 receipts remain untouched and are not relabeled as replacement-ref coverage.

[Byte-exact archive](evidence/p15-u01-original-object-hardening-20261120/original-evidence.zip) and [archive/audit index](evidence/p15-u01-original-object-hardening-20261120/index.json) retain the new counterexample, actual clean process streams, child receipts, and independent auditor. Every archived byte and staged Git blob was verified.
