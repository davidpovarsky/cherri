# P10R15 RP42 / RP44 / RP45: complete Linux component evidence

## Verified result

The three independent extracted-install producers passed on clean source
`9654056e2191b4867a314f1154a9e767ef0e851f` in
[GitHub run 44697366234](https://github.com/davidpovarsky/cherri/actions/runs/44697366234),
attempt 1. Each producer executed its complete scenario and then exercised the
strict acceptance controls against its actual process records. All processes
exited zero with no failed or skipped named test events.

| Requirement | Required tiers | Actual root events | Outer producer/control events | Inner seconds | Outer seconds |
|---|---|---:|---:|---:|---:|
| RP42 | CLI + skill-package | 14 | 210 | 86.908 | 115.510 |
| RP44 | skill-package + integration | 39 | 233 | 134.730 | 157.904 |
| RP45 | skill-package + integration | 41 | 235 | 142.941 | 164.184 |

Each requirement has two records bound to its full independent scenario: six
records total. Each one-case local report accepts exactly its own requirement
and retains `closure_complete: false`. These results establish the three local
component requirements at this exact source. They do not transfer to a later
integration or complete the project's 179-row final acceptance.

The separate Alpine job also passed all ten original-byte parser controls under
the inspected BusyBox image. Those controls remain explicitly synthetic parser
tests and do not confer additional formal or native Shortcut credit.

## What the complete scenarios actually exercised

**RP42** builds the real compiler and migration companion, generates the actual
reference documentation, commits a disposable component compatibility tuple,
creates and verifies the ZIP, extracts it and invokes the shipped installer.
The extracted commands exercise help/schema, check and aliases, format and
aliases, compile/file output, native-equal import, and the installed migration
tool. Both documented flag positions are executed for migration check and
write. No placeholder binary substitutes for those commands.

**RP44** independently exercises real outdated bundled and cached compiler
selection, compatible replacements, explicit override rejection, and checksums
before execution. Original raw capability bytes cover malformed grammar,
decoded duplicate keys, invalid UTF-8 and NUL, incompatible protocol/catalog/
schema/codec/macro identity and wrong documentation provenance. Controlled
capability responses must be observed before their precise rejection; they are
never counted as real compiler output. Changed docs, absent Git provenance,
wrong revisions and both hidden index flags are rejected. The shipped offline
compatibility consumer runs with actual same-source tools and generated docs.

**RP45** independently exercises exact embedded source, role and platform
identity for both tools, mandatory receipts and byte checks before execution,
runtime ZIP ownership, replacement-disabled Git objects and actual offline
installation. The source-build case advances source/docs branches, then builds
the exact pinned revisions. Corrupt replacements and handled interruptions
after real filesystem operations preserve the six previous tool/receipt files
and exact previous docs. The test also covers invalid explicit migration
overrides, lock acquisition signals, an existing lock owner and a successful
complete update. The separate shipped self-test uses actual tool bytes with
explicitly controlled download transport; it does not replace the real source
build scenario.

Rollback evidence covers the implemented handled command/signal boundaries.
It makes no power-loss or simultaneous-reader guarantee.

## Independent source and receipt audit

The original artifact archives were downloaded directly from GitHub. Their
reported source, run, size and SHA-256 were checked before extraction. Extraction
rejects traversal, duplicate members and symbolic links and applies bounded
member counts and total size.

The independent audit then verified all 64 reviewed source digests against
immutable Git blobs at `9654056e`, with replacement objects disabled. All three
original process inventories are identical. The audit also verified:

- Exact clean source before and after each outer and inner process, one manual
  workflow attempt, and the published workflow byte hash.
- Original output/stderr/process digests, finite process durations, nested
  intervals and raw event timestamps.
- Every required named root run/pass pair, including nested ancestor ordering;
  a parent or package pass cannot replace an absent child.
- The exact actual receipt extracted from Go output against the separately
  published component JSON, with two same-source records per requirement.
- Original artifact hashes referenced by each record, exact scenario commands,
  required event inventories, separate tier identities and component-only scope.

The complete producer runs include real-record corruptions, all 64 reconciled
source-digest mutations, linked-requirement rejection and final-closure controls.
Missing skill-package evidence remains an exact local NOT_RUN result with zero
accepted cases and is rejected under final acceptance. Malformed records still
require a local validation error.

Readable original proofs are retained for
[RP42](evidence/p15-extracted-install-linux-20261120/rp42-component-proof.json),
[RP44](evidence/p15-extracted-install-linux-20261120/rp44-component-proof.json)
and [RP45](evidence/p15-extracted-install-linux-20261120/rp45-component-proof.json).
Their historical absolute artifact paths describe the original Linux process;
the unchanged original bytes are inside the retained archives. They are not
rebound to this documentation commit or any later integration.

## Single publication and measured execution

After independent review, exactly one push created
`audit/cherri-p10r15-packaging-r2-20261120` at `9654056e`. The remote ref and
published workflow bytes were verified. Exactly one manual request dispatched
workflow `424727272`; the API then identified the single source-matching run
and attempt. The original probe at `8ddaefb0` remains unchanged.

The workflow SHA-256 is
`25178f84b2b3c403ae01d07b56845a6b43cfb6bcdfa9fa0a52534e1efc012a0b`.
These jobs used Go 1.25.14 on Linux/amd64 with the reviewed bounded process
configuration. None exhausted its deadline. The three inner scenarios took
364.580 seconds in aggregate across independent jobs; this is measured
component work, not a prediction that the entire root suite will take that
duration. Existing full-root, full-acceptance and release deadlines are unchanged.

BusyBox executed image
`sha256:49340b346798875b5592e5d420241f2f9cfba5f47e62ebb1446a1b34734e898f`
with networking disabled and a read-only source mount. All four source hashes,
the executed image ID, original outputs and unique ten-control success marker
were independently checked. Its recorded pull/inspection/execution interval was
1.518 seconds.

## Original archives and previous failures

| Artifact | ID | ZIP bytes | Original ZIP SHA-256 |
|---|---|---:|---|
| RP42 | 14611646565 | 213156 | `42191263dda578deb62670e804e33a7ec8bc28a0943d24807e61186eac0d9dee` |
| RP44 | 14611286922 | 46135 | `e76f3f2665f946177030b41cece3a98d5e07569097f258048ecf50990fbe5c12` |
| RP45 | 14611556493 | 52477 | `caadf6c1fd72891e080c16bfb567710142a8527be35a8e33dd1188e511c44844` |
| BusyBox | 14610838860 | 4102 | `ed11cbf9c65be7b3d0cdca22712a061e9db58eecf02e4b2dec78096ea086b165` |

The [manifest](evidence/p15-extracted-install-linux-20261120/manifest.json),
[independent audit](evidence/p15-extracted-install-linux-20261120/independent-source-and-event-audit.json)
and [original receipt archive](evidence/p15-extracted-install-linux-20261120/original-linux-receipts.zip)
retain 23 original members and 613,819 bytes, including the four original
GitHub ZIPs, all original job logs, terminal metadata and single-publication
receipts. Every member was reopened and compared byte-for-byte. Archive SHA-256:
`2a07496d9eae7c27e56831374895149de2ac0abb000ca263c0fbc10f30d8c88d`.

The [first local evidence](p15-extracted-install-local-20261120.md) and
[first Linux/intake repair evidence](p15-extracted-install-linux-followup-20261120.md)
remain separate and unchanged. Run `44694030530` is still failed. The later
successful run does not relabel its missing tier/name failures or the original
fixture and assertion failures.

## Integration and remaining scope

This isolated component starts from
`059d6bf45ebc27bf58a695c1f5cc016b511b069b`. The published compatibility manifest
is byte-identical to that base: final S/docs/consumer pins have not been changed
by these disposable component tests. Actual packaging and consumer changes need
fresh execution after integration and publication of the coherent final tuple.

Independent read-only reviews covered the installer, parser, pair identity,
rollback, consumers and strict records. Concrete findings received bounded
repairs and retained causal replays; review itself was never counted as execution.
Root integration must preserve all existing requirement registrations, historical
source identities and evidence archives. Apple runtime, complete final CI,
all 179 requirements and deployment remain separate obligations.
