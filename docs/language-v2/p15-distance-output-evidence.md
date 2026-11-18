# Get Distance and explicit native comparison operands

## Canonical Get Distance correction

The existing `getDistance` declaration now returns `number`. Its identifier,
parameters, defaults, builder and native serializer are unchanged. The canonical
language registry is regenerated from the existing live action catalog and still
contains 505 actions.

Independent evidence is the checked-in file
`docs/apple-runtime/macos-27/builtin-actions.json`, SHA-256
`8fb303591888bcb01fe5935b1571d590add428c9bd044f0ab52dcdf99490571a`.
Its `is.workflow.actions.getdistance` entry identifies
`WFGetDistanceAction` with output name `Distance`, `Multiple=false`, and
exact output type list `["NSNumber"]`.

The production controls read that metadata, require the canonical Number result,
compile both dialects, and check the actual distance producer UUID, explicitly
supplied false option values, and ordered numeric consumer. Producer, option,
and RHS mutations must fail the graph oracle. The language service reports
Number. An unrelated opaque result still cannot satisfy a Number annotation.

The native schema fingerprint changes because one previously unspecified return
type is corrected. The receipt records the measured fingerprint. Historical
catalog evidence and consumer pins are not rewritten by these tests.

This is local semantic and serialization evidence. It does not execute location
services, establish route availability, validate caller-supplied locations, or
prove an Apple runtime result.

## Exact native references in ordered comparisons

The unchanged public `TestP12_CF_05_NumericPropertyAndDistanceOrder` also
contains native Height-property references with `PropertyUserInfo`. The
originating Resize Image action has a multiple-output native contract. No
property-specific scalar or cardinality contract is inferred from that metadata.

A resolved built-in `native.ref` may now remain an explicit operand of an
ordered comparison when its reference shape is statically validated. The
expression continues to have type AnyContent. An alias, Number annotation,
ordinary Unknown result, or ordinary AnyContent value receives no exemption.

Two validators extracted from the existing native.ref lowerer check the reference
kind and required UUID/name, and the list-of-dictionaries aggrandizement envelope.
The analyzer and lowerer use the same validators. Opaque property fields and
nested data remain unchanged. This validates reference shape; it does not
establish a producer's existence, runtime scalar type, property semantics, or
native cardinality.

Static target evidence is bound to actual lexical value symbols and valid
imported reference bindings. The exact built-in namespace symbol must resolve;
a local or parameter named native and a user-defined nativeRef lookalike cannot
acquire the exemption. Each analysis and each call recomputes the relevant facts.

Dynamic kind, UUID, property, or metadata expressions whose reference shape
cannot be established retain the ordinary strict diagnostic at an ordered
comparison. Other raw escape uses retain their previous lowering behavior. This
conservative limit does not introduce a new native-reference serializer.

## Causal and compatibility evidence

The original CF05 failure and original getDistance Unknown failures are retained.
An isolated Go overlay tried the existing explicit `.number` view in the
imported ordered comparison. Compilation advanced, but exact verification
rejected its extra aggrandizement with `WIRE_ACTION_PARAM_MISMATCH`. That
projection was not applied to production.

The corrected path preserves both original property attachments, output UUIDs,
PropertyUserInfo, the numeric comparison field, and balanced native control
groups. Six graph mutations independently corrupt those relations and must be
rejected. Existing reference metadata, opaque raw result, strict authored type,
and control-region tests remain part of the focused verification.

Malformed and statically unproved reference shapes are exercised through actual
analyzer, compiler, and Service.Analyze paths. A counterfactual overlay removes
only the new shared validation check; the eight malformed service controls must
then fail. That experiment proves the service guard, while the unchanged CF05
test proves exact native preservation.

The first new fixture runs exposed incorrect test setup: missing Legacy module
inclusion, invalid custom-action bindings syntax, omitted rather than explicit
false options, and semantic parameters where an exact native.plist payload was
intended. Their failed outputs are retained separately. The fixes use the
existing source syntax and exact native parameter path; no wire expectation was
relaxed to accept changed producer identity.

The complete private corpus is validated separately by the integration owner.
No private specimen, generated private source, or raw private log is committed by
this change, and no complete-corpus recovery is inferred from the public test.
