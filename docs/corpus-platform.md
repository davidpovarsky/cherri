# Shortcut Corpus Platform

This document describes how Cherri continuously expands from real Apple Shortcuts evidence while keeping one action source of truth and a small, high-signal queue for coding agents.

## Architecture

```text
approved public sources / public iCloud seeds
        -> source adapters + acquisition state
        -> shared iCloud resolver
        -> SHA-256 content dedupe
        -> ignored corpus-inbox/
        -> tools/shortcut-corpus normalization/schema fingerprints
        -> current cherri --actions-json catalog
        -> automatic reclassification
        -> agent-queue.md / agent-queue.json
        -> evidence review -> action definitions in Cherri
        -> fork provenance where required
        -> compiler / decompiler / catalog
        -> docs / Open Minis Skill / iOS palette / preview metadata
```

Cherri core is the only action knowledge base:

- `actions_catalog.go` builds the shared catalog consumed by the CLI, iOS bridge, corpus analyzer, Skill/tooling, docs, and preview metadata.
- `scripts/generate-action-docs.sh` renders docs from Cherri definitions; never hand-maintain a second signature table.
- `internal/icloudshortcut` is shared infrastructure for public iCloud share-link resolution. Cherri import and corpus acquisition call the same implementation.
- The iOS app remains a consumer of Cherri metadata through `CherriActionCatalog`; do not duplicate action semantics in Swift.

## Normal recurring session

Build the current binaries:

```sh
go build -o dist/cherri .
go build -o dist/shortcut-corpus ./tools/shortcut-corpus
```

Run one session:

```sh
dist/shortcut-corpus session \
  -sources routinehub \
  -state corpus-state.json \
  -acquisition-state corpus-acquisition-state.json \
  -inbox corpus-inbox \
  -out analysis/latest \
  -max-actionable 20 \
  -cherri dist/cherri
```

Review in this order:

1. `analysis/latest/summary.md`
2. `analysis/latest/agent-queue.md`
3. normalized JSON for a selected queue item
4. only then the minimum distinct raw plist samples needed to resolve ambiguity

`KNOWN` records are absent from the queue. Third-party actions are excluded by default.

## Acquisition state versus semantic state

The two state files have different responsibilities.

`corpus-acquisition-state.json` records source/item status, canonical iCloud IDs, plist hashes, and minimal provenance. It prevents unchanged public shares from being downloaded repeatedly.

`corpus-state.json` records privacy-safe normalized action schemas, distinct contributing Shortcut hashes, and classification inputs. It never needs source-site descriptions or temporary iCloud asset URLs.

Both are ignored by Git.

## Deduplication

Deduplication is layered:

1. canonical iCloud share ID;
2. downloaded plist SHA-256, so mirrors or different shares with identical content count once;
3. action schema fingerprint across distinct Shortcut contents.

Evidence sample counts represent distinct Shortcut content hashes, not retries, filenames, forced reprocessing, or community mirrors.

## Schema fingerprints and value observations

Schema fingerprints retain action identifier, keys, nested value kinds/field names, and Apple serialization envelopes. Ordinary text/numeric/boolean/date/URL values do not split one action into noisy variants.

Sanitizer-approved structural constants are stored separately as bounded `valueObservations`. They can expose a genuinely new enum/system constant without retaining arbitrary private text.

## Automatic reclassification and closure

Classification is derived from stored normalized evidence plus the **current** Cherri catalog.

Every analysis/reclassification run recomputes verdicts. State records a catalog digest for traceability.

After an agent adds or corrects an action in Cherri:

```text
previous NEW/VARIANT evidence
        -> current --actions-json now models it
        -> reclassify (no raw reparse)
        -> KNOWN
        -> disappears from agent queue
```

This closure check is required before action work is considered complete.

## Source policy

Run `dist/shortcut-corpus sources` for the machine-visible registry.

The live/default acquisition layer is deliberately conservative:

