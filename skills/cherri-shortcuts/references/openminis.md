# Open Minis / iSH Reference

Open Minis stores imported skill files in the Linux sandbox under:

`/var/minis/skills/cherri-shortcuts/`

Its GitHub skill importer downloads `SKILL.md` plus sibling files recursively, so importing the GitHub URL for this skill installs `scripts/`, `references/`, and metadata together. Archive import also preserves bundled files.

## Install from GitHub

Import the URL pointing to this skill's `SKILL.md` in the Open Minis Skills UI. For the development branch, use the branch URL; after merge, prefer the `main` URL.

Then run:

```sh
sh /var/minis/skills/cherri-shortcuts/scripts/setup.sh
```

## Ready-made ARM64 ZIP

The repository's `OpenMinis Skill` GitHub Action builds a Linux/ARM64 Cherri binary and packages a ready-to-import ZIP that includes it. When that ZIP is imported from Files, `setup.sh` detects the bundled binary and avoids installing Go or compiling Cherri on the device.

Because archive import may not preserve executable mode, invoke skill scripts with `sh ...` and let `setup.sh` apply `chmod +x` to the bundled compiler.

## Persistent state

Default runtime state:

- Compiler/source cache: `/var/minis/cherri/`
- Compiler: `/var/minis/cherri/bin/cherri`
- Upstream docs checkout: `/var/minis/cherri/docs/`

Override with environment variables:

- `CHERRI_HOME`
- `CHERRI_BIN`
- `CHERRI_REPO`
- `CHERRI_REF`
- `CHERRI_DOCS_REPO`
- `CHERRI_DOCS_REF`
- `CHERRI_DOCS_DIR`
- `CHERRI_PREBUILT_URL`

## Editing an existing Shortcut from a shared plist

The user's iOS helper Shortcut extracts and shares a workflow plist (XML
plist, binary plist, or unsigned `.shortcut` containing a workflow plist).
This is a first-class workflow of the skill:

```text
plist received from iOS helper Shortcut
        -> prepare-edit.sh <path> [WORKSPACE]
        -> preserved original + editable .cherri + unsigned validation build
        -> agent edits the .cherri per the user request
        -> unsigned compile (validate)
        -> explicit sign -> AEA1 .shortcut
```

`prepare-edit.sh` lives in `scripts/`. It creates
`/var/minis/shared/cherri-edits/<safe-name>-<timestamp>/` with
`original/`, `source/`, and `builds/` subdirectories, never modifies the
input, rejects signed `AEA1` input with a clear diagnostic (direct signed
extraction is not supported yet — use the extractor Shortcut / iCloud-link
workflow to provide a plist), and proves the decompiled source recompiles
before reporting readiness. It never signs.

Input contract:

- Supported: XML plist, binary plist, unsigned `.shortcut` (workflow plist).
  The plist is the source of truth.
- Optional: a JSON companion is only an inspection/debugging aid or metadata
  companion — never required, never the source of truth.
- Unsupported direct: signed `AEA1` Shortcut wrappers (clear early
  diagnostic; do not feed AEA1 bytes to the plist import).
- iCloud links remain supported through `decompile.sh` for read-only
  inspection; the plist-based edit workflow is preferred for editing.

The agent receives the shared file as an explicit path (attached/shared file
or a file under the Open Minis shared workspace) and passes that exact path
to `prepare-edit.sh`. Do not rely on "latest file in directory" discovery as
the primary behavior.

## Limitations


- macOS native `shortcuts sign` is unavailable; the default signed path uses HubSign.
- The macOS Shortcuts Toolkit SQLite database is not automatically available inside iSH. Decompile uses `--no-toolkit` by default. A user-supplied Toolkit DB can be used manually with Cherri's `--toolkit` option.
- Third-party app actions depend on Cherri definitions, packages, raw actions, or a supplied Toolkit database.
- Network is required for first source setup/docs cloning, package installation, Git packages, iCloud imports, and HubSign signing.
