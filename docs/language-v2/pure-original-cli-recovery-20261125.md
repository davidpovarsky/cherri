# Fresh Pure Original CLI and preserved Pack recovery

This component replaces a missing external command wrapper. It does not change
the upstream compiler, its action files, or the original Apple Actions Pack.

The compiler source is the official upstream commit
bcc0095754e2cc0baaeb4d93dae172b8153b4716, tree
4fe9fbc8c742aaf01d6ec8193d9be708fb1de7b5. Its original documentation source is
electrikmilk/cherrilang.org@4165cc7224b8f4f0500d95129da9fa14eca5458f,
tree f6c3af5143a1bec1a6391c58569fd0aa47673e29.

A new compiler build is identified by its actual Go version, embedded VCS
identity, binary digest and execution receipt. It is not a reconstruction of
the missing historical executable. The installed fork release remains a
separate tree and retains its existing active tuple.

## External CLI version 1.0.1

The replacement implements the original Pack caller interface:

    pure-cherri-service.sh status
    pure-cherri-service.sh verify
    pure-cherri-service.sh docs [keyword]
    pure-cherri-service.sh compile SOURCE.cherri
    pure-cherri-service.sh sign SOURCE.cherri

A SHA-pinned installation manifest binds the fresh binary, both unchanged
ordinary upstream checkouts, and the external wrapper files. Verification
compares every materialized tracked file with its original Git blob and
rejects checkout changes, missing files, external object stores and origin
drift. Pure verification does not read or require the Pack.

Input files must be inside pure-upstream/jobs. For a project in a subdirectory,
the wrapper retains an isolated byte copy of that project. A single source
placed directly in jobs is copied on its own. Local includes/assets therefore
belong in a project subdirectory. Symlinks, escaping include paths and workflow
names containing path separators are rejected before invoking upstream.
The wrapper does not add action includes, rewrite expressions or author plist.

Each request retains its original process streams, copied inputs, resulting
artifact and a JSON receipt in private job/output directories. Compilation calls
the original binary with the source as the first argument and --skip-sign.
Signing is a separate explicit operation and delegates to the original
--hubsign implementation. Its UPSTREAM_AEA1_FORMAT_ONLY status describes the
upstream container-format check; it does not claim cryptographic verification,
Apple import, execution or physical-device acceptance.

There is no new server process, listener, signing provider or compiler engine.

## Original Pack 0.1.1 and the current home alias

The original archive is Git blob 2d8819e77fc0bf90bb6acf2a0574fa88314f3e5c at
davidpovarsky/cherri@141b9cc96cf1df3f9d1593548dc49ae8a46a256a:

- archive SHA256: 428991eec7ec779385066d5ddea4182354a21bd24859ab08b591a61af3357dc7;
- 105,481 bytes, 41 archive members and 37 regular files;
- all 29 checksum entries and all 33 tracked package files match;
- 20 original action modules remain separate from the upstream checkout.

The original Bash entrypoint sets its root from HOME and then compares
realpath(input) against that uncanonicalized root. The server's preserved
home alias exposes that path mismatch. The external pack_adapter.py pins the
exact original wrapper SHA256 and changes only that single root assignment in
the executed copy. It does not edit the original package or checksum manifest.
The original exit trap cannot represent an apostrophe in its root path; the
adapter rejects that path before execution. The canonical server path is valid.

Use the new external pure-upstream/bin/cherri-with-actions.sh entrypoint.
Its original compile route calls the Pure service directly; enhanced compile
uses the exact original Pack staging/import implementation.
The preserved entrypoint inside apple-actions-pack/bin remains original
evidence and is not advertised as alias-compatible.

## Qualification and scope

Run external-wrapper controls with:

    python3 -B -m unittest discover -s tools/pure_remote -p test_service.py -v

The synthetic shell callee in the alias control records invocations only.
It does not count as a compiler or native runtime test. Actual upstream
build/compilation and current installation receipts are separate producers.

The original archive retains all 204 authored entry calls and expected action
IDs/keys. Replaying those unchanged sources on a new binary is new evidence.
The old producer also reads a historical p10-actions.json outside the Pack;
if that original input is unavailable, a replay must be described as such,
not labelled as the old producer's execution.

This component does not close the 15 Go-only action obligations, third-party
Actions App/ShortcutKit bridge, full Pack runtime matrix, current P10 final
179 acceptance, or the final compiler/docs/Skill/RemoteBuilder upgrade.

