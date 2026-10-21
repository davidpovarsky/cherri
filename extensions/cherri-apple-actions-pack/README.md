# Cherri Apple Actions Pack — opt-in, no compiler patches

This external action-definition pack uses the **unmodified original Cherri compiler** from
[electrikmilk/cherri](https://github.com/electrikmilk/cherri), pinned to
`bcc0095754e2cc0baaeb4d93dae172b8153b4716`. It lives **outside** the pure Git checkout.

## Two operational modes

On the `torah-social` Linux server, use:

```bash
ROOT="$HOME/workspaces/cherri-remote/pure-upstream"
RUN="$ROOT/apple-actions-pack/bin/cherri-with-actions.sh"

"$RUN" status

# 1. Pure original Cherri, no external action definitions:
"$RUN" original compile "$ROOT/jobs/example.cherri"

# 2. The identical original compiler plus this optional action pack:
"$RUN" enhanced compile "$ROOT/jobs/example.cherri"

# Signing is opt-in and uses original Cherri + RoutineHub HubSign:
"$RUN" enhanced sign "$ROOT/jobs/example.cherri"
```

Source files remain unchanged. Enhanced mode creates a temporary copy, copies 20 action
modules, and adds `#include 'apple-actions-pack-index.cherri'` to that temporary copy.
The source checkout, its binary, upstream action files and upstream documentation are
never edited. Running `original` does not load any external modules.

Original official documentation: https://cherrilang.org/language/
Pinned docs source: https://github.com/electrikmilk/cherrilang.org at
`4165cc7224b8f4f0500d95129da9fa14eca5458f`.

## What has been ported

- 20 external Cherri modules generated from P10 action definitions.
- 48 additional Cherri **action** names absent from upstream core.
- 12 new enum definitions from P10.
- 54 `*Advanced` **aliases** for expanded optional parameters of existing actions,
  without overriding or weakening original Cherri action definitions.
- Original builtins and original definitions remain available and untouched.
- Added parameters are only enabled through an alias or an entirely new action declaration.

The independent original compiler produced expected action identifiers and selected
native parameter keys in **204/204** synthetic minimum/full argument test cases
covering all **102** new callable action names (48 added actions + 54 advanced aliases).
All 20 modules and the combined import compiled in **21/21** tests. These are
compiler/plist-level results, not device runtime acceptance.

An additional enhanced-mode regression test confirmed:
- original `run("Shortcut", payload)` includes `WFInput`;
- original `statistic("Sum", items)` includes `WFInput`;
- regex literals such as `[0-9a-fA-F]{8}` are preserved unchanged.

Examples of new external declarations include `getType`, `filterPhotos`,
`addEvent`, `filterEvents`, `getTravelTime`, `scanBarcode` and `intercom`.
Examples of advanced aliases include `quicklookAdvanced`, `getNameAdvanced`.
The upstream original code **already** includes v27 storage actions
`storeValue`, `getStoredValue`, `deleteStoredValue` and Apple Intelligence `askModel`.

## Important limitations

These are NOT 505 end-to-end validated actions. Some P10 capabilities depend on
special Go-only action builders and cannot be moved losslessly as ordinary Cherri
declarations, for example App Intent UUID/descriptor output and advanced predicates.
See `LIMITATIONS.md` and `manifest.json`.

Never claim that generated `.shortcut` was executed successfully on iPadOS unless
an actual iPad execution was independently verified. An AEA1 signature is proof of
signed format, not a test of actions on the device.

## Integrity, audits and versioning

```bash
ROOT="$HOME/workspaces/cherri-remote/pure-upstream"
"$ROOT/apple-actions-pack/bin/cherri-with-actions.sh" verify
"$ROOT/apple-actions-pack/tools/generate_pack.py"
python3 "$ROOT/apple-actions-pack/tools/verify_modules.py"
python3 "$ROOT/apple-actions-pack/tools/verify_entries.py"
```

Note: rerunning `generate_pack.py` changes generated files and may invalidate the
pack checksum manifest until reviewed and regenerated; do not regenerate in production
without a versioned acceptance process. The service requires
`checksums.sha256` for `manifest.json` and every module, and the pure
compiler wrapper independently verifies the pinned official compiler and docs.

Evidence files:
`module-verification.json`, `entry-verification.json`,
`entry-verification-summary.json`, `manifest.json`.

**Do not merge this pack into the pure-original branch.** Distribute it separately.
