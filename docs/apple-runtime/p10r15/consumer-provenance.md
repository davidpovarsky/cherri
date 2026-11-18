# Consumer provenance and installed signing gap

Observation date: 2026-11-18. Scope: source and installed state before promotion. This is not a final release approval. Machine-readable source, installed, and package-discovery records are in `consumer-provenance.json`.

## Findings

The connected Linux service is reachable and its existing doctor succeeds against its own old P10 tuple. Its installed compiler is not the P14/P15 compiler. An isolated audit using that actual installed executable found three false-success classes across the CLI, Skill, and Legacy service wrapper: mismatched advertised digest, a four-byte AEA1 header, and an AEA1 header followed by zero bytes. Each returned exit 0 and an artifact. The existing P14 canonical validator already rejects these cases; rebuilding another validator would duplicate working code. Promotion of a coherent current compiler/docs/Skill tuple remains necessary.

The Windows docs checkout is also stale: its checked-in action-reference fingerprint does not match the compiler. Preview source and lockfile point at the same verified checkout, but this observation is not proof of an installed iOS app or rendered Preview bundle.

| Consumer | Source or declared pin | Observed installed state / scope |
| --- | --- | --- |
| Tested compiler | `c2208b28ab0fdcf7c584d54da01c5f36fb3fd941`, compiler files unchanged from P14 | Real Linux/amd64 cross-build, run under Ubuntu WSL; binary SHA-256 `525b34d189ee57747111c7839093d87da93ffd0603fec01ee0e15a2d0e8077ad` |
| Candidate Skill | `679639d69bd1c4599b4f94cb44964f263416b5d7` | Source diagnostics and wrapper tests pass; existing manifest pins not yet refreshed |
| Docs checkout | `a38b62b819e48fea73dfd0636eb6de4dd08c2713`, clean | Schema `787e9842bbe92b14dbf195bf896c371cb622cd0b6178532b146dc1da6164cc46`; current compiler expects `f55851491e52ec9d06e486420f83d4faef0e31292d411235c01bf104bc48045d` |
| Docs smoke compiler pin | `4f4d30f1b863e8d53cd3d56eb8ddcaffab7ce8f2` | Does not match Skill manifest compiler pin `1204e1f7dfac7af8df0813d1514fac629765af69` |
| Preview | `42ee7656e0cc07b00a097ff4d17d90c8eb8b6efd` | Clean checkout matches package and lock; package version 1.3.4; generated app bundle absent in this isolated worktree |
| iOS bridge/UI | Canonical `schema.DefaultRegistry()`; Swift calls `CherriActionCatalog` and decodes schema fingerprint | Source inspected; installed app not observed in this audit |
| Remote installed compiler | `15f29066e627a1776b8cb17405117e99263171d3` | Linux/aarch64; schema `15eed22550806d167aa399fb7b95b761f540a8e9bf2fbb560cb2dd9112b0ddaa`; binary SHA-256 `14fa042e8d0205981b9ec91b93190a3ef0336dc561bd9bb01a6d871d09099afe` |
| Remote installed docs/Skill | Docs `8daeebc33b55b691493dfe75ff823c24dce44c21`; Skill source `2456327cc941f6ae89c870833d3d5f70109d693c` | Actual `cherri-remote-legacy.sh status` exits 0; Legacy remains default |
| Pure upstream | `bcc0095754e2cc0baaeb4d93dae172b8153b4716` | `pure-cherri-service.sh verify` returned `UPSTREAM_PURE=VERIFIED`; source/docs/binary untouched |
| External Apple Pack | Version 0.1.1; definition source `2456327cc941f6ae89c870833d3d5f70109d693c` | Checksums and one synthetic alias compile verified; explicitly not Apple runtime evidence |

The Remote Builder plugin's current registration/version is not exposed in this tool session. The archived 0.1.4 plugin under an old audit directory is not proof of the registered package version. The active service scripts, binary, docs, and source checkout were inspected directly.

## Actual process and artifact fault matrix

