# Remote Builder paired runtime repair — 2026-11-22

## Result and boundary

Implementation [`607691ded86640f3130cf1b9bf17c9072e0ea536`](https://github.com/davidpovarsky/cherri/commit/607691ded86640f3130cf1b9bf17c9072e0ea536)
repairs the Remote Builder staging and promotion path for the current compiler
and migration-tool package. Its parent is `d2f86d2998bb15d9271a58279a51a400eb13d3d5`.
The published tree `280d37a57ef989535e313a73cc2f32aeb515ccfb` exactly matches the
reviewed local implementation, and the remote commit was fetched into a clean
ordinary clone before building the real tools.

The 18 focused Python tests passed. The actual Cherri compiler and migrator then
built from that clean published commit, passed source/runtime ZIP creation and
verification, and passed the extracted Linux amd64 doctor and public wrapper
self-test. This is component verification. No live Remote Builder activation,
ARM64 execution, signing service call, Apple runtime, or final S/D/K acceptance
was performed.

Role: Tomer / integration. Execution: real subagent `/root/docs_consumers`.
Related Asana tasks: `1219604454669950` and `1220432417485023`.

## Defect and repair

The current source package contains 23 members. Its paired runtime adds both
tools and four source/hash sidecars, producing 29 members. Remote promotion
still required the historical 20/23 inventories. It also omitted the migrator
argument to `expected_package`, staged only the compiler, and expected the old
compiler-only provenance shape without the Go main-package identity.

The repair makes staging require `--migrator`, verifies both embedded source
identities and their distinct main-package roles, and requires matching
platforms. Staging copies both verified snapshots and sidecars. The tuple now
includes `migrationSHA256`; the immutable status reader and the promotion
runtime check reject changed migration bytes. Promotion also checks both
migration sidecars and passes both actual tools to package verification.

The evidence gate requires the complete paired provenance, exact 23/29 member
counts, and 39 distinct entrypoint checks: 29 route/integrity checks plus ten
tuple-field corruptions. The new migration drift and tuple checks are required.
Historical receipts remain unchanged and cannot satisfy the current gate.

The maintained [promotion protocol](../apple-runtime/p10r15/remote-promotion-protocol.md)
now describes these inputs. A targeted caller search found no other active
staging command invocation. The signing and smoke collectors preserve and
compare the full tuple; they do not maintain a second fixed field inventory.

## Verification

| Execution | Result | Scope |
|---|---|---|
| Initial regression draft | Failed, retained | Its obsolete expected source count of 20 encountered the actual 23 members before preflight. |
| Baseline with observed inventory | Failed, retained | Actual production preflight rejected the valid source archive with `Unexpected committed package inventory`. |
| `python3 -B -m unittest -v test_remote_package_preflight test_remote_promotion` | 18 tests passed, 22.53 s | Actual Git, ZIP, Go metadata, generated status reader and filesystem transactions. Small Go main programs and unrelated deployment receipts are explicitly synthetic. |
| Actual clean Cherri compiler build | Passed, 4.15 s | Go 1.24.13; `-p=2 -buildvcs=true -trimpath`; Linux amd64, CGO disabled. |
| Actual clean Cherri migrator build | Passed, 0.42 s | Same source, toolchain and platform; correct migration main package. |
| Source ZIP create and verify | Passed | 23 exact committed members. |
| Runtime ZIP create, verify and extract | Passed | 29 exact members including both real tools and sidecars. |
| Extracted `doctor.sh` | Passed, 0.47 s | Both actual tool identities and the exact D8 documentation checkout. |
| Extracted `self-test.sh --skip-setup` | Passed, 7.51 s | Eight public CLI operations and ten controlled installation scenarios with the actual tools. |
| Runtime ZIP verification after smoke | Passed | Original source and runtime inputs still match the archive. All three source checkouts remain clean. |

The focused tests reject wrong tool role, foreign source, differing platforms,
migration byte/commit/hash corruption, old package inventories and missing or
changed migration provenance. Existing lock, no-replace, rollback/reapply,
handled signal, hard interruption, concurrent-file and journal-error controls
still pass. A real historical partial entrypoint receipt remains a rejection
control.

The wrapper self-test executed version, action discovery, catalog, unsigned
build, decompile, recompilation, prepare-edit and AEA1-input rejection. Its ten
installation cases use controlled download responses and an owned minimal docs
fixture. The separate doctor used the real pinned documentation checkout. The
wrapper observation file contains `records: []`; it grants no formal acceptance.

## Exact component identities

| Item | Identity |
|---|---|
| Compiler and migration source | `607691ded86640f3130cf1b9bf17c9072e0ea536` |
| Disposable manifest-only component Skill | `3b7afdbdc76f7f26fd7874b1842bf70a5488b885` |
| Documentation checkout | `56811d6bdf68238b8a213e38274d8ecad01abf70` |
| Compiler SHA-256 | `bde81baab8818bf95dd417bdca92c3cde8d433e6a7ec13ef04c2e7e2104b4f41` |
| Migration SHA-256 | `d7c92f42b631c5a3f0156bfdfb74067f84c052caee229ac8902ce79a6986293b` |
| Source ZIP | 52,505 bytes; `42bf288a6076b3ae3678e2f79ad98f6e002ae0d8dd299772365d77336ce372f1` |
| Runtime ZIP | 17,499,913 bytes; `879c8f0868c08715e7780bec972fcd44f5392c34c59ce450f221ca0106dca696` |

Both tool identities report the exact source above and `buildState: clean`.
The schema, codec and macro fingerprints match the existing manifest:
`dc6548076b8320b15e062439f0d8e7b032e1fd62d032b19b06ab6c79a54890a5`,
`eb550a69bd2e0413369d6b0c0863448c755724b4103c5e4a3617125a72ee3122`, and
`acb4dd5035ee20786c68a01d323f565e880d2a0b8eedb151145181a4960d455c`.

The component Skill changed only `compilerCommit` to the tested source. It is
not the final published tuple. Its exact local commit is retained in the small
`actual-pair/component-k.bundle` inside the original archive; the prerequisite
is the published implementation commit. This permits reconstructing the
component without rewriting its historical identity.

## Retained evidence and remaining work

[The evidence manifest](evidence/p15-remote-paired-runtime-20261122/manifest.json)
hashes 53 original members in `originals.zip`, including both failed attempts,
the focused test log, publication readback, real build identities, all process
receipts, doctor/wrapper output, source ZIP and component Git bundle. Large
rebuildable tool binaries and the runtime ZIP are identified by their exact
original receipt hashes rather than added to repository history.

The Linux ARM64 Remote Builder still needs fresh staging against its actual
unchanged live guards, the full 39-entrypoint and 58-signing-fault reports,
candidate smoke, and the journaled activation/rollback/reapply cycle. The host
was unavailable through SentinelX during this session. The integrated final
compiler source, D10-or-later docs refresh/publication, final manifest-only K,
and installed consumers remain separate work.
