# Function envelope presence diagnostic — preparation only

The RP26/BRG15 completion work first checks whether the current legal sender
preserves the caller-input key when a function receives no external input.
The current dispatcher tests two header markers before selecting a function;
this diagnostic does not repair or certify complete-envelope admission.

## Original preparation and source correction

The first public preparation at 7bc245fa2f4b2d7b46a0c1eea0752e0c519e1e34
failed before plist export. Three calls attempted text(Number) and received
E_ARGUMENT_TYPE. Its original source, process receipts and diagnostics remain
under mac-function-presence-preparation-01/original-intake-01 in the Apple
evidence directory. No signed or unsigned shortcut was produced by that run.

Commit b82bd63dad0b72078d98d57438a61b463b7dcff7 changes only those three count
conversions to explicit f-string interpolation and updates the exact source pin.
The sender checks, eleven corruptions, test selector and budgets are unchanged.
The corrected preparation passed all twelve named checks on Darwin/arm64 in
18.115 seconds of owner time; no test was skipped. Its compiler export is the
unaltered 269,792-byte XML plist with SHA-256
54889840e546acc60ffe05c8399e1d4bd54f0dc846ec3fd520b4fffc0aa6c7d0.

The three diagnostic rows cover a zero-argument call, an omitted default, and
an explicitly supplied Optional absence. The source checks key existence before
reading caller data, and uses UNOBSERVED when the key is absent or a type read
is not permitted. It separately observes raw child-envelope caller data and
the restored system.input.

## Bounded observation parser

function_envelope_observation.py accepts exactly the three ordered public
rows. Canonical counts, bounded ASCII transport, strict UTF-8/base64, conditional
presence fields, and separate empty/default/Optional observations are checked.
An absent caller key and wrong parameter observations are retained as negative
results, without rewriting them into empty values or valid defaults.

All fourteen Python control parents passed in 0.219 seconds of original process
time. They include causal changes to every declared no-input goal, malformed
transport, omitted/extra/reordered fields, canonical base64, absent versus empty,
and real type witnesses. The receipt is
/mnt/development/projects/evidence/cherri-p10r15-ci-release-20261125/function-observation-parser-01/result.original.json
with SHA-256 02449fd8a971a6459ed1db58b30915058f51f4c653c59847bda90ee6860998e9.

This parser authenticates neither source nor execution. Every result explicitly
leaves source, launch input, Apple execution and formal acceptance ungranted.

## Required execution boundary

The existing QA launcher normally supplies CHERRI_QA_INPUT as Text. That launch
cannot establish the no-input hypothesis. The diagnostic needs a separately
reviewed first and only semantic invocation without input, with no fallback that
changes input. The complete compiled action/trace graph, source and signing joins,
original callback, actual launch arguments and native outcome must be checked
before any result can support the function transport repair. Compilation or
parser controls alone grant no RP26/BRG15 or Apple runtime acceptance.

## Complete compiled action and trace contract

The independent expected program describes all 379 actions from the fixed public
source and the legal v2 sender contract. It covers all three function selectors,
default/Optional paths, exact payload fields, empty zero-argument mirror, caller
snapshot and restoration, twelve exact key comparisons, three key-guarded raw
caller reads, separately sourced counts/types, and every returned CASE field.
Both inline traces, collector envelopes, three self-call targets and final result
ordering are included. Only UUID alpha identities and display names are normalized;
native parameter shapes, value types, literals, branch nesting and producer edges
must agree exactly.

The original export matches that full graph. The graph and parser control suite
then passed all 26 parents and 765 subtests in 9.092 seconds, with no skips.
These controls include altering each of the 379 action identities, all exact key
comparison operands/escaping/counts/polarity, caller/payload/default/Optional edges,
trace provenance, typed Boolean/Number distinctions and malformed XML duplicate
keys. No compiler or native runtime was executed by these Python controls.

Two development failures remain in the evidence: the first expected-graph builder
escaped the timestamp attachment marker instead of retaining literal U+FFFC; the
first corruption precondition used Python equality, which equates Boolean and
numeric values. The builder now preserves the exact native marker, and the
precondition compares typed JSON bytes. The original fixture, original plist,
all fourteen Boolean/Number corruptions and graph validator semantics were retained.

Original evidence:

- Independent b82 preparation intake:
  /mnt/development/projects/evidence/cherri-p10r15-ci-release-20261125/function-presence-original-intake-02/result.original.json
  SHA-256 42bf246eff6e2f1ce2420732e30639a397c022182794e01b3bff4a1889c9f2a2.
  It rehashes all 59 transported originals, joins the exact twelve run/pass events,
  all 7,609 source snapshot paths to Git tree metadata and six direct source blobs,
  and verifies the single original base64 export equals the retained plist.
- Full graph validation:
  /mnt/development/projects/evidence/cherri-p10r15-ci-release-20261125/function-presence-full-graph-preparation-01/validation-02/result.original.json
  SHA-256 a33d54bb48821c63fca8049b906918332fa48ac199f87764b16872fa386d6333.
- Full parser/graph controls:
  /mnt/development/projects/evidence/cherri-p10r15-ci-release-20261125/function-presence-full-graph-preparation-01/controls-02/result.original.json
  SHA-256 bb7d2f816cd8a3dc29afd883df3cb17502eee4d68f3af5436e811be182c3ce9b.

The graph deliberately preserves and identifies the current incomplete dispatcher.
It proves what this diagnostic will ask the runtime to do. It does not establish
how Apple serializes a Nothing caller into the legal payload, does not grant a
no-input launch, and does not satisfy the complete RP26/BRG15 scenarios.
