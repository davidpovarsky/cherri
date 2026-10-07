# Shortcut Corpus Tooling

The corpus tool is the incremental evidence pipeline used to expand Cherri from real Apple Shortcuts without asking an agent to reread known actions.

```text
approved public sources / manual iCloud seeds
        -> acquisition state + iCloud/content dedupe
        -> public iCloud resolver
        -> ignored corpus-inbox/
        -> privacy-safe schema normalization
        -> current Cherri --actions-json catalog
        -> automatic reclassification
        -> agent-queue.md / agent-queue.json
        -> verified action work
```

Raw Shortcut files, analysis output, semantic state, and acquisition state are local environment artifacts and are ignored by Git.

## Recurring workflow

Build once:

```sh
go build -o dist/cherri .
go build -o dist/shortcut-corpus ./tools/shortcut-corpus
```

Normal future agent session:

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

Then read only:

```text
analysis/latest/summary.md
analysis/latest/agent-queue.md
```

`KNOWN` actions never enter the default agent queue. Third-party actions are also excluded unless `-include-third-party` is explicitly requested.

## Commands

### `session`

One-command recurring acquisition + analysis + reclassification + queue generation.

### `collect`

Collect approved public evidence into the ignored inbox without running semantic analysis:

```sh
dist/shortcut-corpus collect \
  -sources routinehub,seed \
  -seed public-links.txt \
  -inbox corpus-inbox \
  -acquisition-state corpus-acquisition-state.json \
  -max-items 100
```

Use `-dry-run` to perform discovery without iCloud resolution/download.

### `sources`

Print the source registry and automation policy:

```sh
dist/shortcut-corpus sources
```

Sources marked `MANUAL_ONLY` are deliberately not crawled. Supply their independently obtained public iCloud links through the seed source instead.

### `analyze`

Analyze local Shortcut plist/JSON/XML/binary plist inputs recursively:

```sh
dist/shortcut-corpus analyze \
  -in corpus-inbox \
  -state corpus-state.json \
  -out analysis/latest \
  -cherri dist/cherri
```

### `reclassify`

Recompute every stored verdict against the current Cherri catalog **without re-reading raw Shortcut files**:

```sh
dist/shortcut-corpus reclassify \
  -state corpus-state.json \
  -out analysis/latest \
  -cherri dist/cherri
```

This is what makes completed action work disappear automatically from the next queue.

### `compare`

Structural comparison of two Shortcut files:

```sh
dist/shortcut-corpus compare a.shortcut b.shortcut
```

### Round trip

```sh
sh tools/shortcut-corpus/scripts/roundtrip.sh tests/calc.cherri
```

## Sources

Current source policy is intentionally conservative:

| Source | Mode | Default | Notes |
| --- | --- | ---: | --- |
| `seed` | public/manual input | on when requested | Public iCloud links from txt/JSON/JSONL or any text containing the links. |
| `routinehub` | public feed/listing adapter | yes | Discovery is isolated because RoutineHub's public API does not expose a general list-all endpoint. |
| `shortcutsbench` | historical bootstrap dataset | opt-in | Apache-2.0 research corpus; Cherri consumes only minimal iCloud/provenance fields. |
| `github-public` | disabled/future | no | Requires a deliberately public-only code-search adapter and rate-limit handling. |
| MacStories | manual only | no | Shortcut archive is useful; general site terms constrain automated content access/data mining. |
| Matthew Cassinelli free library | manual/opt-in | no | No automation permission is assumed; member-only content is out of scope. |
| ShareShortcuts | manual only | no | Current terms prohibit automated agents/scripts generating automated requests/searches. |
| ShortcutsGallery | manual only | no | Restrictive use terms; use independently obtained iCloud links. |

The source registry is operational metadata only; it is **not** an action database.

## iCloud resolution

Cherri already supported public links such as:

```text
https://www.icloud.com/shortcuts/<id>
```

The implementation is now isolated in `internal/icloudshortcut` and reused by both Cherri import/decompilation and corpus acquisition. It resolves the iCloud records response, immediately consumes the temporary asset URL, and returns the underlying unsigned plist bytes.

The `/shortcuts/api/records/` endpoint is undocumented by Apple. Keeping it behind one resolver makes failures explicit and replacement localized if Apple changes it.

