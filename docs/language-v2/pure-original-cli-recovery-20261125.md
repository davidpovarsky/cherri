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

## External CLI version 1.0.0

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
