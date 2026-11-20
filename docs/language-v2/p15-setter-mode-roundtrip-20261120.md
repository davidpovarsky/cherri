# Setter mode and literal-value semantic round-trip repair

The actual original iPad acceptance fixture08 at `54f76e9f21d8b5904f99206e73455667e9f98c8d` compiled to nine actions, then imported and recompiled to ten. The event URL removal became a Set with literal Text `Remove`. This was a semantic compiler defect, established by the actual CLI outputs retained in the evidence archive. No Reminder or Calendar action was executed.

## Implementation and provenance

- First tests-only baseline: `5cb53d699a196b71b9cf5e5f8e1635dbbc61a7c1`.
- Corrected straight-line graph oracle: `375a1af54e0d787fc96345f212f9d0e1042120d8`.
- Implementation: `542e265468f8b70bd46da3af9912bf463379ec04`.
- Separate action provenance: `1b92d596ec3539fd0caef2657903a5e6226c3738`.

The migration's general lexical shield hid the quoted third argument before the old setter shortcut detector ran. The bounded helper now inspects only that already-selected argument with the existing lexer. An unlabeled three-position Legacy Remove literal is assigned to mode, preserving comments and canonicalizing the case-insensitive Legacy value. Explicit labels remain authoritative; the word Set in the third Legacy slot remains data.

Canonical calls now preserve the absent fourth mode slot when a declared newValue is supplied. This prevents shared Legacy shorthand logic from reinterpreting a named Text value Remove as mode. The existing shared native setter builders retain an explicitly supplied RemoveSpecifiedValue even when that value is the word Remove. No action schema declaration or alternate serializer was introduced.

The earliest editEvent/editReminder provenance commits are unchanged. Both history entries refer to the stable implementation commit above, in a separate follow-up commit.

## Clean-source verification

All commands ran at the exact provenance commit, with a clean worktree before and after. The source archive from the final successful precommit replay was byte-compared to all five implementation/test files before committing. The clean replay then verified the committed source independently.

| Scope | Named passes | Seconds | Raw stdout SHA256 |
| --- | ---: | ---: | --- |
| root | 77 | 13.037 | 985444abcec0405d6329882ae2eea947a683de28bf49ebd6be7bdf39c47580b9 |
| migrate | 19 | 2.846 | 52a331c116f9379bdb90b369b2fe8e79670114ebaace638cde23843c34331cb7 |
| provenance | 3 | 48.622 | 01c65e4170acdb57d67d1f9c7087c193dedce4898213f745d02ffcc611dc1574 |
| schema | 0 | 2.415 | 7afe09f9dc2bfbf0e1697d8b4ff2d3baef785cd12f6aa057841d9bcb50964913 |

Every command exited zero. There were no failed or skipped named events. The schema command has no Go test event count: it compares the generated registry with the actual live catalog and reported it up to date. Exact commands, source SHA, stdout/stderr and all named events are in `setter-clean-replay.json` inside the archive.

The root scope includes independent complete native action graphs, causal corruptions, quote/case/comment migration cases, literal words and empty Text in both modes, explicit four-position Legacy values, the unchanged iPad acceptance kit and original P10R R11–R13 / P10R2 A11–A14 regressions. The entire migration package and three registry/history ancestry checks also passed.

## Retained failures and oracle correction

The first new graph test accidentally reused a function graph finalizer that appends a dispatcher conditional. These fixtures are straight-line programs. Its failed raw stream is retained, but that extra expected conditional is not attributed to the compiler. The corrected oracle compares the complete actual straight-line graph, parameter values and identity relationships without adding a fictitious dispatcher.

The corrected baseline still actually failed fixture08, event Remove-property round-trip and Legacy Remove migration cases. Separate named-value controls established Reminder default-mode and explicitly specified Remove-value defects. The first production attempt also failed four lower/upper-case Legacy enum cases; those original failures and the exact dirty source archive remain. The final lexer-based canonicalization fixed those cases without relaxing the native graph assertions or mutation controls.

| Actual development run | Named passes | Named failures | Raw SHA256 |
| --- | ---: | ---: | --- |
| baseline | 5 | 25 | 73df1c084c23ee6feea76321d313a4122913a2122ba72cef214abf787563c739 |
| corrected-baseline | 29 | 13 | 9bd27173268ca4006edd527e7b2607dbadb84c826fc64a0426f57dc1770f29a8 |
| named-value-baseline | 18 | 6 | 78eef5c0d5c5691d5ce13953dca732e8add6114cda9ed20ab6f8bc2dbf80c78b |
| precommit-root | 68 | 5 | c849429da29f1f354ae5d6fe2895ebcd17f02327154adc5e0435145808f4e62d |
| canonical-mode-precommit-root | 77 | 0 | bd99ed694f64292763c675f5c0f3a77bef436fec6271ffa8337aab9114953f9f |

Failed package events are additionally retained in the raw streams; the table counts named test events, including parents. Development source ZIPs are explicitly dirty/precommit snapshots and are not labeled clean proof.

## Evidence and limits

The sibling evidence ZIP contains every original receipt, event stream, relevant source snapshot, initial nine-to-ten compiled/imported plist, and the independent clean replay. Its manifest records exact bytes and hashes. Rebuildable executables are omitted from the ZIP; their original build receipts and SHA256 digests are retained.

This component proves compile/import semantics and provenance/catalog consistency at its pinned source. Physical iPad, Apple execution of Reminder/Calendar actions, full repository CI and final179 acceptance remain separate gates. No immutable requirement, original iPad fixture or native golden was changed.

Archive SHA256: `74b533af5fb8ab5d8a0ea4f2de444cc8d6de75169547328c00c1dec6c3319026`.
