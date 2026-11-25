# Owned baseline fixture paths on macOS

The original macOS job 138973248540 in
[Build & Test 45545264693](https://github.com/davidpovarsky/cherri/actions/runs/45545264693)
on `1e02755bbfaf470b4dba1802aa410f29ab4d1188` rejected the positive B01 control
with `B01_SOURCE_REDIRECTED`. The test passed its temporary directory spelling
directly to a producer that deliberately requires the physical source path.
On macOS, the ordinary temporary path can pass through the system
`/var` → `/private/var` alias.

The fixture now resolves its temporary source directory before creating the
test repository and pinning its source locator. The production baseline
implementation is unchanged. A real symlink control verifies that the same
repository supplied through an alias is still rejected. A separate fresh child
test obtains its temporary directory under a selected alias, accepts the resolved
physical source, then requires the original `B01_SOURCE_REDIRECTED` rejection
when that source is presented through the alias again.

Focused qualification on Oracle passed all 25 named tests in 10.09 seconds,
including the original positive and negative baseline cases, both new path
controls, and the dedicated record validation. No test failed or skipped.
This is a real filesystem portability control; the next integrated macOS CI
remains necessary to confirm the actual hosted platform result.

The first new positive control was retained as a failed setup observation:
the parent test had already allocated its cached `t.TempDir` before changing
`TMPDIR`. Moving that control into a fresh child test exercises the intended
alias without changing the production guard or expected result.

Original logs and source hashes are retained at
`/mnt/development/projects/evidence/cherri-p10r15-ci-release-20261125/baseline-darwin-fixture-controls-02/`.
The earlier failed control remains in sibling directory
`baseline-darwin-fixture-controls-01`. Neither these synthetic publication
fixtures nor their successful controls grant project B01 or BRG01 acceptance.
