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
| uppercase/lowercase/capitalize text | Get List Item ActionOutput |
| joinText glue | Text ActionOutput containing double colon |
| getURLs input | Mutable Variable linkText |
| getURLDetail url | Get List Item ActionOutput containing the detected URL |

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
