# P10R15 RP16: exhaustive checked AST and canonical boundary

## Verified result and scope

RP16's two immutable local tiers, `unit` and `native-negative`, pass on clean source
`db4a8ca6e6f42bebb3ecfd059de3d323136c6782`.
The actual acceptance producer launches the production compiler tests, requires all
114 named run/pass pairs, and retains its original process receipt, raw Go JSON,
stderr, and 45 immutable Git blob hashes. Its two evidence records identify RP16
alone. The outer producer/intake suite passed 280 named events in 22.375 seconds
with no failed or skipped test.

The compiler component had already passed 114 root tests and all 38 tests in
`internal/language/lower` on clean
`d07bbf7f049503a2271df28f7fd18c4db839aa06`.

This establishes the approved-node inventory and deliberate-unknown-node boundary.
It does not establish the complete semantics of the linked P03, F03, FN01 or M03
requirements. It supplies no Apple runtime, CI, consumer tuple, release, or
179-requirement closure. The actual final-phase validator still rejects the local
manifest because global tuple and successful CI evidence are independently
required. Integration onto another source requires a fresh producer run; historical
receipts must retain their original source identities.

## Defects established before repair

The earlier narrow unknown-expression fallback repair correctly rejected nodes
that reached expression lowering. It could not inspect children that normal
lowering intentionally did not visit.

On clean tests-only source `94d79013e876b89175af734a9d58fe2e32b9fdc3`,
three injected unknown expression contexts returned success and reached the real
canonical backend finalizer:

| Context | Observed invalid successful artifact |
|---|---:|
| Unused function body | 2,195 bytes |
| Unused Copy body | 2,195 bytes |
| Metadata Record field | 2,197 bytes |

Nine other declaration, statement and expression rejection controls already passed.
The injections happen after successful parsing and semantic analysis of a healthy
program. They test an unknown internal AST kind directly; an unresolved import is
not used as a substitute for an unknown node.

Independent review then identified a second category: type annotations also
implement the syntax Node interface. The first traversal did not visit those
descendants. Clean tests-only source
`c00a039626c7b15ed80f3579cd4e12116185b0e3` reproduced silent canonical
finalization in all 11 injected annotation positions: alias, binding, function
parameter/return, action parameter/return, setup, and Optional, Generic, Union and
Record child types. The 42 approved fixture children passed in that same run.

## Production change

`internal/language/lower/exhaustive_validation.go` is an explicit closed
inventory of approved declaration, statement, expression and annotation kinds.
It recursively validates active AST children, including unused function and Copy
bodies, metadata Records, call arguments and scoped bodies, defaults, control
branches and annotation descendants.

A narrow hook in `LowerProgramToSession` validates the entry program before
resolution/emission and validates each resolved module program before emission.
An unrecognized kind returns an explicit unhandled-node error. The existing
statement/expression lowering fallback remains intact.

This validation does not add a parallel emitter, action schema or native serializer.
Approved programs still use the existing analyzer, module resolver, lowerer and
canonical backend session. Inactive fields such as an expression attached to a
literal FString part are not represented as an approved semantic child by this
contract. The tests do not claim arbitrary malformed-object validation.

## Positive inventory and native observations

The checked-in public fixture inventory is
`tests/language-v2/rp16-approved-nodes.json`.

The test independently parses `internal/language/syntax/ast.go` with Go's
parser and derives its actual marker-method inventory. It compares that inventory
with the fixtures, rejects missing/duplicate/foreign roles, and walks the real
parsed AST in its declared interface context. Dual-role control nodes are exercised
as statements and as expressions in separate authored fixtures.

| Role | Actual approved kinds |
|---|---|
| Declarations, 9 | Import, Shortcut, Trigger, Setup, Type, Enum, Function, Action, Copy |
| Statements, 12 | Binding, Assign, Expr, Return, Yield, Block, If, For, Repeat, Menu, Copy, Paste |
| Expressions, 16 | Literal, FString, Ident, Member, Index, Call, Unary, Binary, List, Map, Record, EnumMember, If, For, Repeat, Menu |
| Type annotations, 5 | Named, Optional, Generic, Union, Record |

Each public fixture is parsed, analyzed, lowered, and compiled through
`CompileSourceToPlistWithModules` and the real canonical backend.
The native oracle checks the required observable wire facts, rather than merely
requiring a nonempty artifact or a minimum action count. These include exact
parameters, producer/consumer UUID edges, unique and balanced control groups,
literal/container data, text attachment offsets, loop ownership, index conversion,
function dispatch/return edges, setup questions, triggers and metadata. Each fixture
also has a deliberately corrupted required native witness that must be rejected.
Workflow metadata that is represented in IR is checked there as well.

Unknown-node tests require explicit lowering failure, no call to the canonical
finalizer, and no artifact. They close the actual session and compile a healthy
restored program afterward, checking its expected native Text action.

## Actual production selection

The four root test parents are:

- `TestP15RP16ApprovedNodeInventory`: 42 approved fixture children.
- `TestP15RP16InventoryControls`: each of the 42 missing-kind controls,
  plus duplicate, unknown and wrong-role controls.
- `TestP15RP16UnknownNodesCanonicalBoundary`: 12 injected contexts.
- `TestP15RP16UnknownTypeAnnotationsCanonicalBoundary`: 11 injected contexts.

Together with their parents these produce exactly 114 named pass events.

The specialized acceptance producer is
`tools/language-acceptance/exhaustive_node_execution.go`.
It runs all four parents through a finite subprocess, checks clean Git state
before and after, and binds both records to the same complete actual execution.
The native-negative record is not inferred from a failed parse or a successful
unit-only parent.