Signed local AEA1 `.shortcut` extraction remains separate and is not required for public iCloud corpus collection.

## Three-level deduplication

1. **Canonical iCloud ID** — the same public share found repeatedly does not download repeatedly.
2. **Shortcut plist SHA-256** — different shares/mirrors containing identical content contribute one content sample.
3. **Action schema fingerprint** — the analyzer collapses repeated action schemas across distinct Shortcuts.

Evidence confidence is based on **distinct Shortcut content hashes**, not retries, filenames, forced runs, or mirrors.

## Schema fingerprints versus values

Schema fingerprinting retains structure such as:

- action identifier;
- parameter keys;
- nested dictionary field names;
- value kinds;
- Apple `WFSerializationType` envelopes;
- reference/App Intent structural forms.

It deliberately ignores ordinary user values such as different text, URLs, dates, booleans, or numbers. Thus `wait(3)` and `wait(5)` are not two action variants merely because the literal changed.

Sanitizer-approved system constants are retained separately as `valueObservations` so a genuine new enum-like Apple constant can still be reviewed.

## Automatic reclassification

Classification is a derived view of:

```text
stored normalized structural evidence + current cherri --actions-json
```

Every run reclassifies existing evidence. State also records a catalog digest for traceability.

If an agent implements a missing action/parameter correctly and the shared Cherri catalog now models it, the old corpus record becomes `KNOWN` on the next run and is removed from the actionable queue without downloading or parsing the source Shortcut again.

State v1 is migrated automatically to v2 schema fingerprints. Old value-specific records that collapse to one schema merge their distinct evidence hashes rather than losing or double-counting observations.

## Agent queue

Outputs:

```text
agent-queue.json
agent-queue.md
```

Queue ranking favors repeated evidence-backed Apple/system additions and variants. Each item contains compact normalized evidence such as:

- identifier/classification;
- distinct sample count;
- parameter keys;
- unmodeled keys versus current Cherri catalog;
- schema fingerprint;
- observed serialization envelopes;
- sanitized structural constants;
- bounded content hashes;
- candidate path when applicable;
- recommended next step.

An agent should inspect this queue before opening any raw plist. Open only the minimum distinct raw samples needed to resolve ambiguity.

## Classification

| State | Meaning |
| --- | --- |
| `KNOWN` | Current Cherri metadata covers the observed parameter surface. |
| `VARIANT` | A known identifier has an unmodeled key/enum-like constant, or an unresolved identifier has another structural schema. |
| `THIRD_PARTY` | App-provided/nonstandard identifier; kept separate by default. |
| `SAFE_CANDIDATE` | New declarative schema suitable for a generated starting candidate. |
| `CUSTOM_IMPLEMENTATION_REQUIRED` | Complex serialization likely needs custom compiler/decompiler work. |
| `UNKNOWN` / `NEEDS_REVIEW` | Evidence is insufficient for a production decision. |

Corpus observations remain `observed`; they never automatically become confirmed semantics. One sample does not prove requiredness, defaults, enum completeness, supported OS versions, or exclusive types.

## Privacy and safety

Raw Shortcut input is untrusted data.

- Never execute downloaded Shortcuts.
- Raw files live only in ignored `corpus-inbox/`.
- Semantic state contains hashes and sanitized structure, not arbitrary user text.
- Emails, phone numbers, URLs, paths, UUIDs, timestamps, high-entropy tokens, and free text are redacted.
- Public iCloud downloads are size-bounded and time-bounded.
- Source adapters do not bypass login, membership, payment, CAPTCHA, 403, robots/access restrictions, or anti-bot mechanisms.
- Temporary iCloud asset URLs are never persisted.

## State files

`corpus-state.json` stores privacy-safe semantic evidence and current classification inputs.

`corpus-acquisition-state.json` separately stores source item/version status, canonical iCloud IDs, plist hashes, and minimal provenance. It does not store raw Shortcut bodies or temporary download URLs.

Both are ignored by Git by default.

## Testing

Normal CI is deterministic and must not depend on live community sites or iCloud. Live source smoke tests, when performed, should be tiny and local-only.

Core checks:

```sh
go build ./...
go vet ./tools/...
go test ./tools/... -v
go test ./internal/icloudshortcut/... -v
go test -run TestCherriNoSign ./...
go test -run TestDecomp ./...
```
