# macOS cumulative budget: two isolated schema parents

## Actual failure retained

Frozen K11 `9b4135e705b08f765b7304e01212d3b9658806fb`, Build & Test run45108200176, macOS job137474256647, failed its original root-package15-minute cumulative limit. The root package reported900.153s. Its only unfinished parent was TestP15CanonicalBehaviorSourcePropagation (59s), then /codec (2s); no individual assertion failure or expired child deadline was demonstrated.

The original log SHA256 is `3938ab765044742110bee1e509162401902a5cfbcd8d4a9ab9ab5ed2d9e8c1c3`. Full-suite artifact14790748782 SHA256 is `2f6ba0b9333ca9f7c1f2be4f3fd07b2986d511aa7d27dbea8164011b8f0cc48f`. Its source-clean original receipt records ordinary exit1/918.709s and acceptance exit0/1711.823s. No result is restamped as a later-source PASS.

BOM-before-timestamp-aware original counts are16,665 named PASS and14 SKIP across the two invocations, with no named FAIL event; the root package still failed by timeout. All1,180 root parents started,1,169 passed,10 skipped and one remained incomplete. All29 opted-in parents continued;28 completed. Both K04 parents actually used the scheduler and passed. Separate log-delivery, wall and monotonic clocks remain qualified in the retained intake.

## Minimal change

Add only the existing p15FullSuiteSubprocessParallel(t) first-line call to:

- TestP15RP08SchemaIdentityProduction in p15_schema_identity_recovery_test.go, originally30.62s serial.
- TestP15SchemaOptionalDeclarationPropagation in p15_schema_optional_propagation_test.go, originally21.07s serial.

Removing these exact calls restores original full-file SHA256 values:

- RP08: `cde58a3a835b9cdd541124d58e4e080183e39f0b8e8b481996615f2aab3ca3ed`.
- Optional declaration: `cc10f2450fc03097c68071599f1ffcb445ffbde4d1e7d48054741b241aae551a`.

The reviewed two-line patch SHA256 is `27dc425812eafcf8db88107c04a0ac6c84744ab4097e924296d56424b829808f`. No original body, subcase, mutation, assertion, compiler input, child command or deadline changes.

RP08 owns a copied temporary replica and private binaries/outputs. Optional owns a unique detached worktree and index; cleanup removes only that worktree and checks original HEAD/status. Mutations and subcases remain sequential. Neither parent mutates process-global environment/CWD or the shared installed-tool byte map. Both share standard Go cache/module artifacts through existing child processes.

The existing helper still requires marker2, an empty selected-run expression, a top-level parent and -parallel=2. There are31 opted-in parents under the same two root-package slots. This is not a global OS-process cap. Selected acceptance producers remain serial; original900-second package and120-second command-context limits remain. Existing direct-child cleanup semantics are not expanded or relabeled.

## Validation and remaining qualification

Root and an independent agent reviewed the complete dependency/isolation chains and verified exact removal-to-original bytes. git apply --check and the staged whitespace check passed.

No selected parent or complete Go suite was replayed locally. A selected -run intentionally disables this scheduling path and would not demonstrate concurrency benefit. The next actual complete macOS CI, retaining marker2, empty -run, -parallel2 and15m, is the required runtime qualification. No successful performance outcome is claimed.

Canonical's seven actual baseline/mutation/restoration probes remain intact. R23 policy remains unchanged and its separate proposal remains inactive.

Original macOS intake, source hashes, scheduler events and timeout stack:
`/mnt/development/projects/evidence/cherri-p10r15-k11-frozen-ci-20261123/macos-terminal-intake-01`,
manifest SHA256 `fab644eba0c463fd6edc1db94896945588efa784597791d021e6cbe6eeeb69fc`.
Inactive review history:
`/mnt/development/projects/evidence/cherri-p10r15-macos-schema-scheduling-20261123`.
