# Remote Builder promotion and recovery

The promoter installs a reviewed immutable compiler/docs/Skill tuple and verifies
an actual activation, rollback and reapplication. A successful invocation ends
only after two installed smoke runs and an exact old-release status check between
them. Its evidence does not establish Apple cryptographic validation, device
execution or broader project completion.

## Required inputs

Use the exact inactive stage created by `stage_remote_release.py`. Supply both
`--compiler /absolute/cherri` and `--migrator /absolute/cherri-migrate`, built
from the same clean pinned source for the same platform. Staging verifies their
different Go main-package roles, copies both binaries and their commit/hash
sidecars, and binds both binary hashes in the active tuple.

The complete 39-row entrypoint report must have no failed, missing, duplicate or `NOT_RUN`
check. The 58-row signing matrix must contain five unsigned controls, 53
nonzero/no-artifact rejections, all three timeout cases and all 22 exact-preflight
requests across the two Remote Builder routes. The source/runtime/unpacked
integrity receipts must agree with their actual 23/29-file ZIP inventories and
the same S/D/K tuple, compiler hash and migration-tool hash. The real candidate smoke must have eight
successful operations, including both canonical external signing commands.

Create a JSON evidence manifest outside source and live trees:

```json
{
  "formatVersion": 1,
  "tuple": {"label": "p15-REPLACE_WITH_THE_EXACT_STAGED_TUPLE"},
  "files": {
    "stage": {"path": "stage/staging-receipt.json", "sha256": "ACTUAL_SHA256"},
    "entrypoints": {"path": "entrypoints.json", "sha256": "ACTUAL_SHA256"},
    "faults": {"path": "signing-faults.json", "sha256": "ACTUAL_SHA256"},
    "sourceIntegrity": {"path": "source-integrity.json", "sha256": "ACTUAL_SHA256"},
    "runtimeIntegrity": {"path": "runtime-integrity.json", "sha256": "ACTUAL_SHA256"},
    "unpackedIntegrity": {"path": "unpacked-integrity.json", "sha256": "ACTUAL_SHA256"},
    "candidateSmoke": {"path": "candidate-smoke.json", "sha256": "ACTUAL_SHA256"},
    "sourceArchive": {"path": "source.zip", "sha256": "ACTUAL_SHA256"},
    "runtimeArchive": {"path": "runtime.zip", "sha256": "ACTUAL_SHA256"}
  }
}
```

The illustrative tuple above is deliberately incomplete. Copy the **entire exact
ten-field tuple** from the staging receipt: label, compilerCommit, docsCommit,
schemaFingerprint, codecFingerprint, macroFingerprint, compilerSHA256,
migrationSHA256, sourceBranch and skillSourceHead. Paths resolve relative to the manifest. Each
digest must be computed from the actual file bytes. Hash binding and strict
receipt validation preserve reviewed evidence; they are not independent
attestation of arbitrary self-authored JSON.

The historical intermediate archive containing 36 passed entrypoint checks and
one `NOT_RUN` is a deliberate rejection control. It cannot authorize promotion.
Historical compiler-only tuples and 20/23-file package receipts also remain
ineligible for the current paired runtime. The 39 checks consist of 29 distinct
route/integrity checks and ten exact tuple-field corruption checks.

## Execution

Run from the actual Linux Remote Builder account. The live path must be that
account's default `HOME/workspaces/cherri-remote`, so the smoke uses the actual
registered route without a home override. The journal parent must already exist;
the journal itself must be new and outside source, stage and live trees.

```sh
python3 tools/p10r15/promote_remote_release.py cycle \
  --stage /absolute/inactive-stage \
  --live-home /absolute/home/workspaces/cherri-remote \
  --evidence-manifest /absolute/evidence/manifest.json \
  --journal /absolute/evidence/new-promotion-journal
```

The tool takes an exclusive deployment lock, validates current evidence and
materials, and records durable old/new bytes, permissions and ownership. It
retains the exact relative registered link and explicit Legacy preference. It
moves only `source/<label>` and `<label>/runtime` into absent destinations on the
same filesystem. It never renames candidate-home over live-home or moves the
candidate tools link into live. Git clones keep canonical GitHub origins and
must not depend on an alternate object store.

Activation replaces runner, Legacy wrapper, validator and finally active.json.
The validator bytes normally remain unchanged. Each replacement checks the old
state immediately before a same-directory atomic replacement and fsync. New
runner/Legacy validators reject the old tuple during the mixed interval. The
tool then runs all eight real smoke operations through the installed default
V2 and registered Legacy routes.

Rollback restores old active.json first, then old runner, Legacy wrapper and
validator. It verifies exact bytes, modes, ownership, the relative link and both
old status outputs. The new inactive source/runtime directories remain for
reapplication. It then reapplies the same reviewed bytes and runs the eight
installed smoke operations again. Only the complete sequence produces a
`COMPLETED` journal event and successful cycle exit.

On an exception or handled SIGINT/SIGTERM, restoration is attempted from the
already durable snapshots. Logging failure must not prevent that attempt. An
unknown concurrent file edit is not overwritten: the tool reports a rollback
conflict and leaves the journal for investigation. No partial outcome is a
successful promotion.

## Interrupted process recovery

After resolving any disk or journal-writing failure, recover a partial write or
an already completed installation explicitly:

```sh
python3 tools/p10r15/promote_remote_release.py rollback \
  --live-home /absolute/home/workspaces/cherri-remote \
  --journal /absolute/evidence/existing-promotion-journal
```

This accepts only old/new states described by the exact journal. It verifies
snapshot hashes before writing and never takes a force option. Inactive release
directories and historical evidence are retained. An explicit `reapply` command
can reuse the retained release after evidence hashes and the old installed state
are rechecked; a manual reapply does not claim a completed original cycle.

Atomicity is **per file**. This is not an all-files atomic transaction, a
power-loss guarantee or a concurrent-reader barrier. An invocation already using
the old immutable runtime can finish on it. The deployment lock excludes other
cooperating promoters; it does not make unrelated writers obey the protocol.

## Verification scope

`test_remote_promotion.py` exercises real POSIX files, permissions, relative
links, exclusive locks, no-replace renames, atomic replacements and subprocess
status routes. It injects exceptions and SIGTERM at every observed mutation
boundary, hard process exits after each replace in activation/rollback/reapply,
concurrent foreign edits, occupied destinations and journal-write failure. It
also rejects damaged/missing/duplicate/stale evidence and the real historical
incomplete entrypoint report. Synthetic smoke models are confined to unit tests;
they are never retained as production acceptance evidence.

```sh
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover \
  -s tools/p10r15 -p test_remote_promotion.py -v
```

Real candidate/installed smoke, the full entrypoint test and signing fault
matrix remain separate required execution gates.
