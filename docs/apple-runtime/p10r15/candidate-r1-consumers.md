# Inactive R1 consumer validation

## Scope

This is an isolated validation package, not the installed release. The compiler is frozen at `d00bc0b9e9a4d8c460623de3d883680b94289e04` and docs at `78d6cc8bc408504a5d9e7abfd1a7120f02a22c4d`. R1 predates the later CLI selection fixes and final evidence gate refresh. The live P10 service, its default Legacy routing, and Pure upstream were not promoted or replaced.

The copied Skill changes only its compiler and docs manifest pins. It has no new immutable release-package commit yet; that distinction is explicit in `candidate-r1-consumer-provenance.json`. The candidate runner's sync command is disabled so its original P10/P9 fallback cannot accidentally activate a different tuple. All other tested entrypoints reuse copies of the existing runner, Legacy wrapper, and unchanged native preflight validator.

Candidate root on torah-social:

`/home/chatgpt-agent/workspaces/cherri-remote/jobs/p10r15-consumer-validation-r1-d00bc0b9/remote-home`

## Immutable inputs and transfer

| Component | Observed evidence |
| --- | --- |
| Compiler source | Clean detached Windows worktree at d00bc0b9; cross-built Linux/arm64 with CGO disabled and existing Go toolchain |
| Binary | SHA-256 `69cf16013de62c85119ee1fbe650af27b488c61317852f3bc49e66319d2a0746`; 15,204,536 bytes |
| Transfer | Existing Sentinel one-file transfer route; digest independently checked on the ARM64 host before execution |
| Source and docs | Both exact SHAs fetched from the real Git remotes into isolated checkouts |
| Schema | `f55851491e52ec9d06e486420f83d4faef0e31292d411235c01bf104bc48045d` |
| Codec | `3e7e3555eefe1e030d254df0a22c62fdc3a139f92e39da34f6d841f8bfa1689e` |
| Macro fingerprint | Not exposed by R1; no macro capability equality is claimed |
| Docs CI | Exact docs head 78d6cc8, [Docs Snippet Smoke Test 44225555435](https://github.com/davidpovarsky/cherrilang.org/actions/runs/44225555435), success |

Remote Go also exists at the runner-scoped path `tools/go/bin/go` (go1.24.13 linux/arm64), although it is absent from the default shell PATH. No toolchain was installed for this validation.

## Actual checks

- Candidate offline setup and doctor passed, validating exact compiler/docs pins and schema/codec.
- Nine causal compatibility checks passed against the actual ARM64 binary, using the test script from its immutable source checkout.
- The copied Skill's wrapper self-test passed, including all ten installation, pinning, cache, offline, and atomic-update scenarios.
- Twelve actual service entrypoint checks passed: runner/Legacy status, runner/Legacy check, runner/Legacy unsigned compile, filtered catalog, independently parsed unfiltered JSON catalog, action help, docs search, disabled-sync guard, and the existing remote Go tests.
- Both unsigned compile routes produced a nonempty independently parsed native plist containing the expected Show Result action.
- Forty-six fault-audit checks passed: four healthy unsigned controls and 42 nonzero/no-artifact rejections through actual CLI V2/Legacy and endpoint-instrumented Skill/Legacy wrapper copies. Every negative observed its multipart loopback request; digest mismatches required the precise digest diagnostic.

The last result closes the old false-success classes **in the inactive candidate**. The live P10 matrix still has the twelve previously documented false successes until a coherent final tuple is promoted and reverified.

Evidence files:

- `candidate-r1-signing-fault-audit.json` — remote process 816831, exit 0, 117.02 seconds.
- `candidate-r1-consumer-surface-audit.json` — remote process 827167, exit 0, 12/12 checks.
- `candidate-r1-consumer-provenance.json` — stage pins, binary and script hashes, manifest overrides, source build and docs CI, before/after live hashes, Skill evidence.
- `candidate-r1-prepare.py` and `candidate-r1-surface-audit.py` — exact one-off staging and audit scripts. They target this inactive R1 snapshot and are not a generic production promotion command.

## Pilot corrections and limits

The first compatibility-isolation invocation used the standalone copied Skill, whose test helper expects the repository's canonical docs reference. It failed because that repository-relative fixture was absent; rerunning the unchanged source-checkout test helper against the same candidate binary passed all nine checks.

The first surface harness incorrectly treated the filtered catalog command as a complete JSON document. The existing wrapper deliberately emits matching grep fragments. The corrected audit checks that filtered route and separately parses the unfiltered catalog; it passed 12/12. The initial failed pilot report remains in the remote audit directory.

No synthetic signed-success, Apple cryptographic validation, Apple import/runtime, installed iOS app, or physical iPad result is claimed. These tests use synthetic source and a loopback signer. The pre-promotion and post-audit hashes of the live active config, runner, Legacy wrapper, and native validator are identical.

## Next tuple

Repeat staging with the final compiler source, regenerated/pushed docs, and a committed Skill manifest. Validate its separate macro fingerprint when declared, then run the same causal and service checks. Refresh the candidate runner's immutable sync source route and Legacy pin/checksum checks for the final package. Promotion and rollback must still update the runner, Legacy wrapper, manifest/runtime, and active tuple coherently; swapping only active.json is insufficient.
