# P15 migration and signing output preservation

Date: 2026-11-19

## Reproduced defects

The production compiler at integration commit
`d7052f4568db26dba169bdefc72623e48c04968a` accepted these malformed legacy
inputs through the public `migrate <file> --write` command and replaced the
original file while returning exit code zero:

~~~cherri
@value = "unterminated
~~~

~~~cherri
@value = 1 +
~~~

Each reproduction includes a real LF byte after the shown line. A healthy
`@value = 1` followed by `show(@value)` provides a restoration control.
The migration library previously returned the generated text even when the
V2 parser rejected it. The shared migration path now returns an error and
line/column diagnostics; callers cannot reach a successful write. The
convenience API retains the original source on error. Commit
`9e218c36dd7c11e7403f5057460e594fc106bfb9` contains this production repair.

The new checked-compilation scenario also exposed a separate output-loss bug.
A valid source, explicit remote signer, and pre-existing output were sufficient:

~~~text
cherri compile source.cherri --hubsign \
  --signing-server=http://127.0.0.1:<port>/sign?response=json \
  --output=retained.shortcut
~~~

The controlled local endpoint returned HTTP 200 with an application/json error
response. Both the low-level signing failure cleanup and the V2 compile wrapper
removed the caller's pre-existing destination. This was data loss on a reported
failure; it was not an invalid source bypassing analysis.

## Repair boundaries

Public signing now supplies a private staging path to the signer. Only a
complete result that passes the existing structural archive validator is
committed through the shared atomic writer. Errors, missing/invalid responses,
and final rename failures leave the prior destination intact and remove private
staging files. This boundary covers explicit `sign`, V2 `compile`, default
V2 compilation, and the retained legacy public signing routes.

The lower-level signer retains its existing scratch-output cleanup behavior.
Existing tests that deliberately populate a managed scratch path with unsigned
XML remain unchanged. The public boundary no longer exposes a caller's previous
artifact to that cleanup.

V2 compilation stages its unsigned input privately and passes those exact
compiled bytes to signing. It preserves a pre-existing sibling
`*_unsigned.shortcut` when a signed destination was requested. Explicit
unsigned V2 output also uses the existing atomic writer. The default unsigned
output names remain compatible with the current CLI.

The direct provenance-signing API already obtains and validates response bytes
before writing. It now preserves the prior destination on failure and commits
validated bytes atomically.

## Executable acceptance producers

**RP04** executes five parser-valid semantic failures: immutable assignment,
wrong type, duplicate label, unknown label, and missing argument. The scenario
exercises the embedded compiler, mobile adapter, public check command, explicit
compile command, and default file command. It observes the real native emission
hook and a working loopback signing endpoint. Invalid inputs produce no emitted
workflow, no signed bytes, no signing requests, and no filesystem changes.
A corrected source reaches the emitter and the controlled signer; an unsigned
restoration compiles to an actual plist.

**RP34** invokes the actual public `migrate --write` route for unsupported
directives/triggers and malformed generated syntax, preserving the original
bytes. The positive restoration writes valid migrated source. Its independent
write-failure scenario copies tracked production sources into an isolated
temporary repository replica and changes only the final rename destination to
a child of the existing source file. The actual OS rename fails after
conversion, temporary writing, flush, and close. The original bytes survive and
the staged file disappears. Restoring the exact production writer and rebuilding
allows the same source to migrate. This explicit test-replica fault is neither
an Apple runtime test nor a mocked successful filesystem operation.

The supplemental runner binds these actual test names and commands to the
five required local tiers: RP04 unit/CLI/embedded and RP34 integration/negative.
Its verification test rejects missing producer assertions, unrelated command
or test identities, stale implementation SHA, invented iOS runtime tiers, and
missing tiers. It never sets release closure.

## Observed verification

| Verification | Observed result |
| --- | --- |
| Complete migration package tests | 19 test/subtest passes, no failures |
| Public signing transaction regressions and unchanged existing remote-signing failure injection suite | 31 test/subtest passes, no failures |
| Actual RP04/RP34 producer and twelve evidence mutation controls | 13 test/subtest passes; both local cases satisfy their five required tiers; closure remains false |
| Malformed source control before repair | Public migration returned zero and replaced original bytes |
| Signing control before repair | Valid compile reached the rejecting signer and deleted the prior output |

The signing test exercises all five public modes against JSON rejection,
truncated AEA1, and returned unsigned plist. It verifies exact unsigned upload
bytes for the explicit signing command, restored byte-exact output using a
protocol-only body, output paths containing spaces and Hebrew, private-stage
cleanup, and a real OS commit failure into a nonempty directory.

Local receipts are under `dist/p15-acceptance-causal/`:
`reproduction-baseline.json`, `baseline.jsonl`,
`migration-fixed-unit.jsonl`, `signing-fixed.jsonl`, and
`atomic-fixed-producer-v2.jsonl`. These are development receipts from the
Windows host, not public corpus uploads.

The passing signing/producer verification ran from a modified working tree
based on `9e218c36`. Its emitted development records must not be published as
clean evidence for that base commit. The integration runner must regenerate
formal records at the final clean implementation SHA. No frozen contract or
179-case result file was edited to manufacture a pass.

## Remaining verification

The local endpoint uses an explicitly synthetic AEA1-shaped body solely to
exercise the existing transport and output-commit protocol. It does not prove
Apple cryptographic validity, import acceptance, physical iOS execution, UI
behavior, or release deployment. Those independent gates remain mandatory.
This work does not claim completion of RP43's entire cross-platform scenario
or the overall 179-requirement acceptance.
