# Debug export verifier: exact main boundary and actual replay

The original frozen K8 OpenMinis run45092778441 failed in the Python validator,
after the unchanged Go DebugExport tests passed. The validator included V2's
explicit terminal Nothing in the macro model's allowlist. Compiler semantics
were not changed.

The first boundary repair was independently reviewed and its seven synthetic
controls passed, but its actual replay at4ecad81e failed: the native plist omits
the empty WFWorkflowActionParameters field on terminal Nothing. That original
failure, actual unsigned plist, streams, and initial review remain preserved.
The follow-up fe37e330 accepts omission or exactly an empty dictionary while
rejecting null, lists, text, nonempty parameters, wrong identifiers, interior
effects, and a missing File producer. Eight focused controls passed. Legacy
keeps its complete tail. The full original action list remains the operand of
exact lossless import/recompile comparison; no field is inserted or normalized.

## Actual successful execution

Clean source05854a2e78df7d6b7b215bf104a628830cc6bdad executed the complete original
CLI/model matrix with the repaired boundary. Actual exit0, no timeout, 4.130
seconds including supervision; source HEAD and tracked bytes stayed unchanged.

- 16 CLI cases and16 exact complete-action roundtrips.
- 80 independent model cases, including empty/one/thousand items, Unicode,
 control characters, redaction, and the native-type matrix.
- 16 invalid-source cases rejected without output.
- 7 causal faults rejected with restored positive controls.
- 12 public native-probe compiles and12 further exact-action roundtrips.

The report SHA256 is4c2373e50f2c94b9c6a6b82db70b942ca0f7c6873e7f5a75753a1d3e7d84ae49.
All actual generated sources, unsigned graphs, imported sources, roundtrip
graphs, execution receipts, original streams and independent reviews are
byte-preserved in originals.zip. Metadata packaging independently rechecks
all compiled source hashes, probe output hashes and28 graph equalities.
This read-only packaging is not another compiler execution.

## Scope

Status is PASS_SOURCE_AND_MODEL_ONLY. Native executions remain0. This verifies
the local macro graph and its independent bounded evaluator; it does not prove
Apple File transport, native item cardinality, native Base64 behavior, physical
device execution, broader feature completeness, or release acceptance.
The separate K8 failure and both source identities remain visible.