- **seed**: user/agent supplied public iCloud links; deterministic fallback.
- **RoutineHub**: isolated public discovery adapter; do not confuse the authenticated account-owned list endpoint with a public list-all API.
- **ShortcutsBench**: opt-in historical bootstrap, consuming only minimal iCloud/provenance fields from its Apache-2.0 published dataset.
- **ShareShortcuts**: manual only because current terms prohibit automated request/search agents.
- **ShortcutsGallery**: manual only under current restrictive use terms.
- **MacStories**: manual/opt-in rather than a default crawler because its Shortcut archive permissions coexist with general automated-content/data-mining restrictions.
- **Matthew Cassinelli free library**: manual/opt-in; no automation permission is assumed and member-only material is out of scope.
- **GitHub public search**: disabled/future until a deliberately public-only, rate-limited adapter is configured.

Never bypass login, payment, membership, CAPTCHA, robots/access controls, 403 responses, or anti-bot measures. An independently obtained public iCloud link can always be fed through the seed source.

## Public iCloud resolution

Public links use the form:

```text
https://www.icloud.com/shortcuts/<id>
```

The shared resolver isolates the currently observed Apple records flow and immediately consumes the temporary asset URL. Apple does not document the records endpoint as a developer API, so callers must treat it as replaceable infrastructure rather than a stable public contract.

A local signed AEA1 `.shortcut` remains a separate format and is not required for public iCloud corpus collection.

## Manual/local batches

Existing local analysis remains supported:

```sh
dist/shortcut-corpus analyze \
  -in corpus-inbox/manual-batch \
  -state corpus-state.json \
  -out analysis/manual-batch \
  -cherri dist/cherri
```

To refresh old evidence after only Cherri definitions changed:

```sh
dist/shortcut-corpus reclassify \
  -state corpus-state.json \
  -out analysis/latest \
  -cherri dist/cherri
```

## Evidence rules

Corpus observations are `observed`, never automatically `confirmed`. One sample does not prove:

- parameter requiredness/optionality;
- defaults;
- complete enum domains;
- OS availability;
- exclusive input/output types;
- every valid serialization form.

For exact action behavior prefer, in order:

1. canonical Apple-produced plist;
2. multiple consistent distinct real samples;
3. existing verified Cherri implementation/tests;
4. reliable/official documentation;
5. explicit inference followed by testing.

## Promoting a queue item

For each selected unresolved item:

1. compare the queue evidence with the current catalog and closest Cherri definition;
2. inspect only the minimum distinct raw samples needed;
3. implement declaratively under `actions/` whenever possible;
4. use custom compiler/decompiler code only when shared definition machinery cannot express the real serialization;
5. run targeted compiler/decompiler/round-trip tests;
6. update `docs/fork-action-provenance.json` for fork-specific action semantics according to the playbook;
7. rerun `reclassify`/`session` and require the item to disappear as `KNOWN`.

## Round-trip verification

```sh
sh tools/shortcut-corpus/scripts/roundtrip.sh tests/calc.cherri
```

The comparator ignores volatile metadata/UUIDs while comparing meaningful action order, identifiers, parameter structures, serialization, fixed structural values, and variable relationships.

## Privacy

Raw corpus data may include tokens, URLs, emails, phone numbers, paths, clipboard content, names, and other private values.

- Keep raw corpus under ignored `corpus-inbox/`.
- Never commit or upload raw community Shortcuts as CI artifacts.
- Persist only sanitized structural evidence and hashes.
- Never execute corpus Shortcuts.
- Open raw records only after automated dedupe/classification has reduced the problem to a specific queue item.

## CI

`.github/workflows/shortcut-corpus.yml` verifies builds, corpus tooling, the shared iCloud package, compiler/decompiler compatibility, sanitized fixture analysis, reclassification, queue generation, and structural comparison.

CI intentionally does **not** crawl live communities or iCloud. Network-source smoke tests are local/manual evidence only; deterministic tests are the required engineering gate.

## Upstream synchronization

Forks keep `origin` = our fork and `upstream` = original project. Existing upstream-sync and pinning rules in `AGENTS.md` remain authoritative. Corpus acquisition is isolated in new tooling/internal modules so future Cherri upstream synchronization does not require a parallel compiler or action database.
