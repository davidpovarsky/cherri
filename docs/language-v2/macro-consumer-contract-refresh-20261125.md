# Intermediate macro consumer contract refresh — 25 November 2026

## Source and actual producer

This checkpoint refreshes the compiler repository's generated action reference and Skill compatibility manifest from the published compiler source `12f07521b3e9612b97f4e1fc52ccbee40e3b867a`, tree `7e445fe46da2c26832b5532b3cb344bbb1843c1c`. It is an intermediate source repair. The final compiler/docs/Skill/RemoteBuilder tuple is still pending.

The source-bound adapter was published at `158f6a5df92696fd397e63474b763b69212ea390`; its SHA256 is `74cafb8189958b797dd62057991f16417162e2d4008695a0d058fa6a13135a0f`. The unchanged original compiler build, capability command, documentation generator and full-reference `--check` completed successfully on Oracle/Linux arm64 with the existing Go 1.24.13 toolchain. All seven commands exited zero. The whole supervisor completed in 13.05 seconds and retained 20 original products.

- Producer: `/mnt/development/projects/evidence/cherri-p10r15-macro-fingerprint-refresh-20261125/oracle-generation-02/products/result.original.json`
- Producer SHA256: `05ee9e878079719d7782c26ebffac5dca2609ac47d9c44496f552b701803f3da`
- Supervisor SHA256: `e14d383379f3bc077e527265f7ee7862c67cec3d515174a64806642c5d34a194`
- Independent intake: `/mnt/development/projects/evidence/cherri-p10r15-macro-fingerprint-refresh-20261125/independent-generation-intake-01/result.original.json`
- Intake SHA256: `d04af16382d738e6b8027b99208a9327dc99748cb51d5cc9b715e418af4b880c`

The independent intake rehashed all 20 products, checked all 7,659 tracked source files against their physical SHA256 and Git blob identities, reconciled the unchanged before/after source inventories, and verified `vcs.revision=12f07521b3e9612b97f4e1fc52ccbee40e3b867a` and `vcs.modified=false` in the actual compiler.

The first attempt remains a failure with its original streams. Go 1.24.13's installed VCS discovery requires a directory named `.git`; the linked worktree therefore produced no VCS build fields. The second attempt used an ordinary checkout with the same immutable source and a read-only shared Git object store. No build-identity assertion was relaxed.

## Actual changed contract

The native catalog remains 505 definitions. Its schema and codec fingerprints are unchanged. The compiler macro fingerprint is now:

`747aeae202f541b1a66fad508eb3b151abd604aa4198dc52efab492e1f842629`

The exact 16-name macro inventory is:

`cache`, `debugAssert`, `debugClearLogs`, `debugExport`, `debugExportTrace`, `debugFinish`, `debugLog`, `debugLogPrevious`, `debugShowLogs`, `debugStartTimer`, `debugStopTimer`, `debugTrace`, `debugTraceEvent`, `measure`, `paginate`, `require`.

The Skill manifest changes only `compilerCommit` and `macroFingerprint`. Its former compiler pin was `b59c5b697ec4140ebb0f0eb465c540824a0df3f9`; the new pin is the source actually built above. The generated reference incorporates the existing scalar cache, export metadata options and pagination headers/modes. Its entire 505-action native prefix is byte-identical to the previous reference.

- Actual compiler SHA256: `1afb4301ee277bd647fcf788e058e2683fef68fc6a6a417fbb8ece6d9fdf19fa`
- Actual generated reference SHA256: `a6b53082fa50c224d6bb52ba76429179f4107dc88886ade14f333df492410ef3`
- Applied manifest SHA256: `3a1042a85a5887f4455734a6fc377721892cb3aaa7e475ba40a0894598c80080`

## Remaining source and release verification

The original Linux run `45659698424` provides the RED evidence: strict Skill capability checks found the old macro fingerprint, and D01 still expected 15 macros. The exact inventory test now lists all 16 original names. The later focused qualification below executes D01 and the original local Skill K02–K05 parents on the refreshed checkpoint; the generation receipt alone does not substitute for those tests.

The initial generated source exposed a separate canonical renderer defect: its common scoped-block paragraph said Legacy rejects scoped blocks and described only a value body. Checkpoint `36141680ed3e42529512b73d553c679e5e134466` retained that exact original output, including the defect. The canonical follow-up below corrects the renderer and registry descriptions; no generated Markdown is edited manually.

