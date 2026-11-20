# P15 Optional If/Menu value joins — verified local compiler component

Date: 2026-11-20 (UTC).

The current integrated-source regression is repaired in **11a4df6c8595ff7bc0df83db11ca6caa6bc10fd9**. An actual clean replay passed **2,599 named root tests** and **184 named tests across six language packages**, with no failures or skips. These are local analysis, compiler and complete native-structure checks. They do not establish Apple execution or final acceptance of the 179 requirements.

## Source and retained RED

The immutable first tests-only source is c68a7362a156e9999ee46e64f2d7f70710a13775, based on the earlier T01/FN04 component. Its actual run failed 29 positive leaf cases (31 named failure events including their parents), while five named negative/unconsumed controls passed.

The identical test file was then cherry-picked onto the current integrated source 2906245c34926e69384ec2c3d7923b0595421500, producing **d2bd7958f89d36fb5ab6f75a4a94f0a5817d18ad**. This second clean baseline reproduced the same failures in 19.781 seconds. The original branch and both original raw streams remain unchanged.

| Attempt | Actual result | Raw SHA256 |
| --- | --- | --- |
| Original c68 baseline | exit1; 5 named PASS, 31 named FAIL; 131.485s | 34c811db60f2a8f37698045224f28ad757eec08cbf44aecc4109444e0014f431 |
| Integrated d2 baseline | exit1; 5 named PASS, 31 named FAIL; 19.781s | 5c4125037ebe972b9be4734470f7c07c9f096d0ea68fc49f1657837a075e955a |
| Clean repaired root selection | exit0; 2599 named PASS; 61.360s | fef0a211778d231784e0c55cb2ec0bfb8e2139a215ca1b20f9ad1318a0bda314 |
| Clean repaired six packages | exit0; 184 named PASS; 9.969s | a15d5ff75b8f3fc8f7e1cec41c373edd5d22c0be3a4663f4c187c9198410473f |

The baseline demonstrated two distinct compiler gaps. First, literal None plus a present Text branch retained a spurious unknown present inner type, so valid Text? annotations, returns and arguments were rejected as Unknown? | Text. Second, normally completing If/Menu branches supplied native content without an independently transported presence flag. An empty present value could not safely be distinguished from absence by downstream Optional consumers.

The initial marker-only oracle was intentionally a boundary specification. The new complete oracle proves that merely retaining both flags is insufficient: swapping the two branch values still passes the original boundary check and is rejected by the complete source-authored graph.

## Repair and exact scope

The analyzer now combines evidence from normally completing branches. A literal or path-local known None contributes absence without inventing an unknown present inner value. Other Optional inner types are retained. Opaque native Unknown/AnyContent does not acquire compiler-owned presence. Unconditional function-return paths do not contribute a join value.

The lowerer emits a private Dictionary carrier only for a nullable control value whose source dependencies have a presence consumer. The carrier has marker _cherri_optional_join_v1 and Text has_value equal to 0 or 1. Only a present carrier has a value field. An absent carrier does not fabricate empty Text, JSON null or a plist null value. Map/List literals retain their semantic AST identity and explicit empty fields inside that private carrier.

The selected native control end joins these carriers. One decoder then reads its presence flag and returns either the selected value or an explicit Nothing action. Early returns retain their existing function-result carrier and Output operation; they are not reclassified as normal join yields. The active yield-carrier scope is restored on every return from lowering.

Demand follows aliases, assignments, user-function arguments, returns and None comparisons. Exact positive and negative demand facts are copied with scoped AST renaming. Unconsumed Optional joins retain their original native last-output/clear representation.

The verified new cases include:

- Empty Text, false, zero, empty List and empty Map, each through If and Menu and both branch orders.
- Typed annotations, returns, direct/inferred/aliased arguments, reassignment and nested If/Menu values.
- Five early-return shapes, including nested controls and the alternate branch.
- Full producer/end/value/presence/consumer edges, branch data and representation types.
- Authored branch effects and argument evaluation in written order, including reversed named argument order.
- Opaque output and wrong-type rejection, analysis reuse, and exact scoped demand retention.
- Causal corruption of flags, value kinds, branch ownership, raw result edges, early outputs, clears, action count, effect values and effect order.

The 522 new named root events and 11 new package events run alongside the previous 2077/173 affected cases. These totals are test events, not additional formal requirements.

## Existing F03 characterization and oracle corrections

