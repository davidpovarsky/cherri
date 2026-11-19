# Export and Clipboard causal characterization

The original S3 Unicode and binding probes returned correlated callbacks with
incorrect semantic output. This public diagnostic distinguishes the remaining
boundaries without changing those original fixtures or acceptance oracles.

The export path reports the actual entries count and separately bounded first
and last payload bytes. Historical [run 44466065158](https://github.com/davidpovarsky/cherri/actions/runs/44466065158)
used the former empty Text seed and observed two entries after logging one Text,
with the first payload empty. The integrated [6d9ff8ac repair](https://github.com/davidpovarsky/cherri/commit/6d9ff8ac)
now emits Nothing followed immediately by Set Variable without WFInput. Current
graph controls require that exact zero-item clear and reject the former seed.
An intentionally logged empty Text is preserved; no blanket empty-item filter
manufactures the intended count. New native execution remains required.

Type observations compare the artifact against independent one-byte Base64
Decode, Text, and Dictionary witnesses. TYPE_FILE means equality with the
decoded binary witness; FILE_TEXT and FILE_DICT expose whether that witness
itself collapses to either other type. No localized Apple type name is assumed.

A separate supported .file control observes coerced type parity, exact encoded-byte equality with the uncoerced artifact, and the reconstructed Name plus File Extension. It does not replace the uncoerced observations or change export production code.

The unchanged public binding body runs once. Four separate routes observe its
result: direct result Text, Get Variable with a Clipboard token, Base64 with a
direct Clipboard token, and the canonical Get Clipboard action. Each route
reports only actual encoded length and equality to independently authored public
bytes. Arbitrary Clipboard content is never included in this diagnostic output.
Three further flags compare the captured number and both Clipboard aliases to
the original known values.

The strict callback oracle retains the intended one-entry/public-byte result.
Every observation is mandatory; a diagnostic mismatch remains FAIL_RUNTIME.
A passing characterization would still not replace the original Unicode or
binding acceptance probes, nor establish physical-device behavior.

Local validation must use an actual source-bound compiler, verify the real
emitted producer/consumer graph, and reject damaged graph controls before this
single registered selector is dispatched. The source hash and original-body
hashes are checked in the production compilation phase. This helper provides no
Apple execution claim from its local PASS result.

## Retained causal observations, 19 November 2026

Run 44466065158 also observed TYPE_FILE=N, TYPE_DICT=Y and COERCED_FILE=N.
The decoded witness comparison therefore remains a failed File boundary.
The successful name/byte control in run 44429796917 compared type before and
after renaming the same decoded JSON value; it cannot independently establish
that value as a generic File. The acceptance oracles retain the separate witness.

The independent Get Variable Clipboard observer returned GETVAR_LEN=0, while
the canonical Get Clipboard observer returned ACTION_LEN=108. The immutable
Clipboard lowering and binding/defaults QA observer were repaired to use the
canonical parameterless Get Clipboard producer on candidate b2961bbd. The four
explicit observers in this causal diagnostic remain intact, including the known
empty Get Variable route. Thus this diagnostic may still fail even when the
original binding acceptance probe passes; it is not the acceptance replacement.

The original binding capture body and expected result remain byte-for-byte
unchanged in [run 44608110760](https://github.com/davidpovarsky/cherri/actions/runs/44608110760).
Only completed source-bound native receipts can establish its result. Physical
iPad behavior and the independent export File boundary remain separate gates.
