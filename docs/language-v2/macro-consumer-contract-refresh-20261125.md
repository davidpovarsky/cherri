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

The original Linux run `45659698424` provides the RED evidence: strict Skill capability checks found the old macro fingerprint, and D01 still expected 15 macros. The exact inventory test now lists all 16 original names. Focused D01 and Skill K02–K05 qualification on the refreshed checkpoint remains to run; this generator result does not substitute for those tests.

The initial generated source exposed a separate canonical renderer defect: its common scoped-block paragraph said Legacy rejects scoped blocks and described only a value body. Checkpoint `36141680ed3e42529512b73d553c679e5e134466` retained that exact original output, including the defect. The canonical follow-up below corrects the renderer and registry descriptions; no generated Markdown is edited manually.

The manifest's docs pin `428f067090bf54f4d2f214c36951689dd599d153` is retained as the historical D16 component. It is not evidence that the published site has this new contract. The live K18 installation is unchanged. Final documentation publication, consumer upgrade, 39/58 entry/signing gates and rollback/reapply must use the later coherent final tuple. Native runtime, File/Creation Date behavior and full 179 acceptance are outside this producer's evidence.

## Canonical scoped-body documentation follow-up

The canonical renderer and registry descriptions were corrected at `f9019b69937020b72d8d26eff935f71d5eac8e2b`, tree `e8a372a1cd6a2096f6838b3559671ed0ec0c8722`. `ScopedBlock` no longer implies V2-only support. Each macro's canonical description states its own dialect and result contract. `measure` documents Legacy and V2, exactly-once body evaluation in both profiles, its typed value form, its non-consumable Void form, Debug timer emission and the workflow-stop boundary. `cache` documents its supported scalar body and unchanged native acceptance boundary. The generic scoped signature uses a body placeholder.

The original documentation generator ran on this clean source, followed by its original full-reference `--check`; both exited zero (3.93 seconds and 0.30 seconds). The supervisor retained exact command streams and full physical source inventories before and after. No compiler CLI was rebuilt for this editorial-only follow-up. The native and macro fingerprints remain unchanged.

- Original generation receipt: `/mnt/development/projects/evidence/cherri-p10r15-macro-fingerprint-refresh-20261125/renderer-generation-01/result.original.json`
- Receipt SHA256: `f26b709f32c1da1d4a49f60b039f330159fadae5df75265977a708f1d548e570`
- Exact generated reference SHA256: `0ceeff870174737de0d0bad90c3f26a1a09ab8d0bae2454893d132ee8c7acda7` (245,300 bytes)

The entire native-action prefix and the other 14 macro sections are byte-identical to the initial producer output. The Skill manifest remains the exact intermediate proposal derived from the real `12f` compiler; this follow-up does not restamp a final compiler or docs release. D01 now selects the reviewed scoped-body contract parent and preserves all original eight CLI drift faults while adding two dialect/Void drift faults. Focused D01 and K02–K05 execution is still pending and will be recorded separately.
