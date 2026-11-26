---
name: cherri-shortcuts
description: Create, inspect, compile, sign, import, decompile and repair Apple Shortcuts with the real Cherri CLI. Use when building a Shortcut, editing an existing shared plist or unsigned .shortcut, translating requirements into Cherri, discovering actions or glyphs, diagnosing compiler errors, or preparing a signed deliverable. Includes the preserved-original import, edit, compare and sign workflow. Intended for Open Minis, iSH, Alpine or POSIX agent environments.
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

4. Write a `.cherri` source file using Cherri Language v2.0:
   - Use `let` for immutable bindings and `var` for mutable variables. Never use `@variable` or `const` (legacy syntax will be rejected with `E_LEGACY_SYNTAX`).
   - Use `f"Hello {name}"` for string interpolation. Braces in plain strings `"..."` are literal.
   - Use 0-based indexing for lists (`list[0]`).
   - Do NOT use `#include` directives; all actions are in scope.
   - Use named arguments: `resizeImage(photo, width: 800)`.
   - Prefer standard Cherri actions over raw plist payloads. When an undocumented or raw plist action is strictly necessary, use `native.action(identifier: "...", parameters: {...})`.

5. Check syntax and types before building:
```sh
"$CHERRI_BIN" check source.cherri --json
```

6. Compile unsigned first while iterating:
```sh
sh "$SKILL_DIR/scripts/build.sh" source.cherri output.shortcut --unsigned
```

7. If compilation fails, read the compiler diagnostic code (e.g. `E_UNKNOWN_ARGUMENT`, `E_ARGUMENT_TYPE`, `E_MISSING_ARGUMENT`), search the relevant action/docs, edit the source, and retry. Never claim success without a successful compiler exit.
8. For final delivery, sign explicitly through Cherri remote signing (Shortcuty Sign / HubSign) unless the user asks for unsigned output:
```sh
sh "$SKILL_DIR/scripts/build.sh" source.cherri output.shortcut --signed
```

9. Verify a signed result starts with `AEA1`. The build wrapper performs this format check automatically; it does not establish successful Apple import or execution.
10. Return both the `.cherri` source and final `.shortcut` when practical so the workflow remains editable and reproducible.

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

   `prepare-edit.sh` requires a new or empty workspace, rejects signed `AEA1`
   input, preserves the original untouched, imports through the shared V2 hybrid
   route, and proves the generated source recompiles with every native field
   and decoded type preserved. Missing comparison support blocks acceptance. It never edits the source and never signs. On success it prints
   the original copy, editable `.cherri`, validation `.shortcut`, and
   workspace paths.
2. Read the generated `.cherri` source and understand the user's requested
   modification.
3. Use `action.sh`/`catalog.sh` before guessing syntax.
4. Edit ONLY the generated `.cherri` source. Prefer normal typed Cherri
   actions; preserve unknown/unsupported actions via `native.action(...)` when
   possible — never delete an unknown action just to make compilation easier.
5. Compile unsigned and fix any diagnostics:

```sh
sh "$SKILL_DIR/scripts/build.sh" WORKSPACE/source/shortcut.cherri WORKSPACE/builds/validation.shortcut --unsigned
```

6. Record the requested edit in `WORKSPACE/allowed-delta.json` before finalizing.
   Bind `originalSHA256` to the preserved original bytes. List exact native field
   paths and before/after values; never generate an authorization by accepting
   every observed difference. For a Text-field replacement, the policy shape is:

```json
{
  "schemaVersion": 1,
  "originalSHA256": "<SHA-256 of WORKSPACE/original/shortcut.plist>",
  "changes": [{
    "path": ["WFWorkflowActions", "0", "WFWorkflowActionParameters", "WFTextActionText"],
    "before": "Original text",
    "after": "Requested text"
  }]
}
```

   This is a contextual template: use the actual original hash, field path and
   requested values. Version 1 supports existing scalar field replacements.
   Aggregate replacement or action insertion/deletion requires a broader edit
   policy implementation and must not be silently authorized.

7. Finalize through the required comparison gate and existing signing service:

