# Go 1.27 CLI cache seed repair — 2026-11-25

## Reproduced failure

The ordinary full Go gate at
`2f4a592b4e014459182d1b6569a4e4aee66eb3a6` failed
`TestP15AcceptanceCLIImmutableReuseControls/malformed_build_id_requires_real_link`.
The deliberately coherent cache entry contained `NOT_A_GO_EXECUTABLE`. The real
Go command ran its linker, then refused to copy the executable over the nonempty
non-object destination. This is an acceptance setup defect, not a Cherri program
or native Shortcut result.

The unchanged parent test was reproduced once in an isolated worktree using the
actual Go 1.27.4 linux/arm64 toolchain. It failed only that subcase. The parent is
the narrow valid selector: its malformed-seed child depends on the earlier
source-change child establishing `CLI_REUSE_TWO`.

## Primary toolchain contract and repair

The installed toolchain source at
`/mnt/development/toolchains/go1.27.4/go/src/cmd/go/internal/work/shell.go`
implements `checkDstOverwrite`: an existing nonempty regular destination must be
an object file. Its `src/cmd/go/testdata/script/build_output_overwrite.txt`
explicitly requires refusal and preservation of the original non-object file.
The retained `toolchain-evidence.json` hashes these actual primary sources and
`src/debug/buildinfo/buildinfo.go`.

`p15AcceptanceCLIAtRoot` now stages its build candidate separately from the
caller-visible output. The candidate writer rejects existing paths, including
symlinks, and uses the existing `atomicWriteFile` implementation. Unusable owned
seed bytes are discarded before the build. Parsing Go build information only
answers whether the seed can be a Go executable; it grants no source, role,
freshness or build-action trust.

Every call still executes the original real `go build -o ... .` command under
its unchanged 90-second bound. Go remains responsible for deciding whether a
valid-looking candidate needs a link. Eligible reuse still holds the existing
cache lock, verifies immutable byte digests, binds the actual Go executable and
full environment, checks the resulting source/role identity, and rechecks clean
source/environment after the build. Only then are the resulting bytes published
to a separate private caller output with `atomicWriteFile` and retained in the
immutable cache. Dirty, linked-worktree and unverified sources remain uncached.

No production compiler/action definitions, expected Shortcut outputs, original
reuse assertions, cache keys, identity gates or time limits were relaxed. The
implementation changes test setup code only; it requires no action-provenance or
compiler compatibility fingerprint update.

## Focused evidence

Evidence directory:
`/mnt/development/projects/evidence/cherri-differential-30-20261125/compatibility/cli-cache`.

| Receipt | Actual result | Log SHA256 |
|---|---|---|
| `red.json` / `red.log` | Existing parent failed on the non-object seed; exit 1, no timeout | `393151ba0aead82efa08c3588468bf4a20ce3f395266b45381089ce47a5cf641` |
| `green.json` / `green.log` | Four focused parents, 19 named PASS events; exit 0, no timeout | `70f972c3fcfb9813174cbf496e7cd72b5ceb94791730aa9bf87f557802f016a3` |

The existing `p15_cli_build_reuse_test.go` is byte-identical to the frozen base,
SHA256 `49f1fd227d1f5a5ebfaf86cfc3b29a8873db7b7072db9b62a7e8a28aaf52eda2`.
Its malformed seed still reports `seeded=true`, exactly one observed real linker
command, and actual execution output `CLI_REUSE_TWO`. All eight original reuse
controls passed.

Three new focused parents verify candidate cleanup and existing-file/symlink
preservation; inject a real executable from an older source into a coherent
current cache entry and require one real link plus current program output; and
execute three concurrent consumers with separate private outputs, two validated
immutable seeds, and deliberate private-output mutations. These are owned small
Go programs that test build setup, not native Shortcut acceptance evidence.

The focused GREEN command was:

```sh
go test -count=1 -timeout=180s -run '^TestP15AcceptanceCLI(ImmutableReuseControls|SeedCandidateSafety|StaleRealSeedRequiresLink|ConcurrentPrivateOutputs)$' -v .
```

A separate agent reviewed the cache, actual identity checks, symlink boundary and
publication path without editing or running tests and found no blocking issue.
The checks above ran on Linux/arm64 only. No full suite, signing, Apple native
execution, fresh Windows validation or release acceptance is claimed here.
