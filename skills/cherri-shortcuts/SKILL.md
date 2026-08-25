---
name: cherri-shortcuts
description: Create, inspect, compile, sign, import, decompile, and repair Apple Shortcuts using the real Cherri compiler and current Cherri documentation. Use for requests to build an Apple Shortcut, generate a signed .shortcut file, translate natural-language Shortcut requirements into Cherri code, search Cherri actions/glyphs/docs, debug Cherri compilation errors, or convert an existing Shortcut/plist/iCloud Shortcut into editable Cherri. Designed to run in Open Minis/iSH/Alpine or any agent environment with a POSIX shell and network access.
---

# Cherri Shortcuts

Use the real Cherri CLI as the source of truth. Do not hand-author Shortcut plist when Cherri can express the requested workflow.

## Bootstrap

Resolve the skill directory, then run setup when Cherri is unavailable:

```sh
SKILL_DIR="${SKILL_DIR:-/var/minis/skills/cherri-shortcuts}"
sh "$SKILL_DIR/scripts/doctor.sh"
```

If doctor reports a missing compiler or docs:

```sh
sh "$SKILL_DIR/scripts/setup.sh"
```

`setup.sh` prefers a bundled Linux/ARM64 Cherri binary when present. Otherwise it installs the minimal Alpine build dependencies, clones Cherri, builds it once, and caches it under `/var/minis/cherri/`.

## Required workflow

1. Translate the user's request into explicit Shortcut behavior: inputs, actions, control flow, outputs, share surfaces, and any app-specific dependencies.
2. Search before guessing action syntax:

```sh
sh "$SKILL_DIR/scripts/action.sh" "search terms"
```

For structured lookups (identifiers, parameter keys, enums, output types),
query the machine-readable action catalog instead of parsing docs by hand:

```sh
sh "$SKILL_DIR/scripts/catalog.sh" 'is.workflow.actions.alert'
```

3. Consult local documentation when syntax, types, includes, imports, signing, or decompilation is uncertain. Documentation comes from the platform docs repository (`CHERRI_DOCS_REPO`, default: the davidpovarsky/cherrilang.org fork; override in the environment if needed):

```sh
sh "$SKILL_DIR/scripts/search-docs.sh" "topic"
```

4. Write a `.cherri` source file in the user's workspace. Prefer standard Cherri actions and packages over raw plist/action payloads.
5. Compile unsigned first while iterating:

```sh
sh "$SKILL_DIR/scripts/build.sh" source.cherri output.shortcut --unsigned
```

6. If compilation fails, read the compiler error, search the relevant action/docs, edit the source, and retry. Never claim success without a successful compiler exit.
7. For final delivery, sign explicitly through Cherri/HubSign unless the user asks for unsigned output:

```sh
sh "$SKILL_DIR/scripts/build.sh" source.cherri output.shortcut --signed
```

8. Verify a signed result starts with `AEA1`. The build wrapper performs this check automatically.
9. Return both the `.cherri` source and final `.shortcut` when practical so the workflow remains editable and reproducible.

## Existing Shortcuts

For a plist/unsigned Shortcut/iCloud import that Cherri can read:

```sh
sh "$SKILL_DIR/scripts/decompile.sh" INPUT [OUTPUT_DIR]
```

Then edit the generated `.cherri`, compile, and sign it normally.

Use `--no-toolkit` behavior on Open Minis/iSH unless the user explicitly provides a macOS Shortcuts Toolkit SQLite database. Third-party app actions that Cherri cannot identify should be preserved as raw actions when possible rather than invented.

## Maintenance

Refresh Cherri source/compiler and upstream docs only when needed:

```sh
sh "$SKILL_DIR/scripts/setup.sh" --update
```

Read `references/workflow.md` for the full decision tree and `references/openminis.md` for Open Minis/iSH paths, installation modes, and limitations.
