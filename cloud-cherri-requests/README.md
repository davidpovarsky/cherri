# Cherri Cloud request backend

This isolated GitHub branch supplies the ChatGPT **Cherri Cloud** plugin with a Go-free chat workflow.

## Authoritative sources

- `cloud-cherri-requests/config.json` selects the **live** canonical Skill branch (`skill_ref`).
- The canonical Skill's `skills/cherri-shortcuts/compatibility-manifest.json` pins immutable compiler and docs commits.
- `scripts/cherri-cloud/bootstrap.sh` fetches the live Skill on **every job**, installs and validates the pinned Cherri compiler and docs using the Skill's own setup/doctor scripts.
- No Cherri DSL or action definitions are duplicated in this plugin/backend.

## Request protocol

Create a new branch from `cloud-cherri-requests`:

`cloud-cherri-request/<mode>-<unique-id>` where `mode=compile|sign|simulate`, `unique-id` uses lowercase alphanumerics/hyphens and is at least 6 characters.

Commit exactly one user-approved `cloud-requests/<mode>-<unique-id>/source.cherri` into the new branch.

The push automatically starts [Cherri Cloud - Compile & Simulator](../.github/workflows/cherri-cloud.yml). Each request has its own commit SHA, workflow run and temporary environment.

Artifacts (`cherri-cloud-output`) contain the source, diagnostics, pinned-version metadata and an unsigned Shortcut, plus a signed AEA1 Shortcut and simulator observations where requested. Artifacts expire in 14 days.

## Privacy and external signing

**This repository and request branches are public.** Never commit user secrets, account information, passwords, personal content or private Shortcuts here. Git history can retain uploaded data even after branch deletion. Explicitly obtain the user's informed approval before publication. To support sensitive Shortcuts, move this infrastructure to a private GitHub repository and update its trust checks; do not rely on branch deletion.

Modes `sign` and `simulate` use the canonical Skill's HubSign pathway: the Shortcut plist may be transferred to RoutineHub's remote signing service. Tell the user before signing.

## Simulator limitations

`simulate` tries to import and run the compiled shortcut on an iOS 27 simulator using the repository's existing XCTest ImportHelper. It captures clipboard changes and screenshots. A `triggered_no_observable_output` status is **not** proof that the Shortcut completed or that external app actions worked; it means only that the run URL was opened without an observed clipboard result. Some actions depend on device accounts, permissions or third-party apps absent from CI.

## Updates

To adopt a newer authoritative Skill release **without republishing the ChatGPT plugin**, change `skill_ref` in `config.json` to the verified Skill branch. Each request uses its current pinned `compilerCommit`/`docsCommit` and validates SHA/fingerprints. Backend protocol changes may still require a plugin release.

Do not merge this request backend into `main` without the repository owner's approval.
