# BRG29 generation event inventory — 2026-11-26

The original BRG29 producer selects three canonical documentation test parents.
Its fault-producing parent now has ten actual child identities, including
`scoped_dialect` and `scoped_void_result`; the intake now requires all thirteen
events. The scope description records 505 native signatures plus sixteen
macros, and all ten original drift controls. No parent or earlier child was
removed and the producer selection is unchanged.

The fixture is the byte-identical, unfiltered fourteen-event Docs stream from
compiler S708, which includes a fourth Docs-only parent. It is used solely as a
historical parser control. The strict BRG29 intake continues to reject that
four-parent stream as a BRG29 execution. It is never filtered into a claimed
three-parent run. Its process receipt is preserved unchanged.

Actual local Python RED selected two tests and produced one source-inventory
failure plus one real parser error (`unselected generation event`). After the
two production corrections, all four Python tests passed. Fourteen adverse
mutations reject missing or failed/skipped fault events. The first earlier
assertion-only failure remains identified in the RED receipt. No Go, compiler,
canonical producer, native runtime or formal acceptance was executed here.

`evidence/p15-brg29-generation-inventory-20261126/verification.json` binds the
original RED/GREEN streams, exact source copies and unfiltered historical
fixtures in the adjacent `originals.zip`. These source changes are outside the
canonical C1, C2 and macro-ledger input closures. The original BRG29 producer
still requires a fresh run on the final consumer tuple.