`tools/p10r15/verify_installed_signing.py` serves synthetic faults only on `127.0.0.1`. It runs actual compiler processes and records their exit status, requests, output presence, size, magic, and digest. Copied Skill/Legacy wrappers change only their signing endpoint. The Legacy copy retains the original runner and validator checksums and uses read-only links to the installed runtime. Nothing is sent to an external signer, and no private corpus input is used.

Every negative in the final run observed the multipart request, avoiding a false result from selecting the wrong signing adapter. Digest negatives additionally require the exact `signed_sha256` diagnostic to count as a rejection. Each surface first compiles a real unsigned synthetic source and parses its nonempty native action plist as a positive control. There is no synthetic signed-success or Apple signature-validity claim.

| Fault | Installed old CLI V2 | Installed old CLI Legacy | Installed Skill copy | Installed Legacy wrapper copy | Current source CLI V2 / Legacy / Skill copy |
| --- | --- | --- | --- | --- | --- |
| XML | Reject, no artifact | Reject, no artifact | Reject, no artifact | Reject, no artifact | All reject, no artifact |
| HTML | Reject, no artifact | Reject, no artifact | Reject, no artifact | Reject, no artifact | All reject, no artifact |
| JSON error | Reject, no artifact | Reject, no artifact | Reject, no artifact | Reject, no artifact | All reject, no artifact |
| HTTP 503 | Reject, no artifact | Reject, no artifact | Reject, no artifact | Reject, no artifact | All reject, no artifact |
| Empty response | Reject, no artifact | Reject, no artifact | Reject, no artifact | Reject, no artifact | All reject, no artifact |
| Corrupted JSON | Reject, no artifact | Reject, no artifact | Reject, no artifact | Reject, no artifact | All reject, no artifact |
| Corrupted base64 | Reject, no artifact | Reject, no artifact | Reject, no artifact | Reject, no artifact | All reject, no artifact |
| Advertised digest mismatch | **Exit 0, 32-byte artifact** | **Exit 0, 32-byte artifact** | **Exit 0, 32-byte artifact** | **Exit 0, 32-byte artifact** | All reject with digest diagnostic, no artifact |
| AEA1 header only | **Exit 0, 4-byte artifact** | **Exit 0, 4-byte artifact** | **Exit 0, 4-byte artifact** | **Exit 0, 4-byte artifact** | All reject, no artifact |
| AEA1 with zero body | **Exit 0, 32-byte artifact** | **Exit 0, 32-byte artifact** | **Exit 0, 32-byte artifact** | **Exit 0, 32-byte artifact** | All reject, no artifact |
| Actual HTTP client timeout | Reject, no artifact | Not repeated | Not repeated | Reject, no artifact | V2 rejects, no artifact; other surfaces not repeated |

Installed evidence: `installed-signing-fault-audit.json`, 46 checks: 4 healthy controls, 30 rejected negatives, **12 false successes**. Remote process 801940 exited 1 as expected for the failing audit.

Source evidence: `source-signing-fault-audit.json`, 34 checks: 3 healthy controls, 31 rejected negatives, zero failures. SentinelX `job_551431f85d96` exited 0. The same source audit is added to the existing Skill CI workflow. Current source retains its existing distinction between structural AEA1 envelope checks and actual Apple cryptographic/import/runtime verification; this report supplies only protocol, process, and artifact evidence.

Run locally on a POSIX host with the freshly built compiler:

```sh
python3 tools/p10r15/verify_installed_signing.py \
  --compiler dist/cherri-linux-amd64 \
  --skill skills/cherri-shortcuts \
  --include-timeout \
  --report dist/signing-protocol-exit-evidence.json
```

For the installed host, add `--remote-home /home/chatgpt-agent/workspaces/cherri-remote`, use its actual compiler and source Skill paths, and write the report to a new audit job directory. The final endpoint uses a path marker selecting the canonical multipart adapter; it avoids an embedded equals sign in a CLI argument. Earlier pilot output using a query parameter is not the digest proof used here.

## Live installed mode isolation

