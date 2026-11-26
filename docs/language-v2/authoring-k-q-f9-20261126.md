# Q/f9 authoring manifest and iOS documentation index

The authoring manifest now binds the actual Q compiler source and the qualified
f9 documentation source. The published authoring change is
[3c920873605bd81e098841c2d41686fd1cda573b](https://github.com/davidpovarsky/cherri/commit/3c920873605bd81e098841c2d41686fd1cda573b).
Its sole parent is Q. Only `compilerCommit` and `docsCommit` changed in
`skills/cherri-shortcuts/compatibility-manifest.json`; every other field and
capability fingerprint is unchanged.

| Role | Exact source |
| --- | --- |
| Compiler S | `44707350fb6e52dd0d947256bced105ac762a1ac` |
| Documentation D | `f9ec0e0c62b5fc26ddb6002275bc1abba2c5a257` |
| Authoring K | `3c920873605bd81e098841c2d41686fd1cda573b` |
| Final release F | Not selected by this component |

The earlier S708/D381 tuple remains historical evidence. Unchanged capability
fingerprints do not imply that its compiler contains the later production
repairs in Q. This change uses the actual Q pair and its source receipts.

## Actual input and generator verification

All seven retained compiler/migrator products were rehashed against the audited
Q pair handoff. The two binary hashes are:

- compiler: `453fed348591bd0e611c10b7571b5a1c6ea8ba8e1168067d85f5541899403def`
- migrator: `b3e9a2b52a752d7e4d211dc9c8eed01c81ae6b21bc45649822858c6dcf63f52b`

This component did not rebuild or execute that pair. It checked the original
product identities and emitted capability records. The f9 Docs CI and its
independent original intake were separate completed inputs.

The original `ios/scripts/build_documentation_index.py` was then executed with
the exact proposed manifest and the existing clean ordinary f9 Docs checkout.
The original `--verify-output` invocation followed it. Both commands passed.
The generator's `--fetch` option used the existing checkout; no substitute
renderer or rewritten documentation was supplied.

The output contains **569 sections from four sources**, occupies **546,645
bytes**, and has SHA-256:

`1f94a32720cad9c6f6e74225a25787aff2accefccd9a5f9b91f6d24462336483`

The original generator source hash is
`f80c86792294bf074ef1ada887f3c2b39869a0e459364cdf56dbea2864b7e252`.
The qualified manifest hash is
`9c18398b2342d232865633fe2e7100ace47710f6f9c1284ec448fe76f214cb00`.
The committed manifest is byte-identical to the proposal used by the actual
generator and verifier. Source guards passed before and after execution.
Generation retained its 180-second child bound, verification 30 seconds, and
the whole owner 300 seconds.

## Original receipts

The adjacent [manifest](evidence/authoring-k-q-f9-20261126/manifest.json)
enumerates every member of the
[original archive](evidence/authoring-k-q-f9-20261126/originals.zip).
The archive preserves all **177** files from the three completed preparation,
generation/verification and publication runs, plus the two exact source inputs
and the retention owner: **180 files**, not a test count.

| Original receipt | SHA-256 |
| --- | --- |
| `preparation-01/result.original.json` | `27f1537759f201d7a4d32cc6662bc3f2b9ea46b5a75f87f9ed7a8abe4f9008fd` |
| `ios-documentation-Q-f9-01/result.original.json` | `ce739cd27a9025319cc7da5fe89ef58e4ed2b5db676ad9d6f020355480f3630d` |
| `source-publication-01/result.original.json` | `9e4a086b54b1fe0cdb8371c5af67fb30e95357954eee7d846cfd34304b62183a` |

The archive is 160,397 bytes, SHA-256
`ac9c57cbcbf24456313d4db06665014e8f2dfc31f09ee82b3e928a23d5a8387c`.
Its complete readback matches each original byte sequence and mode; the
manifest retains source size, owner, group and modification time. The
[retention receipt](evidence/authoring-k-q-f9-20261126/retention.original.json)
records this comparison without rerunning either generator command.

## Remaining boundaries

This is authoring-manifest and documentation-index component evidence. It does
not grant Apple runtime/UI acceptance, final 179 closure, website deployment,
or RemoteBuilder promotion.

Final source F must supply the final package's `--skill-commit` and
`skillSourceHead`; the external compiler and Docs identities remain their exact
qualified S and D. The original 39 entrypoint checks, 58 signing checks, package
and unpack integrity, signed smoke, and live promotion/rollback/reapply are
still required on that final tuple. No such operation ran in this component.

The separately requested historical Authoring Patterns bundle (24 pages,
22 rules and three examples) has not been recovered or installed by this work.
The current documentation and this manifest update do not replace that
original requirement.
