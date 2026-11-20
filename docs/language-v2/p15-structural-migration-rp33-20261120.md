# P10R15 RP33: structural migration with native differential evidence

## Verified result and scope

The two immutable RP33 tiers, `migration` and `native-differential`, pass on clean
source `d233d9f3deb12881126cdd64a753470d70e272f4`. The actual producer runs eight
production test parents and requires all 59 named run/pass events. Its receipt
binds 64 immutable Git source blobs, the exact command, clean state before and
after execution, the original raw Go output and stderr, and finite process timing.
It produces exactly two RP33-only records. The outer producer and corruption
controls passed 251 named events in 51.719 seconds with no failure or skip.

The compiler component had already passed all 19 migration tests, all 81
structural-comparator tests and all 16 parity-runner tests on clean source
`434fda67fb00a2c00f666ab8ad93720724d64e6e`. A freshly built compiler from that
source passed the complete Legacy differential gate: 47/47 program fixtures
(9 exact, 38 normalized) and 17/17 action fixtures, with no upstream, migration,
V2 compilation or comparison failures.

This proof is specific to RP33. It does not establish BRG19, linked CL03/IM02/M01/
M03/R03 requirements, Apple execution, consumer compatibility, successful global
CI, release readiness or closure of all 179 requirements. The final-phase
validator still rejects these local records as a release-completion manifest.
Integration onto another source requires fresh execution; historical receipt
source identities must remain unchanged.

## Defects established before repair

The first clean test-only baseline, `9d980add6ad3294fbf9c08fdfc3e560134f56f15`,
passed the literal-text control and failed seven actual Legacy-to-V2 comparisons.
The failures included Legacy raw backslashes and escape rules, literal condition
text, escaped interpolation, raw interpolation text, directive-like text inside
comments and multiline literals, and a `#ref` used as a native file parameter.
The original implementation performed compatibility rewrites before protecting
lexical regions. A direct `#ref` also reached plist marshaling as an unwrapped
`*ir.RawPlistValue`.

Additional tests on clean `fd6dddec974a5119b80d6b353f9c10abd3c279b5` established
three separate boundary defects: canonical `native.action` parameter text was
rewritten as ordinary Legacy interpolation; a `#ref` sharing a function's name
replaced the function callee; and a Legacy `rawAction` pure text interpolation
was changed from a TextTokenString to an attachment. The repaired cases preserve
their respective existing native representations.

An actual source-directory test on clean
`9d2ad2b25f29948016ddc47763ea4aa5f561100b` put a different `helper.cherri` in the
process working directory. Both migration preview and `--write` selected that
unrelated helper instead of the entry source's helper: the native output changed
from `FUNC:` to `DECOY:`. The test now uses the public CLI from the decoy directory
and proves both modes resolve the original source directory.

One Legacy preprocessing edge is deliberately unsupported. A physical
`#include` line inside a multiline Legacy literal is processed by the old Legacy
preprocessor even though it appears to be literal text. Simply preserving that
line changes the compiled meaning; deleting it structurally would also be an
unsafe conversion. The actual baseline accepted this conversion and `--write`
changed the source. Migration now reports the explicit ambiguous-Legacy-include
diagnostic before writing. Both preview and write tests preserve the original
source and helper bytes. This rejected case is not counted as a successful
native-equal migration.

## Production implementation

`internal/language/migrate/lexical_source.go` classifies active directives,
comments, Legacy literals, canonical literals and JSON strings before the
existing compatibility transformations. Collision-free placeholders keep literal
and comment bytes out of those rewrites. Decoding follows the original Legacy
escape rules, including raw single-quoted strings and unknown escapes; JSON
strings use JSON decoding. Interpolation is applied according to the relevant
language context and literal braces are escaped. Canonical native parameter
syntax and the distinct Legacy `rawAction` text/reference representations retain
their original meaning. A `#ref` payload is consumed as a complete directive, so
Base64 text that happens to contain `//` is not truncated as a comment.

`internal/language/migrate/references.go` removes only validated generated
reference bindings, then substitutes references structurally in the AST. It
respects function parameters, local blocks, loop bindings, imports and setup
bindings. Value references can be inlined without rewriting `CallExpr.Callee`;
the callable namespace remains distinct. The result still passes through the
ordinary V2 parser and formatter and the existing optional/variadic migrations.

`ConvertSemanticValueToParam` unwraps raw plist carriers using the existing
detaching native-value normalization. Direct and nested raw references preserve
their native keys and values; mutating a converted container cannot mutate the
original carrier. This change does not add a serializer or relax authored or
decoded-null validation. Later independent null guards must remain intact when
the component is integrated with newer work.

