# Native nested-List observation: control and experimental row

This component prepares two public synthetic diagnostics. It does not add a
compiler encoding or grant L01, F04, T05, FN05, or any other formal requirement.

## Question and existing evidence

The retained Apple metadata in
docs/apple-runtime/apple-runtime-snapshot.json.gz identifies the List action's
WFItems parameter as WFContentArrayParameter. The established compiler path
for a Dictionary inside a native List uses a separate Dictionary producer and
its ActionOutput attachment. See native-dictionary-list-values.md.

The separately retained native JSON observation at source
03f17a785f31f7bdc8c7607bef1ac9a16c28d3b4, run44713853346, observed nested
matrix fields through a Dictionary/JSON body. Its experimental type5 field was
absent, not null. That observation is not evidence for a nested List item.

The candidate here transfers a known Dictionary array-field representation,
WFItemType:2 with WFArrayParameterState, into the List action's parameter.
This is an explicitly experimental hypothesis. No native List type name or
runtime result is assumed.

## Separate sources and observations

qa-nested-list-control-v2.cherri independently observes:

- A flat List's count and first/last text bytes.
- An authored empty List's actual count.
- Two Dictionary producers inside a List, preserving order, tags, both keys,
  populated first array contents, and the second empty array.
- Actual type-name bytes for Text, Dictionary, and both observed Dictionary
  items. Dictionary parity is evaluated against the independent Dictionary
  witness and a distinct nonempty Text witness, without an English type label.

qa-nested-list-candidate-v2.cherri contains only the fixed two-row native
candidate plus bounded observers. The first row contains INNER_A, INNER_B;
the second is an explicitly empty array. It observes the actual outer count,
both item counts, first/last inner text bytes, and actual type-name bytes.
Get First/Last reads are guarded by the corresponding actual outer/inner count.
The exact -1 and UNOBSERVED sentinels describe a read that did not occur.

Both fixtures have explicit main returns. The unchanged fixed before/after
debug-trace protocol and actual source-bound callback must accompany any native
observation. No clipboard, network, user data, or device state is read by the
fixture itself; the existing authorized Simulator receiver supplies transport.

## Independent local graph gate

nested_list_probe.py compares every action and parameter in both actual
unsigned plists against independently authored graphs:56 control actions and
78 candidate actions. Only generated producer and control-group UUIDs vary.
Their syntax, uniqueness, exact edges and ownership are checked. The candidate
producer UUID is fixed by its public source.

The control's empty List must use the exact existing Nothing/Set/Get clearing
path. Its nested array fields remain Dictionary data. The candidate's row type,
inner order, explicit empty array, count/read inputs, guarding comparisons,
mutable observation stores, trace actions, summary token ranges, and terminal
output are all checked. No unrelated action, dangling/crossed reference, altered
output, extra terminal Nothing, or borrowed Clipboard input is accepted.

The actual graph is separately corrupted in28 control and33 candidate cases.
These include flattening, removed empties, swapped rows, redirected counts,
wrong type witnesses, inverted guards, crossed nested branches, lost trace,
static summaries and replaced output. The Go gate compiles the real sources
through CompileSourceToPlist, validates their original plist bytes, and then
requires rejection of an independently altered final-output plist.

Run the local production path with:

    go test -count=1 -json -timeout=3m -run '^TestP15NestedListProbe(ActualCompiledGraphs|ObservationControls)$' .

CHERRI_NESTED_LIST_PYTHON may select a real Python3 interpreter on hosts where
the default command is unavailable. A missing interpreter fails the gate.

For the exact unsigned artifact selected for native signing:

    python3 tools/shortcuts-simulator-qa/nested_list_probe.py --root . --fixture <fixed-id> --unsigned <actual-shortcut-path>

The single CHERRI_NESTED_LIST_UNSIGNED_GRAPH_V1 receipt is emitted only after
source/contract hash checks, the complete graph comparison, and all actual graph
corruptions. It binds the exact unsigned bytes. It is local evidence only.

## Dynamic callback contract

The two .expected.json contracts pin source bytes, exact prefix, closed field
order, authored shape goals and trace identity. validate_summary(summary,
fixture_id) accepts exactly one semantic line; its caller must independently
authenticate the original complete callback, nonce, source, unsigned/signed
artifacts and NDJSON trace through the established native route.

Counts have a bounded decimal grammar. Base64 must be canonical strict UTF8;
type names are limited to128 decoded bytes and item text to256. Unknown,
missing, repeated or reordered fields, inconsistent guarded branches and
unbounded values fail validation. Native goal contradictions remain actual
observations and are retained unchanged.

controlGoalsMatched concerns only the explicit control goals and independent
Dictionary type parity. candidateShapeMatched concerns only the experimental
two-row count/order goal. Even a shape match or a returned type name spelling
List leaves nativeListTypeProved=false and formalAcceptanceGranted=false.
No expected full summary is derived from an actual callback.

The15 Python parser/contract controls use labeled fabricated summaries solely
to test this observer. They are not Simulator evidence.

## Remaining gate

Local preparation must be sealed on a clean commit and reviewed before the
existing native workflow selector, receiver allowlist and publication checks
are extended. Actual native import/execution, full callback preservation,
cleanup and time-budget gates remain required. Keep control and candidate
results separate if the experimental row fails import or execution. Production
List support requires a later reviewed semantic decision backed by native
cardinality, order and type observations, not this fixture's existence.
