# P15 U01 — actual declaration preservation boundary (2026-11-20)

## Verified component result

Source `6bdddd6d5ea3012b806ae82239cd7a7fe49e2138` passed 135 named acceptance tests. Its actual child process passed all 14 required repository events: two parents and twelve causal boundary controls. The independent audit reconciled all 50 producer-bound source files with their exact Git blobs and verified the three referenced original child artifacts.

The local report contains one U01 repository record, one passed row, and no failed or skipped rows. `closure_complete` remains false. This is source-bound component evidence; the combined integration and final 179-row report must run again on their own source. B01/BRG01, other requirement IDs, native runtime, consumer installation, and final release receive no inherited credit.

## Why the previous claim changed

The previous planning document claimed that the entire declaration tree was byte-identical to upstream. The current repository does not support that statement. The manifest now distinguishes four immutable baselines:

| Role | Commit |
| --- | --- |
| Historical upstream declaration baseline | `a66db15b7f247f3121726c2a2b72becfeef3d96b` |
| Language baseline | `559abceadda1dbf0cd7d7deb1a26315b8ac44c3f` |
| User-specified continuation | `d7052f4568db26dba169bdefc72623e48c04968a` |
| Recorded R3 source | `8e66254d56ea3e4ac70f6cda6ef9d31217e0937c` |

The inventory is exactly 25 `actions/*.cherri` files plus `stdfunc.cherri`. Of these 26 files, 24 are byte-identical to the continuation baseline; only six are byte-identical to the historical upstream and language baselines. Inherited fork differences remain explicit.

The only two changes since the continuation baseline are independently allowlisted one-token parameter-label corrections from `dc86a87de5cd6b2f2e795236ac1fa25e28853ade`:

| File | Original declaration fragment | Corrected fragment |
| --- | --- | --- |
| `actions/device.cherri` | `text ?startTime: 'endTime'` | `text ?endTime: 'endTime'` |
| `actions/location.cherri` | `text addressLine2: 'WFAddressLine1'` | `text addressLine1: 'WFAddressLine1'` |

Native keys, declaration order, defaults, types, and all other bytes are preserved. The original Git numstat is exactly one insertion and one deletion in each of those two files. No syntax conversion or mechanical mass rewrite is admitted.

## Actual executions and integrity

| Execution | Clean source | Named passes | Original stdout SHA-256 |
| --- | --- | ---: | --- |
| Repository component | `dc32dbb68b34078cefd3941415febc0d468c5390` | 14 | `57c0f5147e54834e7624278226b8225950d3eb8e17d3a835a973699a80f3d12f` |
| Strict producer, parser and intake controls | `6bdddd6d5ea3012b806ae82239cd7a7fe49e2138` | 135 | `313db52bdb3b84d74d8e111afc62139d5d6f9100f37209fe2fda0c5a24f7d54c` |
| Actual repository child of that producer | Same source | 14 | `9d1a9a3ae6ea4b0bffcc553ca950a6841619db7e4966e830ff0295cfc19ecb6c` |

All executions returned zero, with no failed or skipped events. Source was clean and unchanged before and after. The producer joins actual Go JSON output chunks, binds command and process time, requires every named event, and rejects edited, missing, foreign-source, false-success, wrong-tier, or unrelated-ID evidence. Synthetic parser streams are parser controls, not production proof.

The repository test reads actual Git objects with `git cat-file --batch`, the actual working bytes, and the complete tracked declaration inventory. Twelve causal controls cover missing or extra files, incomplete or duplicate manifest entries, wrong baseline, wrong length or digest, unapproved rewrites, forged current hashes, incorrect correction metadata, and false equality classifications.

The immutable 94-row acceptance, 52-row repair, and 33-row recovery contracts remain byte-identical. The independent audit checks original event counts, process timing, source hashes, report scope, all 104 historical declaration blobs, all 26 current files, and the exact two-token delta without relying solely on the producer's reported success.

## Original evidence

See `evidence/p15-u01-upstream-boundary-20261120/originals.zip` and `originals-index.json`. The archive contains the unmodified original process and JSONL files, the actual child receipt and artifacts, and the independent audit script and result. Each member is checked against its original bytes and indexed by byte length and SHA-256. Historical R3 and earlier reports remain unchanged.
