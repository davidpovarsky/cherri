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

Use the exact workflow plist shared by the user's iOS helper Shortcut. Preserve
that original, import it, edit the generated source, compare the unsigned result
against the requested native-field policy, and then sign the verified bytes.

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
   unknown actions as `native.action(...)` when the decompiler can preserve them;
   never drop an action merely because it is unsupported.
5. Compile unsigned until the compiler accepts the source:

   ```sh
   sh "$SKILL_DIR/scripts/build.sh" WORKSPACE/source/shortcut.cherri WORKSPACE/builds/validation.shortcut --unsigned
   ```

6. Record the requested change in `WORKSPACE/allowed-delta.json` using the
   policy in `SKILL.md`. Bind the preserved original SHA-256 and exact existing
   scalar field paths, before values and requested after values. Do not derive
   permission by accepting every observed difference. The current policy does
   not authorize replacing aggregates or inserting/deleting actions.

7. Sign through the required comparison gate:

   ```sh
   sh "$SKILL_DIR/scripts/build.sh" WORKSPACE/source/shortcut.cherri WORKSPACE/builds/final.shortcut --signed \
     --original=WORKSPACE/original/shortcut.plist --allowed-delta=WORKSPACE/allowed-delta.json
   ```

   The wrapper signs the exact unsigned bytes that passed the comparison.
   Unlisted identity, reference, metadata or typed-value changes block signing.
   Pass the preserved original explicitly even when the imported source was
   copied to another directory. Keep the original and previous delivery on failure.

8. Verify the final file begins `AEA1` (the wrapper checks this format).
   Successful Apple import and execution require separate observation.
9. Return both the edited `.cherri` and the signed `.shortcut`. Disclose any
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
- Recommended import modes: use `cherri import <file.plist> --fidelity=hybrid --mode=v2`
  (or `--mode=legacy`) for readable Cherri source with `#workflowheader` provenance
  and fail-closed bit-for-bit plist verification (`native.action` / `rawAction`
  only for unrepresentable actions/regions), or `--fidelity=lossless` for pure
  full-workflow native preservation.

## Action discovery

Prefer this order:

1. `cherri --action=<query>` through `scripts/action.sh`.
2. Local upstream docs through `scripts/search-docs.sh`.
3. Cherri source/action definitions only when the CLI/docs are insufficient.
4. `native.action` only when the action cannot be represented by a supported Cherri definition.

Never invent Apple Shortcuts plist keys from memory when the compiler can generate them.

## Signing

On Open Minis/iSH (and non-macOS platforms), signed output uses Cherri's remote signing integration (Shortcuty Sign / HubSign). Signing sends the generated Shortcut plist to the remote signing service. Do not sign automatically if the user explicitly requests local-only/no-network output; return the unsigned result instead.

A signed result must begin with `AEA1`. `scripts/build.sh --signed` validates this container format. Record the actual signing-service result separately from Apple import and runtime observations.

## Retry discipline

When compile fails:

- Read the complete diagnostic.
- Search the action name and expected include/type.
- Make the smallest source correction.
- Re-run compile.
- Stop after repeated identical failures and report the compiler message plus the source fragment; do not claim success.
