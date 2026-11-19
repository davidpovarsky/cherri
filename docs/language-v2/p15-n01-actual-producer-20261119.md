# N01 fractional arithmetic: actual local producer

At clean implementation `db918ee8493907c90da449e67e1be6df1dc43770`, the formal
N01 producer executes the real compiler's numeric diagnostics and serialized
native dataflow tests. Its unchanged contract requires `unit+native+iOS`.
The producer supplies only `unit` and `native-structure`; N01 remains
`PENDING_EXTERNAL` until genuine source-bound Apple execution is accepted.

## Covered behavior

Ten authored programs verify exact native arithmetic and final consumer
edges for `5/2`, negative operands, fractional numerators and denominators,
small fractions, nested expressions, references to previous results, dynamic
variables, fractional compound assignment, and a compound arithmetic chain.
The independent graph oracle accepts only the required Number, Math, variable
assignment, and terminal Clipboard action shapes. It rejects additional
rounding/coercion steps, wrong operands, missing/duplicate/forward producer
identities, and a detached terminal consumer. It inspects serialized actions;
it does not execute or emulate the Apple runtime.

Eight native graph mutations include integer truncation, a changed divisor,
wrong operation, text in a numeric operand, a wrong output edge, unexpected
coercion, inserted rounding, and a missing final observation. Six invalid
programs verify precise source spans and diagnostic codes for literal zero
division/modulus and nonnumeric operands, absence of any emitted artifact,
and successful recovery on valid fractional input. Wrong-code and wrong-span
controls cannot satisfy those diagnostic checks.

The acceptance producer independently selects both required named tests.
Its strict intake binds the complete command and test inventory to distinct
unit/native producers. Eighteen corruption controls reject a wrong producer,
invented iOS tier, borrowed requirement, truncated command, missing test
binding, wrong scope, stale source, failed process, and a missing local tier.
The execution audit retains all 179 immutable rows and gives no related repair
or delivery gate an inherited result.

## Actual verification and retained provenance

| Run | Actual result |
|---|---|
| Core numeric tests plus existing defaults controls | 73 named pass events, four top-level tests, exit 0, no skips/failures, 99.996 seconds; parent `d13ab702` with only the new numeric test file |
| New N01 producer/intake controls before commit | 22 named pass events, exit 0, no skips/failures, 20.482 seconds; exact dirty-source status retained |
| Clean N01 producer/intake replay | 22 named pass events, exit 0, no skips/failures, 9.638 seconds; exact clean `db918ee8` before and after |

Of the 73 initial core events, 26 belong to the new N01 tests and 47 to the
existing presence/default controls. Those existing controls do not register
FN02 or T01 in this commit. The clean producer invokes the actual two N01
tests and preserves their successful named selection in its receipt; its
outer JSONL and local two-tier records are retained. The initial direct core
JSONL retains its original parent/test-file provenance and is not relabeled
as execution of the later clean commit.

The [evidence directory](evidence/p15-n01-producer-20261119/) preserves the
original logs, summaries, empty stderr files, and exact producer receipt.
The clean log SHA-256 is
`4078daa9ae4b764238966ac9642b4ef9ef0a6df20f2cd1f9802d09cec8953dd4`.
`artifact-hashes.json` records all retained file digests. A guarded source
patch initially stopped before any test execution because its exact-match
text omitted a comma; that no-execution receipt is also retained.

Reproduce on a clean checkout:

```sh
go test -count=1 -timeout=5m -json ./tools/language-acceptance \
  -run '^(TestNumberFunctionProducerInventory|TestNumberFunctionActualProductionEvidence)$'
```

No compiler semantics, benchmark/process budgets, or immutable acceptance
contracts changed. FN01–FN03 producers, the complete integrated 179 report,
Apple runtime, consumer deployment, and release remain separate work.
