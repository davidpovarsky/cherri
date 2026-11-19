# Inferred native List-row boundary

Date: 2026-11-19

A native List row cannot currently preserve an arbitrary nested List as one
semantic element. The existing compiler guard rejected a directly nested
literal, but List-valued references, function calls, and value-block results
could bypass it. The integration compiler at
`d7052f4568db26dba169bdefc72623e48c04968a` compiled eleven parser-valid
reproduction forms, including an empty List reference and an optional absent
List, without reporting this limitation.

For example, both of these previously produced an artifact:

~~~cherri
let inner = [1, 2]
let outer = [inner]
getVariable(outer)
~~~

~~~cherri
function rows() -> List<Number> { return [1, 2] }
let outer = [rows()]
getVariable(outer)
~~~

The lowerer now applies the existing `E_NESTED_NATIVE_LIST` boundary to
statically List-valued elements as well as List literals. It inspects inferred
types, Optional and Union members, and the known lowered binding type when a
broad or unknown annotation has erased that information. This last fallback
was tested causally: the first attempted repair still allowed a type-alias
binding; retaining that negative case exposed the hole before the correction.

The change applies only to elements of authored runtime native Lists. Literal
arrays inside typed Dictionary fields and explicit raw native metadata retain
their separate inline representation. Scalar and Dictionary references remain
valid List elements. Existing authored List-yield rejection is unchanged.

## Executed controls

`TestP15L01InferredNestedNativeListFailsClosed` checks fourteen parser-valid
negative forms through both the embedded production compiler and mobile
adapter: immutable nonempty/empty Lists, mutable values, a value alias, an
explicit List annotation, present/absent Optional Lists, function results,
function parameters, conditional results, value-loop results, a type-alias
binding, a broad AnyContent binding whose initializer is known to be a List,
and a union containing List.

The positive control compiles scalar and Dictionary references and inspects
the actual serialized native List cardinality. The focused run also executes
the existing independent nested Dictionary shape and corruption controls,
unknown native-tree XML/binary import round trips in both dialects, and the
existing native-loop/List-yield boundary control.

Observed result: **48 test/subtest passes, zero failures**. Local receipts are
`dist/p15-acceptance-causal/list-row-expanded-baseline.json`,
`list-shape-fixed.jsonl` (the rejected first attempt), and
`list-shape-fixed-v2.jsonl` (the passing correction). The passing local run
used a working tree based on `adec591b`; final integration evidence must be
regenerated at its clean combined SHA.

## Scope that remains incomplete

A genuinely dynamic AnyContent value or a function whose type erases its
runtime container shape may still contain a List. This guard does not prove a
native representation or runtime check that retains such hierarchy.
Dictionary literal wire preservation does not establish arbitrary dynamic
Dictionary-array behavior on a device.

L01 remains an incomplete formal requirement, with the new executed controls
added to its observation producer. F04 and FN05 are not completed by this
change. No frozen requirement, expected native result, or 179-case status file
was changed. The guard prevents the demonstrated static bypass; broader
container transport and Apple runtime proof remain separate work.