The strict Go-event parser was extracted from the existing loop producer into
`strict_go_events.go`, preserving its event ordering, duplicate-key,
UTF-8, finite timing, exact package/name and child-completeness rules. Loop
scenario inventories stay independent. Three existing loop producers and their
actual receipt mutation controls were replayed after that extraction on clean
`a7623a39a170abcfa1fc91dbd6ea2fe9f834cc18`. Their six local records
remain partial because the complete Apple loop tier is still required.

## Evidence intake controls

The successful RP16 run exercises corruption controls against its actual generated
records and files. Outer hashes are recomputed after deliberate inner changes so
a simple digest mismatch cannot hide a missing semantic validation.

The controls reject omitted approved or negative children; missing or duplicate
run/pass events; skipped, failed, foreign and unknown events; parent completion
before children; invalid UTF-8 and duplicate JSON keys; missing, backward or
out-of-process times; missing, negative or excessive elapsed values; forged process
source, commands, clean state, exit/timeout status and benchmark claims; missing or
duplicate artifacts; every one of the 45 forged immutable source hashes; and
incorrect record tier, producer, scope or requirement membership.

All record-corruption checks run in local phase, where an unrelated missing global
CI run cannot mask an invalid acceptance decision. A separate control confirms
that a valid local RP16 row cannot bypass the global final release gate.
Rebinding both actual records to each of P03/F03/FN01/M03 is rejected. The full
179-row execution audit has exactly the RP16 row marked PASS and leaves all
unexecuted rows uncredited.

Synthetic streams are confined to parser unit controls. They never enter the
actual production evidence path.

## Retained original evidence

The archive and complete member digest index are:

- `evidence/p15-exhaustive-node-acceptance-20261120/rp16-original-receipts.zip`
- `evidence/p15-exhaustive-node-acceptance-20261120/receipt-index.json`

The archive contains 64 original files, 167,347 bytes compressed, SHA256
`ab86f0f3533c091615d6a6b9359f8659bbbe80005734ae8ff820b76a18a79d80`.
Raw Windows newline bytes are preserved. The archive includes public synthetic
fixtures/graphs and task execution receipts; it contains no private corpus.

| Stage | Actual observation |
|---|---|
| Initial draft | No test execution: test helper used nonexistent `file.Path`; source and build failure retained. |
| First baseline, `880ce4b9` | Genuine unused-function failure, then test-setup panic because analysis had expanded CopyDecl; incomplete baseline retained. |
| Corrected baseline, `94d79013` | Three genuine silent-finalization failures, nine rejection controls pass. |
| Approved exploration / initial precommit | Dirty positive/native-oracle work retained, without clean formal credit. |
| Annotation baseline, `c00a0396` | Eleven genuine unknown-annotation failures and 42 passing fixture children. |
| Annotation precommit | Dirty 114-event green result retained separately. |
| Clean component, `d07bbf7f` | Root 114 and entire lower package 38 pass; no failure/skip. |
| Registrar precommit | 179 parser/inventory test events pass; this count is not 179 requirement rows. |
| First formal attempt, `a7623a39` | Actual inner RP16 and loop regression pass; outer test incorrectly expected local-phase closure. |
| Second attempt, `65f3a943` | Actual inner RP16 passes; outer test incorrectly expected final release closure without global CI/tuple evidence. |
| Corrected clean formal run, `db4a8ca6` | All 280 producer/intake events pass, two RP16 records accepted; no failure/skip. |

Final outer raw Go JSON SHA256:
`dff35501b56a48a3a66ea2168f2ecadac28f52497ecab78e777b8426e6f38ae8`.

Final inner raw Go JSON SHA256:
`5218d0b61522ef53f1dda184a816e1b56db001ac69e066723390b760c5572a39`.
The inner actual execution took 6.49183 seconds.

The source chain is:
`880ce4b9 -> 94d79013 -> 7eeafecc -> c00a0396 -> d07bbf7f -> a7623a39 -> 65f3a943 -> db4a8ca6`.
The first two commits are tests-only baseline work; `7eeafecc` introduces the
production traversal, and `d07bbf7f` completes annotation traversal.
The last two changes correct only test expectations; acceptance/release
requirements were not weakened.

## Reproduction and integration

From an exact clean checkout with Go and the repository's normal dependencies:

```text
go test -count=1 -json -timeout=4m -run ^TestP15RP16 .
go test -count=1 -json -timeout=3m ./internal/language/lower
go test -count=1 -json -timeout=8m -run ^TestExhaustiveNode ./tools/language-acceptance
```

Set `CHERRI_RP16_EVIDENCE_OUTPUT` to an owned directory outside the checkout
to retain the actual inner artifacts. The producer refuses dirty or mismatched
source. The observed local host used finite process budgets and
`GOMAXPROCS=2`/`GOFLAGS=-p=2`; no race or native Apple claim is derived
from that Windows execution.

Independent acceptance review found the TypeAnnotation gap before its repair and
subsequently found no concrete fail-open in the complete 114-event, immutable-source,
two-tier and no-inheritance intake. Asana master updates are
`1220728106188582`, `1220728296008744` and
`1220727992240980`. Integration and a fresh combined-source run remain
the integration owner's next step.

## Provenance metadata check

The separate infrastructure entry references the already committed implementation
7eeafecc49c0f611b565e92349923acebac3e9f4. TestForkActionProvenanceRegistry
actually passed in 39.532 seconds with only proof/provenance metadata pending.
Its original three files are preserved in evidence/p15-exhaustive-node-acceptance-20261120/rp16-provenance-check.zip;
the digest index records that separate check. This does not replace either clean
compiler or actual acceptance producer run above.
