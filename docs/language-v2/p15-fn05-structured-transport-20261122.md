# FN05 structured argument and result transport

The complete **local FN05 requirement passed** on source
`a38ae7d41b312b4a797ed3ae4493b462bf912a3f`, based on the integrated
`d2f86d2998bb15d9271a58279a51a400eb13d3d5` source. The original contract still
requires unit and native evidence and still permits an exact reported target
limitation. No contract text, required tier, compiler behavior, signing path or
historical result was changed.

## What changed

The old acceptance probe lowered a function with Text and Number parameters and
checked that its IR was nonempty. It could not establish structured transport;
the existing scope guard correctly withheld formal credit.

FN05 now executes two distinct production tests through the real canonical
compiler. The semantic test checks the parameter, call and result types. The
native test independently specifies the complete serialized graph from the
dispatcher through the argument, child result, caller decoder and final return.
Only generated UUID names are learned from the actual graph. Scalar kinds,
container nesting, order, names, references, control ancestry and the exact
private envelope remain independently specified.

The existing staged function cursor was updated to the already integrated
dual-marker dispatcher. This consumes the existing repair; it does not claim
complete malformed-envelope acceptance for RP26 or BRG15. The shared strict
short-circuit cursor and container-field oracle are reused.

Twelve supported cases exercise:

- A bound `List<Number>` containing integer zero and fractional 2.5, plus an
  explicit empty List.
- An empty Map and a nested Map containing false, zero, empty Text, empty List,
  empty Map, literal braces and keys resembling native serialization metadata.
- Optional absence, present empty Text, false and zero.
- Present empty and nonempty Optional Lists and Maps.

Three additional source cases retain precise unsupported-shape behavior:
Optional absence inside ordinary List/Map values reports `E_TARGET_VALUE_SHAPE`,
and a function argument containing native List-valued rows reports
`E_NESTED_NATIVE_LIST` at both exact offending spans. Each invalid source emits
no native artifact. The tests reject changed codes, ranges, contexts, severity,
missing diagnostics and extra diagnostics. These are current reported
limitations, not claims that the shapes can never be implemented.

Native corruption controls cover every argument/result boundary, presence versus
value, scalar kind, producer/consumer identity and final consumer. The nested Map
controls change both argument copies consistently, so their rejection depends
on preserving the actual shape and values rather than merely noticing a broken
positional mirror.

## Actual execution

One clean replay ran on 22 November 2026, 13:48:37–13:48:41 UTC:

| Scope | Observed result |
| --- | --- |
| Outer producer, inventory and intake controls | 14 PASS, 0 FAIL, 0 SKIP; 4.173299218 seconds |
| Nested semantic/limitation test | All 37 fixed events PASS |
| Nested native graph/control test | All 321 fixed events PASS |
| Whole nested production process | 358 PASS, 0 FAIL, 0 SKIP; 2.660058803 seconds |
| Local FN05 validation | 1 requirement PASS; exactly unit and native-structure records |

Both processes exited zero within their original bounds. The source was clean
before and after. No full suite or already completed component suite was
repeated for this proof.

The outer command was:

```text
go test -count=1 -json -timeout=4m -run '^TestFunctionTransport(ProducerInventory|ActualProductionEvidence)$' ./tools/language-acceptance
```

The producer invokes only its two root tests with a three-minute Go timeout and
a four-minute process bound. It records all 358 required run/pass identities,
the exact command, timing, exit status, original stdout/stderr and 68 immutable
Git source witnesses. Intake rejects a partial command, missing/skipped child,
foreign package, early case-parent completion, dirty or detached source,
missing artifact, cross-tier producer and substituted requirement/runtime tier.

Independent inspection reread all 68 Git blobs and checked their recorded
hashes, reconciled the outer and inner events, and decoded all twelve original
plist payloads from the ordered Go output fragments. Their source bytes and
native bytes matched the logged hashes. The payloads contain 30–48 native
actions. The first independent reader incorrectly treated one fragmented Go
output event as a complete Base64 payload; that reader error is retained beside
the corrected intake. The original test streams were not changed or rerun.

## Evidence and limits

Originals, both development runs, process receipts and the independent intake
are retained in
`evidence/p15-fn05-structured-transport-20261122/originals.zip`.
`manifest.json` records every member's size and SHA-256.

- Outer stdout SHA-256:
  `4491194c9fd411e2e5ee3ef2bb51e589817e46bb6e02c5d5d6e5705c464fd895`.
- Inner stdout SHA-256:
  `556ddd2c1e65f632e1da2a4d4a28683678137a7584fe0172135305ae7a9ca3e4`.

The new producer replaces only FN05's old smoke and registers only FN05's two
required local tiers. T05, L01, F04, RP20 and the separate combined function
requirements receive no inherited evidence. In particular, this does not prove
JSON-null preservation, one-element nested loop collection or Apple runtime
values.

The full 179-row audit produced from this focused run still includes every
immutable requirement and grants PASS only to FN05. It reports no project
closure. The last historical full run remains K1 at `d90b69a9`:
106 PASSED, 4 FAILED, 45 NOT_RUN, 23 PENDING_EXTERNAL and 1 AI_EVAL_NOT_RUN.
Only a new combined-source run can replace that aggregate.
