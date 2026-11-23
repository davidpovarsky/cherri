# Oracle continuation checkpoint — 2026-11-23

This packet supports chapter 30 of the living master at the explicit 10:19 UTC
cutoff. It is a bounded publication of verified evidence; it is not a new test
run or a final acceptance report.

## Contents

- `full179.public-projection.json`: exact 179 ID/group/status entries from each
  original K12 report, their original hashes, status totals and the ten newly
  satisfied IDs. Private AI details and unrelated original fields are omitted.
- `remote-installed.public-projection.json`: an explicit allowlist projection
  from six hash-verified installed Remote originals, retaining source roles,
  actual exit codes, 8+8 smoke counts, 58 fault results and acceptance limits.
- `native/*.intake.original.json`: byte-identical already-public K12 F01, full
  iOS and Runtime PoC intake receipts. Prior failures and private boundaries
  remain visible.
- `label/root-intake.original.json`: original component-only K14 label result.
- `rp41/*`: original focused RED test, process receipt, stdout JSONL and empty
  stderr. All test source content is synthetic.
- `manifest.json`: file sizes and SHA256 values for this packet, excluding the
  manifest itself.

## Interpretation

The original complete local run is 112 PASSED / 2 FAILED / 28 NOT_RUN /
37 PENDING_EXTERNAL. The separate retained-evidence intake is 122 PASSED /
27 FAILED / 30 NOT_RUN. Its 27 FAILED categories are the two same AI failures
plus 25 missing local tiers, not 27 new failed test executions. All original
112 passing requirements are retained. No producer was replayed for the intake.

The installed Remote milestone is completed within its original scope. Full
Apple acceptance, physical-device evidence, all eleven macros and global
release remain open. K14's RP41 test intentionally records a real defect and
must be repaired before final source qualification. Later checkpoints must
record new outcomes without rewriting this history.

All copied receipts retain their original schemas and identities. Projection
filenames explicitly distinguish newly created summaries from originals. The
full originals stay in their original server evidence locations; their hashes
are included to make the selected summaries independently traceable.