The existing TestP15F03NoneYieldClearsPrecedingOutput source is unchanged. Its type expectation is now precisely Text? for a known None/Text join, rather than Unknown? | Text. It still explicitly requires MayBeAbsent. All existing native NOISE→Nothing clearing checks, original source variants and the mutation that removes the clear operation are unchanged and passed in the clean replay. The new unconsumed join controls also reject presence metadata that would alter the previous output.

The first exploratory command accidentally anchored the new test prefix and therefore ran only the old F03 case. That failed result is retained and was never counted as a join execution. The corrected family run established the new boundary behavior while the old type characterization still failed.

Four later exploratory effect tests incorrectly required UUIDs on authored Text statement outputs that were not consumed. The native statements and their exact payload/order were present. Only those unused effect observations changed from the oracle's value-producing helper to its ordinary emit helper. The effect deletion, duplication, movement and wrong-payload controls remain and pass. The original failed raw stream and exact pre-correction test bytes are retained.

The broader dirty exploratory selection also ran unrelated K05/K03/stdfunc tests. K05 revealed the copied Skill fixture's missing json-object.awk dependency, K03 correctly rejected dirty source, and std/Std hit the separately assigned JavaScript expectation issue. That run remains FAILED. Root owns those fixes; this component does not claim to repair them.

## Actual clean commands and environment

Executed at exact source 11a4df6c8595ff7bc0df83db11ca6caa6bc10fd9:

```text
go test -count=1 -json -timeout=5m -run '^TestP15(S01|T01|FN0[1-5]|Function|Optional|C01|EmptyList|L01|Computed|SemanticContainer|ExplicitNativeContainer|F03NoneYield|V02|Nil)|Test.*BuildProfile|Test.*Release' .
go test -count=1 -json -timeout=3m ./internal/language/syntax ./internal/language/analysis ./internal/language/lower ./internal/language/service ./internal/language/types ./internal/language/backend
```

The process wrapper kept finite outer budgets of 360 and 240 seconds. Neither run timed out. Both checked a clean source tree before and after. Go was go1.25.6, windows/amd64, CGO_ENABLED=0 and GOMAXPROCS=2. No race or Apple claim follows from this environment.

## Codec and untouched boundaries

The codec family remains v2.2. Its descriptor gains one explicit optional-control-join policy line because the private emitted representation is now specified. The complete existing function/Optional transport and expanded S01 policy lines remain present. Descriptor fingerprint: **f919ed42a667478072243c81d233e4f9608455e3421ff2f59b0e275574bbb534**. The final compiler/docs/Skill consumer tuple must be refreshed and verified by integration; this component does not edit consumer pins.

The evidence source manifest binds every changed file to its immutable Git blob and checks unchanged original 179 contracts, native templates, runtime fixtures and formal registration boundaries. No requirement text, original fixture/golden, current signing policy or acceptance status was weakened to obtain these results. Shared native serializers and the S01 carrier implementation are unchanged by this component.

## Evidence and remaining work

The adjacent evidence directory preserves all 323 original files from the owned baseline/draft/clean run directory, with byte-for-byte copies and hashes. It also contains the exact draft oracle bytes, immutable source witnesses, full and production-only patches, command/event audit and environment receipt. The original receipts' paths and contents were not rewritten.

Evidence manifest SHA256: **87fe2c0549ea88ce14281e3c9909e851e6c3325234cb9feea143942e51ecdca8**.

Evidence directory: evidence/p15-optional-value-joins-20261120/.
Original directory: C:\Users\DAVID\Code\cherri-p10r15-r3-results-20261120\dist\p15-optional-value-joins.

Read-only peer review of exact 11a4df6c found no concrete branch/type/demand bypass in the inspected paths. This review is distinct from the recorded executions.

No Apple run is attributed to this commit. The separate earlier Optional diagnostic was blocked at import before fixture execution and supplies no semantic success or failure for these joins. Arbitrary opaque absence, ordinary container-field None/JSON-null transport, and unproved nested native-content behavior remain separate boundaries. Normal branches without an explicit terminal yield or a supported tail If/Menu are still rejected by this private carrier route; this is a precise current implementation boundary, not proof of intrinsic native unencodability.

This change does not itself close T05, L01, FN05, RP20, RP23, the full179 report, CI, deployment or installed consumers. The actual next full acceptance run must use the integrated source and current consumer tuple.

Asana local evidence stage:1220733935178043, child of1220432417414104. The parent description reached the service size limit, so all previous notes were preserved and the verified local stage is recorded in the child. Parent acceptance remains open.
