# RP25 typed defaults and structured transport

RP25's complete local scenario passed on source
`3f1ba846c241a47c158fa3315279877204bb4410`, based on RP24 proof
`5412923b347536b19c60f745b97c9f891f6295eb`. The requirement remains
**PENDING_EXTERNAL** because its immutable iOS tier requires actual Apple
observations. No compiler behavior, contract, minimum tier or historical result
was changed.

## Joined source coverage

Thirteen independently specified sources compare omitted, positional and named
calls to one type-correct function in each source:

| Source cases | Supplied value | Omitted fallback |
| --- | --- | --- |
| Bool / Optional Bool | false | true |
| Number / Optional Number | 0 | 9 |
| Text / Optional Text | empty Text | `DEFAULT` |
| List / Optional List | empty List | `["DEFAULT"]` |
| Map / Optional Map | empty Map | `{"DEFAULT":9}` |
| Empty List default | `["SUPPLIED"]` | empty List |
| Empty Map default | `{"SUPPLIED":7}` | empty Map |
| Optional None default | empty Text | None |

The seven ordinary cases return a Dictionary containing all three actual typed
results. The six Optional cases additionally supply explicit None and retain all
four result values and their independent presence predicates in the final Text
observation. None is not placed unboxed inside an ordinary container.

The whole native graph is checked from dispatcher entry through supplied-argument
selection, fallback ownership, child result, caller conversion/Optional decoder
and the final references. The supplied flag selects the fallback; an Optional's
separate value-presence flag identifies None. Neither decision uses the value's
truthiness. Empty List defaults retain the explicit Nothing, private variable
clear and exact Get Variable producer sequence.

Every action, scalar kind, field wrapper, native control code, group, authored
name and consumer edge is independently specified. Generated UUID names alone
are learned from the actual graph. A source-backed distinction is retained:
direct Optional Bool defaults use a native Bool field, while supplied Bool
arguments follow the existing numeric function-call normalization. The oracle
rejects substituting a Number field for the default Bool.

Native corruption controls cover absence flags, value flags, argument slots,
default branches and joins, native kinds, empty clearing, result carriers,
consumer edges, and reconciled caller argument/mirror or authored-name changes.
The semantic half checks every call's type and all Optional presence predicates;
ten invalid sources require exact wrong-default/wrong-argument diagnostics and
no emitted artifact.

## Actual execution

One clean replay ran on 22 November 2026, 14:10:24–14:10:32 UTC:

| Scope | Observed result |
| --- | --- |
| Outer producer, intake controls and affected producer inventories | 13 PASS, 0 FAIL, 0 SKIP; 7.178548723 seconds |
| Semantic cases and invalid-source controls | 24 PASS |
| Complete native graphs and causal corruptions | 407 PASS |
| Whole nested production process | 431 PASS, 0 FAIL, 0 SKIP; 2.275270728 seconds |
| Native originals | 13 complete plists; 40–100 actions each |
| RP25 result | PENDING_EXTERNAL; exactly unit and native-structure records |

```text
go test -count=1 -json -timeout=4m -run '^TestFunction(CombinedDefault(ProducerInventory|ActualProductionEvidence)|(Default|Transport|Return)ProducerInventory)$' ./tools/language-acceptance
```

The nested producer invokes only the two new RP25 tests with a three-minute Go
timeout and four-minute process bound. All 431 required run/pass events, original
stdout/stderr, timing, command, exit status and 72 immutable Git source witnesses
are retained. Source was clean before and after.

Intake rejects borrowed FN02/FN05 records, a fabricated runtime tier, a partial
command, missing presence/empty-default controls, a changed scenario source hash
and an early case-parent PASS. The existing FN05 nested-event validator and its
mutation helper were extracted without changing behavior so RP25 can reuse them.
Existing T01/FN04, FN05 and RP24 inventory checks passed; their completed component
processes were not rerun for this proof.

Independent inspection reread all 72 Git blobs, reconciled the actual event set,
reconstructed the thirteen source strings independently and decoded their full
native originals from ordered Go output fragments. It also verified that both
existing T01/FN04 producer files are byte-identical to the parent commit.

## Preserved attempts and scope

`evidence/p15-rp25-default-transport-20261122/originals.zip` retains originals,
process receipts, the focused full179 audit, independent intake and every
development attempt. `manifest.json` records each member's size and SHA-256.

The initial draft expected different argument diagnostic wording and incorrectly
expected a numeric Optional Bool fallback. Existing production source explained
both differences; only the new expectations were corrected, and the native Bool
kind gained a dedicated corruption control. A separate registrar preflight found
a filename/identifier collision with existing T01/FN04 files. Their originals were
restored byte-for-byte and the RP25 files moved to distinct
`function_combined_default_execution` names before source freeze. The failing
streams and drafted source snapshots remain intact. No clean replay failed.

- Clean outer stdout SHA-256:
  `8e1abb40201e9951cb34f140b4e83159dc0a83d0c142f1990c3d49d37937eeb6`.
- Clean inner stdout SHA-256:
  `c163514a2104c6fbc8f0475daacbc7dda36ebcd7afcb66c27af935b71b1f8100`.

The focused audit retains all 179 rows and grants no requirement PASS. Local
manifest validation reports zero passed, zero failed and one skipped requirement
because Apple execution is absent; final validation rejects that missing tier.
The historical short function trace no longer supplies runtime mapping for RP25.

T01, FN02, FN05, RP24, RP26, BRG11 and BRG15 receive no inherited evidence. The
historical full179 K1 aggregate remains unchanged until the next combined-source
run. Apple values/types and project closure are not inferred from these graphs.
