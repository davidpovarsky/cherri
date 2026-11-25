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
The corrected preparation still requires its own actual result.

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
