# Frozen K11 publication and ordinary source provenance

Frozen K11 is 9b4135e705b08f765b7304e01212d3b9658806fb,
tree d3688f219f7454e80778cbcb3013967b6d339aad. The Windows publication lane
created only agent/cherri-p10r15-k11-verification-20261123 at that exact commit
using the existing account and a create-only ref lease. Main was not changed.

One preliminary guard failed before bundle transfer, import or push because
its expected-ref inventory omitted the existing K10 remote-tracking ref.
That original failure is preserved. The corrected second preflight passed,
and the sole actual push completed at 2026-11-23T03:49:18Z. There was no force
replacement of an existing remote branch. The user's primary Windows
repository, its HEAD, observed status and existing untracked fixture hash
were identical before and after this operation.

A separate mirror-script preparation had a SyntaxError before its first Python
statement; it transferred nothing. The corrected transfer preserved and
verified all four Windows receipt/archive originals. These facts are retained
in the independent intake, not normalized into an all-success history.

The Oracle ordinary source was independently cloned and checked with a real
.git directory, canonical origin, full 3591-commit history, no alternates or
hardlinked files, a clean worktree and exact published remote SHA. Its original
process receipts and outputs are included under oracle-clone-attempt-01.

The independent original intake contains 87 passed publication/source checks.
It performed no compiler tests and grants no formal acceptance. The original
bundle itself remains at:
 /mnt/development/projects/evidence/cherri-p10r15-k11-frozen-ci-20261123/publication/cherri-k11.bundle
Its original handoff records 312997 bytes and SHA256
2b7523dba10e8c4a3043b89e28404282259b366ab4fe1fe7b98d2db7bd42e319.
That Git bundle is deliberately not duplicated inside this Git evidence packet.

All included original bytes are unchanged. This is historical K11 provenance;
the later documentation commit containing this packet is not a new K11 source
identity and is not itself proven by these workflow or clone observations.