A separate unsigned check exercised the actual installed service entrypoints, not wrapper copies. `installed-mode-surface-audit.json` records all four expected outcomes:

| Mode check | Result |
| --- | --- |
| Pure original, `show("P10R15 mode isolation")` | Exit 0; native `is.workflow.actions.showresult` |
| Pure original, pack-only `vibrateAdvanced("Selection")` | Exit 1; undefined action; no artifact |
| Pure + Pack, the same pack-only alias | Exit 0; native `is.workflow.actions.vibrate` |
| Fork Legacy, the basic fixture | Exit 0; native `is.workflow.actions.showresult` |

The real Pure and Pack integrity commands both passed after the runs. Hashes of active config, live wrappers, Pure binary, Pack manifest, and original synthetic inputs remained unchanged. A pilot used a directory containing `basic.cherri`, which correctly hit the Pack's reserved module filename guard; the positive Pack case was then run in a separate directory without that collision. The report retains this fixture correction explicitly. This proves these unsigned CLI routes and one external alias, not complete action coverage or device execution.

## Promotion and rollback sequence

No promotion occurred during this observation. Once the final integrated compiler SHA is frozen:

1. Refresh canonical action docs from that compiler in an isolated docs checkout. Update the docs snippet compiler pin, execute the snippets, commit, and record the immutable docs SHA. Refresh the Skill manifest to this exact compiler/docs/schema/codec tuple and run its real healthy and causal negative checks. These pins must be observed, not copied from an old matrix.
2. Stage the new source Skill and built Linux/aarch64 binary under a new release directory such as `source/p15` and `p15/runtime`. Keep `source/p10`, `p10/runtime`, and the Pure tree intact. Run setup/doctor and the actual loopback fault audit against the candidate paths before changing the active service.
3. Prepare reviewable candidate copies of the existing runner and Legacy entrypoint. The current runner's sync selector knows only P10/P9, and the Legacy entrypoint hardcodes `p10/runtime`, a P10 branch assertion, and the runner checksum. Update those exact routing/pin checks coherently for the new immutable release; merely swapping `active.json` would not promote the Legacy entrypoint correctly. Preserve Legacy as the default language and the separate Pure route. Do not silently fall back to an old tuple on failed validation.
4. Record the candidate runner/Legacy/validator/binary SHA-256 values and expected active tuple. Run status, action/catalog/docs access, unsigned compile, and all supported signing failure paths through the candidate service entrypoints. Real external signing and Apple import/runtime receipts remain separate gates.
5. Immediately before activation, save the current `active.json`, `config/default-language-mode`, runner, and Legacy entrypoint in a timestamped rollback directory. Verify their digests against this observation: active JSON `ee9aac247c568fb743d4e505bcaaebe224bfca843d1091438701bd18273807d0`, runner `7ae689e9d494e7ddb9a7b9abfb5c5d96bb4321b069dc2847aec07271122cb954`, Legacy entrypoint `734507b7a981d7d4185fb3582bd9601d958db013dcd7b12b13201d0d3b0e0b4c`. Reject unexpected concurrent changes.
6. Activate the candidate scripts and tuple together during a controlled service update, then rerun status and a synthetic unsigned compile through the registered route. If these fail, restore the saved scripts/config/active tuple together and verify the prior binary/docs hashes and status. A rollback restores the previous operating state; it does not eliminate the known old signing limitations.
7. Reread installed provenance and attach the after-promotion audit to the existing Asana tasks. Source fixes alone cannot close the installed Remote Builder task.

## Original Authoring Patterns package

The original package was not located. Asana task `1219604109827689` has zero attachments. A bounded Downloads filename search returned 40 Cherri/authoring/INSTALL-AGENT paths but no original package. Central directory checks of the four relevant ZIP files found no Authoring Patterns, INSTALL-AGENT, Skill router, or site generator entries (19, 6, 8, and 7 entries respectively). The docs checkout and a bounded remote Cherri workspace search also found none. This is a package-location blocker, not proof that the package is absent everywhere. Its requested 24 pages, 22 rules, and 3 examples were not reconstructed or represented as installed.
