# Consumer authoring alignment — 26 November 2026

## Qualified source component

The Skill prose source is `00fe18b1589dfc37248da49fd5709dd1784c7a4b`.
The executable documentation source is `5e8e738f0b2edb926a2069e042660ef1bbb8e8d4`
in `davidpovarsky/cherrilang.org`. Both branches are based on the actual integrated
compiler source `5f8acea50fec1249390b5aed6db6b073c43d1173` / historical Docs D16
respectively. These are source components, not a final synchronized release.

The Skill and guide replace obsolete exclusions for mutable local loop values
with the implemented declaration/AST-bound behavior: branches and loops,
immutable snapshots, matching mutable RHS values, direct producers, inferred
types and append. They preserve private-carrier boundaries, actual element-type
evidence, distinct None/null/present empty values, and separate native execution.
The guide adds six runnable examples for these cases and the qualified Boolean
require, measure value/Void, scalar cache and page callback components.

Imported-edit references now include the existing required original-SHA-bound
scalar allowed-delta policy and the comparison gate before signing. No wrapper
implementation, original gate, compatibility manifest or live installation
changed. AEA1 is explicitly a container format check, not Apple execution proof.
Pure remains pinned upstream and Pack is explicit opt-in.

## Actual validation

| Check | Actual outcome | Original result SHA-256 |
| --- | --- | --- |
| Original `check_example` helper, supplied source-qualified compiler | 46/46 PASS: 43 structured checks and unsigned native plists, 3 exact diagnostic/no-output rejections | `4fcf8710175c0b0c4adf3b48a97599ff6d6d8a7382c6eb0e59ccd2620266f4f0` |
| Independent source, stream and plist intake | 92 actual CLI commands verified; all 40 old executed examples retained plus 6 new; all 564 former fences retained | `a919b9da75cbecc366967a2af7793ef580c66556b390a2f4e99a2b1345365498` |
| Canonical Skill frontmatter validation | PASS after correcting inherited arrow characters in the description | `69ba7506665235017ab64370e109f91817d64eff7a42318df6aec61b5ac9038d` |
| Original Docs project-board and both Git whitespace checks | All passed in the first static process; the separate initial Skill metadata failure remains in that receipt | `efe6ec75aa3ff2f3f94a3535127591ec1d4094b7c782d7d68230d53be574dbfc` |

The supplied compiler binary SHA-256 is
`b96cba956e2ec5a9c79ad14fba821d64b796b9ade38c4998f10268fcf46617db`,
built by the root owner from clean 5f8 with Go 1.24.13. This lane performed no Go
build or test. The original smoke source SHA-256 is
`33f49564f527bff35a0217dbf7143e23624f803941d32f29cde6abc556093d01`.
Its `check_example` function was used unchanged. An external recording adapter
supplied the independently hash-verified binary and retained inputs, original
text-mode CLI streams and exact native plist bytes. It did not invoke or bypass
the pinned cache-authentication path. Its actual old release pin stays S627c.

The successful owner ran for 5.016 seconds, with unchanged source/binary and no
timeout, capacity abort or remaining owned process group. The 524 generated or
explicit historical reference fences were classified without execution. Their
reference text is not credited as newly generated or tested against 5f8.

## Preserved unsuccessful attempts

The first execution launcher stopped before invoking the CLI because this new
task's evidence root had the wrong traversal owner. Only that task-owned
directory's UID changed; mode/group and other paths stayed unchanged. The second
attempt ran the complete suite once.

The first independent intake incorrectly iterated a JSON null diagnostic list.
The actual compiler represents an empty list that way. The correction accepts
that explicit empty case, as the original helper does. The actual 46 executions,
source and outputs were unchanged and were not rerun.

The initial Skill frontmatter failure came from inherited arrow characters in
the description. The description now keeps the same intended use with valid
plain text. All failed originals remain separate.

## Original evidence and release dependencies

[Originals manifest](evidence/consumer-alignment-20261126/manifest.json) and
[original evidence archive](evidence/consumer-alignment-20261126/originals.zip)
retain the selected public source, original process streams, native products and
intakes. The archive is rehashed member by member against the actual originals.
The unmodified external skill-creator helper remains on Oracle with its pinned
SHA; only its actual result is included here.

Oracle evidence root:
`/mnt/development/projects/evidence/cherri-p10r15-consumer-final-alignment-20261126`.

The root source review is separate:
`/mnt/development/projects/evidence/cherri-p10r15-final-completion-20261125/component-source-reviews-20261126-01/skill-alignment.review.original.json`,
SHA-256 `48e53a5ccb009d90f3cbb819d4da3c9dd0ec7e1b37f024460a7507a843ab763b`.

The final release still needs the actual frozen compiler source and one clean
compiler/migrator pair; canonical capability/reference generation and check;
coherent Docs/Skill pins; original cache/snippet/site gates; new-source S4 and
publication evidence; original RemoteBuilder 39 entry and 58 signing checks,
package integrity, live activation, rollback/reapply and fresh consumer intake.
Do not reuse the historical D16/S627/K18 receipts as final-source credit.

The existing generator adapter is fixed to 12f. Its source/tree selection must
be explicitly updated or bound to the final selection before reuse, without
weakening its inventories or bounds. Final reference content comes from the
canonical generator. Local Oracle currently has no Ruby/Bundler/Jekyll, so the
existing hosted locked site-build gate remains the publication authority.