## Actual external-wrapper qualification, 25 November 2026

The unchanged 30-method Python inventory passed on Oracle in 0.97 seconds.
The original receipt is
/mnt/development/projects/evidence/cherri-p10r15-consumer-recovery-20261125/pure-cli-component-02/result.original.json,
SHA256 9ba0ceac573538ecc6bbb824b37b0f69a69aa04efb10fe1dc1070612c3f7b043.
The first 26-method run, including its real exit-trap quoting failure, remains
preserved in pure-cli-component-01. The correction rejects root paths the
original trap cannot represent and retains the original Pack script unchanged.

Fresh ordinary upstream source/documentation checkouts and the original
Pack archive passed independent materialized-byte verification. Preparation
receipt pure-source-preparation-01/result.original.json, SHA256
5e52d359d0a64cdeab609f3d254929dc969c9af8f62cf0808dcebab54ef66fd7,
binds 112 upstream source files, 87 original documentation files and all
29 Pack checksum entries. This preparation executed no Go build and changed
no live installation. Actual binary qualification and fresh deployment remain
separate, subsequent steps.

The independent review found that a Pack launch could import service.py before
that dependency's digest was checked. A causal control reproduced its import
side effect on the first launcher (red receipt 430ada258de9aa1332a3bbdc34fc3caa16b340ab696fa1e22a99ba95eb55c3cc).
The revised launcher checks both installed Python modules before invoking
Python. All 31 controls passed in pure-launcher-dependency-green-01, receipt
b6580eeef847945dcb8d6a2da9262366a2cc9045a784e83e8939ae561570fe1d.

The single new upstream build passed with Go 1.24.13 and embedded
vcs.revision=bcc0095754e2cc0baaeb4d93dae172b8153b4716, vcs.modified=false.
Its binary SHA256 is
f1f4396d8e45479b323ad63247f49bd28f0e50a6cd79f1f435f3e392fcf4bec5.
Build receipt pure-fresh-build-01/result.original.json has SHA256
6269d703d441ef22f62dbb7ed3909271d6434b915553109439c5bf381099ff75.
Source, documentation and go.sum bytes remained unchanged.

## Process lifecycle version 1.0.1

The external wrapper now handles SIGINT and SIGTERM while its child is running.
Cancellation is recorded without throwing inside the child-spawn assignment
window. The finally path drains only the new, verified child process group,
including descendants left behind when the direct parent has already exited.
It waits after TERM and escalates to KILL within a bounded cleanup interval.
A create-only process receipt records cancellation, timeout, exit and cleanup.
Cancelled work cannot be reported as a successful compilation.

Actual POSIX controls reproduced four process leaks before the repair:
SIGINT, SIGTERM, a surviving descendant after normal parent exit, and a surviving
descendant after timeout terminated the parent. The RED receipt is
pure-process-lifecycle-red-01/result.original.json, SHA256
5008fadaac9ed59529e6c0b02e34b10d4a18c419d052846484ff325797bfdd9d.

The first GREEN attempt passed seven cases, including all four reproduced
leaks. Its eighth case encountered a test setup race while reading a newly
created PID identity file. That failed attempt is preserved as
pure-process-lifecycle-green-01/result.original.json, SHA256
2ed2b48d6bd5f18c24024d45a8d25d04ea4f6343c88b9a6e268d9b988988cf11.
The fixture now publishes the same PID/start/PGID/SID record by atomic rename.
The exact remaining control and all 31 existing wrapper controls passed in
pure-process-lifecycle-replay-01/result.original.json, SHA256
a02d962383ddb1d06be004be7aa40284a8434cccfad709d1f50d4af828bf1517.
The production service bytes are identical across these two qualifications
(SHA256 f14022765a0ee23f98c98da66bad0717c466fe607ad4ae016275fd47cda3256a).
This is a verified union of eight lifecycle identities, not one eight-case
GREEN run. The unrelated control process remained alive until its own
test-owned cleanup. No compiler build was repeated for this wrapper repair.

## Actual Pure and explicit Pack replay

The freshly built upstream binary compiled all 204 exact archived entry-call
sources and all 21 original module-import sources. The new replay retained
each authored source, unsigned Shortcut, process stream and original result.
It passed ten operational controls, including the expected default-Pure
rejection of a Pack-only call, acceptance through the explicit Pack route,
and successful Pure compilation while the entire Pack was unavailable.
Six causal native-graph corruptions were rejected. These are compilation and
structure observations, not Apple execution results.

