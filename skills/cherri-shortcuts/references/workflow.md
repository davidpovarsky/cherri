# Workflow Reference

## Create a new Shortcut

1. Clarify the concrete behavior and any required input/output surfaces.
2. Run `scripts/action.sh <query>` for each uncertain action family.
3. Search `scripts/search-docs.sh <topic>` for syntax, includes, types, globals, functions, packages, or availability details.
4. Write `.cherri` source in the active workspace.
5. Compile unsigned during iteration with `scripts/build.sh ... --unsigned`.
6. Treat compiler diagnostics as authoritative. Fix source and retry instead of constructing plist manually.
7. Build signed with `scripts/build.sh ... --signed` for final delivery.
8. Preserve the source next to the `.shortcut` whenever possible.

## Edit an existing Shortcut

1. If the input is a plist, unsigned Shortcut, or supported iCloud URL, run `scripts/decompile.sh`.
2. Inspect the generated Cherri before editing; decompilation is beta and unsupported third-party actions may become `rawAction` calls.
3. Make the requested change in Cherri.
4. Recompile unsigned, fix diagnostics, then sign the final result.

## Action discovery

Prefer this order:

1. `cherri --action=<query>` through `scripts/action.sh`.
2. Local upstream docs through `scripts/search-docs.sh`.
3. Cherri source/action definitions only when the CLI/docs are insufficient.
4. `rawAction` only when the action cannot be represented by a supported Cherri definition.

Never invent Apple Shortcuts plist keys from memory when the compiler can generate them.

## Signing

On Open Minis/iSH, signed output uses Cherri's HubSign integration. Signing sends the generated Shortcut plist and name to RoutineHub's remote signing service. Do not sign automatically if the user explicitly requests local-only/no-network output; return the unsigned result instead.

A successful signed Shortcut must begin with `AEA1`. `scripts/build.sh --signed` validates this.

## Retry discipline

When compile fails:

- Read the complete diagnostic.
- Search the action name and expected include/type.
- Make the smallest source correction.
- Re-run compile.
- Stop after repeated identical failures and report the compiler message plus the source fragment; do not claim success.
