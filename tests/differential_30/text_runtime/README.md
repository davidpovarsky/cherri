# Supplemental native text-state case Q41

This independent fixture exercises authored dynamic text parameters neighboring
the URL component repair. It is separate from the main 39-case manifest and its
generator. The source files and exact expected output are committed before any
baseline or candidate compilation or native execution.

## Fixed oracle

    CHERRI_QA_41|ALPHA:beta:Alpha:alpha::BETA:example.org

The result has 53 UTF-8 bytes, with no trailing newline. SHA-256:
`7d750a1f29982ee8787c2fcba06550b132bc971518a009d0a9b2e80ce42caed5`.

The mutable Text `alpha,BETA` is split using a comma produced by a real Text
action. The first and last list items produce `ALPHA`, `beta` and `Alpha` through
uppercase, lowercase and capitalize. A separate Text action produces `::`;
joining the original two-item list gives `alpha::BETA`. A mutable Text containing
one URL is passed to Get URLs, then Get First Item and Get URL Component (Host),
which yields `example.org`.

These expectations are literal string operations and URL syntax, independent of
compiler output. All inputs are ASCII and embedded. URL detection and component
parsing do not open or expand the URL. There is no network request, spellchecker,
external input, Clock action, alert or Show Result. Output is the only terminal
action.

## Reference forms covered

| Authored slot | Reference producer |
|---|---|
| splitText text | Mutable Variable sample |
| splitText separator | Text ActionOutput containing comma |
| uppercase/lowercase/capitalize text | Text ActionOutput preserving Get List Item |
| joinText glue | Text ActionOutput containing double colon |
| getURLs input | Mutable Variable linkText |
| getURLDetail url | Text ActionOutput preserving the detected URL item |

The primary joinText input is a list and must retain its attachment encoding.
The other listed slots are declared Text and have Apple VariableString native
state, whose canonical reference serialization is a token string. Dynamic
reference shapes must be checked against that independent native state and
producer identity; a successful compile alone is insufficient.

The original and fork Legacy files differ only in the workflow name. Native
names are `CHERRI_QA_Q41_O`, `CHERRI_QA_Q41_L` and `CHERRI_QA_Q41_V`.

## Compile and native handoff

Use `tests/differential_30/run.py` with the separate manifest at
`tests/differential_30/text_runtime/manifest.json`, pristine original 98d5191,
and the exact candidate fork binary. Keep the 4ca baseline in a separate output
directory. Reuse unchanged source and original compiler receipts when possible;
never replace a failed baseline or edit a produced plist.

At the initial fixture commit, compilation and native status are both NOT_RUN.
Later verification records must preserve this predeclared oracle and bind the
actual source/binary hashes. The native owner signs untouched compiler bytes,
imports the three unique names and requires fresh callback identity, exact raw
UTF-8 SHA-256, byte length and result summary. No native status is inferred from
structural equality.

## Common-language source revision

Initial fixture/oracle commit `9e6de5f5c85ceac295c15c1ecef3603561102370`
was preserved and compiled. Both fork frontends accepted it; pristine upstream
98d5191 rejected the direct generic `getFirstItem()` result supplied to a Text
parameter. Its actual diagnostic and exact source copies remain in the external
`text-runtime/candidate-3ec9e5ed/` receipt.

All three source lanes now insert explicit Text interpolation producers for the
first item, last item and detected URL item before the corresponding Text
consumers. This adapts the fixture to upstream's existing type contract while
preserving every value and all eight targeted dynamic parameter slots. The
independently committed 53-byte oracle is unchanged. The original and fork
Legacy source bodies still differ only in their declared workflow name.

`verify_contract.py` uses captured Apple VariableString/Variable state samples,
checks each source producer or mutable initializer edge, checks the three Text
materialization edges, and retains a separate attachment control for join's
primary list input. It reads compiler-produced artifacts without modifying them:

    python3 -B tests/differential_30/text_runtime/verify_contract.py RUN_DIR --out REPORT.json

The earlier source and each later source revision use separate evidence output
directories; no prior compiler failure or baseline is overwritten.


## Recorded structural verification

The common-language source revision is commit
`fde7d101fcd1bf28854a99bd4e67df0ca861ebb2`. The fixed oracle remains identical
to the initial fixture commit. All compilation below uses actual CLI commands
and retained unsigned compiler output; no native plist is patched.

| Compiler lane | Compile / graph | Eight Text slots | Primary list attachment |
|---|---|---|---|
| Fork baseline 4ca75e76, Legacy | 20 actions / valid | 8 failures | Pass |
| Fork baseline 4ca75e76, V2 | 20 actions / valid | 8 failures | Pass |
| Pristine upstream 98d5191 | 20 actions / valid | Pass | Pass |
| Fork candidate 3ec9e5ed, Legacy | 20 actions / valid | Pass | Pass |
| Fork candidate 3ec9e5ed, V2 | 20 actions / valid | Pass | Pass |

The independent checker exits 1 on the matching-source baseline and 0 on all
three candidate lanes. It also verifies the Text materialization reference
edges. Every baseline failure is the wrong attachment envelope in a declared
Text slot. Raw typed comparisons are retained: Legacy's action tree matches
upstream after UUID alpha-renaming; V2 retains two action-level setvariable
serialization-key differences and a paired generated Text output display name
difference. Workflow metadata differences remain in both reports. None is
silently removed from the comparison or treated as runtime proof.

`verification.json` binds the exact source, binary, checker, Apple state sample,
artifact and command-result hashes. External evidence under
`cherri-differential-30-20261125/text-runtime/` retains both the initial rejected
upstream compile and separate revised-source baseline/candidate receipts.
`runtime-preparation.json` there lists the three unsigned native inputs and the
fixed output digest. At this structural handoff, native status is NOT_RUN; the
native owner records subsequent execution separately.
