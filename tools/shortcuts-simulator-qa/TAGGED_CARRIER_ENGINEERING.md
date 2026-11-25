# Tagged carrier engineering fixture

This fixture is an engineering discriminator for the source-owned List/Map cell
representation. It has not yet established native carrier preservation and
does not grant T05/L01 or final179 acceptance.

## Scope

The fixed source observes a two-number List payload, a present empty List
payload, a two-key Text Dictionary payload, and an absent Optional Dictionary
cell. Eighteen returned fields check presence, count, first/last items, keys and
values. Five canonical nonempty UTF-8 base64 type witnesses distinguish the
original and decoded payloads from Text. Dictionary key/value actions execute
only after the decoded and original Dictionary type witnesses compare equal.

The fixture does not prove false, JSON null, an empty Dictionary, File values,
arbitrary recursive conversions or the complete language contracts.

## Original source graph

Compiler source: cb23a75446f1fdb13da9d10fe7ffe9d6903b47f7.

The public original compiler output contains 225 actions and 154,009 bytes.
Its SHA-256 is
dabf4b0869396585e315a0fbc397945fcdc1efc7defbf64763b274e75b85c2ea.
The exact bytes are retained in
fixtures/source-graphs/qa-tagged-value-carrier-v2.compiler-cb23a754.plist,
with an adjacent provenance JSON. The original source hash is
8abc203ba48ba97e6c3c28240dd861e3a6b8b4a0b5ac91f7e82334ca528a46ec.

The tagged_carrier_graph.py validator independently describes the complete
authored action sequence. It binds the cell producers, four checked indexes,
discriminator and payload reads, type witnesses, every result field and the
returned trace. It additionally invokes the existing exact Text comparison
verifier for the Dictionary type gate. The only alpha normalization covers
producer/control UUIDs, eight source-owned generated slot names, and output
display names. Native parameter wrappers, scalar types, absent parameter maps,
ordering and branch boundaries remain exact.

The parser in tagged_carrier_observation.py verifies a supplied observation
line. Parsing alone authenticates neither source nor Apple execution. Its
result always retains those distinctions.

From this directory:

    python3 -B -m unittest -v test_tagged_carrier_graph test_tagged_carrier_observation

To check a newly produced unsigned artifact against the fixed source contract:

    python3 -B tagged_carrier_graph.py \
      --source fixtures/qa-tagged-value-carrier-v2.cherri \
      --expected fixtures/qa-tagged-value-carrier-v2.expected.json \
      --native /absolute/path/to/unsigned.original.plist

The first graph-verifier draft incorrectly required a parameter map on sixteen
parameterless lowerer Nothing actions. The correction models their omission
exactly and preserves the separate identity-bearing DebugClearLogs Nothing.
The first mutation test harness also used Python structural equality, which
equates False and integer zero; it now compares serialized typed plist hashes
before passing each mutation to the unchanged validator. Both original failed
runs and their source bytes remain in the external evidence store.

The completed source controls passed 102 named tests, including 88 mutations
of the actual compiler artifact. These are graph/parser controls, not native
runtime observations.

## Native continuation

The new fixture ID is qa-tagged-value-carrier-v2. It is intentionally unenrolled
in the historical nested-list probe contract. The original native owner must
bind a reviewed compiler source, source file, unsigned and signed artifacts,
same-device import/run, callback nonce, exact semantic line and before/after
trace envelope before any execution claim.

Do not substitute a rendered payload for a type witness, alias this fixture
to a historical nested-list ID, or infer a successful native run from the
compiler or graph controls. A decoded Text carrier, absent witness or missing
callback remains a real unsuccessful observation.