The manifest's docs pin `428f067090bf54f4d2f214c36951689dd599d153` is retained as the historical D16 component. It is not evidence that the published site has this new contract. The live K18 installation is unchanged. Final documentation publication, consumer upgrade, 39/58 entry/signing gates and rollback/reapply must use the later coherent final tuple. Native runtime, File/Creation Date behavior and full 179 acceptance are outside this producer's evidence.

## Canonical scoped-body documentation follow-up

The canonical renderer and registry descriptions were corrected at `f9019b69937020b72d8d26eff935f71d5eac8e2b`, tree `e8a372a1cd6a2096f6838b3559671ed0ec0c8722`. `ScopedBlock` no longer implies V2-only support. Each macro's canonical description states its own dialect and result contract. `measure` documents Legacy and V2, exactly-once body evaluation in both profiles, its typed value form, its non-consumable Void form, Debug timer emission and the workflow-stop boundary. `cache` documents its supported scalar body and unchanged native acceptance boundary. The generic scoped signature uses a body placeholder.

The original documentation generator ran on this clean source, followed by its original full-reference `--check`; both exited zero (3.93 seconds and 0.30 seconds). The supervisor retained exact command streams and full physical source inventories before and after. No compiler CLI was rebuilt for this editorial-only follow-up. The native and macro fingerprints remain unchanged.

- Original generation receipt: `/mnt/development/projects/evidence/cherri-p10r15-macro-fingerprint-refresh-20261125/renderer-generation-01/result.original.json`
- Receipt SHA256: `f26b709f32c1da1d4a49f60b039f330159fadae5df75265977a708f1d548e570`
- Exact generated reference SHA256: `0ceeff870174737de0d0bad90c3f26a1a09ab8d0bae2454893d132ee8c7acda7` (245,300 bytes)

The entire native-action prefix and the other 14 macro sections are byte-identical to the initial producer output. The Skill manifest remains the exact intermediate proposal derived from the real `12f` compiler; this follow-up does not restamp a final compiler or docs release. D01 now selects the reviewed scoped-body contract parent and preserves all original eight CLI drift faults while adding two dialect/Void drift faults. The focused D01 and local K02–K05 execution below records its own original streams and source guards.

## Original D01 and local Skill qualification

The clean published checkpoint `b2fc920239eda9b4da037755db066c2e7b201b44`, tree `7700ec8d2eb740151f45136028abbf5f12089c3b`, ran the exact 20 selected original D01/K02–K05 parent tests on Oracle/Linux arm64 with Go 1.24.13. The actual Go process exited zero after 266.75 seconds, with **91 named PASS events, zero failures and zero skips**. D01 exercised the canonical 505-native/16-macro reference and all ten CLI drift controls. The local Skill parents exercised pinned installation, cache/bundle selection, update rollback, offline behavior and existing-file/import/signing-policy protection.

The owner receipt remains `FAILED`: its predeclared 69-event inventory omitted 22 existing nested native-graph controls invoked by `TestP15K05BuildPreservesExistingFiles/healthy_unsigned_control`. These are the 21 original causal graph corruptions plus the original consistent-UUID positive. The independent intake derived their exact names from the unchanged Go source, required each of the original 69 and all 22 additional names exactly once in the original JSON stream, and verified all 91 terminal events as PASS. It also rehashed every tracked physical source file and reconciled identical before/after source inventories. No test, fixture, failure receipt or original stream was rewritten, and no Go rerun was used to repair the inventory omission.

- Preserved owner receipt: `/mnt/development/projects/evidence/cherri-p10r15-macro-fingerprint-refresh-20261125/focused-skill-01/result.original.json`
- Owner SHA256: `21d89966376cbf69ee455d26f57c7012e616a95b184211980f435fac204fdfcb`
- Original JSONL SHA256: `a6c10607f0ca9396ea6e0be10e583fd51b7539d0ccc1ad406187e153c0f9b8ee`
- Independent intake: `/mnt/development/projects/evidence/cherri-p10r15-macro-fingerprint-refresh-20261125/focused-skill-independent-intake-01/result.original.json`
- Intake SHA256: `ca534d984e07717a621a04077efd915e854ace826fe69cda1e2dc692016e76c9`
- Source-derived full event inventory SHA256: `291055d6c7624cd94b04b1c9b7d6d843b9acc375695b4d2092c8bf9127057230`

Live K05 signing was deliberately outside this local selection. This qualification grants no Apple runtime, signed shortcut, published-site, final-tuple, full K05 or 179-requirement acceptance credit. The historical D16/K18 installation remains unchanged.
