# P10R15 BRG19: safe migration, import and an authorized native edit

## Verified result and limits

BRG19's immutable integration and native-roundtrip tiers pass on clean source
af9ad2ebe96f48eca06dd3692b83f9b08bb2edcf. The actual producer executes the complete
scenario independently of RP33 and requires all 53 named run/pass events from
two root tests. Its two BRG19-only records bind 68 immutable Git source blobs,
the exact command, clean source before and after, finite process timing, and
the original raw output and stderr. The outer producer, parser, source and
corruption controls passed 258 named events with zero failures or skips in
42.735 seconds.

The separate component run on clean
2f5424b431904b0da71ac4d34407d717425757cf had already passed all 53 named events in
26.094 seconds. No additional compiler, migration, import, serializer or edit
authorization production repair was needed for this scenario on the completed
RP33 base. The new implementation is the independent test scenario and its
strict acceptance producer.

This establishes BRG19 alone. R07, CL03, RP33, RP34, RP37 and BRG20 cannot inherit
these records. The complete 179-row audit credits only BRG19 from this producer;
other requirements stay uncredited. Apple execution, global successful CI, the
consumer/release tuple and project closure remain separate. Final-phase release
validation still rejects the local-only manifest. Replaying on an integrated
commit must produce fresh evidence instead of changing these historical source
identities.

## Immutable scenario and actual public path

The contract requires: migrate Legacy metadata, references and includes; fail an
unsupported conversion; then perform one authorized parameter edit on imported
native data. Originals must survive the failed conversion, the intended delta
must be the only change, structural conversion must not delete directives by
regex, and public output must be V2 without a Legacy retry.

The new public synthetic program contains:

- Name, glyph, color, input and output metadata.
- A public file-location reference used as the saveFile folder.
- An actual local helper include that emits a distinguishable comment.
- The documents action include, an ordinary value, a save operation and a
  separate editable comment.
- A directive-like color line inside a block comment.

Each complete scenario invocation builds or uses the current production CLI,
compiles the original through the explicitly selected Legacy frontend, previews
the public migrate command, parses its output as canonical V2, and verifies the
metadata/reference/include AST facets. It then compiles the migrated file through
the default public V2 command and compares the actual native documents using the
production differential comparator.

The default compile command is also given the original Legacy file. It must
return E_LEGACY_SYNTAX and create no Shortcut; this observes the absence of a
silent Legacy retry.

For failure preservation, a copy of the full program receives an unsupported
directive. Both preview and --write must fail with the migration diagnostic and
no stdout conversion. Hashes of the entire isolated directory before and after
each command must be identical, covering source, include and prior native
outputs as well as detecting newly leaked temporary files. A subsequent healthy
migration must reproduce the original successful converted bytes.

## Actual import and exact edit boundary

The migrated native document is augmented only in the disposable fixture with
an opaque public metadata dictionary containing Data bytes, a large integer
9007199254740993, a Date, false, an ordinary UUID string and an ordinary group
identifier. This fixture is encoded both as XML and binary plist in separate
full scenario executions.

The public CLI imports each native input using hybrid fidelity and the default
V2 mode. Its actual import plan must classify the intended comment action as
NATURAL_VERIFIED. The resulting source must parse as V2. Recompiling that unedited
source must preserve the complete native tree exactly, including all decoded
types, opaque data, action order, identifiers and references. Parity normalization
cannot authorize a native edit.

The test finds the unique comment whose text is BRG19_BEFORE, then locates the
matching natural call in the parsed imported AST. Only that literal's exact
parser-observed byte span is replaced with BRG19_AFTER. It does not perform a
global string replacement, so an embedded exact native header or provenance
payload cannot be edited accidentally.

The edited source is compiled through the default public V2 CLI. An independently
constructed expected document starts with a fresh decode of the original native
bytes and changes exactly one existing WFCommentActionText scalar. The complete
candidate must equal that expected tree.

The actual production VerifyEditFiles verifier is then called with a policy
bound to the SHA-256 of the original native file and precisely one existing path,
before value and after value. It must accept exactly one change. All original
source, include, native input, migrated output and imported source files are
checked again for byte preservation after verification.

