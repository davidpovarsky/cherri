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

## Edit an existing Shortcut from plist (first-class workflow)

Incoming plist from the user's iOS helper Shortcut:

```text
iOS helper Shortcut shares workflow plist (XML/binary plist or
unsigned .shortcut containing a workflow plist)
        -> Open Minis receives the file at an explicit path
        -> prepare-edit.sh INPUT [WORKSPACE]
        -> preserved original + decompiled .cherri + unsigned validation build
        -> agent reads the .cherri and applies the requested edit
        -> unsigned compile -> fix diagnostics until green
        -> explicit sign -> AEA1 .shortcut
        -> return edited .cherri AND signed .shortcut
```

Steps:

1. Pass the exact received file path to `prepare-edit.sh` (never a fragile
   "latest file in directory" rule):

   ```sh
   sh "$SKILL_DIR/scripts/prepare-edit.sh" /path/to/shared.plist [WORKSPACE_DIR]
   ```

   The helper creates a fresh workspace (default
   `/var/minis/shared/cherri-edits/<safe-name>-<timestamp>/`), copies the
   original untouched into `original/`, decompiles into `source/`, and proves
   the generated Cherri recompiles unsigned into `builds/validation.shortcut`.
   It NEVER signs and NEVER performs the semantic edit itself. Exit codes:
   `0` ready, `1` missing input/no source, `2` usage, `3` signed AEA1 input,
   `4` decompiled source does not recompile (workspace preserved for
   diagnosis — report the error; do not silently edit the source).
2. Read the generated `.cherri` and map the user's request onto it.
3. Search `action.sh`/`catalog.sh` before guessing syntax.
4. Edit only the generated `.cherri`. Prefer typed Cherri actions. Keep
   unknown actions as `rawAction(...)` when the decompiler can preserve them;
   never drop an action merely because it is unsupported.
5. Compile unsigned until the compiler accepts the source:

   ```sh
   sh "$SKILL_DIR/scripts/build.sh" WORKSPACE/source/shortcut.cherri WORKSPACE/builds/validation.shortcut --unsigned
   ```

6. Sign explicitly only after the user's edit is complete and validated:

   ```sh
   sh "$SKILL_DIR/scripts/build.sh" WORKSPACE/source/shortcut.cherri WORKSPACE/builds/final.shortcut --signed
   ```

7. Verify the final file begins `AEA1` (build wrapper checks automatically).
8. Return both the edited `.cherri` and the signed `.shortcut`. Disclose any
   action that could not be preserved exactly.

Input contract:

- Supported: XML plist, binary plist, unsigned `.shortcut` (workflow plist).
  The plist is the source of truth.
- Optional: a JSON companion may accompany the plist as an inspection/debug
  aid or metadata companion; it is never required and never the source of
  truth.
- Not supported directly: signed `AEA1` Shortcut files. `prepare-edit.sh`
  rejects them with a clear early diagnostic; use the extractor Shortcut /
  iCloud-link workflow to provide a plist instead. Never pass AEA1 bytes to
  the plist import path.

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
