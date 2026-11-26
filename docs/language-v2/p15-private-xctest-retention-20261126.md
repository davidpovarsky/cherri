# Preserve private XCTest originals before QA cleanup

The selected Simulator QA shell now retains the available completed or partial
XCTest bundles before removing its scratch directory. The original source at
`31d7b8d871618ff3eb82a65f0afd994d2c336d36` deleted these bundles after extracting
summaries. The completed local F01 observation therefore has no retained raw
xcresult, and this change does not reconstruct or replace that missing evidence.

## Scope

Only the shell cleanup and its new filesystem controls change. The three bundle
names and three existing metadata/log names form a fixed allowlist. A
same-filesystem rename preserves original bytes, modes and inode identity inside
the existing private `builds/xcresult-originals` directory, mode 0700. Existing
destinations, symlinks, unexpected object types or ownership cause rejection
before remaining scratch is removed. The ten-second retention deadline remains
armed through receipt close and fsync.

Repeated cleanup after the owned scratch directory has already been removed is
explicitly idempotent. This protects an installed early EXIT trap. The original
successful F01 path reset that trap before final cleanup; the repeated-trap
control is an adverse lifetime control, not a claim about the cause of the
observed F01 result.

All 21 original native command lines and timeouts remain byte-identical. No
fixture, callback parser, assertion, selection, public upload allowlist or
acceptance mapping changes. Available raw bundles remain private; this change
does not itself add a public or encrypted hosted uploader.

## Actual qualification

The retained originals deliberately include the unsuccessful iterations:

| Original run | Actual outcome | Scope |
| --- | --- | --- |
| qualification01 | Original deletion control FAIL; first retention draft 7 PASS | Linux filesystem and shell execution |
| qualification02 | Old draft 2 FAIL; current production 8 PASS / 1 FAIL | The one current failure exposed the test extractor's partial-comment line |
| qualification03 | Corrected extractor against old draft 1 FAIL; current production 5 PASS | Only the five methods that use that extractor were rerun |

The final current-production result joins **nine unique passing control
identities**: five from qualification03 and four unaffected methods from
qualification02. It is not a single nine-control GREEN invocation. Every method
body is unchanged across the extraction correction; only the shared
`normal_cleanup` extraction helper changed. The four reused methods do not call
that helper.

Current production SHA-256:
`b029c0bf1af37e38a8e47dfb1a7a3dab8fda0807260592c01b9fb13fd6dd231c`.

Current control SHA-256:
`e474ff491d1b2d6db29daa921865c518b7efac4c9f45586e5a393e3088d03196`.

The original qualification receipt's `source` field identifies the 31d baseline,
not a clean commit containing these subsequently edited files. Exact executed
production/control bytes are bound by the independent source hashes in each
intent and source snapshot. qualification02 failed before writing its closing
source snapshot; that absence is retained explicitly. Later exact input
readbacks and the final current-source checks are separate observations.

The final original intake made 243 checks, including all six child process and
stream receipts, their original test-event projections, exact source hashes,
bash syntax, the unchanged 21 native command lines, and the ZIP byte/mode
readback. All six child parents were reaped and their owned groups were observed
ABSENT.

## Retained proof

The adjacent `evidence/p15-private-xctest-retention-20261126/` directory contains
the exact synthetic originals ZIP, member manifest, intake receipt, named
control join and intake owner. The ZIP contains public project source and
synthetic fixture/process originals only; it contains no actual private
xcresult, screenshot, user shortcut or signing material.

- Intake: `a1c9e311b50c2d5dc26732fadc2d206e86718509be2025ac1365c39f8ea6ca79`.
- qualification03: `b9a532b3e94b81fef8ff1ce0190a96ee5040b41b5132eeac54398658ae458114`.
- ZIP: 356,675 bytes,
  `56346c5436ed07a8a17b6a14288a015c2e811bfda273432c187b827771b466e7`.

A Mac qualification admission stopped before executing any control because its
existing free-space requirement did not fit. These passing controls were
executed on Linux/Python 3.12.3. This checkpoint grants no new XCTest, Simulator,
physical-device, signing, semantic or formal acceptance credit.
