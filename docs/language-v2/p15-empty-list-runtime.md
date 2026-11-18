# Authored empty List runtime representation

## Native finding and correction

The signed differential run [44346443743](https://github.com/davidpovarsky/cherri/actions/runs/44346443743), built from `4e7956d2519307861fe703a11fbe401fc9faca05`, returned `direct=1:repeat=1:cleared=0:emptyText=1`. Its unchanged expected result is `direct=0:repeat=0:cleared=0:emptyText=1`. The detailed bounded audit is in `docs/apple-runtime/p10r15/empty-list-if-result-4e7956d-audit.json`.

Within that invocation, the native List action with zero rows did not provide the required zero-item value. The existing Legacy mechanism, Nothing followed by Set Variable with no input, supplied a zero-iteration Repeat control. This observation does not identify the type or content of the unexpected List item.

The V2 lowerer now reuses that existing clearing mechanism for authored runtime `[]`:

| Source use | Native representation |
| --- | --- |
| Mutable empty binding or empty assignment | Nothing, then Set Variable with the effective scoped variable name and no `WFInput` |
| Immutable empty binding | Clear a fresh private variable, then the existing Get Variable bridge with the binding's concrete producer UUID |
| Temporary empty value, including a direct Repeat input or branch yield | Clear a fresh private variable, then the same Get Variable producer bridge |

Nothing is required so an earlier native action cannot become the implicit Set Variable input. Each private variable uses the existing lexical name allocator and the result UUID; it is cleared once and is not exposed as an authored source binding. Consumers retain the exact producer UUID and output name. Repeat result consumers still reference the native End Repeat result, without a collection accumulator or body rerun.

This correction matches runtime list values only. Inline Dictionary arrays and exact `native.plist` data continue through their existing codecs. A singleton List containing empty Text remains one native content row. The existing analyzer still rejects `count([])` and `count(input:[])` because those arguments require an attachment; an authored binding is the supported route. Explicit nil assignment and uninitialized declarations are outside this change.

## Causal verification

`p15_empty_list_test.go` compiles actual source through the production compiler and independently follows the serialized Count/Repeat/output references. Its controls cover immutable and mutable bindings, assignment, snapshot lifetime, independent private values, direct and bound Repeat inputs, Return's existing single-token attachment, and branch-final yields. The Return graph check is an identity check, not a claim about native cardinality after the Output action.

Six corruptions must be rejected: missing Nothing, injected prior input, wrong variable identity, missing producer, wrong output name, and a raw empty array substituted for the attachment. The original signed four-count source is pinned by SHA-256 `15d1e0ab8bae11e87cc91d81ea1a9a3cc0466429956ad7bff9b6b18570854a3a`. Its structural oracle now requires the corrected 31-action graph, all four independent Count edges, the original cleared-variable control, the singleton empty-Text control, and the same final summary. Twelve mutations of that graph must fail.

XML and binary native inputs are imported and recompiled in hybrid and lossless modes through both dialects. Exact native trees and the baseline classifications are retained: the captured native List with empty rows remains `EXACT_NATIVE_FALLBACK`, its Get Variable consumer remains `NATURAL_VERIFIED`, and the opaque payload remains exact. A separate dual-dialect pre-overlay verification requires zero demotions. These checks do not change native imports into authored zero-item semantics.

The final tests also run with only the four production hooks reverted through a Go source overlay to the parent source. That causal run must fail while the Legacy, singleton and exact-import controls continue to pass. The corrected graph and compatibility selection then pass without the overlay. The shared test reader now accepts an absent parameter envelope for valid actions such as Nothing; a present malformed envelope still fails its type assertion.

## Remaining native gate

The new representation requires a fresh signed run on its immutable repaired source. Local structural verification is not native runtime proof. The next scoped selections are the unchanged `qa-collect-empty-split` (exact `0/0/0/1`) and `qa-collect-counts-v2` (exact empty 0, singleton 1 with its item, and 100 ordered numeric results from 0 through 99). Existing callback nonce, source/signing provenance, trace and publication gates remain mandatory.

No new runtime success is asserted here. L01's standalone nested List hierarchy remains unverified; this empty-value repair does not establish nested List preservation. Inspector completeness, physical-device execution and overall release completion remain separate gates.
