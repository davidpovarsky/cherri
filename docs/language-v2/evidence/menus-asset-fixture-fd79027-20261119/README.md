# Original menu assets in the isolated migration workspace

Exact repair source: fd790278f40c2b1352eba2821863b33ddea29c06.
Parent and failed CI source: b97ae752205f1e747b6c98ff7511f3484dd53bc2.

## Actual failure

The official S4 Linux job 135353904375 in run 44462962145 failed in the full Go suite. Its terminal output has 23 passing packages, five no-test packages and two failed packages. The root failures are TestP15C03OriginalCLIParity/menus and TestP15MenusMigrationCLIParity, both with E_WORKSPACE_ACCESS: embedded local asset: file does not exist. TestVariadicMigrationProductionEvidence also failed because it independently executes the same C03 original-fixture path.

The complete 1,416,509-byte official log remains outside Git; its SHA256 is c4626d369adca1c6a4a45d6b3832161a9e6945cc59cfd893d957dbcdd9508d4d. Exact job metadata, bounded failure excerpts and the log digest are retained here. This historical CI run remains FAILURE.

An independent Windows execution on the exact clean parent reproduced all three named failure events (including the C03 parent), with four healthy fixture controls passing, zero skips, and unchanged originals in 49.922 seconds. Raw baseline SHA256: 22c9fec819fea13c95c023061ed0c5e3f0f444106cc40dc3e51f5996829ef912.

## Repair boundary

The unchanged menus source references assets/cherri_icon.png. Both old test harnesses wrote migrated source to a fresh temporary V2 workspace but left the asset only in repository CWD. The new held entry-root resource policy correctly rejected that absent dependency.

The repair stages the exact original public PNG bytes at the same relative path in each owned migrated workspace. It changes only three test files. The compiler's resource policy, original source, original PNG, comparison strictness and immutable acceptance contracts are unchanged.

The added actual CLI controls demonstrate:

- With different asset bytes present in process CWD, the migrated entry still uses its own original dependency and retains normalized Legacy parity.
- Removing the entry's dependency produces the precise workspace diagnostic and no output artifact despite the CWD decoy.
- Restoring the exact original bytes recovers native output parity.

## Clean verification

The exact clean fd79027 root gate passed all 11 named tests/subtests in 25.891 seconds, with zero failures, skips or missing required tests. Raw SHA256: ac2f6e3f9d652da11527f8c74e984c79d26a722c2f56ada61dc267ecda625c8a.

The actual independent C03 producer then passed in 18.609 seconds, again with no missing/skipped tests and clean source before/after. Its raw SHA256 is f1af198a099865fc390c6f8dc1ebf2fb89e848444fb611d73bf1e3f5f93dc386. The complete source-bound producer receipt is extracted from concatenated go test JSON Output fragments and retained separately.

All three immutable contracts plus the original source and PNG were hashed before and after verification. All archive members are verified from committed Git blobs. No compiler binary, private corpus input or shortcut artifact is included. These focused results do not claim a successful full S4 suite or native Apple menu interaction.
