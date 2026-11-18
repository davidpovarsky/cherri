# P15: preserve native collection properties during hybrid import

## Observed regression and causal fixture

The immutable kernel run at `4830dd698f984c4f362aeccdb037dc94384b3407` retained complete dual-dialect wire parity, but four previously natural actions were demoted during V2 analysis. A host-only comparison against the prior passing kernel showed that one specimen accounted for the entire change. No private source, names or specimen identifiers are included here.

The sanitized native shape was four `is.workflow.actions.setitemname` consumers with eight `WFPropertyVariableAggrandizement` attachments in `WFName`, referring to the collection output of `is.workflow.actions.detect.images`. All four demotions had stage/reason `V2_COMPILE:V2_LINE_COMPILE_ERROR`. Analysis produced `E_ARGUMENT_TYPE` because ordinary property syntax requires a single value and the producer's canonical result is `List<AnyContent>`.

The causal public fixture contains only a synthetic `getImages(ShortcutInput)` producer and a `Name` property attachment in a synthetic set-name action. Before the repair, job `job_85fa43da97ba` reproduced the same stage and reason: one of its two candidates was demoted. The initial generic List probe did not reproduce this analysis failure and is not used as causal evidence.

## Repair boundary

The importer previously converted the native collection property attachment to ordinary dotted source syntax. That notation cannot prove a scalar field when the source has collection cardinality. `applyAggrandizementSuffixes` now declines that conversion for a known List unless an explicit supported content coercion already establishes the existing property-access route. Its caller uses the established symbolic `native.ref` / `rawRef` representation, retaining the original producer reference and exact aggrandizements.

Some older importer hints leave an output category empty because relative declaration identifiers do not equal full native identifiers. For attachment rendering, a missing collection hint is recovered only from the existing reverse registry's unambiguous candidate with an array result (a uniquely indexed native ID, or a variant resolved from its parameters). Ambiguous variants are not guessed, and the broader symbol-inference rules remain unchanged.

The enclosing action remains an editable natural `setName` call. This uses the existing native-reference machinery and serializer. It changes neither the analyzer's cardinality rule nor the importer verification thresholds. An import header does not exempt newly authored field access from analysis.

## Verification contract

`p15_list_property_import_test.go` checks direct action-output and named-variable references, each as a parameter attachment and within a text token string. For each case, both dialects must retain all natural actions, have zero Stage-3 demotions, match the original complete wire tree, and pass semantic parity for every action before the provenance overlay. A mutation that removes the property operation must fail the pre-overlay oracle.

The same rendered V2 document is then extended with an ordinary field access on its known List symbol. Actual compilation must still reject that newly authored access with `E_ARGUMENT_TYPE` and emit no native bytes. A separate scalar-property control must retain natural dotted syntax. Existing T08 conditional-cardinality tests remain applicable.

These checks prove local import, analysis and serialization behavior. They do not establish Apple execution of a property operation. Final recovery of the complete private corpus remains a separately recorded kernel gate; the required zero-demotion and natural-count thresholds are unchanged.

## Focused execution

Job `job_76c11cc9afbb` passed all 15 selected top-level tests: the three new collection-property checks, both T08 production/service producers, six existing P12 parameter/attachment tests, and all four existing P13 fidelity fixtures. The selected command was `go test -count=1 -timeout=90s -v . -run '^Test(P15Imported|P15T08ConditionalCardinality|P12_PAR_(03|04|05|06|07|08)|P13_FID_)'`. The first causal run remains a recorded failure; no threshold was changed.

## Immutable source receipt

Job `job_c12377d1d4a5` reran the focused command on clean source `89d18c1869b0bba8b532fc36ea367f700d41232d`, passing all 15 top-level tests with exit zero and a clean tree afterward. See [the source-bound receipt](evidence/p15-native-property-89d18c1/receipt.json), [raw Go JSON](evidence/p15-native-property-89d18c1/verification.jsonl), and [the causal baseline failure](evidence/p15-native-property-89d18c1/baseline-red.log). The full private-kernel gate remains independent.
