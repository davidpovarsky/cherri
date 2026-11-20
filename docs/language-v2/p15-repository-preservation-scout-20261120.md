# Repository preservation scout — 20 November 2026

B01 and BRG01 remain open. This read-only scout records recoverable evidence and a historical payload gap. It does not produce a formal acceptance record.

## Evidence that was actually checked

The original staging branch and original local closure branch still resolve to d7052f4568db26dba169bdefc72623e48c04968a. The live handoff ref was read, and Git ancestry to the exact da0ba0e5 native component was verified. The 19 November handoff records a clean original worktree; it does not contain a two-repository inventory of unpublished bytes and private backups.

The current snapshot covers 132 registered code/docs worktrees: 120 code and 12 documentation trees. Of these, 115 were clean, 13 had changes, and four missing/prunable/locked trees were not queried. No HEAD changed during each individual status query. This is a current snapshot, not an initial-state reconstruction. Fresh remote queries were used; stale tracking refs were not substituted for live remote heads.

The existing public main-integration receipt was replayed against actual Git objects. Merge 48828b7ba7eada5237396ef5461e91d13cf286d4 retains 15 main-only commits and all 11 recorded file hashes in both the original main parent and the merge. The relevant merge comparison has no compiler/actions/iOS changes. This proves the recorded committed merge scope only.

## Historical backup boundary

The public October takeover record identifies the original code/docs commits and a private backup location. The referenced backup exists, with four files totaling 27 bytes: one nonempty status record and three empty metadata files. Their hashes are recorded in the companion proof. The nonempty record matches the documented untracked status. No original artifact payload archive or contemporaneous payload file-hash manifest was found there or at the two other known original locations.

The final bounded recovery search enumerated only authorized Cherri Code roots. It returned 957 candidate backup/archive-manifest filenames, 21 unique candidate contents, no filename containing the original public run ID, and no candidate manifest tied to the original public identities. The current GitHub artifact listing for public run 37624625658 is empty. No additional recovery source was found. These observations do not establish that an unauthorized deletion occurred.

The October task branches have advanced. Code ancestry was checked locally and documentation ancestry through the GitHub comparison API. Preserved ancestry is distinct from unchanged historical branch heads. An existing later stash was not treated as an original backup.

## What remains unproved

The initial untracked payload and its contemporaneous digest inventory were not recovered. The present inventory cannot prove that every unrelated unpublished byte remained unchanged throughout the earlier takeover. B01 and BRG01 therefore receive no completion credit from this report. No receipt reader or validator exception was added.

The companion JSON records public source identities, contract hashes, aggregate observations and hashes of host-retained receipts. Raw private filenames, status bodies and backup payloads are not included. The scout changed no source, test, validator or original ref. This commit adds only these two report files on an isolated audit branch.

## Public sources

- [Original handoff](https://github.com/davidpovarsky/cherri/blob/4a35b56a193b1f688a6b53d047a60b8b4867b5da/docs/ai-team/handoffs/cherri-p10r15-final-handoff-20261119.md)
- [October takeover record](https://github.com/davidpovarsky/cherri/blob/46f98a1ebb6cce8c7bfbd097d64a0b2b990f4f6c/docs/language-v2/backend-recovery/takeover.md)
- [Committed main-integration receipt](../ai-team/handoffs/p15-main-integration.json)
- [Companion proof](evidence/p15-repository-preservation-scout-20261120.json)
