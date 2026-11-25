# Documentation recovery follows the pinned source inventory

The original Local179 run [45545317222](https://github.com/davidpovarsky/cherri/actions/runs/45545317222)
on `1e02755bbfaf470b4dba1802aa410f29ab4d1188` failed RP46 and BRG29 because
three wrapper checks still required 88 examples. Both original example collections
actually executed and passed 89 cases: four intended diagnostic examples, ten
explicit Legacy examples, and 21 contextual examples. The failed full acceptance
report and original workflow conclusion remain unchanged.

The pipeline and independent intake now reconstruct the complete inventory from
the exact compiler S, documentation D, and consumer K Git blobs. They require
all guide sources, every inline reference token, every distinct documented case
and alternative, exact source fields, an ordered one-to-one mapping, and a passed
outcome for every case. Counts follow that verified inventory. The Go parent also
requires unique source identities and complete passed outcomes. Compiler identity,
original process limits and streams, exact intended diagnostics, native plist
checks, package retrieval checks, and the strict Go event parser remain intact.

## Verification at this checkpoint

- The original Python collector controls and the new pinned-inventory controls
  passed: 16 named tests. The new controls reject omissions, duplication, extra
  cases, reordered and misbound cases, altered code, false or hidden failures,
  changed counts, malformed identities, foreign source/manifest bindings, missing
  inline tokens, and unsupported native/closure claims.
- Real temporary Git sources with 16 and then 17 documented cases demonstrate
  that a newly pinned inventory is accepted while the old source and relabeled
  old cases are rejected. These are metadata controls, not compiler evidence.
- The candidate independent intake replayed both genuine original 89-case
  collections, including 849 original files each, their compiler metadata,
  process receipts, exact diagnostics and unsigned native artifacts. No compiler
  process was repeated and no historical acceptance result was rewritten.
- Actual execution of the corrected complete producer is the next qualification
  step after this clean commit. This checkpoint does not claim that result.

The replay receipt is retained on Oracle at
`/mnt/development/projects/evidence/cherri-p10r15-ci-release-20261125/documentation-inventory-original-replay-01/receipt.original.json`
with SHA-256
`9fc1f50972f245865dbc0df07fa047046f3f44eea73f3ff27d0d89712d1124e5`.
The source artifact is the authenticated original Local179 artifact 14992011005
(SHA-256 `65d6168fa276f922edf92e22ee2a277b82f54ccfe57d5dc87fa1faa7cb2434fd`).
