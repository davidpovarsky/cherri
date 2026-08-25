# Shortcut Corpus Tooling

Incremental analysis pipeline that turns batches of real Apple Shortcuts
files (JSON/plist) into privacy-safe structural evidence, compares it against
Cherri's machine-readable action catalog, and produces concise reports for
agent/human review.

Raw Shortcut files are **never committed**. Everything this tool stores or
prints is sanitized structural data.

## Usage

```sh
go build -o shortcut-corpus ./tools/shortcut-corpus

# Analyze a batch (files, directories, recursive)
./shortcut-corpus analyze \
  -in inbox/first-batch -in more.shortcut \
  -out analysis/2026-08-25 \
  -state corpus-state.json \
  -cherri ./cherri

# Structural comparison of two Shortcut files (plist XML/binary/JSON)
./shortcut-corpus compare a.shortcut b.shortcut

# Full compile -> decompile -> recompile -> compare loop
sh tools/shortcut-corpus/scripts/roundtrip.sh tests/calc.cherri
```

`-catalog` accepts a pre-generated `cherri --actions-json` file and overrides
`-cherri`. Without either, the analyzer runs `cherri --actions-json`
(`$CHERRI_BIN`, then `cherri` on PATH).

## Pipeline

1. **Discover** inputs (files/directories, recursive; hidden dirs skipped).
2. **Hash** each file with SHA-256 and skip hashes already in the state file.
   Deduplication is content-based; filenames are irrelevant.
3. **Parse** plist XML/binary/JSON documents generically.
4. **Normalize** every action: identifier, parameter keys, structural value
   shapes, serialization types, variable/output references.
5. **Sanitize** all values (see below).
6. **Fingerprint** each action shape (identifier + keys + structure +
   system constants). Identical shapes increment evidence counts instead of
   creating duplicates.
7. **Classify** against the Cherri catalog.
8. **Report**, then emit candidate definitions for safe new shapes.

## Classification

| State | Meaning |
| --- | --- |
| KNOWN | Identifier is in the Cherri catalog and the observed shape conforms to it |
| NEW | Identifier not in the catalog, first structural form |
| VARIANT | Additional structural form of an already-seen identifier, or shape mismatch with the catalog |
| THIRD_PARTY | Non-`is.workflow` identifier (app intents, third-party apps) |
| UNKNOWN / NEEDS_REVIEW | Standard-library identifier absent from the catalog |
| SAFE_CANDIDATE | Declarative NEW shape suitable for a generated candidate |
| CUSTOM_IMPLEMENTATION_REQUIRED | Shape needs custom parameter construction/decompiler handling |

## Evidence model

- `observed` — seen in real corpus data (this tool). Confidence scales with
  sample count but never implies confirmed API behavior.
- `inferred` — analyzer heuristics (e.g. candidate signatures).
- `confirmed` — only facts published by Cherri's catalog.

A single sample never establishes requiredness, defaults, enum completeness,
OS availability, or exclusive types.

## Privacy

Sanitization replaces personal content with typed placeholders:

- emails, phone numbers, URLs, file paths, UUIDs, timestamps, high-entropy
  tokens, and free text are redacted;
- system constants survive: `WF*` serialization types, dotted bundle/action
  identifiers (`is.workflow.*`, `com.*`), and a small curated control-value set;
- numbers/booleans are kept because they are structural (control-flow modes,
  colors), and raw corpus files stay out of Git by default (`corpus-inbox/`,
  `analysis/`, state files).

## Reports

Written to `-out`: `summary.json`, `known-actions.json`, `new-actions.json`,
`variants.json`, `third-party-actions.json`, `needs-review.json`,
`summary.md`, and `candidates/` (non-production `.cherri` + `.json` pairs).

Review order for future batches: read `summary.md`, inspect
`needs-review.json`, open raw files only for NEW/VARIANT records.

## State

`-state` persists file hashes, action fingerprints, evidence counters, and
emitted-candidate markers as JSON. Delete it to force full re-analysis, or
pass `-force`.
