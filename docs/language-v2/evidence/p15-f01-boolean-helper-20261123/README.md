# Authored Boolean helper comparison repair

Implementation: `5411d526ca807eb5120b80d09c43338d2ddffa41`. This packet records a source defect, its focused
regression repair, and unchanged native-import controls. **The repaired full F01
has not yet run on Apple.** The original signed K10 failure remains
[separate evidence](../p15-k10-qa-short-circuit-20261123/README.md).

## Observed failure and reconstruction boundary

The original run45101081393 returned the fresh correlated result
`CHERRI_QA_F01_RESULT:P15_NEGATED_IS_TRUE`, rather than the unchanged
`CHERRI_QA_F01_RESULT:P15_F01_OK`. Its original native condition/input values
are UNOBSERVED: the retained artifact contains a parameter-key projection,
not the complete original unsigned or signed payload.

One separate local unsigned reconstruction used the verified R7 ARM64 compiler
SHA25642f6e42e7b6a8ea6f523a752fd2f06b96265e733fa00ad808190e062d1266301
and the exact K10 QA fixture SHA256ac71d4b09360e8793f167bb5b58b5df161ada9e5da15906c81735efff1de3147.
It completed in0.060031s and emitted156actions/82519bytes with new UUIDs.
It showed Number1 assigned to truth, an inner isTrue(truth) predicate with
condition4 and no RHS, and an outer negation with condition5 and numeric1.
This reconstructed file is not the originally signed payload and proves no
native branch or platform cause.

## Narrow repair

Authored isTrue(x) follows x == true; isFalse(x) follows x != true.
Both single-condition and compound-template emitters now include the explicit
numeric1 operand for statically Bool arguments, using the existing numeric
conditional-input coercion. The compound path retains the existing integer
representation; the single path retains the existing real representation.

The shared guard preserves original imported-symbol/reference predicates and
explicit WFBooleanContentItem coercions, including the imported global
ShortcutInput.bool. Unknown/non-Bool helper operands, date/range helpers,
the Legacy frontend, lazy producer placement, and the complete original F01
fixture remain unchanged. No carrier-analysis subsystem or importer rewrite
was introduced.

## Actual verification

| Phase | Original observed outcome |
| --- | --- |
| Regression before production edit |4parents FAIL; detailed parent16subcases FAIL and8PASS; expected missing-RHS errors; no timeout |
| Native import baseline before edit |8synthetic cases + parent PASS; exact Legacy/V2 pre-overlay equality and3/3natural actions in every case |
| Independent static review |Exact v3 patch cleared; no reviewer test or compiler execution |
| Focused repair run |19parents +91subcases =110PASS;0FAIL,0SKIP;23.319765s including compilation; no timeout |

The green command was `go test -p=1 -count=1 -json -timeout=90s -run '^TestP15F01' .`.
Its300-second outer compilation/test envelope was unchanged from the red run.
All source, fixture, protected-UI and three acceptance-contract bytes recorded
in its claim remained unchanged while it ran. The implementation commit contains
the exact tested bytes; no clean-source replay was performed solely to restamp it.

Existing lazy/control/mutation/wrapper checks run together with the new controls.
The authored equivalence checks retain every action, parameter, plist numeric
type, coercion and reference relationship, mapping only bound random action and
group identities. Direct native Bool calls, immutable/mutable bindings and
snapshots are source-level equivalence checks, not new observations of battery
or charger behavior in Apple Shortcuts. Imported no-RHS controls are explicitly
synthetic input records; exact equality plus natural action counts prevents a
fallback from concealing a regression.

## Evidence and remaining work

The packet retains original claims, streams and receipts for all three Go runs,
the separate local reconstruction, the reviewed patch, and commit/source joins.
Each packet member is verified by packet-manifest.json. No private corpus, key
or provider credential is included.

A new source-bound compiler/migrator pair, genuine private S4 qualification,
downstream documentation/Skill pins, and the unchanged full signed F01 runtime
are still required. Old R7 corpus/505/runtime receipts retain their original
sources and are not reissued for this change. R23 policy and the frozen179
acceptance criteria remain unchanged; this local repair grants no aggregate
or physical-device acceptance.

## Separate provenance validation and independent packet review

The required non-circular provenance validation ran once after the implementation
commit: TestForkActionProvenanceRegistry passed in 5.532021 seconds including
compilation, with exit 0, no timeout and unchanged registry bytes. Its original
claim, streams and receipt are included beside the original 27-member packet.
The packet ZIP and its manifest are unchanged.

The independent metadata review verified all 27 packet members against the
retained originals, all 11 green-claim source hashes against implementation
5411d526, the 19 original passing parent identities, and preservation of every
prior registry entry. The added entry references the immutable implementation
commit. This review executed no compiler, test, native command or private input.
