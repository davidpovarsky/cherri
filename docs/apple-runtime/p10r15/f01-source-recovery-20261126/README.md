# F01 selected-source guard: physical reads and failed cleanup

This component addresses two source-prerequisite defects observed in F01 run
[45831731192](https://github.com/davidpovarsky/cherri/actions/runs/45831731192)
at Q, commit `44707350fb6e52dd0d947256bced105ac762a1ac`.

## Original boundary

The run passed all 995 offline tests and Phase 0. Its simulator was booted,
then the source-before guard failed before listapps, harness build, import,
Shortcut execution, or callback. All ten requested Git objects were returned
completely and independently hash-verified. The guard retained the first
39 of 62 physical file observations, then reached its unchanged 10-second
acceptance cutoff before completing the initial scan or any closing scan.
The originals do not identify the exact filesystem syscall at the cutoff.

The original Git parent was not reaped. Its cleanup group observation was
PERMISSION_DENIED, not ABSENT. The existing cleanup used only the exhausted
acceptance deadline as its wait allowance. No attribution is made for the
Darwin permission result beyond what the original receipt recorded.

The private original intake remains separately retained with SHA-256
`432147f40bb9c194f3118d0ec2e1124a1571f281782e5689f6394f9814cba2ef`.
The additive phase diagnosis is
`668fa25290dbabe659620372220dbe020488270cedd3259869dbbc0c2def0833`.
Private native originals and key material are not included in this proof pack.

## Changes

The existing physical reader can use per-invocation directory handles for
relative filesystem operations. It still freshly verifies every file and
ancestor, file type and size, identity before/open/after, content, Git blob,
physical owner, two complete physical passes, and closing HEAD. Open
directory handles are reused; cached metadata or bytes cannot stand in for
a fresh observation. Replaced or symlinked ancestors fail closed. All
directory and file handles are owned across cancellation and closed on exit.

The listapps caller supplies its existing 90-second phase deadline for
failure cleanup. An already-failed source decision may use at most two seconds,
capped by that phase, to reap its own Git parent. Successful verification
still must complete within ten seconds. Unknown group state remains a
failure. The default caller without an outer cleanup deadline retains its
previous deadline. The 62-file scope, per-file limits, stream limits,
listapps 60-second limit, phase 90 seconds, and outer 120 seconds are unchanged.

Production implementation:
`c4167820514cf8bfab08b1c8164de0a59e85d3da`.

Qualified source with additive safety controls:
`4100ac5bd362cdbe32d6eb2ee0c0746248a9d03d`,
tree `d03ee3a5ab78dee03e9d012250c8180c641f6adc`.

## Actual qualification

| Observation | Source | Actual result |
| --- | --- | --- |
| First control owner | `78fded3e` | No tests ran: the new methods were in the wrong test class. Original failure retained; correction moved the same method bodies. |
| Two causal controls | `5e28502e` | 2 assertion failures, 0 errors, 0 skips |
| Same two causal controls | `c4167820` | 2 passed, 0 errors, 0 skips |
| Directly affected existing and new safety controls | `4100ac5b` | 33 parents and 21 explicit subtests passed, 0 failures/errors/skips |
| Actual clean worktree source guard | `4100ac5b` | 62 files, 11 raw trees, both HEAD observations; SOURCE_VERIFIED in 111 ms on Oracle Linux |

These are distinct executions. The 35 passed parent identities are unique;
this is not one whole 35-test run. The original Q 995-test suite was not
repeated locally.

The physical-read causal control retained both 62-file passes. Its baseline
performed 1,572 full absolute-ancestor lookups for 14 distinct ancestors.
The repaired route used 14 directory handles and no repeated absolute
ancestor lookups; all relative ancestor checks remained fresh and all
handles closed. This measures avoided path-resolution work, not a hosted
runtime speedup.

The reap control uses a real owned child and explicitly delays its synthetic
SIGKILL delivery by 50 ms after injecting an expired decision deadline.
The baseline leaves its parent unreaped; the repaired route reaps it and
observes its group absent while retaining SOURCE_UNVERIFIED. Additional
controls exercise actual SIGTERM during directory/file handle registration,
ancestor replacement and symlinks, same-invocation source/HEAD/mode changes,
redirect rejection, unknown groups, late completion and default reader
compatibility. Original assertion calls are unchanged; one old test callback
only forwards the reader's new optional keyword.

## Retained evidence and remaining scope

The adjacent qualification receipt binds the three control executions,
the real-worktree preflight, exact streams and source snapshots, the
zero-test setup failure, and all 564 proof-pack members. The ZIP contains
original synthetic-control records and public source-only observations.
Its SHA-256 is
`5d15413013d74d68db220ac8a8567aa1b6d398bfca8f8927a5317a6ed0533dec`.

Qualification receipt:
`6ed8573af1b623964b341a25673a91c907372794c513415f8fc67b87020b35da`.

Complete Q-to-qualified-source four-file patch:
`b7fc39816088d32eadefa0d01ebbda23cc85e41cfb44f5bbbf27815a965cffb7`.

This component does not establish that the hosted guard completes within
ten seconds. No native workflow, simulator operation, Shortcut, Go build,
or Apple acceptance was executed by these qualifications. F01 and the
other selected native fixtures remain pending an explicitly allocated
source-bound hosted invocation.
