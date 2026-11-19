# Forwarded zero and Dictionary File carrier: bounded diagnosis

Current compiler source: `71225b71f5353a0f661722d154a8b670b52e76f6`.
The diagnostic used the normal-clone Windows compiler already built from this
source, SHA256 `3159baacfdb92c67afdeb9ede5a85f1ee6c7d175005647bd729c7a67a8ea1e06`,
with an embedded clean VCS revision and retained compiler-cache receipt. No
compiler source or production comparison policy changed during this diagnosis.
The three sources were extracted directly from the immutable source commit.

## require(false), zero presence and paired numeric comparisons

Original run [44342588798](https://github.com/davidpovarsky/cherri/actions/runs/44342588798)
on a7d9b60 reached its correlated final callback with `P15_REQUIRE_ZERO`.
The fixture's later false, text, dictionary, singleton-list and File checks did
not replace that verdict. This does not establish a Boolean runtime type claim,
but there is no observed false-specific rejection in that completed fixture.
The zero-only equality diagnostic on the same source passed literal, direct,
required and mutable-binding paths in run 44333755039.

The paired diagnostic [44386191992](https://github.com/davidpovarsky/cherri/actions/runs/44386191992)
on 42e7d1c independently found:

| Value route | equals zero | not equals zero |
|---|---:|---:|
| Direct Number(0) | true | false |
| require(0) forwarded by Get Variable | true | true |
| Mutable binding of that same result | true | true |

The nonzero operator controls passed. Its original wrong-result callback and
strict `FAIL_RUNTIME_PRESERVED` audit remain in
`../../evidence/number-comparison-42e7d1c-r2/audit.json`.

On current S71225 the original 89-action graph still binds each pair to exactly
the same producer, emits distinct native codes 4/5, writes numeric 0/1 fields, and
preserves each independent outcome flag. Six causal source-graph corruptions
were rejected. `require_macro.go` and the shared conditional wire builder are
byte-identical to 42e7d1c. This evidence does not justify swapping operators or
changing the original expected result. The unresolved boundary is native
comparison behavior after a generic Get Variable output/binding.

The existing separate 89-action explicit Number control preserves direct and
nonzero controls and changes only four forwarded/bound operands to the exact
`WFNumberContentItem` coercion. Current-source graph checks confirm those four
coercions, both paired producer identities and native codes 4/5/4/5; removing a
coercion, substituting Text coercion, or corrupting an operator is rejected.
The earlier run 44408371761 had no target callback and cannot answer whether this
control repairs native behavior. A fresh exact-S71225 run was dispatched once:
[44436273586](https://github.com/davidpovarsky/cherri/actions/runs/44436273586),
with required runtime, original strict expected flags and normal NDJSON trace.
Its first observed state was queued with zero steps. This is a fresh source-bound
observation after the earlier infrastructure failure, not a claimed general
XCTest cleanup repair. No production coercion change is made before its result.

## File label carrier through Dictionary set/get

Original independently separated control
[44333772194](https://github.com/davidpovarsky/cherri/actions/runs/44333772194)
on a7d9b60 returned exactly:

`CHERRI_QA_LABEL_FILE_CONTROLS:B=ok:V=ok:VT=ok:L=ok:T=bad:R=bad:S=ok`

Baseline bytes, Get Variable bytes/type, label and original-source preservation
passed; only restored type and restored bytes failed after the Dictionary
carrier. The original failure is retained in
`../../native-diagnostics-a7d9b60-audit.json`. This is separate from the catalog
callable `labelFile`, which labels a file's Finder color; this probe concerns
proposed debug-log label representation.

The current-source 70-action graph still forwards the exact transported File
producer into `WFDictionaryValue` through a `WFTextTokenString` attachment. The
canonical Apple metadata describes that parameter as `WFTextInputParameter`;
Get Dictionary Value advertises String/Number/Date/Dictionary/Boolean output
classes and does not declare File/Image. The evidence supports an unsupported
typed carrier boundary, rather than a broken Get Variable UUID or original File
mutation. It does not identify the exact native conversion from the sanitized
callback, and no guessed restored bytes/type spelling is substituted.

A universal typed log must not adopt this carrier unchanged. A raw-object log
with primitive label metadata associated to actual item ranges, or an explicit
per-type byte codec with defined reconstruction semantics, needs its own
cardinality and native identity tests. Neither has been implemented or promoted
by this diagnosis. Existing F04 probes separately test nested/empty collection
cardinality; their results remain independent gates.

## Scope of this archive

Three actual CLI compiles, immutable source hashes, exact unsigned bytes,
compiler receipt, metadata and local graph checks are retained. All data is
synthetic. The new local diagnosis executed zero Apple shortcuts. Historical
native failures remain failures; current native behavior awaits the required
callback. There is no physical-device, generic typed-carrier, label-feature or
complete require acceptance claim.