```sh
sh "$SKILL_DIR/scripts/build.sh" WORKSPACE/source/shortcut.cherri WORKSPACE/builds/final.shortcut --signed \
  --original=WORKSPACE/original/shortcut.plist --allowed-delta=WORKSPACE/allowed-delta.json
```

   The wrapper signs the exact verified unsigned bytes and preserves the
   original and previous delivery on failure. Missing comparison support or any
   unlisted metadata, identity, reference or typed-value change blocks signing.
   For a copied imported source, pass the original path explicitly. Without
   an original, signed builds require the canonical compiler to reject any
   native workflow provenance, including through local modules. Changing
   comments or formatting cannot turn an imported edit into a new workflow.

8. Verify the signed result starts with `AEA1` (the wrapper checks this).
9. Return BOTH the edited `.cherri` and the signed `.shortcut`. If anything
   could not be preserved exactly (unsupported action, lossy parameter),
   tell the user explicitly.

Known limits: direct extraction of signed `AEA1` Shortcuts is not supported
yet — use the extractor Shortcut / iCloud-link workflow to provide a plist.
Decompilation supports `--fidelity=lossless` (default for `cherri import`) and
unified `--fidelity=hybrid` (semantic decompilation + `#workflowheader` provenance +
fail-closed whole-workflow verification with per-action/region `native.action` /
`rawAction` fallback; `--fidelity=semantic` remains best-effort); a plist
structure Cherri cannot represent must be reported, not silently dropped.

## Local loop values and mutable aliases

Use the pinned compiler and its Language Guide for nested Lists, Maps, JSON null
and Optional values. Typed local loop sequences support immutable snapshots and
mutable declarations, branch and loop writes, matching mutable RHS values, direct
producers and `+=`. Inference uses actual source types; an empty value alone
does not establish an arbitrary element type. A later assignment does not change
an earlier immutable snapshot.

Use supported typed `count`, checked indexing and `for` consumers. Private
container carriers are an implementation detail, not a public native Dictionary
or a persistence format. Arbitrary native actions, implicit Previous, imports or
function boundaries do not gain conversion permission from a matching spelling.
Keep `none` distinct from present `false`, zero, empty text/collections and
JSON `null`. Do not widen to `AnyContent`, flatten a nested value, or substitute
a sentinel to suppress `E_TARGET_VALUE_SHAPE`.

Read [language and macro boundaries](references/language-boundaries.md) before
combining scoped macros, mutable containers or native output. Successful
compilation establishes a source/native-graph result; signed Apple execution
requires its own evidence.

## Debug logging and native object previews

In the Cherri compiler, `debugLog()` (and `debugLogPrevious()`) captures the
preceding native Shortcuts output without converting it to Text. Its optional
Boolean flags are `withDates`, `showLogs`, and `showLogsAsText`. `withDates`
captures `Creation Date` (`WFFileCreationDate`) adjacent to the native output
producer. Normal `showLogs` previews native objects; `withDates` + `showLogs`
adds `Repeat with Each -> Set Name(Creation Date) -> Quick Look` matching the
verified iPad-exported Shortcuts workflow. Only `showLogsAsText` converts
values for textual display. `showLogs` and `showLogsAsText` are mutually
exclusive.

Use documented V2 named calls such as:
- `debugLog(withDates: true)`
- `debugLog(withDates: true, showLogs: true)`
- `debugLog(withDates: true, showLogsAsText: true)`
- `debugTraceEvent(stage: "after", label: "step-1")` / `debugExportTrace(runId: "qa-run-1")` for opt-in QA structured trace snapshots.

Consult `references/debug-logging.md` and `docs/apple-runtime/cherri-debug/debug-native-object-preview-2026-11-12.md` before choosing flags.

## Maintenance

Refresh Cherri source/compiler and upstream docs only when needed:

```sh
sh "$SKILL_DIR/scripts/setup.sh" --update
```

Read `references/workflow.md` for the full decision tree and `references/openminis.md` for Open Minis/iSH paths, installation modes, and limitations.
