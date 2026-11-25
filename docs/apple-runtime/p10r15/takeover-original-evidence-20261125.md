# BRG01 original takeover evidence

This change adds an operational producer and a dedicated repository-tier intake for BRG01. It does not reuse B01 publication evidence or grant any other requirement.

The producer inventories the selected compiler and documentation worktrees, records exact source and ref identities, retains independent staged and unstaged binary patches, and privately backs up modified and untracked source bytes outside Git. Both preflights finish before either create-only recovery-ref operation. Existing branches, tracked bytes, index state and unpublished files must remain unchanged.

Local Git operations discard inherited GIT_* redirection/configuration, disable replacement objects and hooks, confirm the actual worktree/Git-directory identity, reject alternate or grafted history and replacement refs, and reject assume-unchanged/skip-worktree or unmerged index entries. Verification checks all 58 exact original commands, streams, file inventories and backup hashes against a separately selected SHA-256-pinned context, then rereads current sources and refs. Production execution must be launched by a bounded owner; the canonical acceptance intake is read-only and has a 120-second process-tree deadline.

Configure CHERRI_TAKEOVER_EXPECTED and CHERRI_TAKEOVER_EXPECTED_SHA256 only after independently selecting completed original production evidence. An absent context leaves BRG01 NOT_RUN; malformed or rejected evidence fails. The 179 immutable contracts are unchanged. Fixture transactions cannot be retagged as a production candidate.

## Executed controls on 25 November 2026

- 22 actual temporary-repository Python tests passed in 7.126 seconds. These include staged/worktree splits, deletion/rename, Unicode binary unpublished files, ambient Git redirection, hidden index flags, replacement/graft/alternate history, omitted/corrupted backups, modified originals, ref movement, source mismatch and scope inflation.
- 15 named Go events passed in 21.026 seconds including compilation, with zero failed or skipped events. These exercise immutable-module replay, dedicated BRG01 scope, the independent context, artifacts, original stdout and process status, modified current sources, module substitution and malformed/unconfigured inputs.
- Python receipt: /mnt/development/projects/evidence/cherri-p10r15-final-completion-20261125/takeover-preservation-unit-02/result.original.json, SHA-256 d62f4257d7bfb58b7e3d90fb0b7abf0b2f3c642790744dca3480130be5ff15f2. Its startedAt field was written after the test; treat that field as observation time, not a process-start witness. The actual unittest log and monotonic duration are retained unchanged.
- Go receipt: /mnt/development/projects/evidence/cherri-p10r15-final-completion-20261125/takeover-acceptance-go-controls-01/result.original.json, SHA-256 eab27a22415fd946c26665968e2f053e29fe58165812592398a03f53dbfe27f9. Original JSONL/stderr and before/after hashes of all changed implementation files are retained.

No production project takeover has been executed by this change yet. These are control results, not BRG01 acceptance or overall closure.