The path-aware `MigrateSourceWithDiagnosticsAtPath` entry point resolves includes
relative to the actual source file, without changing process working directory.
The main CLI, both migration utilities and the differential runner supply that
path. The in-memory API remains available for callers that have no source path.
Nested includes preserve the entry-root behavior of the original Legacy path.

## Import-question target identity in the comparator

The complete scenario initially exposed a genuine comparator limitation after
the compiler repairs. Its original Legacy document has 42 actions and the V2
document has 41. Their import question indices are 24 and 25, respectively, but
both point at the same unique alert action: title `RP33 setup`, message `RP33`,
parameter key `WFAlertActionMessage`, prompt `Recipient?`, default `RP33`.
Normalization reduces both action arrays to 18 actions. Comparing the unchanged
raw indices after those removals gave a false difference.

`internal/shortcutcompare/question_targets.go` binds a collision-free private
origin marker to each original target before normalization and remaps the index
only if exactly one final top-level action retains that origin. All markers are
removed before comparison. Invalid, removed or ambiguous targets fall back to
comparison of the original raw documents. Index zero remains valid, identical
actions retain distinct origins, and the question's key, prompt and default
remain exact. No emitted ActionIndex or expected fixture was changed.

The original 42/41-action plist files are retained. Twelve causal mutations of
those actual documents change the target, key, prompt or default and must still
fail through the normalizer. The complete comparator suite also checks zero
indices, identical actions, removed targets, invalid indices and prior negative
comparison cases.

## Complete immutable scenario and production tests

The scenario includes actual Legacy metadata (name, glyph, color, inputs and
outputs), an import question, a public synthetic file-location `#ref`, a local
helper include, the documents action include, a helper function, old
interpolation, a Dictionary index, a has-value condition and a Legacy repeat.
The original Legacy program and its migrated V2 output are compiled by a freshly
built production CLI with signing disabled, then compared through the production
native comparator. The source files and original native artifacts are retained.

The eight mandatory production parents are:

- `TestP15RP33StructuralMigrationProgram`
- `TestP15RP33ReferenceLexicalScope`
- `TestP15RP33SyntaxAwareMigrationDifferential`
- `TestP15RP33ReferenceRawParameter`
- `TestP15RP33RawCarrierDetachment`
- `TestP15RP33CanonicalNativePayload`
- `TestP15RP33AmbiguousLegacyIncludeFailsClosed`
- `TestP15RP33SourceRelativeInclude`

Their 59 required events include the eight structural facets, 18 actual native
differential cases, 12 question mutations, direct/nested/opaque raw carriers,
three canonical native parameter forms with independent payload mutations,
lexical scope and callable collisions, and both preview/write safety paths.
The event inventory records direct slash-named subtests exactly as Go emits
them; nonexistent intermediate parent events are not invented.

## Strict producer and causal intake controls

`tools/language-acceptance/structural_migration_execution.go` independently checks
RP33's immutable title, scenario, expected observation and exact two tiers before
executing the production selection. It accepts no linked requirement ID and no
external prebuilt receipt. The two records identify distinct tier witnesses from
the same complete observed scenario execution.

Intake requires the exact source commit and immutable Git blob hashes, command,
required and discovered event inventories, zero exit, no timeout, finite process
interval, and raw/stderr digest and length. Raw output must be valid UTF-8. Each
required event must run and pass; a package or parent pass cannot cover a missing,
skipped or failed child. Additional selected-ancestor checks require every
selected enclosing test to be active when a child starts and every selected
descendant to pass before its parent passes. This guard catches malformed nested
streams that the more general top-level parser intentionally does not model.

Tests remove each of the 59 events in turn, corrupt timing/event order/encoding,
forge process and source fields while reconciling outer hashes, forge all 64
individual source blob digests, and rebind records to related IDs. All such
controls reject. Synthetic streams are used only for parser controls; the
production proof runs the actual compiler tests. The complete 179-row audit
credits RP33 alone from this producer and leaves unrelated rows uncredited.

## Retained executions

The archive preserves original bytes, including CRLF, process receipts, raw Go
JSON, stderr, captured precommit source patches and the original native plist
pair. The reproducible 25 MB compiler binary is excluded from the archive; its
actual build receipt and SHA-256 are retained.