The observed target index was 3 in each of the three actual scenario executions:
one integration execution and the XML/binary native-roundtrip executions. Each
has its own retained native files and hashes; no bytes or UUIDs are transplanted
between independent runs.

## Causal controls

The integration test independently corrupts migrated metadata, removes the
included helper action, and replaces the original reference. Each corruption
must fail the structural scenario oracle.

For each actual XML and binary edit result, 15 native candidate faults must be
rejected by VerifyEditFiles and by the independent whole-tree exact comparison:

- Remove opaque metadata; change Data to an array; change the large integer,
  Date, false value or opaque UUID.
- Restore the old target value or change its scalar type.
- Change the action identifier or an unlisted sibling parameter.
- Reorder or remove an action.
- Rename an actual producer UUID and its actual references together.
- Retarget the original raw folder reference.
- Leak an extra provenance field into the native output.

Eight authorization faults per format must also reject: missing policy, wrong
original digest, wrong before value, wrong target path, wrong after value,
duplicate path, type-changing replacement and an empty change set. Original
bytes remain unchanged after every rejected candidate and policy.

The required production parents are
TestBackendRecovery19StructuralMigrationIntegration and
TestBackendRecovery19AuthorizedNativeRoundTrip. Their exact inventory contains
53 events: two parents, three structural controls, two format cases and
46 format-specific native/policy controls.

## Source-bound acceptance and resource bounds

The new safe_import_edit_execution.go and its test file independently check
BRG19's exact title, immutable scenario, expected observation and two tiers before
execution. They emit no related-row result and import no externally supplied
production receipt. The main runner, evidence map and execution audit have only
narrow BRG19 hooks.

Intake requires the complete exact command and 53-event selection. Every selected
test must run and pass; parent and package success cannot stand in for a missing
or skipped child. Nested selected format parents must be active before their
children and cannot pass before all required descendants. Raw output and stderr
must retain valid byte/UTF-8 boundaries, exact digests and lengths, and valid
timing within the observed process interval. Every source path and digest is
compared with the actual immutable Git commit using --no-replace-objects.

The first clean formal attempt at
93f1079891ddae9c04f59ea2fb9b6a46fe33a991 stopped correctly before child execution:
its 68-path inventory exceeded the existing reader's 64-path per-call limit.
The 72 fast controls passed, but the actual producer result was NOT_RUN. Its
original output is retained as a failed outer producer test, with no semantic
execution or acceptance credit.

The bounded repair preserves all 68 source files. It first reads object-size
metadata directly from the same immutable Git commit and checks the original
4 MiB per-object limit and 16 MiB aggregate, including Git headers and data
separators. Only then does it invoke the unchanged reader in two batches of at
most 64 objects. The metadata cache key binds repository, exact commit and the
complete path inventory; cached metadata does not replace immutable byte reads.
The shared helper and its limits are unchanged.

Ten causal size corruptions plus the healthy control passed in the separate
precommit run. They include an aggregate excess spanning the batch boundary,
header bytes crossing the total limit, oversized and negative objects,
integer overflow, wrong object/hash, missing/extra objects and truncation.
Synthetic metadata streams are explicitly parser controls only. Actual source
metadata is always obtained from Git, not trusted from a receipt.

The full actual producer suite then removes each required event, changes
time/order/encoding/process fields with outer hashes reconciled, corrupts every
one of the 68 immutable source digests, and attempts linked-ID or final-release
reuse. All controls reject. All source batches use the same before/after source
identity.

## Original executions and hashes

| Stage | Source | Observed result |
|---|---|---|
| Independent complete component | 2f5424b431904b0da71ac4d34407d717425757cf | 53 passes, 26.094 s, clean before/after |
| Initial intake controls | exact precommit files later committed as 93f10798 | 72 passes, 10.313 s; source unchanged |
| Initial formal attempt | 93f1079891ddae9c04f59ea2fb9b6a46fe33a991 | 72 controls pass; producer NOT_RUN before child execution due to source-count limit |
| Aggregate-bound controls | exact patch later committed as af9ad2eb | 11 passes, 10.719 s; source unchanged |
| Clean complete formal producer | af9ad2ebe96f48eca06dd3692b83f9b08bb2edcf | 258 outer passes, actual 53 inner events, 68 source hashes, two exact tiers |

