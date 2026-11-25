# Original iOS UI failures and focused repairs

Original run: https://github.com/davidpovarsky/cherri/actions/runs/45325137966
Source: 6dca0432e5887221ff0e4a27c70ed16025b24a8f.
The retained original has 59 observed tests and three failures. The xcresult
summary was decrypted on the existing authorized workstation; no private key
was transferred. The encrypted and plaintext archive hashes match the
retained public receipt.

| Test | Original observation | Change |
| --- | --- | --- |
| D03 documentation search | The five-second first-result predicate did not complete. The actual bundled index independently ranks the exact show section first. | Give each navigation row an explicit accessible title and document label; start a new query at its highest-ranked result instead of retaining an older row anchor. Preserve the predicate and its five-second budget; include only Boolean failure details. |
| E07 preview parameter editing | The final immediate source snapshot found no cherriCodeEditor after switching back to Code. Prior edit and invalid-edit checks had passed. | Use the existing five-second source readiness helper for transitions, requiring the entire expected UTF-16 source. Retain the original marker, replacement and comment assertions. |
| RP40 actual app workflow | The exact Rename text-field view identifier was unavailable in the native alert. | Scope to the named Rename Symbol dialog and require exactly one field matching the authored identifier or native New Name placeholder, plus one enabled Rename confirmation. Keep real typing, exact semantic source checks, diagnostic checks and manual Build. |

The RP40 selector does not choose an arbitrary field or inject app state.
The existing E03 case passed in the original run while permitting native alert
controls; RP40 now checks those exact native identities with uniqueness.

The startup decision budget, one-document creation rule, all test methods,
compiler calls, user actions and success checkpoints are retained. No skip or
retry path was added. The protected editor test snapshot requires independent
R23 review before integration acceptance.

## Evidence and scope

Oracle originals:
/mnt/development/projects/evidence/cherri-p10r15-apple-completion-20261125/original-intake/.

- Artifact 14890064131 outer ZIP SHA-256:
  d71bb1ca99c7da8f96bdb70f0e8d0057c576cf2d1f3cd21f507bf1a388fabfe9.
- Encrypted summary SHA-256:
  8ed041ef54026b17d3a2c5751792e27c43eb41c2dd9afd9f5fcbb8a56b9016c1.
- Original private plaintext ZIP SHA-256:
  217b1fbad6be2b1fdc9cb00ba603ef55e4e91a5773ef55c82f7a218e82b0576d.
- Original summary stream SHA-256:
  bb8bdde9ecb329f83a540907312b2bab61a9307661436a123f11f96cbbd2a121.

These changes are a source candidate. A fresh full iOS build and real UI run on
the integrated candidate is required before claiming any repaired native PASS.
