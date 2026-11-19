# D03 strict intake: source-bound local proof

Implementation: 1489ce92e597d30a727a285a43768b78a4497551.

The exact clean source passed 78 named Go cases, with zero failures or skips,
in 32.937 seconds. This includes the real iOS collector CLI adapter, dedicated
D03 formal-manifest scopes and canonical 179 inventory/smoke controls.
clean-intake-1489ce9-receipt.json binds the original 79,333-byte Go JSON log
to SHA256 6294a27bbd3477c8ec61b4c24de98b60161e2139200a7977c5e085b5d87c81a5.
Source status was clean before and after execution.

The separate POSIX Python gate passed 11 methods against four exact source
Git-blob hashes from the same implementation. It runs real installed-wrapper
shell subprocesses in synthetic Git fixtures with explicitly modeled HTTP.
The local runtime checkout is a bounded source-file copy, not a complete
Git clone; the receipt records every required file hash before and after.
It establishes neither the production install nor an Apple execution.

## Causal review baseline

review-gap-baseline-* preserves the actual failure before the repair:
five healthy controls passed; three leaf cases failed (five failure events
including parents). Fully rehashed invented or relabeled bundle bodies were
wrongly accepted, and an indented valid raw receipt emitted by the actual
collector was rejected. The 12,146-byte raw log has SHA256
2683d8015fb2da0782d1d6f23d46a784124c2f5470358c1392b97c9747d68fed.
An earlier launch had a Python driver syntax error before any Go process;
it is not represented as an executed test baseline.

The final healthy fixture uses the real pinned canonical generator. The
body mutations preserve canonical JSON ordering and whitespace and must
fail specifically at complete content comparison. Generator bytes are
read from K, and the four Markdown inputs from D, rather than trusting
the index's source-hash declarations. Surrounding receipt whitespace is
parsed consistently with the collector while original hashed bytes remain
unchanged. Missing/extra inputs, mutable worktree generator substitution,
and a missing generator Git object are controlled independently.

The earlier working gates are retained as intermediate observations.
They do not replace the exact clean implementation receipt.

## Scope and dependencies

The implementation depends on the canonical app generator/helper from
d37b433 + abe468550faac63dead02e6e33ca28744f0fd8d3 and the iOS collector
e48a9430c6285c89ba785fe85f2437e41153656e. Their isolated cherry-picks are
ancestors of this branch; integration should use the original owned commits.

D03 remains open until both dedicated external records exist at the final
S/D/K tuple. This archive contains local intake/model proof only. It does not
establish an actual iOS UI run, installed production Skill, native Shortcut
execution, physical-device acceptance or release completion.

All original evidence bytes are preserved in archive-index.json.
Any Go source snapshot has a .go.txt extension so archives do not become
recursive Go packages. Original receipt paths are retained for audit.