Qualification receipt: pure-actual-qualification-01/result.original.json,
SHA256 47f7acb4fb5cf4259f2c4c2a58dc058a588b1a2184a4e9f728522e59977d9ae0.
The replay is deliberately not labelled as execution of the missing historical
catalog producer. Its original archived calls and source templates are exact.

The original public HubSign path produced a 21,929-byte AEA1 container from a
public smoke fixture. Receipt pure-public-hubsign-01/result.original.json,
SHA256 541685c510975848ffa2ace610e0eecd238c26b763f13d6cec66bb1d958bc9e8,
binds the returned bytes. It grants neither cryptographic nor Apple runtime
verification.

The independent intake reconstructed all 225 expected sources directly from
the archived records, reparsed every actual plist, checked every native action
identifier/key set and process-stream hash, verified both ordinary upstream
checkouts and Pack bytes, and joined the exact lifecycle identities.
It preserved the earlier intake setup/status-oracle failures.
Receipt pure-independent-intake-04/result.original.json, SHA256
b4dc26752faa731c8885526f9758811a32c269adf633ec7f36321b2fbc3b8532.

All receipt paths in this document are relative to
/mnt/development/projects/evidence/cherri-p10r15-consumer-recovery-20261125.
The fresh component was subsequently activated and independently verified as
recorded below. This does not change the separate final-fork release gates.


## Actual isolated activation and rollback, 25–26 November 2026

The current wrapper was assembled from d58a55e81720b3396aa0c3cc868876eda22aea81.
Its installation manifest is
ccdc5ef66b0184a89ed62061e3e49ec1b36fe721d0b597fce6d65fd594d74e82.
Five actual current-wrapper commands passed across two attempts: status,
public compilation, the expected default-Pure rejection of a Pack call,
explicit Pack compilation, and public HubSign signing. The original first
attempt stopped at an incorrect new UUID assumption after its first two
commands; both actual unconsumed Text graphs omit UUID and are exactly equal.
The three remaining commands ran once in the separate continuation. Its
21,925-byte AEA1 result remains format-only evidence.

The live activation, rollback and reapply ran once in 25.65 seconds and
executed ten CLI commands as the actual installed user. Canonical and preserved
home-alias paths worked. The before-state contained 955 regular files, all with
single links; ownership changes therefore could not alter another installation
through shared inodes. Rollback restored the staged bytes, modes, owners and
mtime values and the absent-live state. It archived the exact two new request
trees without deleting their originals. Reapply restored the live installation
and successfully compiled the public source again.

The live location is
/mnt/development/projects/agent-workspaces/cherri-remote/pure-upstream;
the preserved home alias resolves to it. The independent intake checked all
962 current regular files, every original CLI stream, actual process cleanup,
three complete action graphs exactly equal to their qualified baselines,
the generated Pack index and all 20 original module bytes. The original K18
ten-file protection set plus both runtime binaries and the alias are unchanged.

| Original receipt | SHA256 |
| --- | --- |
| pure-wrapper-1.0.1-actual-02/result.original.json | 0558d1cf9e691503bc122fe3a1df203e60bf6f55fb5c4fdc1122142436c98b19 |
| pure-activation-cycle-01/result.original.json | b9120c29e72ff282b2d21d7a446ff1818d0472eb1647b3076706c71df3df32d5 |
| pure-live-independent-intake-04/result.original.json | a853872a119e4b7d33ae4d664c516d18020517e00782c9018aaae4fca633fdbf |

The live-intake attempts 01–03 remain preserved: their new input-inventory
assumptions incorrectly treated a Pack request as one source, treated its
generated index as a stored module, and omitted the digit in the original
a11y category parser. Only those intake assumptions changed. No deployment,
compiler run or original output was repeated to correct them.

A byte-exact public evidence archive and manifest are retained under
evidence/pure-cli-recovery-20261125/. It contains only this Pure component's
public authored sources, process receipts, structural artifacts and operational
evidence. The alias-control symlink is represented by metadata, not an active
archive symlink. Private corpus inputs and other-agent history are excluded.

The original tasks were updated in place: Pure 1220105430376487,
Pack 1220105430376551 and RemoteBuilder 1219604454669950. The latter stays open
for the new final compiler/docs/Skill tuple and fresh 39-entrypoint, 58-signing,
package, usage, activation/rollback/reapply and D03 gates.
