# makeArchive format codec: frozen evidence publication

This copy-ready bundle retains original component and integration evidence.
Preparing this directory ran no tests, generators, builds or native workflows
and changed no source, Git state or existing receipt.

## Source identities

| Scope | Commit |
| --- | --- |
| Component base | `992729d9fbd56bf811e7905916c07f8944e2d1f7` |
| Codec implementation | `8df6923ce0852daa62475b82f141174d8ba7ee26` |
| Separate provenance metadata | `022aa2495efe9c3dcefe94d0ff5b5397bd0840be` |
| Integrated registry gate | `831541058d1a215ecb8cf8fe7315a29e62fb91b9` |

The component Go run used a working tree at the base commit with the exact
implementation and test changes. Its nine recorded source/fixture/metadata
pins match the final component commit; explanatory documentation and provenance
metadata were added afterward. The integrated adoption receipt separately
records matching reviewed source pins and preserved implementation ancestry.

## Recorded outcomes

| Evidence | Recorded result |
| --- | --- |
| [RED receipt](testdata/component/red-01/receipt.original.json) and [complete JSONL](testdata/component/red-01/stdout.original.jsonl) | 16 expected wire mismatches across eight formats and two language modes; Go exit 1. |
| [GREEN-02 receipt](testdata/component/green-02/receipt.original.json) and [complete JSONL](testdata/component/green-02/stdout.original.jsonl) | Six families, 52 subtests, 58 named PASS; Go exit 0; 31.651 s; source unchanged. |
| [Independent source review](testdata/component/independent-review.received.original.json) | PASS by read-only source, pin and receipt review; no replay. |
| [Integrated registry receipt](testdata/integration/archive-format-registry-01/receipt.original.json) and [complete JSONL](testdata/integration/archive-format-registry-01/stdout.original.jsonl) | One named `TestForkActionProvenanceRegistry` PASS on the integrated commit; Go exit 0; 35.634 s; source unchanged; parent reaped and process group absent. |

The 58 component results and the one integrated registry result have distinct
source bindings and test scopes. Original failures, empty streams and
point-in-time status fields remain intact. The original component handoff
predates the completed independent review; the adoption receipt and generation
review retain their historical pending registry state, followed by the
separately recorded integrated PASS.

The six component families cover eight literal mappings, omission/default
behavior, invalid public enum rejection, runtime references and raw/native
values, exact pre-overlay action-graph roundtrips, and preview origin behavior.
These are source and wire-metadata results. They do not establish Apple
execution, repair the observed runtime failure, prove File output, or satisfy
full macro, signed import, physical iPad, full Go or formal 179 acceptance.

## Interruption and privacy

[GREEN-01 transport evidence](testdata/component/green-01/transport-result.original.json)
records an outer 20-second timeout (20.02 s observed), with no JSON test
events or completed Go result. Its original intent, started PID, source
snapshots and empty stdout/stderr are included. PID 1569284 was recorded;
PGID was not separately captured, and no parent-reaped receipt exists for
that launch. Later PID absence is reported in the retained original handoff.

One complete original, `green-01/process-observation.original.json`, is omitted
because it also embeds unrelated account notification-state metadata.
It remains unchanged at its original Oracle location; the manifest records
its exact location, 10,160 bytes and SHA-256. No redacted replacement is
presented as an original. The public bundle therefore does not include the
underlying raw process-observation response.

The selected files use synthetic test inputs. Known host paths, process/job
identifiers and build-cache paths are not credentials. The bounded privacy
review found no credential or private continuity identifier matches in the
selected originals; its scope, candidates and omission are recorded in
[privacy-scan.original.json](privacy-scan.original.json). This is a review of
the selected evidence, not an audit of the host or account.

## Files and integrity

- [Original source handoff](testdata/component/handoff.original.json)
- [Exact implementation patch](testdata/component/implementation.original.patch)
- [Exact final source/provenance patch](testdata/component/final-source.original.patch)
- [Integrated adoption receipt](testdata/integration/archive-format-adoption-01.json)
- [Generation-impact review](testdata/integration/archive-format-generation-impact-01.json)

The generation-impact review compares the merged inputs with canonical
`5bc62d057e4bfa8bdd895e7228ca59eedac20b40` and records why generator replay was
not required. It preserves the earlier generation receipt's own source and
timestamps; no new generation result is claimed.

Every copied original is byte-identical to its retained source.
[manifest.json](manifest.json) binds each content file by bytes and SHA-256.
[publication-handoff.json](publication-handoff.json) pins that manifest and
records package counts. Its own hash is the external handoff anchor.

## Packaging revision 02

All selected originals, including the five Go source snapshots, are stored
under the internal `testdata/` subtree. This packaging layout keeps the source
snapshots out of ordinary recursive Go package discovery. The proposed
repository target is `docs/language-v2/evidence/p15-archive-format-codec-20261127`.

Publication 01 remains unchanged. Revision 02 changes publication paths and
metadata only; every selected original and the bounded privacy receipt remain
byte-identical to revision 01. No Go command was needed for this repackaging.
