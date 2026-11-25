# Strict differential comparator

This runner compares compiler-produced Shortcut property lists. It never edits,
repairs, or reconstructs the compiled outputs. Every raw typed difference is
retained, including integer versus real, boolean versus integer, absent versus
explicit values, output names, defaults, and workflow metadata.

## Identity policy

UUID alpha-renaming is available only after both native graphs pass validation
and their action identifier sequences align. It is restricted to these paths:

- Producer UUID at WFWorkflowActions[i].WFWorkflowActionParameters.UUID.
- GroupingIdentifier on native Conditional, Repeat, Repeat Each, and Choose from
  Menu markers, with matching marker modes and action identifiers.
- ActionOutput references in native WFTextTokenAttachment.Value,
  WFTextTokenString.Value.attachmentsByRange, and
  WFQuantityFieldValue.Value.Magnitude slots.

Producer and group bijections are established before comparison and frozen.
An unknown identity pair is never registered while visiting a leaf. A payload
field named UUID, OutputUUID, or GroupingIdentifier remains exact literal data;
a plain object with Type=ActionOutput does not independently grant an identity
exemption. Unrecognized object shapes remain exact.

Graph validation rejects duplicate or invalid producers, missing/malformed
native references, self references, and references to later producers. Native
control starts are globally unique, including after a prior group has closed.
Middle and end markers must match both the active group and its action
identifier. Named Repeat Index / Repeat Item variables have no producer UUID;
a control result is referenced after its end marker. The actual corpus and
the compiler's derived UUID regression fixtures require no forward-reference
exception.

A wire-equal result requires all typed differences to disappear under this
restricted mapping and both graphs to remain valid. The older imported fidelity
oracle is retained as a separate advisory result. It cannot override the
strict comparator or register new alpha mappings.

Reference recognition treats explicit WFSerializationType wrappers inside action
parameters as native wire types. The audit covers those emitted shapes;
application-specific opaque native.plist payload schemas require separate
validation.

Wire equality is not runtime correctness. Differential fixture 15 demonstrated
an inherited statistics input-key bug shared by two compilers; the fixed,
independently calculated output expectation detected their identical failures.

## Regression evidence

verify_runner.py mutates actual compiled fixture trees in memory. It covers
ordinary literal identity fields, plain ActionOutput-shaped data, native
producer/reference alpha renaming, typed quantities, graph edge retargeting,
self/forward references, sequential and nested group collisions, mismatched
middle/end identifiers, metadata/default/display differences, and exact scalar
types. It never emits a mutated shortcut for execution.

The 2026-11-25 review ran 36 checks against both implementations:

- Corrected checker: 36 PASS.
- Original cf10c7eb checker: 15 targeted failures on the same checks.
- Existing default corpus: 151 valid graphs of 152 compiled artifacts; the known
  frozen-original fixture 23 malformed/dangling reference remains invalid.
- Existing old derived-UUID corpus: 127 valid graphs and 25 invalid graphs;
  the historical failures remain preserved.
- Repaired derived-UUID replay: 24 previously compiled artifacts, 12 exact replay
  pairs, all graph-valid and strictly equal.
- All 112 existing default comparison reports retain exactly the same complete
  raw difference lists. This validation recompared existing artifacts only.

Evidence is retained under
/mnt/development/projects/evidence/cherri-differential-30-20261125/suite/comparator-review/.
No native execution or full compiler suite was repeated for this checker repair.
