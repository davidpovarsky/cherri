# Exact K12 source recovery to persistent storage

At04:44UTC on2026-11-23, three task-owned Ubuntu source copies and Ubuntu
temporary directories were observed missing from the dedicated tmpfs. The cause
is **NOT_ESTABLISHED**. Host boot identity and approximately80-day uptime exclude
a recent reboot. The earlier approved four-copy retirement concerned distinct
persistent paths; no causal attribution to that operation is made.

At04:48:58UTC, the K12 worktree was recovered to
`/mnt/development/projects/worktrees/cherri-p10r15-k12-integration-20261123`
at the exact already committed source
`773ea5628547fbc3c497388ccc35a1a23a3fa796`, tree
`8bbfec1f408afa0c2e1d421cde9bafde4cae5651`. All6,837 tracked files were restored
and status was clean. Preserved shared refs/index and seven metadata originals
were recorded before targeted worktree repair and restoration. No force, prune,
reset, commit replay or branch-ref change occurred. Other worktree metadata
remained unchanged.

The persistent K10, K11, R7, D11 and Apple-owner sources were independently
rechecked, as were the exact R7 compiler/migrator and the complete212-member
storage-evidence archive. Their preserved hashes match the earlier originals.
The recovered commit remains historical; later K12 evidence commits legitimately
advance its branch.

## Future runtime paths

A new empty persistent build cache and owner-specific temporary directories
were prepared at04:56:35UTC and checked using the actual ubuntu and
chatgpt-agent identities. Their exact paths and access results are in
`originals/persistent-runtime-preparation.json`. No questionable tmpfs cache
was copied, no old tmpfs path was modified, and no compiler or producer ran.
The module cache remains unchanged.

Inactive K11 RemoteBuilder/AI/full179 plans are **not automatically updated** by
this preparation. Their old tmpfs source/temporary paths require explicit
reviewed revision for the final source before any launch. Historical source
clone/cache receipts remain valid observations at their original timestamps,
not claims that the missing directories still exist.

## Scope

The bounded packet retains exact recovery observations and process streams.
Seven pre-repair Git metadata originals, including the index, remain on Oracle
with exact paths and hashes in `retained-original-metadata.json`; they are not
duplicated here. This operation recovered committed source and preserved
evidence; it supplies no new acceptance, native, AI or CI outcome.
