# P10R15 extracted installation: first Linux run and strict intake repair

## Verified status

The first complete Linux workflow remains **failed**. Its three original inner
installation scenarios passed, but the outer strict producer found missing
packaging-tier accounting and an incorrect selected child name. Those failures
were repaired without changing product tests, immutable contracts, required
counts, generic packaging guards or CI deadlines.

The subsequent complete RP42 local producer passed on clean
`95611881917b8b80f1b00958070984ff76f98f88`: **210 outer named pass events and 14
actual inner scenario pass events**, zero failures/skips, exit zero. The outer
process took 113.594 seconds; the inner current-production scenario took 84.583
seconds. Both source states were clean and identical. All 64 source digests were
independently checked against immutable Git blobs.

This establishes the complete RP42 component's local CLI and skill-package tiers
at that source. RP44/RP45 still require a fresh successful complete strict
producer. The new Linux workflow must run all three independently on its next
reviewed source. Final integrated 179 acceptance, the published consumer tuple,
Apple runtime and deployment remain unproved by this component result.

## Original Linux execution

One exact probe push registered the additive manual workflow. Registration run
[44693862944](https://github.com/davidpovarsky/cherri/actions/runs/44693862944)
succeeded and skipped the expensive manual-only jobs. One subsequent manual
[run 44694030530](https://github.com/davidpovarsky/cherri/actions/runs/44694030530),
at attempt 1 and source
`8ddaefb0a9fdc9d2ad91ad4d5f7032504c402ebe`, produced these original outcomes:

| Requirement | Actual inner events | Inner seconds | Inner exit | Outer result |
|---|---:|---:|---:|---|
| RP42 | 14 pass, 0 fail/skip | 83.181 | 0 | Failed strict intake |
| RP44 | 39 pass, 0 fail/skip | 87.731 | 0 | Failed tier accounting |
| RP45 | 41 pass, 0 fail/skip | 137.837 | 0 | Failed tier accounting |

The actual inner receipts bind clean source before/after, exact commands, all
64 reviewed source blobs, raw Go output and stderr digests. The original
workflow conclusions and outer exit codes are not promoted because the inner
processes succeeded. The 94 inner events are useful product observations at the
original source, not substitute successful formal producer evidence.

Original artifact archives were downloaded directly from GitHub and checked
against their API-reported size, digest and workflow source identity:

| Artifact | ID | Original ZIP SHA-256 |
|---|---|---|
| RP42 | 14610256431 | `ddcf83d2de666abb41565f8ceb5f402738b67ae200dd17c9df858e45badf0f73` |
| RP44 | 14610440393 | `b8d671c7322686c839d5dfcdbca54bf6f4c5568c709b9238c0c8d9cd5dc5420f` |
| RP45 | 14610291244 | `a0cb4fedec277d2f965fc9888c6ba9a0556ffaf6dcfca1fcb499ea42c15bc7f9` |
| BusyBox JSON | 14609951498 | `e56b311cbf18b0a2946cc9710b8ecbb108ce1d2fe7baed12fe08b1c9f14dbabb` |

The actual Go version was 1.25.14 on Linux/amd64. Outer durations were 103.745,
100.326 and 156.001 seconds for RP42, RP44 and RP45 respectively. No process
exhausted its deadline. Existing full-root, full acceptance and release budgets
were not changed based on these measurements.

## Why formal intake failed, and the bounded repair

`Runner.Record` deliberately refuses to create `skill-package` records from the
generic path. The new extracted-install producer had correctly executed its
whole scenario but relied on that generic path for both required tiers. It
therefore produced only the CLI or integration record. The strict accounting
assertion correctly failed rather than accepting incomplete evidence.

Commit `6dfaa735da01a280b507b833d3b7944f5e2bcc12` follows the existing dedicated
package producer convention: after an actually executed complete process it
adds a separate packaging record carrying the actual success/error status and
exit result. The shared postprocessing binds both records to the same complete
command, event inventory, source, artifact bytes and explicit component scope.
An unexecuted producer still returns NOT_RUN without minting records. The
generic packaging guard remains unchanged.

The same commit corrects the RP42 selected name from `after_file` to the
unchanged actual root test's `documented_after_file`. Both check and write still
require the before-file and after-file commands. No event is removed and no
parent pass can cover an omitted child.

The first clean local replay at `6dfaa735` reached the complete positive two-tier
report, but retained an overall failure: 206 pass events and one failing leaf
plus three failing parents. The leaf removed the package tier and incorrectly
expected a malformed-record error. Existing local acceptance policy instead
returns an exact incomplete NOT_RUN case because the missing package tier is
external. It returned zero accepted cases and no closure, so no false acceptance
was observed.

Test-only commit `95611881917b8b80f1b00958070984ff76f98f88` now requires that exact
incomplete result: total 1, passed 0, failed 0, skipped 1, the same requirement,
NOT_RUN, the exact remaining tier and no closure. The same control also requires
an error and zero accepted cases under final acceptance. Every malformed
source/raw/process/identity fault continues to require a local validation error.
No validator, requirement or product behavior changed in that correction.

The successful 210-event replay includes every real-record corruption control,
all 64 hash-reconciled source-digest corruptions, linked-ID rejection and the
unchanged 179-row audit. The current actual records are retained separately in
[rp42-local-proof.json](evidence/p15-extracted-install-linux-followup-20261120/rp42-local-proof.json).
Historical records are not rebound to this source or any later integration.

## Actual BusyBox byte controls

The independent Alpine job succeeded. It executed BusyBox 1.37.0 from inspected
image ID
`sha256:49340b346798875b5592e5d420241f2f9cfba5f47e62ebb1446a1b34734e898f`,
not a mutable tag. The source was mounted read-only and container networking was
disabled. All ten synthetic byte-parser controls passed, including raw NUL,
invalid UTF-8, decoded duplicate keys, malformed numbers, surrogates and trailing
documents. The exact success marker occurred once.

The original receipt was independently checked against the four source blobs,
original raw outputs, image-inspection JSON and executed image ID. Its total
recorded pull/inspection/execution interval was 4.470 seconds. This is actual
BusyBox execution of synthetic parser controls, not native Shortcut execution
or independent credit for any complete formal requirement.

## Preserved evidence and independent review

The [manifest](evidence/p15-extracted-install-linux-followup-20261120/manifest.json)
and [original follow-up archive](evidence/p15-extracted-install-linux-followup-20261120/original-followup-receipts.zip)
preserve the four original GitHub ZIPs, four original job logs, run/job/artifact
metadata, single-push/dispatch receipts, source verification, failed local replay
and successful current replay. The archive contains 37 members and 4,184,985
original bytes; archive SHA-256 is
`2723fd7d970d8e6af9448c2b870fbbd44a6c08cd2412c4cfb312158e25b43b1e`.
Every member was re-read and compared byte-for-byte after ZIP creation.

Independent read-only review found no remaining concrete scope or fail-open
issue in the dedicated record and missing-tier correction. That review is not
execution evidence. The original first-run failures, failed local assertion and
all published source identities remain unchanged.
