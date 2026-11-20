# P10R15 candidate S / D6 / K consumer evidence — 2026-11-20

## Scope and immutable identities

This is a candidate consumer checkpoint, not final project, native-runtime, app, or 179-requirement acceptance. The original OpenMinis failure remains a failure.

| Role | Immutable revision |
| --- | --- |
| Compiler S | fd691b96fb661697a1cbaddab92476d457636ba9 |
| Documentation D6 | 35543290124f00f216f6f4b638a890542c8e8783 |
| Prior documentation D5 | 2bc5f6992cd252f287530b039ee393b1b50ee39e |
| Manifest-only K, direct child of S | 939775dcdf9d2582d483a5cf51107724f2f13e9d |
| Actual consumer candidate C | b09f48564adf8d317911a0d552785426a43d4a18 |

C is K plus the documentation-storage repair: two historical evidence Go files were renamed to .go.txt without byte changes. The original 18-member manifest was preserved. Actual go list changed from 31 to 30 packages, removing only the evidence directory. No active compiler, test, workflow, or compatibility-manifest bytes changed between K and C. K and C archives and observations remain separately attributed.

The observed capability tuple is language 2.0, catalog 3.0, protocol 1.0, 505 actions, 15 compiler macros; schema dc6548076b8320b15e062439f0d8e7b032e1fd62d032b19b06ab6c79a54890a5, codec 71e2bc18eca1d32bdf2563d84f5c4bfce66086fd9bef7426481f8f5b8392d6de, and macro acb4dd5035ee20786c68a01d323f565e880d2a0b8eedb151145181a4960d455c.

## Provenance-bearing tools and local documentation

The original linked-worktree compiler lacks embedded VCS identity despite buildvcs=true; the actual inspector rejected it. That observation was not rewritten. A clean ordinary Git source checkout at S produced genuine compiler and migration binaries. Both tool roles, clean embedded S, exact SHA-256, platform, and capability equality were checked. Linux amd64 binaries were also actually executed in WSL. Binary hashes and original build/identity/capability outputs are in the receipt bundle; large executables remain in the owned evidence store.

D6 changes only the canonical generated action reference and the exact smoke-test compiler pin. Generation and --check passed for 505 native actions and 15 macros. Local smoke execution completed 39 cases: 36 real check/unsigned-compile successes and three exact diagnostic/compile-rejection cases. The separate 524 reference-only blocks were not counted as executed. All 12 local provenance/cache controls passed.

## Published D6 and actual installed documentation consumers