The initial batching-only commit c89fce5da8cb60a6e9510f32fecf350d57fe0b64 was
followed by aggregate-bound validation before the next actual production run.
It has no independent formal success claim.

| Retained artifact | SHA-256 |
|---|---|
| Component raw, 169486 bytes | baa99b3c2e4f9046c1c2a25299fcbec918a9e62c3b61b7ea1d9e58055d30ceab |
| Initial NOT_RUN outer raw | 97964fe3678a00d1e39c74c22b070c05a15645cb2719edc7b679e7e6bf6533e6 |
| Aggregate controls raw | 595fd9d9abec0ad1ecf5180887202facd9030cbf202df9ed36f66d998ff7dcbe |
| Final outer raw, 311347 bytes | 8277d5e9e47e6c37545314ed0f3ac585dc99ef02343788e239f33ae47444951f |
| Actual inner raw, 169491 bytes | 3670819e8e247e6753fa24aa2178ccd5bee15cbf8781b80e2af17ee07311a123 |
| Actual inner process | 4f809c18f70347eb56a7ad1b09106c3898a468c8ab6d3a56ec2db481282d39d4 |
| Formal two-record receipt | 46eb5c0d457e60fe8f1e9e6b81d47d09b650f23e74519bbc52a11f3b60716efd |

The actual child ran from 2026-11-20T04:18:54.1026412Z through
04:19:19.2007129Z in 25.0979663 seconds, exit zero, no timeout, and empty stderr.

| Actual native scenario | Original SHA-256 | Edited SHA-256 |
|---|---|---|
| Integration XML | 55986d01955302942b19d7055601744fbe35da175c1d63c48c39d3ff7bcdf2e2 | 9b763687b5a87586f7a332bbbe20c56b0faa67b8ec324ea7d56d546c7a5adfc0 |
| Roundtrip XML | 024287696f3cece6dc6213b33959fa22055ae284f523287ced81058875ed5944 | c58d7db10d8b3b9b3fe15907e75de694a537d4d080b107356780c957a3392bea |
| Roundtrip binary | 10ad19227777ca204d8ea9ed60e0354b853ee5c64ef093fcfda64688196947f6 | 1e0f76a41b111803f3e0cb1f20546b7034bc12c12134c72b6206e3df808c604f |

The archive preserves original raw Go JSON, stderr, process receipts and exact
precommit patches. Thirty source/native/policy artifacts and three native receipt
objects were additionally extracted from actual child output and verified against
their recorded byte lengths and SHA-256 hashes. These are derived convenience
copies; the original output remains intact, including its original line endings.

See [the evidence manifest](evidence/brg19-20261120/manifest.json),
[the original receipt archive](evidence/brg19-20261120/brg19-original-receipts.zip),
and [the two-record formal receipt](evidence/brg19-20261120/production-receipt.json).

## Independent review and next work

The acceptance agent reviewed both the actual component and final strict producer
read-only. It found no concrete authorization, source-binding, byte-budget or
scope/accounting defect. The review specifically checked the AST-span edit,
original digest and exact scalar authorization, complete typed native comparisons,
all candidate/policy controls, the two bounded immutable source reads and the
absence of linked-row credit. It did not execute tests; the separate observed
runs above establish the actual result.

Asana master 1220635996500993 was updated for the component and formal stages in
comments 1220729565504737 and 1220729635493840. The master remains open.
The next assigned work is independent RP42/RP44/RP45 extracted installation,
migration-tool, compatibility and rollback proof. Frozen consumer tuples and
historical packages remain unchanged.

Both follow-up provenance registry tests passed in 52.188 seconds. Their original
raw log is retained with SHA-256 d99e01682c206e5a6016818fb7e390168b543e67c7a0a7787dfc15c67fb7bac8.
This metadata validation does not rebind the clean semantic source proof.
