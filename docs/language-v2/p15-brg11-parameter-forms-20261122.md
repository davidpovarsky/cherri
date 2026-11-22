# BRG11 parameter forms and typed defaults

BRG11's complete local scenario passed on source
`f1a753096cdf5d00bb969986b27776ca78410fa6`, based on RP25 proof
`8153099d002dceb55d1937c95ecd22cd1272b392`. Its immutable `ios-runtime`
tier remains **PENDING_EXTERNAL**. No compiler behavior, acceptance contract,
minimum tier or historical result changed.

## One joined canonical source

The source defines one function with Bool, Number, Text, List and Map parameters,
then compares three calls: all arguments omitted, explicit false/zero/empty
values, and the same explicit values supplied with reordered named arguments.
Its result Dictionary retains every actual typed result. The same source then
uses the canonical backend for these distinct native parameter routes:

| Route | Authored input and expected structure |
| --- | --- |
| Literal-only argument | Folded `"Oc"+"ean"` emits the exact Ocean App Intent value |
| Enum member | `.Rain` resolves to the exact canonical Rain value |
| Reference-only token codec | Empty Text materializes once before `getText` consumes its reference |
| Existing reference | A distinct Text producer supplies `getText` through its exact attachment |
| Numeric-text codec | `number("-5.125")` retains that exact string; `round` consumes the Number producer |
| Named variadic List | FIRST, the dynamic Text reference, and LAST retain cardinality, order and item kinds |
| Final observation | A Dictionary contains all actual function and native results, followed by explicit output |

The whole 77-action graph is checked from the dispatcher header through the
five presence branches and typed defaults, the child return, all call envelopes,
native arguments and final consumers. Every field, scalar kind, argument slot,
group, authored name and consumer edge is specified. Generated UUID identities
alone are learned from the emitted graph. Existing strict FN05 and RP25
expectation primitives are reused without rerunning their completed scenarios.

Sixteen exact invalid-source controls cover literal-only binding/runtime misuse,
invalid enums, numeric Text without conversion, extra positional arguments,
duplicate and unknown labels, missing required input, reserved function slots,
duplicate variadic packs, and wrong defaults for all five types. Each requires
the exact diagnostic code and span and no emitted artifact. The registry and
typed-program checks execute in the same semantic parent.

Sixty-two native corruption controls include all five default slots, independent
supplied flags, explicit versus omitted values, fallback kinds and joins. They
also break each native route, ordered variadic elements, structured child output
and final result references. Structural controls reject missing/extra actions,
invalid or duplicate producers, invalid groups and changed native control codes.

## Observed execution

The clean replay ran on 22 November 2026, 14:22:27–14:22:30 UTC:

| Scope | Observed result |
| --- | --- |
| Outer producer and intake controls | 9 PASS, 0 FAIL, 0 SKIP; 2.931855452 seconds |
| Semantic scenario and invalid-source controls | 17 PASS |
| Complete native graph and causal corruptions | 63 PASS |
| Nested production process | 80 PASS, 0 FAIL, 0 SKIP; 2.098238013 seconds |
| Original native artifact | One complete plist, 91,609 bytes, 77 actions |
| BRG11 local result | PENDING_EXTERNAL; exactly unit and native-structure records |

```text
go test -count=1 -json -timeout=4m -run '^TestParameterForms(ProducerInventory|ActualProductionEvidence)$' ./tools/language-acceptance
```

The nested producer invokes only the two new BRG11 parents with a three-minute
Go timeout and four-minute process bound. It retains original stdout/stderr,
command, exit status, timing, all 80 required run/pass events and 84 immutable
source witnesses. The source was clean before and after both processes.

Intake rejects a borrowed T01 producer, an RP25 requirement label, fabricated
runtime evidence, a partial test command, missing default/form corruption
events even with reconciled raw hashes, and a changed source-witness digest.
The old short function trace no longer supplies BRG11's runtime mapping.

Independent read-only inspection reconciled every required and observed event,
reread all 84 Git blobs, extracted the exact source string from its committed Go
constant, and decoded the complete original plist from ordered Go output
fragments. All three immutable contract files remain byte-identical to R6.

## Preserved attempts and limits

`evidence/p15-brg11-parameter-forms-20261122/originals.zip` retains the original
process streams, receipts, native bytes within the Go stream, focused full179
audit, independent inspection and development attempts. `manifest.json` records
every member's byte count and SHA-256.

The first draft omitted a separator before `=` after generic type annotations,
so the existing lexer read `>=`; that authored-source syntax was corrected.
The next draft passed the whole native scenario but expected a duplicate
variadic-pack diagnostic at the second pack. Existing canonical binding source
deliberately identifies the first ambiguous pack, so only that new expected
span was corrected. Both failing streams and their source snapshots are
preserved. The focused corrected-span/producer-inventory preflight passed before
the source commit; the clean exact-source replay had no failure.

- Outer stdout SHA-256:
  `69931ce77941384d9e8b347877707c643094d89eba294774a8dfab595d3f3b7b`.
- Nested stdout SHA-256:
  `018a75709e461f6effa21c3b4810ec296d475c301badc30144c72ac59d2b3839`.
- Original native plist SHA-256:
  `362ff8013b6037265920b0ffaca45b130f0f176f9f7f0efbba3cb9da4aa12573`.

The focused audit retains all 179 rows and grants no requirement PASS. Local
manifest validation reports zero passed, zero failed and one skipped requirement
because actual Apple execution is absent; final validation rejects that missing
tier. Related T01, T02, T03, C02, C03, C04, RP25 and BRG15 requirements receive no
inherited credit. No Apple action was executed by this compiler/graph replay.
The historical K1 aggregate remains unchanged until a new combined-source run.
