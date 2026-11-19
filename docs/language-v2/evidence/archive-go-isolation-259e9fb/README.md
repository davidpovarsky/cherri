# S3 archive package-discovery failure and causal repair

Actual failed source: 8b3fcf71455ea2c1b4c1cf75f929ccc96f227d46 (S3).
Exact verified repair: 259e9fbc05fb01ac032031cc07723678887786da.

The actual iOS workflow 44459426458 failed in Test existing Cherri compiler
before XcodeGen, app compilation, or Simulator testing. Its job is
135342310563. The actual Nix job 135342289682 in Build & Test44459420585
failed its Build nix package step for the same reason.

https://github.com/davidpovarsky/cherri/actions/runs/44459426458
https://github.com/davidpovarsky/cherri/actions/runs/44459420585

Both logs identify the archived observation-only Go test at
docs/language-v2/evidence/required-encoding-7b0100a-44452839062/audit
as a recursively discovered package. That file references root-package
EvidenceRecord, nativeEncodingTestID and validateNativeEncodingRecord
symbols, which do not belong to an independent archive package.

The iOS artifact correctly reports NOT_RUN_OR_INCOMPLETE, null Xcode exit,
zero tests, zero UI/startup/input/native receipts and zero formal records.
Its empty sanitized log and exact three-member 824-byte ZIP are preserved.
The earlier S2 50/50 success is not transferred to this S3 failure.

## Actual causal local verification

A clean S3 checkout reproduced both discovery of this accidental package
and the same undefined-helper build failure (actual exit 1). The repair
renames the observation source to native_encoding_ci_observation_test.go.txt.
All original 2,273 bytes and SHA256
06ea504935dd530b510f98cf6a325f51620bb9667ccbfd1276b0f99e6e944c4e
are preserved. The archive path manifest and README are updated; the original
overlay, phase observations and process receipts remain unchanged.

On the exact clean repaired commit, real recursive package discovery changes
from 30 to 29 packages, removing only that archive package. The command
go test -p=1 -run '^$' -count=1 -json -timeout=90s ./...
exited 0 in 76.484 seconds. Twenty-four test-bearing packages compiled and
five packages had no test files. Zero tests were executed. This is a
compile-only gate, not a full test suite, Nix rebuild or iOS runtime pass.

All 28 members of the prior S2 archive were reverified against its updated
manifest, including the byte-preserved observer. No raw evidence was
normalized or changed. New native/CI execution must use an exact source
that contains this fix; no automatic rerun success is claimed here.
