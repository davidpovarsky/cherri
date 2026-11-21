# Imported native append: causal repair and bounded evidence

Compiler source: 9c8daf721aa241127df8d1e3f91bbadf90110e73. Base: 61614095633e50cbc5377d36ca01061a4a36a100. This packet proves the bounded compiler repair and its measured regression coverage. It does not prove the five private-corpus gates, full Go, Apple runtime, consumer publication, or acceptance of the 179 requirements.

## Observed cause and repair

The real Variable import carrying a boolean AppendVariable marker lost that marker in analysis. Public direct and coerced Dictionary-key append reproductions failed on the tests-only source b45351998504275ff761e130af828fcaff7bbd22 with E_ARGUMENT_TYPE. The fixed analyzer preserves the marker only for an actual valid Variable native binding and scopes the exception to +=. Authored Optional and other arithmetic operations retain their checks.

Before assigning compiler-owned Optional presence, the lowerer refuses demanded native append when the RHS is not DefinitelyPresent. This covers both explicitly optional values and opaque AnyContent. The conservative demand analysis is retained; it is not claimed to identify only demands that execute at runtime. No presence-bit protocol or contract was relaxed.

The two public native graph oracles compare the entire four-action graph, all envelopes, labels, key/coercion ordering and edges. Only two validated distinct nonzero UUIDs may be consistently alpha-renamed. The suite includes 37 graph corruption controls, marker/type boundaries, aliases, shadowing, other operators, valid tagged prefixes and opaque presence counterexamples.

## Actual clean results

- Focused clean source: 77 case terminals PASS, no FAIL/SKIP, 89.125 seconds, Go budget 3 minutes.
- Optional regression on the same exact clean source: 1,763 case terminals PASS, no FAIL/SKIP, 115.781 seconds, original Go budget 6 minutes, both package terminals PASS.
- The cold-cache compile-only preparation completed in 368.015 seconds and is explicitly not a test result.
- Every started test case in the retained streams has exactly one terminal; parent counts and original events are enumerated independently in the manifest.

## Contrary originals retained

The tests-only reproduction failed. The analyzer-only development attempt incorrectly accepted a demanded tagged Optional; its original invalid alias keyword parser error is also retained as a test-authoring error. The first lowerer demand guard passed its then-existing checks but failed four opaque-presence cases added afterward. The final source uses !DefinitelyPresent. None of these development streams is relabeled as a clean source pass.

All public original stdout/stderr, receipts, source diffs, public plist outputs, the broad-regression launcher, and five exact Git source bindings are retained byte for byte in originals.zip. Original private input files, private generated source and private diagnostic payloads are not included.

Archive SHA256: 10bf64041db3cda207eb5de21e5dc72660e0c7b0dc7dfd8ce209ddbd1dae6bb1

## Pending acceptance work

Run the full five private gates on this exact source with unchanged budgets and the same original 38-file input manifest. Then assess consumer pins and fresh integrated CI separately. The last full private result on 61614095 remains three PASS and two FAIL; it is not inherited by this source. No formal requirement or project closure is claimed by this packet.