| Stage | Source | Actual result |
|---|---|---|
| Initial native baseline | `9d980add6ad3294fbf9c08fdfc3e560134f56f15` | 1 pass, 7 failing child cases; 26.078 s |
| Question-origin baseline | `f48c6b08367ec56e93b16a4d9d1c746f6a552317` | exact-target and removed/invalid-target controls failed; actual full comparison failed with 12 mutation controls passing |
| Additional structural/native edges | `fd6dddec974a5119b80d6b353f9c10abd3c279b5` | 32 named passes plus new failures; 133.093 s |
| Classified-edge attempt | `bbf5abc3604e25ad74c17bd24c85af92111d00d4` | incomplete outer timeout; one parent started, no completed named test; original attempt retained |
| Safe conversion and source-directory baseline | `9d2ad2b25f29948016ddc47763ea4aa5f561100b` | 13 named passes plus canonical payload, ambiguous include and decoy-directory failures; 48.657 s |
| Final production precommit | exact patch retained, committed as `434fda67…` | migration19 + root59 pass, no failure/skip |
| Clean component and full differential gate | `434fda67fb00a2c00f666ab8ad93720724d64e6e` | migration19, comparator81, runner16 pass; fresh47/47 programs and17/17 actions |
| Strict intake precommit | exact files retained, committed as `d233d9f3…` | 78 named controls pass, 14.610 s |
| Clean actual formal producer and controls | `d233d9f3deb12881126cdd64a753470d70e272f4` | 251 outer passes, 59 required inner events, 64 source hashes, two RP33-only tiers |

Two test-expectation corrections are explicit in the history. The first
structural oracle expected icon metadata to be a RecordExpr although the parser
produces a MapExpr. A new canonical raw-string control also initially assumed
literal braces rather than the existing native parameter symbolic-reference
grammar. Both were corrected against the original production AST/native
witnesses, without changing a production golden. An earlier exploratory
`show(rawReference)` program was invalid for that action and was replaced with
the actual supported `saveFile` folder parameter; its exploratory failure is
also retained separately.

The incomplete classified-edge attempt spent most of its outer 300-second
allowance compiling before its first test began. It is recorded as incomplete,
not passed and not as a completed semantic failure. The subsequent wrapper
allows 540 seconds while retaining the bounded inner Go test deadline. No shared
cache was cleared and no historical failed log was overwritten.

## Exact retained identities

| Artifact | SHA-256 |
|---|---|
| Clean formal outer raw, 310829 bytes | `422225c0c627fe3861c10430d5727c590fa66f1a2dbb3a7db2a36ddade216a58` |
| Actual production child raw, 154875 bytes | `9fe7d8b83e0996a9ba1cea2beb54b8dab5dfdd7d26f1be89b0bc5f27d8855d69` |
| Actual child process receipt | `ccb8672eae5724b344e997b49142bda31eb37fd66e3df6336308d387d6cc09e3` |
| Fresh compiler from clean434 | `adfbe76459a259d6d0119313859ab0003329213cdbd7543c29d18762574112ec` |
| Complete47/47 +17/17 parity report | `03dc7930a6a79776458716933b754de5ad876d9405a449eabdf12f88af51d646` |

The actual child executed from 2026-11-20T03:44:18.3306085Z to
03:44:50.9351278Z in 32.6043824 seconds, exit0, no timeout, with clean source
before and after. Empty stderr has the standard SHA-256
`e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`.

All archive member hashes and the original-to-retained path mapping are in
[the evidence manifest](evidence/rp33-20261120/manifest.json). The complete raw
archive is [rp33-original-receipts.zip](evidence/rp33-20261120/rp33-original-receipts.zip).
The compact source-bound formal receipt is also available as
[production-receipt.json](evidence/rp33-20261120/production-receipt.json).

Asana master `1220635996500993` was updated at each verified stage, including
comments `1220729132717015`, `1220729298289004` and `1220729300161590`.
The master remains open. Independent BRG19 migration/import/edit proof follows;
it must execute its own immutable scenario and cannot inherit this result.

## Independent review

The acceptance agent reviewed the committed producer and controls read-only and
found no concrete fail-open or scope/accounting defect. The review covered the
exact two-tier contract, all59 required events, nested selected-parent ordering,
fresh process/source bindings, immutable Git blobs and reconciled corruption
controls, including linked-ID and global-final rejection. That review did not
execute the tests; the separate source-bound251/59 execution above supplies the
actual result.

The follow-up documentation/provenance change passed both actual registry
validation tests in 43.672 seconds. Its original raw log is retained in the same
archive with SHA-256 acc78e4d5e3e1180f9f5484493e76131656c3b21ce686756f8ff407aab691249.
This metadata validation does not rebind the clean semantic source proof.
