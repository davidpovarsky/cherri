# Empty List fields in authored records: feasibility only

The original F04 List-yield probes remain unchanged. Their actual historical
native failures are outer count four instead of two for nonempty yielded
Lists, and outer count zero instead of two for empty yielded Lists. The
E_TARGET_VALUE_SHAPE boundary remains in place. This diagnostic grants no
formal F04 acceptance and does not implement or claim List<List> semantics.

The separate qa-f04-record-empty-v2 source authors two Dictionary records,
each with tag and items fields, where items is an explicit empty List. The
probe reads actual first/last records, their tags, their key counts and their
items-field counts. It also observes two scalar Repeat Results, an independent
empty-List cardinality control, and record-type parity against a separately
authored Dictionary witness. Type comparison therefore does not depend on
the localized name of the Dictionary native type.

The exact callback required for this bounded feasibility question is:

CHERRI_QA_F04_RECORD_EMPTY_RESULT:outer=2:scalars=2:tags=LEFT,RIGHT:keys=2,2:items=0,0:empty=0:dictionary=Y

It must be accompanied by the actual source-bound NDJSON trace. No expected
value is copied from a runtime result. A different outer count, dropped key,
nonzero inner count, swapped tag, failed type parity, stale run, error callback,
or missing trace fails the probe.

The local graph gate requires a Dictionary body with the items field encoded
as an empty WFArrayParameterState, the direct Repeat Results producer,
explicit item/key reads and counts, an independent empty control, exact type
witness edges, dynamic observed output, and the original F04 source hashes.
It rejects damaged actual compiler graphs; it does not simulate Apple.

Even a future native success would establish only this Dictionary-carrier
and field-cardinality behavior. It would not establish a nested List runtime
type, preservation of arbitrary values, or general language lowering support.
Empty field output cardinality is the observed boundary; an empty native
container's independent runtime element type is not inferred from count zero.

Historical nonempty Dictionary-with-List behavior is separately recorded at
run 44253551852 on 38c7098772f4b822d6a3555137c0c7cf8e5b1e5e. That result is
not relabeled as current-source evidence.

Source preparation and compilation are local-only until a reviewed exact
source dispatch produces native observations. Do not replace the original
F04 oracle with this diagnostic's expected result.
