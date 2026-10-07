---
name: cherri-shortcuts
description: Create, inspect, compile, sign, import, decompile, and repair Apple Shortcuts using the real Cherri compiler and current Cherri documentation. Use for requests to build an Apple Shortcut, generate a signed .shortcut file, translate natural-language Shortcut requirements into Cherri code, search Cherri actions/glyphs/docs, debug Cherri compilation errors, or convert an existing Shortcut/plist/iCloud Shortcut into editable Cherri. Also handles "edit this Shortcut", "modify this existing Shortcut", "here is a plist", "change this action in my Shortcut", "add an action to this Shortcut", and "fix this Shortcut and sign it again" as a first-class existing-Shortcut editing workflow (receive plist -> prepare workspace -> edit Cherri -> validate -> sign). Designed to run in Open Minis/iSH/Alpine or any agent environment with a POSIX shell and network access.
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

## Create a new Shortcut

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

## Edit an existing Shortcut (first-class workflow)

Treat requests like "edit this Shortcut", "modify this existing Shortcut",
"here is a plist", "change this action in my Shortcut", "add an action to
this Shortcut", or "fix this Shortcut and sign it again" as an
existing-Shortcut EDITING workflow — never merely as a file to inspect.

The user's iOS helper Shortcut shares the workflow plist (XML plist, binary
plist, or unsigned `.shortcut` whose contents are a workflow plist). The
plist is the source of truth. A JSON companion, if provided, is an optional
inspection aid only and is never required.

1. Pass the exact input path to `prepare-edit.sh` (an attached/shared file
   path works — do not invent a "latest file in directory" rule):

```sh
sh "$SKILL_DIR/scripts/prepare-edit.sh" INPUT_PLIST_OR_UNSIGNED_SHORTCUT [WORKSPACE_DIR]
```

   `prepare-edit.sh` validates the input, rejects signed `AEA1` input with a
   clear message, preserves the original untouched, decompiles with the real
   decompiler, and immediately proves the generated source recompiles
   unsigned. It never edits the source and never signs. On success it prints
   the original copy, editable `.cherri`, validation `.shortcut`, and
   workspace paths.
2. Read the generated `.cherri` source and understand the user's requested
   modification.
3. Use `action.sh`/`catalog.sh` before guessing syntax.
4. Edit ONLY the generated `.cherri` source. Prefer normal typed Cherri
   actions; preserve unknown/unsupported actions via `rawAction(...)` when
   possible — never delete an unknown action just to make compilation easier.
5. Compile unsigned and fix any diagnostics:

```sh
sh "$SKILL_DIR/scripts/build.sh" WORKSPACE/source/shortcut.cherri WORKSPACE/builds/validation.shortcut --unsigned
```

6. After validation passes, sign explicitly (never sign before the user's
   edit is complete):

```sh
sh "$SKILL_DIR/scripts/build.sh" WORKSPACE/source/shortcut.cherri WORKSPACE/builds/final.shortcut --signed
```

7. Verify the signed result starts with `AEA1` (the wrapper checks this).
8. Return BOTH the edited `.cherri` and the signed `.shortcut`. If anything
   could not be preserved exactly (unsupported action, lossy parameter),
   tell the user explicitly.

Known limits: direct extraction of signed `AEA1` Shortcuts is not supported
yet — use the extractor Shortcut / iCloud-link workflow to provide a plist.
Decompilation is beta: third-party app actions may degrade to `rawAction`
calls, and a plist structure Cherri cannot represent must be reported, not
silently dropped.

## Maintenance

Refresh Cherri source/compiler and upstream docs only when needed:

```sh
sh "$SKILL_DIR/scripts/setup.sh" --update
```

Read `references/workflow.md` for the full decision tree and `references/openminis.md` for Open Minis/iSH paths, installation modes, and limitations.
