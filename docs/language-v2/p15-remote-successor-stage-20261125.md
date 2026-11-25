# Immutable RemoteBuilder successor staging

Date: 2026-11-25. Base compiler source: `6dca0432e5887221ff0e4a27c70ed16025b24a8f`.

## Defect and repair

The original stage producer only accepted the pre-K13 live file hashes and
transformed pre-K13 wrapper bodies. It therefore rejected the already promoted
K13 installation before a later release could be staged.

The optional successor path now binds the installed baseline to the exact SHA256
of a retained previous staging receipt. It verifies the complete ten-file
protected/toolchain inventory, typed active tuple, fixed validator, and both
original wrapper-template hashes. The existing wrapper generator is reused.
The initial-installation path retains its original baseline checks.

All four options are required together:

- `--previous-stage-receipt`
- `--previous-stage-sha256`
- `--original-runner`
- `--original-legacy`

Staging remains inactive. All original entrypoint, signature-fault, packaging,
smoke, promotion, rollback and reapply gates are still required.

## Actual component verification

The actual command was:

```sh
python3 -B -m unittest -v test_remote_restage test_remote_source_containment test_remote_promotion
```

All **38 Python test methods passed** in 48.242 seconds (supervised process:
48.413 seconds, exit 0, no timeout). These include 13 new baseline methods,
8 existing containment methods and 17 existing promotion methods. Component
fixtures are synthetic; they do not claim compiler or Apple acceptance.

Original receipt:
`/mnt/development/projects/evidence/cherri-p10r15-consumers-20261125/restage-component-01/process.original.json`.

Original stdout SHA256:
`3127e854d35f08320eed9ec6502865a2da69aadcd2a1bed9fbd9ba4738661c7a`.

Original stderr SHA256:
`e93a429589bdc5fe5a5f92a7a5df59745947dd3a5e160e0f91e55bb540eff470`.

The three code/test file hashes in that receipt matched before and after the
actual run. A one-line addition of the new baseline module to the existing
OpenMinis offline check is retained on the local workflow branch. GitHub rejected
its first push because the current OAuth token lacks workflow scope; the
source-only publication therefore leaves that workflow integration pending.

## Actual installed K13 input replay

The new reader was also executed read-only against the actual installed
`p15-s57b6-d12-k13-20261123` release. All ten protected/toolchain files matched,
the active tuple matched, and all live bytes remained unchanged.

Previous original staging receipt:
`/mnt/development/projects/releases/cherri-p10r15-k13-remote-stage-20261123/staging-receipt.json`.

Pinned receipt SHA256:
`45992838a2450cc1e92059b6f7cde9d32dfb48c77259028e81535eb268e05cb3`.

Original wrapper templates are preserved at
`/mnt/development/projects/evidence/cherri-p10r15-k13-remote-20261123/promotion-cycle-01/before/0`
(runner) and `before/1` (Legacy). Their hashes remain the original fixed
`7ae689e9d494e7ddb9a7b9abfb5c5d96bb4321b069dc2847aec07271122cb954`
and `734507b7a981d7d4185fb3582bd9601d958db013dcd7b12b13201d0d3b0e0b4c`.

Replay receipt:
`/mnt/development/projects/evidence/cherri-p10r15-consumers-20261125/k13-baseline-read-01/replay.original.json`,
SHA256 `042ca946e721cfc7185504b725ac8a1858ac84dfa8ab2c71675518a854f69181`.

This replay did not create or promote a stage. Final compiler/docs pins, new
39/58 qualification, deployment, installed acceptance and 179-requirement
closure remain separate required work.
