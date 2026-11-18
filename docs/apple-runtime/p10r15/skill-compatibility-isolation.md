# Skill compatibility diagnostics: causal verification

Observed 2026-11-18 on Windows with the real Linux/amd64 CLI cross-built from source `c2208b28ab0fdcf7c584d54da01c5f36fb3fd941` (P14 compiler code plus catalog evidence commits) and run on Ubuntu WSL. The Skill changes in this commit do not change the compiler or installed release pins.

## Fix

`validate_docs_dir` previously checked docs commit identity only when `.git` was a directory. Git linked worktrees use a `.git` file, so a stale expected docs SHA could pass. Both layouts now require a resolvable 40-character commit and compare immutable pins. Invalid Git metadata and unavailable Git fail closed. Stable diagnostic codes distinguish stale schema, stale commit, invalid Git metadata, and missing compiler.

The online real-release integration test now requires both a nonzero process exit and the expected diagnostic code for each negative. Merely printing an error while returning success cannot satisfy the test.

## Executed evidence

The new `skills/cherri-shortcuts/scripts/test-compatibility-isolation.sh` ran against a real compiled CLI, canonical generated action documentation, and temporary Git repositories. It does not replace the compiler with a fixture binary. SentinelX job `job_4a9244fa144e` exited 0 and printed `PASS: 9 compatibility isolation checks using real compiler and canonical docs`.

| Check | Observed result |
| --- | --- |
| Healthy compiler and canonical docs | Exit 0 |
| Mutate only docs schema fingerprint | Nonzero; `CHERRI_STALE_DOCS_SCHEMA` |
| Restore docs | Exit 0 |
| Healthy linked docs worktree | Exit 0 |
| Wrong expected SHA, linked worktree | Nonzero; `CHERRI_STALE_DOCS_COMMIT` |
| Wrong expected SHA, ordinary Git clone | Nonzero; `CHERRI_STALE_DOCS_COMMIT` |
| Invalid linked Git metadata | Nonzero; `CHERRI_DOCS_GIT_INVALID` |
| Missing explicit compiler | Nonzero; `CHERRI_COMPILER_NOT_FOUND` |
| Final restored positive control | Exit 0 |

Docs-negative checks additionally reject contamination by the missing-compiler diagnostic. The test is added to the existing Linux Skill CI job.

The existing `self-test.sh --skip-setup` also passed (SentinelX job `job_b132964bd1ba`, exit 0): real action discovery, catalog, unsigned compile, decompile/recompile, prepare/edit, signed-input rejection, and all 10 existing installation/cache/pinning/atomic-update scenarios. Those 10 installation scenarios retain their existing fixture compiler; they are separate from the new nine real-compiler causal checks.

`git diff --check` and POSIX shell syntax checks passed. Remote signing and installed remote Skill updates were not part of these tests. The actual installed consumer tuple is audited separately.

## Reproduce

```sh
CGO_ENABLED=0 go build -o dist/cherri-linux-amd64 .
CHERRI_BIN="$PWD/dist/cherri-linux-amd64" sh skills/cherri-shortcuts/scripts/test-compatibility-isolation.sh
CHERRI_BIN="$PWD/dist/cherri-linux-amd64" SKILL_DIR="$PWD/skills/cherri-shortcuts" sh skills/cherri-shortcuts/scripts/self-test.sh --skip-setup
```

For a Windows-created worktree run through WSL, pass the verified Windows `git rev-parse HEAD` as `CHERRI_REF` to the new isolation test because its `.git` link contains a Windows path. Do not pass that override to the existing fixture installation scenarios: they intentionally create and validate their own immutable compiler refs.
