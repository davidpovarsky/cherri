# BRG32 installed harness inventory repair and actual producer intake

## Original failure and bounded repair

The original K10 Linux job 137450661884 in
[Build & Test 45100706965](https://github.com/davidpovarsky/cherri/actions/runs/45100706965)
failed the strict installed-package inventory in `TestP15BRG32InstalledAIHarness`.
The tracked package contained 17 files; the literal installed inventory and
the BRG32 production-source witness still named only 15. The omitted files were
`fresh-instance-protocol-v2.md` and `test_process_group_cleanup.py`.
That failure propagated to the two original supplemental parent tests.

Commit `d5b82e3a4e21416556d4a6c81ed12b05b1365b2f` adds those exact two paths to
each list: four inserted lines, no removals. The strict equality assertion,
all six original Go producers, all 15 original Python control methods, eight
corruption cases, deadlines, seven AI producer files and three contracts remain
unchanged. Independent review removed exactly those four lines and reproduced
the K10 parent files byte for byte; the new literal inventory equals all 17
tracked package files.

## One actual focused execution

The original `TestSupplementalObligationHarnessProductionEvidence` ran once
on the clean repair commit using Go 1.24.13 on Oracle ARM64. The original nested
three-minute producer budget remained intact. The outer Go process had a
five-minute limit and the supervisor 330 seconds. No live-provider credentials
or evaluation configuration were supplied.

| Observation | Actual outcome |
|---|---|
| Original Go process | Exit 0, 65.355598 seconds including compilation |
| Original parent | PASS, 25.51 seconds |
| Go package | PASS, 25.523 seconds |
| Eight original corruption subtests | All PASS; zero fail/skip events |
| Exported BRG32 evidence | Three records: unit, integration, repository |
| Scoped original report | One of one requirement passed; closure false |
| AI quality / live evaluation | Not measured / not executed |
| Source before and after | Exact clean d5b82e3a |

## Preserved observer failure

The supervisor initially classified its own intake as `FAILED_OR_INCOMPLETE`
because it tried to parse the long JSON receipt from each Go Output event
individually. Go split the logical output across 12 parent-owned events.
The original producer exit 0, nine pass events and source bindings were
already preserved in that same process receipt.

An independent read-only intake joined the original parent-owned output in
event order and extracted the one complete original JSON receipt. It verified
all three BRG32 records and tiers, the exact six required Go producer identities,
the original 15 Python identities, the eight corruption controls and the scoped
report. No test was rerun; no original stream, process result or verdict was
edited. The original observer failure remains in the packet.

The parent does not export its nested raw Go stream or installed-file hash map.
Those facts remain enforced by its unchanged strict executed producer assertions.
A separate 21-file immutable Git review verifies the 17 installed files and
four relevant Go source files. This source review is not an invented nested
execution artifact.

## Packet and limits

`originals.zip` preserves the original command driver, patch, source review,
process intent/start/result and streams, the exact reconstructed logical
receipt and event map, and the independent intake. Every member is bound by
bytes and SHA256 in `originals-manifest.json`.
The 10,462,939-byte original Linux failed-job log remains outside Git, SHA256
`e1ff3221db3334f9ad2d0791f1575f5dd099d3d557e3262a7e2ef178f57a9b83`.
Its bounded independent review is included.

This is a component BRG32 result on d5b82e3a, not a new complete 179-obligation
report, passing final CI, AI quality score, Apple runtime result or release
authorization. K10's original Linux failure remains unchanged.
