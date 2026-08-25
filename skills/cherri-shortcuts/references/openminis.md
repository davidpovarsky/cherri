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

## Limitations

- macOS native `shortcuts sign` is unavailable; the default signed path uses HubSign.
- The macOS Shortcuts Toolkit SQLite database is not automatically available inside iSH. Decompile uses `--no-toolkit` by default. A user-supplied Toolkit DB can be used manually with Cherri's `--toolkit` option.
- Third-party app actions depend on Cherri definitions, packages, raw actions, or a supplied Toolkit database.
- Network is required for first source setup/docs cloning, package installation, Git packages, iCloud imports, and HubSign signing.