Docs CI run [44710687274](https://github.com/davidpovarsky/cherrilang.org/actions/runs/44710687274) succeeded at D6. Independent audit verified all 563 source hashes, the 39 executed cases, the 524 unexecuted reference blocks, 12 provenance controls, six site-corruption controls with exact restoration, and the downloaded rendered site.

After the exact D5 main-parent guard, D6 was fast-forwarded to documentation main without force or policy changes. Main Docs CI 44711839258 and Pages run [44711839370](https://github.com/davidpovarsky/cherrilang.org/actions/runs/44711839370) succeeded. Deployment 7938235610/status 22707764882 are bound to D6. The original 1,538-byte publication evidence ZIP 14617636818 has SHA-256 22f54e80efb982db39db8402451c4e947292f2ab344ebda558e4bbc9f9ffa652. Original Pages archive and live byte receipts are retained; initial home/actions/search GETs matched its exact bytes.

Installed Documentation Consumers run [44714729238](https://github.com/davidpovarsky/cherri/actions/runs/44714729238) succeeded at C, workflow_dispatch attempt 1. Its original artifact 14618374213 is 250,063 bytes, SHA-256 794b94d9b9505d5ababc270a0eca8fe40788e11be70fb1e382c0695f9fdd2126. The independent audit verified every one of its 50 members, 34 referenced artifact hashes, seven immutable source files, eight actual installed search commands, and seven live HTTP bodies against the original Pages artifact. The existing Go intake passed and accepted the D03 docs record. The original receipt deliberately reports no app integration, no D03 completion, and no final closure.

The first independent audit helper expected the generic status PASS instead of the actual PASS_PACKAGE_INTEGRITY. That audit-only assertion failure and helper are preserved; correcting the expectation did not change or rerun any consumer evidence.

## Actual extracted and real-source consumers at C

Source, Windows amd64 runtime, and Linux amd64 runtime archives were each created, verified, and extracted using the real packager. Those integrity observations alone do not imply execution. The subsequent extracted Linux runtime stage completed 20 direct commands, including setup/doctor, genuine D6 checkout verification, both tool capabilities and identities, public check/compile, whole-native-tree lossless import/recompile equality, companion migration preview/check/write, invalid check, and the real wrapper self-test. The wrapper's eight operations and ten controlled transport/minimal-docs controls remain qualified as such.

The unchanged real-release-integration.sh completed with genuine source clones and real builds of both S tools, exact D6, all seven positive/negative/recovery milestones, and owned temporary cleanup in 118.39 seconds. An earlier attempt failed with Git Bash sha256sum's invalid inherited stdin descriptor. A three-way causal probe showed inherited stdin failed, while DEVNULL and an empty pipe hashed the same unchanged binary correctly. Only the launcher stdin changed for the successful replay; product, script, source and 540-second budget stayed unchanged. Both attempts are retained.

The exact seven-parent non-live K05 selection passed 26 named Go events, zero failures/skips, in 161.062 seconds on clean C. It covered preservation, binary import/repair/build, exact signing-input protocol controls, comparator dependency, explicit Legacy import and copied-import policy. This does not establish Apple signature validity, live signing, full K05, or RP37 completion.

## Original OpenMinis failure and remaining work

OpenMinis run [44714722383](https://github.com/davidpovarsky/cherri/actions/runs/44714722383), workflow_dispatch attempt 1 at C, failed in Verify optional compiler macro compatibility. Earlier actual non-signing/decompiler checks, Linux wrappers, isolated drift diagnostics and pinned install/checksum/rollback succeeded.

The original macro report has 21 cases: six passed and 15 failed. The healthy installed control omitted the platform-suffixed migration companion; the capabilities-only proxy lacked mandatory installed-tool receipts and cannot truthfully satisfy the real Go build-identity contract. Old-doc fixtures also predate the strict installed-doc cleanliness contract. This is a concrete harness compatibility gap, not permission to weaken installation identity checks. The subsequent macro, signing, package and ARM64 steps were skipped and receive no credit.

Original job log 136215070807 is 91,389 bytes, SHA-256 6359d2847d4dece57a0124c2a3ecd1397a1b4f6b3a5d17ed2a9168a9c6bceb93. Original artifact 14619042515 is 1,783 bytes, SHA-256 7809b73aec5f87af7d5868ad99e6a714eb08bf2e3730daa002e3f99594841eab. No retry or source mutation is represented by this proof.

Next work is a bounded actual-pair/capability-harness correction, then any specifically authorized fresh consumer run. Root's pending compiler work requires a later explicit coherent tuple refresh. This candidate cannot be inherited as acceptance for that later source.

## Retention and verification

original-receipts.zip contains byte-preserved selected original command/process outputs, source/archive identity reports, the complete original D03 and OpenMinis evidence ZIPs, original remote metadata/logs, local raw Go JSON, initial failures, and independent audits. original-receipts-manifest.json hashes every bundled file. Large reproducible executables, runtime archives, and full built-site archives are intentionally not duplicated into Git; their exact hashes, source identities, original artifact IDs and owned evidence paths remain in the receipts.

External owned evidence root: C:\\Users\\DAVID\\Code\\cherri-p10r15-parity-evidence-20261119\\r4-consumer-refresh-20261120. The source-witness file binds the relevant compiler/Skill/workflow and D6 files to immutable Git blobs using replacement-disabled reads. No acceptance contract, comparator, final gate, or original failure was changed.
